package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/smtp"
	"os"
	"strings"
	"sync"
	"time"
)

// OutgoingConfig represents the configuration for outgoing mail
type OutgoingConfig struct {
	Host          string
	Port          int
	User          string
	Password      string
	Secure        bool // Use TLS
	AutoRelay     bool
	AutoRelayAddr string
	AllowRules    []string // Allow list patterns
	DenyRules     []string // Deny list patterns
}

// OutgoingMail handles outgoing email relay
type OutgoingMail struct {
	config      *OutgoingConfig
	queue       chan *RelayTask
	workerCount int
	wg          sync.WaitGroup
	mu          sync.RWMutex
	enabled     bool
}

// RelayTask represents a task to relay an email
type RelayTask struct {
	Email       *Email
	EmailPath   string
	RelayTo     string // Optional relay address
	IsAutoRelay bool
	Callback    func(error)
}

// NewOutgoingMail creates a new outgoing mail handler
func NewOutgoingMail(config *OutgoingConfig) *OutgoingMail {
	if config == nil {
		config = &OutgoingConfig{}
	}

	om := &OutgoingMail{
		config:      config,
		queue:       make(chan *RelayTask, 100),
		workerCount: 1,
		enabled:     config.Host != "",
	}

	if om.enabled {
		// Start worker goroutines
		for i := 0; i < om.workerCount; i++ {
			om.wg.Add(1)
			go om.worker()
		}
	}

	return om
}

// worker processes relay tasks from the queue
func (om *OutgoingMail) worker() {
	defer om.wg.Done()

	for task := range om.queue {
		err := om.relayEmail(task)
		if task.Callback != nil {
			task.Callback(err)
		}
	}
}

// relayEmail relays an email to the configured SMTP server
func (om *OutgoingMail) relayEmail(task *RelayTask) error {
	if !om.enabled {
		return fmt.Errorf("outgoing mail not configured")
	}

	// Determine recipients
	recipients := om.getRecipients(task)
	if len(recipients) == 0 {
		return fmt.Errorf("email had no recipients")
	}

	// Read email file
	emailFile, err := os.Open(task.EmailPath)
	if err != nil {
		return fmt.Errorf("failed to open email file: %w", err)
	}
	defer emailFile.Close()

	// Get sender address
	sender := task.Email.Envelope.From
	if sender == "" && len(task.Email.From) > 0 {
		sender = task.Email.From[0].Address
	}
	if sender == "" {
		sender = "noreply@localhost"
	}

	// Read email file content
	emailData, err := io.ReadAll(emailFile)
	if err != nil {
		return fmt.Errorf("failed to read email file: %w", err)
	}

	// Prepare SMTP auth
	var auth smtp.Auth
	if om.config.User != "" && om.config.Password != "" {
		auth = smtp.PlainAuth("", om.config.User, om.config.Password, om.config.Host)
	}

	// Send email using net/smtp
	addr := fmt.Sprintf("%s:%d", om.config.Host, om.config.Port)

	if om.config.Secure {
		// Use TLS
		err = sendMailTLS(addr, auth, sender, recipients, emailData)
	} else {
		// Use plain SMTP
		err = smtp.SendMail(addr, auth, sender, recipients, emailData)
	}

	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	Log("Mail relayed successfully: %s (to: %v)", task.Email.Subject, recipients)
	return nil
}

// getRecipients determines the recipients for relay
func (om *OutgoingMail) getRecipients(task *RelayTask) []string {
	var recipients []string

	// If manual relay with specific address
	if task.RelayTo != "" {
		return []string{task.RelayTo}
	}

	// If auto relay mode with specific address
	if task.IsAutoRelay && om.config.AutoRelayAddr != "" {
		return []string{om.config.AutoRelayAddr}
	}

	// Get recipients from envelope
	if task.Email.Envelope != nil {
		recipients = append(recipients, task.Email.Envelope.To...)
	}

	// Apply allow/deny rules
	if len(om.config.AllowRules) > 0 || len(om.config.DenyRules) > 0 {
		recipients = om.filterRecipients(recipients)
	}

	return recipients
}

// filterRecipients applies allow/deny rules to recipients
func (om *OutgoingMail) filterRecipients(recipients []string) []string {
	filtered := make([]string, 0)

	for _, recipient := range recipients {
		// Check deny rules first
		denied := false
		for _, rule := range om.config.DenyRules {
			if om.matchesRule(recipient, rule) {
				denied = true
				break
			}
		}
		if denied {
			continue
		}

		// If allow rules exist, check them
		if len(om.config.AllowRules) > 0 {
			allowed := false
			for _, rule := range om.config.AllowRules {
				if om.matchesRule(recipient, rule) {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}

		filtered = append(filtered, recipient)
	}

	return filtered
}

// matchesRule checks if an address matches a rule pattern
func (om *OutgoingMail) matchesRule(address, rule string) bool {
	// Simple pattern matching: supports * wildcard
	pattern := strings.ToLower(rule)
	addr := strings.ToLower(address)

	// Exact match
	if pattern == addr {
		return true
	}

	// Wildcard matching
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]
			return strings.HasPrefix(addr, prefix) && strings.HasSuffix(addr, suffix)
		} else if len(parts) == 1 {
			if strings.HasPrefix(pattern, "*") {
				return strings.HasSuffix(addr, parts[0])
			} else if strings.HasSuffix(pattern, "*") {
				return strings.HasPrefix(addr, parts[0])
			}
		}
	}

	return false
}

// RelayMail queues an email for relay
func (om *OutgoingMail) RelayMail(email *Email, emailPath string, relayTo string, isAutoRelay bool, callback func(error)) {
	if !om.enabled {
		if callback != nil {
			callback(fmt.Errorf("outgoing mail not configured"))
		}
		return
	}

	task := &RelayTask{
		Email:       email,
		EmailPath:   emailPath,
		RelayTo:     relayTo,
		IsAutoRelay: isAutoRelay,
		Callback:    callback,
	}

	select {
	case om.queue <- task:
		// Task queued successfully
	case <-time.After(5 * time.Second):
		// Queue full, call callback with error
		if callback != nil {
			callback(fmt.Errorf("relay queue is full"))
		}
	}
}

// IsAutoRelayEnabled checks if auto relay is enabled
func (om *OutgoingMail) IsAutoRelayEnabled() bool {
	om.mu.RLock()
	defer om.mu.RUnlock()
	return om.enabled && om.config.AutoRelay
}

// UpdateConfig updates the outgoing mail configuration
func (om *OutgoingMail) UpdateConfig(config *OutgoingConfig) {
	om.mu.Lock()
	defer om.mu.Unlock()

	om.config = config
	om.enabled = config.Host != ""
}

// GetConfig returns the current configuration
func (om *OutgoingMail) GetConfig() *OutgoingConfig {
	om.mu.RLock()
	defer om.mu.RUnlock()
	return om.config
}

// Close stops the outgoing mail handler
func (om *OutgoingMail) Close() {
	close(om.queue)
	om.wg.Wait()
}

// sendMailTLS sends email using TLS
func sendMailTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	// Connect to SMTP server
	client, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer client.Close()

	// Check if server supports STARTTLS
	if ok, _ := client.Extension("STARTTLS"); ok {
		config := &tls.Config{ServerName: strings.Split(addr, ":")[0]}
		if err = client.StartTLS(config); err != nil {
			return err
		}
	}

	// Authenticate if needed
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return err
		}
	}

	// Set sender
	if err = client.Mail(from); err != nil {
		return err
	}

	// Set recipients
	for _, recipient := range to {
		if err = client.Rcpt(recipient); err != nil {
			return err
		}
	}

	// Send email data
	w, err := client.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(msg)
	if err != nil {
		w.Close()
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}

	return client.Quit()
}

package outgoing

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/smtp"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/soulteary/owlmail/internal/common"
	"github.com/soulteary/owlmail/internal/types"
)

// OutgoingConfig represents the configuration for outgoing mail
type OutgoingConfig struct {
	Host                string
	Port                int
	User                string
	Password            string
	Secure              bool    // Deprecated compatibility alias: true means implicit TLS/SMTPS.
	TLSMode             TLSMode // plain, starttls, or smtps
	InsecureSkipVerify  bool
	ConnectTimeout      string
	TLSHandshakeTimeout string
	AuthTimeout         string
	EnvelopeTimeout     string
	DataTimeout         string
	QuitTimeout         string
	tlsConfig           *tls.Config
	AutoRelay           bool
	AutoRelayAddr       string
	AllowRules          []string // Allow list patterns
	DenyRules           []string // Deny list patterns
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
	Email       *types.Email
	EmailPath   string
	RelayTo     string // Optional relay address
	IsAutoRelay bool
	Callback    func(error)
	Context     context.Context
}

// NewOutgoingMail creates a new outgoing mail handler
func NewOutgoingMail(config *OutgoingConfig) *OutgoingMail {
	if config == nil {
		config = &OutgoingConfig{}
	}
	config = config.withDefaults()

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
	if task.Context != nil {
		if err := task.Context.Err(); err != nil {
			return err
		}
	}
	config := om.configSnapshot()
	if config == nil || config.Host == "" {
		return fmt.Errorf("outgoing mail not configured")
	}
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid outgoing mail configuration: %w", err)
	}

	// Determine recipients
	recipients := om.getRecipientsWithConfig(task, config)
	if len(recipients) == 0 {
		return fmt.Errorf("email had no recipients")
	}

	// Read email file
	emailFile, err := os.Open(task.EmailPath)
	if err != nil {
		return fmt.Errorf("failed to open email file: %w", err)
	}
	defer func() {
		if err := emailFile.Close(); err != nil {
			common.Verbose("Failed to close email file: %v", err)
		}
	}()

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
	if config.User != "" && config.Password != "" {
		auth = smtp.PlainAuth("", config.User, config.Password, config.Host)
	}

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	ctx := task.Context
	if ctx == nil {
		ctx = context.Background()
	}
	err = sendMailWithConfig(ctx, addr, auth, sender, recipients, emailData, config)

	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	common.Log("Mail relayed successfully: %s (to: %v)", task.Email.Subject, recipients)
	return nil
}

// getRecipients determines the recipients for relay
func (om *OutgoingMail) getRecipients(task *RelayTask) []string {
	return om.getRecipientsWithConfig(task, om.configSnapshot())
}

func (om *OutgoingMail) getRecipientsWithConfig(task *RelayTask, config *OutgoingConfig) []string {
	var recipients []string
	if config == nil {
		return recipients
	}

	// If manual relay with specific address
	if task.RelayTo != "" {
		return []string{task.RelayTo}
	}

	// If auto relay mode with specific address
	if task.IsAutoRelay && config.AutoRelayAddr != "" {
		return []string{config.AutoRelayAddr}
	}

	// Get recipients from envelope
	if task.Email.Envelope != nil {
		recipients = append(recipients, task.Email.Envelope.To...)
	}

	// Apply allow/deny rules
	if len(config.AllowRules) > 0 || len(config.DenyRules) > 0 {
		recipients = om.filterRecipientsWithConfig(recipients, config)
	}

	return recipients
}

// filterRecipients applies allow/deny rules to recipients
// Rules are processed in order, and the last matching rule wins (like MailDev)
func (om *OutgoingMail) filterRecipients(recipients []string) []string {
	return om.filterRecipientsWithConfig(recipients, om.configSnapshot())
}

func (om *OutgoingMail) filterRecipientsWithConfig(recipients []string, config *OutgoingConfig) []string {
	filtered := make([]string, 0)
	if config == nil {
		return filtered
	}

	for _, recipient := range recipients {
		// Process all rules in order to find the last matching rule
		// Start with default: allow if no allow rules, deny if allow rules exist
		result := len(config.AllowRules) == 0

		// Process deny rules
		for _, rule := range config.DenyRules {
			if om.matchesRule(recipient, rule) {
				result = false // Deny if matched
			}
		}

		// Process allow rules (can override deny)
		for _, rule := range config.AllowRules {
			if om.matchesRule(recipient, rule) {
				result = true // Allow if matched
			}
		}

		if result {
			filtered = append(filtered, recipient)
		}
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
func (om *OutgoingMail) RelayMail(email *types.Email, emailPath string, relayTo string, isAutoRelay bool, callback func(error)) {
	om.enqueueRelay(context.Background(), false, email, emailPath, relayTo, isAutoRelay, callback)
}

// RelayMailContext queues an email relay that is canceled if the caller's
// context expires while the task is queued or in progress.
func (om *OutgoingMail) RelayMailContext(ctx context.Context, email *types.Email, emailPath string, relayTo string, isAutoRelay bool, callback func(error)) {
	if ctx == nil {
		ctx = context.Background()
	}
	om.enqueueRelay(ctx, true, email, emailPath, relayTo, isAutoRelay, callback)
}

func (om *OutgoingMail) enqueueRelay(ctx context.Context, contextAware bool, email *types.Email, emailPath string, relayTo string, isAutoRelay bool, callback func(error)) {
	if !om.isEnabled() {
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
	if contextAware {
		task.Context = ctx
		if err := ctx.Err(); err != nil {
			if callback != nil {
				callback(err)
			}
			return
		}
	}

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	var canceled <-chan struct{}
	if contextAware {
		canceled = ctx.Done()
	}
	select {
	case om.queue <- task:
		// Task queued successfully
	case <-timer.C:
		// Queue full, call callback with error
		if callback != nil {
			callback(fmt.Errorf("relay queue is full"))
		}
	case <-canceled:
		if callback != nil {
			callback(ctx.Err())
		}
	}
}

// IsAutoRelayEnabled checks if auto relay is enabled
func (om *OutgoingMail) IsAutoRelayEnabled() bool {
	om.mu.RLock()
	defer om.mu.RUnlock()
	return om.enabled && om.config.AutoRelay
}

func (om *OutgoingMail) isEnabled() bool {
	om.mu.RLock()
	defer om.mu.RUnlock()
	return om.enabled
}

// UpdateConfig updates the outgoing mail configuration
func (om *OutgoingMail) UpdateConfig(config interface{}) {
	om.mu.Lock()
	defer om.mu.Unlock()

	if cfg, ok := config.(*OutgoingConfig); ok {
		om.config = cfg.withDefaults()
		om.enabled = cfg.Host != ""
	}
}

// GetConfig returns the current configuration
func (om *OutgoingMail) GetConfig() interface{} {
	return om.configSnapshot()
}

func (om *OutgoingMail) configSnapshot() *OutgoingConfig {
	om.mu.RLock()
	defer om.mu.RUnlock()
	if om.config == nil {
		return nil
	}
	config := *om.config
	config.AllowRules = append([]string(nil), om.config.AllowRules...)
	config.DenyRules = append([]string(nil), om.config.DenyRules...)
	return &config
}

// Close stops the outgoing mail handler
func (om *OutgoingMail) Close() {
	close(om.queue)
	om.wg.Wait()
}

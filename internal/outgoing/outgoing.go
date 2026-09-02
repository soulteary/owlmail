package outgoing

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/soulteary/owlmail/internal/common"
	"github.com/soulteary/owlmail/internal/types"
)

var (
	// ErrNotConfigured is returned when relay is disabled because no host is set.
	ErrNotConfigured = errors.New("outgoing mail not configured")
	// ErrQueueFull is returned when a relay task cannot be queued before the timeout.
	ErrQueueFull = errors.New("relay queue is full")
	// ErrClosed is returned when work is submitted after the relay has begun closing.
	ErrClosed = errors.New("outgoing mail is closed")
)

const defaultEnqueueTimeout = 5 * time.Second

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
	config         *OutgoingConfig
	queue          chan *RelayTask
	workerCount    int
	enqueueTimeout time.Duration
	wg             sync.WaitGroup
	enqueueWG      sync.WaitGroup
	mu             sync.RWMutex
	enabled        bool
	workerStarted  bool
	closed         bool
	closing        chan struct{}
	stop           chan struct{}
	closeOnce      sync.Once
}

// RelayTask represents a task to relay an email
type RelayTask struct {
	Email       *types.Email
	EmailPath   string
	RelayTo     string // Optional relay address
	IsAutoRelay bool
	Callback    func(error)
	Context     context.Context
	config      *OutgoingConfig
}

const relayCopyBufferSize = 32 * 1024

// NewOutgoingMail creates a new outgoing mail handler
func NewOutgoingMail(config *OutgoingConfig) *OutgoingMail {
	config = cloneConfig(config)

	om := &OutgoingMail{
		config:         config,
		queue:          make(chan *RelayTask, 100),
		workerCount:    1,
		enqueueTimeout: defaultEnqueueTimeout,
		enabled:        config.Host != "",
		closing:        make(chan struct{}),
		stop:           make(chan struct{}),
	}

	if om.enabled {
		om.startWorkersLocked()
	}

	return om
}

// worker processes relay tasks from the queue
func (om *OutgoingMail) worker() {
	defer om.wg.Done()

	for {
		select {
		case task := <-om.queue:
			om.processTask(task)
		case <-om.stop:
			// Close waits for active enqueue operations before signaling stop,
			// so the queue is stable and can be drained without closing it.
			for {
				select {
				case task := <-om.queue:
					om.processTask(task)
				default:
					return
				}
			}
		}
	}
}

func (om *OutgoingMail) processTask(task *RelayTask) {
	err := om.relayEmail(task)
	if task.Callback != nil {
		task.Callback(err)
	}
}

func (om *OutgoingMail) startWorkersLocked() {
	if om.workerStarted || om.closed {
		return
	}
	om.workerStarted = true
	for i := 0; i < om.workerCount; i++ {
		om.wg.Add(1)
		go om.worker()
	}
}

// relayEmail relays an email to the configured SMTP server
func (om *OutgoingMail) relayEmail(task *RelayTask) error {
	ctx := task.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	config := task.config
	if config == nil {
		config = om.configSnapshot()
	}
	if config.Host == "" {
		return ErrNotConfigured
	}

	// Determine recipients
	recipients := getRecipients(task, config)
	if len(recipients) == 0 {
		return fmt.Errorf("email had no recipients")
	}

	// Read email file
	emailFile, err := os.Open(task.EmailPath)
	if err != nil {
		return fmt.Errorf("failed to open email file: %w", err)
	}

	// Get sender address
	sender := task.Email.Envelope.From
	if sender == "" && len(task.Email.From) > 0 {
		sender = task.Email.From[0].Address
	}
	if sender == "" {
		sender = "noreply@localhost"
	}

	// Prepare SMTP auth
	var auth smtp.Auth
	if config.User != "" && config.Password != "" {
		auth = smtp.PlainAuth("", config.User, config.Password, config.Host)
	}

	// Send email using net/smtp
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	useSTARTTLS := config.Secure
	authOnlyWhenAdvertised := task.Context == nil && !config.Secure
	if task.Context == nil {
		// smtp.SendMail, which handled the ordinary asynchronous path before
		// relay streaming, opportunistically upgraded whenever STARTTLS was
		// advertised. Preserve that behavior in this performance-only change.
		useSTARTTLS = true
	}
	err = sendMailContext(ctx, addr, auth, sender, recipients, emailFile, useSTARTTLS, authOnlyWhenAdvertised)

	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	common.Log("Mail relayed successfully: %s (to: %v)", task.Email.Subject, recipients)
	return nil
}

// getRecipients determines the recipients for relay
func (om *OutgoingMail) getRecipients(task *RelayTask) []string {
	config := task.config
	if config == nil {
		config = om.configSnapshot()
	}
	return getRecipients(task, config)
}

func getRecipients(task *RelayTask, config *OutgoingConfig) []string {
	var recipients []string

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
		recipients = filterRecipients(recipients, config)
	}

	return recipients
}

// filterRecipients applies allow/deny rules to recipients
// Rules are processed in order, and the last matching rule wins (like MailDev)
func (om *OutgoingMail) filterRecipients(recipients []string) []string {
	return filterRecipients(recipients, om.configSnapshot())
}

func filterRecipients(recipients []string, config *OutgoingConfig) []string {
	filtered := make([]string, 0)

	for _, recipient := range recipients {
		// Process all rules in order to find the last matching rule
		// Start with default: allow if no allow rules, deny if allow rules exist
		result := len(config.AllowRules) == 0

		// Process deny rules
		for _, rule := range config.DenyRules {
			if matchesRule(recipient, rule) {
				result = false // Deny if matched
			}
		}

		// Process allow rules (can override deny)
		for _, rule := range config.AllowRules {
			if matchesRule(recipient, rule) {
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
	return matchesRule(address, rule)
}

func matchesRule(address, rule string) bool {
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
func (om *OutgoingMail) RelayMail(email *types.Email, emailPath string, relayTo string, isAutoRelay bool, callback func(error)) error {
	return om.enqueueRelay(context.Background(), false, email, emailPath, relayTo, isAutoRelay, callback)
}

// RelayMailContext queues an email relay that is canceled if the caller's
// context expires while the task is queued or in progress.
func (om *OutgoingMail) RelayMailContext(ctx context.Context, email *types.Email, emailPath string, relayTo string, isAutoRelay bool, callback func(error)) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return om.enqueueRelay(ctx, true, email, emailPath, relayTo, isAutoRelay, callback)
}

func (om *OutgoingMail) enqueueRelay(ctx context.Context, contextAware bool, email *types.Email, emailPath string, relayTo string, isAutoRelay bool, callback func(error)) error {
	om.mu.Lock()
	if om.closed {
		om.mu.Unlock()
		return relayRejected(callback, ErrClosed)
	}
	if !om.enabled {
		om.mu.Unlock()
		return relayRejected(callback, ErrNotConfigured)
	}
	config := cloneConfig(om.config)
	om.enqueueWG.Add(1)
	om.mu.Unlock()
	finishEnqueue := func(err error) error {
		// Release the lifecycle slot before invoking callbacks. A callback is
		// allowed to close the relay, and Close waits for these slots to drain.
		om.enqueueWG.Done()
		if err != nil {
			return relayRejected(callback, err)
		}
		return nil
	}

	task := &RelayTask{
		Email:       email,
		EmailPath:   emailPath,
		RelayTo:     relayTo,
		IsAutoRelay: isAutoRelay,
		Callback:    callback,
		config:      config,
	}
	if contextAware {
		task.Context = ctx
		if err := ctx.Err(); err != nil {
			return finishEnqueue(err)
		}
	}

	timer := time.NewTimer(om.enqueueTimeout)
	defer timer.Stop()
	var canceled <-chan struct{}
	if contextAware {
		canceled = ctx.Done()
	}
	select {
	case om.queue <- task:
		return finishEnqueue(nil)
	case <-timer.C:
		return finishEnqueue(ErrQueueFull)
	case <-om.closing:
		return finishEnqueue(ErrClosed)
	case <-canceled:
		return finishEnqueue(ctx.Err())
	}
}

func relayRejected(callback func(error), err error) error {
	if callback != nil {
		callback(err)
	}
	return err
}

// IsAutoRelayEnabled checks if auto relay is enabled
func (om *OutgoingMail) IsAutoRelayEnabled() bool {
	om.mu.RLock()
	defer om.mu.RUnlock()
	return om.enabled && om.config.AutoRelay
}

// UpdateConfig atomically replaces the outgoing mail configuration. An empty
// host disables new submissions; already queued tasks retain the snapshot they
// were accepted with. Re-enabling starts the worker once, and an empty password
// explicitly disables authentication for subsequently accepted tasks.
func (om *OutgoingMail) UpdateConfig(config interface{}) error {
	cfg, ok := config.(*OutgoingConfig)
	if !ok {
		return fmt.Errorf("invalid outgoing mail configuration type %T", config)
	}
	next := cloneConfig(cfg)
	if next.Host != "" && (next.Port <= 0 || next.Port > 65535) {
		return fmt.Errorf("outgoing mail port must be between 1 and 65535")
	}

	om.mu.Lock()
	defer om.mu.Unlock()
	if om.closed {
		return ErrClosed
	}
	om.config = next
	om.enabled = next.Host != ""
	if om.enabled {
		om.startWorkersLocked()
	}
	return nil
}

// GetConfig returns the current configuration
func (om *OutgoingMail) GetConfig() interface{} {
	return om.configSnapshot()
}

func (om *OutgoingMail) configSnapshot() *OutgoingConfig {
	om.mu.RLock()
	defer om.mu.RUnlock()
	return cloneConfig(om.config)
}

func cloneConfig(config *OutgoingConfig) *OutgoingConfig {
	if config == nil {
		return &OutgoingConfig{}
	}
	clone := *config
	clone.AllowRules = append([]string(nil), config.AllowRules...)
	clone.DenyRules = append([]string(nil), config.DenyRules...)
	return &clone
}

// Close rejects new work, lets accepted enqueue operations finish, drains the
// stable queue, and stops the worker. It is safe to call more than once.
func (om *OutgoingMail) Close() {
	om.closeOnce.Do(func() {
		om.mu.Lock()
		om.closed = true
		om.enabled = false
		close(om.closing)
		om.mu.Unlock()

		// No new enqueue operation can increment enqueueWG after closed is set.
		// Waiting here guarantees that workers see a stable queue before draining.
		om.enqueueWG.Wait()
		close(om.stop)
		om.wg.Wait()
	})
}

type smtpHelloTracker struct {
	net.Conn

	mu       sync.Mutex
	pending  string
	tracking bool
	usedHELO bool
}

func (conn *smtpHelloTracker) Write(data []byte) (int, error) {
	conn.mu.Lock()
	if conn.tracking {
		conn.pending += string(data)
		for {
			newline := strings.IndexByte(conn.pending, '\n')
			if newline < 0 {
				break
			}
			line := strings.TrimSuffix(conn.pending[:newline], "\r")
			conn.pending = conn.pending[newline+1:]
			if strings.HasPrefix(line, "HELO ") {
				conn.usedHELO = true
			}
		}
	}
	conn.mu.Unlock()
	return conn.Conn.Write(data)
}

func (conn *smtpHelloTracker) stop() bool {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	conn.tracking = false
	conn.pending = ""
	return conn.usedHELO
}

// sendMailContext performs an SMTP transaction on a connection that is
// closed when ctx is canceled. Message data is copied through the writer from
// smtp.Client.Data so net/smtp retains responsibility for CRLF normalization
// and dot-stuffing. The source is always closed by this function.
func sendMailContext(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, source io.ReadCloser, secure, authOnlyWhenAdvertised bool) error {
	if source == nil {
		return fmt.Errorf("email source is nil")
	}
	source = &relayReadCloser{ReadCloser: source}
	defer func() {
		if err := source.Close(); err != nil {
			common.Verbose("Failed to close email source: %v", err)
		}
	}()
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	stopCancel := context.AfterFunc(ctx, func() {
		_ = source.Close()
		_ = conn.Close()
	})
	defer stopCancel()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			_ = conn.Close()
			return err
		}
	}

	smtpConn := net.Conn(conn)
	var helloTracker *smtpHelloTracker
	if auth != nil && authOnlyWhenAdvertised {
		helloTracker = &smtpHelloTracker{Conn: conn, tracking: true}
		smtpConn = helloTracker
	}
	client, err := smtp.NewClient(smtpConn, host)
	if err != nil {
		_ = conn.Close()
		return relayContextError(ctx, err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			common.Verbose("Failed to close SMTP client: %v", err)
		}
	}()

	if secure {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
				return relayContextError(ctx, err)
			}
		}
	}
	if auth != nil {
		if authOnlyWhenAdvertised {
			authAdvertised, _ := client.Extension("AUTH")
			usedHELOFallback := helloTracker.stop()
			if !authAdvertised {
				if !usedHELOFallback {
					return errors.New("smtp: server doesn't support AUTH")
				}
			} else if err := client.Auth(auth); err != nil {
				return relayContextError(ctx, err)
			}
		} else if err := client.Auth(auth); err != nil {
			return relayContextError(ctx, err)
		}
	}
	if err := client.Mail(from); err != nil {
		return relayContextError(ctx, err)
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return relayContextError(ctx, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return relayContextError(ctx, err)
	}
	if _, err := copyRelayMessage(ctx, w, source); err != nil {
		// Closing a DATA writer normally sends the terminating dot. Abort the
		// connection first so a source or write failure cannot submit a
		// truncated message, then close the writer to release its resources.
		_ = client.Close()
		if closeErr := w.Close(); closeErr != nil {
			common.Verbose("Failed to close aborted SMTP DATA writer: %v", closeErr)
		}
		return relayContextError(ctx, fmt.Errorf("stream email data: %w", err))
	}
	if err := w.Close(); err != nil {
		return relayContextError(ctx, err)
	}
	if err := client.Quit(); err != nil {
		return relayContextError(ctx, err)
	}
	return nil
}

func copyRelayMessage(ctx context.Context, destination io.Writer, source io.Reader) (int64, error) {
	return io.CopyBuffer(destination, &relayContextReader{ctx: ctx, source: source}, make([]byte, relayCopyBufferSize))
}

type relayContextReader struct {
	ctx    context.Context
	source io.Reader
}

type relayReadCloser struct {
	io.ReadCloser
	closeOnce sync.Once
	closeErr  error
}

func (reader *relayReadCloser) Close() error {
	reader.closeOnce.Do(func() {
		reader.closeErr = reader.ReadCloser.Close()
	})
	return reader.closeErr
}

func (reader *relayContextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	read, err := reader.source.Read(buffer)
	if contextErr := reader.ctx.Err(); contextErr != nil {
		return 0, contextErr
	}
	return read, err
}

func relayContextError(ctx context.Context, err error) error {
	if contextErr := ctx.Err(); contextErr != nil {
		return contextErr
	}
	return err
}


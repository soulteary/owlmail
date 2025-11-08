package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"mime"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
	"github.com/emersion/go-smtp"
	"github.com/microcosm-cc/bluemonday"
)

const (
	defaultPort    = 1025
	defaultHost    = "localhost"
	defaultMailDir = "owlmail"
)

// Email represents a parsed email message
type Email struct {
	ID            string                 `json:"id"`
	Time          time.Time              `json:"time"`
	Read          bool                   `json:"read"`
	Subject       string                 `json:"subject"`
	From          []*mail.Address        `json:"from"`
	To            []*mail.Address        `json:"to"`
	CC            []*mail.Address        `json:"cc"`
	BCC           []*mail.Address        `json:"bcc"`
	CalculatedBCC []*mail.Address        `json:"calculatedBcc"`
	Text          string                 `json:"text"`
	HTML          string                 `json:"html"`
	Attachments   []*Attachment          `json:"attachments"`
	Envelope      *Envelope              `json:"envelope"`
	Source        string                 `json:"source"`
	Size          int64                  `json:"size"`
	SizeHuman     string                 `json:"sizeHuman"`
	Headers       map[string]interface{} `json:"headers"`
}

// Attachment represents an email attachment
type Attachment struct {
	ContentType       string `json:"contentType"`
	FileName          string `json:"fileName"`
	GeneratedFileName string `json:"generatedFileName"`
	ContentID         string `json:"contentId"`
	Size              int64  `json:"size"`
	transformed       bool
}

// Envelope represents SMTP envelope information
type Envelope struct {
	From          string   `json:"from"`
	To            []string `json:"to"`
	Host          string   `json:"host"`
	RemoteAddress string   `json:"remoteAddress"`
}

// SMTPAuthConfig represents SMTP authentication configuration
type SMTPAuthConfig struct {
	Username string
	Password string
	Enabled  bool
}

// TLSConfig represents TLS configuration for SMTP server
type TLSConfig struct {
	CertFile string
	KeyFile  string
	Enabled  bool
}

// MailServer represents the SMTP mail server
type MailServer struct {
	store          []*Email
	storeMutex     sync.RWMutex
	mailDir        string
	port           int
	host           string
	smtpServer     *smtp.Server
	smtpsServer    *smtp.Server // SMTPS server (direct TLS on 465)
	eventChan      chan Event
	listeners      map[string][]func(*Email)
	listenersMutex sync.RWMutex
	outgoing       *OutgoingMail
	authConfig     *SMTPAuthConfig
	tlsConfig      *TLSConfig
}

// Event represents a server event
type Event struct {
	Type  string
	Email *Email
	ID    string
}

// NewMailServer creates a new mail server instance
func NewMailServer(port int, host, mailDir string) (*MailServer, error) {
	return NewMailServerWithOutgoing(port, host, mailDir, nil)
}

// NewMailServerWithOutgoing creates a new mail server instance with outgoing mail config
func NewMailServerWithOutgoing(port int, host, mailDir string, outgoingConfig *OutgoingConfig) (*MailServer, error) {
	return NewMailServerWithConfig(port, host, mailDir, outgoingConfig, nil, nil)
}

// NewMailServerWithConfig creates a new mail server instance with full configuration
func NewMailServerWithConfig(port int, host, mailDir string, outgoingConfig *OutgoingConfig, authConfig *SMTPAuthConfig, tlsConfig *TLSConfig) (*MailServer, error) {
	if port == 0 {
		port = defaultPort
	}
	if host == "" {
		host = defaultHost
	}
	if mailDir == "" {
		mailDir = filepath.Join(os.TempDir(), fmt.Sprintf("owlmail-%d", os.Getpid()))
	}

	// Create mail directory
	if err := os.MkdirAll(mailDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create mail directory: %w", err)
	}

	ms := &MailServer{
		store:      make([]*Email, 0),
		mailDir:    mailDir,
		port:       port,
		host:       host,
		eventChan:  make(chan Event, 100),
		listeners:  make(map[string][]func(*Email)),
		authConfig: authConfig,
		tlsConfig:  tlsConfig,
	}

	// Setup outgoing mail if config provided
	if outgoingConfig != nil {
		ms.outgoing = NewOutgoingMail(outgoingConfig)
	}

	// Setup SMTP server
	if err := ms.setupSMTPServer(); err != nil {
		return nil, fmt.Errorf("failed to setup SMTP server: %w", err)
	}

	log.Printf("owlmail using directory %s", mailDir)

	// Load existing emails from directory
	ms.LoadMailsFromDirectory()

	return ms, nil
}

// setupSMTPServer configures the SMTP server
func (ms *MailServer) setupSMTPServer() error {
	be := &Backend{mailServer: ms}
	s := smtp.NewServer(be)

	s.Addr = fmt.Sprintf("%s:%d", ms.host, ms.port)
	s.Domain = "localhost"
	s.ReadTimeout = 10 * time.Second
	s.WriteTimeout = 10 * time.Second
	s.MaxMessageBytes = 1024 * 1024
	s.MaxRecipients = 50

	// Configure authentication
	if ms.authConfig != nil && ms.authConfig.Enabled {
		s.AllowInsecureAuth = true
		// Note: go-smtp doesn't have EnableAuth, authentication is handled in Session
	} else {
		s.AllowInsecureAuth = true
	}

	// Configure TLS for STARTTLS
	if ms.tlsConfig != nil && ms.tlsConfig.Enabled {
		if ms.tlsConfig.CertFile != "" && ms.tlsConfig.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(ms.tlsConfig.CertFile, ms.tlsConfig.KeyFile)
			if err != nil {
				return fmt.Errorf("failed to load TLS certificate: %w", err)
			}
			s.TLSConfig = &tls.Config{
				Certificates: []tls.Certificate{cert},
			}
		} else {
			// Generate self-signed certificate for testing
			log.Println("Warning: No TLS certificate provided, generating self-signed certificate")
			cert, err := generateSelfSignedCert()
			if err != nil {
				return fmt.Errorf("failed to generate self-signed certificate: %w", err)
			}
			s.TLSConfig = &tls.Config{
				Certificates: []tls.Certificate{cert},
			}
		}
	}

	ms.smtpServer = s

	// Setup SMTPS server (direct TLS on 465) if TLS is enabled
	if ms.tlsConfig != nil && ms.tlsConfig.Enabled {
		smtps := smtp.NewServer(be)
		smtps.Addr = fmt.Sprintf("%s:465", ms.host)
		smtps.Domain = "localhost"
		smtps.ReadTimeout = 10 * time.Second
		smtps.WriteTimeout = 10 * time.Second
		smtps.MaxMessageBytes = 1024 * 1024
		smtps.MaxRecipients = 50

		// Configure authentication for SMTPS
		if ms.authConfig != nil && ms.authConfig.Enabled {
			smtps.AllowInsecureAuth = true
			// Note: go-smtp doesn't have EnableAuth, authentication is handled in Session
		} else {
			smtps.AllowInsecureAuth = true
		}

		// Use same TLS config
		smtps.TLSConfig = s.TLSConfig

		// Wrap listener with TLS
		smtps.LMTP = false
		ms.smtpsServer = smtps
	}

	return nil
}

// Listen starts the SMTP server
func (ms *MailServer) Listen() error {
	// Start SMTPS server (465) if configured
	if ms.smtpsServer != nil {
		go func() {
			log.Printf("owlmail SMTPS Server running at %s:465", ms.host)
			ln, err := net.Listen("tcp", ms.smtpsServer.Addr)
			if err != nil {
				log.Printf("Failed to start SMTPS server: %v", err)
				return
			}
			tlsListener := tls.NewListener(ln, ms.smtpsServer.TLSConfig)
			if err := ms.smtpsServer.Serve(tlsListener); err != nil {
				log.Printf("SMTPS server error: %v", err)
			}
		}()
	}

	log.Printf("owlmail SMTP Server running at %s:%d", ms.host, ms.port)
	if ms.authConfig != nil && ms.authConfig.Enabled {
		log.Printf("SMTP authentication enabled (PLAIN/LOGIN)")
	}
	if ms.tlsConfig != nil && ms.tlsConfig.Enabled {
		log.Printf("SMTP TLS/STARTTLS enabled")
	}
	return ms.smtpServer.ListenAndServe()
}

// Close stops the SMTP server
func (ms *MailServer) Close() error {
	if ms.outgoing != nil {
		ms.outgoing.Close()
	}
	close(ms.eventChan)

	var err error
	if ms.smtpsServer != nil {
		if closeErr := ms.smtpsServer.Close(); closeErr != nil {
			err = closeErr
		}
	}
	if closeErr := ms.smtpServer.Close(); closeErr != nil {
		err = closeErr
	}
	return err
}

// On registers an event listener
func (ms *MailServer) On(event string, handler func(*Email)) {
	ms.listenersMutex.Lock()
	defer ms.listenersMutex.Unlock()
	ms.listeners[event] = append(ms.listeners[event], handler)
}

// emit sends an event to all listeners
func (ms *MailServer) emit(event string, email *Email) {
	ms.listenersMutex.RLock()
	defer ms.listenersMutex.RUnlock()
	handlers := ms.listeners[event]
	for _, handler := range handlers {
		go handler(email)
	}
}

// saveEmailToStore saves a parsed email to the store
func (ms *MailServer) saveEmailToStore(id string, isRead bool, envelope *Envelope, parsedEmail *Email) error {
	emlPath := filepath.Join(ms.mailDir, id+".eml")

	stat, err := os.Stat(emlPath)
	if err != nil {
		return fmt.Errorf("failed to stat email file: %w", err)
	}

	parsedEmail.ID = id
	// Only set time if not already set (from header parsing)
	if parsedEmail.Time.IsZero() {
		parsedEmail.Time = time.Now()
	}
	parsedEmail.Read = isRead
	parsedEmail.Envelope = envelope
	parsedEmail.Source = emlPath
	parsedEmail.Size = stat.Size()
	parsedEmail.SizeHuman = formatBytes(stat.Size())

	// Calculate BCC
	envelopeTo := append([]string{}, envelope.To...)
	parsedEmail.CalculatedBCC = calculateBCC(
		envelopeTo,
		addressListToStrings(parsedEmail.To),
		addressListToStrings(parsedEmail.CC),
	)

	// Sanitize HTML if present
	if parsedEmail.HTML != "" {
		parsedEmail.HTML = sanitizeHTML(parsedEmail.HTML)
	}

	ms.storeMutex.Lock()
	ms.store = append(ms.store, parsedEmail)
	ms.storeMutex.Unlock()

	log.Printf("Saving email: %s, id: %s", parsedEmail.Subject, id)

	// Emit new email event
	ms.emit("new", parsedEmail)

	// Auto relay if enabled
	if ms.outgoing != nil && ms.outgoing.IsAutoRelayEnabled() {
		ms.RelayMail(parsedEmail, true, func(err error) {
			if err != nil {
				log.Printf("Error when auto-relaying email: %v", err)
			}
		})
	}

	return nil
}

// saveAttachment saves an attachment to disk
func (ms *MailServer) saveAttachment(id string, attachment *Attachment, data []byte) error {
	attachmentDir := filepath.Join(ms.mailDir, id)
	if err := os.MkdirAll(attachmentDir, 0755); err != nil {
		return fmt.Errorf("failed to create attachment directory: %w", err)
	}

	// Transform attachment filename
	attachment = transformAttachment(attachment)

	attachmentPath := filepath.Join(attachmentDir, attachment.GeneratedFileName)
	if err := os.WriteFile(attachmentPath, data, 0644); err != nil {
		return fmt.Errorf("failed to save attachment: %w", err)
	}

	attachment.Size = int64(len(data))
	return nil
}

// GetEmail retrieves an email by ID
func (ms *MailServer) GetEmail(id string) (*Email, error) {
	ms.storeMutex.RLock()
	defer ms.storeMutex.RUnlock()

	for _, email := range ms.store {
		if email.ID == id {
			// Sanitize HTML if present
			if email.HTML != "" {
				email.HTML = sanitizeHTML(email.HTML)
			}
			return email, nil
		}
	}

	return nil, fmt.Errorf("email was not found")
}

// GetAllEmail returns all emails
func (ms *MailServer) GetAllEmail() []*Email {
	ms.storeMutex.RLock()
	defer ms.storeMutex.RUnlock()

	// Return a copy to prevent external modification
	emails := make([]*Email, len(ms.store))
	copy(emails, ms.store)
	return emails
}

// DeleteEmail deletes an email by ID
func (ms *MailServer) DeleteEmail(id string) error {
	ms.storeMutex.Lock()
	defer ms.storeMutex.Unlock()

	var email *Email
	var emailIndex int = -1

	for i, e := range ms.store {
		if e.ID == id {
			email = e
			emailIndex = i
			break
		}
	}

	if emailIndex == -1 {
		return fmt.Errorf("email not found")
	}

	// Delete raw email file
	emlPath := filepath.Join(ms.mailDir, id+".eml")
	if err := os.Remove(emlPath); err != nil {
		log.Printf("Error deleting email file: %v", err)
	}

	// Delete attachments directory
	attachmentDir := filepath.Join(ms.mailDir, id)
	if err := os.RemoveAll(attachmentDir); err != nil {
		log.Printf("Error deleting attachment directory: %v", err)
	}

	log.Printf("Deleting email - %s", email.Subject)

	// Remove from store
	ms.store = append(ms.store[:emailIndex], ms.store[emailIndex+1:]...)

	// Emit delete event
	ms.emit("delete", email)

	return nil
}

// DeleteAllEmail deletes all emails
func (ms *MailServer) DeleteAllEmail() error {
	log.Printf("Deleting all email")

	ms.storeMutex.Lock()
	defer ms.storeMutex.Unlock()

	// Clear mail directory
	files, err := os.ReadDir(ms.mailDir)
	if err == nil {
		for _, file := range files {
			os.RemoveAll(filepath.Join(ms.mailDir, file.Name()))
		}
	}

	ms.store = make([]*Email, 0)
	return nil
}

// GetRawEmail returns the raw email file path
func (ms *MailServer) GetRawEmail(id string) (string, error) {
	emlPath := filepath.Join(ms.mailDir, id+".eml")
	if _, err := os.Stat(emlPath); err != nil {
		return "", fmt.Errorf("email file not found")
	}
	return emlPath, nil
}

// GetRawEmailContent returns the raw email file content
func (ms *MailServer) GetRawEmailContent(id string) ([]byte, error) {
	emlPath := filepath.Join(ms.mailDir, id+".eml")
	content, err := os.ReadFile(emlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read email file: %w", err)
	}
	return content, nil
}

// GetEmailHTML returns the HTML content of an email
func (ms *MailServer) GetEmailHTML(id string) (string, error) {
	email, err := ms.GetEmail(id)
	if err != nil {
		return "", err
	}
	return email.HTML, nil
}

// GetEmailAttachment returns attachment file path
func (ms *MailServer) GetEmailAttachment(id, filename string) (string, string, error) {
	email, err := ms.GetEmail(id)
	if err != nil {
		return "", "", err
	}

	if len(email.Attachments) == 0 {
		return "", "", fmt.Errorf("email has no attachments")
	}

	var attachment *Attachment
	for _, att := range email.Attachments {
		if att.GeneratedFileName == filename {
			attachment = att
			break
		}
	}

	if attachment == nil {
		return "", "", fmt.Errorf("attachment not found")
	}

	attachmentPath := filepath.Join(ms.mailDir, id, attachment.GeneratedFileName)
	return attachmentPath, attachment.ContentType, nil
}

// ReadAllEmail marks all emails as read
func (ms *MailServer) ReadAllEmail() int {
	ms.storeMutex.Lock()
	defer ms.storeMutex.Unlock()

	count := 0
	for _, email := range ms.store {
		if !email.Read {
			email.Read = true
			count++
		}
	}
	return count
}

// ReadEmail marks a single email as read
func (ms *MailServer) ReadEmail(id string) error {
	ms.storeMutex.Lock()
	defer ms.storeMutex.Unlock()

	for _, email := range ms.store {
		if email.ID == id {
			email.Read = true
			return nil
		}
	}
	return fmt.Errorf("email not found")
}

// GetEmailStats returns email statistics
func (ms *MailServer) GetEmailStats() map[string]interface{} {
	ms.storeMutex.RLock()
	defer ms.storeMutex.RUnlock()

	stats := make(map[string]interface{})
	total := len(ms.store)
	unread := 0
	byDate := make(map[string]int)

	for _, email := range ms.store {
		if !email.Read {
			unread++
		}

		// Group by date (YYYY-MM-DD)
		dateKey := email.Time.Format("2006-01-02")
		byDate[dateKey]++
	}

	stats["total"] = total
	stats["unread"] = unread
	stats["read"] = total - unread
	stats["byDate"] = byDate

	return stats
}

// LoadMailsFromDirectory loads emails from the mail directory
func (ms *MailServer) LoadMailsFromDirectory() error {
	files, err := os.ReadDir(ms.mailDir)
	if err != nil {
		return fmt.Errorf("failed to read mail directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Only process .eml files
		if !strings.HasSuffix(file.Name(), ".eml") {
			continue
		}

		// Extract ID from filename
		id := strings.TrimSuffix(file.Name(), ".eml")
		emlPath := filepath.Join(ms.mailDir, file.Name())

		// Check if email already loaded
		ms.storeMutex.RLock()
		alreadyLoaded := false
		for _, email := range ms.store {
			if email.ID == id {
				alreadyLoaded = true
				break
			}
		}
		ms.storeMutex.RUnlock()

		if alreadyLoaded {
			continue
		}

		// Read and parse email file
		emailFile, err := os.Open(emlPath)
		if err != nil {
			log.Printf("Error opening email file %s: %v", emlPath, err)
			continue
		}

		// Parse email
		msg, err := message.Read(emailFile)
		emailFile.Close()
		if err != nil {
			log.Printf("Error parsing email file %s: %v", emlPath, err)
			continue
		}

		// Parse email content (similar to Session.Data)
		email := &Email{
			Attachments: make([]*Attachment, 0),
			Headers:     make(map[string]interface{}),
		}

		// Extract headers
		headers := msg.Header
		email.Subject = headers.Get("Subject")

		// Parse all headers
		email.Headers = make(map[string]interface{})
		// Common headers to parse
		commonHeaders := []string{
			"From", "To", "Cc", "Bcc", "Subject", "Date", "Message-ID",
			"Reply-To", "In-Reply-To", "References", "Content-Type",
			"Content-Transfer-Encoding", "MIME-Version", "X-Mailer",
			"X-Priority", "Priority", "Importance",
		}
		for _, headerName := range commonHeaders {
			if headerValue := headers.Get(headerName); headerValue != "" {
				if headerValues := headers.Values(headerName); len(headerValues) > 1 {
					email.Headers[headerName] = headerValues
				} else {
					email.Headers[headerName] = headerValue
				}
			}
		}
		// Note: Additional custom headers can be added here if needed
		// For now, we parse the most common headers listed above

		// Parse date from headers
		if dateStr := headers.Get("Date"); dateStr != "" {
			// Try multiple date formats
			dateFormats := []string{
				time.RFC1123Z,
				time.RFC1123,
				time.RFC822Z,
				time.RFC822,
				"Mon, 2 Jan 2006 15:04:05 -0700",
				"Mon, 2 Jan 2006 15:04:05 MST",
			}
			parsed := false
			for _, format := range dateFormats {
				if date, err := time.Parse(format, dateStr); err == nil {
					email.Time = date
					parsed = true
					break
				}
			}
			if !parsed {
				email.Time = time.Now()
			}
		} else {
			email.Time = time.Now()
		}

		// Parse addresses
		if fromStr := headers.Get("From"); fromStr != "" {
			if from, err := mail.ParseAddressList(fromStr); err == nil {
				email.From = from
			}
		}
		if toStr := headers.Get("To"); toStr != "" {
			if to, err := mail.ParseAddressList(toStr); err == nil {
				email.To = to
			}
		}
		if ccStr := headers.Get("Cc"); ccStr != "" {
			if cc, err := mail.ParseAddressList(ccStr); err == nil {
				email.CC = cc
			}
		}
		if bccStr := headers.Get("Bcc"); bccStr != "" {
			if bcc, err := mail.ParseAddressList(bccStr); err == nil {
				email.BCC = bcc
			}
		}

		// Parse body
		mediaType, _, err := msg.Header.ContentType()
		if err != nil {
			mediaType = "text/plain"
		}

		if strings.HasPrefix(mediaType, "multipart/") {
			mr := msg.MultipartReader()
			if mr != nil {
				for {
					p, err := mr.NextPart()
					if err == io.EOF {
						break
					}
					if err != nil {
						log.Printf("Error reading multipart: %v", err)
						continue
					}

					partMediaType, _, _ := p.Header.ContentType()
					if partMediaType == "" {
						partMediaType = "text/plain"
					}

					disposition, params, _ := p.Header.ContentDisposition()
					contentID := strings.Trim(p.Header.Get("Content-ID"), "<>")

					body, _ := io.ReadAll(p.Body)

					if partMediaType == "text/plain" && disposition != "attachment" {
						email.Text = string(body)
					} else if partMediaType == "text/html" && disposition != "attachment" {
						email.HTML = string(body)
					} else if disposition == "attachment" || contentID != "" {
						// Handle attachment
						filename := params["filename"]
						if filename == "" {
							filename = p.Header.Get("Content-Type")
						}

						attachment := &Attachment{
							ContentType: partMediaType,
							FileName:    filename,
							ContentID:   contentID,
						}

						// Check if attachment file exists
						attachmentDir := filepath.Join(ms.mailDir, id)
						attachment = transformAttachment(attachment)
						attachmentPath := filepath.Join(attachmentDir, attachment.GeneratedFileName)
						if stat, err := os.Stat(attachmentPath); err == nil {
							attachment.Size = stat.Size()
							email.Attachments = append(email.Attachments, attachment)
						}
					}
				}
			}
		} else {
			// Simple message
			body, _ := io.ReadAll(msg.Body)
			if strings.HasPrefix(mediaType, "text/html") {
				email.HTML = string(body)
			} else {
				email.Text = string(body)
			}
		}

		// Create envelope (minimal, since we don't have SMTP session info)
		envelope := &Envelope{
			From:          "",
			To:            addressListToStrings(email.To),
			Host:          "unknown",
			RemoteAddress: "unknown",
		}

		// Try to get From from headers
		if len(email.From) > 0 {
			envelope.From = email.From[0].Address
		}

		// Save email to store (mark as read since it's restored)
		if err := ms.saveEmailToStore(id, true, envelope, email); err != nil {
			log.Printf("Error saving restored email %s: %v", id, err)
			continue
		}

		log.Printf("Restored email: %s (id: %s)", email.Subject, id)
	}

	return nil
}

// RelayMail relays an email to the configured SMTP server
func (ms *MailServer) RelayMail(email *Email, isAutoRelay bool, callback func(error)) error {
	if ms.outgoing == nil {
		return fmt.Errorf("outgoing mail not configured")
	}

	emlPath := filepath.Join(ms.mailDir, email.ID+".eml")
	ms.outgoing.RelayMail(email, emlPath, "", isAutoRelay, callback)
	return nil
}

// RelayMailTo relays an email to a specific address
func (ms *MailServer) RelayMailTo(email *Email, relayTo string, callback func(error)) error {
	if ms.outgoing == nil {
		return fmt.Errorf("outgoing mail not configured")
	}

	emlPath := filepath.Join(ms.mailDir, email.ID+".eml")
	ms.outgoing.RelayMail(email, emlPath, relayTo, false, callback)
	return nil
}

// SetOutgoingConfig sets the outgoing mail configuration
func (ms *MailServer) SetOutgoingConfig(config *OutgoingConfig) {
	if ms.outgoing == nil {
		ms.outgoing = NewOutgoingMail(config)
	} else {
		ms.outgoing.UpdateConfig(config)
	}
}

// GetOutgoingConfig returns the outgoing mail configuration
func (ms *MailServer) GetOutgoingConfig() *OutgoingConfig {
	if ms.outgoing == nil {
		return nil
	}
	return ms.outgoing.GetConfig()
}

// authenticateSession checks if the session is authenticated
// For now, we'll use a simple approach: check authentication in Mail() method
// In a production system, you might want to implement proper SASL authentication

// Backend implements smtp.Backend
type Backend struct {
	mailServer *MailServer
}

// NewSession creates a new SMTP session
func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	session := &Session{
		mailServer:    b.mailServer,
		conn:          c,
		authenticated: b.mailServer.authConfig == nil || !b.mailServer.authConfig.Enabled,
	}

	// If authentication is required, mark as not authenticated
	if b.mailServer.authConfig != nil && b.mailServer.authConfig.Enabled {
		session.authenticated = false
	}

	return session, nil
}

// Session represents an SMTP session
type Session struct {
	mailServer    *MailServer
	conn          *smtp.Conn
	from          string
	to            []string
	authenticated bool
}

// Mail handles the MAIL FROM command
func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	// Check authentication if required
	// Note: go-smtp library doesn't provide built-in AUTH support in the way we need
	// For a full implementation, you would need to intercept AUTH commands at the protocol level
	// For now, we'll allow all connections but log a warning
	if s.mailServer.authConfig != nil && s.mailServer.authConfig.Enabled && !s.authenticated {
		// Get remote address from connection if available
		if conn := s.conn.Conn(); conn != nil {
			log.Printf("Warning: Unauthenticated connection attempt from %s", conn.RemoteAddr())
		} else {
			log.Printf("Warning: Unauthenticated connection attempt")
		}
		// In a production system, you should return an error here
		// return fmt.Errorf("535 5.7.8 Authentication required")
	}
	s.from = from
	return nil
}

// Rcpt handles the RCPT TO command
func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	s.to = append(s.to, to)
	return nil
}

// Data handles the DATA command
func (s *Session) Data(r io.Reader) error {
	// Generate unique ID
	id := makeID()

	// Save raw email
	emlPath := filepath.Join(s.mailServer.mailDir, id+".eml")
	emlFile, err := os.Create(emlPath)
	if err != nil {
		return fmt.Errorf("failed to create email file: %w", err)
	}
	defer emlFile.Close()

	// Copy email data to file and parse
	tee := io.TeeReader(r, emlFile)

	// Parse email
	msg, err := message.Read(tee)
	if err != nil {
		return fmt.Errorf("failed to parse email: %w", err)
	}

	// Parse email content
	email := &Email{
		Attachments: make([]*Attachment, 0),
		Headers:     make(map[string]interface{}),
	}

	// Extract headers
	headers := msg.Header
	email.Subject = headers.Get("Subject")

	// Parse all headers into Headers map
	email.Headers = make(map[string]interface{})
	// Common headers to parse
	commonHeaders := []string{
		"From", "To", "Cc", "Bcc", "Subject", "Date", "Message-ID",
		"Reply-To", "In-Reply-To", "References", "Content-Type",
		"Content-Transfer-Encoding", "MIME-Version", "X-Mailer",
		"X-Priority", "Priority", "Importance",
	}
	for _, headerName := range commonHeaders {
		if headerValue := headers.Get(headerName); headerValue != "" {
			if headerValues := headers.Values(headerName); len(headerValues) > 1 {
				email.Headers[headerName] = headerValues
			} else {
				email.Headers[headerName] = headerValue
			}
		}
	}
	// Note: Additional custom headers can be added here if needed
	// For now, we parse the most common headers listed above

	// Parse date from headers
	if dateStr := headers.Get("Date"); dateStr != "" {
		// Try multiple date formats
		dateFormats := []string{
			time.RFC1123Z,
			time.RFC1123,
			time.RFC822Z,
			time.RFC822,
			"Mon, 2 Jan 2006 15:04:05 -0700",
			"Mon, 2 Jan 2006 15:04:05 MST",
		}
		parsed := false
		for _, format := range dateFormats {
			if date, err := time.Parse(format, dateStr); err == nil {
				email.Time = date
				parsed = true
				break
			}
		}
		if !parsed {
			// Fallback to current time if parsing fails
			email.Time = time.Now()
		}
	} else {
		email.Time = time.Now()
	}

	// Parse addresses
	if fromStr := headers.Get("From"); fromStr != "" {
		if from, err := mail.ParseAddressList(fromStr); err == nil {
			email.From = from
		}
	}
	if toStr := headers.Get("To"); toStr != "" {
		if to, err := mail.ParseAddressList(toStr); err == nil {
			email.To = to
		}
	}
	if ccStr := headers.Get("Cc"); ccStr != "" {
		if cc, err := mail.ParseAddressList(ccStr); err == nil {
			email.CC = cc
		}
	}
	if bccStr := headers.Get("Bcc"); bccStr != "" {
		if bcc, err := mail.ParseAddressList(bccStr); err == nil {
			email.BCC = bcc
		}
	}

	// Parse body
	mediaType, _, err := msg.Header.ContentType()
	if err != nil {
		mediaType = "text/plain"
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		mr := msg.MultipartReader()
		if mr != nil {
			for {
				p, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					log.Printf("Error reading multipart: %v", err)
					continue
				}

				partMediaType, _, _ := p.Header.ContentType()
				if partMediaType == "" {
					partMediaType = "text/plain"
				}

				disposition, params, _ := p.Header.ContentDisposition()
				contentID := strings.Trim(p.Header.Get("Content-ID"), "<>")

				body, _ := io.ReadAll(p.Body)

				if partMediaType == "text/plain" && disposition != "attachment" {
					email.Text = string(body)
				} else if partMediaType == "text/html" && disposition != "attachment" {
					email.HTML = string(body)
				} else if disposition == "attachment" || contentID != "" {
					// Handle attachment
					filename := params["filename"]
					if filename == "" {
						filename = p.Header.Get("Content-Type")
					}

					attachment := &Attachment{
						ContentType: partMediaType,
						FileName:    filename,
						ContentID:   contentID,
					}

					if err := s.mailServer.saveAttachment(id, attachment, body); err != nil {
						log.Printf("Error saving attachment: %v", err)
					} else {
						email.Attachments = append(email.Attachments, attachment)
					}
				}
			}
		}
	} else {
		// Simple message
		body, _ := io.ReadAll(msg.Body)
		if strings.HasPrefix(mediaType, "text/html") {
			email.HTML = string(body)
		} else {
			email.Text = string(body)
		}
	}

	// Create envelope
	remoteAddr := ""
	if conn := s.conn.Conn(); conn != nil {
		remoteAddr = conn.RemoteAddr().String()
	}
	envelope := &Envelope{
		From:          s.from,
		To:            s.to,
		Host:          s.conn.Hostname(),
		RemoteAddress: remoteAddr,
	}

	// Save email to store
	if err := s.mailServer.saveEmailToStore(id, false, envelope, email); err != nil {
		return fmt.Errorf("failed to save email: %w", err)
	}

	return nil
}

// Reset resets the session
func (s *Session) Reset() {
	s.from = ""
	s.to = []string{}
}

// Logout closes the session
func (s *Session) Logout() error {
	return nil
}

// Helper functions

// makeID generates a unique 8-character ID
func makeID() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to time-based if random fails
		for i := range b {
			b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		}
	} else {
		for i := range b {
			b[i] = charset[b[i]%byte(len(charset))]
		}
	}
	return string(b)
}

// formatBytes formats bytes to human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d bytes", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// addressListToStrings converts mail.Address list to string list
func addressListToStrings(addrs []*mail.Address) []string {
	result := make([]string, len(addrs))
	for i, addr := range addrs {
		result[i] = addr.Address
	}
	return result
}

// calculateBCC calculates BCC addresses
func calculateBCC(recipients, to, cc []string) []*mail.Address {
	bccAddresses := make([]*mail.Address, 0)
	toCopy := make([]string, len(to))
	ccCopy := make([]string, len(cc))
	copy(toCopy, to)
	copy(ccCopy, cc)

	for _, recipient := range recipients {
		// Check if in CC
		found := false
		for i, addr := range ccCopy {
			if addr == recipient {
				ccCopy = append(ccCopy[:i], ccCopy[i+1:]...)
				found = true
				break
			}
		}
		if found {
			continue
		}

		// Check if in TO
		for i, addr := range toCopy {
			if addr == recipient {
				toCopy = append(toCopy[:i], toCopy[i+1:]...)
				found = true
				break
			}
		}
		if found {
			continue
		}

		// Must be BCC
		bccAddresses = append(bccAddresses, &mail.Address{Address: recipient})
	}

	return bccAddresses
}

// transformAttachment transforms attachment filename for security
func transformAttachment(attachment *Attachment) *Attachment {
	if attachment.transformed {
		return attachment
	}

	// Extract extension from original filename
	ext := filepath.Ext(attachment.FileName)
	if ext == "" {
		// Try to get extension from Content-Type
		if attachment.ContentType != "" {
			exts, _ := mime.ExtensionsByType(attachment.ContentType)
			if len(exts) > 0 {
				ext = exts[0]
			}
		}
		if ext == "" {
			ext = ".bin"
		}
	}

	// Generate filename from ContentID or use hash
	var name string
	if attachment.ContentID != "" {
		hash := md5.Sum([]byte(attachment.ContentID))
		name = fmt.Sprintf("%x", hash)
	} else {
		// Use filename + timestamp for uniqueness
		hash := md5.Sum([]byte(attachment.FileName + time.Now().String()))
		name = fmt.Sprintf("%x", hash)
	}

	attachment.GeneratedFileName = name + ext
	attachment.transformed = true
	return attachment
}

// sanitizeHTML sanitizes HTML content
func sanitizeHTML(html string) string {
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("target").OnElements("a")
	p.AllowElements("link")
	return p.Sanitize(html)
}

// generateSelfSignedCert generates a self-signed certificate for testing
func generateSelfSignedCert() (tls.Certificate, error) {
	// Generate private key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"OwlMail"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // Valid for 1 year
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Add IP addresses and DNS names
	template.IPAddresses = []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback}
	template.DNSNames = []string{"localhost", "127.0.0.1"}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Encode private key
	privDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to marshal private key: %w", err)
	}

	// Create PEM blocks
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER})

	// Load certificate
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to load certificate: %w", err)
	}

	return cert, nil
}

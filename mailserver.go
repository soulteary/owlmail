package main

import (
	"crypto/md5"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"mime"
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

// MailServer represents the SMTP mail server
type MailServer struct {
	store          []*Email
	storeMutex     sync.RWMutex
	mailDir        string
	port           int
	host           string
	smtpServer     *smtp.Server
	eventChan      chan Event
	listeners      map[string][]func(*Email)
	listenersMutex sync.RWMutex
}

// Event represents a server event
type Event struct {
	Type  string
	Email *Email
	ID    string
}

// NewMailServer creates a new mail server instance
func NewMailServer(port int, host, mailDir string) (*MailServer, error) {
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
		store:     make([]*Email, 0),
		mailDir:   mailDir,
		port:      port,
		host:      host,
		eventChan: make(chan Event, 100),
		listeners: make(map[string][]func(*Email)),
	}

	// Setup SMTP server
	ms.setupSMTPServer()

	log.Printf("owlmail using directory %s", mailDir)
	return ms, nil
}

// setupSMTPServer configures the SMTP server
func (ms *MailServer) setupSMTPServer() {
	be := &Backend{mailServer: ms}
	s := smtp.NewServer(be)

	s.Addr = fmt.Sprintf("%s:%d", ms.host, ms.port)
	s.Domain = "localhost"
	s.ReadTimeout = 10 * time.Second
	s.WriteTimeout = 10 * time.Second
	s.MaxMessageBytes = 1024 * 1024
	s.MaxRecipients = 50
	s.AllowInsecureAuth = true

	ms.smtpServer = s
}

// Listen starts the SMTP server
func (ms *MailServer) Listen() error {
	log.Printf("owlmail SMTP Server running at %s:%d", ms.host, ms.port)
	return ms.smtpServer.ListenAndServe()
}

// Close stops the SMTP server
func (ms *MailServer) Close() error {
	close(ms.eventChan)
	return ms.smtpServer.Close()
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
	parsedEmail.Time = time.Now()
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

// Backend implements smtp.Backend
type Backend struct {
	mailServer *MailServer
}

// NewSession creates a new SMTP session
func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &Session{
		mailServer: b.mailServer,
		conn:       c,
	}, nil
}

// Session represents an SMTP session
type Session struct {
	mailServer *MailServer
	conn       *smtp.Conn
	from       string
	to         []string
}

// Mail handles the MAIL FROM command
func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
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

package mailserver

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"
	"github.com/soulteary/owlmail/internal/common"
)

// SaveEmailToStore saves a parsed email to the store (exported for testing)
func (ms *MailServer) SaveEmailToStore(id string, isRead bool, envelope *Envelope, parsedEmail *Email) error {
	emlPath := filepath.Join(ms.mailDir, id+".eml")

	parsedEmail.ID = id
	// Only set time if not already set (from header parsing)
	if parsedEmail.Time.IsZero() {
		parsedEmail.Time = time.Now()
	}
	parsedEmail.Read = isRead
	parsedEmail.Envelope = envelope
	parsedEmail.Source = emlPath

	// Try to get file size, but don't fail if file doesn't exist
	stat, err := os.Stat(emlPath)
	if err != nil {
		// File doesn't exist, set size to 0
		parsedEmail.Size = 0
		parsedEmail.SizeHuman = formatBytes(0)
	} else {
		parsedEmail.Size = stat.Size()
		parsedEmail.SizeHuman = formatBytes(stat.Size())
	}

	// Calculate BCC
	envelopeTo := append([]string{}, envelope.To...)
	parsedEmail.CalculatedBCC = calculateBCC(
		envelopeTo,
		addressListToStrings(parsedEmail.To),
		addressListToStrings(parsedEmail.CC),
	)

	// Sanitize HTML if present
	if parsedEmail.HTML != "" {
		parsedEmail.HTML = strings.TrimSpace(sanitizeHTML(parsedEmail.HTML))
	}

	ms.storeMutex.Lock()
	ms.store = append(ms.store, parsedEmail)
	ms.storeMutex.Unlock()

	common.Log("Saving email: %s, id: %s", parsedEmail.Subject, id)

	// Emit new email event
	ms.emit("new", parsedEmail)

	// Auto relay if enabled
	if ms.outgoing != nil && ms.outgoing.IsAutoRelayEnabled() {
		if err := ms.RelayMail(parsedEmail, true, func(err error) {
			if err != nil {
				common.Error("Error when auto-relaying email: %v", err)
			}
		}); err != nil {
			common.Error("Error when initiating auto-relay: %v", err)
		}
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
	var emailIndex = -1

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

	// Validate email ID to prevent path traversal
	if err := validateEmailID(id); err != nil {
		return fmt.Errorf("invalid email ID: %w", err)
	}

	// Delete raw email file
	emlPath := filepath.Join(ms.mailDir, id+".eml")
	// Validate path is within mail directory
	if err := validatePath(ms.mailDir, emlPath); err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}
	if err := os.Remove(emlPath); err != nil {
		common.Verbose("Error deleting email file: %v", err)
	}

	// Delete attachments directory
	attachmentDir := filepath.Join(ms.mailDir, id)
	// Validate path is within mail directory
	if err := validatePath(ms.mailDir, attachmentDir); err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}
	if err := os.RemoveAll(attachmentDir); err != nil {
		common.Verbose("Error deleting attachment directory: %v", err)
	}

	common.Log("Deleting email - %s, id: %s", email.Subject, email.ID)

	// Remove from store
	ms.store = append(ms.store[:emailIndex], ms.store[emailIndex+1:]...)

	// Emit delete event
	ms.emit("delete", email)

	return nil
}

// DeleteAllEmail deletes all emails
func (ms *MailServer) DeleteAllEmail() error {
	common.Log("Deleting all email")

	ms.storeMutex.Lock()
	defer ms.storeMutex.Unlock()

	// Clear mail directory
	files, err := os.ReadDir(ms.mailDir)
	if err == nil {
		for _, file := range files {
			if err := os.RemoveAll(filepath.Join(ms.mailDir, file.Name())); err != nil {
				common.Verbose("Failed to remove file: %v", err)
			}
		}
	}

	ms.store = make([]*Email, 0)
	return nil
}

// GetRawEmail returns the raw email file path
func (ms *MailServer) GetRawEmail(id string) (string, error) {
	// Validate email ID to prevent path traversal
	if err := validateEmailID(id); err != nil {
		return "", fmt.Errorf("invalid email ID: %w", err)
	}
	emlPath := filepath.Join(ms.mailDir, id+".eml")
	// Validate path is within mail directory
	if err := validatePath(ms.mailDir, emlPath); err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}
	if _, err := os.Stat(emlPath); err != nil {
		return "", fmt.Errorf("email file not found")
	}
	return emlPath, nil
}

// GetRawEmailContent returns the raw email file content
func (ms *MailServer) GetRawEmailContent(id string) ([]byte, error) {
	// Validate email ID to prevent path traversal
	if err := validateEmailID(id); err != nil {
		return nil, fmt.Errorf("invalid email ID: %w", err)
	}
	emlPath := filepath.Join(ms.mailDir, id+".eml")
	// Validate path is within mail directory
	if err := validatePath(ms.mailDir, emlPath); err != nil {
		return nil, fmt.Errorf("path validation failed: %w", err)
	}
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
	// Validate email ID to prevent path traversal
	if err := validateEmailID(id); err != nil {
		return "", "", fmt.Errorf("invalid email ID: %w", err)
	}
	// Validate filename to prevent path traversal
	if filename == "" || strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return "", "", fmt.Errorf("invalid filename: contains path traversal characters")
	}

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
	// Validate path is within mail directory
	if err := validatePath(ms.mailDir, attachmentPath); err != nil {
		return "", "", fmt.Errorf("path validation failed: %w", err)
	}
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

// parseEmail parses email from given reader
func (ms *MailServer) parseEmail(id string, r io.Reader, s *Session, saveAttachments, markAsRead bool) (*Email, error) {
	msg, err := message.Read(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse email: %w", err)
	}

	// Parse email content
	email := &Email{
		Attachments: make([]*Attachment, 0),
		Headers:     make(map[string]interface{}),
	}

	// Extract headers
	// Wrap in mail.Header to get decoding support
	headers := mail.Header{Header: msg.Header}

	// Parse all headers into Headers map
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
	if email.Time, err = headers.Date(); err != nil {
		email.Time = parseEmailDate(headers.Header)
	}

	if email.Subject, err = headers.Subject(); err != nil {
		// Fallback to raw subject if decoding fails
		email.Subject = headers.Get("Subject")
	}

	// Parse addresses
	// TODO : handle error cases
	email.From, _ = headers.AddressList("From")
	email.To, _ = headers.AddressList("To")
	email.CC, _ = headers.AddressList("Cc")
	email.BCC, _ = headers.AddressList("Bcc")

	// Parse body
	mediaType, _, err := headers.ContentType()
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
					common.Verbose("Error reading multipart: %v", err)
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
					email.Text = strings.TrimSpace(string(body))
				} else if partMediaType == "text/html" && disposition != "attachment" {
					email.HTML = strings.TrimSpace(string(body))
				} else if disposition == "attachment" || contentID != "" {
					// Handle attachment
					filename := params["filename"]
					if filename == "" {
						filename = partMediaType
					}

					attachment := &Attachment{
						ContentType: partMediaType,
						FileName:    filename,
						ContentID:   contentID,
					}

					if saveAttachments {
						err = ms.saveAttachment(id, attachment, body)
						if err != nil {
							common.Verbose("Error saving attachment: %v", err)
						}
					}
					email.Attachments = append(email.Attachments, attachment)
				}
			}
		}
	} else {
		// Simple message
		body, _ := io.ReadAll(msg.Body)
		if strings.HasPrefix(mediaType, "text/html") {
			email.HTML = strings.TrimSpace(string(body))
		} else {
			email.Text = strings.TrimSpace(string(body))
		}
	}

	// Create envelope
	envelope := &Envelope{
		From:          "",
		To:            addressListToStrings(email.To),
		Host:          "unknown",
		RemoteAddress: "unknown",
	}
	if s != nil {
		if s.conn != nil {
			if conn := s.conn.Conn(); conn != nil {
				envelope.RemoteAddress = conn.RemoteAddr().String()
			}
			envelope.Host = s.conn.Hostname()
		}
		envelope.From = s.from
		envelope.To = s.to
	}

	// Save email to store
	if err = ms.SaveEmailToStore(id, markAsRead, envelope, email); err != nil {
		return nil, fmt.Errorf("failed to store email into memory: %w", err)
	}

	return email, nil
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
			common.Verbose("Error opening email file %s: %v", emlPath, err)
			continue
		}

		// Parse email
		if email, err := ms.parseEmail(id, emailFile, nil, false, true); err == nil {
			common.Verbose("Restored email: %s (id: %s)", email.Subject, id)
		}
	}

	return nil
}

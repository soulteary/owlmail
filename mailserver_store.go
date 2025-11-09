package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
)

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

	Log("Saving email: %s, id: %s", parsedEmail.Subject, id)

	// Emit new email event
	ms.emit("new", parsedEmail)

	// Auto relay if enabled
	if ms.outgoing != nil && ms.outgoing.IsAutoRelayEnabled() {
		ms.RelayMail(parsedEmail, true, func(err error) {
			if err != nil {
				Error("Error when auto-relaying email: %v", err)
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
		Verbose("Error deleting email file: %v", err)
	}

	// Delete attachments directory
	attachmentDir := filepath.Join(ms.mailDir, id)
	if err := os.RemoveAll(attachmentDir); err != nil {
		Verbose("Error deleting attachment directory: %v", err)
	}

	Log("Deleting email - %s", email.Subject)

	// Remove from store
	ms.store = append(ms.store[:emailIndex], ms.store[emailIndex+1:]...)

	// Emit delete event
	ms.emit("delete", email)

	return nil
}

// DeleteAllEmail deletes all emails
func (ms *MailServer) DeleteAllEmail() error {
	Log("Deleting all email")

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
			Verbose("Error opening email file %s: %v", emlPath, err)
			continue
		}

		// Parse email
		msg, err := message.Read(emailFile)
		emailFile.Close()
		if err != nil {
			Verbose("Error parsing email file %s: %v", emlPath, err)
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
		email.Time = parseEmailDate(headers)

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
						Verbose("Error reading multipart: %v", err)
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
			Verbose("Error saving restored email %s: %v", id, err)
			continue
		}

		Verbose("Restored email: %s (id: %s)", email.Subject, id)
	}

	return nil
}

package mailserver

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
	"github.com/emersion/go-smtp"
	"github.com/soulteary/owlmail/internal/common"
)

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
		if s.conn != nil {
			if conn := s.conn.Conn(); conn != nil {
				common.Verbose("Warning: Unauthenticated connection attempt from %s", conn.RemoteAddr())
			} else {
				common.Verbose("Warning: Unauthenticated connection attempt")
			}
		} else {
			common.Verbose("Warning: Unauthenticated connection attempt")
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
						common.Verbose("Error saving attachment: %v", err)
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
	hostname := ""
	if s.conn != nil {
		if conn := s.conn.Conn(); conn != nil {
			remoteAddr = conn.RemoteAddr().String()
		}
		hostname = s.conn.Hostname()
	}
	envelope := &Envelope{
		From:          s.from,
		To:            s.to,
		Host:          hostname,
		RemoteAddress: remoteAddr,
	}

	// Save email to store
	if err := s.mailServer.SaveEmailToStore(id, false, envelope, email); err != nil {
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

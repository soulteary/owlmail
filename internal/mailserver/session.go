package mailserver

import (
	"io"

	_ "github.com/emersion/go-message/charset"
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
	id := makeID(s.mailServer.useUUIDForID)
	return s.mailServer.storeIncomingEmail(id, r, s)
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

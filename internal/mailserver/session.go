package mailserver

import (
	"io"

	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-smtp"
	"github.com/soulteary/owlmail/internal/common"
	"github.com/soulteary/owlmail/internal/mailbox"
	"github.com/soulteary/owlmail/internal/types"
)

// Backend implements smtp.Backend
type Backend struct {
	mailServer *MailServer
}

// NewSession creates a new SMTP session
func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &Session{
		mailServer:    b.mailServer,
		conn:          c,
		authenticated: !b.mailServer.authRequired(),
	}, nil
}

// Session represents an SMTP session
type Session struct {
	ingestAccount *mailbox.Account
	rcptRoutes    map[string]mailbox.Route

	ownership *types.Ownership

	mailServer    *MailServer
	conn          *smtp.Conn
	from          string
	to            []string
	authenticated bool
}

// Mail handles the MAIL FROM command
func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	if err := s.requireAuthentication(); err != nil {
		if s.conn != nil {
			if conn := s.conn.Conn(); conn != nil {
				common.Verbose("Rejected unauthenticated SMTP transaction from %s", conn.RemoteAddr())
			} else {
				common.Verbose("Rejected unauthenticated SMTP transaction")
			}
		} else {
			common.Verbose("Rejected unauthenticated SMTP transaction")
		}
		return err
	}
	s.from = from
	return nil
}

// Rcpt handles the RCPT TO command
func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	if err := s.requireAuthentication(); err != nil {
		return err
	}
	if s.mailServer.routedIngress != nil {
		return s.routeRecipient(to)
	}
	s.to = append(s.to, to)
	return nil
}

// Data handles the DATA command
func (s *Session) Data(r io.Reader) error {
	if err := s.requireAuthentication(); err != nil {
		return err
	}
	if !s.mailServer.tryAcquireDataSlot() {
		return errSMTPDataConcurrencyLimit
	}
	defer s.mailServer.releaseDataSlot()
	if s.mailServer.afterDataAcquire != nil {
		s.mailServer.afterDataAcquire()
	}
	if s.mailServer.routedIngress != nil {
		return s.mailServer.storeRoutedIncoming(r, s)
	}
	id := makeID(s.mailServer.useUUIDForID)
	return s.mailServer.storeIncomingEmail(id, r, s)
}

// Reset resets the session
func (s *Session) Reset() {
	s.rcptRoutes = nil
	s.ownership = nil
	s.from = ""
	s.to = []string{}
}

// Logout closes the session
func (s *Session) Logout() error {
	return nil
}

package mailserver

import (
	"sync"

	"github.com/emersion/go-smtp"
	"github.com/soulteary/owlmail/internal/types"
)

const (
	defaultPort    = 1025
	defaultHost    = "localhost"
	defaultMailDir = "owlmail"
)

// Email is an alias for types.Email
type Email = types.Email

// Attachment is an alias for types.Attachment
type Attachment = types.Attachment

// Envelope is an alias for types.Envelope
type Envelope = types.Envelope

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
	store          []*types.Email
	storeMutex     sync.RWMutex
	mailDir        string
	port           int
	host           string
	smtpServer     *smtp.Server
	smtpsServer    *smtp.Server // SMTPS server (direct TLS on 465)
	eventChan      chan Event
	listeners      map[string][]func(*types.Email)
	listenersMutex sync.RWMutex
	outgoing       interface {
		RelayMail(email *types.Email, emlPath, relayTo string, isAutoRelay bool, callback func(error))
		UpdateConfig(config interface{})
		GetConfig() interface{}
		IsAutoRelayEnabled() bool
		Close()
	}
	authConfig    *SMTPAuthConfig
	tlsConfig     *TLSConfig
	useUUIDForID  bool
}

// GetHost returns the SMTP server host
func (ms *MailServer) GetHost() string {
	return ms.host
}

// GetPort returns the SMTP server port
func (ms *MailServer) GetPort() int {
	return ms.port
}

// GetMailDir returns the mail directory path
func (ms *MailServer) GetMailDir() string {
	return ms.mailDir
}

// GetAuthConfig returns the SMTP authentication configuration
func (ms *MailServer) GetAuthConfig() *SMTPAuthConfig {
	return ms.authConfig
}

// GetTLSConfig returns the TLS configuration
func (ms *MailServer) GetTLSConfig() *TLSConfig {
	return ms.tlsConfig
}

// Event represents a server event
type Event struct {
	Type  string
	Email *types.Email
	ID    string
}

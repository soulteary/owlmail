package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/emersion/go-smtp"
)

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

	Log("owlmail using directory %s", mailDir)

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
			Log("Warning: No TLS certificate provided, generating self-signed certificate")
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

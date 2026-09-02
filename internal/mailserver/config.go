package mailserver

import (
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/soulteary/owlmail/internal/common"
	"github.com/soulteary/owlmail/internal/outgoing"
	"github.com/soulteary/owlmail/internal/types"
)

// NewMailServer creates a new mail server instance
func NewMailServer(port int, host, mailDir string) (*MailServer, error) {
	return NewMailServerWithOutgoing(port, host, mailDir, nil)
}

// NewMailServerWithOutgoing creates a new mail server instance with outgoing mail config
func NewMailServerWithOutgoing(port int, host, mailDir string, outgoingConfig *outgoing.OutgoingConfig) (*MailServer, error) {
	return NewMailServerWithConfig(port, host, mailDir, outgoingConfig, nil, nil)
}

// NewMailServerWithConfig creates a new mail server instance with full configuration
func NewMailServerWithConfig(port int, host, mailDir string, outgoingConfig *outgoing.OutgoingConfig, authConfig *SMTPAuthConfig, tlsConfig *TLSConfig) (*MailServer, error) {
	return NewMailServerWithFullConfig(port, host, mailDir, outgoingConfig, authConfig, tlsConfig, false)
}

// NewMailServerWithFullConfig creates a new mail server instance with full configuration including UUID option
func NewMailServerWithFullConfig(port int, host, mailDir string, outgoingConfig *outgoing.OutgoingConfig, authConfig *SMTPAuthConfig, tlsConfig *TLSConfig, useUUIDForID bool) (*MailServer, error) {
	return NewMailServerWithOptions(port, host, mailDir, ServerOptions{
		OutgoingConfig:  outgoingConfig,
		AuthConfig:      authConfig,
		TLSConfig:       tlsConfig,
		UseUUIDForID:    useUUIDForID,
		MaxMessageBytes: DefaultMaxMessageBytes,
	})
}

// NewMailServerWithOptions creates a mail server with optional external
// attachment storage and a configurable SMTP message-size limit.
func NewMailServerWithOptions(port int, host, mailDir string, options ServerOptions) (*MailServer, error) {
	if options.MaxDataConcurrency < 0 {
		return nil, fmt.Errorf("SMTP DATA max concurrency must be zero or greater")
	}
	if options.ReadTimeout < 0 || options.WriteTimeout < 0 {
		return nil, fmt.Errorf("SMTP timeouts must be zero or greater")
	}
	if options.MaxRecipients < 0 {
		return nil, fmt.Errorf("SMTP max recipients must be zero or greater")
	}
	if options.AuthRequireTLS && (options.TLSConfig == nil || !options.TLSConfig.Enabled) {
		return nil, fmt.Errorf("SMTP AUTH cannot require TLS without an enabled TLS configuration")
	}
	if options.AuthConfig != nil && options.AuthConfig.Enabled && (options.AuthConfig.Username == "" || options.AuthConfig.Password == "") {
		return nil, fmt.Errorf("SMTP username and password are both required when authentication is enabled")
	}
	var authVerifier *credentialVerifier
	if options.AuthConfig != nil && options.AuthConfig.Enabled {
		var err error
		authVerifier, err = newCredentialVerifier(options.AuthConfig.Username, options.AuthConfig.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize SMTP credential verifier: %w", err)
		}
	}
	if port == 0 {
		port = defaultPort
	}
	if host == "" {
		host = defaultHost
	}
	if mailDir == "" {
		mailDir = filepath.Join(os.TempDir(), fmt.Sprintf("owlmail-%d", os.Getpid()))
	}
	maxMessageBytes := options.MaxMessageBytes
	if maxMessageBytes <= 0 {
		maxMessageBytes = DefaultMaxMessageBytes
	}
	readTimeout := options.ReadTimeout
	if readTimeout == 0 {
		readTimeout = defaultSMTPReadTimeout
	}
	writeTimeout := options.WriteTimeout
	if writeTimeout == 0 {
		writeTimeout = defaultSMTPWriteTimeout
	}
	maxRecipients := options.MaxRecipients
	if maxRecipients == 0 {
		maxRecipients = defaultSMTPMaxRecipients
	}

	// Create mail directory
	if err := os.MkdirAll(mailDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create mail directory: %w", err)
	}

	ms := &MailServer{
		storeByID:               make(map[string]*types.Email),
		storeOrder:              make([]string, 0),
		receivedAtByID:          make(map[string]time.Time),
		mailDir:                 mailDir,
		port:                    port,
		host:                    host,
		maxMessageBytes:         maxMessageBytes,
		maxDataConcurrency:      options.MaxDataConcurrency,
		readTimeout:             readTimeout,
		writeTimeout:            writeTimeout,
		maxRecipients:           maxRecipients,
		dataLimiter:             newDataLimiter(options.MaxDataConcurrency),
		attachmentStore:         options.AttachmentStore,
		attachmentHealth:        options.AttachmentHealth,
		mailboxIndex:            options.MailboxIndex,
		attachmentUploadTimeout: defaultAttachmentUploadTimeout,
		attachmentOpenTimeout:   defaultAttachmentOpenTimeout,
		attachmentDeleteTimeout: defaultAttachmentDeleteTimeout,
		eventChan:               make(chan Event, 100),
		listeners:               make(map[string][]eventListener),
		authConfig:              options.AuthConfig,
		authVerifier:            authVerifier,
		authRequireTLS:          options.AuthRequireTLS,
		tlsConfig:               options.TLSConfig,
		useUUIDForID:            options.UseUUIDForID,
	}
	ms.retainAllHeaders.Store(options.RetainAllHeaders)

	// Setup outgoing mail if config provided
	if options.OutgoingConfig != nil {
		ms.outgoing = outgoing.NewOutgoingMail(options.OutgoingConfig)
	}

	// Setup SMTP server
	if err := ms.setupSMTPServer(); err != nil {
		if ms.mailboxIndex != nil {
			_ = ms.mailboxIndex.Close()
		}
		return nil, fmt.Errorf("failed to setup SMTP server: %w", err)
	}

	common.Log("owlmail using directory %s", mailDir)

	// Load existing emails from directory
	if err := ms.LoadMailsFromDirectory(); err != nil {
		common.Error("Failed to load emails from directory: %v", err)
		// Continue anyway, as this is not a fatal error
	}
	if ms.mailboxIndex != nil {
		if err := ms.rebuildMailboxIndex(); err != nil {
			_ = ms.mailboxIndex.Close()
			return nil, fmt.Errorf("rebuild mailbox index: %w", err)
		}
		if err := ms.AddCloser(ms.mailboxIndex); err != nil {
			_ = ms.mailboxIndex.Close()
			return nil, fmt.Errorf("register mailbox index closer: %w", err)
		}
	}

	return ms, nil
}

// setupSMTPServer configures the SMTP server
func (ms *MailServer) setupSMTPServer() error {
	be := &Backend{mailServer: ms}
	s := smtp.NewServer(be)

	s.Addr = fmt.Sprintf("%s:%d", ms.host, ms.port)
	s.Domain = "localhost"
	s.ReadTimeout = ms.readTimeout
	s.WriteTimeout = ms.writeTimeout
	s.MaxMessageBytes = ms.maxMessageBytes
	s.MaxRecipients = ms.maxRecipients
	s.EnableSMTPUTF8 = true

	// Preserve the development-friendly default while allowing deployments to
	// require an encrypted transport before PLAIN or LOGIN is accepted.
	s.AllowInsecureAuth = !ms.authRequireTLS

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
			common.Log("Warning: No TLS certificate provided, generating self-signed certificate")
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
		smtps.ReadTimeout = ms.readTimeout
		smtps.WriteTimeout = ms.writeTimeout
		smtps.MaxMessageBytes = ms.maxMessageBytes
		smtps.MaxRecipients = ms.maxRecipients
		smtps.EnableSMTPUTF8 = true

		smtps.AllowInsecureAuth = !ms.authRequireTLS

		// Use same TLS config
		smtps.TLSConfig = s.TLSConfig

		// Wrap listener with TLS
		smtps.LMTP = false
		ms.smtpsServer = smtps
	}

	return nil
}

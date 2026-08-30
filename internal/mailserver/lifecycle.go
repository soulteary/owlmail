package mailserver

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/soulteary/owlmail/internal/common"
)

// AddCloser registers a component whose lifecycle is owned by the mail server.
func (ms *MailServer) AddCloser(closer io.Closer) error {
	if closer == nil {
		return fmt.Errorf("closer cannot be nil")
	}
	ms.closersMutex.Lock()
	defer ms.closersMutex.Unlock()
	ms.closers = append(ms.closers, closer)
	return nil
}

// Listen starts the SMTP server
func (ms *MailServer) Listen() error {
	// Start SMTPS server (465) if configured
	if ms.smtpsServer != nil {
		go func() {
			common.Log("owlmail SMTPS Server running at %s:465", ms.host)
			ln, err := net.Listen("tcp", ms.smtpsServer.Addr)
			if err != nil {
				common.Error("Failed to start SMTPS server: %v", err)
				return
			}
			tlsListener := tls.NewListener(ln, ms.smtpsServer.TLSConfig)
			if err := ms.smtpsServer.Serve(tlsListener); err != nil {
				common.Error("SMTPS server error: %v", err)
			}
		}()
	}

	common.Log("owlmail SMTP Server running at %s:%d", ms.host, ms.port)
	if ms.authConfig != nil && ms.authConfig.Enabled {
		common.Log("SMTP authentication enabled (PLAIN/LOGIN)")
	}
	if ms.tlsConfig != nil && ms.tlsConfig.Enabled {
		common.Log("SMTP TLS/STARTTLS enabled")
	}
	return ms.smtpServer.ListenAndServe()
}

// Close stops the SMTP server
func (ms *MailServer) Close() error {
	var closeErrors []error
	// Stop accepting SMTP data before draining dependent delivery services.
	if ms.smtpsServer != nil {
		if err := ms.smtpsServer.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	if err := ms.smtpServer.Close(); err != nil {
		closeErrors = append(closeErrors, err)
	}

	ms.closersMutex.Lock()
	closers := append([]io.Closer(nil), ms.closers...)
	ms.closersMutex.Unlock()
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	if ms.outgoing != nil {
		ms.outgoing.Close()
	}

	// Safely close eventChan, handling the case where it's already closed
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Channel is already closed, which is fine
				_ = r
			}
		}()
		close(ms.eventChan)
	}()

	return errors.Join(closeErrors...)
}

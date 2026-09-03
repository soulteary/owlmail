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
	return ms.ListenWithReady(nil)
}

// ListenWithReady binds the primary SMTP listener before calling ready and
// serving. Binding failures are returned without calling ready.
func (ms *MailServer) ListenWithReady(ready func()) error {
	listener, err := net.Listen("tcp", ms.smtpServer.Addr)
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()

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
	if ms.authRequired() {
		common.Log("SMTP AUTH required (PLAIN/LOGIN)")
	} else {
		common.Log("SMTP NO AUTH mode enabled (unauthenticated delivery accepted)")
	}
	if ms.authRequireTLS {
		common.Log("SMTP AUTH restricted to TLS connections")
	} else if !ms.authRequired() {
		common.Log("Arbitrary PLAIN/LOGIN credentials accepted for development clients")
	}
	if ms.tlsConfig != nil && ms.tlsConfig.Enabled {
		common.Log("SMTP TLS/STARTTLS enabled")
	}
	if ms.maxDataConcurrency == 0 {
		common.Log("SMTP DATA concurrency is unlimited")
	} else {
		common.Log("SMTP DATA concurrency limit: %d per process", ms.maxDataConcurrency)
	}
	if ready != nil {
		ready()
	}
	return ms.smtpServer.Serve(listener)
}

// Close stops the SMTP server
func (ms *MailServer) Close() error {
	var closeErrors []error
	// Reject new relay configuration and queue submissions from the start of
	// shutdown while allowing the current relay instance to drain below.
	ms.outgoingMutex.Lock()
	outgoingRelay := ms.outgoing
	shouldCloseOutgoing := !ms.outgoingClosed
	ms.outgoingClosed = true
	ms.outgoingMutex.Unlock()

	// Stop accepting SMTP data before draining dependent delivery services.
	if ms.smtpsServer != nil {
		if err := ms.smtpsServer.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	if err := ms.smtpServer.Close(); err != nil {
		closeErrors = append(closeErrors, err)
	}
	if ms.cleanupCancel != nil {
		ms.cleanupCancel()
		ms.cleanupWG.Wait()
	}

	ms.closersMutex.Lock()
	closers := append([]io.Closer(nil), ms.closers...)
	ms.closersMutex.Unlock()
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	if shouldCloseOutgoing && outgoingRelay != nil {
		outgoingRelay.Close()
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

package mailserver

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/soulteary/owlmail/internal/common"
)

// ErrSMTPSBind reports that the implicit-TLS listener could not take its
// address. The error states only what this package knows -- which listener
// failed, at which address, and why. The remedy is a CLI flag, and no other
// error in this package names one: cmd/owlmail wraps this error and adds the
// flags that move or disable the listener.
var ErrSMTPSBind = errors.New("SMTPS listener bind failed")

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

// ListenWithReady binds every configured SMTP listener before calling ready
// and serving. Binding failures are returned without calling ready, so a
// listener that never came up cannot be reported as a successful start.
func (ms *MailServer) ListenWithReady(ready func()) error {
	listener, err := net.Listen("tcp", ms.smtpServer.Addr)
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()

	// The implicit-TLS listener binds here rather than inside the serving
	// goroutine. Deferring the bind hid its failure behind a log line and left
	// startup and readiness reporting success with nothing accepting SMTPS,
	// which clients could only discover as a refused connection. The deferred
	// close above releases the plain listener when this bind fails, so a
	// rejected start leaves no port bound.
	if ms.smtpsServer != nil {
		smtpsListener, smtpsErr := net.Listen("tcp", ms.smtpsServer.Addr)
		if smtpsErr != nil {
			return fmt.Errorf("%w on %s: %w", ErrSMTPSBind, ms.smtpsServer.Addr, smtpsErr)
		}
		tlsListener := tls.NewListener(smtpsListener, ms.smtpsServer.TLSConfig)
		ms.setSMTPSListener(tlsListener)
		defer func() {
			if bound := ms.takeSMTPSListener(); bound != nil {
				_ = bound.Close()
			}
		}()
		common.Log("owlmail SMTPS Server running at %s", ms.smtpsServer.Addr)
		go func() {
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
		// smtp.Server.Close only closes the listeners Serve has registered, and
		// startup hands Serve the implicit-TLS listener from a goroutine. A
		// shutdown that wins that race would otherwise leave the SMTPS port
		// bound for the life of the process. In the ordinary case Serve did
		// register it and the Close above already closed it, so this close
		// reports "use of closed network connection"; that is the expected
		// outcome of the race rather than a shutdown failure, which is why it
		// is not collected.
		if bound := ms.takeSMTPSListener(); bound != nil {
			_ = bound.Close()
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

// setSMTPSListener records the bound implicit-TLS listener for shutdown.
func (ms *MailServer) setSMTPSListener(listener net.Listener) {
	ms.smtpsListenerMutex.Lock()
	defer ms.smtpsListenerMutex.Unlock()
	ms.smtpsListener = listener
}

// takeSMTPSListener hands the bound implicit-TLS listener to the first caller
// that asks for it. Both the startup path and Close have to be able to release
// the port, and neither can know which of them runs first; clearing the field
// under the mutex gives the listener exactly one owner.
func (ms *MailServer) takeSMTPSListener() net.Listener {
	ms.smtpsListenerMutex.Lock()
	defer ms.smtpsListenerMutex.Unlock()
	listener := ms.smtpsListener
	ms.smtpsListener = nil
	return listener
}

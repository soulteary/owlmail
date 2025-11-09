package main

import (
	"crypto/tls"
	"net"
)

// Listen starts the SMTP server
func (ms *MailServer) Listen() error {
	// Start SMTPS server (465) if configured
	if ms.smtpsServer != nil {
		go func() {
			Log("owlmail SMTPS Server running at %s:465", ms.host)
			ln, err := net.Listen("tcp", ms.smtpsServer.Addr)
			if err != nil {
				Error("Failed to start SMTPS server: %v", err)
				return
			}
			tlsListener := tls.NewListener(ln, ms.smtpsServer.TLSConfig)
			if err := ms.smtpsServer.Serve(tlsListener); err != nil {
				Error("SMTPS server error: %v", err)
			}
		}()
	}

	Log("owlmail SMTP Server running at %s:%d", ms.host, ms.port)
	if ms.authConfig != nil && ms.authConfig.Enabled {
		Log("SMTP authentication enabled (PLAIN/LOGIN)")
	}
	if ms.tlsConfig != nil && ms.tlsConfig.Enabled {
		Log("SMTP TLS/STARTTLS enabled")
	}
	return ms.smtpServer.ListenAndServe()
}

// Close stops the SMTP server
func (ms *MailServer) Close() error {
	if ms.outgoing != nil {
		ms.outgoing.Close()
	}
	close(ms.eventChan)

	var err error
	if ms.smtpsServer != nil {
		if closeErr := ms.smtpsServer.Close(); closeErr != nil {
			err = closeErr
		}
	}
	if closeErr := ms.smtpServer.Close(); closeErr != nil {
		err = closeErr
	}
	return err
}

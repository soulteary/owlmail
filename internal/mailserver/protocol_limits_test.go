package mailserver

import (
	"testing"
	"time"
)

func TestSMTPProtocolLimitsApplyToServer(t *testing.T) {
	server, err := NewMailServerWithOptions(1025, "localhost", t.TempDir(), ServerOptions{
		ReadTimeout:   2 * time.Second,
		WriteTimeout:  3 * time.Second,
		MaxRecipients: 12,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("close mail server: %v", err)
		}
	}()

	if server.GetReadTimeout() != 2*time.Second || server.smtpServer.ReadTimeout != 2*time.Second {
		t.Fatalf("SMTP read timeout was not applied: %v", server.smtpServer.ReadTimeout)
	}
	if server.GetWriteTimeout() != 3*time.Second || server.smtpServer.WriteTimeout != 3*time.Second {
		t.Fatalf("SMTP write timeout was not applied: %v", server.smtpServer.WriteTimeout)
	}
	if server.GetMaxRecipients() != 12 || server.smtpServer.MaxRecipients != 12 {
		t.Fatalf("SMTP max recipients was not applied: %d", server.smtpServer.MaxRecipients)
	}
}

func TestSMTPProtocolLimitDefaults(t *testing.T) {
	server, err := NewMailServerWithOptions(1025, "localhost", t.TempDir(), ServerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("close mail server: %v", err)
		}
	}()

	if server.smtpServer.ReadTimeout != defaultSMTPReadTimeout ||
		server.smtpServer.WriteTimeout != defaultSMTPWriteTimeout ||
		server.smtpServer.MaxRecipients != defaultSMTPMaxRecipients {
		t.Fatalf("unexpected SMTP protocol defaults: %#v", server.smtpServer)
	}
}

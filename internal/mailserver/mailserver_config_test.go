package mailserver

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/soulteary/owlmail/internal/outgoing"
)

func TestNewMailServer(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with default values
	server, err := NewMailServer(0, "", "")
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	if server.port != defaultPort {
		t.Errorf("Expected port %d, got %d", defaultPort, server.port)
	}
	if server.host != defaultHost {
		t.Errorf("Expected host %s, got %s", defaultHost, server.host)
	}
	if server.smtpServer.MaxMessageBytes != DefaultMaxMessageBytes {
		t.Errorf("default MaxMessageBytes = %d, want %d", server.smtpServer.MaxMessageBytes, DefaultMaxMessageBytes)
	}
	if !server.smtpServer.EnableSMTPUTF8 {
		t.Fatal("SMTP server should advertise SMTPUTF8 for internationalized messages")
	}

	// Test with custom values
	server2, err := NewMailServer(2525, "127.0.0.1", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server2.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	if server2.port != 2525 {
		t.Errorf("Expected port 2525, got %d", server2.port)
	}
	if server2.host != "127.0.0.1" {
		t.Errorf("Expected host 127.0.0.1, got %s", server2.host)
	}
	if server2.mailDir != tmpDir {
		t.Errorf("Expected mailDir %s, got %s", tmpDir, server2.mailDir)
	}
}

func TestNewMailServerWithCustomMessageLimit(t *testing.T) {
	const limit = int64(256 << 20)
	server, err := NewMailServerWithOptions(1025, "localhost", t.TempDir(), ServerOptions{
		MaxMessageBytes: limit,
		TLSConfig:       &TLSConfig{Enabled: true},
	})
	if err != nil {
		t.Fatalf("NewMailServerWithOptions() error = %v", err)
	}
	defer func() { _ = server.Close() }()
	if server.smtpServer.MaxMessageBytes != limit {
		t.Fatalf("MaxMessageBytes = %d, want %d", server.smtpServer.MaxMessageBytes, limit)
	}
	if server.smtpsServer == nil || server.smtpsServer.MaxMessageBytes != limit {
		t.Fatalf("SMTPS MaxMessageBytes was not configured: %#v", server.smtpsServer)
	}
	if !server.smtpsServer.EnableSMTPUTF8 {
		t.Fatal("SMTPS server should advertise SMTPUTF8 for internationalized messages")
	}
}

func TestNewMailServerWithDataConcurrencyLimit(t *testing.T) {
	server, err := NewMailServerWithOptions(1025, "localhost", t.TempDir(), ServerOptions{MaxDataConcurrency: 4})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	if got := server.GetMaxDataConcurrency(); got != 4 {
		t.Fatalf("GetMaxDataConcurrency() = %d, want 4", got)
	}
	if _, err := NewMailServerWithOptions(1025, "localhost", t.TempDir(), ServerOptions{MaxDataConcurrency: -1}); err == nil {
		t.Fatal("negative DATA concurrency was accepted")
	}
}

func TestNewMailServerRejectsAuthRequireTLSWithoutTLS(t *testing.T) {
	for _, tlsConfig := range []*TLSConfig{nil, {Enabled: false}} {
		_, err := NewMailServerWithOptions(1025, "localhost", t.TempDir(), ServerOptions{
			AuthRequireTLS: true,
			TLSConfig:      tlsConfig,
		})
		if err == nil || !strings.Contains(err.Error(), "SMTP AUTH cannot require TLS") {
			t.Fatalf("TLS config %#v error = %v, want clear configuration error", tlsConfig, err)
		}
	}
}

func TestNewMailServerWithOutgoing(t *testing.T) {
	tmpDir := t.TempDir()

	outgoingConfig := &outgoing.OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	}

	server, err := NewMailServerWithOutgoing(1025, "localhost", tmpDir, outgoingConfig)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	if server.outgoing == nil {
		t.Error("Outgoing mail should be configured")
	}
}

func TestNewMailServerWithConfig(t *testing.T) {
	tmpDir := t.TempDir()

	outgoingConfig := &outgoing.OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	}

	authConfig := &SMTPAuthConfig{
		Username: "user",
		Password: "pass",
		Enabled:  true,
	}

	tlsConfig := &TLSConfig{
		Enabled: true,
	}

	server, err := NewMailServerWithConfig(1025, "localhost", tmpDir, outgoingConfig, authConfig, tlsConfig)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	if server.authConfig == nil {
		t.Error("Auth config should be set")
	}
	if server.tlsConfig == nil {
		t.Error("TLS config should be set")
	}

	// Test getter methods
	if server.GetHost() != "localhost" {
		t.Errorf("Expected host 'localhost', got '%s'", server.GetHost())
	}
	if server.GetPort() != 1025 {
		t.Errorf("Expected port 1025, got %d", server.GetPort())
	}
	if server.GetMailDir() != tmpDir {
		t.Errorf("Expected mailDir '%s', got '%s'", tmpDir, server.GetMailDir())
	}
	if server.GetAuthConfig() == nil {
		t.Error("Auth config should be set")
	}
	if server.GetTLSConfig() == nil {
		t.Error("TLS config should be set")
	}
}

func TestNewMailServerWithFullConfigControlsGeneratedIDs(t *testing.T) {
	for _, test := range []struct {
		name    string
		useUUID bool
	}{
		{name: "short ID", useUUID: false},
		{name: "UUID", useUUID: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server, err := NewMailServerWithFullConfig(1025, "localhost", t.TempDir(), nil, nil, nil, test.useUUID)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = server.Close() }()
			session := &Session{mailServer: server, from: "from@example.test", to: []string{"to@example.test"}}
			message := []byte("From: from@example.test\r\nTo: to@example.test\r\nSubject: ID test\r\n\r\nbody")
			if err := session.Data(bytes.NewReader(message)); err != nil {
				t.Fatal(err)
			}
			emails := server.GetAllEmail()
			if len(emails) != 1 {
				t.Fatalf("stored emails = %d, want 1", len(emails))
			}
			if test.useUUID {
				if _, err := uuid.Parse(emails[0].ID); err != nil {
					t.Fatalf("generated ID %q is not a UUID: %v", emails[0].ID, err)
				}
			} else if len(emails[0].ID) != 8 {
				t.Fatalf("generated ID length = %d, want 8", len(emails[0].ID))
			}
		})
	}
}

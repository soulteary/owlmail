package mailserver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/soulteary/owlmail/internal/outgoing"
)

func TestMailServerSetOutgoingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Set outgoing config
	config := &outgoing.OutgoingConfig{
		Host:     "smtp.example.com",
		Port:     587,
		User:     "user",
		Password: "pass",
	}

	server.SetOutgoingConfig(config)

	// Get config
	retrieved := server.GetOutgoingConfig()
	if retrieved == nil {
		t.Fatal("Outgoing config should be set")
	}
	if retrieved.Host != config.Host {
		t.Errorf("Expected host %s, got %s", config.Host, retrieved.Host)
	}
}

func TestRelayMail(t *testing.T) {
	tmpDir := t.TempDir()

	// Create server without outgoing config
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Test RelayMail without outgoing config
	email := &Email{
		ID:      "test-id",
		Subject: "Test",
		Envelope: &Envelope{
			From: "from@example.com",
			To:   []string{"to@example.com"},
		},
	}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("test email"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	err = server.RelayMail(email, false, func(err error) {
		if err == nil {
			t.Error("RelayMail should fail without outgoing config")
		}
	})
	if err == nil {
		t.Error("RelayMail should return error without outgoing config")
	}

	// Test RelayMail with outgoing config
	outgoingConfig := &outgoing.OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	}
	server.SetOutgoingConfig(outgoingConfig)

	// RelayMail will queue the task, but actual relay will fail in test
	// We can test that it doesn't panic
	err = server.RelayMail(email, true, func(err error) {
		// Callback will be called with error in test environment
	})
	// Error is expected in test environment, but we just want to verify it doesn't panic
	_ = err
}

func TestRelayMailTo(t *testing.T) {
	tmpDir := t.TempDir()

	// Create server without outgoing config
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Test RelayMailTo without outgoing config
	email := &Email{
		ID:      "test-id",
		Subject: "Test",
		Envelope: &Envelope{
			From: "from@example.com",
			To:   []string{"to@example.com"},
		},
	}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("test email"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	err = server.RelayMailTo(email, "relay@example.com", func(err error) {
		if err == nil {
			t.Error("RelayMailTo should fail without outgoing config")
		}
	})
	if err == nil {
		t.Error("RelayMailTo should return error without outgoing config")
	}

	// Test RelayMailTo with outgoing config
	outgoingConfig := &outgoing.OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	}
	server.SetOutgoingConfig(outgoingConfig)

	// RelayMailTo will queue the task, but actual relay will fail in test
	// We can test that it doesn't panic
	err = server.RelayMailTo(email, "relay@example.com", func(err error) {
		// Callback will be called with error in test environment
	})
	// Error is expected in test environment, but we just want to verify it doesn't panic
	_ = err
}

package main

import (
	"testing"
)

func TestNewMailServer(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with default values
	server, err := NewMailServer(0, "", "")
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	if server.port != defaultPort {
		t.Errorf("Expected port %d, got %d", defaultPort, server.port)
	}
	if server.host != defaultHost {
		t.Errorf("Expected host %s, got %s", defaultHost, server.host)
	}

	// Test with custom values
	server2, err := NewMailServer(2525, "127.0.0.1", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server2.Close()

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

func TestNewMailServerWithOutgoing(t *testing.T) {
	tmpDir := t.TempDir()

	outgoingConfig := &OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	}

	server, err := NewMailServerWithOutgoing(1025, "localhost", tmpDir, outgoingConfig)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	if server.outgoing == nil {
		t.Error("Outgoing mail should be configured")
	}
}

func TestNewMailServerWithConfig(t *testing.T) {
	tmpDir := t.TempDir()

	outgoingConfig := &OutgoingConfig{
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
	defer server.Close()

	if server.authConfig == nil {
		t.Error("Auth config should be set")
	}
	if server.tlsConfig == nil {
		t.Error("TLS config should be set")
	}
}

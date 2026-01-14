package mailserver

import (
	"os"
	"path/filepath"
	"reflect"
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

func TestSetOutgoingConfigUpdate(t *testing.T) {
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

	// Set initial config
	config1 := &outgoing.OutgoingConfig{
		Host:     "smtp.example.com",
		Port:     587,
		User:     "user1",
		Password: "pass1",
	}
	server.SetOutgoingConfig(config1)

	// Update config (test the else branch in SetOutgoingConfig)
	config2 := &outgoing.OutgoingConfig{
		Host:     "smtp.another.com",
		Port:     465,
		User:     "user2",
		Password: "pass2",
	}
	server.SetOutgoingConfig(config2)

	// Verify config was updated
	retrieved := server.GetOutgoingConfig()
	if retrieved == nil {
		t.Fatal("Outgoing config should be set")
	}
	if retrieved.Host != config2.Host {
		t.Errorf("Expected host %s, got %s", config2.Host, retrieved.Host)
	}
	if retrieved.Port != config2.Port {
		t.Errorf("Expected port %d, got %d", config2.Port, retrieved.Port)
	}
}

func TestGetOutgoingConfigNil(t *testing.T) {
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

	// Test GetOutgoingConfig when outgoing is nil
	config := server.GetOutgoingConfig()
	if config != nil {
		t.Error("GetOutgoingConfig should return nil when outgoing is not configured")
	}
}

// mockOutgoing is a mock implementation that returns a non-OutgoingConfig type
type mockOutgoing struct{}

func (m *mockOutgoing) RelayMail(email interface{}, emlPath, relayTo string, isAutoRelay bool, callback func(error)) {
	if callback != nil {
		callback(nil)
	}
}

func (m *mockOutgoing) UpdateConfig(config interface{}) {}

func (m *mockOutgoing) GetConfig() interface{} {
	// Return a string instead of *OutgoingConfig to test type assertion failure
	return "not an OutgoingConfig"
}

func (m *mockOutgoing) IsAutoRelayEnabled() bool {
	return false
}

func (m *mockOutgoing) Close() {}

func TestGetOutgoingConfigTypeAssertionFailure(t *testing.T) {
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

	// Use reflection to set a mock outgoing that returns a non-OutgoingConfig type
	mock := &mockOutgoing{}
	rv := reflect.ValueOf(server).Elem()
	field := rv.FieldByName("outgoing")
	if !field.IsValid() || !field.CanSet() {
		t.Skip("Cannot set outgoing field via reflection, skipping type assertion failure test")
		return
	}

	// Set the mock outgoing
	field.Set(reflect.ValueOf(mock))

	// Test GetOutgoingConfig when type assertion fails
	config := server.GetOutgoingConfig()
	if config != nil {
		t.Error("GetOutgoingConfig should return nil when type assertion fails")
	}
}

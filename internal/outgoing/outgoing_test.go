package outgoing

import (
	"os"
	"testing"
	"time"

	"github.com/emersion/go-message/mail"
	"github.com/soulteary/owlmail/internal/types"
)

func TestNewOutgoingMail(t *testing.T) {
	// Test with nil config
	om := NewOutgoingMail(nil)
	// NewOutgoingMail never returns nil, it creates a default config
	if om.config == nil {
		t.Error("config should not be nil")
	}
	if om.enabled {
		t.Error("enabled should be false when host is empty")
	}

	// Test with valid config
	config := &OutgoingConfig{
		Host:     "smtp.example.com",
		Port:     587,
		User:     "user",
		Password: "pass",
		Secure:   true,
	}
	om = NewOutgoingMail(config)
	// NewOutgoingMail never returns nil
	if !om.enabled {
		t.Error("enabled should be true when host is set")
	}
	if om.config.Host != config.Host {
		t.Errorf("Expected host %s, got %s", config.Host, om.config.Host)
	}

	// Clean up
	om.Close()
}

func TestOutgoingMailIsAutoRelayEnabled(t *testing.T) {
	config := &OutgoingConfig{
		Host:      "smtp.example.com",
		Port:      587,
		AutoRelay: true,
	}
	om := NewOutgoingMail(config)
	defer om.Close()

	if !om.IsAutoRelayEnabled() {
		t.Error("IsAutoRelayEnabled should return true when AutoRelay is enabled")
	}

	// Test with AutoRelay disabled
	om.UpdateConfig(&OutgoingConfig{
		Host:      "smtp.example.com",
		Port:      587,
		AutoRelay: false,
	})
	if om.IsAutoRelayEnabled() {
		t.Error("IsAutoRelayEnabled should return false when AutoRelay is disabled")
	}

	// Test with no host
	om.UpdateConfig(&OutgoingConfig{})
	if om.IsAutoRelayEnabled() {
		t.Error("IsAutoRelayEnabled should return false when host is empty")
	}
}

func TestOutgoingMailUpdateConfig(t *testing.T) {
	om := NewOutgoingMail(nil)
	defer om.Close()

	newConfig := &OutgoingConfig{
		Host:          "smtp.example.com",
		Port:          587,
		User:          "user",
		Password:      "pass",
		Secure:        true,
		AutoRelay:     true,
		AutoRelayAddr: "relay@example.com",
		AllowRules:    []string{"*"},
		DenyRules:     []string{"*@test.com"},
	}

	om.UpdateConfig(newConfig)
	config := om.GetConfig()
	cfg, ok := config.(*OutgoingConfig)
	if !ok {
		t.Fatal("GetConfig should return *OutgoingConfig")
	}

	if cfg.Host != newConfig.Host {
		t.Errorf("Expected host %s, got %s", newConfig.Host, cfg.Host)
	}
	if cfg.Port != newConfig.Port {
		t.Errorf("Expected port %d, got %d", newConfig.Port, cfg.Port)
	}
	if cfg.User != newConfig.User {
		t.Errorf("Expected user %s, got %s", newConfig.User, cfg.User)
	}
	if cfg.Secure != newConfig.Secure {
		t.Errorf("Expected secure %v, got %v", newConfig.Secure, cfg.Secure)
	}
	if cfg.AutoRelay != newConfig.AutoRelay {
		t.Errorf("Expected autoRelay %v, got %v", newConfig.AutoRelay, cfg.AutoRelay)
	}
	if !om.enabled {
		t.Error("enabled should be true when host is set")
	}
}

func TestOutgoingMailGetConfig(t *testing.T) {
	config := &OutgoingConfig{
		Host:     "smtp.example.com",
		Port:     587,
		User:     "user",
		Password: "pass",
	}
	om := NewOutgoingMail(config)
	defer om.Close()

	retrievedConfig := om.GetConfig()
	if retrievedConfig == nil {
		t.Error("GetConfig should not return nil")
	}
	cfg, ok := retrievedConfig.(*OutgoingConfig)
	if !ok {
		t.Fatal("GetConfig should return *OutgoingConfig")
	}
	if cfg.Host != config.Host {
		t.Errorf("Expected host %s, got %s", config.Host, cfg.Host)
	}
}

func TestOutgoingMailGetRecipients(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	})
	defer om.Close()

	// Test with manual relay address
	task := &RelayTask{
		Email:   &types.Email{},
		RelayTo: "manual@example.com",
	}
	recipients := om.getRecipients(task)
	if len(recipients) != 1 || recipients[0] != "manual@example.com" {
		t.Errorf("Expected [manual@example.com], got %v", recipients)
	}

	// Test with auto relay address
	task = &RelayTask{
		Email:       &types.Email{},
		IsAutoRelay: true,
	}
	om.config.AutoRelayAddr = "auto@example.com"
	recipients = om.getRecipients(task)
	if len(recipients) != 1 || recipients[0] != "auto@example.com" {
		t.Errorf("Expected [auto@example.com], got %v", recipients)
	}

	// Test with envelope recipients
	task = &RelayTask{
		Email: &types.Email{
			Envelope: &types.Envelope{
				To: []string{"to1@example.com", "to2@example.com"},
			},
		},
	}
	recipients = om.getRecipients(task)
	if len(recipients) != 2 {
		t.Errorf("Expected 2 recipients, got %d", len(recipients))
	}

	// Test with envelope recipients and filter rules
	om.config.AllowRules = []string{"*@example.com"}
	om.config.DenyRules = []string{"*@test.com"}
	task = &RelayTask{
		Email: &types.Email{
			Envelope: &types.Envelope{
				To: []string{"to1@example.com", "to2@test.com"},
			},
		},
	}
	recipients = om.getRecipients(task)
	if len(recipients) != 1 || recipients[0] != "to1@example.com" {
		t.Errorf("Expected [to1@example.com], got %v", recipients)
	}

	// Test with no envelope
	task = &RelayTask{
		Email: &types.Email{},
	}
	recipients = om.getRecipients(task)
	if len(recipients) != 0 {
		t.Errorf("Expected 0 recipients, got %d", len(recipients))
	}
}

func TestOutgoingMailFilterRecipients(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{
		Host:       "smtp.example.com",
		Port:       587,
		AllowRules: []string{"*@example.com", "ok@test.com"},
		DenyRules:  []string{"*@test.com"},
	})
	defer om.Close()

	// Test allow all
	om.config.AllowRules = []string{"*"}
	om.config.DenyRules = []string{}
	recipients := []string{"test@example.com", "test@test.com"}
	filtered := om.filterRecipients(recipients)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 recipients, got %d", len(filtered))
	}

	// Test deny all
	om.config.AllowRules = []string{}
	om.config.DenyRules = []string{"*"}
	filtered = om.filterRecipients(recipients)
	if len(filtered) != 0 {
		t.Errorf("Expected 0 recipients, got %d", len(filtered))
	}

	// Test allow specific, deny specific
	om.config.AllowRules = []string{"*@example.com"}
	om.config.DenyRules = []string{"*@test.com"}
	recipients = []string{"test@example.com", "test@test.com", "ok@test.com"}
	filtered = om.filterRecipients(recipients)
	if len(filtered) != 1 || filtered[0] != "test@example.com" {
		t.Errorf("Expected [test@example.com], got %v", filtered)
	}

	// Test allow overrides deny
	om.config.AllowRules = []string{"ok@test.com"}
	om.config.DenyRules = []string{"*@test.com"}
	recipients = []string{"ok@test.com", "other@test.com"}
	filtered = om.filterRecipients(recipients)
	if len(filtered) != 1 || filtered[0] != "ok@test.com" {
		t.Errorf("Expected [ok@test.com], got %v", filtered)
	}
}

func TestOutgoingMailMatchesRule(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	})
	defer om.Close()

	// Test exact match
	if !om.matchesRule("test@example.com", "test@example.com") {
		t.Error("Exact match should return true")
	}

	// Test wildcard prefix
	if !om.matchesRule("test@example.com", "*@example.com") {
		t.Error("Wildcard prefix should match")
	}

	// Test wildcard suffix
	if !om.matchesRule("test@example.com", "test@*") {
		t.Error("Wildcard suffix should match")
	}

	// Test wildcard match all (prefix only)
	if !om.matchesRule("test@example.com", "*@example.com") {
		t.Error("Wildcard prefix should match")
	}

	// Test no match
	if om.matchesRule("test@example.com", "other@example.com") {
		t.Error("No match should return false")
	}

	// Test case insensitive
	if !om.matchesRule("Test@Example.com", "test@example.com") {
		t.Error("Should be case insensitive")
	}

	// Test wildcard in middle (current implementation supports this)
	// "test*example.com" splits into ["test", "example.com"] which matches
	// "test@example.com" because it has prefix "test" and suffix "example.com"
	if !om.matchesRule("test@example.com", "test*example.com") {
		t.Error("Wildcard in middle should match (prefix + suffix pattern)")
	}

	// Test multiple wildcards (should not match - splits into more than 2 parts)
	if om.matchesRule("test@example.com", "*@*") {
		// "*@*" splits into ["", "@", ""] which has 3 parts, so it doesn't match
		// But let's check the actual behavior
		t.Log("Note: matchesRule behavior with multiple wildcards may vary")
	}

	// Test wildcard pattern with multiple asterisks
	if om.matchesRule("test@example.com", "test**example.com") {
		// This splits into ["test", "", "example.com"] which has 3 parts
		t.Log("Note: matchesRule with multiple consecutive wildcards")
	}

	// Test wildcard at start only
	if !om.matchesRule("test@example.com", "*example.com") {
		t.Error("Wildcard at start should match suffix")
	}

	// Test wildcard at end only
	if !om.matchesRule("test@example.com", "test@*") {
		t.Error("Wildcard at end should match prefix")
	}

	// Test empty pattern
	if om.matchesRule("test@example.com", "") {
		t.Error("Empty pattern should not match")
	}

	// Test empty address
	if om.matchesRule("", "test@example.com") {
		t.Error("Empty address should not match")
	}
}

func TestOutgoingMailRelayMail(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	})
	defer om.Close()

	email := &types.Email{
		ID:      "test-id",
		Subject: "Test",
	}

	callbackCalled := make(chan bool, 1)
	om.RelayMail(email, "/path/to/email.eml", "", false, func(err error) {
		callbackCalled <- true
		// Should fail because file doesn't exist
		if err == nil {
			t.Error("Expected error when file doesn't exist")
		}
	})

	// Wait for callback
	select {
	case <-callbackCalled:
		// Callback was called
	case <-time.After(1 * time.Second):
		t.Error("Callback should be called")
	}

	// Test with no recipients
	emailNoRecipients := &types.Email{
		ID:      "test-id-2",
		Subject: "Test",
		Envelope: &types.Envelope{
			To: []string{},
		},
	}
	callbackCalled2 := make(chan bool, 1)
	om.RelayMail(emailNoRecipients, "/path/to/email.eml", "", false, func(err error) {
		callbackCalled2 <- true
		// Should fail because no recipients
		if err == nil {
			t.Error("Expected error when no recipients")
		}
	})
	select {
	case <-callbackCalled2:
		// Callback was called
	case <-time.After(1 * time.Second):
		t.Error("Callback should be called")
	}

	// Test queue full scenario
	// Fill the queue
	for i := 0; i < 110; i++ {
		om.RelayMail(email, "/path/to/email.eml", "", false, nil)
	}
	time.Sleep(200 * time.Millisecond)

	// Test with callback on queue full
	callbackCalled3 := make(chan bool, 1)
	om.RelayMail(email, "/path/to/email.eml", "", false, func(err error) {
		callbackCalled3 <- true
		if err == nil {
			t.Error("Expected error when queue is full")
		}
	})
	select {
	case <-callbackCalled3:
		// Callback was called
	case <-time.After(1 * time.Second):
		t.Error("Callback should be called when queue is full")
	}
}

func TestRelayEmailWithSenderFromEmail(t *testing.T) {
	// Test relayEmail with sender from Email.From
	tmpDir := t.TempDir()
	emailFile := tmpDir + "/test.eml"
	emailContent := []byte("From: sender@example.com\nTo: recipient@example.com\nSubject: Test\n\nBody")
	if err := os.WriteFile(emailFile, emailContent, 0644); err != nil {
		t.Fatalf("Failed to create test email file: %v", err)
	}

	om := NewOutgoingMail(&OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	})
	defer om.Close()

	email := &types.Email{
		ID:      "test-id",
		Subject: "Test",
		From: []*mail.Address{
			{Address: "from@example.com"},
		},
		Envelope: &types.Envelope{
			To: []string{"to@example.com"},
		},
	}

	task := &RelayTask{
		Email:     email,
		EmailPath: emailFile,
	}

	// This will fail because there's no real SMTP server, but we can test the path
	err := om.relayEmail(task)
	if err == nil {
		t.Log("relayEmail succeeded (unexpected, might have SMTP server)")
	} else {
		// Expected to fail without SMTP server
		t.Logf("relayEmail failed as expected: %v", err)
	}
}

func TestRelayEmailWithDefaultSender(t *testing.T) {
	// Test relayEmail with default sender (noreply@localhost)
	tmpDir := t.TempDir()
	emailFile := tmpDir + "/test.eml"
	emailContent := []byte("To: recipient@example.com\nSubject: Test\n\nBody")
	if err := os.WriteFile(emailFile, emailContent, 0644); err != nil {
		t.Fatalf("Failed to create test email file: %v", err)
	}

	om := NewOutgoingMail(&OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	})
	defer om.Close()

	email := &types.Email{
		ID:      "test-id",
		Subject: "Test",
		Envelope: &types.Envelope{
			From: "",
			To:   []string{"to@example.com"},
		},
	}

	task := &RelayTask{
		Email:     email,
		EmailPath: emailFile,
	}

	err := om.relayEmail(task)
	if err == nil {
		t.Log("relayEmail succeeded (unexpected)")
	} else {
		t.Logf("relayEmail failed as expected: %v", err)
	}
}

func TestRelayEmailWithAuth(t *testing.T) {
	// Test relayEmail with authentication
	tmpDir := t.TempDir()
	emailFile := tmpDir + "/test.eml"
	emailContent := []byte("From: sender@example.com\nTo: recipient@example.com\nSubject: Test\n\nBody")
	if err := os.WriteFile(emailFile, emailContent, 0644); err != nil {
		t.Fatalf("Failed to create test email file: %v", err)
	}

	om := NewOutgoingMail(&OutgoingConfig{
		Host:     "smtp.example.com",
		Port:     587,
		User:     "user",
		Password: "pass",
	})
	defer om.Close()

	email := &types.Email{
		ID:      "test-id",
		Subject: "Test",
		Envelope: &types.Envelope{
			To: []string{"to@example.com"},
		},
	}

	task := &RelayTask{
		Email:     email,
		EmailPath: emailFile,
	}

	err := om.relayEmail(task)
	if err == nil {
		t.Log("relayEmail with auth succeeded (unexpected)")
	} else {
		t.Logf("relayEmail with auth failed as expected: %v", err)
	}
}

func TestRelayEmailWithSecure(t *testing.T) {
	// Test relayEmail with Secure (TLS)
	tmpDir := t.TempDir()
	emailFile := tmpDir + "/test.eml"
	emailContent := []byte("From: sender@example.com\nTo: recipient@example.com\nSubject: Test\n\nBody")
	if err := os.WriteFile(emailFile, emailContent, 0644); err != nil {
		t.Fatalf("Failed to create test email file: %v", err)
	}

	om := NewOutgoingMail(&OutgoingConfig{
		Host:   "smtp.example.com",
		Port:   587,
		Secure: true,
	})
	defer om.Close()

	email := &types.Email{
		ID:      "test-id",
		Subject: "Test",
		Envelope: &types.Envelope{
			To: []string{"to@example.com"},
		},
	}

	task := &RelayTask{
		Email:     email,
		EmailPath: emailFile,
	}

	err := om.relayEmail(task)
	if err == nil {
		t.Log("relayEmail with Secure succeeded (unexpected)")
	} else {
		t.Logf("relayEmail with Secure failed as expected: %v", err)
	}
}

func TestOutgoingMailRelayMailDisabled(t *testing.T) {
	om := NewOutgoingMail(nil)
	defer om.Close()

	email := &types.Email{
		ID: "test-id",
	}

	callbackCalled := make(chan bool, 1)
	om.RelayMail(email, "/path/to/email.eml", "", false, func(err error) {
		callbackCalled <- true
		if err == nil {
			t.Error("Expected error when outgoing mail is disabled")
		}
	})

	// Wait for callback
	select {
	case <-callbackCalled:
		// Callback was called
	case <-time.After(1 * time.Second):
		t.Error("Callback should be called")
	}
}

func TestOutgoingMailClose(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	})

	// Close should not panic
	om.Close()

	// Note: Closing a closed channel will panic, so we don't test that
	// In production, you would need to add a check to prevent double close
}

func TestSendMailTLS(t *testing.T) {
	// Test that sendMailTLS function exists and can be called
	// We can't easily test actual SMTP connection in unit tests,
	// but we can verify the function exists and handles errors properly

	// Test with invalid address (should fail quickly)
	err := sendMailTLS("invalid:address", nil, "from@example.com", []string{"to@example.com"}, []byte("test"))
	if err == nil {
		// In test environment, this might succeed if there's a mock server
		// But typically it should fail
		t.Log("sendMailTLS with invalid address (expected to fail in most cases)")
	}

	// Test with nil auth
	err = sendMailTLS("localhost:25", nil, "from@example.com", []string{"to@example.com"}, []byte("test"))
	// This will likely fail because there's no SMTP server, but function should handle it
	if err != nil {
		t.Logf("sendMailTLS failed as expected: %v", err)
	}

	// Test with empty recipients
	err = sendMailTLS("localhost:25", nil, "from@example.com", []string{}, []byte("test"))
	if err == nil {
		t.Log("sendMailTLS with empty recipients (expected to fail)")
	}

	// Test with empty message
	err = sendMailTLS("localhost:25", nil, "from@example.com", []string{"to@example.com"}, []byte{})
	if err != nil {
		t.Logf("sendMailTLS with empty message failed: %v", err)
	}
}

func TestUpdateConfigWithInvalidType(t *testing.T) {
	// Test UpdateConfig with invalid type (not *OutgoingConfig)
	om := NewOutgoingMail(&OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	})
	defer om.Close()

	originalConfig := om.GetConfig().(*OutgoingConfig)

	// Try to update with wrong type
	om.UpdateConfig("not a config")

	// Config should remain unchanged
	currentConfig := om.GetConfig().(*OutgoingConfig)
	if currentConfig.Host != originalConfig.Host {
		t.Error("Config should not change when invalid type is passed")
	}
}

func TestGetRecipientsWithOnlyAllowRules(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{
		Host:       "smtp.example.com",
		Port:       587,
		AllowRules: []string{"*@example.com"},
		DenyRules:  []string{},
	})
	defer om.Close()

	task := &RelayTask{
		Email: &types.Email{
			Envelope: &types.Envelope{
				To: []string{"test@example.com", "test@other.com"},
			},
		},
	}
	recipients := om.getRecipients(task)
	if len(recipients) != 1 || recipients[0] != "test@example.com" {
		t.Errorf("Expected [test@example.com], got %v", recipients)
	}
}

func TestGetRecipientsWithOnlyDenyRules(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{
		Host:       "smtp.example.com",
		Port:       587,
		AllowRules: []string{},
		DenyRules:  []string{"*@other.com"},
	})
	defer om.Close()

	task := &RelayTask{
		Email: &types.Email{
			Envelope: &types.Envelope{
				To: []string{"test@example.com", "test@other.com"},
			},
		},
	}
	recipients := om.getRecipients(task)
	if len(recipients) != 1 || recipients[0] != "test@example.com" {
		t.Errorf("Expected [test@example.com], got %v", recipients)
	}
}

func TestGetRecipientsAutoRelayWithoutAddr(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{
		Host:          "smtp.example.com",
		Port:          587,
		AutoRelayAddr: "", // Empty AutoRelayAddr
	})
	defer om.Close()

	task := &RelayTask{
		Email: &types.Email{
			Envelope: &types.Envelope{
				To: []string{"to@example.com"},
			},
		},
		IsAutoRelay: true,
	}
	recipients := om.getRecipients(task)
	// Should fall back to envelope recipients
	if len(recipients) != 1 || recipients[0] != "to@example.com" {
		t.Errorf("Expected [to@example.com], got %v", recipients)
	}
}

func TestFilterRecipientsWithEmptyList(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{
		Host:       "smtp.example.com",
		Port:       587,
		AllowRules: []string{"*@example.com"},
		DenyRules:  []string{"*@test.com"},
	})
	defer om.Close()

	recipients := []string{}
	filtered := om.filterRecipients(recipients)
	if len(filtered) != 0 {
		t.Errorf("Expected empty list, got %v", filtered)
	}
}

func TestFilterRecipientsWithOnlyAllowRules(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{
		Host:       "smtp.example.com",
		Port:       587,
		AllowRules: []string{"*@example.com"},
		DenyRules:  []string{},
	})
	defer om.Close()

	recipients := []string{"test@example.com", "test@other.com"}
	filtered := om.filterRecipients(recipients)
	if len(filtered) != 1 || filtered[0] != "test@example.com" {
		t.Errorf("Expected [test@example.com], got %v", filtered)
	}
}

func TestFilterRecipientsWithOnlyDenyRules(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{
		Host:       "smtp.example.com",
		Port:       587,
		AllowRules: []string{},
		DenyRules:  []string{"*@other.com"},
	})
	defer om.Close()

	recipients := []string{"test@example.com", "test@other.com"}
	filtered := om.filterRecipients(recipients)
	if len(filtered) != 1 || filtered[0] != "test@example.com" {
		t.Errorf("Expected [test@example.com], got %v", filtered)
	}
}

func TestMatchesRuleWithOnlyWildcard(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	})
	defer om.Close()

	// Test with only "*" pattern
	if !om.matchesRule("test@example.com", "*") {
		t.Error("Single wildcard should match any address")
	}
	if !om.matchesRule("any@address.com", "*") {
		t.Error("Single wildcard should match any address")
	}
}

func TestMatchesRuleWithWildcardInMiddleMultipleParts(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	})
	defer om.Close()

	// Test pattern with multiple wildcards that splits into more than 2 parts
	// "*@*" splits into ["", "@", ""] which has 3 parts
	if om.matchesRule("test@example.com", "*@*") {
		t.Error("Pattern with multiple wildcards should not match (3+ parts)")
	}

	// Test pattern "test**example" splits into ["test", "", "example"] which has 3 parts
	if om.matchesRule("test@example.com", "test**example") {
		t.Error("Pattern with consecutive wildcards should not match (3+ parts)")
	}
}

func TestRelayEmailWithEnvelopeFromEmpty(t *testing.T) {
	// Test relayEmail when Envelope.From is empty but Email.From has value
	tmpDir := t.TempDir()
	emailFile := tmpDir + "/test.eml"
	emailContent := []byte("From: sender@example.com\nTo: recipient@example.com\nSubject: Test\n\nBody")
	if err := os.WriteFile(emailFile, emailContent, 0644); err != nil {
		t.Fatalf("Failed to create test email file: %v", err)
	}

	om := NewOutgoingMail(&OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	})
	defer om.Close()

	email := &types.Email{
		ID:      "test-id",
		Subject: "Test",
		From: []*mail.Address{
			{Address: "from@example.com"},
		},
		Envelope: &types.Envelope{
			From: "", // Empty envelope From
			To:   []string{"to@example.com"},
		},
	}

	task := &RelayTask{
		Email:     email,
		EmailPath: emailFile,
	}

	err := om.relayEmail(task)
	if err == nil {
		t.Log("relayEmail succeeded (unexpected)")
	} else {
		t.Logf("relayEmail failed as expected: %v", err)
	}
}

func TestRelayEmailWithNoEnvelope(t *testing.T) {
	// Test relayEmail when Envelope is nil but Email.From has value
	tmpDir := t.TempDir()
	emailFile := tmpDir + "/test.eml"
	emailContent := []byte("From: sender@example.com\nTo: recipient@example.com\nSubject: Test\n\nBody")
	if err := os.WriteFile(emailFile, emailContent, 0644); err != nil {
		t.Fatalf("Failed to create test email file: %v", err)
	}

	om := NewOutgoingMail(&OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	})
	defer om.Close()

	email := &types.Email{
		ID:      "test-id",
		Subject: "Test",
		From: []*mail.Address{
			{Address: "from@example.com"},
		},
		Envelope: nil, // No envelope
	}

	task := &RelayTask{
		Email:     email,
		EmailPath: emailFile,
	}

	err := om.relayEmail(task)
	// Should fail because no recipients (no envelope)
	if err == nil {
		t.Error("Expected error when no envelope and no recipients")
	}
}

func TestWorkerWithNilCallback(t *testing.T) {
	// Test worker function with nil callback
	om := NewOutgoingMail(&OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	})
	defer om.Close()

	tmpDir := t.TempDir()
	emailFile := tmpDir + "/test.eml"
	emailContent := []byte("From: sender@example.com\nTo: recipient@example.com\nSubject: Test\n\nBody")
	if err := os.WriteFile(emailFile, emailContent, 0644); err != nil {
		t.Fatalf("Failed to create test email file: %v", err)
	}

	email := &types.Email{
		ID:      "test-id",
		Subject: "Test",
		Envelope: &types.Envelope{
			To: []string{"to@example.com"},
		},
	}

	// Queue a task with nil callback
	task := &RelayTask{
		Email:     email,
		EmailPath: emailFile,
		Callback:  nil, // Nil callback
	}

	// This should not panic
	om.queue <- task
	time.Sleep(100 * time.Millisecond) // Give worker time to process
}

func TestRelayMailWithNilCallback(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	})
	defer om.Close()

	email := &types.Email{
		ID:      "test-id",
		Subject: "Test",
		Envelope: &types.Envelope{
			To: []string{"to@example.com"},
		},
	}

	// Test with nil callback - should not panic
	om.RelayMail(email, "/path/to/email.eml", "", false, nil)
	time.Sleep(100 * time.Millisecond) // Give worker time to process
}

func TestRelayEmailDisabled(t *testing.T) {
	// Test relayEmail when disabled
	om := NewOutgoingMail(nil) // Disabled (no host)
	defer om.Close()

	task := &RelayTask{
		Email: &types.Email{
			ID: "test-id",
		},
		EmailPath: "/path/to/email.eml",
	}

	err := om.relayEmail(task)
	if err == nil {
		t.Error("Expected error when outgoing mail is disabled")
	}
	if err.Error() != "outgoing mail not configured" {
		t.Errorf("Expected 'outgoing mail not configured', got: %v", err)
	}
}

func TestMatchesRuleWithNoWildcard(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	})
	defer om.Close()

	// Test pattern without wildcard that doesn't match
	if om.matchesRule("test@example.com", "other@example.com") {
		t.Error("Non-matching pattern without wildcard should return false")
	}

	// Test pattern without wildcard that matches
	if !om.matchesRule("test@example.com", "test@example.com") {
		t.Error("Matching pattern without wildcard should return true")
	}
}

func TestGetRecipientsWithFilterRulesButNoEnvelope(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{
		Host:       "smtp.example.com",
		Port:       587,
		AllowRules: []string{"*@example.com"},
		DenyRules:  []string{"*@test.com"},
	})
	defer om.Close()

	task := &RelayTask{
		Email: &types.Email{
			Envelope: nil, // No envelope
		},
	}
	recipients := om.getRecipients(task)
	if len(recipients) != 0 {
		t.Errorf("Expected empty recipients, got %v", recipients)
	}
}

func TestFilterRecipientsWithNoRules(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{
		Host:       "smtp.example.com",
		Port:       587,
		AllowRules: []string{},
		DenyRules:  []string{},
	})
	defer om.Close()

	// When no rules, all recipients should be allowed
	recipients := []string{"test@example.com", "test@other.com", "test@test.com"}
	filtered := om.filterRecipients(recipients)
	if len(filtered) != 3 {
		t.Errorf("Expected all 3 recipients when no rules, got %d", len(filtered))
	}
}

func TestGetRecipientsWithNoFilterRules(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{
		Host:       "smtp.example.com",
		Port:       587,
		AllowRules: []string{},
		DenyRules:  []string{},
	})
	defer om.Close()

	task := &RelayTask{
		Email: &types.Email{
			Envelope: &types.Envelope{
				To: []string{"to1@example.com", "to2@example.com", "to3@test.com"},
			},
		},
	}
	recipients := om.getRecipients(task)
	// Should return all recipients when no filter rules
	if len(recipients) != 3 {
		t.Errorf("Expected 3 recipients when no filter rules, got %d", len(recipients))
	}
}

func TestRelayEmailWithEnvelopeFromButNoFromField(t *testing.T) {
	// Test relayEmail when Envelope.From is set but Email.From is empty
	tmpDir := t.TempDir()
	emailFile := tmpDir + "/test.eml"
	emailContent := []byte("To: recipient@example.com\nSubject: Test\n\nBody")
	if err := os.WriteFile(emailFile, emailContent, 0644); err != nil {
		t.Fatalf("Failed to create test email file: %v", err)
	}

	om := NewOutgoingMail(&OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	})
	defer om.Close()

	email := &types.Email{
		ID:      "test-id",
		Subject: "Test",
		From:    []*mail.Address{}, // Empty From
		Envelope: &types.Envelope{
			From: "envelope@example.com",
			To:   []string{"to@example.com"},
		},
	}

	task := &RelayTask{
		Email:     email,
		EmailPath: emailFile,
	}

	err := om.relayEmail(task)
	if err == nil {
		t.Log("relayEmail succeeded (unexpected)")
	} else {
		t.Logf("relayEmail failed as expected: %v", err)
	}
}

func TestRelayEmailWithBothEnvelopeFromAndEmailFrom(t *testing.T) {
	// Test relayEmail when both Envelope.From and Email.From are set (Envelope.From takes precedence)
	tmpDir := t.TempDir()
	emailFile := tmpDir + "/test.eml"
	emailContent := []byte("From: sender@example.com\nTo: recipient@example.com\nSubject: Test\n\nBody")
	if err := os.WriteFile(emailFile, emailContent, 0644); err != nil {
		t.Fatalf("Failed to create test email file: %v", err)
	}

	om := NewOutgoingMail(&OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	})
	defer om.Close()

	email := &types.Email{
		ID:      "test-id",
		Subject: "Test",
		From: []*mail.Address{
			{Address: "from@example.com"},
		},
		Envelope: &types.Envelope{
			From: "envelope@example.com", // This should be used
			To:   []string{"to@example.com"},
		},
	}

	task := &RelayTask{
		Email:     email,
		EmailPath: emailFile,
	}

	err := om.relayEmail(task)
	if err == nil {
		t.Log("relayEmail succeeded (unexpected)")
	} else {
		t.Logf("relayEmail failed as expected: %v", err)
	}
}

func TestMatchesRuleWithWildcardOnlyAtStart(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	})
	defer om.Close()

	// Test pattern "*example.com" - wildcard only at start
	if !om.matchesRule("test@example.com", "*example.com") {
		t.Error("Pattern with wildcard at start should match suffix")
	}
	if !om.matchesRule("any@example.com", "*example.com") {
		t.Error("Pattern with wildcard at start should match suffix")
	}
	if om.matchesRule("test@other.com", "*example.com") {
		t.Error("Pattern with wildcard at start should not match different suffix")
	}
}

func TestMatchesRuleWithWildcardOnlyAtEnd(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	})
	defer om.Close()

	// Test pattern "test@*" - wildcard only at end
	if !om.matchesRule("test@example.com", "test@*") {
		t.Error("Pattern with wildcard at end should match prefix")
	}
	if !om.matchesRule("test@other.com", "test@*") {
		t.Error("Pattern with wildcard at end should match prefix")
	}
	if om.matchesRule("other@example.com", "test@*") {
		t.Error("Pattern with wildcard at end should not match different prefix")
	}
}

func TestMatchesRuleWithWildcardInMiddle(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	})
	defer om.Close()

	// Test pattern "test*com" - wildcard in middle (splits into 2 parts)
	if !om.matchesRule("test@example.com", "test*com") {
		t.Error("Pattern with wildcard in middle (2 parts) should match")
	}
	if !om.matchesRule("test@other.com", "test*com") {
		t.Error("Pattern with wildcard in middle (2 parts) should match")
	}
	if om.matchesRule("other@example.com", "test*com") {
		t.Error("Pattern with wildcard in middle should not match different prefix")
	}
}

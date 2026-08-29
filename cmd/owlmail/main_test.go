package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-message/mail"
	"github.com/soulteary/cli-kit/testutil"
	"github.com/soulteary/owlmail/internal/common"
	"github.com/soulteary/owlmail/internal/config"
	"github.com/soulteary/owlmail/internal/mailserver"
	webhooknotify "github.com/soulteary/owlmail/internal/webhook"
)

// loggerEventTestMu serializes tests that use common.InitLogger or registerEventHandlers,
// so they do not race with logger-kit/zerolog globals when one test's handler runs
// while another calls InitLogger.
var loggerEventTestMu sync.Mutex

func TestCompleteWebAuthConfig(t *testing.T) {
	if _, err := completeWebAuthConfig(nil, nil); err == nil {
		t.Fatal("nil config should fail")
	}

	t.Run("both omitted keeps authentication disabled", func(t *testing.T) {
		cfg := &config.Config{}
		result, err := completeWebAuthConfig(cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result != (webAuthCompletion{}) || cfg.WebUser != "" || cfg.WebPassword != "" {
			t.Fatalf("unexpected completion: %#v, config: %#v", result, cfg)
		}
	})

	t.Run("username only generates strong password", func(t *testing.T) {
		randomBytes := bytes.Repeat([]byte{0xab}, generatedWebPasswordBytes)
		cfg := &config.Config{WebUser: "operator"}
		result, err := completeWebAuthConfig(cfg, bytes.NewReader(randomBytes))
		if err != nil {
			t.Fatal(err)
		}
		wantPassword := base64.RawURLEncoding.EncodeToString(randomBytes)
		if !result.generatedPassword || result.defaultedUsername {
			t.Fatalf("completion = %#v", result)
		}
		if cfg.WebUser != "operator" || cfg.WebPassword != wantPassword || len(cfg.WebPassword) != 32 {
			t.Fatalf("completed config = %#v", cfg)
		}
	})

	t.Run("password only defaults username", func(t *testing.T) {
		cfg := &config.Config{WebPassword: "configured-secret"}
		result, err := completeWebAuthConfig(cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result.generatedPassword || !result.defaultedUsername {
			t.Fatalf("completion = %#v", result)
		}
		if cfg.WebUser != "admin" || cfg.WebPassword != "configured-secret" {
			t.Fatalf("completed config = %#v", cfg)
		}
	})

	t.Run("complete credentials remain unchanged", func(t *testing.T) {
		cfg := &config.Config{WebUser: "operator", WebPassword: "configured-secret"}
		result, err := completeWebAuthConfig(cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result != (webAuthCompletion{}) || cfg.WebUser != "operator" || cfg.WebPassword != "configured-secret" {
			t.Fatalf("unexpected completion: %#v, config: %#v", result, cfg)
		}
	})

	t.Run("random source failure is fatal", func(t *testing.T) {
		cfg := &config.Config{WebUser: "operator"}
		if _, err := completeWebAuthConfig(cfg, bytes.NewReader([]byte("short"))); err == nil {
			t.Fatal("short random source should fail")
		}
		if cfg.WebPassword != "" {
			t.Fatal("failed generation should not set a partial password")
		}
	})
}

func TestLoadAutoRelayRules(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Test with valid JSON file
	rules := []AutoRelayRule{
		{Allow: "*"},
		{Deny: "*@test.com"},
		{Allow: "ok@test.com"},
	}

	jsonData, err := json.Marshal(rules)
	if err != nil {
		t.Fatalf("Failed to marshal rules: %v", err)
	}

	filePath := filepath.Join(tmpDir, "rules.json")
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		t.Fatalf("Failed to write rules file: %v", err)
	}

	allowRules, denyRules, err := loadAutoRelayRules(filePath)
	if err != nil {
		t.Fatalf("Failed to load rules: %v", err)
	}

	if len(allowRules) != 2 {
		t.Errorf("Expected 2 allow rules, got %d", len(allowRules))
	}
	if len(denyRules) != 1 {
		t.Errorf("Expected 1 deny rule, got %d", len(denyRules))
	}

	if allowRules[0] != "*" {
		t.Errorf("Expected allow rule '*', got '%s'", allowRules[0])
	}
	if allowRules[1] != "ok@test.com" {
		t.Errorf("Expected allow rule 'ok@test.com', got '%s'", allowRules[1])
	}
	if denyRules[0] != "*@test.com" {
		t.Errorf("Expected deny rule '*@test.com', got '%s'", denyRules[0])
	}

	// Test with non-existent file
	_, _, err = loadAutoRelayRules(filepath.Join(tmpDir, "nonexistent.json"))
	if err == nil {
		t.Error("Expected error for non-existent file")
	}

	// Test with invalid JSON
	invalidJSON := []byte("{invalid json}")
	invalidFilePath := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(invalidFilePath, invalidJSON, 0644); err != nil {
		t.Fatalf("Failed to write invalid JSON file: %v", err)
	}

	_, _, err = loadAutoRelayRules(invalidFilePath)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}

	// Test with empty rules
	emptyRules := []AutoRelayRule{}
	emptyJSON, _ := json.Marshal(emptyRules)
	emptyFilePath := filepath.Join(tmpDir, "empty.json")
	if err := os.WriteFile(emptyFilePath, emptyJSON, 0644); err != nil {
		t.Fatalf("Failed to write empty rules file: %v", err)
	}

	allowRules, denyRules, err = loadAutoRelayRules(emptyFilePath)
	if err != nil {
		t.Fatalf("Failed to load empty rules: %v", err)
	}
	if len(allowRules) != 0 {
		t.Errorf("Expected 0 allow rules, got %d", len(allowRules))
	}
	if len(denyRules) != 0 {
		t.Errorf("Expected 0 deny rules, got %d", len(denyRules))
	}
}

func TestLoadAutoRelayRulesOrder(t *testing.T) {
	// Test that rules are processed in order (last matching rule wins)
	tmpDir := t.TempDir()

	rules := []AutoRelayRule{
		{Allow: "*"},
		{Deny: "*@test.com"},
		{Allow: "ok@test.com"},
		{Deny: "ok@test.com"},
		{Allow: "ok@test.com"},
	}

	jsonData, err := json.Marshal(rules)
	if err != nil {
		t.Fatalf("Failed to marshal rules: %v", err)
	}

	filePath := filepath.Join(tmpDir, "rules.json")
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		t.Fatalf("Failed to write rules file: %v", err)
	}

	allowRules, denyRules, err := loadAutoRelayRules(filePath)
	if err != nil {
		t.Fatalf("Failed to load rules: %v", err)
	}

	// Should have all allow and deny rules in order
	if len(allowRules) != 3 {
		t.Errorf("Expected 3 allow rules, got %d", len(allowRules))
	}
	if len(denyRules) != 2 {
		t.Errorf("Expected 2 deny rules, got %d", len(denyRules))
	}

	// Check order
	if allowRules[0] != "*" {
		t.Errorf("Expected first allow rule '*', got '%s'", allowRules[0])
	}
	if allowRules[1] != "ok@test.com" {
		t.Errorf("Expected second allow rule 'ok@test.com', got '%s'", allowRules[1])
	}
	if allowRules[2] != "ok@test.com" {
		t.Errorf("Expected third allow rule 'ok@test.com', got '%s'", allowRules[2])
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		levelStr string
		expected common.LogLevel
	}{
		{"silent", "silent", common.LogLevelSilent},
		{"verbose", "verbose", common.LogLevelVerbose},
		{"normal", "normal", common.LogLevelNormal},
		{"default", "", common.LogLevelNormal},
		{"invalid", "invalid", common.LogLevelNormal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseLogLevel(tt.levelStr)
			if result != tt.expected {
				t.Errorf("parseLogLevel(%q) = %d, want %d", tt.levelStr, result, tt.expected)
			}
		})
	}
}

func TestSetupOutgoingConfig(t *testing.T) {
	// Test with empty outgoing host (should return nil)
	cfg := &config.Config{
		OutgoingHost: "",
	}
	result, err := setupOutgoingConfig(cfg)
	if err != nil {
		t.Errorf("setupOutgoingConfig() error = %v, want nil", err)
	}
	if result != nil {
		t.Errorf("setupOutgoingConfig() = %v, want nil", result)
	}

	// Test with outgoing host set
	cfg = &config.Config{
		OutgoingHost:   "smtp.example.com",
		OutgoingPort:   587,
		OutgoingUser:   "user",
		OutgoingPass:   "pass",
		OutgoingSecure: true,
		AutoRelay:      true,
		AutoRelayAddr:  "relay@example.com",
	}
	result, err = setupOutgoingConfig(cfg)
	if err != nil {
		t.Errorf("setupOutgoingConfig() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("setupOutgoingConfig() = nil, want non-nil")
	}
	if result.Host != "smtp.example.com" {
		t.Errorf("setupOutgoingConfig().Host = %q, want %q", result.Host, "smtp.example.com")
	}
	if result.Port != 587 {
		t.Errorf("setupOutgoingConfig().Port = %d, want %d", result.Port, 587)
	}
	if result.User != "user" {
		t.Errorf("setupOutgoingConfig().User = %q, want %q", result.User, "user")
	}
	if result.Password != "pass" {
		t.Errorf("setupOutgoingConfig().Password = %q, want %q", result.Password, "pass")
	}
	if result.Secure != true {
		t.Errorf("setupOutgoingConfig().Secure = %v, want %v", result.Secure, true)
	}
	if result.AutoRelay != true {
		t.Errorf("setupOutgoingConfig().AutoRelay = %v, want %v", result.AutoRelay, true)
	}
	if result.AutoRelayAddr != "relay@example.com" {
		t.Errorf("setupOutgoingConfig().AutoRelayAddr = %q, want %q", result.AutoRelayAddr, "relay@example.com")
	}

	// Test with auto relay rules file
	tmpDir := t.TempDir()
	rules := []AutoRelayRule{
		{Allow: "*"},
		{Deny: "*@test.com"},
	}
	jsonData, _ := json.Marshal(rules)
	filePath := filepath.Join(tmpDir, "rules.json")
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		t.Fatalf("Failed to write rules file: %v", err)
	}

	cfg = &config.Config{
		OutgoingHost:   "smtp.example.com",
		AutoRelayRules: filePath,
	}
	result, err = setupOutgoingConfig(cfg)
	if err != nil {
		t.Errorf("setupOutgoingConfig() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("setupOutgoingConfig() = nil, want non-nil")
	}
	if len(result.AllowRules) != 1 {
		t.Errorf("setupOutgoingConfig().AllowRules = %v, want 1 rule", result.AllowRules)
	}
	if len(result.DenyRules) != 1 {
		t.Errorf("setupOutgoingConfig().DenyRules = %v, want 1 rule", result.DenyRules)
	}

	// Test with invalid rules file
	cfg = &config.Config{
		OutgoingHost:   "smtp.example.com",
		AutoRelayRules: filepath.Join(tmpDir, "nonexistent.json"),
	}
	_, err = setupOutgoingConfig(cfg)
	if err == nil {
		t.Error("setupOutgoingConfig() error = nil, want error")
	}
}

func TestSetupWebhookDispatcher(t *testing.T) {
	if _, err := setupWebhookDispatcher(nil); err == nil {
		t.Fatal("setupWebhookDispatcher(nil) should fail")
	}

	dispatcher, err := setupWebhookDispatcher(&config.Config{})
	if err != nil || dispatcher != nil {
		t.Fatalf("empty setupWebhookDispatcher() = %v, %v", dispatcher, err)
	}

	filePath := filepath.Join(t.TempDir(), "webhooks.json")
	if err := os.WriteFile(filePath, []byte(`{"version":1,"targets":[{"name":"test","url":"https://example.com/hook"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	dispatcher, err = setupWebhookDispatcher(&config.Config{WebhookConfig: filePath})
	if err != nil {
		t.Fatalf("setupWebhookDispatcher() error = %v", err)
	}
	if dispatcher.TargetCount() != 1 {
		t.Fatalf("TargetCount() = %d", dispatcher.TargetCount())
	}

	if _, err := setupWebhookDispatcher(&config.Config{WebhookConfig: filepath.Join(t.TempDir(), "missing.json")}); err == nil {
		t.Fatal("missing webhook config should fail")
	}
}

func TestRegisterWebhookHandlerDispatchesNewEmail(t *testing.T) {
	received := make(chan struct{}, 1)
	hookServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		received <- struct{}{}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer hookServer.Close()

	dispatcher, err := webhooknotify.NewDispatcher(webhooknotify.Config{Targets: []webhooknotify.Target{{
		Name: "test",
		URL:  hookServer.URL,
	}}}, hookServer.Client())
	if err != nil {
		t.Fatal(err)
	}
	server, err := mailserver.NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	registerWebhookHandler(server, dispatcher)

	email := &mailserver.Email{Subject: "Webhook integration"}
	envelope := &mailserver.Envelope{From: "sender@example.com", To: []string{"recipient@example.com"}}
	if err := server.SaveEmailToStore("webhook-test", false, envelope, email); err != nil {
		t.Fatal(err)
	}
	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("webhook request was not received")
	}
}

func TestRegisterWebhookHandlerHandlesNil(t *testing.T) {
	registerWebhookHandler(nil, nil)
	server, err := mailserver.NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	registerWebhookHandler(server, nil)
}

func TestSetupAuthConfig(t *testing.T) {
	// Test with empty user and password (should return nil)
	cfg := &config.Config{
		SMTPUser:     "",
		SMTPPassword: "",
	}
	result := setupAuthConfig(cfg)
	if result != nil {
		t.Errorf("setupAuthConfig() = %v, want nil", result)
	}

	// Test with empty user (should return nil)
	cfg = &config.Config{
		SMTPUser:     "",
		SMTPPassword: "pass",
	}
	result = setupAuthConfig(cfg)
	if result != nil {
		t.Errorf("setupAuthConfig() = %v, want nil", result)
	}

	// Test with empty password (should return nil)
	cfg = &config.Config{
		SMTPUser:     "user",
		SMTPPassword: "",
	}
	result = setupAuthConfig(cfg)
	if result != nil {
		t.Errorf("setupAuthConfig() = %v, want nil", result)
	}

	// Test with both user and password set
	cfg = &config.Config{
		SMTPUser:     "user",
		SMTPPassword: "pass",
	}
	result = setupAuthConfig(cfg)
	if result == nil {
		t.Fatal("setupAuthConfig() = nil, want non-nil")
	}
	if result.Username != "user" {
		t.Errorf("setupAuthConfig().Username = %q, want %q", result.Username, "user")
	}
	if result.Password != "pass" {
		t.Errorf("setupAuthConfig().Password = %q, want %q", result.Password, "pass")
	}
	if result.Enabled != true {
		t.Errorf("setupAuthConfig().Enabled = %v, want %v", result.Enabled, true)
	}
}

func TestSetupTLSConfig(t *testing.T) {
	// Test with TLS disabled (should return nil)
	cfg := &config.Config{
		TLSEnabled: false,
	}
	result := setupTLSConfig(cfg)
	if result != nil {
		t.Errorf("setupTLSConfig() = %v, want nil", result)
	}

	// Test with TLS enabled
	cfg = &config.Config{
		TLSEnabled:  true,
		TLSCertFile: "/path/to/cert.pem",
		TLSKeyFile:  "/path/to/key.pem",
	}
	result = setupTLSConfig(cfg)
	if result == nil {
		t.Fatal("setupTLSConfig() = nil, want non-nil")
	}
	if result.CertFile != "/path/to/cert.pem" {
		t.Errorf("setupTLSConfig().CertFile = %q, want %q", result.CertFile, "/path/to/cert.pem")
	}
	if result.KeyFile != "/path/to/key.pem" {
		t.Errorf("setupTLSConfig().KeyFile = %q, want %q", result.KeyFile, "/path/to/key.pem")
	}
	if result.Enabled != true {
		t.Errorf("setupTLSConfig().Enabled = %v, want %v", result.Enabled, true)
	}
}

func TestRegisterEventHandlers(t *testing.T) {
	loggerEventTestMu.Lock()
	defer loggerEventTestMu.Unlock()
	// Create a test mail server
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Logf("Failed to close server: %v", err)
		}
	}()

	// Register event handlers
	registerEventHandlers(server)

	// Verify handlers are registered by checking that On can be called without error
	// The actual event triggering is tested in mailserver package
	// Here we just verify that registerEventHandlers doesn't panic
}

func TestStartAPIServer(t *testing.T) {
	// Create a test mail server
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Logf("Failed to close server: %v", err)
		}
	}()

	// Test with nil server
	cfg := &config.Config{
		WebPort: 0,
		WebHost: "localhost",
	}
	_, err = startAPIServer(nil, cfg)
	if err == nil {
		t.Error("startAPIServer() with nil server should return error")
	}

	// Test with nil config
	_, err = startAPIServer(server, nil)
	if err == nil {
		t.Error("startAPIServer() with nil config should return error")
	}

	// Test with HTTPS enabled but empty cert file (should fail immediately)
	cfg = &config.Config{
		WebPort:       0,
		WebHost:       "localhost",
		HTTPSEnabled:  true,
		HTTPSCertFile: "",
		HTTPSKeyFile:  "",
	}

	errChan := make(chan error, 1)
	go func() {
		_, startErr := startAPIServer(server, cfg)
		errChan <- startErr
	}()

	select {
	case err := <-errChan:
		if err == nil {
			t.Error("startAPIServer with HTTPS (empty cert) should return error")
		} else {
			t.Logf("startAPIServer with HTTPS (empty cert) failed as expected: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("startAPIServer with HTTPS (empty cert) should fail immediately, not timeout")
	}
}

func TestRegisterEventHandlersWithNilServer(t *testing.T) {
	// Test that registerEventHandlers handles nil server gracefully
	registerEventHandlers(nil)
	// Should not panic
}

func TestSetupGracefulShutdownWithNilServer(t *testing.T) {
	// Test that setupGracefulShutdown handles nil server gracefully
	setupGracefulShutdown(nil)
	// Should not panic
}

// TestRegisterEventHandlersWithEvents tests that event handlers are actually called when events are triggered
func TestRegisterEventHandlersWithEvents(t *testing.T) {
	loggerEventTestMu.Lock()
	defer loggerEventTestMu.Unlock()
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Logf("Failed to close server: %v", err)
		}
	}()

	// Track if events were fired
	newEventFired := make(chan bool, 1)
	deleteEventFired := make(chan bool, 1)

	// Register event handlers
	registerEventHandlers(server)

	// Add custom handlers to track events
	server.On("new", func(email *mailserver.Email) {
		newEventFired <- true
	})

	server.On("delete", func(email *mailserver.Email) {
		deleteEventFired <- true
	})

	// Create a test email and save it to trigger "new" event
	testEmail := &mailserver.Email{
		ID:      "test-email-id",
		Subject: "Test Subject",
		From:    []*mail.Address{{Address: "test@example.com"}},
		To:      []*mail.Address{{Address: "recipient@example.com"}},
		Text:    "Test email body",
	}

	// Create envelope for the email
	envelope := &mailserver.Envelope{
		From: "test@example.com",
		To:   []string{"recipient@example.com"},
	}

	// Save email to trigger "new" event
	if err := server.SaveEmailToStore("test-email-id", false, envelope, testEmail); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	// Wait for "new" event handler to be called
	select {
	case <-newEventFired:
		// Event handler was called
	case <-time.After(2 * time.Second):
		t.Error("'new' event handler should have been called")
	}

	// Delete email to trigger "delete" event
	if err := server.DeleteEmail(testEmail.ID); err != nil {
		t.Fatalf("Failed to delete email: %v", err)
	}

	// Wait for "delete" event handler to be called
	select {
	case <-deleteEventFired:
		// Event handler was called
	case <-time.After(2 * time.Second):
		t.Error("'delete' event handler should have been called")
	}

	// Wait for registerEventHandlers' handlers (common.Log/Verbose) to finish
	// before releasing loggerEventTestMu.
	time.Sleep(500 * time.Millisecond)
}

// TestSetupGracefulShutdown tests the graceful shutdown mechanism
func TestSetupGracefulShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Logf("Failed to close server: %v", err)
		}
	}()

	// Setup graceful shutdown
	// This sets up signal handlers but doesn't block
	setupGracefulShutdown(server)

	// Give it a moment to set up signal handlers
	time.Sleep(50 * time.Millisecond)

	// Note: We can't easily test the actual shutdown behavior without
	// potentially affecting the test process, so we just verify it doesn't panic
}

// TestInitializeApplication tests the initializeApplication function
func TestInitializeApplication(t *testing.T) {
	loggerEventTestMu.Lock()
	defer loggerEventTestMu.Unlock()
	// Test with nil config
	err := initializeApplication(nil)
	if err == nil {
		t.Error("initializeApplication with nil config should return error")
	}

	// Test with valid config
	cfg := &config.Config{
		LogLevel: "verbose",
	}
	err = initializeApplication(cfg)
	if err != nil {
		t.Errorf("initializeApplication() error = %v, want nil", err)
	}

	// Test with different log levels
	testCases := []struct {
		name     string
		logLevel string
	}{
		{"silent", "silent"},
		{"normal", "normal"},
		{"verbose", "verbose"},
		{"invalid", "invalid"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{LogLevel: tc.logLevel}
			err := initializeApplication(cfg)
			if err != nil {
				t.Errorf("initializeApplication() error = %v, want nil", err)
			}
		})
	}
}

// TestCreateMailServer tests the createMailServer function
func TestCreateMailServer(t *testing.T) {
	// Test with nil config
	_, err := createMailServer(nil)
	if err == nil {
		t.Error("createMailServer with nil config should return error")
	}

	// Test with valid config (no outgoing host)
	tmpDir1 := t.TempDir()
	cfg := &config.Config{
		SMTPPort:          1025,
		SMTPHost:          "localhost",
		MailDir:           tmpDir1,
		OutgoingHost:      "", // No outgoing host
		UseUUIDForEmailID: false,
	}

	server, err := createMailServer(cfg)
	if err != nil {
		t.Fatalf("createMailServer() error = %v, want nil", err)
	}
	if server == nil {
		t.Fatal("createMailServer() = nil, want non-nil")
	}
	defer func() {
		if server != nil {
			if err := server.Close(); err != nil {
				t.Logf("Failed to close server: %v", err)
			}
		}
	}()

	// Test with outgoing host configured
	tmpDir2 := t.TempDir()
	cfg = &config.Config{
		SMTPPort:          1026,
		SMTPHost:          "localhost",
		MailDir:           tmpDir2,
		OutgoingHost:      "smtp.example.com",
		OutgoingPort:      587,
		OutgoingUser:      "user",
		OutgoingPass:      "pass",
		OutgoingSecure:    true,
		UseUUIDForEmailID: false,
	}

	server2, err := createMailServer(cfg)
	if err != nil {
		t.Fatalf("createMailServer() with outgoing config error = %v, want nil", err)
	}
	if server2 == nil {
		t.Fatal("createMailServer() = nil, want non-nil")
	}
	defer func() {
		if server2 != nil {
			if err := server2.Close(); err != nil {
				t.Logf("Failed to close server: %v", err)
			}
		}
	}()

	// Test with SMTP authentication
	tmpDir3 := t.TempDir()
	cfg = &config.Config{
		SMTPPort:          1027,
		SMTPHost:          "localhost",
		MailDir:           tmpDir3,
		SMTPUser:          "smtpuser",
		SMTPPassword:      "smtppass",
		UseUUIDForEmailID: false,
	}

	server3, err := createMailServer(cfg)
	if err != nil {
		t.Fatalf("createMailServer() with auth config error = %v, want nil", err)
	}
	if server3 == nil {
		t.Fatal("createMailServer() = nil, want non-nil")
	}
	defer func() {
		if server3 != nil {
			if err := server3.Close(); err != nil {
				t.Logf("Failed to close server: %v", err)
			}
		}
	}()

	// Test with invalid outgoing config (invalid rules file)
	tmpDir5 := t.TempDir()
	cfg = &config.Config{
		SMTPPort:          1029,
		SMTPHost:          "localhost",
		MailDir:           tmpDir5,
		OutgoingHost:      "smtp.example.com",
		AutoRelayRules:    "/nonexistent/rules.json",
		UseUUIDForEmailID: false,
	}

	_, err = createMailServer(cfg)
	if err == nil {
		t.Error("createMailServer() with invalid rules file should return error")
	}
}

// TestStartServers tests the startServers function
func TestStartServers(t *testing.T) {
	// Test with nil server
	cfg := &config.Config{
		WebPort: 1080,
		WebHost: "localhost",
	}
	err := startServers(nil, cfg)
	if err == nil {
		t.Error("startServers with nil server should return error")
	}

	// Test with nil config
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Logf("Failed to close server: %v", err)
		}
	}()

	err = startServers(server, nil)
	if err == nil {
		t.Error("startServers with nil config should return error")
	}
}

// TestConfigPackageIntegration tests that the config package works correctly with main
func TestConfigPackageIntegration(t *testing.T) {
	envMgr := testutil.NewEnvManager()
	defer envMgr.Cleanup()

	// Test with MAILDEV compatibility
	_ = envMgr.Set("MAILDEV_SMTP_PORT", "2025")
	_ = envMgr.Set("MAILDEV_IP", "0.0.0.0")
	_ = envMgr.Set("MAILDEV_VERBOSE", "1")

	// Use the config package's ResolveLogLevel
	logLevel := config.ResolveLogLevel(nil, "log-level", "normal")
	if logLevel != "verbose" {
		t.Errorf("Expected log level 'verbose', got '%s'", logLevel)
	}
}

// TestRegisterEventHandlersWithEmptyEmail tests event handlers with email that has empty fields
func TestRegisterEventHandlersWithEmptyEmail(t *testing.T) {
	loggerEventTestMu.Lock()
	defer loggerEventTestMu.Unlock()
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Logf("Failed to close server: %v", err)
		}
	}()

	// Register event handlers
	registerEventHandlers(server)

	// Create email with empty subject and no from address
	testEmail := &mailserver.Email{
		ID:      "test-empty-email-id",
		Subject: "",                // Empty subject
		From:    []*mail.Address{}, // Empty from
		Text:    "Test email body",
	}

	// Create envelope for the email
	envelope := &mailserver.Envelope{
		From: "",
		To:   []string{},
	}

	// Save email to trigger "new" event
	if err := server.SaveEmailToStore("test-empty-email-id", false, envelope, testEmail); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	// Give handlers time to process
	time.Sleep(100 * time.Millisecond)

	// Delete email to trigger "delete" event
	if err := server.DeleteEmail(testEmail.ID); err != nil {
		t.Fatalf("Failed to delete email: %v", err)
	}

	// Wait for async handlers to finish before returning (and releasing loggerEventTestMu).
	// InitLogger in the next test must not run while handlers are still in Log/Verbose.
	time.Sleep(500 * time.Millisecond)
}

// TestRegisterEventHandlersWithVerboseLogging tests event handlers with verbose logging enabled
func TestRegisterEventHandlersWithVerboseLogging(t *testing.T) {
	loggerEventTestMu.Lock()
	defer loggerEventTestMu.Unlock()
	// Set verbose logging. We do not reset the logger in a defer because
	// InitLogger calls logger-kit New(), which writes zerolog globals, and
	// event handlers call Verbose()/Log() from other goroutines. Resetting
	// here would introduce a data race. Other tests that need a specific
	// level should call common.InitLogger at their start.
	common.InitLogger(common.LogLevelVerbose)

	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Logf("Failed to close server: %v", err)
		}
	}()

	// Register event handlers
	registerEventHandlers(server)

	// Create email with attachments to trigger verbose logging
	testEmail := &mailserver.Email{
		ID:        "test-verbose-email-id",
		Subject:   "Test Subject",
		From:      []*mail.Address{{Address: "test@example.com"}},
		To:        []*mail.Address{{Address: "recipient@example.com"}},
		Text:      "Test email body",
		SizeHuman: "1.5 KB",
		Attachments: []*mailserver.Attachment{
			{FileName: "test.txt", ContentType: "text/plain"},
		},
	}

	// Create envelope for the email
	envelope := &mailserver.Envelope{
		From: "test@example.com",
		To:   []string{"recipient@example.com"},
	}

	// Save email to trigger "new" event with verbose logging
	if err := server.SaveEmailToStore("test-verbose-email-id", false, envelope, testEmail); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	// Give handlers time to process
	time.Sleep(200 * time.Millisecond)

	// Delete email to trigger "delete" event with verbose logging
	if err := server.DeleteEmail(testEmail.ID); err != nil {
		t.Fatalf("Failed to delete email: %v", err)
	}

	// Give handlers time to process before test ends
	time.Sleep(200 * time.Millisecond)
}

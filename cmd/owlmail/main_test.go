package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/soulteary/owlmail/internal/common"
	"github.com/soulteary/owlmail/internal/mailserver"
)

func TestGetEnvString(t *testing.T) {
	// Test with environment variable set
	if err := os.Setenv("TEST_VAR", "test-value"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("TEST_VAR"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()

	result := getEnvString("TEST_VAR", "default")
	if result != "test-value" {
		t.Errorf("Expected 'test-value', got '%s'", result)
	}

	// Test with environment variable not set
	if err := os.Unsetenv("TEST_VAR"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	result = getEnvString("TEST_VAR", "default")
	if result != "default" {
		t.Errorf("Expected 'default', got '%s'", result)
	}

	// Test with empty environment variable
	if err := os.Setenv("TEST_VAR", ""); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("TEST_VAR"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()
	result = getEnvString("TEST_VAR", "default")
	if result != "default" {
		t.Errorf("Expected 'default' for empty env var, got '%s'", result)
	}
}

func TestGetEnvInt(t *testing.T) {
	// Test with valid integer
	if err := os.Setenv("TEST_INT", "123"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("TEST_INT"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()

	result := getEnvInt("TEST_INT", 0)
	if result != 123 {
		t.Errorf("Expected 123, got %d", result)
	}

	// Test with environment variable not set
	if err := os.Unsetenv("TEST_INT"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	result = getEnvInt("TEST_INT", 456)
	if result != 456 {
		t.Errorf("Expected 456, got %d", result)
	}

	// Test with invalid integer
	if err := os.Setenv("TEST_INT", "invalid"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("TEST_INT"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()
	result = getEnvInt("TEST_INT", 789)
	if result != 789 {
		t.Errorf("Expected 789 for invalid int, got %d", result)
	}

	// Test with empty environment variable
	if err := os.Setenv("TEST_INT", ""); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("TEST_INT"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()
	result = getEnvInt("TEST_INT", 999)
	if result != 999 {
		t.Errorf("Expected 999 for empty env var, got %d", result)
	}
}

func TestGetEnvBool(t *testing.T) {
	// Test with "true"
	if err := os.Setenv("TEST_BOOL", "true"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("TEST_BOOL"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()

	result := getEnvBool("TEST_BOOL", false)
	if result != true {
		t.Errorf("Expected true, got %v", result)
	}

	// Test with "false"
	if err := os.Setenv("TEST_BOOL", "false"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	result = getEnvBool("TEST_BOOL", true)
	if result != false {
		t.Errorf("Expected false, got %v", result)
	}

	// Test with "1"
	if err := os.Setenv("TEST_BOOL", "1"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	result = getEnvBool("TEST_BOOL", false)
	if result != true {
		t.Errorf("Expected true for '1', got %v", result)
	}

	// Test with "0"
	if err := os.Setenv("TEST_BOOL", "0"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	result = getEnvBool("TEST_BOOL", true)
	if result != false {
		t.Errorf("Expected false for '0', got %v", result)
	}

	// Test with environment variable not set
	if err := os.Unsetenv("TEST_BOOL"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	result = getEnvBool("TEST_BOOL", true)
	if result != true {
		t.Errorf("Expected true (default), got %v", result)
	}

	// Test with invalid boolean
	if err := os.Setenv("TEST_BOOL", "invalid"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("TEST_BOOL"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()
	result = getEnvBool("TEST_BOOL", false)
	if result != false {
		t.Errorf("Expected false for invalid bool, got %v", result)
	}
}

func TestGetLogLevelFromEnv(t *testing.T) {
	// Test with MAILDEV_VERBOSE
	if err := os.Setenv("MAILDEV_VERBOSE", "1"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("MAILDEV_VERBOSE"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()
	if err := os.Unsetenv("MAILDEV_SILENT"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	if err := os.Unsetenv("OWLMAIL_LOG_LEVEL"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}

	result := getLogLevelFromEnv()
	if result != common.LogLevelVerbose {
		t.Errorf("Expected LogLevelVerbose, got %d", result)
	}

	// Test with MAILDEV_SILENT
	if err := os.Unsetenv("MAILDEV_VERBOSE"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	if err := os.Setenv("MAILDEV_SILENT", "1"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("MAILDEV_SILENT"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()

	result = getLogLevelFromEnv()
	if result != common.LogLevelSilent {
		t.Errorf("Expected LogLevelSilent, got %d", result)
	}

	// Test with OWLMAIL_LOG_LEVEL=verbose
	if err := os.Unsetenv("MAILDEV_VERBOSE"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	if err := os.Unsetenv("MAILDEV_SILENT"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	if err := os.Setenv("OWLMAIL_LOG_LEVEL", "verbose"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("OWLMAIL_LOG_LEVEL"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()

	result = getLogLevelFromEnv()
	if result != common.LogLevelVerbose {
		t.Errorf("Expected LogLevelVerbose, got %d", result)
	}

	// Test with OWLMAIL_LOG_LEVEL=silent
	if err := os.Setenv("OWLMAIL_LOG_LEVEL", "silent"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	result = getLogLevelFromEnv()
	if result != common.LogLevelSilent {
		t.Errorf("Expected LogLevelSilent, got %d", result)
	}

	// Test with default
	if err := os.Unsetenv("MAILDEV_VERBOSE"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	if err := os.Unsetenv("MAILDEV_SILENT"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	if err := os.Unsetenv("OWLMAIL_LOG_LEVEL"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}

	result = getLogLevelFromEnv()
	if result != common.LogLevelNormal {
		t.Errorf("Expected LogLevelNormal, got %d", result)
	}
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
	cfg := &Config{
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
	cfg = &Config{
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

	cfg = &Config{
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
	cfg = &Config{
		OutgoingHost:   "smtp.example.com",
		AutoRelayRules: filepath.Join(tmpDir, "nonexistent.json"),
	}
	_, err = setupOutgoingConfig(cfg)
	if err == nil {
		t.Error("setupOutgoingConfig() error = nil, want error")
	}
}

func TestSetupAuthConfig(t *testing.T) {
	// Test with empty user and password (should return nil)
	cfg := &Config{
		SMTPUser:     "",
		SMTPPassword: "",
	}
	result := setupAuthConfig(cfg)
	if result != nil {
		t.Errorf("setupAuthConfig() = %v, want nil", result)
	}

	// Test with empty user (should return nil)
	cfg = &Config{
		SMTPUser:     "",
		SMTPPassword: "pass",
	}
	result = setupAuthConfig(cfg)
	if result != nil {
		t.Errorf("setupAuthConfig() = %v, want nil", result)
	}

	// Test with empty password (should return nil)
	cfg = &Config{
		SMTPUser:     "user",
		SMTPPassword: "",
	}
	result = setupAuthConfig(cfg)
	if result != nil {
		t.Errorf("setupAuthConfig() = %v, want nil", result)
	}

	// Test with both user and password set
	cfg = &Config{
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
	cfg := &Config{
		TLSEnabled: false,
	}
	result := setupTLSConfig(cfg)
	if result != nil {
		t.Errorf("setupTLSConfig() = %v, want nil", result)
	}

	// Test with TLS enabled
	cfg = &Config{
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

func TestParseConfig(t *testing.T) {
	// Save original os.Args and flag.CommandLine
	originalArgs := os.Args
	originalCommandLine := flag.CommandLine

	// Helper function to reset flag state
	resetFlags := func() {
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}

	// Helper function to clear environment variables
	clearEnvVars := func() {
		envVars := []string{
			"OWLMAIL_SMTP_PORT", "MAILDEV_SMTP_PORT",
			"OWLMAIL_SMTP_HOST", "MAILDEV_IP",
			"OWLMAIL_MAIL_DIR", "MAILDEV_MAIL_DIRECTORY",
			"OWLMAIL_WEB_PORT", "MAILDEV_WEB_PORT",
			"OWLMAIL_WEB_HOST", "MAILDEV_WEB_IP",
			"OWLMAIL_WEB_USER", "MAILDEV_WEB_USER",
			"OWLMAIL_WEB_PASSWORD", "MAILDEV_WEB_PASS",
			"OWLMAIL_HTTPS_ENABLED", "MAILDEV_HTTPS",
			"OWLMAIL_HTTPS_CERT", "MAILDEV_HTTPS_CERT",
			"OWLMAIL_HTTPS_KEY", "MAILDEV_HTTPS_KEY",
			"OWLMAIL_OUTGOING_HOST", "MAILDEV_OUTGOING_HOST",
			"OWLMAIL_OUTGOING_PORT", "MAILDEV_OUTGOING_PORT",
			"OWLMAIL_OUTGOING_USER", "MAILDEV_OUTGOING_USER",
			"OWLMAIL_OUTGOING_PASSWORD", "MAILDEV_OUTGOING_PASS",
			"OWLMAIL_OUTGOING_SECURE", "MAILDEV_OUTGOING_SECURE",
			"OWLMAIL_AUTO_RELAY", "MAILDEV_AUTO_RELAY",
			"OWLMAIL_AUTO_RELAY_ADDR", "MAILDEV_AUTO_RELAY_ADDR",
			"OWLMAIL_AUTO_RELAY_RULES", "MAILDEV_AUTO_RELAY_RULES",
			"OWLMAIL_SMTP_USER", "MAILDEV_INCOMING_USER",
			"OWLMAIL_SMTP_PASSWORD", "MAILDEV_INCOMING_PASS",
			"OWLMAIL_TLS_ENABLED", "MAILDEV_INCOMING_SECURE",
			"OWLMAIL_TLS_CERT", "MAILDEV_INCOMING_CERT",
			"OWLMAIL_TLS_KEY", "MAILDEV_INCOMING_KEY",
			"OWLMAIL_LOG_LEVEL", "MAILDEV_VERBOSE", "MAILDEV_SILENT",
		}
		for _, envVar := range envVars {
			_ = os.Unsetenv(envVar)
		}
	}

	// Helper function to restore original state
	restoreState := func() {
		os.Args = originalArgs
		flag.CommandLine = originalCommandLine
		clearEnvVars()
	}

	// Always restore state at the end
	defer restoreState()

	t.Run("default values", func(t *testing.T) {
		resetFlags()
		clearEnvVars()
		os.Args = []string{"owlmail"}
		cfg := parseConfig()

		// Check default values
		if cfg.SMTPPort != 1025 {
			t.Errorf("Expected SMTPPort 1025, got %d", cfg.SMTPPort)
		}
		if cfg.SMTPHost != "localhost" {
			t.Errorf("Expected SMTPHost 'localhost', got '%s'", cfg.SMTPHost)
		}
		if cfg.MailDir != "" {
			t.Errorf("Expected MailDir '', got '%s'", cfg.MailDir)
		}
		if cfg.WebPort != 1080 {
			t.Errorf("Expected WebPort 1080, got %d", cfg.WebPort)
		}
		if cfg.WebHost != "localhost" {
			t.Errorf("Expected WebHost 'localhost', got '%s'", cfg.WebHost)
		}
		if cfg.WebUser != "" {
			t.Errorf("Expected WebUser '', got '%s'", cfg.WebUser)
		}
		if cfg.WebPassword != "" {
			t.Errorf("Expected WebPassword '', got '%s'", cfg.WebPassword)
		}
		if cfg.HTTPSEnabled != false {
			t.Errorf("Expected HTTPSEnabled false, got %v", cfg.HTTPSEnabled)
		}
		if cfg.OutgoingPort != 587 {
			t.Errorf("Expected OutgoingPort 587, got %d", cfg.OutgoingPort)
		}
		if cfg.OutgoingSecure != false {
			t.Errorf("Expected OutgoingSecure false, got %v", cfg.OutgoingSecure)
		}
		if cfg.AutoRelay != false {
			t.Errorf("Expected AutoRelay false, got %v", cfg.AutoRelay)
		}
		if cfg.TLSEnabled != false {
			t.Errorf("Expected TLSEnabled false, got %v", cfg.TLSEnabled)
		}
		if cfg.LogLevel != "normal" {
			t.Errorf("Expected LogLevel 'normal', got '%s'", cfg.LogLevel)
		}
	})

	t.Run("environment variables - OWLMAIL_*", func(t *testing.T) {
		resetFlags()
		clearEnvVars()
		os.Args = []string{"owlmail"}

		// Set environment variables
		_ = os.Setenv("OWLMAIL_SMTP_PORT", "2025")
		_ = os.Setenv("OWLMAIL_SMTP_HOST", "0.0.0.0")
		_ = os.Setenv("OWLMAIL_MAIL_DIR", "/tmp/mail")
		_ = os.Setenv("OWLMAIL_WEB_PORT", "2080")
		_ = os.Setenv("OWLMAIL_WEB_HOST", "127.0.0.1")
		_ = os.Setenv("OWLMAIL_WEB_USER", "testuser")
		_ = os.Setenv("OWLMAIL_WEB_PASSWORD", "testpass")
		_ = os.Setenv("OWLMAIL_HTTPS_ENABLED", "true")
		_ = os.Setenv("OWLMAIL_HTTPS_CERT", "/path/to/cert.pem")
		_ = os.Setenv("OWLMAIL_HTTPS_KEY", "/path/to/key.pem")
		_ = os.Setenv("OWLMAIL_OUTGOING_HOST", "smtp.example.com")
		_ = os.Setenv("OWLMAIL_OUTGOING_PORT", "465")
		_ = os.Setenv("OWLMAIL_OUTGOING_USER", "outuser")
		_ = os.Setenv("OWLMAIL_OUTGOING_PASSWORD", "outpass")
		_ = os.Setenv("OWLMAIL_OUTGOING_SECURE", "true")
		_ = os.Setenv("OWLMAIL_AUTO_RELAY", "true")
		_ = os.Setenv("OWLMAIL_AUTO_RELAY_ADDR", "relay@example.com")
		_ = os.Setenv("OWLMAIL_AUTO_RELAY_RULES", "/path/to/rules.json")
		_ = os.Setenv("OWLMAIL_SMTP_USER", "smtpuser")
		_ = os.Setenv("OWLMAIL_SMTP_PASSWORD", "smtppass")
		_ = os.Setenv("OWLMAIL_TLS_ENABLED", "true")
		_ = os.Setenv("OWLMAIL_TLS_CERT", "/path/to/tls-cert.pem")
		_ = os.Setenv("OWLMAIL_TLS_KEY", "/path/to/tls-key.pem")
		_ = os.Setenv("OWLMAIL_LOG_LEVEL", "verbose")

		defer func() {
			_ = os.Unsetenv("OWLMAIL_SMTP_PORT")
			_ = os.Unsetenv("OWLMAIL_SMTP_HOST")
			_ = os.Unsetenv("OWLMAIL_MAIL_DIR")
			_ = os.Unsetenv("OWLMAIL_WEB_PORT")
			_ = os.Unsetenv("OWLMAIL_WEB_HOST")
			_ = os.Unsetenv("OWLMAIL_WEB_USER")
			_ = os.Unsetenv("OWLMAIL_WEB_PASSWORD")
			_ = os.Unsetenv("OWLMAIL_HTTPS_ENABLED")
			_ = os.Unsetenv("OWLMAIL_HTTPS_CERT")
			_ = os.Unsetenv("OWLMAIL_HTTPS_KEY")
			_ = os.Unsetenv("OWLMAIL_OUTGOING_HOST")
			_ = os.Unsetenv("OWLMAIL_OUTGOING_PORT")
			_ = os.Unsetenv("OWLMAIL_OUTGOING_USER")
			_ = os.Unsetenv("OWLMAIL_OUTGOING_PASSWORD")
			_ = os.Unsetenv("OWLMAIL_OUTGOING_SECURE")
			_ = os.Unsetenv("OWLMAIL_AUTO_RELAY")
			_ = os.Unsetenv("OWLMAIL_AUTO_RELAY_ADDR")
			_ = os.Unsetenv("OWLMAIL_AUTO_RELAY_RULES")
			_ = os.Unsetenv("OWLMAIL_SMTP_USER")
			_ = os.Unsetenv("OWLMAIL_SMTP_PASSWORD")
			_ = os.Unsetenv("OWLMAIL_TLS_ENABLED")
			_ = os.Unsetenv("OWLMAIL_TLS_CERT")
			_ = os.Unsetenv("OWLMAIL_TLS_KEY")
			_ = os.Unsetenv("OWLMAIL_LOG_LEVEL")
		}()

		cfg := parseConfig()

		if cfg.SMTPPort != 2025 {
			t.Errorf("Expected SMTPPort 2025, got %d", cfg.SMTPPort)
		}
		if cfg.SMTPHost != "0.0.0.0" {
			t.Errorf("Expected SMTPHost '0.0.0.0', got '%s'", cfg.SMTPHost)
		}
		if cfg.MailDir != "/tmp/mail" {
			t.Errorf("Expected MailDir '/tmp/mail', got '%s'", cfg.MailDir)
		}
		if cfg.WebPort != 2080 {
			t.Errorf("Expected WebPort 2080, got %d", cfg.WebPort)
		}
		if cfg.WebHost != "127.0.0.1" {
			t.Errorf("Expected WebHost '127.0.0.1', got '%s'", cfg.WebHost)
		}
		if cfg.WebUser != "testuser" {
			t.Errorf("Expected WebUser 'testuser', got '%s'", cfg.WebUser)
		}
		if cfg.WebPassword != "testpass" {
			t.Errorf("Expected WebPassword 'testpass', got '%s'", cfg.WebPassword)
		}
		if cfg.HTTPSEnabled != true {
			t.Errorf("Expected HTTPSEnabled true, got %v", cfg.HTTPSEnabled)
		}
		if cfg.HTTPSCertFile != "/path/to/cert.pem" {
			t.Errorf("Expected HTTPSCertFile '/path/to/cert.pem', got '%s'", cfg.HTTPSCertFile)
		}
		if cfg.HTTPSKeyFile != "/path/to/key.pem" {
			t.Errorf("Expected HTTPSKeyFile '/path/to/key.pem', got '%s'", cfg.HTTPSKeyFile)
		}
		if cfg.OutgoingHost != "smtp.example.com" {
			t.Errorf("Expected OutgoingHost 'smtp.example.com', got '%s'", cfg.OutgoingHost)
		}
		if cfg.OutgoingPort != 465 {
			t.Errorf("Expected OutgoingPort 465, got %d", cfg.OutgoingPort)
		}
		if cfg.OutgoingUser != "outuser" {
			t.Errorf("Expected OutgoingUser 'outuser', got '%s'", cfg.OutgoingUser)
		}
		if cfg.OutgoingPass != "outpass" {
			t.Errorf("Expected OutgoingPass 'outpass', got '%s'", cfg.OutgoingPass)
		}
		if cfg.OutgoingSecure != true {
			t.Errorf("Expected OutgoingSecure true, got %v", cfg.OutgoingSecure)
		}
		if cfg.AutoRelay != true {
			t.Errorf("Expected AutoRelay true, got %v", cfg.AutoRelay)
		}
		if cfg.AutoRelayAddr != "relay@example.com" {
			t.Errorf("Expected AutoRelayAddr 'relay@example.com', got '%s'", cfg.AutoRelayAddr)
		}
		if cfg.AutoRelayRules != "/path/to/rules.json" {
			t.Errorf("Expected AutoRelayRules '/path/to/rules.json', got '%s'", cfg.AutoRelayRules)
		}
		if cfg.SMTPUser != "smtpuser" {
			t.Errorf("Expected SMTPUser 'smtpuser', got '%s'", cfg.SMTPUser)
		}
		if cfg.SMTPPassword != "smtppass" {
			t.Errorf("Expected SMTPPassword 'smtppass', got '%s'", cfg.SMTPPassword)
		}
		if cfg.TLSEnabled != true {
			t.Errorf("Expected TLSEnabled true, got %v", cfg.TLSEnabled)
		}
		if cfg.TLSCertFile != "/path/to/tls-cert.pem" {
			t.Errorf("Expected TLSCertFile '/path/to/tls-cert.pem', got '%s'", cfg.TLSCertFile)
		}
		if cfg.TLSKeyFile != "/path/to/tls-key.pem" {
			t.Errorf("Expected TLSKeyFile '/path/to/tls-key.pem', got '%s'", cfg.TLSKeyFile)
		}
		if cfg.LogLevel != "verbose" {
			t.Errorf("Expected LogLevel 'verbose', got '%s'", cfg.LogLevel)
		}
	})

	t.Run("environment variables - MAILDEV_* compatibility", func(t *testing.T) {
		resetFlags()
		clearEnvVars()
		os.Args = []string{"owlmail"}

		// Set MailDev environment variables (should take precedence over OWLMAIL_*)
		_ = os.Setenv("MAILDEV_SMTP_PORT", "3025")
		_ = os.Setenv("MAILDEV_IP", "192.168.1.1")
		_ = os.Setenv("MAILDEV_MAIL_DIRECTORY", "/tmp/maildev")
		_ = os.Setenv("MAILDEV_WEB_PORT", "3080")
		_ = os.Setenv("MAILDEV_WEB_IP", "192.168.1.2")
		_ = os.Setenv("MAILDEV_WEB_USER", "maildevuser")
		_ = os.Setenv("MAILDEV_WEB_PASS", "maildevpass")
		_ = os.Setenv("MAILDEV_HTTPS", "true")
		_ = os.Setenv("MAILDEV_HTTPS_CERT", "/path/to/maildev-cert.pem")
		_ = os.Setenv("MAILDEV_HTTPS_KEY", "/path/to/maildev-key.pem")
		_ = os.Setenv("MAILDEV_OUTGOING_HOST", "smtp.maildev.com")
		_ = os.Setenv("MAILDEV_OUTGOING_PORT", "25")
		_ = os.Setenv("MAILDEV_OUTGOING_USER", "maildevout")
		_ = os.Setenv("MAILDEV_OUTGOING_PASS", "maildevoutpass")
		_ = os.Setenv("MAILDEV_OUTGOING_SECURE", "false")
		_ = os.Setenv("MAILDEV_AUTO_RELAY", "false")
		_ = os.Setenv("MAILDEV_AUTO_RELAY_ADDR", "maildev@example.com")
		_ = os.Setenv("MAILDEV_AUTO_RELAY_RULES", "/path/to/maildev-rules.json")
		_ = os.Setenv("MAILDEV_INCOMING_USER", "maildevsmtp")
		_ = os.Setenv("MAILDEV_INCOMING_PASS", "maildevsmtppass")
		_ = os.Setenv("MAILDEV_INCOMING_SECURE", "true")
		_ = os.Setenv("MAILDEV_INCOMING_CERT", "/path/to/maildev-tls-cert.pem")
		_ = os.Setenv("MAILDEV_INCOMING_KEY", "/path/to/maildev-tls-key.pem")
		_ = os.Setenv("MAILDEV_VERBOSE", "1")

		defer func() {
			_ = os.Unsetenv("MAILDEV_SMTP_PORT")
			_ = os.Unsetenv("MAILDEV_IP")
			_ = os.Unsetenv("MAILDEV_MAIL_DIRECTORY")
			_ = os.Unsetenv("MAILDEV_WEB_PORT")
			_ = os.Unsetenv("MAILDEV_WEB_IP")
			_ = os.Unsetenv("MAILDEV_WEB_USER")
			_ = os.Unsetenv("MAILDEV_WEB_PASS")
			_ = os.Unsetenv("MAILDEV_HTTPS")
			_ = os.Unsetenv("MAILDEV_HTTPS_CERT")
			_ = os.Unsetenv("MAILDEV_HTTPS_KEY")
			_ = os.Unsetenv("MAILDEV_OUTGOING_HOST")
			_ = os.Unsetenv("MAILDEV_OUTGOING_PORT")
			_ = os.Unsetenv("MAILDEV_OUTGOING_USER")
			_ = os.Unsetenv("MAILDEV_OUTGOING_PASS")
			_ = os.Unsetenv("MAILDEV_OUTGOING_SECURE")
			_ = os.Unsetenv("MAILDEV_AUTO_RELAY")
			_ = os.Unsetenv("MAILDEV_AUTO_RELAY_ADDR")
			_ = os.Unsetenv("MAILDEV_AUTO_RELAY_RULES")
			_ = os.Unsetenv("MAILDEV_INCOMING_USER")
			_ = os.Unsetenv("MAILDEV_INCOMING_PASS")
			_ = os.Unsetenv("MAILDEV_INCOMING_SECURE")
			_ = os.Unsetenv("MAILDEV_INCOMING_CERT")
			_ = os.Unsetenv("MAILDEV_INCOMING_KEY")
			_ = os.Unsetenv("MAILDEV_VERBOSE")
		}()

		cfg := parseConfig()

		if cfg.SMTPPort != 3025 {
			t.Errorf("Expected SMTPPort 3025, got %d", cfg.SMTPPort)
		}
		if cfg.SMTPHost != "192.168.1.1" {
			t.Errorf("Expected SMTPHost '192.168.1.1', got '%s'", cfg.SMTPHost)
		}
		if cfg.MailDir != "/tmp/maildev" {
			t.Errorf("Expected MailDir '/tmp/maildev', got '%s'", cfg.MailDir)
		}
		if cfg.WebPort != 3080 {
			t.Errorf("Expected WebPort 3080, got %d", cfg.WebPort)
		}
		if cfg.WebHost != "192.168.1.2" {
			t.Errorf("Expected WebHost '192.168.1.2', got '%s'", cfg.WebHost)
		}
		if cfg.WebUser != "maildevuser" {
			t.Errorf("Expected WebUser 'maildevuser', got '%s'", cfg.WebUser)
		}
		if cfg.WebPassword != "maildevpass" {
			t.Errorf("Expected WebPassword 'maildevpass', got '%s'", cfg.WebPassword)
		}
		if cfg.HTTPSEnabled != true {
			t.Errorf("Expected HTTPSEnabled true, got %v", cfg.HTTPSEnabled)
		}
		if cfg.OutgoingHost != "smtp.maildev.com" {
			t.Errorf("Expected OutgoingHost 'smtp.maildev.com', got '%s'", cfg.OutgoingHost)
		}
		if cfg.OutgoingPort != 25 {
			t.Errorf("Expected OutgoingPort 25, got %d", cfg.OutgoingPort)
		}
		if cfg.SMTPUser != "maildevsmtp" {
			t.Errorf("Expected SMTPUser 'maildevsmtp', got '%s'", cfg.SMTPUser)
		}
		if cfg.TLSEnabled != true {
			t.Errorf("Expected TLSEnabled true, got %v", cfg.TLSEnabled)
		}
		if cfg.LogLevel != "verbose" {
			t.Errorf("Expected LogLevel 'verbose', got '%s'", cfg.LogLevel)
		}
	})

	t.Run("command line flags override environment variables", func(t *testing.T) {
		resetFlags()
		clearEnvVars()

		// Set environment variables
		_ = os.Setenv("OWLMAIL_SMTP_PORT", "2025")
		_ = os.Setenv("OWLMAIL_SMTP_HOST", "0.0.0.0")
		_ = os.Setenv("OWLMAIL_WEB_PORT", "2080")
		_ = os.Setenv("OWLMAIL_WEB_HOST", "127.0.0.1")
		_ = os.Setenv("OWLMAIL_HTTPS_ENABLED", "true")
		_ = os.Setenv("OWLMAIL_AUTO_RELAY", "true")
		_ = os.Setenv("OWLMAIL_TLS_ENABLED", "true")
		_ = os.Setenv("OWLMAIL_LOG_LEVEL", "verbose")

		defer func() {
			_ = os.Unsetenv("OWLMAIL_SMTP_PORT")
			_ = os.Unsetenv("OWLMAIL_SMTP_HOST")
			_ = os.Unsetenv("OWLMAIL_WEB_PORT")
			_ = os.Unsetenv("OWLMAIL_WEB_HOST")
			_ = os.Unsetenv("OWLMAIL_HTTPS_ENABLED")
			_ = os.Unsetenv("OWLMAIL_AUTO_RELAY")
			_ = os.Unsetenv("OWLMAIL_TLS_ENABLED")
			_ = os.Unsetenv("OWLMAIL_LOG_LEVEL")
		}()

		// Set command line arguments (should override environment variables)
		os.Args = []string{
			"owlmail",
			"-smtp", "4025",
			"-ip", "10.0.0.1",
			"-web", "4080",
			"-web-ip", "10.0.0.2",
			"-https=false",
			"-auto-relay=false",
			"-tls=false",
			"-log-level", "silent",
		}

		cfg := parseConfig()

		// Command line flags should override environment variables
		if cfg.SMTPPort != 4025 {
			t.Errorf("Expected SMTPPort 4025 (from flag), got %d", cfg.SMTPPort)
		}
		if cfg.SMTPHost != "10.0.0.1" {
			t.Errorf("Expected SMTPHost '10.0.0.1' (from flag), got '%s'", cfg.SMTPHost)
		}
		if cfg.WebPort != 4080 {
			t.Errorf("Expected WebPort 4080 (from flag), got %d", cfg.WebPort)
		}
		if cfg.WebHost != "10.0.0.2" {
			t.Errorf("Expected WebHost '10.0.0.2' (from flag), got '%s'", cfg.WebHost)
		}
		if cfg.HTTPSEnabled != false {
			t.Errorf("Expected HTTPSEnabled false (from flag), got %v", cfg.HTTPSEnabled)
		}
		if cfg.AutoRelay != false {
			t.Errorf("Expected AutoRelay false (from flag), got %v", cfg.AutoRelay)
		}
		if cfg.TLSEnabled != false {
			t.Errorf("Expected TLSEnabled false (from flag), got %v", cfg.TLSEnabled)
		}
		if cfg.LogLevel != "silent" {
			t.Errorf("Expected LogLevel 'silent' (from flag), got '%s'", cfg.LogLevel)
		}
	})

	t.Run("all command line flags", func(t *testing.T) {
		resetFlags()
		clearEnvVars()
		os.Args = []string{
			"owlmail",
			"-smtp", "5025",
			"-ip", "192.168.0.1",
			"-mail-directory", "/custom/mail",
			"-web", "5080",
			"-web-ip", "192.168.0.2",
			"-web-user", "flaguser",
			"-web-password", "flagpass",
			"-https=true",
			"-https-cert", "/flag/cert.pem",
			"-https-key", "/flag/key.pem",
			"-outgoing-host", "smtp.flag.com",
			"-outgoing-port", "2525",
			"-outgoing-user", "flagout",
			"-outgoing-pass", "flagoutpass",
			"-outgoing-secure=true",
			"-auto-relay=true",
			"-auto-relay-addr", "flag@example.com",
			"-auto-relay-rules", "/flag/rules.json",
			"-smtp-user", "flagsmtp",
			"-smtp-password", "flagsmtppass",
			"-tls=true",
			"-tls-cert", "/flag/tls-cert.pem",
			"-tls-key", "/flag/tls-key.pem",
			"-log-level", "verbose",
		}

		cfg := parseConfig()

		if cfg.SMTPPort != 5025 {
			t.Errorf("Expected SMTPPort 5025, got %d", cfg.SMTPPort)
		}
		if cfg.SMTPHost != "192.168.0.1" {
			t.Errorf("Expected SMTPHost '192.168.0.1', got '%s'", cfg.SMTPHost)
		}
		if cfg.MailDir != "/custom/mail" {
			t.Errorf("Expected MailDir '/custom/mail', got '%s'", cfg.MailDir)
		}
		if cfg.WebPort != 5080 {
			t.Errorf("Expected WebPort 5080, got %d", cfg.WebPort)
		}
		if cfg.WebHost != "192.168.0.2" {
			t.Errorf("Expected WebHost '192.168.0.2', got '%s'", cfg.WebHost)
		}
		if cfg.WebUser != "flaguser" {
			t.Errorf("Expected WebUser 'flaguser', got '%s'", cfg.WebUser)
		}
		if cfg.WebPassword != "flagpass" {
			t.Errorf("Expected WebPassword 'flagpass', got '%s'", cfg.WebPassword)
		}
		if cfg.HTTPSEnabled != true {
			t.Errorf("Expected HTTPSEnabled true, got %v", cfg.HTTPSEnabled)
		}
		if cfg.HTTPSCertFile != "/flag/cert.pem" {
			t.Errorf("Expected HTTPSCertFile '/flag/cert.pem', got '%s'", cfg.HTTPSCertFile)
		}
		if cfg.HTTPSKeyFile != "/flag/key.pem" {
			t.Errorf("Expected HTTPSKeyFile '/flag/key.pem', got '%s'", cfg.HTTPSKeyFile)
		}
		if cfg.OutgoingHost != "smtp.flag.com" {
			t.Errorf("Expected OutgoingHost 'smtp.flag.com', got '%s'", cfg.OutgoingHost)
		}
		if cfg.OutgoingPort != 2525 {
			t.Errorf("Expected OutgoingPort 2525, got %d", cfg.OutgoingPort)
		}
		if cfg.OutgoingUser != "flagout" {
			t.Errorf("Expected OutgoingUser 'flagout', got '%s'", cfg.OutgoingUser)
		}
		if cfg.OutgoingPass != "flagoutpass" {
			t.Errorf("Expected OutgoingPass 'flagoutpass', got '%s'", cfg.OutgoingPass)
		}
		if cfg.OutgoingSecure != true {
			t.Errorf("Expected OutgoingSecure true, got %v", cfg.OutgoingSecure)
		}
		if cfg.AutoRelay != true {
			t.Errorf("Expected AutoRelay true, got %v", cfg.AutoRelay)
		}
		if cfg.AutoRelayAddr != "flag@example.com" {
			t.Errorf("Expected AutoRelayAddr 'flag@example.com', got '%s'", cfg.AutoRelayAddr)
		}
		if cfg.AutoRelayRules != "/flag/rules.json" {
			t.Errorf("Expected AutoRelayRules '/flag/rules.json', got '%s'", cfg.AutoRelayRules)
		}
		if cfg.SMTPUser != "flagsmtp" {
			t.Errorf("Expected SMTPUser 'flagsmtp', got '%s'", cfg.SMTPUser)
		}
		if cfg.SMTPPassword != "flagsmtppass" {
			t.Errorf("Expected SMTPPassword 'flagsmtppass', got '%s'", cfg.SMTPPassword)
		}
		if cfg.TLSEnabled != true {
			t.Errorf("Expected TLSEnabled true, got %v", cfg.TLSEnabled)
		}
		if cfg.TLSCertFile != "/flag/tls-cert.pem" {
			t.Errorf("Expected TLSCertFile '/flag/tls-cert.pem', got '%s'", cfg.TLSCertFile)
		}
		if cfg.TLSKeyFile != "/flag/tls-key.pem" {
			t.Errorf("Expected TLSKeyFile '/flag/tls-key.pem', got '%s'", cfg.TLSKeyFile)
		}
		if cfg.LogLevel != "verbose" {
			t.Errorf("Expected LogLevel 'verbose', got '%s'", cfg.LogLevel)
		}
	})
}

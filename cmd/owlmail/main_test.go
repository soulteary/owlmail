package main

import (
	"encoding/json"
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
	defer server.Close()

	// Register event handlers
	registerEventHandlers(server)

	// Verify handlers are registered by checking that On can be called without error
	// The actual event triggering is tested in mailserver package
	// Here we just verify that registerEventHandlers doesn't panic
}

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGetEnvString(t *testing.T) {
	// Test with environment variable set
	os.Setenv("TEST_VAR", "test-value")
	defer os.Unsetenv("TEST_VAR")

	result := getEnvString("TEST_VAR", "default")
	if result != "test-value" {
		t.Errorf("Expected 'test-value', got '%s'", result)
	}

	// Test with environment variable not set
	os.Unsetenv("TEST_VAR")
	result = getEnvString("TEST_VAR", "default")
	if result != "default" {
		t.Errorf("Expected 'default', got '%s'", result)
	}

	// Test with empty environment variable
	os.Setenv("TEST_VAR", "")
	defer os.Unsetenv("TEST_VAR")
	result = getEnvString("TEST_VAR", "default")
	if result != "default" {
		t.Errorf("Expected 'default' for empty env var, got '%s'", result)
	}
}

func TestGetEnvInt(t *testing.T) {
	// Test with valid integer
	os.Setenv("TEST_INT", "123")
	defer os.Unsetenv("TEST_INT")

	result := getEnvInt("TEST_INT", 0)
	if result != 123 {
		t.Errorf("Expected 123, got %d", result)
	}

	// Test with environment variable not set
	os.Unsetenv("TEST_INT")
	result = getEnvInt("TEST_INT", 456)
	if result != 456 {
		t.Errorf("Expected 456, got %d", result)
	}

	// Test with invalid integer
	os.Setenv("TEST_INT", "invalid")
	defer os.Unsetenv("TEST_INT")
	result = getEnvInt("TEST_INT", 789)
	if result != 789 {
		t.Errorf("Expected 789 for invalid int, got %d", result)
	}

	// Test with empty environment variable
	os.Setenv("TEST_INT", "")
	defer os.Unsetenv("TEST_INT")
	result = getEnvInt("TEST_INT", 999)
	if result != 999 {
		t.Errorf("Expected 999 for empty env var, got %d", result)
	}
}

func TestGetEnvBool(t *testing.T) {
	// Test with "true"
	os.Setenv("TEST_BOOL", "true")
	defer os.Unsetenv("TEST_BOOL")

	result := getEnvBool("TEST_BOOL", false)
	if result != true {
		t.Errorf("Expected true, got %v", result)
	}

	// Test with "false"
	os.Setenv("TEST_BOOL", "false")
	result = getEnvBool("TEST_BOOL", true)
	if result != false {
		t.Errorf("Expected false, got %v", result)
	}

	// Test with "1"
	os.Setenv("TEST_BOOL", "1")
	result = getEnvBool("TEST_BOOL", false)
	if result != true {
		t.Errorf("Expected true for '1', got %v", result)
	}

	// Test with "0"
	os.Setenv("TEST_BOOL", "0")
	result = getEnvBool("TEST_BOOL", true)
	if result != false {
		t.Errorf("Expected false for '0', got %v", result)
	}

	// Test with environment variable not set
	os.Unsetenv("TEST_BOOL")
	result = getEnvBool("TEST_BOOL", true)
	if result != true {
		t.Errorf("Expected true (default), got %v", result)
	}

	// Test with invalid boolean
	os.Setenv("TEST_BOOL", "invalid")
	defer os.Unsetenv("TEST_BOOL")
	result = getEnvBool("TEST_BOOL", false)
	if result != false {
		t.Errorf("Expected false for invalid bool, got %v", result)
	}
}

func TestGetLogLevelFromEnv(t *testing.T) {
	// Test with MAILDEV_VERBOSE
	os.Setenv("MAILDEV_VERBOSE", "1")
	defer os.Unsetenv("MAILDEV_VERBOSE")
	os.Unsetenv("MAILDEV_SILENT")
	os.Unsetenv("OWLMAIL_LOG_LEVEL")

	result := getLogLevelFromEnv()
	if result != LogLevelVerbose {
		t.Errorf("Expected LogLevelVerbose, got %d", result)
	}

	// Test with MAILDEV_SILENT
	os.Unsetenv("MAILDEV_VERBOSE")
	os.Setenv("MAILDEV_SILENT", "1")
	defer os.Unsetenv("MAILDEV_SILENT")

	result = getLogLevelFromEnv()
	if result != LogLevelSilent {
		t.Errorf("Expected LogLevelSilent, got %d", result)
	}

	// Test with OWLMAIL_LOG_LEVEL=verbose
	os.Unsetenv("MAILDEV_VERBOSE")
	os.Unsetenv("MAILDEV_SILENT")
	os.Setenv("OWLMAIL_LOG_LEVEL", "verbose")
	defer os.Unsetenv("OWLMAIL_LOG_LEVEL")

	result = getLogLevelFromEnv()
	if result != LogLevelVerbose {
		t.Errorf("Expected LogLevelVerbose, got %d", result)
	}

	// Test with OWLMAIL_LOG_LEVEL=silent
	os.Setenv("OWLMAIL_LOG_LEVEL", "silent")
	result = getLogLevelFromEnv()
	if result != LogLevelSilent {
		t.Errorf("Expected LogLevelSilent, got %d", result)
	}

	// Test with default
	os.Unsetenv("MAILDEV_VERBOSE")
	os.Unsetenv("MAILDEV_SILENT")
	os.Unsetenv("OWLMAIL_LOG_LEVEL")

	result = getLogLevelFromEnv()
	if result != LogLevelNormal {
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

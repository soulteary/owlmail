package main

import (
	"os"
	"testing"
)

func TestGetEnvStringWithMailDevCompat(t *testing.T) {
	// Test MailDev environment variable priority
	os.Setenv("MAILDEV_SMTP_PORT", "test-maildev")
	defer os.Unsetenv("MAILDEV_SMTP_PORT")
	os.Unsetenv("OWLMAIL_SMTP_PORT")

	result := getEnvStringWithMailDevCompat("MAILDEV_SMTP_PORT", "OWLMAIL_SMTP_PORT", "default")
	if result != "test-maildev" {
		t.Errorf("Expected 'test-maildev', got '%s'", result)
	}

	// Test OwlMail environment variable fallback
	os.Unsetenv("MAILDEV_SMTP_PORT")
	os.Setenv("OWLMAIL_SMTP_PORT", "test-owlmail")
	defer os.Unsetenv("OWLMAIL_SMTP_PORT")

	result = getEnvStringWithMailDevCompat("MAILDEV_SMTP_PORT", "OWLMAIL_SMTP_PORT", "default")
	if result != "test-owlmail" {
		t.Errorf("Expected 'test-owlmail', got '%s'", result)
	}

	// Test default value
	os.Unsetenv("MAILDEV_SMTP_PORT")
	os.Unsetenv("OWLMAIL_SMTP_PORT")

	result = getEnvStringWithMailDevCompat("MAILDEV_SMTP_PORT", "OWLMAIL_SMTP_PORT", "default")
	if result != "default" {
		t.Errorf("Expected 'default', got '%s'", result)
	}
}

func TestGetEnvIntWithMailDevCompat(t *testing.T) {
	// Test MailDev environment variable priority
	os.Setenv("MAILDEV_WEB_PORT", "8080")
	defer os.Unsetenv("MAILDEV_WEB_PORT")
	os.Unsetenv("OWLMAIL_WEB_PORT")

	result := getEnvIntWithMailDevCompat("MAILDEV_WEB_PORT", "OWLMAIL_WEB_PORT", 1080)
	if result != 8080 {
		t.Errorf("Expected 8080, got %d", result)
	}

	// Test OwlMail environment variable fallback
	os.Unsetenv("MAILDEV_WEB_PORT")
	os.Setenv("OWLMAIL_WEB_PORT", "9090")
	defer os.Unsetenv("OWLMAIL_WEB_PORT")

	result = getEnvIntWithMailDevCompat("MAILDEV_WEB_PORT", "OWLMAIL_WEB_PORT", 1080)
	if result != 9090 {
		t.Errorf("Expected 9090, got %d", result)
	}

	// Test default value
	os.Unsetenv("MAILDEV_WEB_PORT")
	os.Unsetenv("OWLMAIL_WEB_PORT")

	result = getEnvIntWithMailDevCompat("MAILDEV_WEB_PORT", "OWLMAIL_WEB_PORT", 1080)
	if result != 1080 {
		t.Errorf("Expected 1080, got %d", result)
	}

	// Test invalid integer
	os.Setenv("MAILDEV_WEB_PORT", "invalid")
	defer os.Unsetenv("MAILDEV_WEB_PORT")

	result = getEnvIntWithMailDevCompat("MAILDEV_WEB_PORT", "OWLMAIL_WEB_PORT", 1080)
	if result != 1080 {
		t.Errorf("Expected default 1080 for invalid int, got %d", result)
	}
}

func TestGetEnvBoolWithMailDevCompat(t *testing.T) {
	// Test MailDev environment variable priority
	os.Setenv("MAILDEV_HTTPS", "true")
	defer os.Unsetenv("MAILDEV_HTTPS")
	os.Unsetenv("OWLMAIL_HTTPS_ENABLED")

	result := getEnvBoolWithMailDevCompat("MAILDEV_HTTPS", "OWLMAIL_HTTPS_ENABLED", false)
	if result != true {
		t.Errorf("Expected true, got %v", result)
	}

	// Test OwlMail environment variable fallback
	os.Unsetenv("MAILDEV_HTTPS")
	os.Setenv("OWLMAIL_HTTPS_ENABLED", "false")
	defer os.Unsetenv("OWLMAIL_HTTPS_ENABLED")

	result = getEnvBoolWithMailDevCompat("MAILDEV_HTTPS", "OWLMAIL_HTTPS_ENABLED", true)
	if result != false {
		t.Errorf("Expected false, got %v", result)
	}

	// Test default value
	os.Unsetenv("MAILDEV_HTTPS")
	os.Unsetenv("OWLMAIL_HTTPS_ENABLED")

	result = getEnvBoolWithMailDevCompat("MAILDEV_HTTPS", "OWLMAIL_HTTPS_ENABLED", true)
	if result != true {
		t.Errorf("Expected true, got %v", result)
	}

	// Test invalid boolean
	os.Setenv("MAILDEV_HTTPS", "invalid")
	defer os.Unsetenv("MAILDEV_HTTPS")

	result = getEnvBoolWithMailDevCompat("MAILDEV_HTTPS", "OWLMAIL_HTTPS_ENABLED", false)
	if result != false {
		t.Errorf("Expected default false for invalid bool, got %v", result)
	}
}

func TestGetMailDevEnvString(t *testing.T) {
	// Test with mapped environment variable
	os.Setenv("MAILDEV_SMTP_PORT", "1026")
	defer os.Unsetenv("MAILDEV_SMTP_PORT")
	os.Unsetenv("OWLMAIL_SMTP_PORT")

	result := getMailDevEnvString("OWLMAIL_SMTP_PORT", "1025")
	if result != "1026" {
		t.Errorf("Expected '1026', got '%s'", result)
	}

	// Test with unmapped environment variable
	os.Unsetenv("MAILDEV_SMTP_PORT")
	os.Setenv("OWLMAIL_TEST_VAR", "test-value")
	defer os.Unsetenv("OWLMAIL_TEST_VAR")

	result = getMailDevEnvString("OWLMAIL_TEST_VAR", "default")
	if result != "test-value" {
		t.Errorf("Expected 'test-value', got '%s'", result)
	}
}

func TestGetMailDevEnvInt(t *testing.T) {
	// Test with mapped environment variable
	os.Setenv("MAILDEV_WEB_PORT", "8080")
	defer os.Unsetenv("MAILDEV_WEB_PORT")
	os.Unsetenv("OWLMAIL_WEB_PORT")

	result := getMailDevEnvInt("OWLMAIL_WEB_PORT", 1080)
	if result != 8080 {
		t.Errorf("Expected 8080, got %d", result)
	}
}

func TestGetMailDevEnvBool(t *testing.T) {
	// Test with mapped environment variable
	os.Setenv("MAILDEV_HTTPS", "true")
	defer os.Unsetenv("MAILDEV_HTTPS")
	os.Unsetenv("OWLMAIL_HTTPS_ENABLED")

	result := getMailDevEnvBool("OWLMAIL_HTTPS_ENABLED", false)
	if result != true {
		t.Errorf("Expected true, got %v", result)
	}
}

func TestGetMailDevLogLevel(t *testing.T) {
	// Test MAILDEV_VERBOSE
	os.Setenv("MAILDEV_VERBOSE", "1")
	defer os.Unsetenv("MAILDEV_VERBOSE")
	os.Unsetenv("MAILDEV_SILENT")
	os.Unsetenv("OWLMAIL_LOG_LEVEL")

	result := getMailDevLogLevel("normal")
	if result != "verbose" {
		t.Errorf("Expected 'verbose', got '%s'", result)
	}

	// Test MAILDEV_SILENT
	os.Unsetenv("MAILDEV_VERBOSE")
	os.Setenv("MAILDEV_SILENT", "1")
	defer os.Unsetenv("MAILDEV_SILENT")

	result = getMailDevLogLevel("normal")
	if result != "silent" {
		t.Errorf("Expected 'silent', got '%s'", result)
	}

	// Test OWLMAIL_LOG_LEVEL fallback
	os.Unsetenv("MAILDEV_VERBOSE")
	os.Unsetenv("MAILDEV_SILENT")
	os.Setenv("OWLMAIL_LOG_LEVEL", "verbose")
	defer os.Unsetenv("OWLMAIL_LOG_LEVEL")

	result = getMailDevLogLevel("normal")
	if result != "verbose" {
		t.Errorf("Expected 'verbose', got '%s'", result)
	}

	// Test default value
	os.Unsetenv("MAILDEV_VERBOSE")
	os.Unsetenv("MAILDEV_SILENT")
	os.Unsetenv("OWLMAIL_LOG_LEVEL")

	result = getMailDevLogLevel("normal")
	if result != "normal" {
		t.Errorf("Expected 'normal', got '%s'", result)
	}
}

func TestMailDevEnvMapping(t *testing.T) {
	// Test that all mappings in maildevEnvMapping work correctly
	testCases := []struct {
		maildevKey string
		owlmailKey string
		value      string
	}{
		{"MAILDEV_SMTP_PORT", "OWLMAIL_SMTP_PORT", "1026"},
		{"MAILDEV_IP", "OWLMAIL_SMTP_HOST", "127.0.0.1"},
		{"MAILDEV_MAIL_DIRECTORY", "OWLMAIL_MAIL_DIR", "/tmp/test"},
		{"MAILDEV_WEB_PORT", "OWLMAIL_WEB_PORT", "8080"},
		{"MAILDEV_WEB_IP", "OWLMAIL_WEB_HOST", "0.0.0.0"},
		{"MAILDEV_WEB_USER", "OWLMAIL_WEB_USER", "admin"},
		{"MAILDEV_WEB_PASS", "OWLMAIL_WEB_PASSWORD", "password"},
		{"MAILDEV_HTTPS", "OWLMAIL_HTTPS_ENABLED", "true"},
		{"MAILDEV_HTTPS_CERT", "OWLMAIL_HTTPS_CERT", "/path/to/cert"},
		{"MAILDEV_HTTPS_KEY", "OWLMAIL_HTTPS_KEY", "/path/to/key"},
		{"MAILDEV_OUTGOING_HOST", "OWLMAIL_OUTGOING_HOST", "smtp.example.com"},
		{"MAILDEV_OUTGOING_PORT", "OWLMAIL_OUTGOING_PORT", "587"},
		{"MAILDEV_OUTGOING_USER", "OWLMAIL_OUTGOING_USER", "user"},
		{"MAILDEV_OUTGOING_PASS", "OWLMAIL_OUTGOING_PASSWORD", "pass"},
		{"MAILDEV_OUTGOING_SECURE", "OWLMAIL_OUTGOING_SECURE", "true"},
		{"MAILDEV_AUTO_RELAY", "OWLMAIL_AUTO_RELAY", "true"},
		{"MAILDEV_AUTO_RELAY_ADDR", "OWLMAIL_AUTO_RELAY_ADDR", "relay@example.com"},
		{"MAILDEV_AUTO_RELAY_RULES", "OWLMAIL_AUTO_RELAY_RULES", "/path/to/rules"},
		{"MAILDEV_INCOMING_USER", "OWLMAIL_SMTP_USER", "incoming"},
		{"MAILDEV_INCOMING_PASS", "OWLMAIL_SMTP_PASSWORD", "incomingpass"},
		{"MAILDEV_INCOMING_SECURE", "OWLMAIL_TLS_ENABLED", "true"},
		{"MAILDEV_INCOMING_CERT", "OWLMAIL_TLS_CERT", "/path/to/tls/cert"},
		{"MAILDEV_INCOMING_KEY", "OWLMAIL_TLS_KEY", "/path/to/tls/key"},
	}

	for _, tc := range testCases {
		// Set MailDev env var
		os.Setenv(tc.maildevKey, tc.value)
		defer os.Unsetenv(tc.maildevKey)
		os.Unsetenv(tc.owlmailKey)

		result := getMailDevEnvString(tc.owlmailKey, "default")
		if result != tc.value {
			t.Errorf("For %s -> %s: Expected '%s', got '%s'", tc.maildevKey, tc.owlmailKey, tc.value, result)
		}
	}
}


package maildev

import (
	"os"
	"testing"
)

func TestGetEnvStringWithMailDevCompat(t *testing.T) {
	// Test MailDev environment variable priority
	if err := os.Setenv("MAILDEV_SMTP_PORT", "test-maildev"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("MAILDEV_SMTP_PORT"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()
	if err := os.Unsetenv("OWLMAIL_SMTP_PORT"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}

	result := getEnvStringWithMailDevCompat("MAILDEV_SMTP_PORT", "OWLMAIL_SMTP_PORT", "default")
	if result != "test-maildev" {
		t.Errorf("Expected 'test-maildev', got '%s'", result)
	}

	// Test OwlMail environment variable fallback
	if err := os.Unsetenv("MAILDEV_SMTP_PORT"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	if err := os.Setenv("OWLMAIL_SMTP_PORT", "test-owlmail"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("OWLMAIL_SMTP_PORT"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()

	result = getEnvStringWithMailDevCompat("MAILDEV_SMTP_PORT", "OWLMAIL_SMTP_PORT", "default")
	if result != "test-owlmail" {
		t.Errorf("Expected 'test-owlmail', got '%s'", result)
	}

	// Test default value
	if err := os.Unsetenv("MAILDEV_SMTP_PORT"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	if err := os.Unsetenv("OWLMAIL_SMTP_PORT"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}

	result = getEnvStringWithMailDevCompat("MAILDEV_SMTP_PORT", "OWLMAIL_SMTP_PORT", "default")
	if result != "default" {
		t.Errorf("Expected 'default', got '%s'", result)
	}
}

func TestGetEnvIntWithMailDevCompat(t *testing.T) {
	// Test MailDev environment variable priority
	if err := os.Setenv("MAILDEV_WEB_PORT", "8080"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("MAILDEV_WEB_PORT"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()
	if err := os.Unsetenv("OWLMAIL_WEB_PORT"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}

	result := getEnvIntWithMailDevCompat("MAILDEV_WEB_PORT", "OWLMAIL_WEB_PORT", 1080)
	if result != 8080 {
		t.Errorf("Expected 8080, got %d", result)
	}

	// Test OwlMail environment variable fallback
	if err := os.Unsetenv("MAILDEV_WEB_PORT"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	if err := os.Setenv("OWLMAIL_WEB_PORT", "9090"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("OWLMAIL_WEB_PORT"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()

	result = getEnvIntWithMailDevCompat("MAILDEV_WEB_PORT", "OWLMAIL_WEB_PORT", 1080)
	if result != 9090 {
		t.Errorf("Expected 9090, got %d", result)
	}

	// Test default value
	if err := os.Unsetenv("MAILDEV_WEB_PORT"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	if err := os.Unsetenv("OWLMAIL_WEB_PORT"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}

	result = getEnvIntWithMailDevCompat("MAILDEV_WEB_PORT", "OWLMAIL_WEB_PORT", 1080)
	if result != 1080 {
		t.Errorf("Expected 1080, got %d", result)
	}

	// Test invalid integer
	if err := os.Setenv("MAILDEV_WEB_PORT", "invalid"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("MAILDEV_WEB_PORT"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()

	result = getEnvIntWithMailDevCompat("MAILDEV_WEB_PORT", "OWLMAIL_WEB_PORT", 1080)
	if result != 1080 {
		t.Errorf("Expected default 1080 for invalid int, got %d", result)
	}
}

func TestGetEnvBoolWithMailDevCompat(t *testing.T) {
	// Test MailDev environment variable priority
	if err := os.Setenv("MAILDEV_HTTPS", "true"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("MAILDEV_HTTPS"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()
	if err := os.Unsetenv("OWLMAIL_HTTPS_ENABLED"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}

	result := getEnvBoolWithMailDevCompat("MAILDEV_HTTPS", "OWLMAIL_HTTPS_ENABLED", false)
	if result != true {
		t.Errorf("Expected true, got %v", result)
	}

	// Test OwlMail environment variable fallback
	if err := os.Unsetenv("MAILDEV_HTTPS"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	if err := os.Setenv("OWLMAIL_HTTPS_ENABLED", "false"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("OWLMAIL_HTTPS_ENABLED"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()

	result = getEnvBoolWithMailDevCompat("MAILDEV_HTTPS", "OWLMAIL_HTTPS_ENABLED", true)
	if result != false {
		t.Errorf("Expected false, got %v", result)
	}

	// Test default value
	if err := os.Unsetenv("MAILDEV_HTTPS"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	if err := os.Unsetenv("OWLMAIL_HTTPS_ENABLED"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}

	result = getEnvBoolWithMailDevCompat("MAILDEV_HTTPS", "OWLMAIL_HTTPS_ENABLED", true)
	if result != true {
		t.Errorf("Expected true, got %v", result)
	}

	// Test invalid boolean
	if err := os.Setenv("MAILDEV_HTTPS", "invalid"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("MAILDEV_HTTPS"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()

	result = getEnvBoolWithMailDevCompat("MAILDEV_HTTPS", "OWLMAIL_HTTPS_ENABLED", false)
	if result != false {
		t.Errorf("Expected default false for invalid bool, got %v", result)
	}
}

func TestGetMailDevEnvString(t *testing.T) {
	// Test with mapped environment variable
	if err := os.Setenv("MAILDEV_SMTP_PORT", "1026"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("MAILDEV_SMTP_PORT"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()
	if err := os.Unsetenv("OWLMAIL_SMTP_PORT"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}

	result := GetMailDevEnvString("OWLMAIL_SMTP_PORT", "1025")
	if result != "1026" {
		t.Errorf("Expected '1026', got '%s'", result)
	}

	// Test with unmapped environment variable
	if err := os.Unsetenv("MAILDEV_SMTP_PORT"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	if err := os.Setenv("OWLMAIL_TEST_VAR", "test-value"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("OWLMAIL_TEST_VAR"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()

	result = GetMailDevEnvString("OWLMAIL_TEST_VAR", "default")
	if result != "test-value" {
		t.Errorf("Expected 'test-value', got '%s'", result)
	}
}

func TestGetMailDevEnvInt(t *testing.T) {
	// Test with mapped environment variable
	if err := os.Setenv("MAILDEV_WEB_PORT", "8080"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("MAILDEV_WEB_PORT"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()
	if err := os.Unsetenv("OWLMAIL_WEB_PORT"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}

	result := GetMailDevEnvInt("OWLMAIL_WEB_PORT", 1080)
	if result != 8080 {
		t.Errorf("Expected 8080, got %d", result)
	}
}

func TestGetMailDevEnvBool(t *testing.T) {
	// Test with mapped environment variable
	if err := os.Setenv("MAILDEV_HTTPS", "true"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("MAILDEV_HTTPS"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()
	if err := os.Unsetenv("OWLMAIL_HTTPS_ENABLED"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}

	result := GetMailDevEnvBool("OWLMAIL_HTTPS_ENABLED", false)
	if result != true {
		t.Errorf("Expected true, got %v", result)
	}
}

func TestGetMailDevLogLevel(t *testing.T) {
	// Test MAILDEV_VERBOSE
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

	result := GetMailDevLogLevel("normal")
	if result != "verbose" {
		t.Errorf("Expected 'verbose', got '%s'", result)
	}

	// Test MAILDEV_SILENT
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

	result = GetMailDevLogLevel("normal")
	if result != "silent" {
		t.Errorf("Expected 'silent', got '%s'", result)
	}

	// Test OWLMAIL_LOG_LEVEL fallback
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

	result = GetMailDevLogLevel("normal")
	if result != "verbose" {
		t.Errorf("Expected 'verbose', got '%s'", result)
	}

	// Test default value
	if err := os.Unsetenv("MAILDEV_VERBOSE"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	if err := os.Unsetenv("MAILDEV_SILENT"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	if err := os.Unsetenv("OWLMAIL_LOG_LEVEL"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}

	result = GetMailDevLogLevel("normal")
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
		if err := os.Setenv(tc.maildevKey, tc.value); err != nil {
			t.Fatalf("Failed to set environment variable %s: %v", tc.maildevKey, err)
		}
		defer func(key string) {
			if err := os.Unsetenv(key); err != nil {
				t.Fatalf("Failed to unset environment variable %s: %v", key, err)
			}
		}(tc.maildevKey)
		if err := os.Unsetenv(tc.owlmailKey); err != nil {
			t.Fatalf("Failed to unset environment variable %s: %v", tc.owlmailKey, err)
		}

		result := GetMailDevEnvString(tc.owlmailKey, "default")
		if result != tc.value {
			t.Errorf("For %s -> %s: Expected '%s', got '%s'", tc.maildevKey, tc.owlmailKey, tc.value, result)
		}
	}
}

func TestGetMailDevEnvStringUnmapped(t *testing.T) {
	// Test with unmapped environment variable
	if err := os.Setenv("OWLMAIL_UNMAPPED_VAR", "test-value"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("OWLMAIL_UNMAPPED_VAR"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()

	result := GetMailDevEnvString("OWLMAIL_UNMAPPED_VAR", "default")
	if result != "test-value" {
		t.Errorf("Expected 'test-value', got '%s'", result)
	}

	// Test with unmapped variable not set
	if err := os.Unsetenv("OWLMAIL_UNMAPPED_VAR"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	result = GetMailDevEnvString("OWLMAIL_UNMAPPED_VAR", "default")
	if result != "default" {
		t.Errorf("Expected 'default', got '%s'", result)
	}
}

func TestGetMailDevEnvIntUnmapped(t *testing.T) {
	// Test with unmapped environment variable
	if err := os.Setenv("OWLMAIL_UNMAPPED_INT", "123"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("OWLMAIL_UNMAPPED_INT"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()

	result := GetMailDevEnvInt("OWLMAIL_UNMAPPED_INT", 0)
	if result != 123 {
		t.Errorf("Expected 123, got %d", result)
	}

	// Test with invalid integer
	if err := os.Setenv("OWLMAIL_UNMAPPED_INT", "invalid"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	result = GetMailDevEnvInt("OWLMAIL_UNMAPPED_INT", 456)
	if result != 456 {
		t.Errorf("Expected default 456 for invalid int, got %d", result)
	}
}

func TestGetMailDevEnvBoolUnmapped(t *testing.T) {
	// Test with unmapped environment variable
	if err := os.Setenv("OWLMAIL_UNMAPPED_BOOL", "true"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("OWLMAIL_UNMAPPED_BOOL"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()

	result := GetMailDevEnvBool("OWLMAIL_UNMAPPED_BOOL", false)
	if !result {
		t.Errorf("Expected true, got %v", result)
	}

	// Test with invalid boolean
	if err := os.Setenv("OWLMAIL_UNMAPPED_BOOL", "invalid"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	result = GetMailDevEnvBool("OWLMAIL_UNMAPPED_BOOL", false)
	if result {
		t.Errorf("Expected default false for invalid bool, got %v", result)
	}
}

func TestGetEnvStringWithMailDevCompatEmpty(t *testing.T) {
	// Test with empty MailDev env var
	if err := os.Setenv("MAILDEV_TEST", ""); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("MAILDEV_TEST"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()
	if err := os.Setenv("OWLMAIL_TEST", "owlmail-value"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("OWLMAIL_TEST"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()

	result := getEnvStringWithMailDevCompat("MAILDEV_TEST", "OWLMAIL_TEST", "default")
	if result != "owlmail-value" {
		t.Errorf("Expected 'owlmail-value', got '%s'", result)
	}
}

func TestGetEnvIntWithMailDevCompatInvalid(t *testing.T) {
	// Test with invalid MailDev int, should fallback to OwlMail
	if err := os.Setenv("MAILDEV_TEST", "invalid"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("MAILDEV_TEST"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()
	if err := os.Setenv("OWLMAIL_TEST", "123"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("OWLMAIL_TEST"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()

	result := getEnvIntWithMailDevCompat("MAILDEV_TEST", "OWLMAIL_TEST", 0)
	if result != 123 {
		t.Errorf("Expected 123, got %d", result)
	}
}

func TestGetEnvBoolWithMailDevCompatInvalid(t *testing.T) {
	// Test with invalid MailDev bool, should fallback to OwlMail
	if err := os.Setenv("MAILDEV_TEST", "invalid"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("MAILDEV_TEST"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()
	if err := os.Setenv("OWLMAIL_TEST", "true"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("OWLMAIL_TEST"); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()

	result := getEnvBoolWithMailDevCompat("MAILDEV_TEST", "OWLMAIL_TEST", false)
	if !result {
		t.Errorf("Expected true, got %v", result)
	}
}

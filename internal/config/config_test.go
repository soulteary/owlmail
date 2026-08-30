package config

import (
	"flag"
	"testing"

	"github.com/soulteary/cli-kit/testutil"
)

func TestGetMailDevKey(t *testing.T) {
	tests := []struct {
		owlmailKey string
		expected   string
	}{
		{"OWLMAIL_SMTP_PORT", "MAILDEV_SMTP_PORT"},
		{"OWLMAIL_SMTP_HOST", "MAILDEV_IP"},
		{"OWLMAIL_WEB_PORT", "MAILDEV_WEB_PORT"},
		{"OWLMAIL_WEB_USER", "MAILDEV_WEB_USER"},
		{"OWLMAIL_HTTPS_ENABLED", "MAILDEV_HTTPS"},
		{"OWLMAIL_TLS_ENABLED", "MAILDEV_INCOMING_SECURE"},
		{"NONEXISTENT_KEY", ""},
	}

	for _, tt := range tests {
		t.Run(tt.owlmailKey, func(t *testing.T) {
			result := GetMailDevKey(tt.owlmailKey)
			if result != tt.expected {
				t.Errorf("GetMailDevKey(%q) = %q, want %q", tt.owlmailKey, result, tt.expected)
			}
		})
	}
}

func TestResolveString(t *testing.T) {
	envMgr := testutil.NewEnvManager()
	defer envMgr.Cleanup()

	t.Run("returns default when no env set", func(t *testing.T) {
		result := ResolveString(nil, "test-flag", "OWLMAIL_SMTP_HOST", "default-host")
		if result != "default-host" {
			t.Errorf("ResolveString() = %q, want %q", result, "default-host")
		}
	})

	t.Run("OWLMAIL env takes precedence over default", func(t *testing.T) {
		_ = envMgr.Set("OWLMAIL_SMTP_HOST", "owlmail-host")
		defer envMgr.Cleanup()

		result := ResolveString(nil, "ip", "OWLMAIL_SMTP_HOST", "default-host")
		if result != "owlmail-host" {
			t.Errorf("ResolveString() = %q, want %q", result, "owlmail-host")
		}
	})

	t.Run("MAILDEV env takes precedence over OWLMAIL env", func(t *testing.T) {
		_ = envMgr.Set("OWLMAIL_SMTP_HOST", "owlmail-host")
		_ = envMgr.Set("MAILDEV_IP", "maildev-host")
		defer envMgr.Cleanup()

		result := ResolveString(nil, "ip", "OWLMAIL_SMTP_HOST", "default-host")
		if result != "maildev-host" {
			t.Errorf("ResolveString() = %q, want %q", result, "maildev-host")
		}
	})

	t.Run("CLI flag takes precedence over all env vars", func(t *testing.T) {
		_ = envMgr.Set("OWLMAIL_SMTP_HOST", "owlmail-host")
		_ = envMgr.Set("MAILDEV_IP", "maildev-host")
		defer envMgr.Cleanup()

		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.String("ip", "", "test flag")
		_ = fs.Parse([]string{"-ip", "cli-host"})

		result := ResolveString(fs, "ip", "OWLMAIL_SMTP_HOST", "default-host")
		if result != "cli-host" {
			t.Errorf("ResolveString() = %q, want %q", result, "cli-host")
		}
	})
}

func TestResolveInt(t *testing.T) {
	envMgr := testutil.NewEnvManager()
	defer envMgr.Cleanup()

	t.Run("returns default when no env set", func(t *testing.T) {
		result := ResolveInt(nil, "smtp", "OWLMAIL_SMTP_PORT", 1025)
		if result != 1025 {
			t.Errorf("ResolveInt() = %d, want %d", result, 1025)
		}
	})

	t.Run("OWLMAIL env takes precedence over default", func(t *testing.T) {
		_ = envMgr.Set("OWLMAIL_SMTP_PORT", "2025")
		defer envMgr.Cleanup()

		result := ResolveInt(nil, "smtp", "OWLMAIL_SMTP_PORT", 1025)
		if result != 2025 {
			t.Errorf("ResolveInt() = %d, want %d", result, 2025)
		}
	})

	t.Run("MAILDEV env takes precedence over OWLMAIL env", func(t *testing.T) {
		_ = envMgr.Set("OWLMAIL_SMTP_PORT", "2025")
		_ = envMgr.Set("MAILDEV_SMTP_PORT", "3025")
		defer envMgr.Cleanup()

		result := ResolveInt(nil, "smtp", "OWLMAIL_SMTP_PORT", 1025)
		if result != 3025 {
			t.Errorf("ResolveInt() = %d, want %d", result, 3025)
		}
	})

	t.Run("CLI flag takes precedence over all env vars", func(t *testing.T) {
		_ = envMgr.Set("OWLMAIL_SMTP_PORT", "2025")
		_ = envMgr.Set("MAILDEV_SMTP_PORT", "3025")
		defer envMgr.Cleanup()

		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.Int("smtp", 0, "test flag")
		_ = fs.Parse([]string{"-smtp", "4025"})

		result := ResolveInt(fs, "smtp", "OWLMAIL_SMTP_PORT", 1025)
		if result != 4025 {
			t.Errorf("ResolveInt() = %d, want %d", result, 4025)
		}
	})
}

func TestResolveBool(t *testing.T) {
	envMgr := testutil.NewEnvManager()
	defer envMgr.Cleanup()

	t.Run("returns default when no env set", func(t *testing.T) {
		result := ResolveBool(nil, "https", "OWLMAIL_HTTPS_ENABLED", false)
		if result != false {
			t.Errorf("ResolveBool() = %v, want %v", result, false)
		}
	})

	t.Run("OWLMAIL env takes precedence over default", func(t *testing.T) {
		_ = envMgr.Set("OWLMAIL_HTTPS_ENABLED", "true")
		defer envMgr.Cleanup()

		result := ResolveBool(nil, "https", "OWLMAIL_HTTPS_ENABLED", false)
		if result != true {
			t.Errorf("ResolveBool() = %v, want %v", result, true)
		}
	})

	t.Run("MAILDEV env takes precedence over OWLMAIL env", func(t *testing.T) {
		_ = envMgr.Set("OWLMAIL_HTTPS_ENABLED", "false")
		_ = envMgr.Set("MAILDEV_HTTPS", "true")
		defer envMgr.Cleanup()

		result := ResolveBool(nil, "https", "OWLMAIL_HTTPS_ENABLED", false)
		if result != true {
			t.Errorf("ResolveBool() = %v, want %v", result, true)
		}
	})

	t.Run("CLI flag takes precedence over all env vars", func(t *testing.T) {
		_ = envMgr.Set("OWLMAIL_HTTPS_ENABLED", "true")
		_ = envMgr.Set("MAILDEV_HTTPS", "true")
		defer envMgr.Cleanup()

		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.Bool("https", false, "test flag")
		_ = fs.Parse([]string{"-https=false"})

		result := ResolveBool(fs, "https", "OWLMAIL_HTTPS_ENABLED", true)
		if result != false {
			t.Errorf("ResolveBool() = %v, want %v", result, false)
		}
	})
}

func TestResolveLogLevel(t *testing.T) {
	envMgr := testutil.NewEnvManager()
	defer envMgr.Cleanup()

	t.Run("returns default when no env set", func(t *testing.T) {
		result := ResolveLogLevel(nil, "log-level", "normal")
		if result != "normal" {
			t.Errorf("ResolveLogLevel() = %q, want %q", result, "normal")
		}
	})

	t.Run("OWLMAIL_LOG_LEVEL takes precedence over default", func(t *testing.T) {
		_ = envMgr.Set("OWLMAIL_LOG_LEVEL", "verbose")
		defer envMgr.Cleanup()

		result := ResolveLogLevel(nil, "log-level", "normal")
		if result != "verbose" {
			t.Errorf("ResolveLogLevel() = %q, want %q", result, "verbose")
		}
	})

	t.Run("MAILDEV_VERBOSE takes precedence over OWLMAIL_LOG_LEVEL", func(t *testing.T) {
		_ = envMgr.Set("OWLMAIL_LOG_LEVEL", "silent")
		_ = envMgr.Set("MAILDEV_VERBOSE", "1")
		defer envMgr.Cleanup()

		result := ResolveLogLevel(nil, "log-level", "normal")
		if result != "verbose" {
			t.Errorf("ResolveLogLevel() = %q, want %q", result, "verbose")
		}
	})

	t.Run("MAILDEV_SILENT takes precedence over OWLMAIL_LOG_LEVEL", func(t *testing.T) {
		_ = envMgr.Set("OWLMAIL_LOG_LEVEL", "verbose")
		_ = envMgr.Set("MAILDEV_SILENT", "1")
		defer envMgr.Cleanup()

		result := ResolveLogLevel(nil, "log-level", "normal")
		if result != "silent" {
			t.Errorf("ResolveLogLevel() = %q, want %q", result, "silent")
		}
	})

	t.Run("MAILDEV_VERBOSE takes precedence over MAILDEV_SILENT", func(t *testing.T) {
		_ = envMgr.Set("MAILDEV_VERBOSE", "1")
		_ = envMgr.Set("MAILDEV_SILENT", "1")
		defer envMgr.Cleanup()

		result := ResolveLogLevel(nil, "log-level", "normal")
		if result != "verbose" {
			t.Errorf("ResolveLogLevel() = %q, want %q", result, "verbose")
		}
	})

	t.Run("CLI flag takes precedence over all env vars", func(t *testing.T) {
		_ = envMgr.Set("MAILDEV_VERBOSE", "1")
		defer envMgr.Cleanup()

		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.String("log-level", "", "test flag")
		_ = fs.Parse([]string{"-log-level", "silent"})

		result := ResolveLogLevel(fs, "log-level", "normal")
		if result != "silent" {
			t.Errorf("ResolveLogLevel() = %q, want %q", result, "silent")
		}
	})
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.SMTPPort != 1025 {
		t.Errorf("DefaultConfig().SMTPPort = %d, want %d", cfg.SMTPPort, 1025)
	}
	if cfg.SMTPHost != "localhost" {
		t.Errorf("DefaultConfig().SMTPHost = %q, want %q", cfg.SMTPHost, "localhost")
	}
	if cfg.WebPort != 1080 {
		t.Errorf("DefaultConfig().WebPort = %d, want %d", cfg.WebPort, 1080)
	}
	if cfg.WebHost != "localhost" {
		t.Errorf("DefaultConfig().WebHost = %q, want %q", cfg.WebHost, "localhost")
	}
	if cfg.OutgoingPort != 587 {
		t.Errorf("DefaultConfig().OutgoingPort = %d, want %d", cfg.OutgoingPort, 587)
	}
	if cfg.LogLevel != "normal" {
		t.Errorf("DefaultConfig().LogLevel = %q, want %q", cfg.LogLevel, "normal")
	}
	if cfg.HTTPSEnabled != false {
		t.Errorf("DefaultConfig().HTTPSEnabled = %v, want %v", cfg.HTTPSEnabled, false)
	}
	if cfg.TLSEnabled != false {
		t.Errorf("DefaultConfig().TLSEnabled = %v, want %v", cfg.TLSEnabled, false)
	}
	if cfg.UseUUIDForEmailID != false {
		t.Errorf("DefaultConfig().UseUUIDForEmailID = %v, want %v", cfg.UseUUIDForEmailID, false)
	}
	if cfg.WebhookConfig != "" {
		t.Errorf("DefaultConfig().WebhookConfig = %q, want empty", cfg.WebhookConfig)
	}
	if cfg.WebhookMaxConcurrency != 8 {
		t.Errorf("DefaultConfig().WebhookMaxConcurrency = %d, want 8", cfg.WebhookMaxConcurrency)
	}
	if cfg.WebhookRedisURL != "" || cfg.WebhookRedisPrefix != "owlmail:webhooks" || cfg.WebhookShutdownTimeout != "15s" {
		t.Errorf("unexpected default webhook queue config: %#v", cfg)
	}
}

func TestDefineAndResolveConfig(t *testing.T) {
	// Save and restore environment
	envMgr := testutil.NewEnvManager()
	defer envMgr.Cleanup()

	t.Run("default values", func(t *testing.T) {
		fs := flag.NewFlagSet("test-default", flag.ContinueOnError)
		refs := DefineFlags(fs)
		_ = fs.Parse([]string{})
		cfg := ResolveConfig(fs, refs)

		if cfg.SMTPPort != 1025 {
			t.Errorf("ResolveConfig().SMTPPort = %d, want %d", cfg.SMTPPort, 1025)
		}
		if cfg.SMTPHost != "localhost" {
			t.Errorf("ResolveConfig().SMTPHost = %q, want %q", cfg.SMTPHost, "localhost")
		}
		if cfg.WebPort != 1080 {
			t.Errorf("ResolveConfig().WebPort = %d, want %d", cfg.WebPort, 1080)
		}
		if cfg.LogLevel != "normal" {
			t.Errorf("ResolveConfig().LogLevel = %q, want %q", cfg.LogLevel, "normal")
		}
	})

	t.Run("CLI flags override defaults", func(t *testing.T) {
		fs := flag.NewFlagSet("test-cli", flag.ContinueOnError)
		refs := DefineFlags(fs)
		_ = fs.Parse([]string{"-smtp", "2025", "-ip", "0.0.0.0", "-web", "8080", "-webhook-config", "cli-webhooks.json", "-webhook-max-concurrency", "0", "-webhook-redis-url", "redis://localhost:6379/2", "-webhook-redis-prefix", "test:hooks", "-webhook-shutdown-timeout", "30s"})
		cfg := ResolveConfig(fs, refs)

		if cfg.SMTPPort != 2025 {
			t.Errorf("ResolveConfig().SMTPPort = %d, want %d", cfg.SMTPPort, 2025)
		}
		if cfg.SMTPHost != "0.0.0.0" {
			t.Errorf("ResolveConfig().SMTPHost = %q, want %q", cfg.SMTPHost, "0.0.0.0")
		}
		if cfg.WebPort != 8080 {
			t.Errorf("ResolveConfig().WebPort = %d, want %d", cfg.WebPort, 8080)
		}
		if cfg.WebhookConfig != "cli-webhooks.json" {
			t.Errorf("ResolveConfig().WebhookConfig = %q", cfg.WebhookConfig)
		}
		if cfg.WebhookMaxConcurrency != 0 {
			t.Errorf("ResolveConfig().WebhookMaxConcurrency = %d, want 0", cfg.WebhookMaxConcurrency)
		}
		if cfg.WebhookRedisURL != "redis://localhost:6379/2" || cfg.WebhookRedisPrefix != "test:hooks" || cfg.WebhookShutdownTimeout != "30s" {
			t.Errorf("unexpected CLI webhook queue config: %#v", cfg)
		}
	})

	t.Run("environment variables work", func(t *testing.T) {
		_ = envMgr.Set("OWLMAIL_SMTP_PORT", "3025")
		_ = envMgr.Set("OWLMAIL_SMTP_HOST", "192.168.1.1")
		_ = envMgr.Set("OWLMAIL_WEB_EXTERNAL_SCHEME", "https")
		_ = envMgr.Set("OWLMAIL_WEBHOOK_CONFIG", "env-webhooks.json")
		_ = envMgr.Set("OWLMAIL_WEBHOOK_MAX_CONCURRENCY", "24")
		_ = envMgr.Set("OWLMAIL_WEBHOOK_REDIS_URL", "rediss://redis.example.test:6380/0")
		_ = envMgr.Set("OWLMAIL_WEBHOOK_REDIS_PREFIX", "env:hooks")
		_ = envMgr.Set("OWLMAIL_WEBHOOK_SHUTDOWN_TIMEOUT", "45s")
		defer envMgr.Cleanup()

		fs := flag.NewFlagSet("test-env", flag.ContinueOnError)
		refs := DefineFlags(fs)
		_ = fs.Parse([]string{})
		cfg := ResolveConfig(fs, refs)

		if cfg.SMTPPort != 3025 {
			t.Errorf("ResolveConfig().SMTPPort = %d, want %d", cfg.SMTPPort, 3025)
		}
		if cfg.SMTPHost != "192.168.1.1" {
			t.Errorf("ResolveConfig().SMTPHost = %q, want %q", cfg.SMTPHost, "192.168.1.1")
		}
		if cfg.WebExternalScheme != "https" {
			t.Errorf("ResolveConfig().WebExternalScheme = %q, want https", cfg.WebExternalScheme)
		}
		if cfg.WebhookConfig != "env-webhooks.json" {
			t.Errorf("ResolveConfig().WebhookConfig = %q", cfg.WebhookConfig)
		}
		if cfg.WebhookMaxConcurrency != 24 {
			t.Errorf("ResolveConfig().WebhookMaxConcurrency = %d, want 24", cfg.WebhookMaxConcurrency)
		}
		if cfg.WebhookRedisURL != "rediss://redis.example.test:6380/0" || cfg.WebhookRedisPrefix != "env:hooks" || cfg.WebhookShutdownTimeout != "45s" {
			t.Errorf("unexpected environment webhook queue config: %#v", cfg)
		}
	})

	t.Run("MAILDEV compatibility", func(t *testing.T) {
		_ = envMgr.Set("MAILDEV_SMTP_PORT", "4025")
		_ = envMgr.Set("MAILDEV_IP", "10.0.0.1")
		_ = envMgr.Set("MAILDEV_WEB_PORT", "9080")
		defer envMgr.Cleanup()

		fs := flag.NewFlagSet("test-maildev", flag.ContinueOnError)
		refs := DefineFlags(fs)
		_ = fs.Parse([]string{})
		cfg := ResolveConfig(fs, refs)

		if cfg.SMTPPort != 4025 {
			t.Errorf("ResolveConfig().SMTPPort = %d, want %d", cfg.SMTPPort, 4025)
		}
		if cfg.SMTPHost != "10.0.0.1" {
			t.Errorf("ResolveConfig().SMTPHost = %q, want %q", cfg.SMTPHost, "10.0.0.1")
		}
		if cfg.WebPort != 9080 {
			t.Errorf("ResolveConfig().WebPort = %d, want %d", cfg.WebPort, 9080)
		}
	})

	t.Run("CLI flags override environment variables", func(t *testing.T) {
		_ = envMgr.Set("OWLMAIL_SMTP_PORT", "3025")
		_ = envMgr.Set("MAILDEV_IP", "10.0.0.1")
		defer envMgr.Cleanup()

		fs := flag.NewFlagSet("test-cli-override", flag.ContinueOnError)
		refs := DefineFlags(fs)
		_ = fs.Parse([]string{"-smtp", "5025", "-ip", "127.0.0.1"})
		cfg := ResolveConfig(fs, refs)

		if cfg.SMTPPort != 5025 {
			t.Errorf("ResolveConfig().SMTPPort = %d, want %d", cfg.SMTPPort, 5025)
		}
		if cfg.SMTPHost != "127.0.0.1" {
			t.Errorf("ResolveConfig().SMTPHost = %q, want %q", cfg.SMTPHost, "127.0.0.1")
		}
	})
}

func TestEnvMapping(t *testing.T) {
	// Verify all expected mappings exist
	expectedMappings := map[string]string{
		"MAILDEV_SMTP_PORT":        "OWLMAIL_SMTP_PORT",
		"MAILDEV_IP":               "OWLMAIL_SMTP_HOST",
		"MAILDEV_MAIL_DIRECTORY":   "OWLMAIL_MAIL_DIR",
		"MAILDEV_WEB_PORT":         "OWLMAIL_WEB_PORT",
		"MAILDEV_WEB_IP":           "OWLMAIL_WEB_HOST",
		"MAILDEV_WEB_USER":         "OWLMAIL_WEB_USER",
		"MAILDEV_WEB_PASS":         "OWLMAIL_WEB_PASSWORD",
		"MAILDEV_HTTPS":            "OWLMAIL_HTTPS_ENABLED",
		"MAILDEV_HTTPS_CERT":       "OWLMAIL_HTTPS_CERT",
		"MAILDEV_HTTPS_KEY":        "OWLMAIL_HTTPS_KEY",
		"MAILDEV_OUTGOING_HOST":    "OWLMAIL_OUTGOING_HOST",
		"MAILDEV_OUTGOING_PORT":    "OWLMAIL_OUTGOING_PORT",
		"MAILDEV_OUTGOING_USER":    "OWLMAIL_OUTGOING_USER",
		"MAILDEV_OUTGOING_PASS":    "OWLMAIL_OUTGOING_PASSWORD",
		"MAILDEV_OUTGOING_SECURE":  "OWLMAIL_OUTGOING_SECURE",
		"MAILDEV_AUTO_RELAY":       "OWLMAIL_AUTO_RELAY",
		"MAILDEV_AUTO_RELAY_ADDR":  "OWLMAIL_AUTO_RELAY_ADDR",
		"MAILDEV_AUTO_RELAY_RULES": "OWLMAIL_AUTO_RELAY_RULES",
		"MAILDEV_INCOMING_USER":    "OWLMAIL_SMTP_USER",
		"MAILDEV_INCOMING_PASS":    "OWLMAIL_SMTP_PASSWORD",
		"MAILDEV_INCOMING_SECURE":  "OWLMAIL_TLS_ENABLED",
		"MAILDEV_INCOMING_CERT":    "OWLMAIL_TLS_CERT",
		"MAILDEV_INCOMING_KEY":     "OWLMAIL_TLS_KEY",
	}

	for maildevKey, expectedOwlmailKey := range expectedMappings {
		t.Run(maildevKey, func(t *testing.T) {
			actualOwlmailKey, ok := EnvMapping[maildevKey]
			if !ok {
				t.Errorf("EnvMapping missing key %q", maildevKey)
				return
			}
			if actualOwlmailKey != expectedOwlmailKey {
				t.Errorf("EnvMapping[%q] = %q, want %q", maildevKey, actualOwlmailKey, expectedOwlmailKey)
			}
		})
	}
}

func TestEnvMappingCount(t *testing.T) {
	expectedCount := 23
	if len(EnvMapping) != expectedCount {
		t.Errorf("len(EnvMapping) = %d, want %d", len(EnvMapping), expectedCount)
	}
}

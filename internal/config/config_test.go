package config

import (
	"flag"
	"strings"
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
		{"OWLMAIL_BASE_PATHNAME", "MAILDEV_BASE_PATHNAME"},
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

func TestBasePathnameConfiguration(t *testing.T) {
	envMgr := testutil.NewEnvManager()
	defer envMgr.Cleanup()

	_ = envMgr.Set("OWLMAIL_BASE_PATHNAME", "/owlmail/")
	fs := flag.NewFlagSet("base-pathname", flag.ContinueOnError)
	refs := DefineFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatal(err)
	}
	if got := ResolveConfig(fs, refs).BasePathname; got != "/owlmail/" {
		t.Fatalf("OWLMAIL_BASE_PATHNAME resolved to %q", got)
	}

	_ = envMgr.Set("MAILDEV_BASE_PATHNAME", "maildev")
	if got := ResolveConfig(fs, refs).BasePathname; got != "maildev" {
		t.Fatalf("MAILDEV_BASE_PATHNAME resolved to %q, want compatibility precedence", got)
	}

	fs = flag.NewFlagSet("base-pathname-flag", flag.ContinueOnError)
	refs = DefineFlags(fs)
	if err := fs.Parse([]string{"-base-pathname", "/cli/"}); err != nil {
		t.Fatal(err)
	}
	if got := ResolveConfig(fs, refs).BasePathname; got != "/cli/" {
		t.Fatalf("-base-pathname resolved to %q, want CLI precedence", got)
	}
}

func TestMailDevRESTCompatConfiguration(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		refs := DefineFlags(fs)
		if got := ResolveConfig(fs, refs).MailDevRESTCompat; got {
			t.Fatal("MailDev REST compatibility must be disabled by default")
		}
	})

	t.Run("environment enables facade", func(t *testing.T) {
		t.Setenv("OWLMAIL_MAILDEV_REST_COMPAT", "true")
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		refs := DefineFlags(fs)
		if got := ResolveConfig(fs, refs).MailDevRESTCompat; !got {
			t.Fatal("OWLMAIL_MAILDEV_REST_COMPAT=true did not enable facade")
		}
	})

	t.Run("CLI has priority", func(t *testing.T) {
		t.Setenv("OWLMAIL_MAILDEV_REST_COMPAT", "true")
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		refs := DefineFlags(fs)
		if err := fs.Parse([]string{"-maildev-rest-compat=false"}); err != nil {
			t.Fatal(err)
		}
		if got := ResolveConfig(fs, refs).MailDevRESTCompat; got {
			t.Fatal("explicit CLI false did not override the environment")
		}
	})
}

func TestOutgoingTransportPrecedence(t *testing.T) {
	t.Run("CLI secure overrides OWLMAIL TLS mode", func(t *testing.T) {
		t.Setenv("OWLMAIL_OUTGOING_TLS_MODE", "starttls")
		fs := flag.NewFlagSet("outgoing-secure-cli", flag.ContinueOnError)
		refs := DefineFlags(fs)
		if err := fs.Parse([]string{"-outgoing-secure"}); err != nil {
			t.Fatal(err)
		}
		config := ResolveConfig(fs, refs)
		if !config.OutgoingSecure || config.OutgoingTLSMode != "" {
			t.Fatalf("outgoing transport = secure:%v mode:%q, want CLI SMTPS alias", config.OutgoingSecure, config.OutgoingTLSMode)
		}
	})

	t.Run("explicit CLI false overrides OWLMAIL TLS mode", func(t *testing.T) {
		t.Setenv("OWLMAIL_OUTGOING_TLS_MODE", "smtps")
		fs := flag.NewFlagSet("outgoing-plain-cli", flag.ContinueOnError)
		refs := DefineFlags(fs)
		if err := fs.Parse([]string{"-outgoing-secure=false"}); err != nil {
			t.Fatal(err)
		}
		config := ResolveConfig(fs, refs)
		if config.OutgoingSecure || config.OutgoingTLSMode != "" {
			t.Fatalf("outgoing transport = secure:%v mode:%q, want CLI plain alias", config.OutgoingSecure, config.OutgoingTLSMode)
		}
	})

	t.Run("CLI TLS mode overrides MAILDEV secure", func(t *testing.T) {
		t.Setenv("MAILDEV_OUTGOING_SECURE", "true")
		fs := flag.NewFlagSet("outgoing-mode-cli", flag.ContinueOnError)
		refs := DefineFlags(fs)
		if err := fs.Parse([]string{"-outgoing-tls-mode", "starttls"}); err != nil {
			t.Fatal(err)
		}
		config := ResolveConfig(fs, refs)
		if config.OutgoingSecure || config.OutgoingTLSMode != "starttls" {
			t.Fatalf("outgoing transport = secure:%v mode:%q, want CLI STARTTLS", config.OutgoingSecure, config.OutgoingTLSMode)
		}
	})

	t.Run("MAILDEV secure overrides OWLMAIL TLS mode", func(t *testing.T) {
		t.Setenv("MAILDEV_OUTGOING_SECURE", "true")
		t.Setenv("OWLMAIL_OUTGOING_TLS_MODE", "plain")
		fs := flag.NewFlagSet("outgoing-maildev-env", flag.ContinueOnError)
		refs := DefineFlags(fs)
		if err := fs.Parse(nil); err != nil {
			t.Fatal(err)
		}
		config := ResolveConfig(fs, refs)
		if !config.OutgoingSecure || config.OutgoingTLSMode != "" {
			t.Fatalf("outgoing transport = secure:%v mode:%q, want MAILDEV SMTPS alias", config.OutgoingSecure, config.OutgoingTLSMode)
		}
	})
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
	if cfg.SMTPMaxMessageMB != DefaultSMTPMaxMessageMB {
		t.Errorf("DefaultConfig().SMTPMaxMessageMB = %d, want %d", cfg.SMTPMaxMessageMB, DefaultSMTPMaxMessageMB)
	}
	if cfg.SMTPMaxConcurrency != DefaultSMTPMaxConcurrency {
		t.Errorf("DefaultConfig().SMTPMaxConcurrency = %d, want %d", cfg.SMTPMaxConcurrency, DefaultSMTPMaxConcurrency)
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
	if cfg.SMTPAuthRequireTLS {
		t.Error("DefaultConfig().SMTPAuthRequireTLS = true, want false")
	}
	if cfg.UseUUIDForEmailID != false {
		t.Errorf("DefaultConfig().UseUUIDForEmailID = %v, want %v", cfg.UseUUIDForEmailID, false)
	}
	if cfg.S3Enabled || cfg.S3Endpoint != "" || cfg.S3Bucket != "" || cfg.S3Region != DefaultS3Region || cfg.S3Prefix != DefaultS3Prefix || cfg.S3UsePathStyle || cfg.S3StartupCheck || cfg.S3HealthInterval != DefaultS3HealthCheckInterval || cfg.S3HealthTimeout != DefaultS3HealthCheckTimeout {
		t.Errorf("unexpected default S3 attachment config: %#v", cfg)
	}
	if cfg.WebhookConfig != "" {
		t.Errorf("DefaultConfig().WebhookConfig = %q, want empty", cfg.WebhookConfig)
	}
	if cfg.WebhookMaxConcurrency != 8 {
		t.Errorf("DefaultConfig().WebhookMaxConcurrency = %d, want 8", cfg.WebhookMaxConcurrency)
	}
	if cfg.MailRetentionDays != 0 || cfg.MailMaxMessages != 0 || cfg.MailMaxDiskMB != 0 || cfg.MailCleanupInterval != "1h" {
		t.Errorf("unexpected default storage policy: %#v", cfg)
	}
	if cfg.WebhookRedisURL != "" || cfg.WebhookRedisPrefix != "owlmail:webhooks" || cfg.WebhookShutdownTimeout != "15s" {
		t.Errorf("unexpected default webhook queue config: %#v", cfg)
	}
}

func TestSMTPMaxConcurrencyResolution(t *testing.T) {
	t.Run("environment", func(t *testing.T) {
		t.Setenv("OWLMAIL_SMTP_MAX_CONCURRENCY", "3")
		fs := flag.NewFlagSet("smtp-concurrency-env", flag.ContinueOnError)
		refs := DefineFlags(fs)
		if err := fs.Parse(nil); err != nil {
			t.Fatal(err)
		}
		cfg := ResolveConfig(fs, refs)
		if cfg.SMTPMaxConcurrency != 3 {
			t.Fatalf("SMTPMaxConcurrency = %d, want 3", cfg.SMTPMaxConcurrency)
		}
	})

	t.Run("zero is unlimited", func(t *testing.T) {
		t.Setenv("OWLMAIL_SMTP_MAX_CONCURRENCY", "0")
		fs := flag.NewFlagSet("smtp-concurrency-zero", flag.ContinueOnError)
		refs := DefineFlags(fs)
		if err := fs.Parse(nil); err != nil {
			t.Fatal(err)
		}
		cfg := ResolveConfig(fs, refs)
		if cfg.SMTPMaxConcurrency != 0 {
			t.Fatalf("SMTPMaxConcurrency = %d, want 0", cfg.SMTPMaxConcurrency)
		}
		if err := ValidateConfig(cfg); err != nil {
			t.Fatalf("unlimited config failed validation: %v", err)
		}
	})

	t.Run("CLI overrides invalid environment", func(t *testing.T) {
		t.Setenv("OWLMAIL_SMTP_MAX_CONCURRENCY", "not-a-number")
		fs := flag.NewFlagSet("smtp-concurrency-cli", flag.ContinueOnError)
		refs := DefineFlags(fs)
		if err := fs.Parse([]string{"-smtp-max-concurrency", "5"}); err != nil {
			t.Fatal(err)
		}
		cfg := ResolveConfig(fs, refs)
		if cfg.SMTPMaxConcurrency != 5 {
			t.Fatalf("SMTPMaxConcurrency = %d, want 5", cfg.SMTPMaxConcurrency)
		}
	})

	for _, value := range []string{"-1", "invalid", "8.5"} {
		t.Run("reject "+value, func(t *testing.T) {
			t.Setenv("OWLMAIL_SMTP_MAX_CONCURRENCY", value)
			fs := flag.NewFlagSet("smtp-concurrency-invalid", flag.ContinueOnError)
			refs := DefineFlags(fs)
			if err := fs.Parse(nil); err != nil {
				t.Fatal(err)
			}
			cfg := ResolveConfig(fs, refs)
			if err := ValidateConfig(cfg); err == nil || !strings.Contains(err.Error(), "non-negative integer") {
				t.Fatalf("ValidateConfig() error = %v", err)
			}
		})
	}

	t.Run("invalid CLI string", func(t *testing.T) {
		fs := flag.NewFlagSet("smtp-concurrency-invalid-cli", flag.ContinueOnError)
		_ = DefineFlags(fs)
		if err := fs.Parse([]string{"-smtp-max-concurrency", "invalid"}); err == nil {
			t.Fatal("invalid CLI integer was accepted")
		}
	})
}

func TestSMTPAuthRequireTLSResolution(t *testing.T) {
	t.Run("CLI flag", func(t *testing.T) {
		fs := flag.NewFlagSet("smtp-auth-require-tls", flag.ContinueOnError)
		refs := DefineFlags(fs)
		if err := fs.Parse([]string{"-smtp-auth-require-tls"}); err != nil {
			t.Fatal(err)
		}
		if cfg := ResolveConfig(fs, refs); !cfg.SMTPAuthRequireTLS {
			t.Fatal("CLI flag did not enable SMTP AUTH TLS requirement")
		}
	})

	t.Run("environment variable", func(t *testing.T) {
		t.Setenv("OWLMAIL_SMTP_AUTH_REQUIRE_TLS", "true")
		fs := flag.NewFlagSet("smtp-auth-require-tls-env", flag.ContinueOnError)
		refs := DefineFlags(fs)
		if err := fs.Parse(nil); err != nil {
			t.Fatal(err)
		}
		if cfg := ResolveConfig(fs, refs); !cfg.SMTPAuthRequireTLS {
			t.Fatal("OWLMAIL_SMTP_AUTH_REQUIRE_TLS did not enable SMTP AUTH TLS requirement")
		}
	})
}

func TestSMTPAndS3ConfigResolution(t *testing.T) {
	t.Run("CLI flags", func(t *testing.T) {
		fs := flag.NewFlagSet("smtp-s3-cli", flag.ContinueOnError)
		refs := DefineFlags(fs)
		err := fs.Parse([]string{
			"-smtp-max-message-mb", "256",
			"-s3-enabled",
			"-s3-endpoint", "http://minio:9000",
			"-s3-region", "test-region-1",
			"-s3-bucket", "owlmail-test",
			"-s3-prefix", "mail/attachments",
			"-s3-access-key", "access",
			"-s3-secret-key", "secret",
			"-s3-session-token", "token",
			"-s3-use-path-style",
			"-s3-startup-check",
			"-s3-health-check-interval", "15s",
			"-s3-health-check-timeout", "2s",
		})
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		cfg := ResolveConfig(fs, refs)
		if cfg.SMTPMaxMessageMB != 256 || !cfg.S3Enabled || cfg.S3Endpoint != "http://minio:9000" || cfg.S3Region != "test-region-1" || cfg.S3Bucket != "owlmail-test" || cfg.S3Prefix != "mail/attachments" || cfg.S3AccessKeyID != "access" || cfg.S3SecretAccessKey != "secret" || cfg.S3SessionToken != "token" || !cfg.S3UsePathStyle || !cfg.S3StartupCheck || cfg.S3HealthInterval != "15s" || cfg.S3HealthTimeout != "2s" {
			t.Fatalf("unexpected resolved CLI config: %#v", cfg)
		}
	})

	t.Run("environment variables", func(t *testing.T) {
		t.Setenv("OWLMAIL_SMTP_MAX_MESSAGE_MB", "512")
		t.Setenv("OWLMAIL_S3_ENABLED", "true")
		t.Setenv("OWLMAIL_S3_ENDPOINT", "https://objects.example.test")
		t.Setenv("OWLMAIL_S3_REGION", "region-2")
		t.Setenv("OWLMAIL_S3_BUCKET", "mail-bucket")
		t.Setenv("OWLMAIL_S3_PREFIX", "tenant/owlmail")
		t.Setenv("OWLMAIL_S3_ACCESS_KEY", "env-access")
		t.Setenv("OWLMAIL_S3_SECRET_KEY", "env-secret")
		t.Setenv("OWLMAIL_S3_SESSION_TOKEN", "env-token")
		t.Setenv("OWLMAIL_S3_USE_PATH_STYLE", "true")
		t.Setenv("OWLMAIL_S3_STARTUP_CHECK", "true")
		t.Setenv("OWLMAIL_S3_HEALTH_CHECK_INTERVAL", "20s")
		t.Setenv("OWLMAIL_S3_HEALTH_CHECK_TIMEOUT", "3s")

		fs := flag.NewFlagSet("smtp-s3-env", flag.ContinueOnError)
		refs := DefineFlags(fs)
		if err := fs.Parse(nil); err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		cfg := ResolveConfig(fs, refs)
		if cfg.SMTPMaxMessageMB != 512 || !cfg.S3Enabled || cfg.S3Endpoint != "https://objects.example.test" || cfg.S3Region != "region-2" || cfg.S3Bucket != "mail-bucket" || cfg.S3Prefix != "tenant/owlmail" || cfg.S3AccessKeyID != "env-access" || cfg.S3SecretAccessKey != "env-secret" || cfg.S3SessionToken != "env-token" || !cfg.S3UsePathStyle || !cfg.S3StartupCheck || cfg.S3HealthInterval != "20s" || cfg.S3HealthTimeout != "3s" {
			t.Fatalf("unexpected resolved environment config: %#v", cfg)
		}
	})
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
		"MAILDEV_BASE_PATHNAME":    "OWLMAIL_BASE_PATHNAME",
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
	expectedCount := 24
	if len(EnvMapping) != expectedCount {
		t.Errorf("len(EnvMapping) = %d, want %d", len(EnvMapping), expectedCount)
	}
}

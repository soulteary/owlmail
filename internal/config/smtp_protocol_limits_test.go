package config

import (
	"flag"
	"strings"
	"testing"
)

func TestSMTPProtocolLimitDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.SMTPReadTimeout != "10s" || cfg.SMTPWriteTimeout != "10s" || cfg.SMTPMaxRecipients != 50 {
		t.Fatalf("unexpected SMTP protocol defaults: %#v", cfg)
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("default configuration should be valid: %v", err)
	}
}

func TestSMTPProtocolLimitsResolveFlagsAndEnvironment(t *testing.T) {
	t.Setenv("OWLMAIL_SMTP_READ_TIMEOUT", "12s")
	t.Setenv("OWLMAIL_SMTP_WRITE_TIMEOUT", "13s")
	t.Setenv("OWLMAIL_SMTP_MAX_RECIPIENTS", "75")
	fs := flag.NewFlagSet("smtp-protocol-limits", flag.ContinueOnError)
	refs := DefineFlags(fs)
	if err := fs.Parse([]string{"-smtp-write-timeout=14s", "-smtp-max-recipients=80"}); err != nil {
		t.Fatal(err)
	}
	cfg := ResolveConfig(fs, refs)
	if cfg.SMTPReadTimeout != "12s" || cfg.SMTPWriteTimeout != "14s" || cfg.SMTPMaxRecipients != 80 {
		t.Fatalf("unexpected resolved SMTP protocol limits: %#v", cfg)
	}
}

func TestSMTPMaxRecipientsRejectsMalformedEnvironment(t *testing.T) {
	t.Setenv("OWLMAIL_SMTP_MAX_RECIPIENTS", "not-a-number")
	fs := flag.NewFlagSet("smtp-max-recipients-invalid", flag.ContinueOnError)
	refs := DefineFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatal(err)
	}
	cfg := ResolveConfig(fs, refs)
	if cfg.SMTPMaxRecipients != -1 {
		t.Fatalf("SMTPMaxRecipients = %d, want invalid sentinel -1", cfg.SMTPMaxRecipients)
	}
	if err := ValidateConfig(cfg); err == nil || !strings.Contains(err.Error(), "greater than zero") {
		t.Fatalf("ValidateConfig() error = %v, want recipient-limit validation error", err)
	}
}

func TestSMTPMaxRecipientsRejectsExplicitZero(t *testing.T) {
	for _, test := range []struct {
		name string
		env  bool
	}{
		{name: "flag"},
		{name: "environment", env: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fs := flag.NewFlagSet("smtp-max-recipients-zero", flag.ContinueOnError)
			refs := DefineFlags(fs)
			args := []string{"-smtp-max-recipients=0"}
			if test.env {
				t.Setenv("OWLMAIL_SMTP_MAX_RECIPIENTS", "0")
				args = nil
			}
			if err := fs.Parse(args); err != nil {
				t.Fatal(err)
			}
			cfg := ResolveConfig(fs, refs)
			if cfg.SMTPMaxRecipients != -1 {
				t.Fatalf("SMTPMaxRecipients = %d, want invalid sentinel -1", cfg.SMTPMaxRecipients)
			}
			if err := ValidateConfig(cfg); err == nil || !strings.Contains(err.Error(), "greater than zero") {
				t.Fatalf("ValidateConfig() error = %v, want recipient-limit validation error", err)
			}
		})
	}
}

func TestSMTPProtocolLimitsRejectInvalidValues(t *testing.T) {
	for name, mutate := range map[string]func(*Config){
		"read timeout":  func(cfg *Config) { cfg.SMTPReadTimeout = "never" },
		"write timeout": func(cfg *Config) { cfg.SMTPWriteTimeout = "0s" },
		"recipients":    func(cfg *Config) { cfg.SMTPMaxRecipients = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := DefaultConfig()
			mutate(cfg)
			if err := ValidateConfig(cfg); err == nil {
				t.Fatal("expected invalid SMTP protocol limit to fail")
			}
		})
	}
}

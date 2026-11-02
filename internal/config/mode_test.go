package config

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModeDefaultsAndPriority(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Mode != "standalone" {
		t.Fatal(cfg.Mode)
	}
	t.Setenv("OWLMAIL_MODE", "multiuser")
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	refs := DefineFlags(fs)
	if err := fs.Parse([]string{"-mode=standalone"}); err != nil {
		t.Fatal(err)
	}
	if got := ResolveConfig(fs, refs).Mode; got != "standalone" {
		t.Fatal(got)
	}
	if err := ValidateModeRuntime(DefaultConfig()); err != nil {
		t.Fatal(err)
	}
}
func TestMultiuserModeGate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = "multiuser"
	cfg.IdentityTrustedProxies = "127.0.0.1/32"
	cfg.ClaimPolicy = "verified-email"
	cfg.ControlDatabase = filepath.Join(t.TempDir(), "control.sqlite")
	cfg.MailDir = t.TempDir()
	cfg.SystemSMTPAccountsFile = "accounts.json"
	cfg.RecipientAllowedDomains = "example.com"
	cfg.TLSEnabled = true
	cfg.SMTPAuthRequireTLS = true
	if err := ValidateMode(cfg); err != nil {
		t.Fatal(err)
	}
	if err := ValidateModeRuntime(cfg); err == nil || !strings.Contains(err.Error(), "not available yet") {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Config){
		func(c *Config) { c.MCPEnabled = true }, func(c *Config) { c.WebPassword = "bad" },
		func(c *Config) { c.SMTPUser = "gorge" }, func(c *Config) { c.SMTPAuthRequireTLS = false },
		func(c *Config) { c.ClaimPolicy = "" }, func(c *Config) { c.IdentityTrustedProxies = "0.0.0.0/0" },
	} {
		copy := *cfg
		mutate(&copy)
		if err := ValidateMode(&copy); err == nil {
			t.Fatal("invalid multiuser config accepted")
		}
	}
}

func TestModeConfigFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owlmail.yaml")
	if err := os.WriteFile(path, []byte("mode: standalone\nidentity-namespace: test\nclaim-policy: explicit-history\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != "standalone" || cfg.IdentityNamespace != "test" || cfg.ClaimPolicy != "explicit-history" {
		t.Fatalf("lost flat configuration: %+v", cfg)
	}
}

func TestMultiuserQuotaAndRetentionValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = "multiuser"
	cfg.IdentityTrustedProxies = "127.0.0.1/32"
	cfg.ClaimPolicy = "verified-email"
	cfg.ControlDatabase = filepath.Join(t.TempDir(), "control.sqlite")
	cfg.MailDir = t.TempDir()
	cfg.SystemSMTPAccountsFile = "accounts.yaml"
	cfg.RecipientAllowedDomains = "example.com"
	cfg.TLSEnabled = true
	cfg.SMTPAuthRequireTLS = true
	for _, mutate := range []func(*Config){
		func(c *Config) { c.MailRetentionDays = 1 },
		func(c *Config) { c.MailMaxMessages = -1 },
		func(c *Config) { c.MailMaxDiskMB = -1 },
		func(c *Config) { c.MultiuserMaxPending = 0 }, func(c *Config) { c.MultiuserRCPTPerMinute = -1 },
		func(c *Config) { c.MultiuserAuthPerMinute = 0 }, func(c *Config) { c.MultiuserMaxMessages = 0 },
		func(c *Config) { c.MultiuserMaxDiskMB = int(^uint(0) >> 1) }, func(c *Config) { c.PendingRetentionDays = 0 },
		func(c *Config) { c.PendingRetentionDays = int(^uint(0) >> 1) }, func(c *Config) { c.PendingEmptyRetentionHours = -1 },
		func(c *Config) { c.PendingCleanupInterval = "not-a-duration" }, func(c *Config) { c.PendingCleanupInterval = "0s" },
	} {
		copy := *cfg
		mutate(&copy)
		if err := ValidateMode(&copy); err == nil {
			t.Fatalf("accepted invalid policy: %+v", copy)
		}
	}
	// These options must not impose a new policy on historical standalone config.
	cfg.Mode = "standalone"
	cfg.MultiuserMaxMessages = 0
	cfg.PendingCleanupInterval = ""
	if err := ValidateMode(cfg); err != nil {
		t.Fatal(err)
	}
}

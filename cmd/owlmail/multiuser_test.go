package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/soulteary/owlmail/internal/config"
	"golang.org/x/crypto/bcrypt"
)

func multiuserComponentConfig(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Mode = "multiuser"
	cfg.IdentityTrustedProxies = "127.0.0.1/32"
	cfg.ClaimPolicy = "verified-email"
	cfg.ControlDatabase = filepath.Join(root, "control.sqlite")
	cfg.MailDir = filepath.Join(root, "mail")
	cfg.SystemSMTPAccountsFile = filepath.Join(root, "accounts.yaml")
	cfg.RecipientAllowedDomains = "example.com"
	cfg.TLSEnabled = true
	cfg.SMTPAuthRequireTLS = true
	cfg.SMTPSPort = 0
	hash, err := bcrypt.GenerateFromPassword([]byte("component-test-secret"), 10)
	if err != nil {
		t.Fatal(err)
	}
	data := fmt.Sprintf("version: 1\naccounts:\n  - id: gorge\n    username: gorge\n    password-hash: %s\n    namespace: main\n    enabled: true\n    allowed-domains: [example.com]\n", hash)
	if err = os.WriteFile(cfg.SystemSMTPAccountsFile, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return cfg
}
func TestMultiuserAuthorityLifecycleAndReleaseGate(t *testing.T) {
	cfg := multiuserComponentConfig(t)
	if err := validateMCPStdioConfig(cfg); err == nil {
		t.Fatal("multiuser stdio MCP enabled")
	}
	if err := config.ValidateModeRuntime(cfg); err == nil {
		t.Fatal("public CLI release gate was removed")
	}
	server, err := createMailServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !server.RequiresOwnership() || server.MailboxControl() == nil {
		t.Fatal("component wiring omitted isolation authority")
	}
	if err = server.Close(); err != nil {
		t.Fatal(err)
	}
	// An existing directory must never recreate a lost authority database.
	if err = os.Remove(cfg.ControlDatabase); err != nil {
		t.Fatal(err)
	}
	if reopened, err := createMailServer(cfg); err == nil {
		_ = reopened.Close()
		t.Fatal("lost authority recreated")
	}
}
func TestMultiuserControlLocationAndUnsupportedIntegrations(t *testing.T) {
	cfg := multiuserComponentConfig(t)
	cfg.ControlDatabase = filepath.Join(cfg.MailDir, "control.sqlite")
	if err := validateControlLocation(cfg); err == nil {
		t.Fatal("authority inside message directory accepted")
	}
	cfg.ControlDatabase = filepath.Join(filepath.Dir(cfg.MailDir), "control.sqlite")
	for _, mutate := range []func(*config.Config){
		func(c *config.Config) { c.S3Enabled = true }, func(c *config.Config) { c.OutgoingHost = "smtp.example.com" },
		func(c *config.Config) { c.WebhookConfig = "webhooks.json" }, func(c *config.Config) { c.AutoRelay = true },
	} {
		copy := *cfg
		mutate(&copy)
		if err := config.ValidateMode(&copy); err == nil {
			t.Fatal("unsupported external integration accepted")
		}
	}
}
func TestStandaloneDoesNotInitializeAuthority(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ControlDatabase = filepath.Join(t.TempDir(), "control.sqlite")
	ingress, err := setupMultiuserIngress(cfg)
	if err != nil || ingress != nil {
		t.Fatal("standalone initialized multiuser ingress")
	}
	if _, err = os.Stat(cfg.ControlDatabase); !os.IsNotExist(err) {
		t.Fatal("standalone created authority file")
	}
}

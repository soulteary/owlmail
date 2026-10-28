package config

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

func TestEphemeralListenerConfiguration(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SMTPPort = 0
	cfg.WebPort = 0
	cfg.SMTPSPort = 0
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("listener zero rejected: %v", err)
	}
	cfg.OutgoingHost = "smtp.example.test"
	cfg.OutgoingPort = 0
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("outgoing zero accepted")
	}
	cfg.OutgoingHost = ""
	cfg.SMTPPort = -1
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("negative listener port accepted")
	}
	for _, args := range [][]string{{"-smtp", "0", "-web", "0", "-listen-info-file", "listeners.json"}, nil} {
		fs := flag.NewFlagSet("ephemeral", flag.ContinueOnError)
		refs := DefineFlags(fs)
		if err := fs.Parse(args); err != nil {
			t.Fatal(err)
		}
		got := ResolveConfig(fs, refs)
		if args != nil {
			if got.SMTPPort != 0 || got.WebPort != 0 || got.ListenInfoFile != "listeners.json" {
				t.Fatalf("explicit values lost: %+v", got)
			}
		} else if got.SMTPPort != 1025 || got.WebPort != 1080 {
			t.Fatal("default ports changed")
		}
	}
	t.Setenv("OWLMAIL_SMTP_PORT", "0")
	t.Setenv("OWLMAIL_WEB_PORT", "0")
	t.Setenv("OWLMAIL_LISTEN_INFO_FILE", "from-env.json")
	fs := flag.NewFlagSet("ephemeral-env", flag.ContinueOnError)
	refs := DefineFlags(fs)
	got := ResolveConfig(fs, refs)
	if got.SMTPPort != 0 || got.WebPort != 0 || got.ListenInfoFile != "from-env.json" {
		t.Fatal("dynamic environment values lost")
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("smtp: 0\nweb: 0\nlisten-info-file: from-file.json\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.SMTPPort != 0 || got.WebPort != 0 || got.ListenInfoFile != "from-file.json" {
		t.Fatal("dynamic file values lost")
	}
}

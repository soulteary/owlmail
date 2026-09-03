package config

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestConfigFileLayering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owlmail.yaml")
	if err := os.WriteFile(path, []byte("web: 8080\nmail-directory: /tmp/owlmail-from-file\nmetrics-enabled: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fileDefaults, err := LoadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if fileDefaults.WebPort != 8080 || fileDefaults.MailDir != "/tmp/owlmail-from-file" || !fileDefaults.MetricsEnabled {
		t.Fatalf("unexpected file config: %#v", fileDefaults)
	}

	t.Setenv("OWLMAIL_WEB_PORT", "9090")
	fs := flag.NewFlagSet("environment", flag.ContinueOnError)
	refs := DefineFlagsWithDefaults(fs, fileDefaults)
	_ = fs.Parse(nil)
	if got := ResolveConfig(fs, refs).WebPort; got != 9090 {
		t.Fatalf("environment layer = %d, want 9090", got)
	}

	fs = flag.NewFlagSet("cli", flag.ContinueOnError)
	refs = DefineFlagsWithDefaults(fs, fileDefaults)
	_ = fs.Parse([]string{"-web", "7070"})
	if got := ResolveConfig(fs, refs).WebPort; got != 7070 {
		t.Fatalf("CLI layer = %d, want 7070", got)
	}
}

func TestConfigFileSecretsAreNotFlagDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owlmail.yaml")
	secrets := []string{"value-web-991", "value-outgoing-992", "value-smtp-993", "value-access-994", "value-private-995", "value-session-996"}
	content := "web-password: value-web-991\noutgoing-pass: value-outgoing-992\nsmtp-password: value-smtp-993\ns3-access-key: value-access-994\ns3-secret-key: value-private-995\ns3-session-token: value-session-996\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	fileDefaults, err := LoadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fs := flag.NewFlagSet("redacted-help", flag.ContinueOnError)
	var output bytes.Buffer
	fs.SetOutput(&output)
	refs := DefineFlagsWithDefaults(fs, fileDefaults)
	fs.PrintDefaults()
	for _, secret := range secrets {
		if strings.Contains(output.String(), secret) {
			t.Fatalf("flag defaults exposed %q: %s", secret, output.String())
		}
	}
	if cfg := ResolveConfig(fs, refs); cfg.WebPassword != "value-web-991" || cfg.OutgoingPass != "value-outgoing-992" || cfg.SMTPPassword != "value-smtp-993" || cfg.S3SecretAccessKey != "value-private-995" {
		t.Fatalf("redacting flag defaults changed resolution: %#v", cfg)
	}
}

func TestConfigFileReadLimitCoversZeroSizedSources(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("/dev/zero is Unix-specific")
	}
	if _, err := LoadConfigFile("/dev/zero"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("unbounded zero-sized config source error = %v", err)
	}
}

func TestLoadJSONConfigAndRejectInvalidFiles(t *testing.T) {
	directory := t.TempDir()
	jsonPath := filepath.Join(directory, "owlmail.json")
	if err := os.WriteFile(jsonPath, []byte(`{"smtp":2025,"mcp-enabled":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfigFile(jsonPath)
	if err != nil || cfg.SMTPPort != 2025 || !cfg.MCPEnabled {
		t.Fatalf("JSON config = %#v, %v", cfg, err)
	}

	for name, content := range map[string]string{
		"unknown.yaml":  "webb: 8080\n",
		"nested.yaml":   "web:\n  port: 8080\n",
		"null.yaml":     "web: null\n",
		"multiple.yaml": "web: 8080\n---\nmetrics-enabled: true\n",
	} {
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadConfigFile(path); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
}

func TestConfigFileFromArgs(t *testing.T) {
	t.Setenv("OWLMAIL_CONFIG_FILE", "environment.yaml")
	path, err := configFileFromArgs([]string{"-web", "8080", "--config=cli.json"})
	if err != nil || path != "cli.json" {
		t.Fatalf("path = %q, err = %v", path, err)
	}
	if _, err := configFileFromArgs([]string{"-config"}); err == nil || !strings.Contains(err.Error(), "requires a path") {
		t.Fatalf("missing path error = %v", err)
	}
	for name, args := range map[string][]string{
		"double dash":      {"--", "-config=ignored.yaml"},
		"positional":       {"serve", "-config=ignored.yaml"},
		"other flag value": {"-mail-directory", "-config=mail-directory-value"},
	} {
		path, err := configFileFromArgs(args)
		if err != nil || path != "environment.yaml" {
			t.Fatalf("%s: path = %q, err = %v", name, path, err)
		}
	}
}

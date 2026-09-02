package config

import (
	"flag"
	"os"
	"path/filepath"
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
		"unknown.yaml": "webb: 8080\n",
		"nested.yaml":  "web:\n  port: 8080\n",
		"null.yaml":    "web: null\n",
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
}

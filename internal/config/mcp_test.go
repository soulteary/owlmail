package config

import (
	"flag"
	"testing"
)

func TestMCPConfigurationDefaultsAndResolution(t *testing.T) {
	defaults := DefaultConfig()
	if defaults.MCPEnabled || defaults.MCPSessionTimeout != DefaultMCPSessionTimeout || defaults.MCPShutdownTimeout != DefaultMCPShutdownTimeout || defaults.WebExternalURL != "" {
		t.Fatalf("unexpected MCP defaults: %#v", defaults)
	}

	t.Run("environment", func(t *testing.T) {
		t.Setenv("OWLMAIL_MCP_ENABLED", "true")
		t.Setenv("OWLMAIL_MCP_SESSION_TIMEOUT", "10m")
		t.Setenv("OWLMAIL_MCP_SHUTDOWN_TIMEOUT", "3s")
		t.Setenv("OWLMAIL_WEB_EXTERNAL_URL", "https://mail.example.test")
		fs := flag.NewFlagSet("mcp-environment", flag.ContinueOnError)
		refs := DefineFlags(fs)
		if err := fs.Parse(nil); err != nil {
			t.Fatal(err)
		}
		cfg := ResolveConfig(fs, refs)
		if !cfg.MCPEnabled || cfg.MCPSessionTimeout != "10m" || cfg.MCPShutdownTimeout != "3s" || cfg.WebExternalURL != "https://mail.example.test" {
			t.Fatalf("environment was not resolved: %#v", cfg)
		}
	})

	t.Run("CLI precedence", func(t *testing.T) {
		t.Setenv("OWLMAIL_MCP_ENABLED", "false")
		t.Setenv("OWLMAIL_MCP_SESSION_TIMEOUT", "10m")
		t.Setenv("OWLMAIL_MCP_SHUTDOWN_TIMEOUT", "3s")
		t.Setenv("OWLMAIL_WEB_EXTERNAL_URL", "https://environment.example.test")
		fs := flag.NewFlagSet("mcp-cli", flag.ContinueOnError)
		refs := DefineFlags(fs)
		if err := fs.Parse([]string{"-mcp-enabled", "-mcp-session-timeout", "1h", "-mcp-shutdown-timeout", "8s", "-web-external-url", "https://cli.example.test"}); err != nil {
			t.Fatal(err)
		}
		cfg := ResolveConfig(fs, refs)
		if !cfg.MCPEnabled || cfg.MCPSessionTimeout != "1h" || cfg.MCPShutdownTimeout != "8s" || cfg.WebExternalURL != "https://cli.example.test" {
			t.Fatalf("CLI values did not take precedence: %#v", cfg)
		}
	})
}

func TestNormalizeWebExternalURL(t *testing.T) {
	for _, test := range []struct {
		value string
		want  string
		valid bool
	}{
		{value: "", want: "", valid: true},
		{value: "https://mail.example.test/", want: "https://mail.example.test", valid: true},
		{value: "http://mail.example.test:8080", want: "http://mail.example.test:8080", valid: true},
		{value: "mail.example.test", valid: false},
		{value: "ftp://mail.example.test", valid: false},
		{value: "https://user:pass@mail.example.test", valid: false},
		{value: "https://mail.example.test/owlmail", valid: false},
		{value: "https://mail.example.test?token=secret", valid: false},
		{value: "https://mail.example.test/#fragment", valid: false},
	} {
		got, err := NormalizeWebExternalURL(test.value)
		if test.valid && (err != nil || got != test.want) {
			t.Errorf("NormalizeWebExternalURL(%q) = %q, %v; want %q", test.value, got, err, test.want)
		}
		if !test.valid && err == nil {
			t.Errorf("NormalizeWebExternalURL(%q) unexpectedly succeeded: %q", test.value, got)
		}
	}

	cfg := DefaultConfig()
	cfg.WebExternalURL = "https://mail.example.test"
	cfg.WebExternalScheme = "http"
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("ValidateConfig accepted conflicting external URL and scheme")
	}
}

func TestValidateMCPTimeouts(t *testing.T) {
	for _, test := range []struct {
		name     string
		session  string
		shutdown string
	}{
		{name: "invalid session", session: "never", shutdown: "5s"},
		{name: "zero session", session: "0s", shutdown: "5s"},
		{name: "invalid shutdown", session: "30m", shutdown: "never"},
		{name: "zero shutdown", session: "30m", shutdown: "0s"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.MCPSessionTimeout = test.session
			cfg.MCPShutdownTimeout = test.shutdown
			if err := ValidateConfig(cfg); err == nil {
				t.Fatal("ValidateConfig accepted an invalid MCP timeout")
			}
		})
	}
}

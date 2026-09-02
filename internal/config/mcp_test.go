package config

import (
	"flag"
	"testing"
)

func TestMCPConfigurationDefaultsAndResolution(t *testing.T) {
	defaults := DefaultConfig()
	if defaults.MCPEnabled || defaults.MCPSessionTimeout != DefaultMCPSessionTimeout || defaults.MCPShutdownTimeout != DefaultMCPShutdownTimeout {
		t.Fatalf("unexpected MCP defaults: %#v", defaults)
	}

	t.Run("environment", func(t *testing.T) {
		t.Setenv("OWLMAIL_MCP_ENABLED", "true")
		t.Setenv("OWLMAIL_MCP_SESSION_TIMEOUT", "10m")
		t.Setenv("OWLMAIL_MCP_SHUTDOWN_TIMEOUT", "3s")
		fs := flag.NewFlagSet("mcp-environment", flag.ContinueOnError)
		refs := DefineFlags(fs)
		if err := fs.Parse(nil); err != nil {
			t.Fatal(err)
		}
		cfg := ResolveConfig(fs, refs)
		if !cfg.MCPEnabled || cfg.MCPSessionTimeout != "10m" || cfg.MCPShutdownTimeout != "3s" {
			t.Fatalf("environment was not resolved: %#v", cfg)
		}
	})

	t.Run("CLI precedence", func(t *testing.T) {
		t.Setenv("OWLMAIL_MCP_ENABLED", "false")
		t.Setenv("OWLMAIL_MCP_SESSION_TIMEOUT", "10m")
		t.Setenv("OWLMAIL_MCP_SHUTDOWN_TIMEOUT", "3s")
		fs := flag.NewFlagSet("mcp-cli", flag.ContinueOnError)
		refs := DefineFlags(fs)
		if err := fs.Parse([]string{"-mcp-enabled", "-mcp-session-timeout", "1h", "-mcp-shutdown-timeout", "8s"}); err != nil {
			t.Fatal(err)
		}
		cfg := ResolveConfig(fs, refs)
		if !cfg.MCPEnabled || cfg.MCPSessionTimeout != "1h" || cfg.MCPShutdownTimeout != "8s" {
			t.Fatalf("CLI values did not take precedence: %#v", cfg)
		}
	})
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

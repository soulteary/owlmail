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

func TestParseMCPAllowedOrigins(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
		want  []string
	}{
		{name: "empty", value: "", want: []string{}},
		{name: "single", value: "https://inspector.example", want: []string{"https://inspector.example"}},
		{
			name:  "comma and whitespace separated",
			value: "https://a.example:8443, http://b.example\nhttps://c.example",
			want:  []string{"https://a.example:8443", "http://b.example", "https://c.example"},
		},
		{name: "wildcard", value: "*", want: []string{MCPAllowAnyOrigin}},
		{name: "trailing slash is a bare origin", value: "https://a.example/", want: []string{"https://a.example"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			origins, err := ParseMCPAllowedOrigins(test.value)
			if err != nil {
				t.Fatalf("ParseMCPAllowedOrigins(%q) error = %v", test.value, err)
			}
			if len(origins) != len(test.want) {
				t.Fatalf("ParseMCPAllowedOrigins(%q) = %v, want %v", test.value, origins, test.want)
			}
			for index, origin := range origins {
				if origin != test.want[index] {
					t.Fatalf("ParseMCPAllowedOrigins(%q) = %v, want %v", test.value, origins, test.want)
				}
			}
		})
	}

	for _, test := range []struct {
		name  string
		value string
	}{
		{name: "not a URL", value: "inspector.example"},
		{name: "unsupported scheme", value: "ws://inspector.example"},
		{name: "carries a path", value: "https://inspector.example/mcp"},
		{name: "carries credentials", value: "https://user:pass@inspector.example"},
		{name: "wildcard combined with an origin", value: "*,https://inspector.example"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseMCPAllowedOrigins(test.value); err == nil {
				t.Fatalf("ParseMCPAllowedOrigins(%q) accepted an invalid value", test.value)
			}
			cfg := DefaultConfig()
			cfg.MCPAllowedOrigins = test.value
			if err := ValidateConfig(cfg); err == nil {
				t.Fatal("ValidateConfig accepted an invalid MCP allowed origin")
			}
		})
	}
}

func TestParseWebAllowedOrigins(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
		want  []string
	}{
		{name: "empty", value: "", want: []string{}},
		{name: "single", value: "https://console.example", want: []string{"https://console.example"}},
		{
			name:  "comma and whitespace separated",
			value: "https://a.example:8443, http://b.example\nhttps://c.example",
			want:  []string{"https://a.example:8443", "http://b.example", "https://c.example"},
		},
		{name: "wildcard", value: "*", want: []string{WebAllowAnyOrigin}},
		{name: "trailing slash is a bare origin", value: "https://a.example/", want: []string{"https://a.example"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			origins, err := ParseWebAllowedOrigins(test.value)
			if err != nil {
				t.Fatalf("ParseWebAllowedOrigins(%q) error = %v", test.value, err)
			}
			if len(origins) != len(test.want) {
				t.Fatalf("ParseWebAllowedOrigins(%q) = %v, want %v", test.value, origins, test.want)
			}
			for index, origin := range origins {
				if origin != test.want[index] {
					t.Fatalf("ParseWebAllowedOrigins(%q) = %v, want %v", test.value, origins, test.want)
				}
			}
		})
	}

	for _, test := range []struct {
		name  string
		value string
	}{
		{name: "not a URL", value: "console.example"},
		{name: "unsupported scheme", value: "ws://console.example"},
		{name: "carries a path", value: "https://console.example/inbox"},
		{name: "carries credentials", value: "https://user:pass@console.example"},
		// Pairing the opt-out with a named origin silently opens a list its
		// author meant to keep narrow, so the parser refuses it rather than
		// leaving the mistake to be found in a browser.
		{name: "wildcard combined with an origin", value: "*,https://console.example"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseWebAllowedOrigins(test.value); err == nil {
				t.Fatalf("ParseWebAllowedOrigins(%q) accepted an invalid value", test.value)
			}
			cfg := DefaultConfig()
			cfg.WebAllowedOrigins = test.value
			if err := ValidateConfig(cfg); err == nil {
				t.Fatal("ValidateConfig accepted an invalid Web allowed origin")
			}
		})
	}
}

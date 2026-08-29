package webhook

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigStrictJSON(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantError string
	}{
		{
			name:    "valid",
			content: `{"version":1,"targets":[{"name":"test","url":"https://example.com/hook"}]}`,
		},
		{
			name:      "unknown field",
			content:   `{"targets":[],"targetz":[]}`,
			wantError: "unknown field",
		},
		{
			name:      "trailing value",
			content:   `{"targets":[]} {"targets":[]}`,
			wantError: "multiple JSON values",
		},
		{
			name:      "invalid json",
			content:   `{"targets":`,
			wantError: "decode webhook config",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filePath := filepath.Join(t.TempDir(), "webhooks.json")
			if err := os.WriteFile(filePath, []byte(test.content), 0600); err != nil {
				t.Fatal(err)
			}
			config, err := LoadConfig(filePath)
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("LoadConfig() error = %v", err)
				}
				if config.Version != 1 || len(config.Targets) != 1 {
					t.Fatalf("LoadConfig() = %#v", config)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("LoadConfig() error = %v, want substring %q", err, test.wantError)
			}
		})
	}
}

func TestLoadConfigLimitsSize(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "webhooks.json")
	if err := os.WriteFile(filePath, bytes.Repeat([]byte{'x'}, maxConfigBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(filePath)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("LoadConfig() error = %v, want size error", err)
	}
}

func TestCompileConfigValidation(t *testing.T) {
	validTarget := func() Target {
		return Target{Name: "primary", URL: "https://example.com/hooks/owlmail"}
	}

	tests := []struct {
		name      string
		config    Config
		wantError string
	}{
		{name: "unsupported version", config: Config{Version: 2, Targets: []Target{validTarget()}}, wantError: "unsupported"},
		{name: "no targets", config: Config{Version: 1}, wantError: "at least one"},
		{name: "duplicate names", config: Config{Targets: []Target{validTarget(), validTarget()}}, wantError: "duplicate name"},
		{name: "missing name", config: Config{Targets: []Target{{URL: "https://example.com"}}}, wantError: "name is required"},
		{name: "name newline", config: Config{Targets: []Target{{Name: "bad\nname", URL: "https://example.com"}}}, wantError: "no newlines"},
		{name: "bad scheme", config: Config{Targets: []Target{{Name: "bad", URL: "file:///tmp/hook"}}}, wantError: "scheme"},
		{name: "missing host", config: Config{Targets: []Target{{Name: "bad", URL: "http:///hook"}}}, wantError: "host"},
		{name: "url credentials", config: Config{Targets: []Target{{Name: "bad", URL: "https://user:pass@example.com"}}}, wantError: "user information"},
		{name: "url fragment", config: Config{Targets: []Target{{Name: "bad", URL: "https://example.com/#secret"}}}, wantError: "fragments"},
		{name: "unsupported method", config: Config{Targets: []Target{{Name: "bad", URL: "https://example.com", Method: "GET"}}}, wantError: "not supported"},
		{name: "invalid timeout", config: Config{Targets: []Target{{Name: "bad", URL: "https://example.com", Timeout: "soon"}}}, wantError: "invalid timeout"},
		{name: "excessive timeout", config: Config{Targets: []Target{{Name: "bad", URL: "https://example.com", Timeout: "61s"}}}, wantError: "no more than"},
		{name: "too many retries", config: Config{Targets: []Target{{Name: "bad", URL: "https://example.com", Retries: 6}}}, wantError: "retries"},
		{name: "content type newline", config: Config{Targets: []Target{{Name: "bad", URL: "https://example.com", ContentType: "text/plain\r\nX-Test: yes"}}}, wantError: "contentType"},
		{name: "invalid header name", config: Config{Targets: []Target{{Name: "bad", URL: "https://example.com", Headers: map[string]string{"Bad Header": "x"}}}}, wantError: "invalid header"},
		{name: "managed header", config: Config{Targets: []Target{{Name: "bad", URL: "https://example.com", Headers: map[string]string{"Content-Length": "10"}}}}, wantError: "managed"},
		{name: "header newline", config: Config{Targets: []Target{{Name: "bad", URL: "https://example.com", Headers: map[string]string{"X-Test": "x\ny"}}}}, wantError: "newline"},
		{name: "empty pattern", config: Config{Targets: []Target{{Name: "bad", URL: "https://example.com", Match: Match{Subject: []string{""}}}}}, wantError: "empty pattern"},
		{name: "invalid pattern", config: Config{Targets: []Target{{Name: "bad", URL: "https://example.com", Match: Match{Subject: []string{"["}}}}}, wantError: "invalid pattern"},
		{name: "invalid template", config: Config{Targets: []Target{{Name: "bad", URL: "https://example.com", BodyTemplate: "{{"}}}, wantError: "bodyTemplate"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewDispatcher(test.config, nil)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("NewDispatcher() error = %v, want substring %q", err, test.wantError)
			}
		})
	}
}

func TestCompileConfigExpandsEnvironmentAndDefaults(t *testing.T) {
	t.Setenv("OWLMAIL_TEST_HOOK_HOST", "example.com")
	t.Setenv("OWLMAIL_TEST_HOOK_TOKEN", "token-value")

	dispatcher, err := NewDispatcher(Config{Targets: []Target{{
		Name:    "primary",
		URL:     "https://${OWLMAIL_TEST_HOOK_HOST}/hooks/owlmail",
		Secret:  "${OWLMAIL_TEST_HOOK_TOKEN}",
		Headers: map[string]string{"Authorization": "Bearer ${OWLMAIL_TEST_HOOK_TOKEN}"},
	}}}, nil)
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}
	target := dispatcher.targets[0]
	if target.url != "https://example.com/hooks/owlmail" {
		t.Errorf("url = %q", target.url)
	}
	if target.method != "POST" || target.timeout != 5*time.Second || target.contentType != "application/json" {
		t.Errorf("defaults not applied: %#v", target)
	}
	if target.secret != "token-value" || target.headers.Get("Authorization") != "Bearer token-value" {
		t.Errorf("environment variables not expanded")
	}
}

func TestCompileConfigRejectsMissingEnvironment(t *testing.T) {
	const variable = "OWLMAIL_TEST_MISSING_VARIABLE_7D896"
	_ = os.Unsetenv(variable)
	_, err := NewDispatcher(Config{Targets: []Target{{
		Name: "primary",
		URL:  "https://example.com/${" + variable + "}",
	}}}, nil)
	if err == nil || !strings.Contains(err.Error(), variable) {
		t.Fatalf("NewDispatcher() error = %v, want missing environment variable", err)
	}
}

func TestCompileConfigRejectsEmptyEnvironment(t *testing.T) {
	const variable = "OWLMAIL_TEST_EMPTY_SECRET_42C1"
	t.Setenv(variable, "")
	_, err := NewDispatcher(Config{Targets: []Target{{
		Name:   "primary",
		URL:    "https://example.com/hooks/owlmail",
		Secret: "${" + variable + "}",
	}}}, nil)
	if err == nil || !strings.Contains(err.Error(), variable+" is empty") {
		t.Fatalf("NewDispatcher() error = %v, want empty environment variable error", err)
	}
}

func TestCompileConfigLimitsTargets(t *testing.T) {
	targets := make([]Target, maxTargets+1)
	for index := range targets {
		targets[index] = Target{Name: "target", URL: "https://example.com"}
	}
	_, err := NewDispatcher(Config{Targets: targets}, nil)
	if err == nil || !strings.Contains(err.Error(), "maximum") {
		t.Fatalf("NewDispatcher() error = %v, want maximum target error", err)
	}
}

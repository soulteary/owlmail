package webhook

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDocumentedExampleConfigurations(t *testing.T) {
	t.Setenv("OWLMAIL_WEBHOOK_URL", "http://127.0.0.1:18080/custom")
	t.Setenv("OWLMAIL_WEBHOOK_TOKEN", "example-token")
	t.Setenv("OWLMAIL_WEBHOOK_SECRET", "example-secret")

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))

	tests := []struct {
		path        string
		targetCount int
	}{
		{path: "examples/webhooks.json", targetCount: 1},
		{path: "examples/webhooks/minimal.json", targetCount: 1},
		{path: "examples/webhooks/filtered-alerts.json", targetCount: 1},
		{path: "examples/webhooks/custom-json.json", targetCount: 1},
		{path: "examples/webhooks/multiple-targets.json", targetCount: 2},
		{path: "examples/webhooks/plain-text.json", targetCount: 1},
		{path: "examples/webhooks/soulteary-webhook/owlmail.json", targetCount: 1},
	}

	payload := EmailPayload{
		ID:              "example-id",
		Time:            time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC),
		Subject:         "Critical \"example\" alert",
		From:            []string{"sender@example.test"},
		To:              []string{"ops@example.test"},
		Text:            "Example\n\"message\" body",
		Size:            20,
		SizeHuman:       "20 B",
		AttachmentCount: 0,
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			config, err := LoadConfig(filepath.Join(repositoryRoot, filepath.FromSlash(tt.path)))
			if err != nil {
				t.Fatalf("LoadConfig() error: %v", err)
			}
			targets, err := compileConfig(config)
			if err != nil {
				t.Fatalf("compileConfig() error: %v", err)
			}
			if len(targets) != tt.targetCount {
				t.Fatalf("target count = %d, want %d", len(targets), tt.targetCount)
			}
			for _, target := range targets {
				body, err := renderBody(target, payload)
				if err != nil {
					t.Fatalf("renderBody(%q) error: %v", target.name, err)
				}
				if len(body) == 0 {
					t.Errorf("renderBody(%q) produced an empty body", target.name)
				}
				if strings.HasPrefix(target.contentType, "application/json") && !json.Valid(body) {
					t.Errorf("renderBody(%q) produced invalid JSON: %s", target.name, body)
				}
			}
		})
	}
}

func TestDocumentedMatchingScenarios(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))

	tests := []struct {
		name         string
		path         string
		to           string
		subject      string
		matchedNames string
	}{
		{
			name:         "filtered alert matches both fields",
			path:         "examples/webhooks/filtered-alerts.json",
			to:           "ops@example.test",
			subject:      "Critical service alert",
			matchedNames: "filtered-alerts",
		},
		{
			name:    "filtered alert rejects a routine subject",
			path:    "examples/webhooks/filtered-alerts.json",
			to:      "ops@example.test",
			subject: "Weekly report",
		},
		{
			name:    "filtered alert rejects another recipient",
			path:    "examples/webhooks/filtered-alerts.json",
			to:      "inbox@example.test",
			subject: "Critical service alert",
		},
		{
			name:         "critical message fans out to both targets",
			path:         "examples/webhooks/multiple-targets.json",
			to:           "ops@example.test",
			subject:      "Critical outage",
			matchedNames: "all-messages,critical-alerts",
		},
		{
			name:         "routine message uses the archive target only",
			path:         "examples/webhooks/multiple-targets.json",
			to:           "inbox@example.test",
			subject:      "Weekly report",
			matchedNames: "all-messages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := LoadConfig(filepath.Join(repositoryRoot, filepath.FromSlash(tt.path)))
			if err != nil {
				t.Fatalf("LoadConfig() error: %v", err)
			}
			targets, err := compileConfig(config)
			if err != nil {
				t.Fatalf("compileConfig() error: %v", err)
			}

			payload := EmailPayload{To: []string{tt.to}, Subject: tt.subject}
			matched := make([]string, 0, len(targets))
			for _, target := range targets {
				if target.matches(payload) {
					matched = append(matched, target.name)
				}
			}
			if got := strings.Join(matched, ","); got != tt.matchedNames {
				t.Fatalf("matched targets = %q, want %q", got, tt.matchedNames)
			}
		})
	}
}

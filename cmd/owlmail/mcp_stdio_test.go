package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soulteary/owlmail/internal/config"
)

func TestRunMCPStdioHelp(t *testing.T) {
	var stderr bytes.Buffer
	err := runMCPStdio(context.Background(), []string{"-h"}, &stderr)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("runMCPStdio(-h) error = %v", err)
	}
	if !strings.Contains(stderr.String(), "mail-directory") {
		t.Fatalf("help did not include mailbox configuration: %s", stderr.String())
	}
}

func TestValidateMCPStdioConfigIgnoresUnusedServerSettings(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.SMTPMaxMessageMB = -1
	cfg.MailCleanupInterval = "never"
	cfg.OutgoingHost = "smtp.example.test"
	cfg.OutgoingPort = -1
	cfg.WebhookMaxConcurrency = -1
	cfg.WebhookRedisPrefix = ""
	cfg.WebhookShutdownTimeout = "never"
	cfg.S3Enabled = true
	cfg.S3Bucket = ""

	if err := validateMCPStdioConfig(cfg); err != nil {
		t.Fatalf("stdio validation rejected an unused server setting: %v", err)
	}
	cfg.LogFormat = "invalid"
	if err := validateMCPStdioConfig(cfg); err == nil {
		t.Fatal("stdio validation accepted an invalid active log format")
	}
}

func TestRunMCPStdioFormatsTerminalErrorsAsJSON(t *testing.T) {
	var stderr bytes.Buffer
	err := runMCPStdio(context.Background(), []string{
		"-log-format", "json",
		"-mail-directory", filepath.Join(t.TempDir(), "missing"),
	}, &stderr)
	var reported *reportedMCPStdioError
	if !errors.As(err, &reported) {
		t.Fatalf("runMCPStdio() error = %v, want reportedMCPStdioError", err)
	}
	var entry map[string]interface{}
	if decodeErr := json.Unmarshal(bytes.TrimSpace(stderr.Bytes()), &entry); decodeErr != nil {
		t.Fatalf("terminal stderr is not JSON: %q: %v", stderr.String(), decodeErr)
	}
	if entry["level"] != "error" || !strings.Contains(entry["message"].(string), "MCP stdio bridge failed") {
		t.Fatalf("terminal JSON entry = %#v", entry)
	}
}


func TestReportMCPStdioResult(t *testing.T) {
	reportedCause := errors.New("already logged")
	for _, test := range []struct {
		name       string
		err        error
		wantCode   int
		wantOutput bool
	}{
		{name: "success", wantCode: 0},
		{name: "help", err: flag.ErrHelp, wantCode: 0},
		{name: "canceled", err: context.Canceled, wantCode: 0},
		{name: "unreported failure", err: errors.New("transport failed"), wantCode: 1, wantOutput: true},
		{name: "reported failure", err: &reportedMCPStdioError{err: reportedCause}, wantCode: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer
			if code := reportMCPStdioResult(test.err, &stderr); code != test.wantCode {
				t.Fatalf("exit code = %d, want %d", code, test.wantCode)
			}
			if got := stderr.Len() > 0; got != test.wantOutput {
				t.Fatalf("stderr present = %t, want %t: %q", got, test.wantOutput, stderr.String())
			}
		})
	}
}

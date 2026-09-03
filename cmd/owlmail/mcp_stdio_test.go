package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
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

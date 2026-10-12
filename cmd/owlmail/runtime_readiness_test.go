package main

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/config"
	"github.com/soulteary/owlmail/internal/mailserver"
)

func TestRuntimeProbeRejectsMissingMalformedAndStaleSnapshots(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MailDir = t.TempDir()
	if err := probeRuntimeReadiness(cfg); err == nil {
		t.Fatal("missing owner snapshot accepted")
	}
	writeHealthyRuntimeSnapshot(t, cfg)
	if err := probeRuntimeReadiness(cfg); err != nil {
		t.Fatal(err)
	}
	path := mailserver.RuntimeReadinessPath(cfg.MailDir, cfg.SMTPHost, cfg.SMTPPort)
	for _, data := range [][]byte{[]byte("invalid JSON"), make([]byte, 65537), []byte(`{"version":1,"pid":1,"host":"localhost","port":1025,"updated_at":"2000-01-01T00:00:00Z"}`)} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := probeRuntimeReadiness(cfg); err == nil {
			t.Fatal("invalid snapshot accepted")
		}
	}
}

func TestRuntimeProbeBoundsBlockedFilesystemRead(t *testing.T) {
	mkfifo, err := exec.LookPath("mkfifo")
	if err != nil {
		t.Skip("FIFO unavailable")
	}
	cfg := config.DefaultConfig()
	cfg.MailDir = t.TempDir()
	path := mailserver.RuntimeReadinessPath(cfg.MailDir, cfg.SMTPHost, cfg.SMTPPort)
	if err := exec.Command(mkfifo, path).Run(); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := probeRuntimeReadiness(cfg); err == nil {
		t.Fatal("blocked read reported ready")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("probe blocked for %v", elapsed)
	}
	// Release the timed-out reader so this regression does not leak a goroutine.
	done := make(chan error, 1)
	go func() { done <- os.WriteFile(path, []byte("{}"), 0600) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed-out reader did not release")
	}
}

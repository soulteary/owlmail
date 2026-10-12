package mailserver

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/attachmentstore"
	"github.com/soulteary/owlmail/internal/outgoing"
)

func TestRuntimeReadinessRejectsStaleAndUnreadyComponents(t *testing.T) {
	now := time.Now()
	healthy := RuntimeReadiness{Version: 1, PID: 1, Host: "localhost", Port: 1025, UpdatedAt: now, SMTP: true, Storage: attachmentstore.HealthStatus{State: attachmentstore.HealthReady, CheckedAt: now}}
	if err := healthy.Validate("localhost", 1025, now); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		change func(*RuntimeReadiness)
	}{
		{"stale snapshot", func(s *RuntimeReadiness) { s.UpdatedAt = now.Add(-6 * time.Second) }},
		{"future snapshot", func(s *RuntimeReadiness) { s.UpdatedAt = now.Add(time.Minute) }},
		{"wrong listener", func(s *RuntimeReadiness) { s.Port = 2525 }},
		{"SMTP stopped", func(s *RuntimeReadiness) { s.SMTP = false }},
		{"storage pending", func(s *RuntimeReadiness) { s.Storage.State = attachmentstore.HealthChecking }},
		{"stale storage", func(s *RuntimeReadiness) { s.Storage.CheckedAt = now.Add(-time.Minute) }},
		{"remote attachments unavailable", func(s *RuntimeReadiness) {
			s.Attachment = &attachmentstore.HealthStatus{State: attachmentstore.HealthUnready}
		}},
		{"worker missing", func(s *RuntimeReadiness) { s.Relay = &outgoing.RuntimeReadiness{Enabled: true, Capacity: 100} }},
		{"queue full", func(s *RuntimeReadiness) {
			s.Relay = &outgoing.RuntimeReadiness{Enabled: true, Workers: 1, Queued: 100, Capacity: 100}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			status := healthy
			test.change(&status)
			if err := status.Validate("localhost", 1025, now); err == nil {
				t.Fatal("unready state accepted")
			}
		})
	}
}

func TestRuntimeSnapshotPublicationAndShutdown(t *testing.T) {
	server, err := NewMailServerWithOptions(1025, "localhost", t.TempDir(), ServerOptions{RuntimeReadiness: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	server.smtpReady.Store(true)
	server.probeMailStorage()
	if err := server.publishRuntimeReadiness(); err != nil {
		t.Fatal(err)
	}
	path := RuntimeReadinessPath(server.mailDir, server.host, server.port)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("snapshot permissions: %v", info.Mode())
	}
	read := func() RuntimeReadiness {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var status RuntimeReadiness
		if err := json.Unmarshal(data, &status); err != nil {
			t.Fatal(err)
		}
		return status
	}
	if err := read().Validate("localhost", 1025, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	if err := server.publishRuntimeReadiness(); err != nil {
		t.Fatal(err)
	}
	if err := read().Validate("localhost", 1025, time.Now()); err == nil {
		t.Fatal("shutdown published ready")
	}
}

func TestReadOnlyObserverCannotPublishRuntimeReadiness(t *testing.T) {
	if server, err := NewMailServerWithOptions(1025, "localhost", t.TempDir(), ServerOptions{ReadOnly: true, RuntimeReadiness: true}); err == nil {
		_ = server.Close()
		t.Fatal("observer allowed to mutate runtime state")
	}
}

func TestRuntimeReadinessNamespaceCannotBeUsedAsMailboxIndex(t *testing.T) {
	directory := t.TempDir()
	if err := ValidateMailboxIndexPath(directory, RuntimeReadinessPath(directory, "localhost", 1025)); err == nil {
		t.Fatal("index allowed to collide with runtime snapshot")
	}
}

func TestFailedDuplicateStartupPreservesOwnersRuntimeSnapshot(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	directory := t.TempDir()
	port := listener.Addr().(*net.TCPAddr).Port
	owner, err := NewMailServerWithOptions(port, "127.0.0.1", directory, ServerOptions{RuntimeReadiness: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = owner.Close() }()
	owner.smtpReady.Store(true)
	owner.probeMailStorage()
	if err := owner.publishRuntimeReadiness(); err != nil {
		t.Fatal(err)
	}
	path := RuntimeReadinessPath(directory, "127.0.0.1", port)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := NewMailServerWithOptions(port, "127.0.0.1", directory, ServerOptions{RuntimeReadiness: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = duplicate.Close() }()
	if err := duplicate.ListenWithReady(nil); err == nil {
		t.Fatal("duplicate unexpectedly acquired listener")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("failed duplicate modified live owner's snapshot")
	}
}

package api

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRelayJobsPersistTerminalStatus(t *testing.T) {
	mailDirectory := t.TempDir()
	store, err := newPersistentRelayJobStore(mailDirectory)
	if err != nil {
		t.Fatal(err)
	}
	store.newID = func() (string, error) { return "0123456789abcdef0123456789abcdef", nil }
	job, err := store.create("mail-1", "recipient@example.test")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(mailDirectory, ".owlmail-meta", "relay-jobs", job.ID+".json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("job mode = %o", info.Mode().Perm())
	}
	store.complete(job.ID, errors.New("connection refused"))

	reloaded, err := newPersistentRelayJobStore(mailDirectory)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := reloaded.get(job.ID)
	if !ok || got.Status != relayJobFailed || got.ErrorCategory != "connection" || got.CompletedAt == nil {
		t.Fatalf("reloaded job = %#v, found %t", got, ok)
	}
}

func TestRelayJobsReloadQueuedWork(t *testing.T) {
	mailDirectory := t.TempDir()
	store, err := newPersistentRelayJobStore(mailDirectory)
	if err != nil {
		t.Fatal(err)
	}
	store.newID = func() (string, error) { return "abcdef0123456789abcdef0123456789", nil }
	job, err := store.create("mail-2", "")
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := newPersistentRelayJobStore(mailDirectory)
	if err != nil {
		t.Fatal(err)
	}
	queued := reloaded.queued()
	if len(queued) != 1 || queued[0].ID != job.ID {
		t.Fatalf("queued jobs = %#v", queued)
	}
}

func TestRelayJobRetryStateIsDurable(t *testing.T) {
	mailDirectory := t.TempDir()
	store, err := newPersistentRelayJobStore(mailDirectory)
	if err != nil {
		t.Fatal(err)
	}
	store.newID = func() (string, error) { return "fedcba9876543210fedcba9876543210", nil }
	job, err := store.create("mail-retry", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.beginAttempt(job.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.queueRetry(job.ID, errors.New("connection refused"), time.Second); !ok {
		t.Fatal("retry was not queued")
	}
	reloaded, err := newPersistentRelayJobStore(mailDirectory)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := reloaded.get(job.ID)
	if !ok || got.Attempts != 1 || got.NextAttemptAt == nil || got.ErrorCategory != "connection" {
		t.Fatalf("reloaded retry = %#v, found %t", got, ok)
	}
}

func TestRelayRetryDelayHasBoundedJitter(t *testing.T) {
	for attempts, base := range map[int]time.Duration{1: 250 * time.Millisecond, 2: 500 * time.Millisecond, 3: time.Second} {
		delay := relayRetryDelay(attempts)
		if delay < base-base/5 || delay > base+base/5 {
			t.Fatalf("attempt %d delay %s outside jitter bound", attempts, delay)
		}
	}
}

func TestRelayJobsRejectCorruptState(t *testing.T) {
	directory := filepath.Join(t.TempDir(), ".owlmail-meta", "relay-jobs")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "bad.json"), []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := newPersistentRelayJobStore(filepath.Dir(filepath.Dir(directory))); err == nil {
		t.Fatal("corrupt persisted state was accepted")
	}
}

func TestNewAPIRecoversPersistedQueuedJob(t *testing.T) {
	first, server, _ := setupTestAPI(t)
	defer func() { _ = server.Close() }()
	job, err := first.relayJobs.create("missing-message", "recipient@example.test")
	if err != nil {
		t.Fatal(err)
	}

	restarted := NewAPI(server, 0, "localhost")
	got, ok := restarted.relayJobs.get(job.ID)
	if !ok || got.Status != relayJobFailed || got.CompletedAt == nil {
		t.Fatalf("recovered job = %#v, found %t", got, ok)
	}
}

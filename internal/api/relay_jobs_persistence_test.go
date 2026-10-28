package api

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/outgoing"
	"github.com/soulteary/owlmail/internal/relay"
	"github.com/soulteary/owlmail/internal/types"
)

func TestNewAPIRecoversPersistedQueuedJob(t *testing.T) {
	first, server, _ := setupTestAPI(t)
	defer func() { _ = server.Close() }()
	job := relay.Job{ID: "0123456789abcdef0123456789abcdef", EmailID: "missing-message", RelayTo: "recipient@example.test", Status: relay.Queued, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	installWorkflowRelay(t, first, relay.Options{}, job)

	restarted := first
	if err := server.SetOutgoingConfig(&outgoing.OutgoingConfig{Host: "127.0.0.1", Port: 1}); err != nil {
		t.Fatal(err)
	}
	restarted.relayService.Start()
	got, ok := waitForTerminalRelayJob(t, restarted.relayService, job.ID)
	if !ok || got.Status != relayJobFailed || got.CompletedAt == nil {
		t.Fatalf("recovered job = %#v, found %t", got, ok)
	}
}

func TestNewAPIStopsRecoveringExhaustedRelayJob(t *testing.T) {
	first, server, _ := setupTestAPI(t)
	defer func() { _ = server.Close() }()
	job := relay.Job{ID: "0123456789abcdef0123456789abcdef", EmailID: "missing-message", RelayTo: "recipient@example.test", Status: relay.Queued, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	installWorkflowRelay(t, first, relay.Options{}, job)
	job.Attempts = 3
	installWorkflowRelay(t, first, relay.Options{}, job)

	restarted := first
	if err := server.SetOutgoingConfig(&outgoing.OutgoingConfig{Host: "127.0.0.1", Port: 1}); err != nil {
		t.Fatal(err)
	}
	restarted.relayService.Start()
	got, ok := waitForTerminalRelayJob(t, restarted.relayService, job.ID)
	if !ok || got.Status != relayJobFailed || got.CompletedAt == nil || got.Attempts != 3 {
		t.Fatalf("recovered exhausted job = %#v, found %t", got, ok)
	}
}

func TestNewAPIProtectsPersistedRetrySourceBeforeBackgroundRecovery(t *testing.T) {
	first, server, mailDirectory := setupTestAPI(t)
	defer func() { _ = server.Close() }()
	email := &types.Email{ID: "persisted-retry-source", Subject: "protected retry"}
	if err := os.WriteFile(filepath.Join(mailDirectory, email.ID+".eml"), []byte("Subject: protected retry\r\n\r\nbody\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := server.SaveEmailToStore(email.ID, false, &types.Envelope{To: []string{"recipient@example.test"}}, email); err != nil {
		t.Fatal(err)
	}
	job := relay.Job{ID: "1123456789abcdef0123456789abcdef", EmailID: email.ID, Status: relay.Queued, CreatedAt: time.Now(), UpdatedAt: time.Now(), Attempts: 1}
	next := time.Now().Add(time.Hour)
	job.NextAttemptAt = &next
	installWorkflowRelay(t, first, relay.Options{}, job)

	restarted := first
	if err := server.SetOutgoingConfig(&outgoing.OutgoingConfig{Host: "127.0.0.1", Port: 1}); err != nil {
		t.Fatal(err)
	}
	restarted.relayService.Start()
	if err := server.ConfigureStoragePolicy(mailserver.StoragePolicy{MaxDiskBytes: 1, CleanupInterval: time.Hour}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(mailDirectory, email.ID+".eml")); err != nil {
		t.Fatalf("initial retention cleanup removed a queued relay source: %v", err)
	}
	if err := server.DeleteEmail(email.ID); !errors.Is(err, mailserver.ErrEmailSourceInUse) {
		t.Fatalf("DeleteEmail() error = %v, want ErrEmailSourceInUse", err)
	}
}

func TestDeferredAPILoadsAndProtectsQueuedWorkBeforeStartingRecovery(t *testing.T) {
	first, server, mailDirectory := setupTestAPI(t)
	defer func() { _ = server.Close() }()
	email := &types.Email{ID: "deferred-recovery-source", Subject: "deferred recovery"}
	if err := os.WriteFile(filepath.Join(mailDirectory, email.ID+".eml"), []byte("Subject: deferred recovery\r\n\r\nbody\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := server.SaveEmailToStore(email.ID, false, &types.Envelope{To: []string{"recipient@example.test"}}, email); err != nil {
		t.Fatal(err)
	}
	job := relay.Job{ID: "2123456789abcdef0123456789abcdef", EmailID: email.ID, Status: relay.Queued, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	installWorkflowRelay(t, first, relay.Options{}, job)

	restarted := first
	if err := server.DeleteEmail(email.ID); !errors.Is(err, mailserver.ErrEmailSourceInUse) {
		t.Fatalf("DeleteEmail() before recovery error = %v, want ErrEmailSourceInUse", err)
	}
	time.Sleep(20 * time.Millisecond)
	if queued, ok := restarted.relayService.Get(job.ID); !ok || queued.Attempts != 0 || queued.CompletedAt != nil {
		t.Fatalf("deferred job started before StartRelayRecovery: %#v, found %t", queued, ok)
	}

	if err := server.SetOutgoingConfig(&outgoing.OutgoingConfig{Host: "127.0.0.1", Port: 1}); err != nil {
		t.Fatal(err)
	}
	restarted.relayService.Start()
	// Wait for persistence to finish before TempDir cleanup; an attempt reservation
	// only proves the worker started and can still be followed by a spool write.
	completed, ok := waitForTerminalRelayJob(t, restarted.relayService, job.ID)
	if !ok || completed.Attempts == 0 {
		t.Fatalf("deferred recovery did not complete: %#v", completed)
	}

}

func waitForTerminalRelayJob(t *testing.T, store *relay.Service, id string) (relayJob, bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		job, ok := store.Get(id)
		if !ok || job.CompletedAt != nil {
			return job, ok
		}
		if time.Now().After(deadline) {
			t.Fatalf("relay job %s did not finish recovery", id)
		}
		time.Sleep(time.Millisecond)
	}
}

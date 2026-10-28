package relay

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRelayJobStoreTracksSafeTerminalStatus(t *testing.T) {
	store := newRelayJobStore()
	store.newID = func() (string, error) { return "job-1", nil }
	job, err := store.create("mail-1", "relay@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != relayJobQueued {
		t.Fatalf("new job status = %q", job.Status)
	}
	store.complete(job.ID, errors.New("dial tcp 127.0.0.1:25: connection refused; password=secret"))
	got, ok := store.get(job.ID)
	if !ok || got.Status != relayJobFailed || got.ErrorCategory != "connection" || got.CompletedAt == nil {
		t.Fatalf("completed job = %#v, %t", got, ok)
	}
	encoded, _ := json.Marshal(got)
	if strings.Contains(string(encoded), "password") || strings.Contains(string(encoded), "127.0.0.1") {
		t.Fatalf("relay status leaked raw error details: %s", encoded)
	}
}

func TestRelayJobStoreRejectsOversizedRecipient(t *testing.T) {
	store := newRelayJobStore()
	if _, err := store.create("mail-1", strings.Repeat("x", defaultRelayRecipientMaxBytes+1)); !errors.Is(err, errRelayRecipientTooLong) {
		t.Fatalf("oversized recipient error = %v, want %v", err, errRelayRecipientTooLong)
	}
	if len(store.jobs) != 0 || len(store.order) != 0 {
		t.Fatalf("oversized recipient was retained: jobs=%d order=%d", len(store.jobs), len(store.order))
	}
}

func TestRelayJobStoreRejectsDuplicatePendingEmail(t *testing.T) {
	store := newRelayJobStore()
	ids := []string{"first-job", "duplicate-job", "after-completion"}
	store.newID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}
	first, err := store.create("mail-1", "first@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.create("mail-1", "second@example.test"); !errors.Is(err, errRelayAlreadyPending) {
		t.Fatalf("duplicate create error = %v, want errRelayAlreadyPending", err)
	}
	store.complete(first.ID, nil)
	if _, err := store.create("mail-1", "second@example.test"); err != nil {
		t.Fatalf("create after completion: %v", err)
	}
}

func TestRelayJobStoreRejectsCapacityWithoutEvictingActiveJob(t *testing.T) {
	store := newRelayJobStore()
	store.limit = 1
	store.minimumRetention = 0
	ids := []string{"job-active", "job-rejected", "job-replacement"}
	store.newID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}

	active, err := store.create("mail-active", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.create("mail-rejected", ""); !errors.Is(err, errRelayJobCapacity) {
		t.Fatalf("create at active capacity error = %v, want %v", err, errRelayJobCapacity)
	}
	if got, ok := store.get(active.ID); !ok || got.Status != relayJobQueued {
		t.Fatalf("active job was evicted: %#v, %t", got, ok)
	}

	store.complete(active.ID, nil)
	replacement, err := store.create("mail-replacement", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.get(active.ID); ok {
		t.Fatal("completed job was not evicted to make room")
	}
	if got, ok := store.get(replacement.ID); !ok || got.EmailID != "mail-replacement" {
		t.Fatalf("replacement job = %#v, %t", got, ok)
	}
}

func TestRelayJobStoreKeepsFreshlyCompletedJobQueryable(t *testing.T) {
	currentTime := time.Unix(1_700_000_000, 0).UTC()
	store := newRelayJobStore()
	store.limit = 1
	store.minimumRetention = time.Minute
	store.now = func() time.Time { return currentTime }
	ids := []string{"job-accepted", "job-too-early", "job-after-window"}
	store.newID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}

	accepted, err := store.create("mail-accepted", "")
	if err != nil {
		t.Fatal(err)
	}
	store.complete(accepted.ID, nil)
	if _, err := store.create("mail-too-early", ""); !errors.Is(err, errRelayJobCapacity) {
		t.Fatalf("create during minimum retention error = %v, want %v", err, errRelayJobCapacity)
	}
	if got, ok := store.get(accepted.ID); !ok || got.Status != relayJobSucceeded {
		t.Fatalf("freshly completed job was not queryable: %#v, %t", got, ok)
	}

	currentTime = currentTime.Add(time.Minute)
	if _, err := store.create("mail-after-window", ""); err != nil {
		t.Fatalf("create after minimum retention: %v", err)
	}
	if _, ok := store.get(accepted.ID); ok {
		t.Fatal("completed job was retained after its protected window under capacity pressure")
	}
}

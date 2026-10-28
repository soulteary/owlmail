package relay

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestRelayJobsLoadKeepsOnlyOneQueuedJobPerEmail(t *testing.T) {
	mailDirectory := t.TempDir()
	store, err := newPersistentRelayJobStore(mailDirectory)
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Now().UTC().Add(-time.Minute)
	first := relayJob{
		ID: "00000000000000000000000000000001", EmailID: "duplicate-mail", Status: relayJobQueued,
		CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	second := relayJob{
		ID: "00000000000000000000000000000002", EmailID: "duplicate-mail", Status: relayJobQueued,
		CreatedAt: createdAt.Add(time.Second), UpdatedAt: createdAt.Add(time.Second),
	}
	for _, job := range []relayJob{first, second} {
		if err := store.persistLocked(job); err != nil {
			t.Fatal(err)
		}
	}

	reloaded, err := newPersistentRelayJobStore(mailDirectory)
	if err != nil {
		t.Fatal(err)
	}
	queued := reloaded.queued()
	if len(queued) != 1 || queued[0].ID != first.ID {
		t.Fatalf("queued jobs = %#v, want only oldest job", queued)
	}
	superseded, ok := reloaded.get(second.ID)
	if !ok || superseded.Status != relayJobFailed || superseded.CompletedAt == nil || superseded.ErrorCategory != "duplicate_pending" {
		t.Fatalf("superseded duplicate = %#v, found %t", superseded, ok)
	}

	reloadedAgain, err := newPersistentRelayJobStore(mailDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if queued := reloadedAgain.queued(); len(queued) != 1 || queued[0].ID != first.ID {
		t.Fatalf("durably deduplicated queued jobs = %#v", queued)
	}
}

func TestRelayJobsRejectConfirmedRecipientsThatCannotRemainLoadable(t *testing.T) {
	mailDirectory := t.TempDir()
	store, err := newPersistentRelayJobStore(mailDirectory)
	if err != nil {
		t.Fatal(err)
	}
	store.newID = func() (string, error) { return "fedcba9876543210fedcba9876543210", nil }
	recipients := make([]string, 1024)
	for index := range recipients {
		recipients[index] = fmt.Sprintf("recipient-%04d-%s@example.test", index, strings.Repeat("x", 64))
	}
	if _, err := store.createConfirmed("mail-too-large", "", recipients); !errors.Is(err, errRelayJobTooLarge) {
		t.Fatalf("createConfirmed() error = %v, want errRelayJobTooLarge", err)
	}
	entries, err := os.ReadDir(store.directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("oversized relay job left %d persisted entries", len(entries))
	}
}

func TestRelayJobsRemovePersistedRecordDurably(t *testing.T) {
	mailDirectory := t.TempDir()
	store, err := newPersistentRelayJobStore(mailDirectory)
	if err != nil {
		t.Fatal(err)
	}
	store.newID = func() (string, error) { return "00112233445566778899aabbccddeeff", nil }
	job, err := store.create("mail-remove", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.remove(job.ID); err != nil {
		t.Fatal(err)
	}
	reloaded, err := newPersistentRelayJobStore(mailDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reloaded.get(job.ID); ok {
		t.Fatal("removed relay job was recovered")
	}
}

func TestRelayJobCreateRollsBackAfterDirectorySyncFailure(t *testing.T) {
	mailDirectory := t.TempDir()
	store, err := newPersistentRelayJobStore(mailDirectory)
	if err != nil {
		t.Fatal(err)
	}
	store.newID = func() (string, error) { return "11223344556677889900aabbccddeeff", nil }
	realSync := store.syncDirectory
	syncCalls := 0
	store.syncDirectory = func(path string) error {
		syncCalls++
		if syncCalls == 1 {
			return errors.New("injected sync failure")
		}
		return realSync(path)
	}

	_, err = store.create("mail-sync-failure", "")
	if !errors.Is(err, errRelayJobPersistence) {
		t.Fatalf("create error = %v, want persistence error", err)
	}
	if len(store.jobs) != 0 || len(store.order) != 0 {
		t.Fatal("failed create remained in memory")
	}
	path := filepath.Join(store.directory, "11223344556677889900aabbccddeeff.json")
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("failed create remained on disk: %v", statErr)
	}
}

func TestRelayJobCreateRetainsQueryableStateWhenCleanupIsIndeterminate(t *testing.T) {
	mailDirectory := t.TempDir()
	store, err := newPersistentRelayJobStore(mailDirectory)
	if err != nil {
		t.Fatal(err)
	}
	store.newID = func() (string, error) { return "22334455667788990011aabbccddeeff", nil }
	store.syncDirectory = func(string) error { return errors.New("injected sync failure") }

	job, err := store.create("mail-retained", "")
	if !errors.Is(err, errRelayJobRetained) {
		t.Fatalf("create error = %v, want retained error", err)
	}
	if got, ok := store.get(job.ID); !ok || got.EmailID != "mail-retained" {
		t.Fatalf("retained job = %#v, found %t", got, ok)
	}
}

func TestRelayJobsEnforceAttemptLimitAtPersistenceBoundary(t *testing.T) {
	store := newRelayJobStore()
	store.newID = func() (string, error) { return "ffeeddccbbaa99887766554433221100", nil }
	job, err := store.create("mail-attempts", "")
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < defaultRelayMaxAttempts; attempt++ {
		if _, err := store.beginAttempt(job.ID); err != nil {
			t.Fatalf("attempt %d: %v", attempt+1, err)
		}
	}
	if _, err := store.beginAttempt(job.ID); !errors.Is(err, errRelayAttemptsExhausted) {
		t.Fatalf("fourth attempt error = %v, want errRelayAttemptsExhausted", err)
	}
	got, _ := store.get(job.ID)
	if got.Attempts != defaultRelayMaxAttempts {
		t.Fatalf("attempts = %d, want %d", got.Attempts, defaultRelayMaxAttempts)
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

func TestUnrelatedRelayWriteDoesNotClearPersistenceFailure(t *testing.T) {
	store, err := newPersistentRelayJobStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	original := store.directory
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	store.directory = blocker
	first := relayJob{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", EmailID: "first", Status: relayJobSucceeded}
	if err := store.persistLocked(first); err == nil {
		t.Fatal("expected failed write")
	}
	store.directory = original
	second := relayJob{ID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", EmailID: "second", Status: relayJobSucceeded}
	if err := store.persistLocked(second); err != nil {
		t.Fatal(err)
	}
	if store.lastPersistenceErr == nil {
		t.Fatal("unrelated write concealed unresolved failure")
	}
	if err := store.persistLocked(first); err != nil {
		t.Fatal(err)
	}
	if store.lastPersistenceErr != nil {
		t.Fatal(store.lastPersistenceErr)
	}
}

package mailbox

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "control.sqlite"), true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
func TestPendingConcurrentThenClaim(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	ids := make(chan string, 20)
	for range 20 {
		wg.Go(func() {
			r, err := s.EnsurePending(ctx, "main", "Alice@EXAMPLE.com", 10)
			if err != nil {
				t.Error(err)
				return
			}
			ids <- r.MailboxID
		})
	}
	wg.Wait()
	close(ids)
	first := ""
	for id := range ids {
		if first == "" {
			first = id
		}
		if id != first {
			t.Fatal("duplicate pending mailbox")
		}
	}
	result, err := s.BindLogin(ctx, Login{"main", "alice", "Alice@example.com", true}, VerifiedEmail)
	if err != nil {
		t.Fatal(err)
	}
	if result.MailboxID != first {
		t.Fatal("history was not adopted")
	}
	for range 3 {
		next, err := s.BindLogin(ctx, Login{"main", "alice", "Alice@example.com", true}, VerifiedEmail)
		if err != nil || next != result {
			t.Fatalf("not idempotent: %+v %v", next, err)
		}
	}
	other, err := s.BindLogin(ctx, Login{"main", "bob", "Alice@example.com", true}, VerifiedEmail)
	if err != nil {
		t.Fatal(err)
	}
	if !other.Conflict || other.MailboxID == first {
		t.Fatal("conflicting identity acquired mailbox")
	}
}
func TestNoEmailEvidenceAndExplicitHistory(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	pending, err := s.EnsurePending(ctx, "main", "a@example.com", 10)
	if err != nil {
		t.Fatal(err)
	}
	own, err := s.BindLogin(ctx, Login{"main", "a", "a@example.com", false}, VerifiedEmail)
	if err != nil {
		t.Fatal(err)
	}
	if own.MailboxID == pending.MailboxID {
		t.Fatal("unverified history claim")
	}
	explicit, err := s.BindLogin(ctx, Login{"main", "a", "a@example.com", true}, ExplicitHistory)
	if err != nil {
		t.Fatal(err)
	}
	if explicit.MailboxID != own.MailboxID || explicit.MergeID != "" {
		t.Fatal("explicit history auto merged")
	}
	err = s.WithRouteSnapshot(ctx, "main", []string{"a@example.com"}, func(routes []Route) error {
		if routes[0].MailboxID != own.MailboxID {
			t.Error("future mail not rebound")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var state string
	if err = s.db.QueryRow(`SELECT state FROM mailbox WHERE id=?`, pending.MailboxID).Scan(&state); err != nil || state != "pending" {
		t.Fatal("history not retained")
	}
}
func TestExistingPersonalCreatesDurableMerge(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	own, err := s.BindLogin(ctx, Login{"main", "a", "", false}, VerifiedEmail)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := s.EnsurePending(ctx, "main", "a@example.com", 10)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.BindLogin(ctx, Login{"main", "a", "a@example.com", true}, VerifiedEmail)
	if err != nil {
		t.Fatal(err)
	}
	if result.MailboxID != own.MailboxID || result.MergeID == "" {
		t.Fatal("missing merge")
	}
	var source, target, state string
	if err = s.db.QueryRow(`SELECT source_id,target_id,state FROM merge_journal WHERE id=?`, result.MergeID).Scan(&source, &target, &state); err != nil {
		t.Fatal(err)
	}
	if source != pending.MailboxID || target != own.MailboxID || state != "pending" {
		t.Fatal("wrong merge journal")
	}
	repeated, err := s.BindLogin(ctx, Login{"main", "a", "a@example.com", true}, VerifiedEmail)
	if err != nil || repeated.MergeID != "" {
		t.Fatal("duplicate merge")
	}
}
func TestPendingLimitsAndNamespaces(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	first, err := s.EnsurePending(ctx, "main", "a@example.com", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.EnsurePending(ctx, "main", "b@example.com", 1); !errors.Is(err, ErrLimit) {
		t.Fatalf("limit ignored: %v", err)
	}
	again, err := s.EnsurePending(ctx, "main", "a@example.com", 1)
	if err != nil || first.ID != again.ID {
		t.Fatal("existing route blocked by creation limit")
	}
	other, err := s.EnsurePending(ctx, "other", "a@example.com", 2)
	if err != nil || first.MailboxID == other.MailboxID {
		t.Fatal("namespace mixed")
	}
}
func TestControlDatabaseMustNotBeReinitialized(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sqlite")
	if _, err := Open(path, false); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path, true); err == nil {
		t.Fatal("reinitialized existing empty database")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	var count int
	if err = s.db.QueryRow(`SELECT count(*) FROM identity`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	var missing string
	if err = s.db.QueryRow(`SELECT id FROM identity`).Scan(&missing); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
}
func TestNormalizeAddress(t *testing.T) {
	for _, v := range []string{"", "Alice <a@example.com>", " a@example.com", "a@example.com\n", "a@localhost.", "a@-bad.test", "a@x..test", "\"a\"@example.com"} {
		if _, err := NormalizeAddress(v); err == nil {
			t.Errorf("accepted %q", v)
		}
	}
	for input, want := range map[string]string{"Alice+run@EXAMPLE.com": "Alice+run@example.com", "alice@example.com": "alice@example.com"} {
		got, err := NormalizeAddress(input)
		if err != nil || got != want {
			t.Errorf("%q: %q %v", input, got, err)
		}
	}
}

func TestSnapshotSerializesWithLogin(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	original, err := s.EnsurePending(ctx, "main", "a@example.com", 10)
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	snapshotDone := make(chan error, 1)
	go func() {
		snapshotDone <- s.WithRouteSnapshot(ctx, "main", []string{"a@example.com"}, func(routes []Route) error {
			if routes[0].MailboxID != original.MailboxID {
				t.Error("wrong original route")
			}
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	loginStarted := make(chan struct{})
	loginDone := make(chan error, 1)
	go func() {
		close(loginStarted)
		_, err := s.BindLogin(ctx, Login{"main", "a", "a@example.com", true}, VerifiedEmail)
		loginDone <- err
	}()
	<-loginStarted
	// Do not depend on scheduler timing: the held snapshot owns s.mu. A
	// successful try-lock here would prove the serialization boundary is absent.
	if s.mu.TryLock() {
		s.mu.Unlock()
		t.Fatal("route snapshot did not retain mutation lock")
	}
	close(release)
	if err = <-snapshotDone; err != nil {
		t.Fatal(err)
	}
	if err = <-loginDone; err != nil {
		t.Fatal(err)
	}
}

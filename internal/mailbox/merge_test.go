package mailbox

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func plannedMerge(t *testing.T, s *Store) (ClaimResult, Route) {
	t.Helper()
	ctx := context.Background()
	owner, err := s.BindLogin(ctx, Login{"main", "alice", "", false}, VerifiedEmail)
	if err != nil {
		t.Fatal(err)
	}
	source, err := s.EnsurePending(ctx, "main", "alice@example.com", 10)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.BindLogin(ctx, Login{"main", "alice", "alice@example.com", true}, VerifiedEmail)
	if err != nil {
		t.Fatal(err)
	}
	if result.MailboxID != owner.MailboxID || result.MergeID == "" {
		t.Fatal("merge was not planned")
	}
	return result, source
}
func TestMergeRecoveryKeepsFrozenUntilDurableSuccess(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "control.sqlite")
	s, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	result, source := plannedMerge(t, s)
	called := 0
	failure := errors.New("simulated interruption after file commit")
	err = s.RunPendingMerges(ctx, func(_ context.Context, item Merge) error {
		called++
		if item.SourceID != source.MailboxID || item.TargetID != result.MailboxID {
			t.Fatal("wrong transfer")
		}
		return failure
	})
	if !errors.Is(err, failure) || called != 1 {
		t.Fatal(err, called)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	pending, err := s.PendingMerges(ctx)
	if err != nil || len(pending) != 1 {
		t.Fatal(pending, err)
	}
	if err = s.WithPersonalMailbox(ctx, "main", "alice", func(id string) error {
		if id != result.MailboxID {
			t.Fatal("source was exposed")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = s.RunPendingMerges(ctx, func(_ context.Context, item Merge) error { called++; return nil }); err != nil {
		t.Fatal(err)
	}
	pending, err = s.PendingMerges(ctx)
	if err != nil || len(pending) != 0 {
		t.Fatal(pending, err)
	}
	var state, target string
	if err = s.db.QueryRow(`SELECT state,merged_into_id FROM mailbox WHERE id=?`, source.MailboxID).Scan(&state, &target); err != nil || state != "merged" || target != result.MailboxID {
		t.Fatal(state, target, err)
	}
	if err = s.RunPendingMerges(ctx, func(context.Context, Merge) error { t.Fatal("complete merge replayed"); return nil }); err != nil {
		t.Fatal(err)
	}
	if called != 2 {
		t.Fatal(called)
	}
	if err = validateControlState(ctx, s.db); err != nil {
		t.Fatal(err)
	}
}
func TestMergeRejectsInvalidControlGraph(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	_, source := plannedMerge(t, s)
	// Repointing a future route back to the source invalidates its frozen status.
	if _, err := s.db.Exec(`UPDATE recipient_route SET mailbox_id=?`, source.MailboxID); err != nil {
		t.Fatal(err)
	}
	called := false
	if err := s.RunPendingMerges(ctx, func(context.Context, Merge) error { called = true; return nil }); err == nil || called {
		t.Fatal("invalid graph reached file mutation")
	}
	if err := validateControlState(ctx, s.db); err == nil {
		t.Fatal("invalid graph accepted")
	}
}
func TestPersonalMailboxDoesNotCreateOrExposePending(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if _, err := s.PersonalMailbox(ctx, "main", "missing"); !errors.Is(err, ErrMailboxUnavailable) {
		t.Fatal(err)
	}
	called := false
	if err := s.WithPersonalMailbox(ctx, "main", "missing", func(string) error { called = true; return nil }); !errors.Is(err, ErrMailboxUnavailable) || called {
		t.Fatal("missing user callback ran")
	}
	personal, err := s.BindLogin(ctx, Login{"main", "alice", "", false}, VerifiedEmail)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.WithPersonalMailbox(ctx, "main", "alice", func(id string) error {
		if id != personal.MailboxID {
			t.Fatal(id)
		}
		if s.mu.TryLock() {
			s.mu.Unlock()
			t.Fatal("operation not serialized")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.PersonalMailbox(ctx, "other", "alice"); !errors.Is(err, ErrMailboxUnavailable) {
		t.Fatal("namespace bypass")
	}
}

func TestFrozenAndHistoricalSourcesDoNotConsumeActivePendingQuota(t *testing.T) {
	for _, policy := range []ClaimPolicy{VerifiedEmail, ExplicitHistory} {
		t.Run(string(policy), func(t *testing.T) {
			s := testStore(t)
			ctx := context.Background()
			if _, err := s.BindLogin(ctx, Login{"main", "alice", "", false}, policy); err != nil {
				t.Fatal(err)
			}
			if _, err := s.EnsurePending(ctx, "main", "alice@example.com", 1); err != nil {
				t.Fatal(err)
			}
			if _, err := s.BindLogin(ctx, Login{"main", "alice", "alice@example.com", true}, policy); err != nil {
				t.Fatal(err)
			}
			if _, err := s.EnsurePending(ctx, "main", "next@example.com", 1); err != nil {
				t.Fatal("unroutable history consumed pending creation quota", err)
			}
		})
	}
}

func TestQuotaDeferredMergeDoesNotBlockPersonalMailbox(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	result, _ := plannedMerge(t, s)
	if err := s.RunPendingMerges(ctx, func(context.Context, Merge) error { return ErrQuotaExceeded }); err != nil {
		t.Fatal("quota blocked authority", err)
	}
	if id, err := s.PersonalMailbox(ctx, "main", "alice"); err != nil || id != result.MailboxID {
		t.Fatal("user cannot make room in personal mailbox", err)
	}
	pending, err := s.PendingMerges(ctx)
	if err != nil || len(pending) != 1 {
		t.Fatal("deferred source was unfrozen", err)
	}
	if err = s.RunPendingMerges(ctx, func(context.Context, Merge) error { return nil }); err != nil {
		t.Fatal(err)
	}
}

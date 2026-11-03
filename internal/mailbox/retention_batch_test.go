package mailbox

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestCleanupPendingBatchReleasesAuthorityBetweenMailboxes(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if _, err := s.BindLogin(ctx, Login{Namespace: "main", Subject: "bob"}, VerifiedEmail); err != nil {
		t.Fatal(err)
	}
	var boxes []string
	for _, address := range []string{"first@example.com", "second@example.com"} {
		route, err := s.EnsurePending(ctx, "main", address, 10)
		if err != nil {
			t.Fatal(err)
		}
		agePending(t, s, route.MailboxID, now.Add(-30*24*time.Hour))
		boxes = append(boxes, route.MailboxID)
	}
	sort.Strings(boxes)
	calls := 0
	apply := func(_ context.Context, item PendingRetention) (PendingRetentionResult, bool, error) {
		if item.MailboxID != boxes[calls] {
			t.Fatalf("unexpected mailbox %s", item.MailboxID)
		}
		calls++
		if s.mu.TryLock() {
			s.mu.Unlock()
			t.Fatal("callback not serialized")
		}
		return PendingRetentionResult{}, true, nil
	}
	cursor, err := s.CleanupPendingBatch(ctx, now, 7*24*time.Hour, 24*time.Hour, "", apply)
	if err != nil || calls != 1 {
		t.Fatal(calls, err)
	}
	// A foreground authority lookup completes between batches without waiting for
	// the second mailbox. The old whole-sweep API could not provide this boundary.
	if _, err = s.PersonalMailbox(ctx, "main", "bob"); err != nil {
		t.Fatal(err)
	}
	var planned int
	if err = s.db.QueryRow(`SELECT count(*) FROM retention_journal`).Scan(&planned); err != nil || planned != 1 {
		t.Fatal("unvisited mailbox was frozen", planned, err)
	}
	cursor, err = s.CleanupPendingBatch(ctx, now, 7*24*time.Hour, 24*time.Hour, cursor, apply)
	if err != nil || calls != 2 {
		t.Fatal(calls, err)
	}
	cursor, err = s.CleanupPendingBatch(ctx, now, 7*24*time.Hour, 24*time.Hour, cursor, apply)
	if err != nil || cursor != "" || calls != 2 {
		t.Fatal("sweep did not finish", cursor, calls, err)
	}
}

func TestCleanupPendingBatchPartialRecoveryAndFairness(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sqlite")
	s, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC()
	routes := map[string]Route{}
	var ids []string
	for _, address := range []string{"first@example.com", "second@example.com"} {
		route, e := s.EnsurePending(ctx, "main", address, 10)
		if e != nil {
			t.Fatal(e)
		}
		agePending(t, s, route.MailboxID, now.Add(-30*24*time.Hour))
		routes[route.MailboxID] = route
		ids = append(ids, route.MailboxID)
	}
	sort.Strings(ids)
	var original PendingRetention
	failure := errors.New("one mailbox storage unavailable")
	cursor, err := s.CleanupPendingBatch(ctx, now, 7*24*time.Hour, 24*time.Hour, "", func(_ context.Context, item PendingRetention) (PendingRetentionResult, bool, error) {
		original = item
		return PendingRetentionResult{Deleted: 1, Remaining: 1}, false, failure
	})
	if cursor != ids[0] || !errors.Is(err, failure) {
		t.Fatal(cursor, err)
	}
	if _, err = s.EnsurePending(ctx, "main", routes[ids[0]].Address, 10); !errors.Is(err, ErrRetentionPending) {
		t.Fatal("partial box accepted delivery", err)
	}
	if _, err = s.BindLogin(ctx, Login{"main", "alice", routes[ids[0]].Address, true}, VerifiedEmail); !errors.Is(err, ErrRetentionPending) {
		t.Fatal("partial box accepted claim", err)
	}
	cursor, err = s.CleanupPendingBatch(ctx, now, 7*24*time.Hour, 24*time.Hour, cursor, func(_ context.Context, item PendingRetention) (PendingRetentionResult, bool, error) {
		if item.MailboxID != ids[1] {
			t.Fatal("failure starved next mailbox")
		}
		return PendingRetentionResult{}, true, nil
	})
	if err != nil || cursor != ids[1] {
		t.Fatal(cursor, err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	cursor, err = s.CleanupPendingBatch(ctx, now.Add(time.Hour), 7*24*time.Hour, 24*time.Hour, "", func(_ context.Context, item PendingRetention) (PendingRetentionResult, bool, error) {
		if item != original {
			t.Fatal("recovery changed journal/cutoff", item, original)
		}
		return PendingRetentionResult{Deleted: 1, Remaining: 1}, false, nil
	})
	if err != nil || cursor != ids[0] {
		t.Fatal(cursor, err)
	}
	var audits int
	if err = s.db.QueryRow(`SELECT count(*) FROM retention_audit WHERE mailbox_id=?`, ids[0]).Scan(&audits); err != nil || audits != 0 {
		t.Fatal("partial callback finalized audit", audits, err)
	}
	if err = s.RunPendingRetention(ctx, func(_ context.Context, item PendingRetention) (PendingRetentionResult, error) {
		if item != original {
			t.Fatal("offline recovery changed original journal")
		}
		return PendingRetentionResult{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = validateControlState(ctx, s.db); err != nil {
		t.Fatal(err)
	}
	if err = s.db.QueryRow(`SELECT count(*) FROM retention_audit WHERE mailbox_id=?`, ids[0]).Scan(&audits); err != nil || audits != 1 {
		t.Fatal("completion missing audit", audits, err)
	}
}

func TestCleanupPendingBatchSkipsPersonalAndMergeAndRejectsInvalidGraph(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	personal, source := plannedMerge(t, s)
	agePending(t, s, personal.MailboxID, now.Add(-30*24*time.Hour))
	agePending(t, s, source.MailboxID, now.Add(-30*24*time.Hour))
	apply := func(context.Context, PendingRetention) (PendingRetentionResult, bool, error) {
		t.Fatal("invalid candidate reached callback")
		return PendingRetentionResult{}, true, nil
	}
	if cursor, err := s.CleanupPendingBatch(ctx, now, 7*24*time.Hour, 24*time.Hour, "", apply); err != nil || cursor != "" {
		t.Fatal(cursor, err)
	}
	// A corrupt pending journal must be reported, not hidden by candidate filters.
	if _, err := s.db.Exec(`INSERT INTO retention_journal(id,mailbox_id,state,cutoff,empty_before,created_at) VALUES('bad',?,'pending',1,1,1)`, personal.MailboxID); err != nil {
		t.Fatal(err)
	}
	if cursor, err := s.CleanupPendingBatch(ctx, now, 7*24*time.Hour, 24*time.Hour, "", apply); err == nil || cursor != personal.MailboxID {
		t.Fatal(cursor, err)
	}
}

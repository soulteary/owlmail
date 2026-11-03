package mailbox

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func agePending(t *testing.T, s *Store, id string, created time.Time) {
	t.Helper()
	if _, err := s.db.Exec(`UPDATE mailbox SET created_at=? WHERE id=?`, created.Unix(), id); err != nil {
		t.Fatal(err)
	}
}
func TestPendingRetentionRetiresEmptyAndAllowsNewDelivery(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	route, err := s.EnsurePending(ctx, "main", "alice@example.com", 10)
	if err != nil {
		t.Fatal(err)
	}
	agePending(t, s, route.MailboxID, now.Add(-48*time.Hour))
	calls := 0
	if err = s.CleanupPending(ctx, now, 7*24*time.Hour, 24*time.Hour, func(_ context.Context, item PendingRetention) (PendingRetentionResult, error) {
		calls++
		if item.MailboxID != route.MailboxID || !item.Cutoff.Equal(now.Add(-7*24*time.Hour)) {
			t.Fatal("wrong retention decision")
		}
		return PendingRetentionResult{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	next, err := s.EnsurePending(ctx, "main", "alice@example.com", 10)
	if err != nil || next.MailboxID == route.MailboxID {
		t.Fatal("expired route was not recreated", err)
	}
	var audits int
	if err = s.db.QueryRow(`SELECT count(*) FROM retention_audit WHERE mailbox_id=? AND retired=1`, route.MailboxID).Scan(&audits); err != nil || audits != 1 {
		t.Fatal("missing retention audit", err)
	}
	if err = s.CleanupPending(ctx, now.Add(time.Hour), 7*24*time.Hour, 24*time.Hour, func(context.Context, PendingRetention) (PendingRetentionResult, error) {
		calls++
		return PendingRetentionResult{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("retired tombstone swept again")
	}
}
func TestPendingRetentionFailureFreezesOnlyItsSourceAndRecovers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sqlite")
	s, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC()
	route, err := s.EnsurePending(ctx, "main", "alice@example.com", 10)
	if err != nil {
		t.Fatal(err)
	}
	agePending(t, s, route.MailboxID, now.Add(-8*24*time.Hour))
	failed := errors.New("disk failure")
	if err = s.CleanupPending(ctx, now, 7*24*time.Hour, 24*time.Hour, func(context.Context, PendingRetention) (PendingRetentionResult, error) {
		return PendingRetentionResult{}, failed
	}); !errors.Is(err, failed) {
		t.Fatal(err)
	}
	if _, err = s.BindLogin(ctx, Login{"main", "alice", "alice@example.com", true}, VerifiedEmail); !errors.Is(err, ErrRetentionPending) {
		t.Fatal("partially cleaned source could be claimed", err)
	}
	if _, err = s.EnsurePending(ctx, "main", "alice@example.com", 10); !errors.Is(err, ErrRetentionPending) {
		t.Fatal("partially cleaned source could receive", err)
	}
	if _, err = s.BindLogin(ctx, Login{"main", "bob", "bob@example.com", true}, VerifiedEmail); err != nil {
		t.Fatal("unrelated identity blocked", err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err = s.RunPendingRetention(ctx, func(context.Context, PendingRetention) (PendingRetentionResult, error) {
		return PendingRetentionResult{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	claimed, err := s.BindLogin(ctx, Login{"main", "alice", "alice@example.com", true}, VerifiedEmail)
	if err != nil || claimed.MailboxID == route.MailboxID {
		t.Fatal("cleaned history rebound", err)
	}
}
func TestPendingRetentionNeverTargetsPersonalOrFrozenMerge(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	personal, source := plannedMerge(t, s)
	agePending(t, s, source.MailboxID, now.Add(-30*24*time.Hour))
	agePending(t, s, personal.MailboxID, now.Add(-30*24*time.Hour))
	if err := s.CleanupPending(ctx, now, 7*24*time.Hour, 24*time.Hour, func(context.Context, PendingRetention) (PendingRetentionResult, error) {
		t.Fatal("personal or merge source reached retention callback")
		return PendingRetentionResult{}, nil
	}); err != nil {
		t.Fatal(err)
	}
}
func TestExplicitHistoryOrphanExpiresWithoutMovingCurrentRoute(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	own, err := s.BindLogin(ctx, Login{"main", "alice", "", false}, ExplicitHistory)
	if err != nil {
		t.Fatal(err)
	}
	source, err := s.EnsurePending(ctx, "main", "alice@example.com", 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.BindLogin(ctx, Login{"main", "alice", "alice@example.com", true}, ExplicitHistory); err != nil {
		t.Fatal(err)
	}
	agePending(t, s, source.MailboxID, now.Add(-30*24*time.Hour))
	if err = s.CleanupPending(ctx, now, 7*24*time.Hour, 24*time.Hour, func(_ context.Context, item PendingRetention) (PendingRetentionResult, error) {
		if item.MailboxID != source.MailboxID {
			t.Fatal("wrong source")
		}
		return PendingRetentionResult{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	next, err := s.EnsurePending(ctx, "main", "alice@example.com", 10)
	if err != nil || next.MailboxID != own.MailboxID {
		t.Fatal("personal route altered", err)
	}
	if err = validateControlState(ctx, s.db); err != nil {
		t.Fatal(err)
	}
}
func TestCleanupSerializesWithClaim(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	source, err := s.EnsurePending(ctx, "main", "alice@example.com", 10)
	if err != nil {
		t.Fatal(err)
	}
	agePending(t, s, source.MailboxID, now.Add(-30*24*time.Hour))
	entered, release := make(chan struct{}), make(chan struct{})
	cleanupDone := make(chan error, 1)
	go func() {
		cleanupDone <- s.CleanupPending(ctx, now, 7*24*time.Hour, 24*time.Hour, func(context.Context, PendingRetention) (PendingRetentionResult, error) {
			close(entered)
			<-release
			return PendingRetentionResult{}, nil
		})
	}()
	<-entered
	if s.mu.TryLock() {
		s.mu.Unlock()
		t.Fatal("cleanup did not retain mutation lock")
	}
	var wg sync.WaitGroup
	var claimed ClaimResult
	var claimErr error
	wg.Go(func() {
		claimed, claimErr = s.BindLogin(ctx, Login{"main", "alice", "alice@example.com", true}, VerifiedEmail)
	})
	close(release)
	if err = <-cleanupDone; err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if claimErr != nil || claimed.MailboxID == source.MailboxID {
		t.Fatal("claim acquired a deleted mailbox", claimErr)
	}
}
func TestPendingRetentionPreservesBlockedDenyRoutes(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	route, err := s.EnsurePending(ctx, "main", "blocked@example.com", 10)
	if err != nil {
		t.Fatal(err)
	}
	agePending(t, s, route.MailboxID, now.Add(-48*time.Hour))
	if _, err = s.db.Exec(`UPDATE recipient_route SET state='blocked' WHERE id=?`, route.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.CleanupPending(ctx, now, 7*24*time.Hour, 24*time.Hour, func(context.Context, PendingRetention) (PendingRetentionResult, error) {
		return PendingRetentionResult{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.EnsurePending(ctx, "main", "blocked@example.com", 10); !errors.Is(err, ErrBlocked) {
		t.Fatal("retention unblocked denied recipient", err)
	}
	if err = validateControlState(ctx, s.db); err != nil {
		t.Fatal(err)
	}
}
func TestSchemaV1UpgradeAndReadOnlyAuthority(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sqlite")
	// Build exactly the old schema, not a v2 database with a forged user_version.
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	legacyIdentity, legacyBox := uuid.NewString(), uuid.NewString()
	if _, err = db.Exec(`INSERT INTO mailbox(id,state,owner_identity_id,created_at,claimed_at) VALUES(?,'personal',?,1,1)`, legacyBox, legacyIdentity); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO identity(id,namespace,subject,personal_mailbox_id,created_at,last_seen_at) VALUES(?,'main','alice',?,1,1)`, legacyIdentity, legacyBox); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	legacyReadOnly, err := OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = legacyReadOnly.CheckPersonalMailboxID(context.Background(), legacyBox); err != nil {
		t.Fatal("v1 personal export check failed", err)
	}
	if err = legacyReadOnly.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("read-only v1 open mutated database")
	}

	upgraded, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := upgraded.BindLogin(context.Background(), Login{"main", "alice", "", false}, VerifiedEmail)
	if err != nil {
		t.Fatal(err)
	}
	if err = upgraded.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(path, 0440); err != nil {
		t.Fatal(err)
	}
	readonly, err := OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = readonly.Close() }()
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0440 {
		t.Fatal("readonly open changed permissions", err)
	}
	if err = readonly.CheckPersonalMailboxID(context.Background(), owner.MailboxID); err != nil {
		t.Fatal(err)
	}
	if err = readonly.CheckPersonalMailboxID(context.Background(), uuid.NewString()); !errors.Is(err, ErrMailboxUnavailable) {
		t.Fatal(err)
	}
	if _, err = readonly.BindLogin(context.Background(), Login{"main", "bob", "", false}, VerifiedEmail); err == nil {
		t.Fatal("readonly control accepted mutation")
	}
	if err = readonly.RunPendingMerges(context.Background(), func(context.Context, Merge) error { t.Fatal("readonly merge callback ran"); return nil }); !errors.Is(err, ErrReadOnly) {
		t.Fatal(err)
	}
	if err = readonly.RunPendingRetention(context.Background(), func(context.Context, PendingRetention) (PendingRetentionResult, error) {
		t.Fatal("readonly retention callback ran")
		return PendingRetentionResult{}, nil
	}); !errors.Is(err, ErrReadOnly) {
		t.Fatal(err)
	}
	if _, err = readonly.CleanupPendingBatch(context.Background(), time.Now(), time.Hour, time.Hour, "", func(context.Context, PendingRetention) (PendingRetentionResult, bool, error) {
		t.Fatal("readonly retention batch callback ran")
		return PendingRetentionResult{}, true, nil
	}); !errors.Is(err, ErrReadOnly) {
		t.Fatal(err)
	}

}

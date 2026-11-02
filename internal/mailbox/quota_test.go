package mailbox

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAttemptQuotaSurvivesRestartAndClockRollback(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "control.sqlite")
	s, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Unix(1800000000, 0)
	for range 2 {
		if err = s.ConsumeAttempt(ctx, "rcpt", "main", "gorge", 2, at); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.ConsumeAttempt(ctx, "rcpt", "main", "gorge", 2, at); !errors.Is(err, ErrAttemptQuota) {
		t.Fatalf("quota ignored: %v", err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	for _, now := range []time.Time{at, at.Add(-time.Hour)} {
		if err = s.ConsumeAttempt(ctx, "rcpt", "main", "gorge", 2, now); !errors.Is(err, ErrAttemptQuota) {
			t.Fatalf("restart/clock rollback reset budget: %v", err)
		}
	}
	if err = s.ConsumeAttempt(ctx, "rcpt", "main", "gorge", 2, at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err = s.ConsumeAttempt(ctx, "rcpt", "main", "other", 2, at.Add(time.Minute)); err != nil {
		t.Fatal("account budgets were shared", err)
	}
	if err = s.ConsumeAttempt(ctx, "auth", "", "", 1, at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err = s.ConsumeAttempt(ctx, "auth", "", "", 1, at); !errors.Is(err, ErrAttemptQuota) {
		t.Fatal("global auth budget reset", err)
	}
	if err = s.ConsumeAttempt(ctx, "auth", "main", "attacker-controlled-user", 1, at); err == nil {
		t.Fatal("arbitrary auth keys accepted")
	}
}
func TestAttemptQuotaConcurrentAdmission(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	at := time.Unix(1800000000, 0)
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			err := s.ConsumeAttempt(ctx, "rcpt", "main", "gorge", 5, at)
			if err == nil {
				accepted.Add(1)
			} else if !errors.Is(err, ErrAttemptQuota) {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if accepted.Load() != 5 {
		t.Fatalf("accepted %d, want 5", accepted.Load())
	}
	var stored int
	if err := s.db.QueryRow(`SELECT attempts FROM smtp_attempt_quota WHERE kind='rcpt' AND namespace='main' AND account_id='gorge'`).Scan(&stored); err != nil || stored != 5 {
		t.Fatalf("stored attempts %d: %v", stored, err)
	}
}
func TestAttemptQuotaPrunesOnlyInactiveWindows(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	at := time.Unix(1800000000, 0)
	if err := s.ConsumeAttempt(ctx, "auth", "", "", 1, at); err != nil {
		t.Fatal(err)
	}
	if err := s.ConsumeAttempt(ctx, "auth", "", "", 1, at.Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	var rows int
	if err := s.db.QueryRow(`SELECT count(*) FROM smtp_attempt_quota`).Scan(&rows); err != nil || rows != 1 {
		t.Fatal(rows, err)
	}
	if err := s.ConsumeAttempt(ctx, "auth", "", "", 1, at); !errors.Is(err, ErrAttemptQuota) {
		t.Fatal("pruning reopened an old budget", err)
	}
}

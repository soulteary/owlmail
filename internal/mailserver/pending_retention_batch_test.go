package mailserver

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/mailbox"
)

func TestRetainPendingBatchBoundsDeletionAndReleasesStorage(t *testing.T) {
	server := isolatedServer(t, t.TempDir())
	ctx := context.Background()
	box := sampleOwnership().MailboxID
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		retainedMessage(t, server, fmt.Sprintf("old-%d", i), box, now.Add(-48*time.Hour))
	}
	retainedMessage(t, server, "recent", box, now)
	other := sampleOwnership().MailboxID
	retainedMessage(t, server, "other", other, now.Add(-48*time.Hour))
	item := mailbox.PendingRetention{MailboxID: box, Cutoff: now.Add(-24 * time.Hour)}
	for i := 0; i < 3; i++ {
		result, complete, err := server.RetainPendingBatch(ctx, item, 2)
		wantDeleted := 2
		if i == 2 {
			wantDeleted = 1
		}
		if err != nil || result.Deleted != wantDeleted || result.Remaining != 6-(i*2+wantDeleted) || complete != (i == 2) {
			t.Fatal(i, result, complete, err)
		}
		if !server.storageTransactionMutex.TryLock() {
			t.Fatal("storage lock held between batches")
		}
		server.storageTransactionMutex.Unlock()
		if _, err = server.GetEmail("other"); err != nil {
			t.Fatal("unrelated mailbox affected", err)
		}
	}
	if _, err := server.GetEmail("recent"); err != nil {
		t.Fatal("new receivedAt removed", err)
	}
	if result, complete, err := server.RetainPendingBatch(ctx, item, 2); err != nil || !complete || result.Deleted != 0 || result.Remaining != 1 {
		t.Fatal("repeat not idempotent", result, complete, err)
	}
	for _, limit := range []int{0, -1, 129} {
		if _, _, err := server.RetainPendingBatch(ctx, item, limit); err == nil {
			t.Fatal("invalid batch limit accepted", limit)
		}
	}
}

func TestRetainPendingBatchRestartCompletesOriginalJournal(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	server := isolatedServer(t, dir)
	controlPath := filepath.Join(t.TempDir(), "control.sqlite")
	control, err := mailbox.Open(controlPath, true)
	if err != nil {
		t.Fatal(err)
	}
	route, err := control.EnsurePending(ctx, "main", "alice@example.com", 10)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(10 * 24 * time.Hour)
	for i := 0; i < 3; i++ {
		retainedMessage(t, server, fmt.Sprintf("old-%d", i), route.MailboxID, now.Add(-8*24*time.Hour))
	}
	retainedMessage(t, server, "recent", route.MailboxID, now.Add(-time.Hour))
	var journal mailbox.PendingRetention
	_, err = control.CleanupPendingBatch(ctx, now, 7*24*time.Hour, 24*time.Hour, "", func(ctx context.Context, item mailbox.PendingRetention) (mailbox.PendingRetentionResult, bool, error) {
		journal = item
		return server.RetainPendingBatch(ctx, item, 2)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(server.GetAllEmail()) != 2 {
		t.Fatal("batch did not bound physical deletions")
	}
	if _, err = control.EnsurePending(ctx, "main", "alice@example.com", 10); !errors.Is(err, mailbox.ErrRetentionPending) {
		t.Fatal("partial journal unfroze route", err)
	}
	if err = server.Close(); err != nil {
		t.Fatal(err)
	}
	if err = control.Close(); err != nil {
		t.Fatal(err)
	}
	server = isolatedServer(t, dir)
	control, err = mailbox.Open(controlPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = control.Close() }()
	// Startup/offline recovery remains full and idempotent, including a journal
	// that was written by the new online batch API before the restart.
	if err = control.RunPendingRetention(ctx, func(ctx context.Context, item mailbox.PendingRetention) (mailbox.PendingRetentionResult, error) {
		if item != journal {
			t.Fatal("cutoff changed across restart")
		}
		return server.RetainPending(ctx, item)
	}); err != nil {
		t.Fatal(err)
	}
	if emails := server.GetAllEmail(); len(emails) != 1 || emails[0].ID != "recent" {
		t.Fatal("recovery removed newer data or kept expired data", emails)
	}
	if next, err := control.EnsurePending(ctx, "main", "alice@example.com", 10); err != nil || next.MailboxID != route.MailboxID {
		t.Fatal("nonempty mailbox retired", err)
	}
	if err = control.RunPendingRetention(ctx, server.RetainPending); err != nil {
		t.Fatal(err)
	}
}

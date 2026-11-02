package mailserver

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/mailbox"
)

func retainedMessage(t *testing.T, server *MailServer, id, box string, received time.Time) {
	t.Helper()
	ownership := sampleOwnership()
	ownership.MailboxID = box
	raw := "From: sender@example.com\r\nTo: alice@example.com\r\nDate: Mon, 01 Jan 1900 00:00:00 +0000\r\nSubject: pending\r\n\r\nbody"
	if err := server.storeIncomingEmail(id, strings.NewReader(raw), &Session{mailServer: server, ownership: ownership, to: []string{"alice@example.com"}}); err != nil {
		t.Fatal(err)
	}
	server.storeMutex.Lock()
	defer server.storeMutex.Unlock()
	server.receivedAtByID[id] = received
	if err := server.persistEmailMetadataAt(server.storeByID[id], received); err != nil {
		t.Fatal(err)
	}
}
func TestRetainPendingUsesReceivedAtAndNeverDeletesPersonal(t *testing.T) {
	ctx := context.Background()
	server := isolatedServer(t, t.TempDir())
	control, err := mailbox.Open(filepath.Join(t.TempDir(), "control.sqlite"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = control.Close() }()
	pending, err := control.EnsurePending(ctx, "main", "alice@example.com", 10)
	if err != nil {
		t.Fatal(err)
	}
	personal, err := control.BindLogin(ctx, mailbox.Login{Namespace: "main", Subject: "bob"}, mailbox.VerifiedEmail)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(10 * 24 * time.Hour)
	retainedMessage(t, server, "old", pending.MailboxID, now.Add(-8*24*time.Hour))
	retainedMessage(t, server, "new", pending.MailboxID, now.Add(-time.Hour))
	retainedMessage(t, server, "personal", personal.MailboxID, now.Add(-30*24*time.Hour))
	if err = control.CleanupPending(ctx, now, 7*24*time.Hour, 24*time.Hour, server.RetainPending); err != nil {
		t.Fatal(err)
	}
	if _, err = server.GetEmail("old"); !errors.Is(err, ErrEmailNotFound) {
		t.Fatal("expired message remained", err)
	}
	if _, err = server.GetEmail("new"); err != nil {
		t.Fatal("old MIME Date deleted recently received message", err)
	}
	if _, err = server.GetEmail("personal"); err != nil {
		t.Fatal("personal message deleted", err)
	}
	again, err := control.EnsurePending(ctx, "main", "alice@example.com", 10)
	if err != nil || again.MailboxID != pending.MailboxID {
		t.Fatal("nonempty pending box retired", err)
	}
	if err = control.CleanupPending(ctx, now.Add(8*24*time.Hour), 7*24*time.Hour, 24*time.Hour, server.RetainPending); err != nil {
		t.Fatal(err)
	}
	again, err = control.EnsurePending(ctx, "main", "alice@example.com", 10)
	if err != nil || again.MailboxID == pending.MailboxID {
		t.Fatal("empty expired pending box not retired", err)
	}
}
func TestRetainPendingCrashRecoveryAndClaimExclusion(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	server := isolatedServer(t, directory)
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
	for _, id := range []string{"first", "second"} {
		retainedMessage(t, server, id, route.MailboxID, now.Add(-8*24*time.Hour))
	}
	interrupted := errors.New("crash after first deletion")
	err = control.CleanupPending(ctx, now, 7*24*time.Hour, 24*time.Hour, func(ctx context.Context, item mailbox.PendingRetention) (mailbox.PendingRetentionResult, error) {
		return server.retainPending(ctx, item, func(string) error { return interrupted })
	})
	if !errors.Is(err, interrupted) {
		t.Fatal(err)
	}
	if _, err = control.BindLogin(ctx, mailbox.Login{Namespace: "main", Subject: "alice", Email: "alice@example.com", Verified: true}, mailbox.VerifiedEmail); !errors.Is(err, mailbox.ErrRetentionPending) {
		t.Fatal("claim raced interrupted deletion", err)
	}
	if err = server.Close(); err != nil {
		t.Fatal(err)
	}
	if err = control.Close(); err != nil {
		t.Fatal(err)
	}
	server = isolatedServer(t, directory)
	control, err = mailbox.Open(controlPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = control.Close() }()
	if err = control.RunPendingRetention(ctx, server.RetainPending); err != nil {
		t.Fatal(err)
	}
	if emails := server.GetAllEmail(); len(emails) != 0 {
		t.Fatal("recovery left pending content")
	}
	claimed, err := control.BindLogin(ctx, mailbox.Login{Namespace: "main", Subject: "alice", Email: "alice@example.com", Verified: true}, mailbox.VerifiedEmail)
	if err != nil || claimed.MailboxID == route.MailboxID {
		t.Fatal("deleted source exposed", err)
	}
}
func TestPendingRetentionRespectsSourceLease(t *testing.T) {
	server := isolatedServer(t, t.TempDir())
	ownership := sampleOwnership()
	now := time.Now().UTC()
	retainedMessage(t, server, "leased", ownership.MailboxID, now.Add(-24*time.Hour))
	_, release, err := server.AcquireEmailSource("leased")
	if err != nil {
		t.Fatal(err)
	}
	item := mailbox.PendingRetention{MailboxID: ownership.MailboxID, Cutoff: now}
	if _, err = server.RetainPending(context.Background(), item); !errors.Is(err, ErrEmailSourceInUse) {
		t.Fatal("leased source deleted", err)
	}
	release()
	if _, err = server.RetainPending(context.Background(), item); err != nil {
		t.Fatal(err)
	}
}

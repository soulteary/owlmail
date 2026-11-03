package main

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/mailbox"
)

func TestPendingMaintenanceRunsAtStartupAndOnSchedule(t *testing.T) {
	cfg := multiuserComponentConfig(t)
	cfg.PendingCleanupInterval = "10ms"
	ingress, err := setupMultiuserIngress(cfg)
	if err != nil {
		t.Fatal(err)
	}
	first, err := ingress.Control.EnsurePending(context.Background(), "main", "pending@example.com", 10)
	if err != nil {
		t.Fatal(err)
	}
	if err = ingress.Control.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", cfg.ControlDatabase)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err = db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		t.Fatal(err)
	}
	// Test-only clock fixture; production mutations go through the authority.
	age := time.Now().Add(-48 * time.Hour).Unix()
	if _, err = db.Exec(`UPDATE mailbox SET created_at=? WHERE id=?`, age, first.MailboxID); err != nil {
		t.Fatal(err)
	}
	server, err := createMailServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	second, err := server.MailboxControl().EnsurePending(context.Background(), "main", "pending@example.com", 10)
	if err != nil {
		t.Fatal(err)
	}
	if second.MailboxID == first.MailboxID {
		t.Fatal("startup did not retire empty old pending mailbox")
	}
	if _, err = db.Exec(`UPDATE mailbox SET created_at=? WHERE id=?`, age, second.MailboxID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var retired sql.NullInt64
		if err = db.QueryRowContext(ctx, `SELECT retired_at FROM mailbox WHERE id=?`, second.MailboxID).Scan(&retired); err != nil {
			t.Fatal(err)
		}
		if retired.Valid {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("scheduled pending cleanup did not run")
		case <-ticker.C:
		}
	}
	third, err := server.MailboxControl().EnsurePending(context.Background(), "main", "pending@example.com", 10)
	if err != nil {
		t.Fatal(err)
	}
	if third.MailboxID == second.MailboxID {
		t.Fatal("scheduled cleanup left retired route active")
	}
	if err = server.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPendingMergeProgressDoesNotWaitForRetentionInterval(t *testing.T) {
	cfg := multiuserComponentConfig(t)
	cfg.PendingCleanupInterval = "1h"
	server, err := createMailServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	control := server.MailboxControl()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err = control.BindLogin(ctx, mailbox.Login{Namespace: "main", Subject: "alice"}, mailbox.VerifiedEmail); err != nil {
		t.Fatal(err)
	}
	if _, err = control.EnsurePending(ctx, "main", "alice@example.com", 10); err != nil {
		t.Fatal(err)
	}
	claim, err := control.BindLogin(ctx, mailbox.Login{Namespace: "main", Subject: "alice", Email: "alice@example.com", Verified: true}, mailbox.VerifiedEmail)
	if err != nil || claim.MergeID == "" {
		t.Fatal(claim, err)
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		pending, err := control.PendingMerges(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(pending) == 0 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("merge waited for hourly retention")
		case <-ticker.C:
		}
	}
	if err = server.Close(); err != nil {
		t.Fatal(err)
	}
}

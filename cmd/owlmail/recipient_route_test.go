package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/soulteary/owlmail/internal/mailbox"
)

func recipientRouteFixture(t *testing.T) (string, mailbox.OperatorRoute, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "control.sqlite")
	s, err := mailbox.Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.BindLogin(context.Background(), mailbox.Login{Namespace: "main", Subject: "alice", Email: "alice@example.test", Verified: true}, mailbox.ExplicitHistory); err != nil {
		t.Fatal(err)
	}
	b, err := s.BindLogin(context.Background(), mailbox.Login{Namespace: "main", Subject: "bob"}, mailbox.ExplicitHistory)
	if err != nil {
		t.Fatal(err)
	}
	route, err := s.InspectOperatorRoute(context.Background(), "main", "alice@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	return path, route, b.MailboxID
}

func TestRecipientRouteCLIValidation(t *testing.T) {
	var out, errout bytes.Buffer
	if err := runRecipientRoute(context.Background(), []string{"-help"}, &out, &errout); !errors.Is(err, flag.ErrHelp) {
		t.Fatal(err)
	}
	for _, args := range [][]string{{}, {"unexpected"}, {"-apply"}, {"-action=inspect", "-offline"}, {"-action=block", "-apply", "-offline"}, {"-action=block", "-confirm-plan=wrong"}} {
		if err := runRecipientRoute(context.Background(), args, &out, &errout); err == nil {
			t.Fatal("accepted incomplete options", args)
		}
	}
	if err := runRecipientRoute(context.Background(), nil, nil, &errout); err == nil {
		t.Fatal("nil output")
	}
}

func TestRecipientRouteCLIPlanFenceAndTwoStepReassignment(t *testing.T) {
	database, route, newBox := recipientRouteFixture(t)
	ctx := context.Background()
	var out, errout bytes.Buffer
	base := []string{"-control-database", database, "-identity-namespace", "main", "-address", route.Address}
	if err := os.Remove(database + ".writer-lock"); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(database)
	if err != nil {
		t.Fatal(err)
	}
	if err = runRecipientRoute(ctx, base, &out, &errout); err != nil {
		t.Fatal(err)
	}
	var inspected mailbox.OperatorRoute
	if err = json.Unmarshal(out.Bytes(), &inspected); err != nil || inspected.ID != route.ID || inspected.Revision != route.Revision {
		t.Fatal(inspected, err)
	}
	blockArgs := append(append([]string{}, base...), "-action", "block", "-expected-route-id", route.ID, "-expected-revision", strconv.FormatInt(route.Revision, 10), "-reason", "Mailbox address reassigned")
	out.Reset()
	if err = runRecipientRoute(ctx, blockArgs, &out, &errout); err != nil {
		t.Fatal(err)
	}
	var block mailbox.RouteChangePlan
	if err = json.Unmarshal(out.Bytes(), &block); err != nil || block.Applied || block.AuditID != "" || len(block.Confirmation) != 64 {
		t.Fatal(block, err)
	}
	after, err := os.ReadFile(database)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("inspect/dry-run changed authority", err)
	}
	if _, err = os.Stat(database + ".writer-lock"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("read-only command created fence", err)
	}
	applyBlock := append(append([]string{}, blockArgs...), "-apply", "-offline", "-confirm-plan", block.Confirmation)
	wrong := append(append([]string{}, blockArgs...), "-apply", "-offline", "-confirm-plan", "wrong")
	if err = runRecipientRoute(ctx, wrong, &out, &errout); err == nil {
		t.Fatal("incorrect confirmation accepted")
	}
	if _, err = os.Stat(database + ".writer-lock"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("bad token created fence", err)
	}
	live, err := mailbox.Open(database, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = runRecipientRoute(ctx, applyBlock, &out, &errout); !errors.Is(err, mailbox.ErrWriterActive) {
		_ = live.Close()
		t.Fatal("live writer not refused", err)
	}
	if err = live.Close(); err != nil {
		t.Fatal(err)
	}
	mail := filepath.Join(t.TempDir(), "old-private.eml")
	original := []byte("Subject: old owner only\r\n\r\nprivate history")
	if err = os.WriteFile(mail, original, 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err = runRecipientRoute(ctx, applyBlock, &out, &errout); err != nil {
		t.Fatal(err)
	}
	var blocked mailbox.RouteChangePlan
	if err = json.Unmarshal(out.Bytes(), &blocked); err != nil || !blocked.Applied || blocked.AuditID == "" {
		t.Fatal(blocked, err)
	}
	reassignArgs := append(append([]string{}, base...), "-action", "reassign", "-expected-route-id", route.ID, "-expected-revision", strconv.FormatInt(blocked.NextRevision, 10), "-target-subject", "bob", "-reason", "New address ownership confirmed")
	out.Reset()
	if err = runRecipientRoute(ctx, reassignArgs, &out, &errout); err != nil {
		t.Fatal(err)
	}
	var move mailbox.RouteChangePlan
	if err = json.Unmarshal(out.Bytes(), &move); err != nil || move.TargetMailboxID != newBox {
		t.Fatal(move, err)
	}
	applyMove := append(append([]string{}, reassignArgs...), "-apply", "-offline", "-confirm-plan", move.Confirmation)
	out.Reset()
	if err = runRecipientRoute(ctx, applyMove, &out, &errout); err != nil {
		t.Fatal(err)
	}
	restarted, err := mailbox.Open(database, false)
	if err != nil {
		t.Fatal("authority failed restart validation", err)
	}
	defer func() { _ = restarted.Close() }()
	got, err := restarted.InspectOperatorRoute(ctx, "main", route.Address)
	if err != nil || got.Subject != "bob" || got.MailboxID != newBox || got.Revision != route.Revision+2 {
		t.Fatal(got, err)
	}
	old, err := restarted.PersonalMailbox(ctx, "main", "alice")
	if err != nil || old != route.MailboxID {
		t.Fatal("old owner changed", old, err)
	}
	contents, err := os.ReadFile(mail)
	if err != nil || !bytes.Equal(contents, original) {
		t.Fatal("old mail changed", err)
	}
}

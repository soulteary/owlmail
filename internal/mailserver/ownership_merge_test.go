package mailserver

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/soulteary/owlmail/internal/mailbox"
)

func TestMergeOwnershipRecoversAfterMetadataCommit(t *testing.T) {
	dir := t.TempDir()
	s := isolatedServer(t, dir)
	source, target := uuid.NewString(), uuid.NewString()
	for _, id := range []string{"a", "b"} {
		own := sampleOwnership()
		own.MailboxID = source
		session := &Session{mailServer: s, ownership: own, to: []string{"alice@example.com"}}
		if err := s.storeIncomingEmail(id, strings.NewReader("Subject: history\r\n\r\nbody"), session); err != nil {
			t.Fatal(err)
		}
	}
	fail := errors.New("crash after sidecar commit")
	if err := s.mergeOwnership(context.Background(), source, target, func(string) error { return fail }); !errors.Is(err, fail) {
		t.Fatal(err)
	}
	persisted, err := s.loadEmailMetadata("a")
	if err != nil || persisted.Ownership.MailboxID != target {
		t.Fatal("durable fence missing", err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s = isolatedServer(t, dir)
	if err = s.MergeOwnership(context.Background(), source, target); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b"} {
		email, err := s.GetEmail(id)
		if err != nil || email.Ownership.MailboxID != target {
			t.Fatal("incomplete recovery", id, err)
		}
	}
	if err = s.MergeOwnership(context.Background(), source, target); err != nil {
		t.Fatal("not idempotent", err)
	}
}
func TestControlAndFileMergePreserveIdentityAndDelivery(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := isolatedServer(t, dir)
	control, err := mailbox.Open(t.TempDir()+"/control.sqlite", true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = control.Close() }()
	personal, err := control.BindLogin(ctx, mailbox.Login{Namespace: "main", Subject: "alice"}, mailbox.VerifiedEmail)
	if err != nil {
		t.Fatal(err)
	}
	route, err := control.EnsurePending(ctx, "main", "alice@example.com", 10)
	if err != nil {
		t.Fatal(err)
	}
	own := sampleOwnership()
	own.MailboxID = route.MailboxID
	if err = s.storeIncomingEmail("history", strings.NewReader("Subject: history\r\n\r\nbody"), &Session{mailServer: s, ownership: own, to: []string{"alice@example.com"}}); err != nil {
		t.Fatal(err)
	}
	before, err := s.GetEmail("history")
	if err != nil {
		t.Fatal(err)
	}
	cursor := s.DeliveryCursor()
	claimed, err := control.BindLogin(ctx, mailbox.Login{Namespace: "main", Subject: "alice", Email: "alice@example.com", Verified: true}, mailbox.VerifiedEmail)
	if err != nil || claimed.MergeID == "" {
		t.Fatal(claimed, err)
	}
	if err = control.RunPendingMerges(ctx, func(ctx context.Context, item mailbox.Merge) error {
		return s.MergeOwnership(ctx, item.SourceID, item.TargetID)
	}); err != nil {
		t.Fatal(err)
	}
	after, err := s.GetEmail("history")
	if err != nil {
		t.Fatal(err)
	}
	if after.Ownership.MailboxID != personal.MailboxID || after.ID != before.ID || after.Read != before.Read || after.Ownership.TransactionID != before.Ownership.TransactionID {
		t.Fatal("merge changed message identity or delivery evidence")
	}
	if s.DeliveryCursor() != cursor {
		t.Fatal("historical message replayed as a new delivery")
	}
	pending, err := control.PendingMerges(ctx)
	if err != nil || len(pending) != 0 {
		t.Fatal(pending, err)
	}
}
func TestMergeOwnershipRequiresIsolatedWritableStore(t *testing.T) {
	s, err := NewMailServer(0, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err = s.MergeOwnership(context.Background(), uuid.NewString(), uuid.NewString()); err == nil {
		t.Fatal("standalone ownership mutation allowed")
	}
}

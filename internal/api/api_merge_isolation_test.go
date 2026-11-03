package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/soulteary/owlmail/internal/mailbox"
)

func TestLoginDoesNotRunAnotherIdentitysPendingMerge(t *testing.T) {
	f := newMultiFixture(t)
	ctx := context.Background()
	if _, err := f.store.EnsurePending(ctx, "main", "alice@example.com", 10); err != nil {
		t.Fatal(err)
	}
	claim, err := f.store.BindLogin(ctx, mailbox.Login{Namespace: "main", Subject: "alice", Email: "alice@example.com", Verified: true}, mailbox.VerifiedEmail)
	if err != nil || claim.MergeID == "" {
		t.Fatal(claim, err)
	}
	// The old global replay would complete Alice's empty journal during Bob's
	// first request. Bob must not execute, wait on, or surface failures from it.
	response := f.request(t, "bob", http.MethodGet, "/api/v1/emails", "")
	if response.StatusCode != http.StatusOK {
		t.Fatal("unrelated login failed", response.StatusCode)
	}
	pending, err := f.store.PendingMerges(ctx)
	if err != nil || len(pending) != 1 || pending[0].ID != claim.MergeID {
		t.Fatal("request executed unrelated recovery", pending, err)
	}
}

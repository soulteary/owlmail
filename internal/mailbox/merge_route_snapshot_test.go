package mailbox

import (
	"context"
	"errors"
	"testing"
)

func TestExpectedRouteAllowsAuditedPendingClaim(t *testing.T) {
	for _, policy := range []ClaimPolicy{VerifiedEmail, ExplicitHistory} {
		t.Run(string(policy), func(t *testing.T) {
			s := testStore(t)
			ctx := context.Background()
			// Existing personal mailbox exercises the source→target transition instead
			// of only the same-id pending promotion case.
			owner, err := s.BindLogin(ctx, Login{"main", "alice", "", false}, policy)
			if err != nil {
				t.Fatal(err)
			}
			old, err := s.EnsurePending(ctx, "main", "alice@example.com", 10)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.BindLogin(ctx, Login{"main", "alice", "alice@example.com", true}, policy); err != nil {
				t.Fatal(err)
			}
			if err = s.WithExpectedRouteSnapshot(ctx, "main", []Route{old}, func(current []Route) error {
				if current[0].MailboxID != owner.MailboxID {
					t.Fatal("did not follow authorized claim")
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestExpectedRouteRejectsReassignmentAndRecreation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	alice, err := s.BindLogin(ctx, Login{"main", "alice", "alice@example.com", true}, VerifiedEmail)
	if err != nil {
		t.Fatal(err)
	}
	bob, err := s.BindLogin(ctx, Login{"main", "bob", "bob@example.com", true}, VerifiedEmail)
	if err != nil {
		t.Fatal(err)
	}
	old, err := s.EnsurePending(ctx, "main", "alice@example.com", 10)
	if err != nil {
		t.Fatal(err)
	}
	if old.MailboxID != alice.MailboxID {
		t.Fatal("wrong old owner")
	}
	if _, err = s.db.Exec(`UPDATE recipient_route SET mailbox_id=?,revision=revision+1 WHERE id=?`, bob.MailboxID, old.ID); err != nil {
		t.Fatal(err)
	}
	called := false
	if err = s.WithExpectedRouteSnapshot(ctx, "main", []Route{old}, func([]Route) error { called = true; return nil }); !errors.Is(err, ErrRouteChanged) || called {
		t.Fatal("silently followed reassignment", err)
	}
	// Even a fresh pending RCPT cannot follow a target replacement without the
	// binding audit that proves a legitimate claim.
	pending, err := s.EnsurePending(ctx, "main", "pending@example.com", 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`UPDATE recipient_route SET mailbox_id=?,state='bound',revision=revision+1 WHERE id=?`, bob.MailboxID, pending.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.WithExpectedRouteSnapshot(ctx, "main", []Route{pending}, func([]Route) error { t.Fatal("unaudited route replacement accepted"); return nil }); !errors.Is(err, ErrRouteChanged) {
		t.Fatal(err)
	}
}

func TestExpectedRouteAllowsDirectPendingPromotion(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	old, err := s.EnsurePending(ctx, "main", "alice@example.com", 10)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := s.BindLogin(ctx, Login{"main", "alice", "alice@example.com", true}, VerifiedEmail)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.MergeID != "" || claimed.MailboxID != old.MailboxID {
		t.Fatal("direct promotion incorrectly planned a merge")
	}
	if err = s.WithExpectedRouteSnapshot(ctx, "main", []Route{old}, func(current []Route) error {
		if current[0].MailboxID != old.MailboxID || current[0].State != "bound" {
			t.Fatal("incorrect promotion target")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

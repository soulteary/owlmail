package mailbox

import (
	"context"
	"testing"
)

func TestLoginStatusDoesNotEnumerateUnverifiedAddresses(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if _, err := s.BindLogin(ctx, Login{Namespace: "main", Subject: "alice"}, ExplicitHistory); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnsurePending(ctx, "main", "pending@example.test", 10); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindLogin(ctx, Login{"main", "bob", "bound@example.test", true}, ExplicitHistory); err != nil {
		t.Fatal(err)
	}
	want := PersonalStatus{"ready", "verification_required", "none", "verify_email"}
	for _, address := range []string{"absent@example.test", "pending@example.test", "bound@example.test", "invalid"} {
		got, err := s.LoginStatus(ctx, Login{"main", "alice", address, false})
		if err != nil || got != want {
			t.Fatalf("%q: %+v %v", address, got, err)
		}
	}
	got, err := s.LoginStatus(ctx, Login{Namespace: "main", Subject: "alice"})
	if err != nil || got != (PersonalStatus{"ready", "unbound", "none", "none"}) {
		t.Fatal(got, err)
	}
	if _, err = s.LoginStatus(ctx, Login{Namespace: "other", Subject: "alice"}); err == nil {
		t.Fatal("namespace isolation lost")
	}
	if _, err = s.LoginStatus(ctx, Login{Namespace: "main", Subject: "missing"}); err == nil {
		t.Fatal("query created an identity")
	}
}

func TestLoginStatusOwnedHistoryAndConflicts(t *testing.T) {
	for _, fresh := range []bool{true, false} {
		t.Run(map[bool]string{true: "fresh", false: "existing"}[fresh], func(t *testing.T) {
			s := testStore(t)
			ctx := context.Background()
			login := Login{"main", "alice", "alice@example.test", true}
			if !fresh {
				if _, err := s.BindLogin(ctx, Login{Namespace: "main", Subject: "alice"}, ExplicitHistory); err != nil {
					t.Fatal(err)
				}
			}
			route, err := s.EnsurePending(ctx, "main", login.Email, 10)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.BindLogin(ctx, login, ExplicitHistory); err != nil {
				t.Fatal(err)
			}
			got, err := s.LoginStatus(ctx, login)
			if err != nil || got != (PersonalStatus{"ready", "bound", "retained", "none"}) {
				t.Fatal(got, err)
			}
			other := Login{"main", "bob", login.Email, true}
			if _, err = s.BindLogin(ctx, other, ExplicitHistory); err != nil {
				t.Fatal(err)
			}
			got, err = s.LoginStatus(ctx, other)
			if err != nil || got != (PersonalStatus{"ready", "conflict", "none", "contact_operator"}) {
				t.Fatal(got, err)
			}
			// Historical details disappear after retention retires the isolated source.
			if _, err = s.db.Exec(`UPDATE mailbox SET retired_at=1 WHERE id=?`, route.MailboxID); err != nil {
				t.Fatal(err)
			}
			got, err = s.LoginStatus(ctx, login)
			if err != nil || got.HistoryState != "none" {
				t.Fatal(got, err)
			}
		})
	}
}

func TestLoginStatusMergeAndRouteRevision(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	login := Login{"main", "alice", "alice@example.test", true}
	if _, err := s.BindLogin(ctx, Login{Namespace: "main", Subject: "alice"}, VerifiedEmail); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnsurePending(ctx, "main", login.Email, 10); err != nil {
		t.Fatal(err)
	}
	result, err := s.BindLogin(ctx, login, VerifiedEmail)
	if err != nil || result.MergeID == "" {
		t.Fatal(result, err)
	}
	got, err := s.LoginStatus(ctx, login)
	if err != nil || got != (PersonalStatus{"recovering", "bound", "merging", "retry"}) {
		t.Fatal(got, err)
	}
	if err = s.RunPendingMerges(ctx, func(context.Context, Merge) error { return nil }); err != nil {
		t.Fatal(err)
	}
	got, err = s.LoginStatus(ctx, login)
	if err != nil || got != (PersonalStatus{"ready", "bound", "complete", "none"}) {
		t.Fatal(got, err)
	}
	if _, err = s.db.Exec(`UPDATE recipient_route SET revision=revision+1 WHERE namespace='main' AND address=?`, login.Email); err != nil {
		t.Fatal(err)
	}
	got, err = s.LoginStatus(ctx, login)
	if err != nil || got.HistoryState != "none" {
		t.Fatal("stale audit reused", got, err)
	}
	if _, err = s.db.Exec(`UPDATE recipient_route SET state='blocked' WHERE namespace='main' AND address=?`, login.Email); err != nil {
		t.Fatal(err)
	}
	got, err = s.LoginStatus(ctx, login)
	if err != nil || got != (PersonalStatus{"ready", "conflict", "none", "contact_operator"}) {
		t.Fatal(got, err)
	}
}

func TestLoginStatusUnboundVerifiedDoesNotCreateRoute(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if _, err := s.BindLogin(ctx, Login{Namespace: "main", Subject: "alice"}, ExplicitHistory); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoginStatus(ctx, Login{"main", "alice", "new@example.test", true})
	if err != nil || got != (PersonalStatus{"ready", "unbound", "none", "none"}) {
		t.Fatal(got, err)
	}
	var count int
	if err = s.db.QueryRow(`SELECT count(*) FROM recipient_route`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

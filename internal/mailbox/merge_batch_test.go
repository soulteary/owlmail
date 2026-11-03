package mailbox

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestMergeBatchScopesAndReleasesBetweenSteps(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	alice, _ := plannedMerge(t, s)
	bob, err := s.BindLogin(ctx, Login{"main", "bob", "", false}, VerifiedEmail)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.EnsurePending(ctx, "main", "bob@example.com", 10); err != nil {
		t.Fatal(err)
	}
	if _, err = s.BindLogin(ctx, Login{"main", "bob", "bob@example.com", true}, VerifiedEmail); err != nil {
		t.Fatal(err)
	}
	calls := 0
	id, err := s.RunMergeBatch(ctx, alice.MailboxID, "", func(_ context.Context, item Merge) (bool, error) {
		calls++
		if item.TargetID != alice.MailboxID {
			t.Fatal("unrelated merge selected")
		}
		return false, nil
	})
	if err != nil || id == "" || calls != 1 {
		t.Fatal(id, calls, err)
	}
	// An incomplete batch releases the control lock and leaves the journal frozen.
	if _, err = s.PersonalMailbox(ctx, "main", "bob"); err != nil {
		t.Fatal(err)
	}
	items, err := s.PendingMerges(ctx)
	if err != nil || len(items) != 2 {
		t.Fatal(items, err)
	}
	if _, err = s.RunMergeBatch(ctx, bob.MailboxID, "", func(_ context.Context, item Merge) (bool, error) {
		if item.TargetID != bob.MailboxID {
			t.Fatal("wrong owner")
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RunMergeBatch(ctx, alice.MailboxID, "", func(context.Context, Merge) (bool, error) { return true, nil }); err != nil {
		t.Fatal(err)
	}
	items, err = s.PendingMerges(ctx)
	if err != nil || len(items) != 0 {
		t.Fatal(items, err)
	}
}

func TestMergeBatchCursorDoesNotStarveOtherMailboxes(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	for i := range 3 {
		subject := fmt.Sprintf("user%d", i)
		if _, err := s.BindLogin(ctx, Login{"main", subject, "", false}, VerifiedEmail); err != nil {
			t.Fatal(err)
		}
		address := subject + "@example.com"
		if _, err := s.EnsurePending(ctx, "main", address, 10); err != nil {
			t.Fatal(err)
		}
		if _, err := s.BindLogin(ctx, Login{"main", subject, address, true}, VerifiedEmail); err != nil {
			t.Fatal(err)
		}
	}
	failure := errors.New("one mailbox cannot persist")
	first, err := s.RunMergeBatch(ctx, "", "", func(context.Context, Merge) (bool, error) { return false, failure })
	if first == "" || !errors.Is(err, failure) {
		t.Fatal(first, err)
	}
	cursor := first
	for range 2 {
		next, err := s.RunMergeBatch(ctx, "", cursor, func(_ context.Context, item Merge) (bool, error) {
			if item.ID == first {
				t.Fatal("failed task starved a different mailbox")
			}
			return true, nil
		})
		if err != nil || next == "" {
			t.Fatal(next, err)
		}
		cursor = next
	}
	end, err := s.RunMergeBatch(ctx, "", cursor, func(context.Context, Merge) (bool, error) { t.Fatal("cursor did not end"); return false, nil })
	if err != nil || end != "" {
		t.Fatal(end, err)
	}
	retried, err := s.RunMergeBatch(ctx, "", end, func(context.Context, Merge) (bool, error) { return true, nil })
	if err != nil || retried != first {
		t.Fatal(retried, err)
	}
}

func TestMergeBatchRejectsInvalidGraphBeforeCallback(t *testing.T) {
	s := testStore(t)
	_, source := plannedMerge(t, s)
	if _, err := s.db.Exec(`UPDATE recipient_route SET mailbox_id=?`, source.MailboxID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RunMergeBatch(context.Background(), "", "", func(context.Context, Merge) (bool, error) {
		t.Fatal("invalid graph reached mutation")
		return true, nil
	}); err == nil {
		t.Fatal("invalid graph accepted")
	}
}

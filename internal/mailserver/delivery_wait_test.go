package mailserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func deliveryTestServer(t *testing.T) *MailServer {
	t.Helper()
	s, err := NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
func commitTestDelivery(t *testing.T, s *MailServer, id, run, recipient string) {
	t.Helper()
	if err := s.SaveEmailToStore(id, false, &Envelope{To: []string{recipient}}, &Email{Subject: "Verification", Text: "123456", RunID: run}); err != nil {
		t.Fatal(err)
	}
}
func TestDeliveryCursorReplaysAndIsolates(t *testing.T) {
	s := deliveryTestServer(t)
	cursor := s.DeliveryCursor()
	commitTestDelivery(t, s, "other", "run-other", "a@example.test")
	commitTestDelivery(t, s, "wanted", "run-a", "a@example.test")
	result, err := s.WaitDeliveries(context.Background(), cursor, DeliveryFilter{RunID: "run-a", Recipient: "a@example.test"}, 1, time.Second)
	if err != nil || result.TimedOut || len(result.Emails) != 1 || result.Emails[0].ID != "wanted" {
		t.Fatalf("%+v, %v", result, err)
	}
	result.Emails[0].Subject = "mutated"
	e, _ := s.GetEmail("wanted")
	if e.Subject == "mutated" || e.Read {
		t.Fatal("wait mutated stored mail")
	}
	for _, bad := range []string{"garbage", cursor + "0", deliveryTestServer(t).DeliveryCursor()} {
		if _, err := s.WaitDeliveries(context.Background(), bad, DeliveryFilter{}, 1, time.Second); !errors.Is(err, ErrInvalidCursor) {
			t.Fatalf("cursor %q: %v", bad, err)
		}
	}
}
func TestDeliveryWaitParallelCancellationAndShutdown(t *testing.T) {
	s := deliveryTestServer(t)
	cursor := s.DeliveryCursor()
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, err := s.WaitDeliveries(context.Background(), cursor, DeliveryFilter{RunID: fmt.Sprintf("run-%d", i)}, 1, time.Second)
			if err != nil || len(r.Emails) != 1 || r.Emails[0].RunID != fmt.Sprintf("run-%d", i) {
				t.Errorf("worker %d: %+v %v", i, r, err)
			}
		}(i)
	}
	for i := 0; i < 12; i++ {
		commitTestDelivery(t, s, fmt.Sprintf("id-%d", i), fmt.Sprintf("run-%d", i), "a@example.test")
	}
	wg.Wait()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.WaitDeliveries(ctx, cursor, DeliveryFilter{}, 1, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	result, err := s.WaitDeliveries(context.Background(), s.DeliveryCursor(), DeliveryFilter{}, 1, time.Millisecond)
	if err != nil || !result.TimedOut {
		t.Fatalf("timeout: %+v %v", result, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.WaitDeliveries(context.Background(), cursor, DeliveryFilter{}, 1, time.Second); !errors.Is(err, ErrMailboxClosed) {
		t.Fatal(err)
	}
}
func TestRunIDParseAndRecovery(t *testing.T) {
	s := deliveryTestServer(t)
	msg := "From: a@example.test\r\nTo: b@example.test\r\nX-OwlMail-Run-ID: run-42\r\nSubject: Test\r\n\r\nBody"
	e, _, err := s.parseEmailMessage("test", strings.NewReader(msg), nil, false, "")
	if err != nil || e.RunID != "run-42" {
		t.Fatalf("%+v %v", e, err)
	}
	for _, bad := range []string{strings.Repeat("a", 129), "run/42", "run 42"} {
		if ValidateRunID(bad) == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	if _, _, err := s.parseEmailMessage("test", strings.NewReader(strings.Replace(msg, "run-42", "run/42", 1)), &Session{}, false, ""); err == nil {
		t.Fatal("accepted invalid header")
	}
}

func TestCursorCountDeletionAndCapacity(t *testing.T) {
	s := deliveryTestServer(t)
	cursor := s.DeliveryCursor()
	commitTestDelivery(t, s, "first", "run-a", "a@example.test")
	result, err := s.WaitDeliveries(context.Background(), cursor, DeliveryFilter{RunID: "run-a"}, 2, time.Millisecond)
	if err != nil || !result.TimedOut || len(result.Emails) != 1 {
		t.Fatalf("partial %+v %v", result, err)
	}
	if err := s.DeleteAllEmail(); err != nil {
		t.Fatal(err)
	}
	commitTestDelivery(t, s, "second", "run-a", "a@example.test")
	result, err = s.WaitDeliveries(context.Background(), cursor, DeliveryFilter{}, 1, time.Second)
	if err != nil || result.Emails[0].ID != "second" {
		t.Fatalf("post-delete %+v %v", result, err)
	}
	s.storeMutex.Lock()
	s.deliveryWaiters = 64
	s.storeMutex.Unlock()
	if _, err := s.WaitDeliveries(context.Background(), cursor, DeliveryFilter{}, 1, time.Second); !errors.Is(err, ErrWaitCapacity) {
		t.Fatal(err)
	}
	s.storeMutex.Lock()
	s.deliveryWaiters = 0
	s.storeMutex.Unlock()
}

func TestRunIDSurvivesRestartAndOldCursorExpires(t *testing.T) {
	s := deliveryTestServer(t)
	old := s.DeliveryCursor()
	raw := "From: a@example.test\r\nTo: b@example.test\r\nX-OwlMail-Run-ID: restart-42\r\nSubject: Recovery\r\n\r\nBody"
	if err := os.WriteFile(filepath.Join(s.GetMailDir(), "persisted.eml"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	next, err := NewMailServer(1025, "localhost", s.GetMailDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = next.Close() }()
	if err := next.LoadMailsFromDirectory(); err != nil {
		t.Fatal(err)
	}
	email, err := next.GetEmail("persisted")
	if err != nil || email.RunID != "restart-42" {
		t.Fatalf("%+v %v", email, err)
	}
	if _, err := next.WaitDeliveries(context.Background(), old, DeliveryFilter{}, 1, time.Second); !errors.Is(err, ErrInvalidCursor) {
		t.Fatal(err)
	}
}

func TestLegacyInvalidRunHeadersRemainReadableOnRecovery(t *testing.T) {
	for _, header := range []string{"X-OwlMail-Run-ID: run/legacy", "X-OwlMail-Run-ID: one\r\nX-OwlMail-Run-ID: two"} {
		t.Run(header, func(t *testing.T) {
			s := deliveryTestServer(t)
			raw := "From: sender@example.test\r\n" + header + "\r\nSubject: Legacy\r\n\r\nPreserve this body"
			if err := os.WriteFile(filepath.Join(s.GetMailDir(), "legacy.eml"), []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			if err := s.LoadMailsFromDirectory(); err != nil {
				t.Fatal(err)
			}
			email, err := s.GetEmail("legacy")
			if err != nil || email.RunID != "" || !strings.Contains(email.Text, "Preserve this body") {
				t.Fatal(email, err)
			}
			if _, _, err = s.parseEmailMessage("new", strings.NewReader(raw), &Session{}, false, ""); err == nil {
				t.Fatal("accepted invalid SMTP header")
			}
		})
	}
}

func TestDeliveryWaitCursorDoesNotSkipUnreturnedMatches(t *testing.T) {
	s := deliveryTestServer(t)
	cursor := s.DeliveryCursor()
	for i := 0; i < 5; i++ {
		commitTestDelivery(t, s, fmt.Sprintf("batch-%d", i), "batch", "a@example.test")
	}
	var received []string
	for i := 0; i < 3; i++ {
		result, err := s.WaitDeliveries(context.Background(), cursor, DeliveryFilter{RunID: "batch"}, 2, time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		for _, email := range result.Emails {
			received = append(received, email.ID)
		}
		cursor = result.Cursor
	}
	if strings.Join(received, ",") != "batch-0,batch-1,batch-2,batch-3,batch-4" {
		t.Fatal(received)
	}
}

func TestDeliveryWaitSequenceOrderSurvivesReloadSort(t *testing.T) {
	server := deliveryTestServer(t)
	cursor := server.DeliveryCursor()
	for index, id := range []string{"a-newer", "z-older"} {
		path := filepath.Join(server.mailDir, id+".eml")
		if err := os.WriteFile(path, []byte("Subject: reload\r\n\r\nbody"), 0600); err != nil {
			t.Fatal(err)
		}
		stamp := time.Now().Add(-time.Duration(index+1) * time.Hour)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	if err := server.LoadMailsFromDirectory(); err != nil {
		t.Fatal(err)
	}
	server.storeMutex.RLock()
	first := server.storeOrder[0]
	server.storeMutex.RUnlock()
	if first != "z-older" {
		t.Fatal("reload did not exercise reversed capture order", first)
	}
	for _, expected := range []string{"a-newer", "z-older"} {
		result, err := server.WaitDeliveries(context.Background(), cursor, DeliveryFilter{}, 1, time.Second)
		if err != nil || len(result.Emails) != 1 || result.Emails[0].ID != expected {
			t.Fatal(result, err, expected)
		}
		cursor = result.Cursor
	}
}

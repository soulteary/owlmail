package relay

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/outgoing"
	"github.com/soulteary/owlmail/internal/types"
)

func serviceFixture(t *testing.T) (*Service, *fakeMailbox) {
	t.Helper()
	directory := t.TempDir()
	mailbox := newFakeMailbox(directory)
	mailbox.config = &outgoing.OutgoingConfig{Host: "localhost", Port: 25}
	mailbox.emails["mail"] = &types.Email{ID: "mail", Envelope: &types.Envelope{To: []string{"recipient@example.test"}, SMTPTransaction: true}}
	service := New(mailbox, directory)
	t.Cleanup(func() { _ = service.Close() })
	return service, mailbox
}
func awaitService(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("service did not reach expected state")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestHeldHandoffRequiresDurableCommit(t *testing.T) {
	service, mailbox := serviceFixture(t)
	var sent atomic.Int32
	mailbox.send = func(_ *types.Email, _ []string, callback func(error)) error { sent.Add(1); callback(nil); return nil }
	job, err := service.Submit(Request{EmailID: "mail", Origin: "auto", Key: "mail", DeferDispatch: true})
	if err != nil {
		t.Fatal(err)
	}
	service.Start()
	time.Sleep(30 * time.Millisecond)
	if sent.Load() != 0 {
		t.Fatal("unacknowledged handoff dispatched")
	}
	if err := service.CommitHandoff(job.ID); err != nil {
		t.Fatal(err)
	}
	awaitService(t, func() bool { got, _ := service.Get(job.ID); return got.Status == Succeeded && !got.ReceiptPending })
	if err := service.CommitHandoff(job.ID); err != nil {
		t.Fatal(err)
	}
	if sent.Load() != 1 {
		t.Fatalf("deliveries=%d", sent.Load())
	}
}
func TestWaitCancellationDoesNotDiscardAcceptedJob(t *testing.T) {
	service, _ := serviceFixture(t)
	job, err := service.Submit(Request{EmailID: "mail"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Wait(ctx, job.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait=%v", err)
	}
	service.Start()
	awaitService(t, func() bool { got, _ := service.Get(job.ID); return got.Status == Succeeded })
}
func TestCloseWaitsForAdmittedSubmissionAndReleasesLease(t *testing.T) {
	service, mailbox := serviceFixture(t)
	entered := make(chan struct{})
	resume := make(chan struct{})
	mailbox.acquire = func(string) { close(entered); <-resume }
	submitted := make(chan error, 1)
	go func() { _, err := service.Submit(Request{EmailID: "mail"}); submitted <- err }()
	<-entered
	closed := make(chan struct{})
	go func() { _ = service.Close(); close(closed) }()
	select {
	case <-closed:
		t.Fatal("Close finished before admitted Submit")
	case <-time.After(20 * time.Millisecond):
	}
	close(resume)
	if err := <-submitted; err != nil {
		t.Fatal(err)
	}
	<-closed
	mailbox.mu.Lock()
	defer mailbox.mu.Unlock()
	if mailbox.leases != 0 {
		t.Fatalf("leaked leases=%d", mailbox.leases)
	}
}
func TestRetryRelayJobPreservesQueuedWorkDuringShutdown(t *testing.T) {
	service, mailbox := serviceFixture(t)
	job, err := service.Submit(Request{EmailID: "mail"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.store.beginAttempt(job.ID); err != nil {
		t.Fatal(err)
	}
	queued, ok := service.store.queueRetry(job.ID, errors.New("connection refused"), time.Hour)
	if !ok {
		t.Fatal("not queued")
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	service.Start()
	reloaded, err := newPersistentRelayJobStore(mailbox.directory)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := reloaded.get(job.ID)
	if !ok || got.Status != Queued || got.Attempts != queued.Attempts || got.CompletedAt != nil {
		t.Fatalf("persisted task=%#v", got)
	}
}

func TestReceiptClearPersistenceFailureRetriesWithoutResending(t *testing.T) {
	service, mailbox := serviceFixture(t)
	var sent atomic.Int32
	mailbox.send = func(_ *types.Email, _ []string, callback func(error)) error { sent.Add(1); callback(nil); return nil }
	job, err := service.Submit(Request{EmailID: "mail"})
	if err != nil {
		t.Fatal(err)
	}
	// Drive the attempt and successful write-ahead record before injecting the
	// receipt acknowledgment fault, so only metadata repair is retried.
	service.dispatch()
	got, _ := service.Get(job.ID)
	if got.Status != Succeeded || !got.ReceiptPending {
		t.Fatalf("completion=%#v", got)
	}
	realSync := service.store.syncDirectory
	service.store.syncDirectory = func(string) error { return errors.New("receipt fsync unavailable") }
	service.repairReceipts()
	got, _ = service.Get(job.ID)
	if !got.ReceiptPending || service.receiptPending.Load() != 1 {
		t.Fatalf("failed acknowledgment lost repair: %#v", got)
	}
	service.store.syncDirectory = realSync
	service.repairReceipts()
	got, _ = service.Get(job.ID)
	if got.ReceiptPending || service.receiptPending.Load() != 0 {
		t.Fatalf("receipt did not recover: %#v", got)
	}
	if sent.Load() != 1 {
		t.Fatalf("accepted SMTP was resent %d times", sent.Load())
	}
}

func TestRecoveryPreservesCapturedEnvelopeAndHeldHandoff(t *testing.T) {
	service, mailbox := serviceFixture(t)
	envelope := &types.Envelope{From: "", To: []string{"hidden@example.test"}, SMTPTransaction: true}
	job, err := service.Submit(Request{EmailID: "mail", Origin: "auto", Key: "mail", Envelope: envelope, ConfirmedRecipients: envelope.To, DeferDispatch: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	restored := New(mailbox, mailbox.directory)
	defer func() { _ = restored.Close() }()
	got, ok := restored.Get(job.ID)
	if !ok || !got.HandoffPending || got.Envelope == nil || !got.Envelope.SMTPTransaction || got.Envelope.From != "" || got.Envelope.To[0] != "hidden@example.test" {
		t.Fatalf("restored envelope=%#v", got)
	}
	got.Envelope.To[0] = "mutated@example.test"
	again, _ := restored.Get(job.ID)
	if again.Envelope.To[0] != "hidden@example.test" {
		t.Fatal("Get leaked mutable envelope")
	}
}

func TestRecoveryAppliesRetentionOptionsBeforeLoading(t *testing.T) {
	service, mailbox := serviceFixture(t)
	job, err := service.Submit(Request{EmailID: "mail"})
	if err != nil {
		t.Fatal(err)
	}
	service.store.complete(job.ID, nil)
	service.store.mu.Lock()
	old := service.store.jobs[job.ID]
	completed := time.Now().Add(-48 * time.Hour)
	old.CompletedAt = &completed
	old.UpdatedAt = completed
	old.CreatedAt = completed
	persistErr := service.store.persistLocked(old)
	service.store.mu.Unlock()
	if persistErr != nil {
		t.Fatal(persistErr)
	}
	_ = service.Close()
	restored := NewWithOptions(mailbox, mailbox.directory, Options{TTL: 7 * 24 * time.Hour})
	defer func() { _ = restored.Close() }()
	if _, ok := restored.Get(job.ID); !ok {
		t.Fatal("default retention pruned a record before custom TTL was applied")
	}
}

func TestDisabledOutgoingPreservesAcceptedJobUntilResumed(t *testing.T) {
	service, mailbox := serviceFixture(t)
	var sent atomic.Int32
	mailbox.send = func(_ *types.Email, _ []string, callback func(error)) error { sent.Add(1); callback(nil); return nil }
	job, err := service.Submit(Request{EmailID: "mail"})
	if err != nil {
		t.Fatal(err)
	}
	if err := mailbox.SetOutgoingConfig(nil); err != nil {
		t.Fatal(err)
	}
	service.Start()
	time.Sleep(150 * time.Millisecond)
	queued, ok := service.Get(job.ID)
	if !ok || queued.Status != Queued || queued.Attempts != 0 || queued.CompletedAt != nil || sent.Load() != 0 {
		t.Fatalf("disabled host consumed accepted work: %+v sent=%d", queued, sent.Load())
	}
	mailbox.mu.Lock()
	leases := mailbox.leases
	mailbox.mu.Unlock()
	if leases == 0 {
		t.Fatal("disabled queued work lost source protection")
	}
	if err := mailbox.SetOutgoingConfig(&outgoing.OutgoingConfig{Host: "localhost", Port: 25}); err != nil {
		t.Fatal(err)
	}
	service.notify()
	awaitService(t, func() bool { got, _ := service.Get(job.ID); return got.Status == Succeeded && !got.ReceiptPending })
	if sent.Load() != 1 {
		t.Fatalf("resume deliveries=%d", sent.Load())
	}
}

func TestOutgoingDisabledDuringDispatchRestoresUnattemptedJob(t *testing.T) {
	service, mailbox := serviceFixture(t)
	var sent atomic.Int32
	mailbox.send = func(_ *types.Email, _ []string, callback func(error)) error {
		if sent.Add(1) == 1 {
			_ = mailbox.SetOutgoingConfig(nil)
			callback(outgoing.ErrNotConfigured)
			return outgoing.ErrNotConfigured
		}
		callback(nil)
		return nil
	}
	job, err := service.Submit(Request{EmailID: "mail"})
	if err != nil {
		t.Fatal(err)
	}
	service.Start()
	awaitService(t, func() bool { return sent.Load() == 1 })
	awaitService(t, func() bool { got, _ := service.Get(job.ID); return got.Status == Queued && got.Attempts == 0 })
	if err := mailbox.SetOutgoingConfig(&outgoing.OutgoingConfig{Host: "localhost", Port: 25}); err != nil {
		t.Fatal(err)
	}
	service.notify()
	awaitService(t, func() bool { got, _ := service.Get(job.ID); return got.Status == Succeeded && !got.ReceiptPending })
	if sent.Load() != 2 {
		t.Fatalf("unexpected transport calls=%d", sent.Load())
	}
}

func TestStoreRejectsReusingRequestKeyForAnotherSource(t *testing.T) {
	store := newRelayJobStore()
	if _, err := store.createRequest(Request{EmailID: "first", Origin: "auto", Key: "same"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.createRequest(Request{EmailID: "second", Origin: "auto", Key: "same"}); err == nil {
		t.Fatal("shared request key accepted another source")
	}
}

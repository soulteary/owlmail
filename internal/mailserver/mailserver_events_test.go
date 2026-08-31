package mailserver

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMailServerOn(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	eventFired := make(chan bool, 1)
	server.On("new", func(email *Email) {
		eventFired <- true
	})

	// Emit event
	email := &Email{ID: "test-id", Subject: "Test"}
	server.emit("new", email)

	// Wait for event handler to be called
	select {
	case <-eventFired:
		// Event handler was called
	case <-time.After(1 * time.Second):
		t.Error("Event handler should have been called")
	}
}

func TestEventListenersReceiveIndependentSnapshots(t *testing.T) {
	server, err := NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	mutated := make(chan struct{})
	observed := make(chan string, 1)
	server.On("isolated", func(email *Email) {
		email.Subject = "mutated"
		email.Envelope.To[0] = "mutated@example.test"
		close(mutated)
	})
	server.On("isolated", func(email *Email) {
		<-mutated
		observed <- email.Subject + ":" + email.Envelope.To[0]
	})

	original := &Email{Subject: "original", Envelope: &Envelope{To: []string{"receiver@example.test"}}}
	server.emit("isolated", original)
	select {
	case got := <-observed:
		if got != "original:receiver@example.test" {
			t.Fatalf("listener observed another listener's mutation: %s", got)
		}
	case <-time.After(time.Second):
		t.Fatal("listeners did not finish")
	}
	if original.Subject != "original" || original.Envelope.To[0] != "receiver@example.test" {
		t.Fatal("listener mutated the emitter-owned object")
	}
}

func TestOnWithConcurrencyBoundsHandlersBeforeStartingGoroutines(t *testing.T) {
	server, err := NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	const total = 12
	const limit = 2
	var active atomic.Int32
	var maximum atomic.Int32
	started := make(chan struct{}, total)
	done := make(chan struct{}, total)
	release := make(chan struct{})
	if err := server.OnWithConcurrency("limited", limit, func(_ *Email) {
		current := active.Add(1)
		for {
			observed := maximum.Load()
			if current <= observed || maximum.CompareAndSwap(observed, current) {
				break
			}
		}
		started <- struct{}{}
		<-release
		active.Add(-1)
		done <- struct{}{}
	}); err != nil {
		t.Fatal(err)
	}

	var emitters sync.WaitGroup
	for index := 0; index < total; index++ {
		emitters.Add(1)
		go func() {
			defer emitters.Done()
			server.emit("limited", &Email{})
		}()
	}
	for index := 0; index < limit; index++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("limited handlers did not start")
		}
	}
	select {
	case <-started:
		t.Fatal("handler started above the configured concurrency limit")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	emitters.Wait()
	for index := 0; index < total; index++ {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("limited handler did not finish")
		}
	}
	if got := maximum.Load(); got > limit {
		t.Fatalf("maximum concurrency = %d, want <= %d", got, limit)
	}
}

func TestEmitStartsUnlimitedListenersBeforeBoundedBackpressure(t *testing.T) {
	server, err := NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	boundedStarted := make(chan struct{}, 1)
	release := make(chan struct{})
	if err := server.OnWithConcurrency("new", 1, func(_ *Email) {
		boundedStarted <- struct{}{}
		<-release
	}); err != nil {
		t.Fatal(err)
	}
	server.emit("new", &Email{})
	<-boundedStarted

	unlimitedStarted := make(chan struct{}, 1)
	server.On("new", func(_ *Email) { unlimitedStarted <- struct{}{} })
	secondEmitDone := make(chan struct{})
	go func() {
		server.emit("new", &Email{})
		close(secondEmitDone)
	}()
	select {
	case <-unlimitedStarted:
	case <-time.After(time.Second):
		t.Fatal("unlimited listener was delayed by bounded listener")
	}
	select {
	case <-secondEmitDone:
		t.Fatal("bounded listener did not apply backpressure")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case <-secondEmitDone:
	case <-time.After(time.Second):
		t.Fatal("emit did not resume after a concurrency slot was released")
	}
}

func TestOnWithConcurrencyRejectsInvalidRegistration(t *testing.T) {
	server, err := NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	if err := server.OnWithConcurrency("new", -1, func(_ *Email) {}); err == nil {
		t.Fatal("negative concurrency should fail")
	}
	if err := server.OnWithConcurrency("new", 1, nil); err == nil {
		t.Fatal("nil handler should fail")
	}
}

func TestSynchronousFailureStopsUncommittedNotifications(t *testing.T) {
	server, err := NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	handoffErr := errors.New("injected durable handoff failure")
	if err := server.OnSynchronous("new", func(*Email) error { return handoffErr }); err != nil {
		t.Fatal(err)
	}
	notified := make(chan struct{}, 1)
	server.On("new", func(*Email) { notified <- struct{}{} })

	err = server.emit("new", &Email{ID: "uncommitted"})
	if !errors.Is(err, handoffErr) {
		t.Fatalf("emit error = %v, want %v", err, handoffErr)
	}
	select {
	case <-notified:
		t.Fatal("asynchronous listener observed an event whose durable handoff failed")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestOnSynchronousRejectsMultipleTransactionalHandlers(t *testing.T) {
	server, err := NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	if err := server.OnSynchronous("new", func(*Email) error { return nil }); err != nil {
		t.Fatal(err)
	}
	err = server.OnSynchronous("new", func(*Email) error {
		return errors.New("must never run")
	})
	if err == nil {
		t.Fatal("multiple transactional handlers would allow a partial durable handoff")
	}
}

package mailserver

import (
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

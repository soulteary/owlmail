package outgoing

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/types"
)

func TestOutgoingMailDynamicEnableStartsWorker(t *testing.T) {
	om := NewOutgoingMail(nil)
	defer om.Close()

	if err := om.UpdateConfig(&OutgoingConfig{Host: "smtp.example.test", Port: 25}); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	err := om.RelayMail(&types.Email{
		Envelope: &types.Envelope{To: []string{"to@example.test"}},
	}, "/missing", "", false, func(err error) { result <- err })
	if err != nil {
		t.Fatalf("RelayMail() error = %v", err)
	}
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("worker returned nil for a missing message file")
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not consume task after dynamic enable")
	}
}

func TestOutgoingMailConfigSnapshotsAndDisableSemantics(t *testing.T) {
	source := &OutgoingConfig{
		Host:       "old.example.test",
		Port:       25,
		Password:   "old-secret",
		AllowRules: []string{"*@example.test"},
	}
	copyingRelay := NewOutgoingMail(source)

	// Neither constructor/update inputs nor returned snapshots may alias the
	// relay's immutable internal configuration.
	source.Host = "mutated.example.test"
	source.AllowRules[0] = "mutated"
	got := copyingRelay.GetConfig().(*OutgoingConfig)
	got.Host = "escaped.example.test"
	got.AllowRules[0] = "escaped"
	again := copyingRelay.GetConfig().(*OutgoingConfig)
	if again.Host != "old.example.test" || again.AllowRules[0] != "*@example.test" {
		t.Fatalf("internal config was mutated through an external pointer: %#v", again)
	}
	copyingRelay.Close()

	om := &OutgoingMail{
		config:         cloneConfig(again),
		queue:          make(chan *RelayTask, 2),
		workerCount:    1,
		enqueueTimeout: time.Second,
		enabled:        true,
		workerStarted:  true,
		closing:        make(chan struct{}),
		stop:           make(chan struct{}),
	}
	defer om.Close()

	if err := om.RelayMail(&types.Email{}, "/missing", "to@example.test", false, nil); err != nil {
		t.Fatal(err)
	}
	if err := om.UpdateConfig(&OutgoingConfig{
		Host:       "new.example.test",
		Port:       587,
		Password:   "new-secret",
		AllowRules: []string{"*@new.example.test"},
	}); err != nil {
		t.Fatal(err)
	}
	queued := <-om.queue
	if queued.config.Host != "old.example.test" || queued.config.Password != "old-secret" {
		t.Fatalf("queued task config = %#v, want original atomic snapshot", queued.config)
	}

	if err := om.UpdateConfig(&OutgoingConfig{}); err != nil {
		t.Fatal(err)
	}
	callback := make(chan error, 1)
	err := om.RelayMail(&types.Email{}, "/missing", "to@example.test", false, func(err error) {
		callback <- err
	})
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("disabled RelayMail() error = %v, want %v", err, ErrNotConfigured)
	}
	if callbackErr := <-callback; !errors.Is(callbackErr, ErrNotConfigured) {
		t.Fatalf("disabled callback error = %v, want %v", callbackErr, ErrNotConfigured)
	}
}

func TestOutgoingMailRejectedUpdatePreservesConfig(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{Host: "smtp.example.test", Port: 25, Password: "secret"})
	defer om.Close()

	if err := om.UpdateConfig(&OutgoingConfig{Host: "invalid.example.test", Port: 0}); err == nil {
		t.Fatal("UpdateConfig() accepted an invalid enabled configuration")
	}
	got := om.GetConfig().(*OutgoingConfig)
	if got.Host != "smtp.example.test" || got.Port != 25 || got.Password != "secret" {
		t.Fatalf("failed update changed config: %#v", got)
	}
}

func TestOutgoingMailQueueFull(t *testing.T) {
	om := &OutgoingMail{
		config:         &OutgoingConfig{Host: "smtp.example.test", Port: 25},
		queue:          make(chan *RelayTask, 1),
		workerCount:    1,
		enqueueTimeout: 10 * time.Millisecond,
		enabled:        true,
		workerStarted:  true,
		closing:        make(chan struct{}),
		stop:           make(chan struct{}),
	}
	defer om.Close()

	if err := om.RelayMail(&types.Email{}, "/missing", "to@example.test", false, nil); err != nil {
		t.Fatal(err)
	}
	callback := make(chan error, 1)
	err := om.RelayMail(&types.Email{}, "/missing", "to@example.test", false, func(err error) {
		callback <- err
	})
	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf("RelayMail() error = %v, want %v", err, ErrQueueFull)
	}
	if callbackErr := <-callback; !errors.Is(callbackErr, ErrQueueFull) {
		t.Fatalf("callback error = %v, want %v", callbackErr, ErrQueueFull)
	}
}

func TestOutgoingMailConcurrentUpdateEnqueueAndClose(t *testing.T) {
	om := NewOutgoingMail(nil)
	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < 32; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			<-start
			config := &OutgoingConfig{}
			if i%3 != 0 {
				config = &OutgoingConfig{Host: "smtp.example.test", Port: 25, Password: "secret"}
			}
			_ = om.UpdateConfig(config)
		}(i)
		go func() {
			defer wg.Done()
			<-start
			_ = om.RelayMail(&types.Email{}, "/missing", "to@example.test", false, func(error) {})
		}()
	}
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		om.Close()
	}()
	go func() {
		defer wg.Done()
		<-start
		om.Close()
	}()

	close(start)
	wg.Wait()
	om.Close()

	if err := om.UpdateConfig(&OutgoingConfig{Host: "smtp.example.test", Port: 25}); !errors.Is(err, ErrClosed) {
		t.Fatalf("UpdateConfig() after Close error = %v, want %v", err, ErrClosed)
	}
	callback := make(chan error, 1)
	err := om.RelayMail(&types.Email{}, "/missing", "to@example.test", false, func(err error) {
		callback <- err
	})
	if !errors.Is(err, ErrClosed) {
		t.Fatalf("RelayMail() after Close error = %v, want %v", err, ErrClosed)
	}
	if callbackErr := <-callback; !errors.Is(callbackErr, ErrClosed) {
		t.Fatalf("callback after Close error = %v, want %v", callbackErr, ErrClosed)
	}
}

func TestRejectedRelayCallbackCanClose(t *testing.T) {
	om := NewOutgoingMail(&OutgoingConfig{Host: "smtp.example.test", Port: 25})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)

	go func() {
		done <- om.RelayMailContext(ctx, &types.Email{}, "/missing", "to@example.test", false, func(error) {
			om.Close()
		})
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RelayMailContext() error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("rejection callback deadlocked while closing relay")
	}
	om.Close()
}

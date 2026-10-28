package mailserver

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type shutdownTestCloser func() error

func (f shutdownTestCloser) Close() error { return f() }

func TestConcurrentCloseWaitsForOwnedResources(t *testing.T) {
	server, err := NewMailServer(1025, "127.0.0.1", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	var calls atomic.Int32
	sentinel := errors.New("owned resource close failed")
	if err := server.AddCloser(shutdownTestCloser(func() error {
		calls.Add(1)
		close(entered)
		<-release
		return sentinel
	})); err != nil {
		t.Fatal(err)
	}
	first := make(chan error, 1)
	second := make(chan error, 1)
	go func() { first <- server.Close() }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not reach owned resource")
	}
	go func() { second <- server.Close() }()
	select {
	case err := <-second:
		t.Fatalf("concurrent Close returned before cleanup: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	releaseOnce.Do(func() { close(release) })
	for _, result := range []<-chan error{first, second} {
		select {
		case err := <-result:
			if !errors.Is(err, sentinel) {
				t.Fatalf("shutdown error lost: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Close did not finish")
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("resource closed %d times", calls.Load())
	}
}

package mailserver

import (
	"os"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/attachmentstore"
)

func TestServiceReadinessTracksListenersStorageAndShutdown(t *testing.T) {
	dir := t.TempDir()
	server, err := NewMailServer(0, "127.0.0.1", dir)
	if err != nil {
		t.Fatal(err)
	}
	ready, health := server.GetServiceReadiness()
	if ready || health.Ready() {
		t.Fatal("unstarted server is ready")
	}
	started := make(chan struct{})
	result := make(chan error, 1)
	go func() { result <- server.ListenWithReady(func() { close(started) }) }()
	select {
	case <-started:
	case err := <-result:
		t.Fatal(err)
	case <-time.After(3 * time.Second):
		t.Fatal("startup timeout")
	}
	defer func() { _ = server.Close() }()
	ready, health = server.GetServiceReadiness()
	if !ready || !health.Ready() {
		t.Fatalf("started state: %v %+v", ready, health)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("storage probe left files behind")
	}
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	server.probeMailStorage()
	_, health = server.GetServiceReadiness()
	if health.Ready() || health.ErrorCategory != attachmentstore.HealthErrorNotFound {
		t.Fatalf("missing storage: %+v", health)
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	server.probeMailStorage()
	_, health = server.GetServiceReadiness()
	if !health.Ready() {
		t.Fatal("storage did not recover")
	}
	_ = server.Close()
	ready, health = server.GetServiceReadiness()
	if ready || health.State != attachmentstore.HealthClosed {
		t.Fatalf("closed state: %v %+v", ready, health)
	}
	select {
	case <-result:
	case <-time.After(3 * time.Second):
		t.Fatal("listener did not stop")
	}
}

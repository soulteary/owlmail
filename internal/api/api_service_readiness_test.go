package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/mailserver"
)

func TestReadinessRequiresSMTPAndStorageWhileLivenessStaysIndependent(t *testing.T) {
	server, err := mailserver.NewMailServer(0, "127.0.0.1", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	api := NewAPI(server, 1080, "127.0.0.1")
	check := func(path string, want int) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, path, nil)
		resp, err := api.app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != want {
			t.Fatalf("%s status %d want %d", path, resp.StatusCode, want)
		}
	}
	check("/healthz", 200)
	check("/readyz", 503)
	started := make(chan struct{})
	finished := make(chan error, 1)
	go func() { finished <- server.ListenWithReady(func() { close(started) }) }()
	select {
	case <-started:
	case err := <-finished:
		t.Fatal(err)
	case <-time.After(3 * time.Second):
		t.Fatal("startup timeout")
	}
	check("/readyz", 200)
	_ = server.Close()
	check("/readyz", 503)
	check("/healthz", 200)
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("listener shutdown timeout")
	}
}

package api

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/outgoing"
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
	deadline := time.Now().Add(3 * time.Second)
	for {
		_, storage := server.GetServiceReadiness()
		if storage.Ready() {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("storage readiness timeout")
		}
		time.Sleep(time.Millisecond)
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

func TestHTTPAndRuntimeReadinessAgreeOnOutgoingTransport(t *testing.T) {
	reserved, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := reserved.Addr().(*net.TCPAddr).Port
	_ = reserved.Close()
	directory := t.TempDir()
	server, err := mailserver.NewMailServerWithOptions(port, "127.0.0.1", directory, mailserver.ServerOptions{
		RuntimeReadiness: true,
		OutgoingConfig:   &outgoing.OutgoingConfig{Host: "127.0.0.1", Port: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	api := NewAPI(server, 1080, "127.0.0.1")
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
	wait := func(ready bool) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for {
			status := server.GetRuntimeReadiness()
			if status.Storage.Ready() && status.Transport != nil && status.Transport.State != "checking" && !ready {
				return
			}
			if ready && status.ValidateService(time.Now()) == nil {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("dependency readiness timeout: %+v", status)
			}
			time.Sleep(time.Millisecond)
		}
	}
	wait(false)
	liveness, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	liveResponse, err := api.app.Test(liveness)
	if err != nil {
		t.Fatal(err)
	}
	_ = liveResponse.Body.Close()
	if liveResponse.StatusCode != 200 {
		t.Fatal("outgoing outage affected liveness")
	}
	for _, path := range []string{"/readyz", "/api/v1/ready"} {
		req, _ := http.NewRequest(http.MethodGet, path, nil)
		resp, err := api.app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		var body readinessResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&body)
		_ = resp.Body.Close()
		if decodeErr != nil || resp.StatusCode != 503 || body.Checks["outgoing_transport"].Status != "unready" {
			t.Fatalf("%s: status %d body %+v error %v", path, resp.StatusCode, body, decodeErr)
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		data, err := os.ReadFile(mailserver.RuntimeReadinessPath(directory, "127.0.0.1", port))
		var snapshot mailserver.RuntimeReadiness
		if err == nil && json.Unmarshal(data, &snapshot) == nil && snapshot.Transport != nil && snapshot.Transport.State == "unready" {
			if snapshot.Validate("127.0.0.1", port, time.Now()) == nil {
				t.Fatal("runtime snapshot accepted HTTP-unready transport")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("runtime snapshot not published")
		}
		time.Sleep(time.Millisecond)
	}
	// Removing an optional downstream transport recovers both modes without restart.
	if err := server.SetOutgoingConfig(&outgoing.OutgoingConfig{}); err != nil {
		t.Fatal(err)
	}
	wait(true)
	req, _ := http.NewRequest(http.MethodGet, "/readyz", nil)
	resp, err := api.app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 || server.GetRuntimeReadiness().Validate("127.0.0.1", port, time.Now()) != nil {
		t.Fatal("modes disagreed after transport removal")
	}
	req, _ = http.NewRequest(http.MethodGet, "/healthz", nil)
	resp, err = api.app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatal("outgoing outage affected liveness")
	}
	_ = server.Close()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown timeout")
	}
}

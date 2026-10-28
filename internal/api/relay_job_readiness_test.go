package api

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	smtp "github.com/emersion/go-smtp"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/outgoing"
	"github.com/soulteary/owlmail/internal/relay"
	"github.com/soulteary/owlmail/internal/types"
)

func TestDurableRelayHTTPReadinessFailsAndRecoversWhileLivenessStaysHealthy(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	backend := &retryFaultBackend{}
	backend.attempts.Store(2) // This receiver accepts the first actual DATA transaction.
	receiver := smtp.NewServer(backend)
	receiver.Domain = "localhost"
	receiverDone := make(chan error, 1)
	go func() { receiverDone <- receiver.Serve(listener) }()
	defer func() { _ = receiver.Close(); <-receiverDone }()
	server, err := mailserver.NewMailServerWithOptions(0, "127.0.0.1", t.TempDir(), mailserver.ServerOptions{
		AllowEphemeralPort: true, RuntimeReadiness: true,
		OutgoingConfig: &outgoing.OutgoingConfig{Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, TLSMode: outgoing.TLSModePlain},
	})
	if err != nil {
		t.Fatal(err)
	}
	api := NewAPIWithHTTPSDeferredRecovery(server, 0, "127.0.0.1", "", "", false, "", "")
	var writes atomic.Int32
	var failCompletion atomic.Bool
	service := installWorkflowRelay(t, api, relay.Options{SyncDirectory: func(string) error {
		if writes.Add(1) >= 3 && failCompletion.Load() {
			return errors.New("durable completion failed")
		}
		return nil
	}})
	service.Start()
	startUnifiedSMTP(t, server)
	service.CheckPersistence()
	wait := func(condition func() bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for !condition() {
			if time.Now().After(deadline) {
				t.Fatalf("readiness transition timed out: %+v", server.GetRuntimeReadiness())
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	wait(func() bool { return server.GetRuntimeReadiness().ValidateService(time.Now()) == nil })
	check := func(path string, want int) {
		t.Helper()
		response, err := api.app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = response.Body.Close() }()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want {
			t.Fatalf("%s = %d want %d: %s", path, response.StatusCode, want, body)
		}
		if path == "/readyz" && !strings.Contains(string(body), "relay_job_storage") {
			t.Fatal("durable readiness diagnostics missing")
		}
	}
	check("/readyz", http.StatusOK)
	const id = "ready-completion"
	raw := []byte("From: sender@example.test\r\nTo: recipient@example.test\r\nSubject: durable completion\r\n\r\nbody")
	if err := os.WriteFile(filepath.Join(server.GetMailDir(), id+".eml"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := server.SaveEmailToStore(id, false, &types.Envelope{From: "sender@example.test", To: []string{"recipient@example.test"}}, &types.Email{Subject: "durable completion", Text: "body"}); err != nil {
		t.Fatal(err)
	}
	failCompletion.Store(true)
	if _, err := service.Submit(relay.Request{EmailID: id, Origin: "native"}); err != nil {
		t.Fatal(err)
	}
	wait(func() bool { return service.PersistenceError() != nil })
	check("/readyz", http.StatusServiceUnavailable)
	check("/healthz", http.StatusOK)
	failCompletion.Store(false)
	service.CheckPersistence()
	wait(func() bool { return service.Snapshot().Ready() })
	check("/readyz", http.StatusOK)
}

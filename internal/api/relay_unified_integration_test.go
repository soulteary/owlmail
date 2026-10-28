package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/outgoing"
)

func startUnifiedSMTP(t *testing.T, server *mailserver.MailServer) func() {
	t.Helper()
	ready := make(chan struct{})
	result := make(chan error, 1)
	go func() { result <- server.ListenWithReady(func() { close(ready) }) }()
	select {
	case <-ready:
	case err := <-result:
		t.Fatalf("SMTP startup: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("SMTP startup timed out")
	}
	stopped := false
	stop := func() {
		t.Helper()
		if stopped {
			return
		}
		stopped = true
		if err := server.Close(); err != nil {
			t.Errorf("SMTP shutdown: %v", err)
		}
		select {
		case <-result:
		case <-time.After(5 * time.Second):
			t.Error("SMTP shutdown timed out")
		}
	}
	t.Cleanup(stop)
	return stop
}

func unifiedRelayHTTP(t *testing.T, api *API, method, path, body string) (int, http.Header, []byte) {
	t.Helper()
	request, err := http.NewRequest(method, path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := api.app.Test(request, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, response.Header, data
}

// Exercise the actual SMTP source and destination, not a mocked transport, so
// each HTTP facade and automatic capture must reach the same durable history.
func TestUnifiedRelayEntrancesAndRecovery(t *testing.T) {
	for _, base := range []string{"", "/mail"} {
		for _, origin := range []string{"native", "legacy", "maildev", "auto"} {
			t.Run(base+"/"+origin, func(t *testing.T) {
				destination, err := mailserver.NewMailServerWithOptions(0, "127.0.0.1", t.TempDir(), mailserver.ServerOptions{AllowEphemeralPort: true})
				if err != nil {
					t.Fatal(err)
				}
				startUnifiedSMTP(t, destination)
				dir := t.TempDir()
				config := &outgoing.OutgoingConfig{Host: "127.0.0.1", Port: destination.GetPort(), TLSMode: outgoing.TLSModePlain, AutoRelay: origin == "auto"}
				newSource := func() (*mailserver.MailServer, *API) {
					t.Helper()
					server, err := mailserver.NewMailServerWithOptions(0, "127.0.0.1", dir, mailserver.ServerOptions{AllowEphemeralPort: true, OutgoingConfig: config})
					if err != nil {
						t.Fatal(err)
					}
					api := NewAPI(server, 0, "127.0.0.1")
					api.SetMailDevRESTCompat(true)
					if err := api.SetBasePathname(base); err != nil {
						t.Fatal(err)
					}
					return server, api
				}
				source, api := newSource()
				stop := startUnifiedSMTP(t, source)
				raw := "From: sender@example.test\r\nTo: visible@example.test\r\nSubject: unified " + origin + "\r\n\r\nrelay body\r\n"
				address := net.JoinHostPort("127.0.0.1", fmt.Sprint(source.GetPort()))
				if err := smtp.SendMail(address, nil, "sender@example.test", []string{"envelope@example.test"}, []byte(raw)); err != nil {
					t.Fatal(err)
				}
				emails := source.GetAllEmail()
				if len(emails) != 1 {
					t.Fatalf("source captured %d messages", len(emails))
				}
				id := emails[0].ID
				if origin != "auto" {
					path := base + "/api/v1/emails/" + id + "/actions/relay"
					switch origin {
					case "legacy":
						path = base + "/email/" + id + "/relay"
					case "maildev":
						path = base + "/api/email/" + id + "/relay"
					}
					status, headers, body := unifiedRelayHTTP(t, api, http.MethodPost, path, "")
					if origin == "native" {
						if status != 202 || !strings.HasPrefix(headers.Get("Location"), base+"/api/v1/relay-jobs/") {
							t.Fatalf("native acceptance: %d %s %s", status, headers.Get("Location"), body)
						}
					} else if status != 200 || (origin == "maildev" && string(bytes.TrimSpace(body)) != "true") {
						t.Fatalf("%s compatibility response: %d %s", origin, status, body)
					}
				}
				var jobID string
				deadline := time.Now().Add(8 * time.Second)
				for {
					status, _, body := unifiedRelayHTTP(t, api, http.MethodGet, base+"/api/v1/relay-jobs?email_id="+id, "")
					var history struct {
						Jobs []struct {
							ID, Origin, Status string
						} `json:"jobs"`
					}
					if err := json.Unmarshal(body, &history); err != nil || status != 200 {
						t.Fatalf("history: %d %s %v", status, body, err)
					}
					receipt, _ := source.GetEmail(id)
					if len(history.Jobs) == 1 && history.Jobs[0].Status == "succeeded" && receipt != nil && receipt.RelayedAt != nil {
						if history.Jobs[0].Origin != origin {
							t.Fatalf("origin: %s", body)
						}
						jobID = history.Jobs[0].ID
						break
					}
					if time.Now().After(deadline) {
						t.Fatalf("no single successful durable job: %s", body)
					}
					time.Sleep(10 * time.Millisecond)
				}
				if received := destination.GetAllEmail(); len(received) != 1 || len(received[0].Envelope.To) != 1 || received[0].Envelope.To[0] != "envelope@example.test" {
					t.Fatalf("wrong downstream capture: %#v", received)
				}
				checkReceipt := func(api *API) {
					t.Helper()
					for _, path := range []string{base + "/api/v1/emails/" + id, base + "/api/email/" + id} {
						status, _, body := unifiedRelayHTTP(t, api, http.MethodGet, path, "")
						var receipt struct {
							RelayedAt *time.Time `json:"relayedAt"`
							RelayedTo []string   `json:"relayedTo"`
						}
						if err := json.Unmarshal(body, &receipt); err != nil || status != 200 || receipt.RelayedAt == nil || receipt.RelayedAt.IsZero() || len(receipt.RelayedTo) != 1 || receipt.RelayedTo[0] != "envelope@example.test" {
							t.Fatalf("receipt: %d %s %v", status, body, err)
						}
					}
				}
				checkReceipt(api)
				stop()
				restored, restoredAPI := newSource()
				stopRestored := startUnifiedSMTP(t, restored)
				checkReceipt(restoredAPI)
				status, _, body := unifiedRelayHTTP(t, restoredAPI, http.MethodGet, base+"/api/v1/relay-jobs/"+jobID, "")
				if status != 200 || !bytes.Contains(body, []byte(`"succeeded"`)) {
					t.Fatalf("restored job: %d %s", status, body)
				}
				// Repeated explicit reload is also a recovery boundary; it must not
				// manufacture another automatic task for an already accepted email.
				if err := restored.LoadMailsFromDirectory(); err != nil {
					t.Fatal(err)
				}
				time.Sleep(100 * time.Millisecond)
				if count := len(destination.GetAllEmail()); count != 1 {
					t.Fatalf("recovery unexpectedly resent the message: %d", count)
				}
				if origin == "auto" {
					stopRestored()
					path := filepath.Join(dir, ".owlmail-meta", "relay-jobs", jobID+".json")
					data, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					var record map[string]any
					if err := json.Unmarshal(data, &record); err != nil {
						t.Fatal(err)
					}
					past := time.Now().Add(-48 * time.Hour).UTC()
					record["createdAt"], record["updatedAt"], record["completedAt"] = past.Add(-time.Minute), past, past
					data, err = json.Marshal(record)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, data, 0600); err != nil {
						t.Fatal(err)
					}
					aged, agedAPI := newSource()
					startUnifiedSMTP(t, aged)
					checkReceipt(agedAPI)
					status, _, body := unifiedRelayHTTP(t, agedAPI, http.MethodGet, base+"/api/v1/relay-jobs/"+jobID, "")
					if status != 404 {
						t.Fatalf("expired job remains in bounded history: %d %s", status, body)
					}
					if err := aged.LoadMailsFromDirectory(); err != nil {
						t.Fatal(err)
					}
					time.Sleep(100 * time.Millisecond)
					if count := len(destination.GetAllEmail()); count != 1 {
						t.Fatalf("expired history caused automatic resend: %d", count)
					}
				}
			})
		}
	}
}

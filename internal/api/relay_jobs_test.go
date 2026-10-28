package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/outgoing"
	"github.com/soulteary/owlmail/internal/relay"
	"github.com/soulteary/owlmail/internal/types"
)

func TestNativeRelayRejectsOversizedRecipientBeforeRetaining(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("close mail server: %v", err)
		}
	}()
	email := &types.Email{ID: "oversized-relay", Subject: "Oversized relay", Time: time.Now()}
	envelope := &types.Envelope{From: "sender@example.test", To: []string{"recipient@example.test"}}
	if err := server.SaveEmailToStore(email.ID, false, envelope, email); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]string{"relayTo": strings.Repeat("x", 1024+1)})
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/emails/"+email.ID+"/actions/relay", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("oversized recipient status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if jobs, _ := api.relayService.List("", "", 100); len(jobs) != 0 {
		t.Fatalf("oversized recipient created relay jobs: %d", len(jobs))
	}
}

func TestNativeRelayRejectsInvalidRequestBody(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() { _ = server.Close() }()

	for name, payload := range map[string]string{
		"malformed JSON":          `{"confirmedRecipients":[`,
		"invalid recipient shape": `{"confirmedRecipients":"recipient@example.test"}`,
	} {
		t.Run(name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/emails/missing/actions/relay", strings.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), ErrorCodeInvalidRequest) {
				t.Fatalf("invalid relay body status = %d, body = %s", resp.StatusCode, body)
			}
			if jobs, _ := api.relayService.List("", "", 100); len(jobs) != 0 {
				t.Fatalf("invalid relay body created %d job(s)", len(jobs))
			}
		})
	}
}

func TestNativeRelayReturnsQueryableJob(t *testing.T) {
	directory := t.TempDir()
	server, err := mailserver.NewMailServerWithOutgoing(1025, "localhost", directory, &outgoing.OutgoingConfig{
		Host: "127.0.0.1", Port: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("close mail server: %v", err)
		}
	}()
	email := &types.Email{ID: "relay-status-mail", Subject: "Relay status", Time: time.Now()}
	envelope := &types.Envelope{From: "sender@example.test", To: []string{"recipient@example.test"}}
	if err := os.WriteFile(filepath.Join(directory, email.ID+".eml"), []byte("From: sender@example.test\r\nTo: recipient@example.test\r\n\r\nbody"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := server.SaveEmailToStore(email.ID, false, envelope, email); err != nil {
		t.Fatal(err)
	}
	api := NewAPI(server, 1080, "localhost")

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/emails/"+email.ID+"/actions/relay", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("relay status = %d, body = %s", resp.StatusCode, body)
	}
	var accepted struct {
		Data struct {
			Job struct {
				ID string `json:"id"`
			} `json:"job"`
			StatusURL string `json:"statusUrl"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &accepted); err != nil {
		t.Fatal(err)
	}
	if accepted.Data.Job.ID == "" || accepted.Data.StatusURL == "" || resp.Header.Get("Location") != accepted.Data.StatusURL {
		t.Fatalf("invalid accepted relay response: %s", body)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		req, _ = http.NewRequest(http.MethodGet, accepted.Data.StatusURL, nil)
		resp, err = api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
		if err != nil {
			t.Fatal(err)
		}
		body, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if strings.Contains(string(body), `"status":"failed"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("relay job did not reach a terminal state: %s", body)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if strings.Contains(string(body), "127.0.0.1") {
		t.Fatalf("status endpoint leaked downstream details: %s", body)
	}
}

func TestRelayPreflightBindsConfirmedRecipientsToEnqueueSnapshot(t *testing.T) {
	directory := t.TempDir()
	server, err := mailserver.NewMailServerWithOutgoing(1025, "localhost", directory, &outgoing.OutgoingConfig{
		Host: "127.0.0.1", Port: 1, AllowRules: []string{"allowed@example.test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	email := &types.Email{ID: "confirmed-relay", Subject: "Confirmed relay", Time: time.Now()}
	envelope := &types.Envelope{From: "sender@example.test", To: []string{"allowed@example.test", "blocked@example.test"}}
	if err := os.WriteFile(filepath.Join(directory, email.ID+".eml"), []byte("Subject: Confirmed relay\r\n\r\nbody"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := server.SaveEmailToStore(email.ID, false, envelope, email); err != nil {
		t.Fatal(err)
	}
	api := NewAPI(server, 1080, "localhost")

	preflightRequest, _ := http.NewRequest(http.MethodGet, "/api/v1/emails/"+email.ID+"/actions/relay/preflight", nil)
	preflightResponse, err := api.app.Test(preflightRequest, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	preflightBody, _ := io.ReadAll(preflightResponse.Body)
	_ = preflightResponse.Body.Close()
	if preflightResponse.StatusCode != http.StatusOK || !strings.Contains(string(preflightBody), `"recipients":["allowed@example.test"]`) {
		t.Fatalf("preflight status = %d, body = %s", preflightResponse.StatusCode, preflightBody)
	}

	if err := server.SetOutgoingConfig(&outgoing.OutgoingConfig{Host: "127.0.0.1", Port: 1, AllowRules: []string{"*"}}); err != nil {
		t.Fatal(err)
	}
	payload := strings.NewReader(`{"confirmedRecipients":["allowed@example.test"]}`)
	relayRequest, _ := http.NewRequest(http.MethodPost, "/api/v1/emails/"+email.ID+"/actions/relay", payload)
	relayRequest.Header.Set("Content-Type", "application/json")
	relayResponse, err := api.app.Test(relayRequest, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	relayBody, _ := io.ReadAll(relayResponse.Body)
	_ = relayResponse.Body.Close()
	if relayResponse.StatusCode != http.StatusBadRequest || !strings.Contains(string(relayBody), "configuration changed") {
		t.Fatalf("changed-config relay status = %d, body = %s", relayResponse.StatusCode, relayBody)
	}
	if jobs, _ := api.relayService.List("", "", 100); len(jobs) != 0 {
		t.Fatalf("changed-config relay retained %d job(s)", len(jobs))
	}
}

func TestRelayJobNotFound(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("close mail server: %v", err)
		}
	}()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/relay-jobs/missing", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing relay job status = %d", resp.StatusCode)
	}
}

func TestNativeRelayReturnsServiceUnavailableAtStatusCapacity(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("close mail server: %v", err)
		}
	}()
	email := &types.Email{ID: "relay-capacity-mail", Subject: "Relay capacity", Time: time.Now()}
	envelope := &types.Envelope{From: "sender@example.test", To: []string{"recipient@example.test"}}
	if err := server.SaveEmailToStore(email.ID, false, envelope, email); err != nil {
		t.Fatal(err)
	}

	if err := server.SetOutgoingConfig(&outgoing.OutgoingConfig{Host: "127.0.0.1", Port: 1}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	installWorkflowRelay(t, api, relay.Options{Limit: 1}, relay.Job{ID: strings.Repeat("a", 32), EmailID: "other-mail", Status: relay.Queued, CreatedAt: now, UpdatedAt: now})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/emails/"+email.ID+"/actions/relay", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("relay capacity status = %d, body = %s", resp.StatusCode, body)
	}
	if resp.Header.Get("Retry-After") != "1" {
		t.Fatalf("Retry-After = %q", resp.Header.Get("Retry-After"))
	}
	if !strings.Contains(string(body), "Relay status capacity reached") {
		t.Fatalf("relay capacity response = %s", body)
	}
}

func TestDeleteEndpointsRejectMessagesWithPendingRelayJobs(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() { _ = server.Close() }()
	email := &types.Email{ID: "relay-delete-mail", Subject: "Protected source", Time: time.Now()}
	if err := server.SaveEmailToStore(email.ID, false, &types.Envelope{To: []string{"recipient@example.test"}}, email); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	installWorkflowRelay(t, api, relay.Options{}, relay.Job{ID: strings.Repeat("b", 32), EmailID: email.ID, Status: relay.Queued, CreatedAt: now, UpdatedAt: now})

	for _, path := range []string{"/api/v1/emails/" + email.ID, "/api/v1/emails"} {
		req, _ := http.NewRequest(http.MethodDelete, path, nil)
		resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("DELETE %s returned %d, want 409", path, resp.StatusCode)
		}
	}
	if _, err := server.GetEmail(email.ID); err != nil {
		t.Fatalf("pending relay source was deleted: %v", err)
	}
}

func TestNativeRelayReturnsServiceUnavailableForRuntimePersistenceFailure(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() { _ = server.Close() }()
	email := &types.Email{ID: "relay-persistence-mail", Subject: "Relay persistence", Time: time.Now()}
	envelope := &types.Envelope{From: "sender@example.test", To: []string{"recipient@example.test"}}
	if err := server.SaveEmailToStore(email.ID, false, envelope, email); err != nil {
		t.Fatal(err)
	}

	if err := server.SetOutgoingConfig(&outgoing.OutgoingConfig{Host: "127.0.0.1", Port: 1}); err != nil {
		t.Fatal(err)
	}
	syncCalls := 0
	installWorkflowRelay(t, api, relay.Options{SyncDirectory: func(path string) error {
		syncCalls++
		if syncCalls == 1 {
			return errors.New("injected runtime sync failure")
		}
		return syncRelayFixtureDirectory(path)
	}})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/emails/"+email.ID+"/actions/relay", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable || resp.Header.Get("Retry-After") != "1" {
		t.Fatalf("runtime persistence status = %d, Retry-After = %q, body = %s", resp.StatusCode, resp.Header.Get("Retry-After"), body)
	}
	if jobs, _ := api.relayService.List("", "", 100); len(jobs) != 0 {
		t.Fatal("rejected persistence failure left a latent relay job")
	}
}

func TestNativeRelayRetainsJobWhenSynchronousRejectionCannotBeDurablyRemoved(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() { _ = server.Close() }()
	email := &types.Email{ID: "relay-retained-rejection", Subject: "Retained rejection", Time: time.Now()}
	if err := server.SaveEmailToStore(email.ID, false, &types.Envelope{To: []string{"recipient@example.test"}}, email); err != nil {
		t.Fatal(err)
	}

	if err := server.SetOutgoingConfig(&outgoing.OutgoingConfig{Host: "127.0.0.1", Port: 1}); err != nil {
		t.Fatal(err)
	}
	installWorkflowRelay(t, api, relay.Options{SyncDirectory: func(string) error {
		return errors.New("injected acceptance and rejection sync failure")
	}})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/emails/"+email.ID+"/actions/relay", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted || !strings.Contains(string(body), `"status":"queued"`) {
		t.Fatalf("retained rejection status = %d, body = %s", resp.StatusCode, body)
	}
	if jobs, _ := api.relayService.List("", "", 100); len(jobs) != 1 {
		t.Fatalf("retained jobs = %d, want 1", len(jobs))
	}
}

func TestNativeRelayRejectsSynchronousDisabledCallback(t *testing.T) {
	directory := t.TempDir()
	server, err := mailserver.NewMailServerWithOutgoing(1025, "localhost", directory, &outgoing.OutgoingConfig{Host: "smtp.example.test", Port: 25})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	email := &types.Email{ID: "disabled-relay", Subject: "Disabled relay", Time: time.Now()}
	if err := os.WriteFile(filepath.Join(directory, email.ID+".eml"), []byte("Subject: Disabled relay\r\n\r\nbody"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := server.SaveEmailToStore(email.ID, false, &types.Envelope{To: []string{"recipient@example.test"}}, email); err != nil {
		t.Fatal(err)
	}
	if err := server.SetOutgoingConfig(&outgoing.OutgoingConfig{}); err != nil {
		t.Fatal(err)
	}
	api := NewAPI(server, 1080, "localhost")

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/emails/"+email.ID+"/actions/relay", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("disabled relay status = %d, body = %s", resp.StatusCode, body)
	}
	if jobs, _ := api.relayService.List("", "", 100); len(jobs) != 0 {
		t.Fatalf("disabled relay retained jobs: %d", len(jobs))
	}
}

func syncRelayFixtureDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = directory.Close() }()
	return directory.Sync()
}

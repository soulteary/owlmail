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
	"github.com/soulteary/owlmail/internal/types"
)

func TestRelayJobStoreTracksSafeTerminalStatus(t *testing.T) {
	store := newRelayJobStore()
	store.newID = func() (string, error) { return "job-1", nil }
	job, err := store.create("mail-1", "relay@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != relayJobQueued {
		t.Fatalf("new job status = %q", job.Status)
	}
	store.complete(job.ID, errors.New("dial tcp 127.0.0.1:25: connection refused; password=secret"))
	got, ok := store.get(job.ID)
	if !ok || got.Status != relayJobFailed || got.ErrorCategory != "connection" || got.CompletedAt == nil {
		t.Fatalf("completed job = %#v, %t", got, ok)
	}
	encoded, _ := json.Marshal(got)
	if strings.Contains(string(encoded), "password") || strings.Contains(string(encoded), "127.0.0.1") {
		t.Fatalf("relay status leaked raw error details: %s", encoded)
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


func TestRelayJobStoreRejectsCapacityWithoutEvictingActiveJob(t *testing.T) {
	store := newRelayJobStore()
	store.limit = 1
	ids := []string{"job-active", "job-rejected", "job-replacement"}
	store.newID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}

	active, err := store.create("mail-active", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.create("mail-rejected", ""); !errors.Is(err, errRelayJobCapacity) {
		t.Fatalf("create at active capacity error = %v, want %v", err, errRelayJobCapacity)
	}
	if got, ok := store.get(active.ID); !ok || got.Status != relayJobQueued {
		t.Fatalf("active job was evicted: %#v, %t", got, ok)
	}

	store.complete(active.ID, nil)
	replacement, err := store.create("mail-replacement", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.get(active.ID); ok {
		t.Fatal("completed job was not evicted to make room")
	}
	if got, ok := store.get(replacement.ID); !ok || got.EmailID != "mail-replacement" {
		t.Fatalf("replacement job = %#v, %t", got, ok)
	}
}

package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/types"
	"github.com/soulteary/owlmail/internal/webhook"
)

func TestWorkflowListsAndReplayAdmission(t *testing.T) {
	api, s, _ := setupTestAPI(t)
	defer func() { _ = s.Close() }()
	job, err := api.relayJobs.create("gone", "recipient@example.test")
	if err != nil {
		t.Fatal(err)
	}
	api.relayJobs.complete(job.ID, errors.New("failure"))
	resp, err := api.app.Test(httptest.NewRequest("GET", "/api/v1/relay-jobs?status=failed", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var result struct {
		Jobs []relayJob `json:"jobs"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result.Jobs) != 1 {
		t.Fatal(result, err)
	}
	for _, tc := range []struct {
		method, path string
		status       int
	}{{"GET", "/api/v1/relay-jobs?limit=0", 400}, {"POST", "/api/v1/relay-jobs/" + job.ID + "/retry", 404}, {"POST", "/api/v1/webhook-deliveries/missing/retry", 503}, {"GET", "/api/v1/webhook-deliveries", 200}} {
		resp, err = api.app.Test(httptest.NewRequest(tc.method, tc.path, nil))
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != tc.status {
			t.Fatal(tc, resp.StatusCode)
		}
	}
}

func TestRelayPrunePersistenceFailureIsVisible(t *testing.T) {
	api, s, _ := setupTestAPI(t)
	defer func() { _ = s.Close() }()
	job, err := api.relayJobs.create("source", "recipient@example.test")
	if err != nil {
		t.Fatal(err)
	}
	api.relayJobs.complete(job.ID, nil)
	api.relayJobs.mu.Lock()
	old := api.relayJobs.jobs[job.ID]
	expired := time.Now().Add(-48 * time.Hour)
	old.CompletedAt = &expired
	api.relayJobs.jobs[job.ID] = old
	api.relayJobs.syncDirectory = func(string) error { return errors.New("injected deletion sync error") }
	api.relayJobs.mu.Unlock()
	resp, err := api.app.Test(httptest.NewRequest("GET", "/api/v1/relay-jobs", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var result struct {
		Healthy bool `json:"persistence_healthy"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil || result.Healthy {
		t.Fatal(result, err)
	}
	api.relayJobs.mu.Lock()
	api.relayJobs.syncDirectory = syncRelayJobDirectory
	api.relayJobs.mu.Unlock()
	recovered, err := api.app.Test(httptest.NewRequest("GET", "/api/v1/relay-jobs", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = recovered.Body.Close() }()
	if err = json.NewDecoder(recovered.Body).Decode(&result); err != nil || !result.Healthy {
		t.Fatal("successful deletion retry did not restore health", result, err)
	}

}

func TestWebhookReplayInfrastructureFailureReturnsUnavailable(t *testing.T) {
	api, server, directory := setupTestAPI(t)
	defer func() { _ = server.Close() }()
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(400) }))
	defer receiver.Close()
	dispatcher, err := webhook.NewDispatcher(webhook.Config{Targets: []webhook.Target{{Name: "failed", URL: receiver.URL}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	completed := make(chan string, 1)
	service, err := webhook.NewService(dispatcher, webhook.ServiceOptions{OnResults: func(id string, _ []webhook.Result) { completed <- id }})
	if err != nil {
		t.Fatal(err)
	}
	email := &types.Email{ID: "replay-source", Subject: "test", Text: "body"}
	if err = os.WriteFile(filepath.Join(directory, email.ID+".eml"), []byte("Subject: test\r\n\r\nbody"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = server.SaveEmailToStore(email.ID, false, &types.Envelope{To: []string{"recipient@example.test"}}, email); err != nil {
		t.Fatal(err)
	}
	if err = service.Enqueue(email); err != nil {
		t.Fatal(err)
	}
	var id string
	select {
	case id = <-completed:
	case <-time.After(3 * time.Second):
		t.Fatal("delivery timeout")
	}
	if err = service.Close(); err != nil {
		t.Fatal(err)
	}
	api.SetWebhookService(service)
	response, err := api.app.Test(httptest.NewRequest("POST", "/api/v1/webhook-deliveries/"+id+"/retry", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != 503 {
		t.Fatal(response.StatusCode)
	}
}

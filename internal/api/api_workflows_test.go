package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/relay"
	"github.com/soulteary/owlmail/internal/types"
	"github.com/soulteary/owlmail/internal/webhook"
)

func TestWorkflowListsAndReplayAdmission(t *testing.T) {
	api, s, _ := setupTestAPI(t)
	defer func() { _ = s.Close() }()
	now := time.Now().UTC()
	job := relay.Job{ID: strings.Repeat("a", 32), EmailID: "gone", RelayTo: "recipient@example.test", Status: relay.Failed, CreatedAt: now, UpdatedAt: now, CompletedAt: &now}
	installWorkflowRelay(t, api, relay.Options{}, job)
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
	}{{"GET", "/api/v1/relay-jobs?limit=0", 400}, {"POST", "/api/v1/relay-jobs/" + job.ID + "/retry", 404}, {"POST", "/api/v1/webhook-deliveries/missing/retry", 503}, {"GET", "/api/v1/webhook-deliveries", 200}, {"GET", "/api/v1/webhook-deliveries?status=typo", 400}, {"GET", "/api/v1/webhook-deliveries?status=failed", 200}} {
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
	api, server, _ := setupTestAPI(t)
	defer func() { _ = server.Close() }()
	past := time.Now().Add(-48 * time.Hour).UTC()
	var clock atomic.Int64
	clock.Store(past.UnixNano())
	var failSync atomic.Bool
	job := relay.Job{ID: strings.Repeat("b", 32), EmailID: "source", Status: relay.Succeeded, CreatedAt: past, UpdatedAt: past, CompletedAt: &past}
	installWorkflowRelay(t, api, relay.Options{
		Now: func() time.Time { return time.Unix(0, clock.Load()) },
		SyncDirectory: func(string) error {
			if failSync.Load() {
				return errors.New("injected deletion sync error")
			}
			return nil
		},
	}, job)
	failSync.Store(true)
	clock.Store(time.Now().UnixNano())
	check := func(wantHealthy bool) {
		t.Helper()
		response, err := api.app.Test(httptest.NewRequest("GET", "/api/v1/relay-jobs", nil))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = response.Body.Close() }()
		var result struct {
			Healthy bool `json:"persistence_healthy"`
		}
		if err := json.NewDecoder(response.Body).Decode(&result); err != nil || result.Healthy != wantHealthy {
			t.Fatalf("health=%+v want=%v err=%v", result, wantHealthy, err)
		}
	}
	check(false)
	failSync.Store(false)
	check(true)
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

func TestRelayReplayUnavailableTargetReturnsConflict(t *testing.T) {
	api, server, directory := setupTestAPI(t)
	defer func() { _ = server.Close() }()
	email := &types.Email{ID: "unavailable-replay-target", Subject: "test", Text: "body"}
	if err := os.WriteFile(filepath.Join(directory, email.ID+".eml"), []byte("Subject: test\r\n\r\nbody"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := server.SaveEmailToStore(email.ID, false, &types.Envelope{To: []string{"recipient@example.test"}}, email); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	job := relay.Job{ID: strings.Repeat("c", 32), EmailID: email.ID, RelayTo: "recipient@example.test", Status: relay.Failed, CreatedAt: now, UpdatedAt: now, CompletedAt: &now}
	installWorkflowRelay(t, api, relay.Options{}, job)
	response, err := api.app.Test(httptest.NewRequest("POST", "/api/v1/relay-jobs/"+job.ID+"/retry", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != 409 {
		t.Fatal(response.StatusCode)
	}
}

package api

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"
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
}

package api

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
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

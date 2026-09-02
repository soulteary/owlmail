package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestEmailQueryRejectsInvalidValues(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() { _ = server.Close() }()

	paths := []string{
		"/api/v1/emails?dateFrom=yesterday",
		"/api/v1/emails?dateTo=2026-99-99",
		"/api/v1/emails?read=maybe",
		"/api/v1/emails?sortBy=unknown",
		"/api/v1/emails/preview?sortOrder=sideways",
	}
	for _, path := range paths {
		req, _ := http.NewRequest(http.MethodGet, path, nil)
		resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("GET %s returned %d, want 400", path, resp.StatusCode)
		}
	}
}

func TestBatchMutationsRejectTooManyIDs(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() { _ = server.Close() }()

	ids := make([]string, maxBatchEmailIDs+1)
	for index := range ids {
		ids[index] = "mail-id"
	}
	body, err := json.Marshal(map[string]interface{}{"ids": ids})
	if err != nil {
		t.Fatal(err)
	}
	for _, methodAndPath := range []struct{ method, path string }{
		{http.MethodDelete, "/api/v1/emails/batch"},
		{http.MethodPatch, "/api/v1/emails/batch/read"},
	} {
		req, _ := http.NewRequest(methodAndPath.method, methodAndPath.path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Errorf("%s %s returned %d, want 413", methodAndPath.method, methodAndPath.path, resp.StatusCode)
		}
	}
}

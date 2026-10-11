package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/mailserver"
)

func TestDeliveryWaitRESTAndRunCleanup(t *testing.T) {
	api, s, _ := setupTestAPI(t)
	defer func() { _ = s.Close() }()
	cursor := s.DeliveryCursor()
	for _, run := range []string{"run-a", "run-b"} {
		if err := s.SaveEmailToStore(run, false, &mailserver.Envelope{To: []string{"a@example.test"}}, &mailserver.Email{RunID: run, Subject: "Test"}); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest("POST", "/api/v1/emails/wait", strings.NewReader(`{"after":"`+cursor+`","run_id":"run-a","recipient":"a@example.test"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var result mailserver.DeliveryResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || len(result.Emails) != 1 || result.Emails[0].RunID != "run-a" {
		t.Fatalf("%d %+v", resp.StatusCode, result)
	}
	resp, err = api.app.Test(httptest.NewRequest("GET", "/api/v1/emails?run_id=run-a", nil))
	if err != nil {
		t.Fatal(err)
	}
	var page struct{ Total int }
	_ = json.NewDecoder(resp.Body).Decode(&page)
	_ = resp.Body.Close()
	if page.Total != 1 {
		t.Fatal(page)
	}
	resp, err = api.app.Test(httptest.NewRequest("DELETE", "/api/v1/test-runs/run-a/emails", nil))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatal(resp.StatusCode)
	}
	if _, err := s.GetEmail("run-b"); err != nil {
		t.Fatal("deleted another run")
	}
	for _, body := range []string{`{}`, `{"after":"invalid"}`, `{"after":"` + cursor + `","timeout_seconds":121}`} {
		req := httptest.NewRequest("POST", "/api/v1/emails/wait", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := api.app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Fatalf("%s: %d", body, resp.StatusCode)
		}
	}
}

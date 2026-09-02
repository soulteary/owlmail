package api

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/types"
)

func TestPrometheusMetricsAreOptIn(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("close mail server: %v", err)
		}
	}()

	req, _ := http.NewRequest(http.MethodGet, "/metrics", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound || strings.Contains(string(body), "<!DOCTYPE html>") {
		t.Fatalf("disabled metrics status/body = %d %q", resp.StatusCode, body)
	}

	api.SetMetricsEnabled(true)
	email := &types.Email{ID: "metrics-message", Subject: "Metrics"}
	envelope := &types.Envelope{From: "sender@example.test", To: []string{"recipient@example.test"}}
	if err := server.SaveEmailToStore("metrics-message", false, envelope, email); err != nil {
		t.Fatal(err)
	}
	_ = tmpDir

	req, _ = http.NewRequest(http.MethodGet, "/metrics", nil)
	resp, err = api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metrics status = %d, body = %s", resp.StatusCode, body)
	}
	if contentType := resp.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain; version=0.0.4") {
		t.Fatalf("metrics content type = %q", contentType)
	}
	for _, sample := range []string{
		"owlmail_mailbox_messages{state=\"total\"} 1",
		"owlmail_mailbox_messages{state=\"unread\"} 1",
		"owlmail_emails_received_total 1",
		"owlmail_websocket_connections 0",
		"owlmail_storage_cleanup_runs_total",
		"owlmail_uptime_seconds",
	} {
		if !strings.Contains(string(body), sample) {
			t.Errorf("metrics output missing %q:\n%s", sample, body)
		}
	}
}

func TestPrometheusMetricsCountMessagesStoredBeforeAPI(t *testing.T) {
	server, err := mailserver.NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("close mail server: %v", err)
		}
	}()

	email := &types.Email{ID: "before-api", Subject: "Before API"}
	envelope := &types.Envelope{From: "sender@example.test", To: []string{"recipient@example.test"}}
	if err := server.SaveEmailToStore(email.ID, false, envelope, email); err != nil {
		t.Fatal(err)
	}

	api := NewAPI(server, 1080, "localhost")
	api.SetMetricsEnabled(true)
	req, _ := http.NewRequest(http.MethodGet, "/metrics", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), "owlmail_emails_received_total 1\n") {
		t.Fatalf("pre-API receive was not counted:\n%s", body)
	}
}

func TestPrometheusMetricsUseBasePathAndBasicAuth(t *testing.T) {
	_, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("close mail server: %v", err)
		}
	}()
	api := NewAPIWithAuth(server, 1080, "localhost", "metrics", "secret")
	if err := api.SetBasePathname("/owlmail"); err != nil {
		t.Fatal(err)
	}
	api.SetMetricsEnabled(true)

	req, _ := http.NewRequest(http.MethodGet, "/owlmail/metrics", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated metrics status = %d", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodGet, "/owlmail/metrics", nil)
	req.SetBasicAuth("metrics", "secret")
	resp, err = api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("authenticated metrics status = %d", resp.StatusCode)
	}
}

func TestPrometheusMetricsCountDeleteAll(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("close mail server: %v", err)
		}
	}()
	api.SetMetricsEnabled(true)

	for _, id := range []string{"delete-all-1", "delete-all-2"} {
		email := &types.Email{ID: id, Subject: id}
		envelope := &types.Envelope{From: "sender@example.test", To: []string{"recipient@example.test"}}
		if err := server.SaveEmailToStore(id, false, envelope, email); err != nil {
			t.Fatal(err)
		}
	}
	if err := server.DeleteAllEmail(); err != nil {
		t.Fatal(err)
	}

	if got := server.GetMailboxMetrics().DeletedMessages; got != 2 {
		t.Fatalf("deleted metric after delete-all = %d, want 2", got)
	}
}

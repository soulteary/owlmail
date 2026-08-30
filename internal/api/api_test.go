package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/types"
)

func setupTestAPI(t *testing.T) (*API, *mailserver.MailServer, string) {
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}

	api := NewAPI(server, 1080, "localhost")
	return api, server, tmpDir
}

func TestNewAPI(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	api := NewAPI(server, 1080, "localhost")
	if api.mailServer != server {
		t.Error("API should have correct mail server")
	}
	if api.port != 1080 {
		t.Errorf("Expected port 1080, got %d", api.port)
	}
	if api.host != "localhost" {
		t.Errorf("Expected host localhost, got %s", api.host)
	}
}

func TestNewAPIWithAuth(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	api := NewAPIWithAuth(server, 1080, "localhost", "user", "pass")
	if api.authUser != "user" {
		t.Errorf("Expected auth user 'user', got '%s'", api.authUser)
	}
	if api.authPassword != "pass" {
		t.Errorf("Expected auth password 'pass', got '%s'", api.authPassword)
	}
}

func TestNewAPIWithHTTPS(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	api := NewAPIWithHTTPS(server, 1080, "localhost", "user", "pass", true, "cert.pem", "key.pem")
	if !api.httpsEnabled {
		t.Error("HTTPS should be enabled")
	}
	if api.httpsCertFile != "cert.pem" {
		t.Errorf("Expected cert file 'cert.pem', got '%s'", api.httpsCertFile)
	}
	if api.httpsKeyFile != "key.pem" {
		t.Errorf("Expected key file 'key.pem', got '%s'", api.httpsKeyFile)
	}
}

func TestAPIHealthCheck(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	req, _ := http.NewRequest("GET", "/api/v1/health", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%v'", response["status"])
	}
	if response["service"] != "owlmail" {
		t.Errorf("Expected service 'owlmail', got '%v'", response["service"])
	}
}

func TestAPISetupEventListeners(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	api.mailServer.On("new", func(email *types.Email) {})

	email := &types.Email{ID: "test-id", Subject: "Test", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}
	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if api.mailServer == nil {
		t.Error("Mail server should be set")
	}
}

func TestAPISetupRoutes(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	if api.app == nil {
		t.Error("App should be set up")
	}

	req, _ := http.NewRequest("GET", "/", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if api.app == nil {
		t.Error("App should be configured")
	}

	req2, _ := http.NewRequest("GET", "/some-page", nil)
	_, _ = api.app.Test(req2, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})

	req3, _ := http.NewRequest("GET", "/api/v1/health", nil)
	resp3, err := api.app.Test(req3, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp3.Body.Close() }()
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("API route should work, got status %d", resp3.StatusCode)
	}

	testCases := []string{"/email", "/config", "/healthz", "/socket.io", "/api/", "/style.css", "/app.js", "/webhooks", "/webhooks.css", "/webhooks.js"}
	for _, path := range testCases {
		req, _ := http.NewRequest("GET", path, nil)
		_, _ = api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	}
}

func TestEmbeddedWebAssets(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Embedded assets must not depend on the process being started from the
	// repository root (for example, after `go install`).
	t.Chdir(t.TempDir())

	tests := []struct {
		path        string
		contentType string
		contains    string
	}{
		{path: "/", contentType: "text/html", contains: `href="/help"`},
		{path: "/some-page", contentType: "text/html", contains: "OwlMail"},
		{path: "/help", contentType: "text/html", contains: "OwlMail Help"},
		{path: "/help/", contentType: "text/html", contains: "快速上手 OwlMail"},
		{path: "/style.css", contentType: "text/css", contains: ".header"},
		{path: "/app.js", contentType: "text/javascript", contains: "connectWebSocket"},
		{path: "/service-worker.js", contentType: "text/javascript", contains: "notificationclick"},
		{path: "/help.css", contentType: "text/css", contains: ".help-shell"},
		{path: "/help.js", contentType: "text/javascript", contains: "applyLanguage"},
		{path: "/webhooks", contentType: "text/html", contains: "Webhook Configurator"},
		{path: "/webhooks/", contentType: "text/html", contains: `id="targetList"`},
		{path: "/webhooks.css", contentType: "text/css", contains: ".webhook-workspace"},
		{path: "/webhooks.js", contentType: "text/javascript", contains: "parseConfigText"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tt.path, nil)
			if err != nil {
				t.Fatalf("NewRequest(%q) error: %v", tt.path, err)
			}
			resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
			if err != nil {
				t.Fatalf("GET %s failed: %v", tt.path, err)
			}
			body, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				t.Fatalf("reading %s response: %v", tt.path, readErr)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET %s status = %d, want %d", tt.path, resp.StatusCode, http.StatusOK)
			}
			if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, tt.contentType) {
				t.Errorf("GET %s Content-Type = %q, want prefix %q", tt.path, got, tt.contentType)
			}
			if !strings.Contains(string(body), tt.contains) {
				t.Errorf("GET %s body does not contain %q", tt.path, tt.contains)
			}
		})
	}
}

func TestAPIStart(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	api := NewAPI(server, 0, "localhost")
	if api == nil {
		t.Fatal("NewAPI should not return nil")
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- api.Start()
	}()

	time.Sleep(100 * time.Millisecond)

	select {
	case err := <-errChan:
		if err != nil {
			t.Logf("Server start error (expected in some cases): %v", err)
		}
	default:
	}

	apiHTTPS := NewAPIWithHTTPS(server, 0, "localhost", "", "", true, "nonexistent.pem", "nonexistent.key")
	errChan2 := make(chan error, 1)
	go func() {
		errChan2 <- apiHTTPS.Start()
	}()

	time.Sleep(100 * time.Millisecond)
	select {
	case err := <-errChan2:
		if err == nil {
			t.Error("Expected error when cert files don't exist")
		}
	default:
		t.Error("Expected error when cert files don't exist")
	}

	apiHTTPS2 := NewAPIWithHTTPS(server, 0, "localhost", "", "", true, "", "key.pem")
	errChan3 := make(chan error, 1)
	go func() {
		errChan3 <- apiHTTPS2.Start()
	}()

	time.Sleep(100 * time.Millisecond)
	select {
	case err := <-errChan3:
		if err == nil {
			t.Error("Expected error when cert file is empty")
		}
	default:
		t.Error("Expected error when cert file is empty")
	}

	apiHTTPS3 := NewAPIWithHTTPS(server, 0, "localhost", "", "", true, "cert.pem", "")
	errChan4 := make(chan error, 1)
	go func() {
		errChan4 <- apiHTTPS3.Start()
	}()

	time.Sleep(100 * time.Millisecond)
	select {
	case err := <-errChan4:
		if err == nil {
			t.Error("Expected error when key file is empty")
		}
	default:
		t.Error("Expected error when key file is empty")
	}
}

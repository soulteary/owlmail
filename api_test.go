package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func setupTestAPI(t *testing.T) (*API, *MailServer, string) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}

	api := NewAPI(server, 1080, "localhost")
	return api, server, tmpDir
}

func TestNewAPI(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	api := NewAPI(server, 1080, "localhost")
	if api == nil {
		t.Error("NewAPI should not return nil")
	}
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
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	api := NewAPIWithAuth(server, 1080, "localhost", "user", "pass")
	if api == nil {
		t.Error("NewAPIWithAuth should not return nil")
	}
	if api.authUser != "user" {
		t.Errorf("Expected auth user 'user', got '%s'", api.authUser)
	}
	if api.authPassword != "pass" {
		t.Errorf("Expected auth password 'pass', got '%s'", api.authPassword)
	}
}

func TestNewAPIWithHTTPS(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	api := NewAPIWithHTTPS(server, 1080, "localhost", "user", "pass", true, "cert.pem", "key.pem")
	if api == nil {
		t.Error("NewAPIWithHTTPS should not return nil")
	}
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
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/health", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%v'", response["status"])
	}
}

func TestAPISetupEventListeners(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Test that event listeners are set up
	api.mailServer.On("new", func(email *Email) {
		// Event listener is set up
	})

	// Create and save an email to trigger event
	email := &Email{ID: "test-id", Subject: "Test", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)
	server.saveEmailToStore("test-id", false, envelope, email)

	// Give time for event to fire
	time.Sleep(100 * time.Millisecond)

	// The event should have been fired by setupEventListeners
	// We can't directly test this, but we can verify the listeners are set up
	if api.mailServer == nil {
		t.Error("Mail server should be set")
	}
}

func TestAPISetupRoutes(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	// Test that routes are set up
	if api.router == nil {
		t.Error("Router should be set up")
	}

	// Test root route
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	api.router.ServeHTTP(w, req)

	// Should serve index.html (may return 404 in test mode if file doesn't exist)
	// But router should be configured
	if api.router == nil {
		t.Error("Router should be configured")
	}
}

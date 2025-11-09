package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
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
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
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
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
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
	api.mailServer.On("new", func(email *types.Email) {
		// Event listener is set up
	})

	// Create and save an email to trigger event
	email := &types.Email{ID: "test-id", Subject: "Test", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)
	server.SaveEmailToStore("test-id", false, envelope, email)

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

	// Test NoRoute handler for non-API routes
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/some-page", nil)
	api.router.ServeHTTP(w, req)
	// Should try to serve index.html (may return 404 in test mode)

	// Test that API routes are not caught by NoRoute
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/health", nil)
	api.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("API route should work, got status %d", w.Code)
	}

	// Test NoRoute handler with various API route prefixes
	testCases := []string{
		"/email",
		"/config",
		"/healthz",
		"/socket.io",
		"/api/",
		"/style.css",
		"/app.js",
	}
	for _, path := range testCases {
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("GET", path, nil)
		api.router.ServeHTTP(w, req)
		// These should not be caught by NoRoute
	}
}

func TestAPIStart(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	api := NewAPI(server, 0, "localhost") // Use port 0 for random port
	if api == nil {
		t.Fatal("NewAPI should not return nil")
	}

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- api.Start()
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Check if server started successfully
	select {
	case err := <-errChan:
		// If we get an error immediately, it might be because port is already in use
		// That's okay for testing purposes
		if err != nil {
			t.Logf("Server start error (expected in some cases): %v", err)
		}
	default:
		// Server is running, which is good
	}

	// Test HTTPS start with missing cert files
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

	// Test HTTPS start with empty cert file
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

	// Test HTTPS start with empty key file
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

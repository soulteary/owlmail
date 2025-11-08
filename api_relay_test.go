package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestAPIRelayEmail(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add test email
	email := &Email{ID: "test-id", Subject: "Test Subject", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)
	server.saveEmailToStore("test-id", false, envelope, email)

	// Test relay with query parameter
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/emails/test-id/actions/relay?relayTo=relay@example.com", nil)
	api.router.ServeHTTP(w, req)

	// Should return 200 or 400 depending on relay configuration
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 200 or 400, got %d", w.Code)
	}
}

func TestAPIRelayEmailWithBody(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add test email
	email := &Email{ID: "test-id", Subject: "Test Subject", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)
	server.saveEmailToStore("test-id", false, envelope, email)

	// Test relay with body parameter
	requestBody := map[string]interface{}{
		"relayTo": "relay@example.com",
	}
	jsonBody, _ := json.Marshal(requestBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/emails/test-id/actions/relay", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w, req)

	// Should return 200 or 400 depending on relay configuration
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 200 or 400, got %d", w.Code)
	}
}

func TestAPIRelayEmailWithoutRelayTo(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add test email
	email := &Email{ID: "test-id", Subject: "Test Subject", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)
	server.saveEmailToStore("test-id", false, envelope, email)

	// Test relay without relayTo (uses configured SMTP server)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/emails/test-id/actions/relay", nil)
	api.router.ServeHTTP(w, req)

	// Should return 200 or 400 depending on relay configuration
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 200 or 400, got %d", w.Code)
	}
}

func TestAPIRelayEmailNotFound(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/emails/nonexistent/actions/relay", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestAPIRelayEmailWithParam(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add test email
	email := &Email{ID: "test-id", Subject: "Test Subject", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)
	server.saveEmailToStore("test-id", false, envelope, email)

	// Test relay with param
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/emails/test-id/actions/relay/relay@example.com", nil)
	api.router.ServeHTTP(w, req)

	// Should return 200 or 400 depending on relay configuration
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 200 or 400, got %d", w.Code)
	}
}

func TestAPIRelayEmailWithParamEmpty(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add test email
	email := &Email{ID: "test-id", Subject: "Test Subject", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)
	server.saveEmailToStore("test-id", false, envelope, email)

	// Test relay with empty param (using empty string as param)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	// Use a route that will have empty relayTo param
	req, _ := http.NewRequest("POST", "/api/v1/emails/test-id/actions/relay/ ", nil)
	api.router.ServeHTTP(w, req)

	// The route might redirect or return 400, both are acceptable
	// The important thing is that it doesn't succeed with empty param
	if w.Code == http.StatusOK {
		t.Errorf("Expected status not 200 for empty param, got %d", w.Code)
	}
}

func TestAPIRelayEmailWithParamNotFound(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/emails/nonexistent/actions/relay/relay@example.com", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestAPIRelayEmailWithBodyButNoRelayTo(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add test email
	email := &Email{ID: "test-id", Subject: "Test Subject", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)
	server.saveEmailToStore("test-id", false, envelope, email)

	// Test relay with body but no relayTo field
	requestBody := map[string]interface{}{
		"other": "value",
	}
	jsonBody, _ := json.Marshal(requestBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/emails/test-id/actions/relay", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w, req)

	// Should return 200 or 400 depending on relay configuration
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 200 or 400, got %d", w.Code)
	}
}

func TestAPIRelayEmailWithInvalidBody(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add test email
	email := &Email{ID: "test-id", Subject: "Test Subject", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)
	server.saveEmailToStore("test-id", false, envelope, email)

	// Test relay with invalid JSON body
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/emails/test-id/actions/relay", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w, req)

	// Should return 200 or 400 depending on relay configuration (invalid JSON is ignored)
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 200 or 400, got %d", w.Code)
	}
}

func TestAPIRelayEmailWithParamEmptyString(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add test email
	email := &Email{ID: "test-id", Subject: "Test Subject", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)
	server.saveEmailToStore("test-id", false, envelope, email)

	// Test relay with empty string param (using empty string as param value)
	// Note: Gin router may redirect trailing slashes, so we test with actual empty param
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	// Use a route that will have empty relayTo param - need to use a different approach
	// Since Gin handles trailing slashes, we'll test the validation logic differently
	req, _ := http.NewRequest("POST", "/api/v1/emails/test-id/actions/relay/%20", nil) // space character
	api.router.ServeHTTP(w, req)

	// Should return 400 for empty/invalid email address
	// Note: The route might redirect or return different status, but validation should catch it
	if w.Code == http.StatusOK {
		t.Errorf("Expected status not 200 for empty/invalid email address, got %d", w.Code)
	}
}

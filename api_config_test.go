package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAPIGetConfig(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/settings", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if response["version"] == nil {
		t.Error("Response should have version field")
	}
}

func TestAPIGetOutgoingConfig(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/settings/outgoing", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIUpdateOutgoingConfig(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	config := map[string]interface{}{
		"host":   "smtp.example.com",
		"port":   587,
		"user":   "user",
		"secure": true,
	}
	jsonBody, _ := json.Marshal(config)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/settings/outgoing", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIPatchOutgoingConfig(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	// First set a config
	config := map[string]interface{}{
		"host": "smtp.example.com",
		"port": 587,
	}
	jsonBody, _ := json.Marshal(config)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/settings/outgoing", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w, req)

	// Then patch it
	patch := map[string]interface{}{
		"port": 465,
	}
	patchBody, _ := json.Marshal(patch)

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("PATCH", "/api/v1/settings/outgoing", bytes.NewBuffer(patchBody))
	req2.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w2.Code)
	}
}

func TestAPIGetConfigWithOutgoing(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	// Set outgoing config
	outgoingConfig := &OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	}
	server.SetOutgoingConfig(outgoingConfig)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/settings", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if response["outgoing"] == nil {
		t.Error("Response should have outgoing field")
	}
}

func TestAPIGetConfigWithAuth(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	// Set auth config
	authConfig := &SMTPAuthConfig{
		Enabled:  true,
		Username: "user",
	}
	server.authConfig = authConfig

	api := NewAPI(server, 1080, "localhost")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/settings", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if response["smtpAuth"] == nil {
		t.Error("Response should have smtpAuth field")
	}
}

func TestAPIGetConfigWithTLS(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	// Set TLS config
	tlsConfig := &TLSConfig{
		Enabled:  true,
		CertFile: "cert.pem",
		KeyFile:  "key.pem",
	}
	server.tlsConfig = tlsConfig

	api := NewAPI(server, 1080, "localhost")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/settings", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if response["tls"] == nil {
		t.Error("Response should have tls field")
	}
}

func TestAPIUpdateOutgoingConfigInvalidRequest(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/settings/outgoing", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAPIUpdateOutgoingConfigMissingHost(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	config := map[string]interface{}{
		"port": 587,
	}
	jsonBody, _ := json.Marshal(config)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/settings/outgoing", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAPIUpdateOutgoingConfigInvalidPort(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	config := map[string]interface{}{
		"host": "smtp.example.com",
		"port": 0,
	}
	jsonBody, _ := json.Marshal(config)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/settings/outgoing", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAPIUpdateOutgoingConfigPortTooLarge(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	config := map[string]interface{}{
		"host": "smtp.example.com",
		"port": 70000,
	}
	jsonBody, _ := json.Marshal(config)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/settings/outgoing", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAPIPatchOutgoingConfigInvalidRequest(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/settings/outgoing", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAPIPatchOutgoingConfigAllFields(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	// Patch with all fields
	patch := map[string]interface{}{
		"host":          "smtp.example.com",
		"port":          587,
		"user":          "user",
		"password":      "pass",
		"secure":        true,
		"autoRelay":     true,
		"autoRelayAddr": "relay@example.com",
		"allowRules":    []interface{}{"allow@example.com"},
		"denyRules":     []interface{}{"deny@example.com"},
	}
	patchBody, _ := json.Marshal(patch)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/settings/outgoing", bytes.NewBuffer(patchBody))
	req.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIPatchOutgoingConfigMissingHostAfterPatch(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	// Patch without host
	patch := map[string]interface{}{
		"port": 587,
	}
	patchBody, _ := json.Marshal(patch)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/settings/outgoing", bytes.NewBuffer(patchBody))
	req.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

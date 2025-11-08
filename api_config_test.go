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

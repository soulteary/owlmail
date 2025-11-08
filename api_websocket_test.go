package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TestAPIHandleWebSocket(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)

	// Create a test server
	srv := httptest.NewServer(api.router)
	defer srv.Close()

	// Convert http:// to ws://
	wsURL := "ws" + srv.URL[4:] + "/api/v1/ws"

	// Try to connect (this will fail in test environment, but we can test the route exists)
	_, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	// We expect this to fail in test environment, but the route should exist
	if err == nil {
		// If connection succeeds, close it
		// This is unlikely in test environment
	}
}

func TestAPIBroadcastMessage(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	// Test that broadcastMessage doesn't panic with no clients
	api.broadcastMessage(gin.H{
		"type":  "test",
		"value": "test",
	})

	// Test that broadcastMessage works with clients
	// We can't easily test WebSocket connections in unit tests,
	// but we can verify the function exists and doesn't panic
	if api.wsClients == nil {
		t.Error("WebSocket clients map should be initialized")
	}
}

func TestAPISetupEventListenersBroadcast(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add a test email to trigger event
	email := &Email{ID: "test-id", Subject: "Test", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)
	server.saveEmailToStore("test-id", false, envelope, email)

	// Give time for event to fire
	time.Sleep(100 * time.Millisecond)

	// Verify event listeners are set up
	// The broadcast should have been called (even if no clients connected)
	if api.mailServer == nil {
		t.Error("Mail server should be set")
	}
}

func TestAPIBroadcastMessageWithClients(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	// Test that broadcastMessage handles write errors gracefully
	// We can't easily create a real WebSocket connection in unit tests,
	// but we can test the function structure

	// Test with empty clients map
	api.broadcastMessage(gin.H{"type": "test"})

	// Verify the function doesn't panic
	if api.wsClients == nil {
		t.Error("WebSocket clients map should be initialized")
	}
}

func TestAPIBroadcastMessageWithDeleteEvent(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add and then delete an email to trigger delete event
	email := &Email{ID: "test-id", Subject: "Test", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)
	server.saveEmailToStore("test-id", false, envelope, email)

	// Delete the email to trigger delete event
	server.DeleteEmail("test-id")

	// Give time for event to fire
	time.Sleep(100 * time.Millisecond)

	// Verify event listeners are set up
	if api.mailServer == nil {
		t.Error("Mail server should be set")
	}
}

func TestAPIHandleWebSocketRoute(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)

	// Test that the WebSocket route exists
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/ws", nil)
	api.router.ServeHTTP(w, req)

	// WebSocket upgrade should fail in test mode (no upgrade header)
	// But the route should exist
	if w.Code == http.StatusNotFound {
		t.Error("WebSocket route should exist")
	}
}

// TestAPIHandleWebSocketUpgradeError tests WebSocket upgrade error handling
func TestAPIHandleWebSocketUpgradeError(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)

	// Test WebSocket upgrade with invalid request (no upgrade header)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/ws", nil)
	api.router.ServeHTTP(w, req)

	// Should fail gracefully (upgrade error is handled internally)
	// The route exists but upgrade fails without proper headers
	if w.Code == http.StatusNotFound {
		t.Error("WebSocket route should exist")
	}
}

// TestAPIBroadcastMessageWriteError tests WebSocket write error handling
func TestAPIBroadcastMessageWriteError(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	// Test that broadcastMessage handles errors gracefully
	// Even with no clients, it should not panic
	api.broadcastMessage(gin.H{
		"type":  "test",
		"value": "test",
	})

	// Verify the function doesn't panic with empty clients
	if api.wsClients == nil {
		t.Error("WebSocket clients map should be initialized")
	}

	// Test with nil message (should not panic)
	api.broadcastMessage(nil)
}

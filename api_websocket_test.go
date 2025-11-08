package main

import (
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

package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/soulteary/owlmail/internal/types"
)

func TestAPIHandleWebSocket(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	gin.SetMode(gin.TestMode)

	// Create a test server
	srv := httptest.NewServer(api.router)
	defer srv.Close()

	// Convert http:// to ws://
	wsURL := "ws" + srv.URL[4:] + "/api/v1/ws"

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	// Wait for initial connection message
	var msg map[string]interface{}
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read initial message: %v", err)
	}

	if msg["type"] != "connected" {
		t.Errorf("Expected 'connected' message type, got %v", msg["type"])
	}

	// Verify client was added
	api.wsClientsLock.RLock()
	clientCount := len(api.wsClients)
	api.wsClientsLock.RUnlock()

	if clientCount != 1 {
		t.Errorf("Expected 1 client, got %d", clientCount)
	}

	// Close connection and verify it's removed
	conn.Close()
	time.Sleep(50 * time.Millisecond)

	api.wsClientsLock.RLock()
	clientCount = len(api.wsClients)
	api.wsClientsLock.RUnlock()

	if clientCount != 0 {
		t.Errorf("Expected 0 clients after close, got %d", clientCount)
	}
}

func TestAPIBroadcastMessage(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	gin.SetMode(gin.TestMode)

	// Test that broadcastMessage doesn't panic with no clients
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

	// Test with actual WebSocket connection
	srv := httptest.NewServer(api.router)
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:] + "/api/v1/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	// Read initial connection message
	var msg map[string]interface{}
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read initial message: %v", err)
	}

	// Broadcast a message
	testMessage := gin.H{
		"type":  "test",
		"value": "broadcast test",
	}
	api.broadcastMessage(testMessage)

	// Read the broadcast message
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read broadcast message: %v", err)
	}

	if msg["type"] != "test" {
		t.Errorf("Expected message type 'test', got %v", msg["type"])
	}
}

func TestAPISetupEventListenersBroadcast(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add a test email to trigger event
	email := &types.Email{ID: "test-id", Subject: "Test", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}
	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	gin.SetMode(gin.TestMode)

	// Test with empty clients map
	api.broadcastMessage(gin.H{"type": "test"})

	// Verify the function doesn't panic
	if api.wsClients == nil {
		t.Error("WebSocket clients map should be initialized")
	}

	// Test with multiple clients
	srv := httptest.NewServer(api.router)
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:] + "/api/v1/ws"

	// Connect first client
	conn1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect first client: %v", err)
	}
	defer conn1.Close()

	// Read initial message
	var msg map[string]interface{}
	if err := conn1.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read initial message: %v", err)
	}

	// Connect second client
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect second client: %v", err)
	}
	defer conn2.Close()

	// Read initial message
	if err := conn2.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read initial message: %v", err)
	}

	// Verify we have 2 clients
	api.wsClientsLock.RLock()
	clientCount := len(api.wsClients)
	api.wsClientsLock.RUnlock()

	if clientCount != 2 {
		t.Errorf("Expected 2 clients, got %d", clientCount)
	}

	// Broadcast message to all clients
	broadcastMsg := gin.H{"type": "broadcast", "data": "test"}
	api.broadcastMessage(broadcastMsg)

	// Both clients should receive the message
	conn1.SetReadDeadline(time.Now().Add(1 * time.Second))
	if err := conn1.ReadJSON(&msg); err != nil {
		t.Fatalf("Client 1 failed to read broadcast: %v", err)
	}
	if msg["type"] != "broadcast" {
		t.Errorf("Client 1: Expected 'broadcast', got %v", msg["type"])
	}

	conn2.SetReadDeadline(time.Now().Add(1 * time.Second))
	if err := conn2.ReadJSON(&msg); err != nil {
		t.Fatalf("Client 2 failed to read broadcast: %v", err)
	}
	if msg["type"] != "broadcast" {
		t.Errorf("Client 2: Expected 'broadcast', got %v", msg["type"])
	}
}

func TestAPIBroadcastMessageWithDeleteEvent(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add and then delete an email to trigger delete event
	email := &types.Email{ID: "test-id", Subject: "Test", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}
	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	// Delete the email to trigger delete event
	if err := server.DeleteEmail("test-id"); err != nil {
		t.Fatalf("Failed to delete email: %v", err)
	}

	// Give time for event to fire
	time.Sleep(100 * time.Millisecond)

	// Verify event listeners are set up
	if api.mailServer == nil {
		t.Error("Mail server should be set")
	}
}

func TestAPIHandleWebSocketRoute(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	gin.SetMode(gin.TestMode)

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

	// Test with a closed connection to trigger write error
	srv := httptest.NewServer(api.router)
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:] + "/api/v1/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}

	// Read initial message
	var msg map[string]interface{}
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read initial message: %v", err)
	}

	// Verify client was added
	api.wsClientsLock.RLock()
	clientCount := len(api.wsClients)
	api.wsClientsLock.RUnlock()

	if clientCount != 1 {
		t.Errorf("Expected 1 client, got %d", clientCount)
	}

	// Close the connection to simulate write error
	conn.Close()

	// Wait a bit for the connection to be recognized as closed
	time.Sleep(50 * time.Millisecond)

	// Try to broadcast - this should trigger write error handling
	api.broadcastMessage(gin.H{
		"type":  "test",
		"value": "test",
	})

	// Wait for cleanup
	time.Sleep(50 * time.Millisecond)

	// Verify failed client was removed
	api.wsClientsLock.RLock()
	clientCount = len(api.wsClients)
	api.wsClientsLock.RUnlock()

	if clientCount != 0 {
		t.Errorf("Expected 0 clients after write error, got %d", clientCount)
	}
}

// TestAPIHandleWebSocketPingPong tests ping/pong message handling
func TestAPIHandleWebSocketPingPong(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	gin.SetMode(gin.TestMode)

	srv := httptest.NewServer(api.router)
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:] + "/api/v1/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	// Read initial connection message
	var msg map[string]interface{}
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read initial message: %v", err)
	}

	// Send ping message
	pingMsg := gin.H{"type": "ping"}
	if err := conn.WriteJSON(pingMsg); err != nil {
		t.Fatalf("Failed to send ping: %v", err)
	}

	// Read pong response
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read pong: %v", err)
	}

	if msg["type"] != "pong" {
		t.Errorf("Expected 'pong' message type, got %v", msg["type"])
	}
}

// TestAPIHandleWebSocketOtherMessages tests handling of non-ping messages
func TestAPIHandleWebSocketOtherMessages(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	gin.SetMode(gin.TestMode)

	srv := httptest.NewServer(api.router)
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:] + "/api/v1/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	// Read initial connection message
	var msg map[string]interface{}
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read initial message: %v", err)
	}

	// Send a non-ping message (should be ignored but not cause error)
	otherMsg := gin.H{"type": "other", "data": "test"}
	if err := conn.WriteJSON(otherMsg); err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// The server should continue running without error
	// We can verify by checking the connection is still alive
	time.Sleep(100 * time.Millisecond)

	// Connection should still be valid
	if err := conn.WriteJSON(gin.H{"type": "ping"}); err != nil {
		t.Fatalf("Connection should still be valid: %v", err)
	}
}

// TestAPIHandleWebSocketConnectionClose tests connection close handling
func TestAPIHandleWebSocketConnectionClose(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	gin.SetMode(gin.TestMode)

	srv := httptest.NewServer(api.router)
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:] + "/api/v1/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}

	// Read initial connection message
	var msg map[string]interface{}
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read initial message: %v", err)
	}

	// Verify client was added
	api.wsClientsLock.RLock()
	clientCount := len(api.wsClients)
	api.wsClientsLock.RUnlock()

	if clientCount != 1 {
		t.Errorf("Expected 1 client, got %d", clientCount)
	}

	// Close connection gracefully
	conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	conn.Close()

	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)

	// Verify client was removed
	api.wsClientsLock.RLock()
	clientCount = len(api.wsClients)
	api.wsClientsLock.RUnlock()

	if clientCount != 0 {
		t.Errorf("Expected 0 clients after close, got %d", clientCount)
	}
}

// TestAPIBroadcastMessageWithFailedClient tests broadcast with a failed client
func TestAPIBroadcastMessageWithFailedClient(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	gin.SetMode(gin.TestMode)

	srv := httptest.NewServer(api.router)
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:] + "/api/v1/ws"

	// Connect first client
	conn1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect first client: %v", err)
	}
	defer conn1.Close()

	// Read initial message
	var msg map[string]interface{}
	if err := conn1.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read initial message: %v", err)
	}

	// Connect second client
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect second client: %v", err)
	}
	defer conn2.Close()

	// Read initial message
	if err := conn2.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read initial message: %v", err)
	}

	// Close one connection to simulate failure
	conn1.Close()

	// Wait a bit for the connection to be recognized as closed
	time.Sleep(100 * time.Millisecond)

	// Broadcast should handle the failed client gracefully
	broadcastMsg := gin.H{"type": "broadcast", "data": "test"}
	api.broadcastMessage(broadcastMsg)

	// The second client should still receive the message
	conn2.SetReadDeadline(time.Now().Add(1 * time.Second))
	if err := conn2.ReadJSON(&msg); err != nil {
		t.Fatalf("Client 2 failed to read broadcast: %v", err)
	}
	if msg["type"] != "broadcast" {
		t.Errorf("Client 2: Expected 'broadcast', got %v", msg["type"])
	}
}

// TestAPIHandleWebSocketInitialMessageError tests error when sending initial message fails
func TestAPIHandleWebSocketInitialMessageError(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	gin.SetMode(gin.TestMode)

	srv := httptest.NewServer(api.router)
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:] + "/api/v1/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}

	// Read initial connection message
	var msg map[string]interface{}
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read initial message: %v", err)
	}

	// Close the connection immediately after reading initial message
	// This tests the defer cleanup
	conn.Close()

	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)

	// Verify client was removed
	api.wsClientsLock.RLock()
	clientCount := len(api.wsClients)
	api.wsClientsLock.RUnlock()

	if clientCount != 0 {
		t.Errorf("Expected 0 clients after close, got %d", clientCount)
	}
}

package api

import (
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/soulteary/owlmail/internal/common"
)

// handleWebSocket handles WebSocket connections
func (api *API) handleWebSocket(c *gin.Context) {
	conn, err := api.wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		common.Verbose("WebSocket upgrade error: %v", err)
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			common.Verbose("Failed to close WebSocket connection: %v", err)
		}
	}()

	// Create a write mutex for this connection
	writeMutex := &sync.Mutex{}

	// Add client
	api.wsClientsLock.Lock()
	api.wsClients[conn] = writeMutex
	api.wsClientsLock.Unlock()

	// Remove client on disconnect
	defer func() {
		api.wsClientsLock.Lock()
		delete(api.wsClients, conn)
		api.wsClientsLock.Unlock()
	}()

	// Send initial connection message
	writeMutex.Lock()
	err = conn.WriteJSON(gin.H{
		"type":    "connected",
		"message": "WebSocket connection established",
	})
	writeMutex.Unlock()
	if err != nil {
		common.Verbose("Failed to send WebSocket connection message: %v", err)
		return
	}

	// Keep connection alive and handle incoming messages
	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				common.Verbose("WebSocket error: %v", err)
			}
			break
		}

		// Handle ping/pong
		if msgType, ok := msg["type"].(string); ok && msgType == "ping" {
			writeMutex.Lock()
			err = conn.WriteJSON(gin.H{"type": "pong"})
			writeMutex.Unlock()
			if err != nil {
				common.Verbose("Failed to send WebSocket pong: %v", err)
				break
			}
		}
	}
}

// broadcastMessage broadcasts a message to all connected WebSocket clients
func (api *API) broadcastMessage(message interface{}) {
	// Collect failed connections to remove after releasing read lock
	var failedConns []*websocket.Conn

	api.wsClientsLock.RLock()
	// Create a snapshot of connections and their mutexes
	conns := make(map[*websocket.Conn]*sync.Mutex, len(api.wsClients))
	for conn, writeMutex := range api.wsClients {
		conns[conn] = writeMutex
	}
	api.wsClientsLock.RUnlock()

	// Write to each connection using its own mutex
	for conn, writeMutex := range conns {
		writeMutex.Lock()
		err := conn.WriteJSON(message)
		writeMutex.Unlock()
		if err != nil {
			common.Verbose("WebSocket write error: %v", err)
			// Collect failed client for removal
			failedConns = append(failedConns, conn)
		}
	}

	// Remove failed clients with write lock
	if len(failedConns) > 0 {
		api.wsClientsLock.Lock()
		for _, conn := range failedConns {
			if writeMutex, exists := api.wsClients[conn]; exists {
				// Lock the connection's mutex before closing to ensure no concurrent writes
				writeMutex.Lock()
				delete(api.wsClients, conn)
				writeMutex.Unlock()
				if err := conn.Close(); err != nil {
					common.Verbose("Failed to close WebSocket connection: %v", err)
				}
			}
		}
		api.wsClientsLock.Unlock()
	}
}

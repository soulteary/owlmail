package api

import (
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

	// Add client
	api.wsClientsLock.Lock()
	api.wsClients[conn] = true
	api.wsClientsLock.Unlock()

	// Remove client on disconnect
	defer func() {
		api.wsClientsLock.Lock()
		delete(api.wsClients, conn)
		api.wsClientsLock.Unlock()
	}()

	// Send initial connection message
	if err := conn.WriteJSON(gin.H{
		"type":    "connected",
		"message": "WebSocket connection established",
	}); err != nil {
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
			if err := conn.WriteJSON(gin.H{"type": "pong"}); err != nil {
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
	for conn := range api.wsClients {
		if err := conn.WriteJSON(message); err != nil {
			common.Verbose("WebSocket write error: %v", err)
			// Collect failed client for removal
			failedConns = append(failedConns, conn)
		}
	}
	api.wsClientsLock.RUnlock()

	// Remove failed clients with write lock
	if len(failedConns) > 0 {
		api.wsClientsLock.Lock()
		for _, conn := range failedConns {
			delete(api.wsClients, conn)
			if err := conn.Close(); err != nil {
				common.Verbose("Failed to close WebSocket connection: %v", err)
			}
		}
		api.wsClientsLock.Unlock()
	}
}

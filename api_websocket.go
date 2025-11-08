package main

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// handleWebSocket handles WebSocket connections
func (api *API) handleWebSocket(c *gin.Context) {
	conn, err := api.wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		Verbose("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

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
	conn.WriteJSON(gin.H{
		"type":    "connected",
		"message": "WebSocket connection established",
	})

	// Keep connection alive and handle incoming messages
	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				Verbose("WebSocket error: %v", err)
			}
			break
		}

		// Handle ping/pong
		if msgType, ok := msg["type"].(string); ok && msgType == "ping" {
			conn.WriteJSON(gin.H{"type": "pong"})
		}
	}
}

// broadcastMessage broadcasts a message to all connected WebSocket clients
func (api *API) broadcastMessage(message interface{}) {
	api.wsClientsLock.RLock()
	defer api.wsClientsLock.RUnlock()

	for conn := range api.wsClients {
		if err := conn.WriteJSON(message); err != nil {
			Verbose("WebSocket write error: %v", err)
			// Remove failed client
			delete(api.wsClients, conn)
			conn.Close()
		}
	}
}

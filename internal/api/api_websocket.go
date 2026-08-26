package api

import (
	"net/http"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gorilla/websocket"
	"github.com/soulteary/owlmail/internal/common"
)

// handleWebSocketHTTP handles WebSocket connections via standard http.ResponseWriter and *http.Request.
// Used with adaptor.HTTPHandlerFunc for Fiber routes.
func (api *API) handleWebSocketHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := api.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		common.Verbose("WebSocket upgrade error: %v", err)
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			common.Verbose("Failed to close WebSocket connection: %v", err)
		}
	}()

	writeMutex := &sync.Mutex{}

	api.wsClientsLock.Lock()
	api.wsClients[conn] = writeMutex
	api.wsClientsLock.Unlock()

	defer func() {
		api.wsClientsLock.Lock()
		delete(api.wsClients, conn)
		api.wsClientsLock.Unlock()
	}()

	writeMutex.Lock()
	err = conn.WriteJSON(fiber.Map{
		"type":    "connected",
		"message": "WebSocket connection established",
	})
	writeMutex.Unlock()
	if err != nil {
		common.Verbose("Failed to send WebSocket connection message: %v", err)
		return
	}

	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				common.Verbose("WebSocket error: %v", err)
			}
			break
		}

		if msgType, ok := msg["type"].(string); ok && msgType == "ping" {
			writeMutex.Lock()
			err = conn.WriteJSON(fiber.Map{"type": "pong"})
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
	var failedConns []*websocket.Conn

	api.wsClientsLock.RLock()
	conns := make(map[*websocket.Conn]*sync.Mutex, len(api.wsClients))
	for conn, writeMutex := range api.wsClients {
		conns[conn] = writeMutex
	}
	api.wsClientsLock.RUnlock()

	for conn, writeMutex := range conns {
		writeMutex.Lock()
		err := conn.WriteJSON(message)
		writeMutex.Unlock()
		if err != nil {
			common.Verbose("WebSocket write error: %v", err)
			failedConns = append(failedConns, conn)
		}
	}

	if len(failedConns) > 0 {
		api.wsClientsLock.Lock()
		for _, conn := range failedConns {
			if writeMutex, exists := api.wsClients[conn]; exists {
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

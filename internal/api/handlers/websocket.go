package handlers

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"

	"github.com/ParkWardRR/relaystation/internal/stream"
)

// WebSocketHandler handles WebSocket connections for real-time updates
type WebSocketHandler struct {
	manager *stream.Manager
	clients map[*websocket.Conn]bool
	mu      sync.RWMutex
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(mgr *stream.Manager) *WebSocketHandler {
	h := &WebSocketHandler{
		manager: mgr,
		clients: make(map[*websocket.Conn]bool),
	}

	// Start broadcasting events
	go h.broadcastLoop()

	return h
}

// Upgrade returns the WebSocket upgrade middleware
func (h *WebSocketHandler) Upgrade() fiber.Handler {
	return websocket.New(h.handle)
}

func (h *WebSocketHandler) handle(c *websocket.Conn) {
	// Register client
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()

	log.Printf("[WebSocket] Client connected: %s", c.RemoteAddr())

	defer func() {
		// Unregister client
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()

		c.Close()
		log.Printf("[WebSocket] Client disconnected: %s", c.RemoteAddr())
	}()

	// Keep connection alive and handle incoming messages
	for {
		_, _, err := c.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (h *WebSocketHandler) broadcastLoop() {
	events := h.manager.Subscribe()

	// Also send periodic status updates
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			h.broadcast(event)

		case <-ticker.C:
			// Send status update to all clients
			status := h.manager.GetStatus()
			h.broadcast(map[string]interface{}{
				"type": "status_update",
				"data": status,
			})
		}
	}
}

func (h *WebSocketHandler) broadcast(msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if err := client.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("[WebSocket] Error sending to client: %v", err)
		}
	}
}

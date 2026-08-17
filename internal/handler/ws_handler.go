package handler

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/yourusername/go-starter/internal/socket"
	"github.com/yourusername/go-starter/internal/store"
	"github.com/yourusername/go-starter/pkg/response"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all connections in development
		return true
	},
}

// WebSocketHandler handles upgrading HTTP requests to WebSocket.
type WebSocketHandler struct {
	store *store.Store
	hub   *socket.Hub
}

// NewWebSocketHandler creates a new WebSocketHandler.
func NewWebSocketHandler(s *store.Store, h *socket.Hub) *WebSocketHandler {
	return &WebSocketHandler{
		store: s,
		hub:   h,
	}
}

// ServeWS upgrades the connection and registers the client in the hub.
func (h *WebSocketHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
	// 1. Extract query parameters
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		response.JSON(w, http.StatusBadRequest, "user_id query parameter is required", nil)
		return
	}

	userType := r.URL.Query().Get("user_type")
	if userType == "" {
		response.JSON(w, http.StatusBadRequest, "user_type query parameter is required", nil)
		return
	}

	// 2. Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade HTTP connection: %v", err)
		return
	}

	// 3. Create and register client
	client := &socket.Client{
		UserID: userID,
		Name:   userType, // Decoupled: Name fallback to Type
		Role:   userType,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		Hub:    h.hub,
	}

	h.hub.Register(client)

	// Start reading and writing loops in goroutines
	go client.WritePump()
	go client.ReadPump()
}

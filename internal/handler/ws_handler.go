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
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		response.JSON(w, http.StatusBadRequest, "user_id query parameter is required", nil)
		return
	}

	// 1. Fetch user from DB to verify identity and get their role
	user, err := h.store.GetUserByID(r.Context(), userID)
	if err != nil {
		log.Printf("WebSocket connection rejected: user %s not found: %v", userID, err)
		response.JSON(w, http.StatusNotFound, "user not found", nil)
		return
	}

	// 2. If Customer/Driver, ensure they have a conversation
	var conversationID string
	if user.Role != "ADMIN" {
		conversationID, err = h.store.GetOrCreateConversation(r.Context(), user.ID)
		if err != nil {
			log.Printf("Failed to get/create conversation for user %s: %v", user.ID, err)
			response.JSON(w, http.StatusInternalServerError, "failed to initialize chat session", nil)
			return
		}
	}

	// 3. Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade HTTP connection: %v", err)
		return
	}

	// 4. Create and register client
	client := &socket.Client{
		UserID:         user.ID,
		Name:           user.Name,
		Role:           user.Role,
		ConversationID: conversationID,
		Conn:           conn,
		Send:           make(chan []byte, 256),
		Hub:            h.hub,
	}

	h.hub.Register(client)

	// Start reading and writing loops in goroutines
	go client.WritePump()
	go client.ReadPump()
}

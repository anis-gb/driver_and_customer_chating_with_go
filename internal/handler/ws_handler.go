package handler

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/yourusername/go-starter/internal/socket"
	"github.com/yourusername/go-starter/internal/store"
	"github.com/yourusername/go-starter/pkg/auth"
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
	store  *store.Store
	hub    *socket.Hub
	secret string
}

// NewWebSocketHandler creates a new WebSocketHandler.
func NewWebSocketHandler(s *store.Store, h *socket.Hub, secret string) *WebSocketHandler {
	return &WebSocketHandler{
		store:  s,
		hub:    h,
		secret: secret,
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

	// For CUSTOMER and DRIVER, verify HMAC signature via query params
	if user.Role != "ADMIN" {
		timestamp := r.URL.Query().Get("timestamp")
		nonce := r.URL.Query().Get("nonce")
		signature := r.URL.Query().Get("signature")

		if timestamp == "" || nonce == "" || signature == "" {
			response.JSON(w, http.StatusUnauthorized, "missing authentication query parameters (timestamp, nonce, signature)", nil)
			return
		}

		if err := auth.VerifySignature(timestamp, nonce, signature, h.secret); err != nil {
			response.JSON(w, http.StatusUnauthorized, "invalid signature: "+err.Error(), nil)
			return
		}
	}

	// 2. Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade HTTP connection: %v", err)
		return
	}

	// 3. Create and register client
	client := &socket.Client{
		UserID: user.ID,
		Name:   user.Name,
		Role:   user.Role,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		Hub:    h.hub,
	}

	h.hub.Register(client)

	// Start reading and writing loops in goroutines
	go client.WritePump()
	go client.ReadPump()
}

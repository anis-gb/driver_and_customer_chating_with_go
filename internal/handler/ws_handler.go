package handler

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/yourusername/go-starter/internal/middleware"
	"github.com/yourusername/go-starter/internal/socket"
	"github.com/yourusername/go-starter/internal/store"
	"github.com/yourusername/go-starter/pkg/response"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// In production, you should check r.Header.Get("Origin") against allowed domains.
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
	// 1. Extract user identifiers
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

	// 2. Extract authentication parameters from query string
	tsStr := r.URL.Query().Get("timestamp")
	if tsStr == "" {
		tsStr = r.URL.Query().Get("current_timestamp")
	}
	nonce := r.URL.Query().Get("nonce")
	if nonce == "" {
		nonce = r.URL.Query().Get("current_nonce")
	}
	signature := r.URL.Query().Get("signature")
	if signature == "" {
		signature = r.URL.Query().Get("current_signature")
	}

	// Validate HMAC signature if provided, or enforce for non-ADMIN connections
	if tsStr != "" || nonce != "" || signature != "" {
		err := middleware.ValidateHMAC(r.Method, r.URL.Path, tsStr, nonce, signature, h.secret)
		if err != nil {
			response.JSON(w, http.StatusUnauthorized, err.Error(), nil)
			return
		}
	} else if userType != "ADMIN" {
		response.JSON(w, http.StatusUnauthorized, "missing authentication parameters (timestamp, nonce, signature)", nil)
		return
	}

	// 3. Upgrade to WebSocket
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

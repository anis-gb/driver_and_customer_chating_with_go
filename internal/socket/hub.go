package socket

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yourusername/go-starter/internal/store"
)

// Client represents a connected user.
type Client struct {
	UserID         string
	Name           string
	Role           string // "ADMIN", "CUSTOMER", "DRIVER"
	ConversationID string // empty for ADMIN
	Conn           *websocket.Conn
	Send           chan []byte
	Hub            *Hub
}

// Hub maintains active client connections and coordinates message routing.
type Hub struct {
	// Active connections mapped by UserID
	clients map[string]*Client

	// Active Admins mapped by UserID for easy broadcasting
	admins map[string]*Client

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	// Database store for message persistence
	store *store.Store

	mu sync.RWMutex
}

// NewHub creates a new Hub.
func NewHub(s *store.Store) *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		admins:     make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		store:      s,
	}
}

// Run starts the Hub main loop to manage registrations/unregistrations.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			// Replace existing connection if any to prevent leaks
			if existing, ok := h.clients[client.UserID]; ok {
				existing.Conn.Close()
				close(existing.Send)
				delete(h.clients, client.UserID)
				delete(h.admins, client.UserID)
			}

			h.clients[client.UserID] = client
			if client.Role == "ADMIN" {
				h.admins[client.UserID] = client
			}
			h.mu.Unlock()
			log.Printf("Client registered: %s (%s, %s)", client.UserID, client.Name, client.Role)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.UserID]; ok {
				client.Conn.Close()
				close(client.Send)
				delete(h.clients, client.UserID)
				delete(h.admins, client.UserID)
				log.Printf("Client unregistered: %s", client.UserID)
			}
			h.mu.Unlock()
		}
	}
}

// Register registers a new client with the Hub.
func (h *Hub) Register(c *Client) {
	h.register <- c
}

// Unregister unregisters a client from the Hub.
func (h *Hub) Unregister(c *Client) {
	h.unregister <- c
}


// RouteMessage handles the real-time routing logic and DB persistence.
func (h *Hub) RouteMessage(sender *Client, rawPayload []byte) {
	// Parse message depending on role
	var incoming struct {
		ConversationID string `json:"conversation_id"`
		Content        string `json:"content"`
	}

	if err := json.Unmarshal(rawPayload, &incoming); err != nil {
		log.Printf("Error unmarshaling socket message: %v", err)
		return
	}

	if incoming.Content == "" {
		return
	}

	var conversationID string
	if sender.Role == "ADMIN" {
		if incoming.ConversationID == "" {
			log.Println("Admin sent message without conversation_id")
			return
		}
		conversationID = incoming.ConversationID
	} else {
		// Customer/Driver messages are routed to their associated conversation
		conversationID = sender.ConversationID
	}

	// 1. Sync insertion to DB
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msg, err := h.store.InsertMessage(ctx, conversationID, sender.UserID, incoming.Content)
	if err != nil {
		log.Printf("Failed to insert message: %v", err)
		return
	}

	// 2. Prepare outgoing payloads
	adminPayload, err := json.Marshal(store.OutgoingMessage{
		ID:             msg.ID,
		ConversationID: msg.ConversationID,
		SenderID:       msg.SenderID,
		SenderName:     sender.Name,
		SenderRole:     sender.Role,
		Content:        msg.Content,
		CreatedAt:      msg.CreatedAt,
	})
	if err != nil {
		log.Printf("Failed to marshal admin message: %v", err)
		return
	}

	// For Customer/Driver client, anonymize Admin name/ID
	var userPayload []byte
	if sender.Role == "ADMIN" {
		userPayload, err = json.Marshal(store.OutgoingMessage{
			ID:             msg.ID,
			ConversationID: msg.ConversationID,
			SenderID:       "", // Anonymized
			SenderName:     "Support Admin",
			SenderRole:     "ADMIN",
			Content:        msg.Content,
			CreatedAt:      msg.CreatedAt,
		})
		if err != nil {
			log.Printf("Failed to marshal user message: %v", err)
			return
		}
	} else {
		userPayload = adminPayload
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	// 3. Routing logic
	if sender.Role != "ADMIN" {
		// Send confirmation back to the sender customer/driver
		if client, online := h.clients[sender.UserID]; online {
			select {
			case client.Send <- userPayload:
			default:
				go func() { h.unregister <- client }()
			}
		}

		// Broadcast to all active Admins
		for _, admin := range h.admins {
			select {
			case admin.Send <- adminPayload:
			default:
				go func() { h.unregister <- admin }()
			}
		}
	} else {
		// Broadcast admin reply to all active Admins (so their dashboards stay in sync)
		for _, admin := range h.admins {
			select {
			case admin.Send <- adminPayload:
			default:
				go func() { h.unregister <- admin }()
			}
		}

		// Look up conversation owner from DB
		ownerID, err := h.store.GetConversationOwner(ctx, conversationID)
		if err != nil {
			log.Printf("Failed to find conversation owner for %s: %v", conversationID, err)
			return
		}

		// Send to the specific customer/driver if online
		if client, online := h.clients[ownerID]; online {
			select {
			case client.Send <- userPayload:
			default:
				go func() { h.unregister <- client }()
			}
		}
	}
}

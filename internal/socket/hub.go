package socket

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/yourusername/go-starter/internal/store"
)

// Client represents a connected user.
type Client struct {
	UserID  string
	Name    string
	Role    string // "ADMIN", "CUSTOMER", "DRIVER"
	Channel string // "DRIVER", "CUSTOMER"
	Conn    *websocket.Conn
	Send    chan []byte
	Hub     *Hub
}

// Hub maintains active client connections and coordinates message routing.
type Hub struct {
	// Active connections mapped by UserID. A single user can have multiple connections (tabs)
	clients map[string]map[*Client]bool

	// All active Admin connections for easy broadcasting
	admins map[*Client]bool

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
		clients:    make(map[string]map[*Client]bool),
		admins:     make(map[*Client]bool),
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
			
			if h.clients[client.UserID] == nil {
				h.clients[client.UserID] = make(map[*Client]bool)
			}
			h.clients[client.UserID][client] = true

			if client.Role == "ADMIN" {
				h.admins[client] = true
			}
			h.mu.Unlock()
			log.Printf("Client registered: %s (%s, %s)", client.UserID, client.Name, client.Role)

		case client := <-h.unregister:
			h.mu.Lock()
			if userConns, ok := h.clients[client.UserID]; ok {
				if _, exists := userConns[client]; exists {
					client.Conn.Close()
					close(client.Send)
					delete(userConns, client)
					if len(userConns) == 0 {
						delete(h.clients, client.UserID)
					}
					
					if client.Role == "ADMIN" {
						delete(h.admins, client)
					}
					log.Printf("Client unregistered: %s", client.UserID)
				}
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

// BroadcastMessage sends a parsed outgoing message to all active admins and the target user.
func (h *Hub) BroadcastMessage(msg store.OutgoingMessage) {
	payload, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal broadcast message: %v", err)
		return
	}

	// Create anonymized payload for customer/driver if sent by ADMIN
	var userPayload []byte
	if msg.SendedBy == "ADMIN" {
		anonymizedMsg := msg
		anonymizedMsg.AdminID = nil // Hide admin ID
		anonymizedMsg.SenderName = "Support Admin"
		
		userPayload, err = json.Marshal(anonymizedMsg)
		if err != nil {
			log.Printf("Failed to marshal user message: %v", err)
			return
		}
	} else {
		userPayload = payload
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	// 1. Broadcast to active Admins (filtered by channel if specified)
	adminCount := 0
	for admin := range h.admins {
		a := admin
		if msg.TargetRole != "" && a.Channel != "" && a.Channel != msg.TargetRole {
			continue // Skip admin tab that belongs to another channel
		}
		select {
		case a.Send <- payload:
			adminCount++
		default:
			go func() { h.unregister <- a }()
		}
	}
	log.Printf("Broadcasted message to %d admin connections", adminCount)

	// 2. Send to the specific customer/driver's active connections (if online)
	if userConns, online := h.clients[msg.UserID]; online {
		for client := range userConns {
			if client.Role == "ADMIN" {
				continue // Already sent to admins above
			}
			if msg.TargetRole != "" && client.Role != msg.TargetRole {
				continue // Do not broadcast to driver if message target_role is CUSTOMER (and vice-versa)
			}
			c := client
			select {
			case c.Send <- userPayload:
				log.Printf("Broadcasted message to customer/driver connection: %s", msg.UserID)
			default:
				go func() { h.unregister <- c }()
			}
		}
	}
}


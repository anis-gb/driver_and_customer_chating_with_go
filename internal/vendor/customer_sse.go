package vendor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yourusername/go-starter/internal/socket"
	"github.com/yourusername/go-starter/internal/store"
	"github.com/yourusername/go-starter/internal/utils" // ← যোগ করুন
)

// CustomerSSEManager manages SSE connections for customers
type CustomerSSEManager struct {
	customerClient *CustomerClient
	dbStore        *store.Store
	hub            *socket.Hub
	listeners      map[string]context.CancelFunc
	processedMsgs  map[string]time.Time
	streamClient   *http.Client
	mu             sync.Mutex
}

// CustomerSSEEvent represents an SSE event from the vendor API for customers
type CustomerSSEEvent struct {
	Type              string `json:"type"`
	MessageID         string `json:"messageId,omitempty"`
	Content           string `json:"content,omitempty"`
	MessageType       string `json:"messageType,omitempty"`
	IsFromPage        bool   `json:"isFromPage,omitempty"`
	PlatformMessageID string `json:"platformMessageId,omitempty"`
	Timestamp         string `json:"timestamp,omitempty"`
	SenderType        string `json:"senderType,omitempty"`
	SenderName        string `json:"senderName,omitempty"`
}

// NewCustomerSSEManager creates a new customer SSE manager
func NewCustomerSSEManager(cc *CustomerClient, s *store.Store, hub *socket.Hub) *CustomerSSEManager {
	manager := &CustomerSSEManager{
		customerClient: cc,
		dbStore:        s,
		hub:            hub,
		listeners:      make(map[string]context.CancelFunc),
		processedMsgs:  make(map[string]time.Time),
		streamClient: &http.Client{
			Timeout: 0, // Infinite timeout for SSE streaming
		},
	}

	// Periodically clean up the deduplication map (keys older than 10 minutes)
	go manager.startDeduplicationCleanup()

	return manager
}

// ============================================================
// DEDUPLICATION CLEANUP
// ============================================================

func (sm *CustomerSSEManager) startDeduplicationCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		sm.mu.Lock()
		now := time.Now()
		for msgID, timestamp := range sm.processedMsgs {
			if now.Sub(timestamp) > 10*time.Minute {
				delete(sm.processedMsgs, msgID)
			}
		}
		sm.mu.Unlock()
	}
}

// ============================================================
// PUBLIC METHODS
// ============================================================

// StartCustomerSSEListener starts a background listener for the specified customer if not already running.
// customerID should be prefixed (e.g., "customer_1")
func (sm *CustomerSSEManager) StartCustomerSSEListener(customerID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.listeners[customerID]; exists {
		log.Printf("[CustomerSSEManager] Listener already exists for customer %s", customerID)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	sm.listeners[customerID] = cancel

	go sm.listenLoop(ctx, customerID)
	log.Printf("[CustomerSSEManager] Started background SSE listener for customer %s", customerID)
}

// StopCustomerSSEListener stops the background listener for the specified customer.
func (sm *CustomerSSEManager) StopCustomerSSEListener(customerID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if cancel, exists := sm.listeners[customerID]; exists {
		cancel()
		delete(sm.listeners, customerID)
		log.Printf("[CustomerSSEManager] Stopped background SSE listener for customer %s", customerID)
	}
}

// AddProcessedMessage adds a vendor message ID to the deduplication map.
func (sm *CustomerSSEManager) AddProcessedMessage(msgID string) {
	if msgID == "" {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.processedMsgs[msgID] = time.Now()
}

// IsListenerActive checks if a listener is active for a customer
func (sm *CustomerSSEManager) IsListenerActive(customerID string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	_, exists := sm.listeners[customerID]
	return exists
}

// GetActiveListenerCount returns the number of active listeners
func (sm *CustomerSSEManager) GetActiveListenerCount() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return len(sm.listeners)
}

// ============================================================
// INTERNAL METHODS
// ============================================================

func (sm *CustomerSSEManager) isDuplicate(msgID string) bool {
	if msgID == "" {
		return false
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.processedMsgs[msgID]; exists {
		return true
	}
	sm.processedMsgs[msgID] = time.Now()
	return false
}

func (sm *CustomerSSEManager) listenLoop(ctx context.Context, customerID string) {
	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second

	for {
		select {
		case <-ctx.Done():
			log.Printf("[CustomerSSEManager] Listen loop stopped for customer %s", customerID)
			return
		default:
		}

		err := sm.readStream(ctx, customerID)
		if err != nil {
			log.Printf("[CustomerSSEManager] Stream read error for customer %s: %v. Reconnecting in %v...", customerID, err, backoff)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
			// Exponential backoff
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (sm *CustomerSSEManager) readStream(ctx context.Context, customerID string) error {
	// Get session token for customer (customerID is prefixed, which vendor expects)
	token, err := sm.customerClient.GetSessionToken(ctx, customerID)
	if err != nil {
		return fmt.Errorf("failed to get customer session token: %w", err)
	}

	// Build stream URL with token
	url := fmt.Sprintf("%s/stream?token=%s", sm.customerClient.apiURL, token)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create stream request: %w", err)
	}

	// Set headers for SSE
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	// Make the request
	resp, err := sm.streamClient.Do(req)
	if err != nil {
		return fmt.Errorf("stream request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stream request returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	log.Printf("[CustomerSSEManager] Stream connected for customer %s", customerID)

	// Read SSE events
	reader := bufio.NewReader(resp.Body)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[CustomerSSEManager] Stream context cancelled for customer %s", customerID)
			return nil
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				log.Printf("[CustomerSSEManager] Stream EOF for customer %s", customerID)
				return nil
			}
			return fmt.Errorf("error reading stream line: %w", err)
		}

		line = strings.TrimSpace(line)

		// Parse SSE data events
		if strings.HasPrefix(line, "data:") {
			dataStr := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if dataStr == "" {
				continue
			}

			var event CustomerSSEEvent
			if err := json.Unmarshal([]byte(dataStr), &event); err != nil {
				log.Printf("[CustomerSSEManager] Failed to unmarshal SSE event: %v. Data: %s", err, dataStr)
				continue
			}

			// Process message events
			if event.Type == "message" {
				// Deduplicate using message ID
				if sm.isDuplicate(event.MessageID) {
					log.Printf("[CustomerSSEManager] Duplicate message ignored: %s", event.MessageID)
					continue
				}

				// Check if this is a message from admin/business (isFromPage = true)
				if event.IsFromPage {
					log.Printf("[CustomerSSEManager] Received admin reply for customer %s: %s", customerID, event.Content)
					sm.processCustomerMessage(ctx, customerID, event)
				} else {
					log.Printf("[CustomerSSEManager] Received customer message for customer %s: %s (ignoring - already in DB)", customerID, event.Content)
				}
			}
		}
	}
}

// processCustomerMessage processes an incoming admin/bot reply from vendor and saves it to DB
func (sm *CustomerSSEManager) processCustomerMessage(ctx context.Context, customerID string, event CustomerSSEEvent) {
	log.Printf("[CustomerSSEManager] Processing admin message for customer %s: %s", customerID, event.Content)

	// 🔥 customerID is prefixed (e.g., "customer_1") – parse to get original ID
	_, originalID := utils.ParseVendorUserID(customerID)
	if originalID == "" {
		log.Printf("[CustomerSSEManager] Invalid customer ID format: %s", customerID)
		return
	}
	log.Printf("[CustomerSSEManager] Parsed original customer ID: %s", originalID)

	// Extract media URLs if present
	var voicePath, photoPath, filePath string
	if event.MessageType == "IMAGE" || event.MessageType == "PHOTO" {
		photoPath = event.Content
	} else if event.MessageType == "AUDIO" || event.MessageType == "VOICE" {
		voicePath = event.Content
	} else if event.MessageType == "FILE" || event.MessageType == "VIDEO" || event.MessageType == "DOCUMENT" {
		filePath = event.Content
	}

	// Determine sender
	adminID := "SYSTEM_AI"
	sendedBy := "ADMIN"

	if event.SenderType == "CUSTOMER" {
		sendedBy = "CUSTOMER"
		adminID = ""
	}

	// Get sender name
	senderName := "System Ai"
	if event.SenderName != "" {
		senderName = event.SenderName
	}

	// Save message to database using originalID (without prefix)
	msg, err := sm.dbStore.InsertCustomerMessage(
		ctx,
		originalID, // ← original customer ID
		&adminID,
		sendedBy,
		event.Content,
		voicePath,
		photoPath,
		filePath,
		"", // userPhone - will be populated from customer profile
		senderName,
		"", // profilePicture - will be populated from customer profile
		"", // gender - will be populated from customer profile
	)
	if err != nil {
		log.Printf("[CustomerSSEManager] Failed to save customer message to database: %v", err)
		return
	}

	log.Printf("[CustomerSSEManager] Successfully saved message %s for customer %s (original ID: %s)", msg.ID, customerID, originalID)

	// Broadcast to WebSocket Hub
	outgoingMsg := store.OutgoingMessage{
		Type:           "NEW_MESSAGE",
		ID:             msg.ID,
		UserID:         msg.UserID, // original ID from DB
		AdminID:        msg.AdminID,
		SendedBy:       msg.SendedBy,
		SenderName:     senderName,
		Content:        msg.Content,
		Seen:           msg.Seen,
		VoiceMessages:  msg.VoiceMessages,
		Photo:          msg.Photo,
		File:           msg.File,
		UserPhone:      msg.UserPhone,
		FullName:       msg.FullName,
		ProfilePicture: msg.ProfilePicture,
		Gender:         msg.Gender,
		CreatedAt:      msg.CreatedAt,
	}
	sm.hub.BroadcastMessage(outgoingMsg)
	log.Printf("[CustomerSSEManager] Broadcasted message %s to WebSocket hub", msg.ID)
}

// SendCustomerNotification sends a notification to the customer
func (sm *CustomerSSEManager) SendCustomerNotification(customerID, message string) error {
	log.Printf("[CustomerSSEManager] Sending notification to customer %s: %s", customerID, message)
	return nil
}

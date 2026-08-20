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
)

type SSEManager struct {
	vendorClient *VendorClient
	dbStore      *store.Store
	hub          *socket.Hub
	listeners    map[string]context.CancelFunc
	processedMsgs map[string]time.Time
	streamClient *http.Client
	mu           sync.Mutex
}

type SSEEvent struct {
	Type              string `json:"type"`
	MessageID         string `json:"messageId,omitempty"`
	Content           string `json:"content,omitempty"`
	MessageType       string `json:"messageType,omitempty"`
	IsFromPage        bool   `json:"isFromPage,omitempty"`
	PlatformMessageID string `json:"platformMessageId,omitempty"`
	Timestamp         string `json:"timestamp,omitempty"`
}

func NewSSEManager(vc *VendorClient, s *store.Store, hub *socket.Hub) *SSEManager {
	manager := &SSEManager{
		vendorClient:  vc,
		dbStore:       s,
		hub:           hub,
		listeners:     make(map[string]context.CancelFunc),
		processedMsgs: make(map[string]time.Time),
		streamClient: &http.Client{
			Timeout: 0, // Infinite timeout for SSE streaming
		},
	}

	// Periodically clean up the deduplication map (keys older than 10 minutes)
	go manager.startDeduplicationCleanup()

	return manager
}

func (sm *SSEManager) startDeduplicationCleanup() {
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

// StartSSEListener starts a background listener for the specified driver if not already running.
func (sm *SSEManager) StartSSEListener(driverID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.listeners[driverID]; exists {
		// Listener already active for this driver
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	sm.listeners[driverID] = cancel

	go sm.listenLoop(ctx, driverID)
	log.Printf("[SSEManager] Started background SSE listener for driver %s", driverID)
}

// StopSSEListener stops the background listener for the specified driver.
func (sm *SSEManager) StopSSEListener(driverID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if cancel, exists := sm.listeners[driverID]; exists {
		cancel()
		delete(sm.listeners, driverID)
		log.Printf("[SSEManager] Stopped background SSE listener for driver %s", driverID)
	}
}

func (sm *SSEManager) isDuplicate(msgID string) bool {
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

func (sm *SSEManager) listenLoop(ctx context.Context, driverID string) {
	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := sm.readStream(ctx, driverID)
		if err != nil {
			log.Printf("[SSEManager] Stream read error for driver %s: %v. Reconnecting in %v...", driverID, err, backoff)
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

func (sm *SSEManager) readStream(ctx context.Context, driverID string) error {
	token, err := sm.vendorClient.GetSessionToken(ctx, driverID)
	if err != nil {
		return fmt.Errorf("failed to get token for stream: %w", err)
	}

	// Use query parameter as per documentation for EventSource clients
	url := fmt.Sprintf("%s/stream?token=%s", sm.vendorClient.apiURL, token)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create stream request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	resp, err := sm.streamClient.Do(req)
	if err != nil {
		return fmt.Errorf("stream request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stream request returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	log.Printf("[SSEManager] Stream connected for driver %s", driverID)
	reader := bufio.NewReader(resp.Body)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("error reading stream line: %w", err)
		}

		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			dataStr := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if dataStr == "" {
				continue
			}

			var event SSEEvent
			if err := json.Unmarshal([]byte(dataStr), &event); err != nil {
				log.Printf("[SSEManager] Failed to unmarshal SSE event: %v. Data: %s", err, dataStr)
				continue
			}

			// We are only interested in events that represent messages sent by the business/bot (isFromPage == true)
			if event.Type == "message" && event.IsFromPage {
				// Deduplicate using message ID
				if sm.isDuplicate(event.MessageID) {
					continue
				}

				log.Printf("[SSEManager] Received business reply for driver %s: %s", driverID, event.Content)

				// Save message in PostgreSQL database
				adminID := "SYSTEM_AI"
				sendedBy := "ADMIN"
				
				// Optional: extract photo, file, or audio if the vendor returns a media URL
				var voicePath, photoPath, filePath string
				if event.MessageType == "IMAGE" {
					photoPath = event.Content
				} else if event.MessageType == "AUDIO" {
					voicePath = event.Content
				} else if event.MessageType == "FILE" || event.MessageType == "VIDEO" {
					filePath = event.Content
				}

				msg, err := sm.dbStore.InsertDriverMessage(
					ctx,
					driverID,
					&adminID,
					sendedBy,
					event.Content,
					voicePath,
					photoPath,
					filePath,
					"", // userPhone
					"Support AI", // fullName
					"", // profilePicture
					"", // gender
				)
				if err != nil {
					log.Printf("[SSEManager] Failed to save business message to database: %v", err)
					continue
				}

				// Broadcast to WebSocket Hub
				outgoingMsg := store.OutgoingMessage{
					Type:           "NEW_MESSAGE",
					ID:             msg.ID,
					UserID:         msg.UserID,
					AdminID:        msg.AdminID,
					SendedBy:       msg.SendedBy,
					SenderName:     "Support Admin", // show as Support Admin to match client expectation
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
			}
		}
	}
}

// AddProcessedMessage adds a vendor message ID to the deduplication map.
func (sm *SSEManager) AddProcessedMessage(msgID string) {
	if msgID == "" {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.processedMsgs[msgID] = time.Now()
}


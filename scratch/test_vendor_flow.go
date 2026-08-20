package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/yourusername/go-starter/internal/config"
	"github.com/yourusername/go-starter/internal/database"
	"github.com/yourusername/go-starter/internal/socket"
	"github.com/yourusername/go-starter/internal/store"
	"github.com/yourusername/go-starter/internal/vendor"
)

type MockVendor struct {
	mu         sync.Mutex
	botEnabled bool
	clients    map[string]chan string
}

func main() {
	log.Println("=== STARTING INTEGRATION TEST ===")

	// 1. Initialize Mock Vendor
	mock := &MockVendor{
		botEnabled: true,
		clients:    make(map[string]chan string),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"token":     "mock-jwt-token-123",
				"expiresIn": 1800,
			})
			return
		}

		if r.URL.Path == "/message" {
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			log.Printf("[Mock Vendor] Received message content: %s", body["content"])
			w.WriteHeader(http.StatusCreated)

			mock.mu.Lock()
			bot := mock.botEnabled
			mock.mu.Unlock()

			if bot {
				// Simulate vendor AI bot response event after 100ms
				go func() {
					time.Sleep(100 * time.Millisecond)
					reply := map[string]interface{}{
						"type":              "message",
						"messageId":         fmt.Sprintf("msg-bot-%d", time.Now().UnixNano()),
						"content":           "Hello, I am the Support AI! How can I help you?",
						"messageType":       "TEXT",
						"isFromPage":        true,
						"platformMessageId": "plat-bot-123",
						"timestamp":         time.Now().Format(time.RFC3339),
					}
					dataBytes, _ := json.Marshal(reply)
					mock.mu.Lock()
					for _, ch := range mock.clients {
						select {
						case ch <- string(dataBytes):
						default:
						}
					}
					mock.mu.Unlock()
				}()
			}
			return
		}

		if r.URL.Path == "/stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.WriteHeader(http.StatusOK)

			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
				return
			}

			ch := make(chan string, 10)
			clientID := fmt.Sprintf("%d", time.Now().UnixNano())

			mock.mu.Lock()
			mock.clients[clientID] = ch
			mock.mu.Unlock()

			defer func() {
				mock.mu.Lock()
				delete(mock.clients, clientID)
				close(ch)
				mock.mu.Unlock()
			}()

			// Send connected event
			_, _ = fmt.Fprint(w, "data: {\"type\":\"connected\",\"appId\":\"app_test\",\"endUserId\":\"driver_test_99\"}\n\n")
			flusher.Flush()

			for {
				select {
				case <-r.Context().Done():
					return
				case data := <-ch:
					_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
					flusher.Flush()
				}
			}
		}

		if r.URL.Path == "/agent/bot" {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			enabled := body["enabled"].(bool)
			log.Printf("[Mock Vendor] AI Bot toggle received. Enabled: %v", enabled)

			mock.mu.Lock()
			mock.botEnabled = enabled
			mock.mu.Unlock()

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"conversationWithId": "driver_test_99",
				"botEnabled":         enabled,
			})
			return
		}
	}))
	defer server.Close()
	log.Printf("Mock Vendor Server running at: %s", server.URL)

	// 2. Database Connection
	cfg := config.Load()
	db, err := database.NewPostgresPool(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()
	log.Println("Connected to PostgreSQL")

	// 3. Initialize Store & Hub
	s := store.NewStore(db)
	hub := socket.NewHub(s)
	go hub.Run()

	// Clean up old settings/messages for the test user
	ctx := context.Background()
	_, _ = db.Exec(ctx, "DELETE FROM driver_messages WHERE user_id = $1", "driver_test_99")
	_, _ = db.Exec(ctx, "DELETE FROM driver_ai_settings WHERE user_id = $1", "driver_test_99")

	// Ensure tables exist
	err = s.EnsureAISettingsTable(ctx)
	if err != nil {
		log.Fatalf("EnsureAISettingsTable failed: %v", err)
	}

	// 4. Initialize VendorClient and SSEManager
	vc := vendor.NewVendorClient(server.URL, "sk_live_test_secret_key")
	sse := vendor.NewSSEManager(vc, s, hub)

	// 5. Test Step 1: Start SSE listener for test driver
	sse.StartSSEListener("driver_test_99")
	time.Sleep(200 * time.Millisecond) // Let connection establish

	// 6. Test Step 2: Insert driver message locally & forward it to vendor
	log.Println("[Test] Driver sending message: 'Where is my passenger?'")
	msg, err := s.InsertDriverMessage(ctx, "driver_test_99", nil, "DRIVER", "Where is my passenger?", "", "", "", "", "Driver Dave", "", "")
	if err != nil {
		log.Fatalf("Failed to save driver message: %v", err)
	}
	log.Printf("[Test] Saved driver message ID: %s", msg.ID)

	err = vc.ForwardMessage(ctx, "driver_test_99", "Where is my passenger?")
	if err != nil {
		log.Fatalf("ForwardMessage failed: %v", err)
	}

	// 7. Wait and verify that vendor AI bot replied and message is stored locally
	log.Println("[Test] Waiting for AI bot reply event to trigger...")
	time.Sleep(500 * time.Millisecond)

	// Retrieve driver messages history
	history, err := s.GetDriverHistory(ctx, "driver_test_99", time.Time{}, 10)
	if err != nil {
		log.Fatalf("GetDriverHistory failed: %v", err)
	}

	log.Printf("[Test] Retrieved history length: %d", len(history))
	foundAI := false
	for _, m := range history {
		log.Printf("  Message: [%s] -> %s (Sender: %s)", m.SendedBy, m.Content, m.SenderName)
		if m.SendedBy == "ADMIN" && m.Content == "Hello, I am the Support AI! How can I help you?" {
			foundAI = true
		}
	}

	if !foundAI {
		log.Fatalf("FAIL: AI bot reply was not processed, saved, or sync'ed in local DB")
	}
	log.Println("SUCCESS: Driver message forwarded and AI bot reply successfully written to Postgres!")

	// 8. Test Step 3: Toggle bot to OFF
	log.Println("[Test] Toggling AI bot to OFF...")
	err = s.SetDriverAISetting(ctx, "driver_test_99", false)
	if err != nil {
		log.Fatalf("SetDriverAISetting failed: %v", err)
	}
	err = vc.ToggleVendorBot(ctx, "driver_test_99", false)
	if err != nil {
		log.Fatalf("ToggleVendorBot failed: %v", err)
	}

	// Verify vendor bot toggle was received
	mock.mu.Lock()
	toggleState := mock.botEnabled
	mock.mu.Unlock()
	if toggleState {
		log.Fatalf("FAIL: Mock vendor did not receive bot toggle OFF")
	}
	log.Println("SUCCESS: AI Bot successfully toggled to OFF locally and synced with vendor!")

	// Send another message when bot is OFF
	log.Println("[Test] Driver sending second message when bot is OFF: 'Hello?'")
	_, err = s.InsertDriverMessage(ctx, "driver_test_99", nil, "DRIVER", "Hello?", "", "", "", "", "Driver Dave", "", "")
	if err != nil {
		log.Fatalf("Failed to save second driver message: %v", err)
	}
	err = vc.ForwardMessage(ctx, "driver_test_99", "Hello?")
	if err != nil {
		log.Fatalf("ForwardMessage failed: %v", err)
	}

	// Wait to ensure no bot replies are generated
	time.Sleep(300 * time.Millisecond)

	history2, _ := s.GetDriverHistory(ctx, "driver_test_99", time.Time{}, 10)
	aiReplyCount := 0
	for _, m := range history2 {
		if m.SendedBy == "ADMIN" && m.Content == "Hello, I am the Support AI! How can I help you?" {
			aiReplyCount++
		}
	}

	if aiReplyCount > 1 {
		log.Fatalf("FAIL: Bot responded when it was supposed to be toggled OFF")
	}
	log.Println("SUCCESS: No extra AI bot responses generated while muted!")

	// Clean up listeners
	sse.StopSSEListener("driver_test_99")

	log.Println("=== ALL INTEGRATION TESTS PASSED SUCCESSFULLY! ===")
}

package handler

import (
	"log"
	"net/http"
	"time"

	"github.com/yourusername/go-starter/internal/store"
	"github.com/yourusername/go-starter/pkg/response"
)

// HistoryHandler handles fetching previous messages.
type HistoryHandler struct {
	store *store.Store
}

// NewHistoryHandler creates a new HistoryHandler.
func NewHistoryHandler(s *store.Store) *HistoryHandler {
	return &HistoryHandler{store: s}
}

// GetHistory fetches the chat history for a customer/driver.
// GET /api/messages?user_id=...&requester_id=...&cursor=...
func (h *HistoryHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	targetUserID := r.URL.Query().Get("user_id")
	if targetUserID == "" {
		response.JSON(w, http.StatusBadRequest, "user_id query parameter is required", nil)
		return
	}

	// Determine who is requesting the history (defaults to targetUserID if empty)
	requesterID := r.URL.Query().Get("requester_id")
	if requesterID == "" {
		requesterID = targetUserID
	}

	// 1. Fetch requester to check role for anonymization
	requester, err := h.store.GetUserByID(r.Context(), requesterID)
	if err != nil {
		response.JSON(w, http.StatusUnauthorized, "requester not found", nil)
		return
	}

	// 2. Find target user's conversation
	conversationID, err := h.store.GetOrCreateConversation(r.Context(), targetUserID)
	if err != nil {
		log.Printf("Failed to get/create conversation for user %s: %v", targetUserID, err)
		response.JSON(w, http.StatusInternalServerError, "failed to load conversation", nil)
		return
	}

	// 3. Parse optional cursor (timestamp in RFC3339)
	var cursorTime time.Time
	cursorStr := r.URL.Query().Get("cursor")
	if cursorStr != "" {
		var parseErr error
		cursorTime, parseErr = time.Parse(time.RFC3339, cursorStr)
		if parseErr != nil {
			response.JSON(w, http.StatusBadRequest, "invalid cursor format (must be RFC3339)", nil)
			return
		}
	}

	// 4. Fetch up to 50 older messages
	messages, err := h.store.GetChatHistory(r.Context(), conversationID, cursorTime, 50)
	if err != nil {
		log.Printf("Failed to fetch chat history: %v", err)
		response.JSON(w, http.StatusInternalServerError, "failed to fetch messages", nil)
		return
	}

	if messages == nil {
		messages = []store.OutgoingMessage{}
	}

	// 5. Apply Admin Anonymization Rule
	if requester.Role != "ADMIN" {
		for i, m := range messages {
			if m.SenderRole == "ADMIN" {
				messages[i].SenderID = ""
				messages[i].SenderName = "Support Admin"
			}
		}
	}

	response.JSON(w, http.StatusOK, "", messages)
}

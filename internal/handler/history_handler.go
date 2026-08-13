package handler

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/yourusername/go-starter/internal/store"
	"github.com/yourusername/go-starter/pkg/response"
)

// HistoryHandler handles fetching message history for conversations.
type HistoryHandler struct {
	store *store.Store
}

// NewHistoryHandler creates a new HistoryHandler.
func NewHistoryHandler(s *store.Store) *HistoryHandler {
	return &HistoryHandler{store: s}
}

// GetHistory fetches message history with cursor-based pagination.
// GET /api/messages?user_id=...&conversation_id=...&cursor=...&limit=...
func (h *HistoryHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		response.JSON(w, http.StatusBadRequest, "user_id query parameter is required", nil)
		return
	}

	// 1. Fetch requesting user from DB to verify role
	user, err := h.store.GetUserByID(r.Context(), userID)
	if err != nil {
		response.JSON(w, http.StatusUnauthorized, "user not found", nil)
		return
	}

	// 2. Determine target conversation_id
	conversationID := r.URL.Query().Get("conversation_id")
	if user.Role == "ADMIN" {
		if conversationID == "" {
			response.JSON(w, http.StatusBadRequest, "conversation_id query parameter is required for admins", nil)
			return
		}
	} else {
		// If non-admin and conversation_id is not supplied, fetch/create user's conversation
		if conversationID == "" {
			conversationID, err = h.store.GetOrCreateConversation(r.Context(), user.ID)
			if err != nil {
				log.Printf("Failed to get/create conversation for user %s: %v", user.ID, err)
				response.JSON(w, http.StatusInternalServerError, "failed to load conversation", nil)
				return
			}
		}
	}

	// 3. Parse optional cursor (RFC3339 timestamp)
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

	// 4. Parse optional limit (default 50)
	limit := 50
	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// 5. Fetch older messages from store
	messages, err := h.store.GetChatHistory(r.Context(), conversationID, cursorTime, limit)
	if err != nil {
		log.Printf("Failed to fetch chat history for conversation %s: %v", conversationID, err)
		response.JSON(w, http.StatusInternalServerError, "failed to fetch messages", nil)
		return
	}

	if messages == nil {
		messages = []store.OutgoingMessage{}
	}

	// 6. Apply Admin Anonymization Rule for Customer/Driver requests
	if user.Role != "ADMIN" {
		for i, m := range messages {
			if m.SenderRole == "ADMIN" {
				messages[i].SenderID = ""
				messages[i].SenderName = "Support Admin"
			}
		}
	}

	response.RawJSON(w, http.StatusOK, messages)
}

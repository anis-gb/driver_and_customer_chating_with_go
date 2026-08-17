package customer

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/yourusername/go-starter/internal/store"
	"github.com/yourusername/go-starter/pkg/response"
)

// HistoryHandler handles fetching message history.
type HistoryHandler struct {
	store *store.Store
}

// NewHistoryHandler creates a new HistoryHandler.
func NewHistoryHandler(s *store.Store) *HistoryHandler {
	return &HistoryHandler{store: s}
}

func (h *HistoryHandler) GetCustomerHistory(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		response.JSON(w, http.StatusBadRequest, "user_id query parameter is required", nil)
		return
	}

	userType := r.URL.Query().Get("user_type")
	if userType == "" {
		userType = "CUSTOMER"
	}

	// Determine target user_id for the chat history
	var targetUserID string
	if userType == "ADMIN" {
		targetUserID = r.URL.Query().Get("target_user_id")
		if targetUserID == "" {
			response.JSON(w, http.StatusBadRequest, "target_user_id query parameter is required for admins", nil)
			return
		}
	} else {
		// If non-admin, they can only fetch their own chat history
		targetUserID = userID
		// Enforce alignment of sender type with endpoint channel
		userType = "CUSTOMER"
	}

	// Parse optional cursor (RFC3339 timestamp)
	var cursorTime time.Time
	cursorStr := r.URL.Query().Get("cursor")
	if cursorStr == "" {
		cursorStr = r.URL.Query().Get("before")
	}
	if cursorStr != "" {
		var parseErr error
		cursorTime, parseErr = time.Parse(time.RFC3339, cursorStr)
		if parseErr != nil {
			response.JSON(w, http.StatusBadRequest, "invalid cursor format (must be RFC3339)", nil)
			return
		}
	}

	// Parse optional limit (default 50)
	limit := 50
	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Fetch older messages from store
	rawMessages, err := h.store.GetCustomerHistory(r.Context(), targetUserID, cursorTime, limit)
	if err != nil {
		log.Printf("Failed to fetch chat history for user %s: %v", targetUserID, err)
		response.JSON(w, http.StatusInternalServerError, "failed to fetch messages", nil)
		return
	}

	if rawMessages == nil {
		rawMessages = []store.OutgoingMessage{}
	}

	// Apply Admin Anonymization Rule for Customer requests
	if userType != "ADMIN" {
		for i, m := range rawMessages {
			if m.SendedBy == "ADMIN" {
				rawMessages[i].AdminID = nil
				rawMessages[i].SenderName = "Support Admin"
			}
		}
	}

	// Calculate pagination metadata (has_more, next_cursor)
	hasMore := false
	nextCursor := ""
	messagesList := rawMessages

	if len(rawMessages) > limit {
		hasMore = true
		// The next_cursor is the timestamp of the last message in the requested page
		nextCursor = rawMessages[limit-1].CreatedAt.Format(time.RFC3339)
		messagesList = rawMessages[:limit]
	}

	// Return paginated response payload
	response.RawJSON(w, http.StatusOK, map[string]any{
		"messages":    messagesList,
		"next_cursor": nextCursor,
		"has_more":    hasMore,
	})
}

package customer

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/yourusername/go-starter/internal/store"
	"github.com/yourusername/go-starter/pkg/response"
)

// HistoryHandler handles fetching customer message history.
type HistoryHandler struct {
	store *store.Store
}

// NewHistoryHandler creates a new HistoryHandler.
func NewHistoryHandler(s *store.Store) *HistoryHandler {
	return &HistoryHandler{
		store: s,
	}
}

// GetCustomerHistory returns paginated customer chat history.
func (h *HistoryHandler) GetCustomerHistory(
	w http.ResponseWriter,
	r *http.Request,
) {
	// -------------------------------------------------
	// User ID
	// -------------------------------------------------
	userID := r.URL.Query().Get("user_id")

	if userID == "" {
		response.JSON(
			w,
			http.StatusBadRequest,
			"user_id query parameter is required",
			nil,
		)
		return
	}

	// -------------------------------------------------
	// User type
	// -------------------------------------------------
	userType := r.URL.Query().Get("user_type")

	if userType == "" {
		userType = "CUSTOMER"
	}

	// Normalize user type.
	if userType != "ADMIN" {
		userType = "CUSTOMER"
	}

	// -------------------------------------------------
	// Determine target user ID
	// -------------------------------------------------
	var targetUserID string

	if userType == "ADMIN" {

		// Admin can view a specific customer's conversation.
		targetUserID = r.URL.Query().Get("target_user_id")

		if targetUserID == "" {
			response.JSON(
				w,
				http.StatusBadRequest,
				"target_user_id query parameter is required for admins",
				nil,
			)
			return
		}

	} else {

		// Customer can only view their own conversation.
		targetUserID = userID
	}

	// -------------------------------------------------
	// Cursor
	// -------------------------------------------------
	var cursorTime time.Time

	cursorStr := r.URL.Query().Get("cursor")

	if cursorStr == "" {
		cursorStr = r.URL.Query().Get("before")
	}

	if cursorStr != "" {

		parsedTime, err := time.Parse(
			time.RFC3339,
			cursorStr,
		)

		if err != nil {
			response.JSON(
				w,
				http.StatusBadRequest,
				"invalid cursor format (must be RFC3339)",
				nil,
			)
			return
		}

		cursorTime = parsedTime
	}

	// -------------------------------------------------
	// Limit
	// -------------------------------------------------
	limit := 50

	limitStr := r.URL.Query().Get("limit")

	if limitStr != "" {

		parsedLimit, err := strconv.Atoi(limitStr)

		if err != nil || parsedLimit <= 0 {
			response.JSON(
				w,
				http.StatusBadRequest,
				"limit must be a positive integer",
				nil,
			)
			return
		}

		// Maximum 100 messages per request.
		if parsedLimit > 100 {
			parsedLimit = 100
		}

		limit = parsedLimit
	}

	// -------------------------------------------------
	// Get customer messages
	// -------------------------------------------------
	rawMessages, err := h.store.GetCustomerHistory(
		r.Context(),
		targetUserID,
		cursorTime,
		limit,
	)

	if err != nil {
		log.Printf(
			"Failed to fetch customer chat history for user %s: %v",
			targetUserID,
			err,
		)

		response.JSON(
			w,
			http.StatusInternalServerError,
			"failed to fetch messages",
			nil,
		)
		return
	}

	// Prevent null response.
	if rawMessages == nil {
		rawMessages = []store.OutgoingMessage{}
	}

	// -------------------------------------------------
	// Hide admin ID from customers
	// -------------------------------------------------
	if userType != "ADMIN" {

		for i := range rawMessages {

			if rawMessages[i].SendedBy == "ADMIN" {
				rawMessages[i].AdminID = nil
				rawMessages[i].SenderName = "Support Admin"
			}
		}
	}

	// -------------------------------------------------
	// Pagination
	// -------------------------------------------------
	hasMore := false
	nextCursor := ""

	messagesList := rawMessages

	// Store returns limit + 1 records.
	if len(rawMessages) > limit {

		hasMore = true

		// Last message of current page.
		nextCursor = rawMessages[limit-1].CreatedAt.Format(
			time.RFC3339,
		)

		messagesList = rawMessages[:limit]
	}

	// -------------------------------------------------
	// Response
	// -------------------------------------------------
	response.RawJSON(
		w,
		http.StatusOK,
		map[string]any{
			"messages":    messagesList,
			"next_cursor": nextCursor,
			"has_more":    hasMore,
		},
	)
}

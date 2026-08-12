package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/yourusername/go-starter/internal/store"
	"github.com/yourusername/go-starter/pkg/response"
)

// MessageHandler handles HTTP requests for messages.
type MessageHandler struct {
	store *store.MessageStore
}

// NewMessageHandler creates a new MessageHandler.
func NewMessageHandler(s *store.MessageStore) *MessageHandler {
	return &MessageHandler{store: s}
}

// CreateMessage godoc
// POST /api/v1/messages
// Accepts a JSON body with sender_id and content, persists it, and returns the created message.
func (h *MessageHandler) CreateMessage(w http.ResponseWriter, r *http.Request) {
	var params store.CreateMessageParams

	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		response.JSON(w, http.StatusBadRequest, "invalid request body", nil)
		return
	}

	if params.Content == "" {
		response.JSON(w, http.StatusUnprocessableEntity, "content is required", nil)
		return
	}
	if params.SenderID == 0 {
		response.JSON(w, http.StatusUnprocessableEntity, "sender_id is required", nil)
		return
	}

	msg, err := h.store.Create(r.Context(), params)
	if err != nil {
		log.Printf("ERROR CreateMessage: %v", err)
		response.JSON(w, http.StatusInternalServerError, "failed to create message", nil)
		return
	}

	response.JSON(w, http.StatusCreated, "message created", msg)
}

// ListMessages godoc
// GET /api/v1/messages
// Returns all messages ordered by newest first.
func (h *MessageHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	messages, err := h.store.List(r.Context())
	if err != nil {
		log.Printf("ERROR ListMessages: %v", err)
		response.JSON(w, http.StatusInternalServerError, "failed to fetch messages", nil)
		return
	}

	// Return an empty array instead of null when there are no messages.
	if messages == nil {
		messages = []store.Message{}
	}

	response.JSON(w, http.StatusOK, "", messages)
}

// GetMessage godoc
// GET /api/v1/messages/{id}
// Returns a single message by ID.
func (h *MessageHandler) GetMessage(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.JSON(w, http.StatusBadRequest, "invalid message id", nil)
		return
	}

	msg, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.JSON(w, http.StatusNotFound, "message not found", nil)
			return
		}
		log.Printf("ERROR GetMessage: %v", err)
		response.JSON(w, http.StatusInternalServerError, "failed to fetch message", nil)
		return
	}

	response.JSON(w, http.StatusOK, "", msg)
}



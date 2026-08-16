package handler

import (
	"net/http"

	"github.com/yourusername/go-starter/internal/socket"
	"github.com/yourusername/go-starter/internal/store"
	"github.com/yourusername/go-starter/pkg/response"
)

type MessageHandler struct {
	store *store.Store
	hub   *socket.Hub
}

func NewMessageHandler(s *store.Store, h *socket.Hub) *MessageHandler {
	return &MessageHandler{
		store: s,
		hub:   h,
	}
}

func (h *MessageHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form data with a max memory of 10MB
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		if err := r.ParseForm(); err != nil {
			response.JSON(w, http.StatusBadRequest, "failed to parse form data", nil)
			return
		}
	}

	// Authenticated User ID (the sender) is now expected in the form data
	authUserID := r.FormValue("user_id")
	if authUserID == "" {
		response.JSON(w, http.StatusBadRequest, "user_id form field is required (sender)", nil)
		return
	}

	sender, err := h.store.GetUserByID(r.Context(), authUserID)
	if err != nil {
		response.JSON(w, http.StatusUnauthorized, "invalid sender user_id", nil)
		return
	}

	content := r.FormValue("content")
	if content == "" {
		response.JSON(w, http.StatusBadRequest, "content cannot be empty", nil)
		return
	}

	var targetUserID string
	var adminID *string

	if sender.Role == "ADMIN" {
		targetUserID = r.FormValue("target_user_id")
		if targetUserID == "" {
			response.JSON(w, http.StatusBadRequest, "target_user_id form field is required when admin sends a message", nil)
			return
		}
		adminID = &sender.ID
	} else {
		// Customers and drivers always send messages to their own chat thread
		targetUserID = sender.ID
		adminID = nil
	}

	// Persist the message
	msg, err := h.store.InsertMessage(r.Context(), targetUserID, adminID, sender.Role, content)
	if err != nil {
		response.JSON(w, http.StatusInternalServerError, "failed to save message", nil)
		return
	}

	// Broadcast via WebSocket
	outgoingMsg := store.OutgoingMessage{
		ID:         msg.ID,
		UserID:     msg.UserID,
		AdminID:    msg.AdminID,
		SendedBy:   msg.SendedBy,
		SenderName: sender.Name, // This gets anonymized inside BroadcastMessage if needed
		Content:    msg.Content,
		Seen:       msg.Seen,
		CreatedAt:  msg.CreatedAt,
	}
	
	h.hub.BroadcastMessage(outgoingMsg)

	response.JSON(w, http.StatusCreated, "message sent", outgoingMsg)
}

func (h *MessageHandler) MarkMessagesSeen(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		if err := r.ParseForm(); err != nil {
			response.JSON(w, http.StatusBadRequest, "failed to parse form data", nil)
			return
		}
	}

	authUserID := r.FormValue("user_id")
	if authUserID == "" {
		response.JSON(w, http.StatusBadRequest, "user_id form field is required", nil)
		return
	}

	viewer, err := h.store.GetUserByID(r.Context(), authUserID)
	if err != nil {
		response.JSON(w, http.StatusUnauthorized, "invalid user_id", nil)
		return
	}

	var targetUserID string
	if viewer.Role == "ADMIN" {
		targetUserID = r.FormValue("target_user_id")
		if targetUserID == "" {
			response.JSON(w, http.StatusBadRequest, "target_user_id form field is required for admins", nil)
			return
		}
	} else {
		targetUserID = viewer.ID
	}

	if err := h.store.MarkMessagesAsSeen(r.Context(), targetUserID, viewer.Role); err != nil {
		response.JSON(w, http.StatusInternalServerError, "failed to update messages", nil)
		return
	}

	response.JSON(w, http.StatusOK, "messages marked as seen", nil)
}


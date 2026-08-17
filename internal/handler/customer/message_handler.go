package customer

import (
	"net/http"

	"github.com/go-chi/chi/v5"
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

// SendCustomerMessage sends and stores a customer/admin message.
func (h *MessageHandler) SendCustomerMessage(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form data.
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		if err := r.ParseForm(); err != nil {
			response.JSON(
				w,
				http.StatusBadRequest,
				"failed to parse form data",
				nil,
			)
			return
		}
	}

	// Sender ID.
	authUserID := r.FormValue("user_id")
	if authUserID == "" {
		response.JSON(
			w,
			http.StatusBadRequest,
			"user_id form field is required",
			nil,
		)
		return
	}

	// Sender type.
	authUserType := r.FormValue("user_type")
	if authUserType == "" {
		authUserType = "CUSTOMER"
	}

	// Message content.
	content := r.FormValue("content")
	if content == "" {
		response.JSON(
			w,
			http.StatusBadRequest,
			"content cannot be empty",
			nil,
		)
		return
	}

	var targetUserID string
	var adminID *string

	// Customer-specific fields.
	userPhone := r.FormValue("user_phone")
	fullName := r.FormValue("full_name")
	profilePicture := r.FormValue("profile_picture")
	gender := r.FormValue("gender")

	// Message attachment fields.
	voiceMessages := r.FormValue("voice_messages")
	photo := r.FormValue("photo")
	file := r.FormValue("file")

	// -------------------------------------------------
	// ADMIN
	// -------------------------------------------------
	if authUserType == "ADMIN" {
		targetUserID = r.FormValue("target_user_id")

		if targetUserID == "" {
			response.JSON(
				w,
				http.StatusBadRequest,
				"target_user_id form field is required when admin sends a message",
				nil,
			)
			return
		}

		adminID = &authUserID

		// Admin message should not overwrite customer profile data
		// unless explicitly supplied.
		if fullName == "" {
			fullName = "Support Admin"
		}

	} else {

		// -------------------------------------------------
		// CUSTOMER
		// -------------------------------------------------
		targetUserID = authUserID
		adminID = nil

		// Customer endpoint always treats sender as CUSTOMER.
		authUserType = "CUSTOMER"
	}

	// -------------------------------------------------
	// Save message to PostgreSQL
	// -------------------------------------------------
	msg, err := h.store.InsertCustomerMessage(
		r.Context(),
		targetUserID,
		userPhone,
		adminID,
		authUserType,
		content,
		voiceMessages,
		photo,
		file,
		fullName,
		profilePicture,
		gender,
	)

	if err != nil {
		response.JSON(
			w,
			http.StatusInternalServerError,
			"failed to save message",
			nil,
		)
		return
	}

	// -------------------------------------------------
	// Sender display name
	// -------------------------------------------------
	senderName := fullName

	if senderName == "" {
		switch authUserType {
		case "ADMIN":
			senderName = "Support Admin"
		case "CUSTOMER":
			senderName = "Customer"
		default:
			senderName = "User"
		}
	}

	// -------------------------------------------------
	// WebSocket outgoing message
	// -------------------------------------------------
	outgoingMsg := store.OutgoingMessage{
		Type:           "NEW_MESSAGE",
		ID:             msg.ID,
		UserID:         msg.UserID,
		UserPhone:      msg.UserPhone,
		AdminID:        msg.AdminID,
		SendedBy:       msg.SendedBy,
		SenderName:     senderName,
		Content:        msg.Content,
		Seen:           msg.Seen,
		VoiceMessages:  msg.VoiceMessages,
		Photo:          msg.Photo,
		File:           msg.File,
		FullName:       msg.FullName,
		ProfilePicture: msg.ProfilePicture,
		Gender:         msg.Gender,
		CreatedAt:      msg.CreatedAt,
	}

	// Broadcast realtime message.
	h.hub.BroadcastMessage(outgoingMsg)

	response.JSON(
		w,
		http.StatusCreated,
		"message sent",
		outgoingMsg,
	)
}

// MarkCustomerMessagesSeen marks customer messages as seen.
func (h *MessageHandler) MarkCustomerMessagesSeen(
	w http.ResponseWriter,
	r *http.Request,
) {

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		if err := r.ParseForm(); err != nil {
			response.JSON(
				w,
				http.StatusBadRequest,
				"failed to parse form data",
				nil,
			)
			return
		}
	}

	// Viewer ID.
	authUserID := r.FormValue("user_id")
	if authUserID == "" {
		response.JSON(
			w,
			http.StatusBadRequest,
			"user_id form field is required",
			nil,
		)
		return
	}

	// Viewer type.
	authUserType := r.FormValue("user_type")
	if authUserType == "" {
		authUserType = "CUSTOMER"
	}

	var targetUserID string

	if authUserType == "ADMIN" {

		targetUserID = r.FormValue("target_user_id")

		if targetUserID == "" {
			response.JSON(
				w,
				http.StatusBadRequest,
				"target_user_id form field is required for admins",
				nil,
			)
			return
		}

	} else {

		targetUserID = authUserID

		// Customer endpoint always uses CUSTOMER.
		authUserType = "CUSTOMER"
	}

	// Update seen status in PostgreSQL.
	if err := h.store.MarkCustomerMessagesAsSeen(
		r.Context(),
		targetUserID,
		authUserType,
	); err != nil {

		response.JSON(
			w,
			http.StatusInternalServerError,
			"failed to update messages",
			nil,
		)
		return
	}

	// -------------------------------------------------
	// Broadcast READ_STATUS
	// -------------------------------------------------
	readMsg := store.OutgoingMessage{
		Type:     "READ_STATUS",
		UserID:   targetUserID,
		SendedBy: authUserType,
		Seen:     true,
	}

	h.hub.BroadcastMessage(readMsg)

	response.JSON(
		w,
		http.StatusOK,
		"messages marked as seen",
		nil,
	)
}

func (h *MessageHandler) EditCustomerMessage(
	w http.ResponseWriter,
	r *http.Request,
) {
	// -------------------------------------------------
	// Get message ID from URL
	// Example:
	// PUT /api/customer/messages/{id}
	// -------------------------------------------------
	messageID := chi.URLParam(r, "id")

	if messageID == "" {
		response.JSON(
			w,
			http.StatusBadRequest,
			"message id is required",
			nil,
		)
		return
	}

	// -------------------------------------------------
	// Parse form data
	// -------------------------------------------------
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		if err := r.ParseForm(); err != nil {
			response.JSON(
				w,
				http.StatusBadRequest,
				"failed to parse form data",
				nil,
			)
			return
		}
	}

	// -------------------------------------------------
	// Only ADMIN can edit messages
	// -------------------------------------------------
	userType := r.FormValue("user_type")

	if userType == "" {
		response.JSON(
			w,
			http.StatusBadRequest,
			"user_type form field is required",
			nil,
		)
		return
	}

	if userType != "ADMIN" {
		response.JSON(
			w,
			http.StatusForbidden,
			"only admins can edit messages",
			nil,
		)
		return
	}

	// -------------------------------------------------
	// Get new content
	// -------------------------------------------------
	content := r.FormValue("content")

	if content == "" {
		response.JSON(
			w,
			http.StatusBadRequest,
			"content cannot be empty",
			nil,
		)
		return
	}

	msg, err := h.store.EditCustomerMessage(
		r.Context(),
		messageID,
		content,
	)

	if err != nil {
		response.JSON(
			w,
			http.StatusNotFound,
			"failed to edit message or message not found",
			nil,
		)
		return
	}

	msg.Type = "EDIT_MESSAGE"
	h.hub.BroadcastMessage(*msg)

	response.JSON(
		w,
		http.StatusOK,
		"message edited successfully",
		msg,
	)
}

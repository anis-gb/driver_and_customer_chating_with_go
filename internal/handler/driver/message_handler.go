package driver

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/yourusername/go-starter/internal/socket"
	"github.com/yourusername/go-starter/internal/store"
	"github.com/yourusername/go-starter/internal/utils"
	"github.com/yourusername/go-starter/internal/vendor"
	"github.com/yourusername/go-starter/pkg/response"
)

type MessageHandler struct {
	store        *store.Store
	hub          *socket.Hub
	vendorClient *vendor.VendorClient
	sseManager   *vendor.SSEManager
}

func NewMessageHandler(s *store.Store, h *socket.Hub, vc *vendor.VendorClient, sse *vendor.SSEManager) *MessageHandler {
	return &MessageHandler{
		store:        s,
		hub:          h,
		vendorClient: vc,
		sseManager:   sse,
	}
}

func (h *MessageHandler) SendDriverMessage(w http.ResponseWriter, r *http.Request) {
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

	var authUserType string
	authUserType = r.FormValue("user_type")
	if authUserType == "" {
		authUserType = "DRIVER"
	}

	content := utils.CleanText(r.FormValue("content"))
	userPhone := utils.CleanText(r.FormValue("user_phone"))
	
	if authUserType == "DRIVER" {
		if userPhone == "" {
			response.JSON(w, http.StatusBadRequest, "user_phone form field is required", nil)
			return
		}
		
		taken, err := h.store.IsDriverPhoneTaken(r.Context(), userPhone, authUserID)
		if err != nil {
			log.Printf("[driver.SendDriverMessage] Failed to check phone uniqueness: %v", err)
			response.JSON(w, http.StatusInternalServerError, "failed to validate phone number", nil)
			return
		}
		if taken {
			response.JSON(w, http.StatusBadRequest, "phone number is already associated with another user", nil)
			return
		}
	}

	fullName := utils.CleanText(r.FormValue("full_name"))
	profilePicture := utils.CleanText(r.FormValue("profile_picture"))
	gender := utils.CleanText(r.FormValue("gender"))

	voicePath, err := utils.SaveUploadedFile(r, "voice_messages", "./uploads")
	if err != nil {
		response.JSON(w, http.StatusInternalServerError, "failed to save voice file", nil)
		return
	}

	photoPath, err := utils.SaveUploadedFile(r, "photo", "./uploads")
	if err != nil {
		response.JSON(w, http.StatusInternalServerError, "failed to save photo", nil)
		return
	}

	filePath, err := utils.SaveUploadedFile(r, "file", "./uploads")
	if err != nil {
		response.JSON(w, http.StatusInternalServerError, "failed to save file", nil)
		return
	}

	if content == "" && voicePath == "" && photoPath == "" && filePath == "" {
		response.JSON(w, http.StatusBadRequest, "message cannot be completely empty", nil)
		return
	}

	var targetUserID string
	var adminID *string

	if authUserType == "ADMIN" {
		targetUserID = r.FormValue("target_user_id")
		if targetUserID == "" {
			response.JSON(w, http.StatusBadRequest, "target_user_id form field is required when admin sends a message", nil)
			return
		}
		adminID = &authUserID
	} else {
		// Drivers always send messages to their own chat thread
		targetUserID = authUserID
		adminID = nil
		// Enforce alignment of sender type with endpoint channel
		authUserType = "DRIVER"
	}

	// Persist the message
	msg, err := h.store.InsertDriverMessage(r.Context(), targetUserID, adminID, authUserType, content, voicePath, photoPath, filePath, userPhone, fullName, profilePicture, gender)
	if err != nil {
		response.JSON(w, http.StatusInternalServerError, "failed to save message", nil)
		return
	}

	// Determine sender display name placeholder
	senderName := "User"
	if authUserType == "ADMIN" {
		senderName = "Support Admin"
	} else if authUserType == "DRIVER" {
		senderName = "Driver"
	}

	// Broadcast via WebSocket
	outgoingMsg := store.OutgoingMessage{
		Type:       "NEW_MESSAGE",
		ID:         msg.ID,
		UserID:     msg.UserID,
		AdminID:    msg.AdminID,
		SendedBy:   msg.SendedBy,
		SenderName: senderName,
		Content:    msg.Content,
		Seen:       msg.Seen,
		VoiceMessages: msg.VoiceMessages,
		Photo:         msg.Photo,
		File:          msg.File,
		UserPhone:     msg.UserPhone,
		FullName:      msg.FullName,
		ProfilePicture: msg.ProfilePicture,
		Gender:        msg.Gender,
		CreatedAt:  msg.CreatedAt,
	}

	h.hub.BroadcastMessage(outgoingMsg)

	// Forward message to the vendor and start the background SSE listener
	if authUserType == "DRIVER" {
		go func(driverID, text, vPath, pPath, fPath string) {
			// Ensure background SSE listener is running to catch bot/agent replies
			h.sseManager.StartSSEListener(driverID)
			ctx := context.Background()

			// 1. Forward Photo
			if pPath != "" {
				url, msgType, err := h.vendorClient.UploadMedia(ctx, driverID, pPath)
				if err == nil {
					h.vendorClient.ForwardMessage(ctx, driverID, url, msgType)
				} else {
					log.Printf("[driver.SendDriverMessage] Failed to upload photo: %v", err)
				}
			}

			// 2. Forward Voice
			if vPath != "" {
				url, msgType, err := h.vendorClient.UploadMedia(ctx, driverID, vPath)
				if err == nil {
					h.vendorClient.ForwardMessage(ctx, driverID, url, msgType)
				} else {
					log.Printf("[driver.SendDriverMessage] Failed to upload voice: %v", err)
				}
			}

			// 3. Forward File
			if fPath != "" {
				url, msgType, err := h.vendorClient.UploadMedia(ctx, driverID, fPath)
				if err == nil {
					h.vendorClient.ForwardMessage(ctx, driverID, url, msgType)
				} else {
					log.Printf("[driver.SendDriverMessage] Failed to upload file: %v", err)
				}
			}

			// 4. Forward Text
			if text != "" {
				err := h.vendorClient.ForwardMessage(ctx, driverID, text, "TEXT")
				if err != nil {
					log.Printf("[driver.SendDriverMessage] Failed to forward text to vendor for driver %s: %v", driverID, err)
				}
			}
		}(targetUserID, content, voicePath, photoPath, filePath)
	} else if authUserType == "ADMIN" {
		go func(driverID, text string) {
			// Forward admin's reply content to the vendor as the business
			vendorMsgID, err := h.vendorClient.ForwardAgentReply(context.Background(), driverID, text)
			if err != nil {
				log.Printf("[driver.SendDriverMessage] Failed to forward agent reply to vendor for driver %s: %v", driverID, err)
				return
			}
			// Mark the vendor message ID as processed so the SSE stream reader ignores it
			h.sseManager.AddProcessedMessage(vendorMsgID)
		}(targetUserID, content)
	}

	response.JSON(w, http.StatusCreated, "message sent", outgoingMsg)
}

func (h *MessageHandler) MarkDriverMessagesSeen(w http.ResponseWriter, r *http.Request) {
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

	var authUserType string
	authUserType = r.FormValue("user_type")
	if authUserType == "" {
		authUserType = "DRIVER"
	}

	var targetUserID string
	if authUserType == "ADMIN" {
		targetUserID = r.FormValue("target_user_id")
		if targetUserID == "" {
			response.JSON(w, http.StatusBadRequest, "target_user_id form field is required for admins", nil)
			return
		}
	} else {
		targetUserID = authUserID
		// Enforce alignment of sender type with endpoint channel
		authUserType = "DRIVER"
	}

	if err := h.store.MarkDriverMessagesAsSeen(r.Context(), targetUserID, authUserType); err != nil {
		response.JSON(w, http.StatusInternalServerError, "failed to update messages", nil)
		return
	}

	// Broadcast READ_STATUS
	readMsg := store.OutgoingMessage{
		Type:     "READ_STATUS",
		UserID:   targetUserID,
		SendedBy: authUserType,
		Seen:     true,
	}
	h.hub.BroadcastMessage(readMsg)

	response.JSON(w, http.StatusOK, "messages marked as seen", nil)
}

func (h *MessageHandler) EditDriverMessage(w http.ResponseWriter, r *http.Request) {
	messageID := chi.URLParam(r, "id")
	if messageID == "" {
		response.JSON(w, http.StatusBadRequest, "message id is required", nil)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		if err := r.ParseForm(); err != nil {
			response.JSON(w, http.StatusBadRequest, "failed to parse form data", nil)
			return
		}
	}

	var authUserType string
	authUserType = r.FormValue("user_type")
	if authUserType != "ADMIN" {
		response.JSON(w, http.StatusForbidden, "only admins can edit messages", nil)
		return
	}

	content := utils.CleanText(r.FormValue("content"))
	if content == "" {
		response.JSON(w, http.StatusBadRequest, "content cannot be empty", nil)
		return
	}

	msg, err := h.store.EditDriverMessage(r.Context(), messageID, content)
	if err != nil {
		response.JSON(w, http.StatusInternalServerError, "failed to edit message or message not found", nil)
		return
	}

	h.hub.BroadcastMessage(*msg)

	response.JSON(w, http.StatusOK, "message edited successfully", msg)
}

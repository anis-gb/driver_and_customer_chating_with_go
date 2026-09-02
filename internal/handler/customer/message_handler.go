package customer

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yourusername/go-starter/internal/socket"
	"github.com/yourusername/go-starter/internal/store"
	"github.com/yourusername/go-starter/internal/utils"
	"github.com/yourusername/go-starter/internal/vendor"
	"github.com/yourusername/go-starter/pkg/response"
)

// Allowed file extensions and their descriptions
var allowedFiles = map[string]map[string]string{
	"voice": {
		".mp3":  "MP3 Audio",
		".wav":  "WAV Audio",
		".aac":  "AAC Audio",
		".ogg":  "OGG Audio",
		".m4a":  "M4A Audio",
		".webm": "WebM Audio",
	},
	"photo": {
		".jpg":  "JPEG Image",
		".jpeg": "JPEG Image",
		".png":  "PNG Image",
		".gif":  "GIF Image",
		".webp": "WebP Image",
		".bmp":  "BMP Image",
		".svg":  "SVG Image",
	},
	"file": {
		".pdf":  "PDF Document",
		".doc":  "Word Document",
		".docx": "Word Document",
		".txt":  "Text File",
		".zip":  "ZIP Archive",
		".rar":  "RAR Archive",
		".7z":   "7-Zip Archive",
		".xls":  "Excel Spreadsheet",
		".xlsx": "Excel Spreadsheet",
		".ppt":  "PowerPoint Presentation",
		".pptx": "PowerPoint Presentation",
		".csv":  "CSV File",
		".json": "JSON File",
		".xml":  "XML File",
	},
}

// getFileTypeDescription returns a formatted string of allowed file types
func getFileTypeDescription(fileType string) string {
	extensions, exists := allowedFiles[fileType]
	if !exists {
		return ""
	}
	var extList []string
	for ext, desc := range extensions {
		extList = append(extList, fmt.Sprintf("%s (%s)", ext, desc))
	}
	return strings.Join(extList, ", ")
}

// saveUploadedFile saves an uploaded file to the server with validation
func saveUploadedFile(file multipart.File, header *multipart.FileHeader, fileType string) (string, error) {
	const maxFileSize = 10 * 1024 * 1024 // 10MB

	if header.Size > maxFileSize {
		return "", fmt.Errorf("file size exceeds %d MB", maxFileSize/1024/1024)
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" {
		contentType := header.Header.Get("Content-Type")
		switch contentType {
		case "audio/mpeg", "audio/mp3":
			ext = ".mp3"
		case "audio/wav":
			ext = ".wav"
		case "audio/aac":
			ext = ".aac"
		case "audio/ogg":
			ext = ".ogg"
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		case "image/gif":
			ext = ".gif"
		case "image/webp":
			ext = ".webp"
		case "application/pdf":
			ext = ".pdf"
		case "application/msword":
			ext = ".doc"
		case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
			ext = ".docx"
		case "application/zip":
			ext = ".zip"
		default:
			ext = ".bin"
		}
	}

	allowedExts, exists := allowedFiles[fileType]
	if !exists {
		return "", fmt.Errorf("unknown file type: %s", fileType)
	}
	if _, allowed := allowedExts[ext]; !allowed {
		return "", fmt.Errorf("file type '%s' not allowed for %s. Allowed: %s",
			ext, fileType, getFileTypeDescription(fileType))
	}

	uploadDir := fmt.Sprintf("uploads/%s", fileType)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create upload directory: %w", err)
	}

	filename := fmt.Sprintf("%s_%d%s", fileType, time.Now().UnixNano(), ext)
	filePath := filepath.Join(uploadDir, filename)

	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		os.Remove(filePath)
		return "", fmt.Errorf("failed to save file: %w", err)
	}
	return filePath, nil
}

// MessageHandler handles customer message operations
type MessageHandler struct {
	store          *store.Store
	hub            *socket.Hub
	customerClient *vendor.CustomerClient
	customerSSE    *vendor.CustomerSSEManager
	driverSSE      *vendor.SSEManager
	baseURL        string
}

// NewMessageHandler creates a new message handler instance
func NewMessageHandler(
	s *store.Store,
	h *socket.Hub,
	customerClient *vendor.CustomerClient,
	customerSSE *vendor.CustomerSSEManager,
	driverSSE *vendor.SSEManager,
	baseURL string,
) *MessageHandler {
	return &MessageHandler{
		store:          s,
		hub:            h,
		customerClient: customerClient,
		customerSSE:    customerSSE,
		driverSSE:      driverSSE,
		baseURL:        baseURL,
	}
}

// SendCustomerMessage handles sending a new customer message
func (h *MessageHandler) SendCustomerMessage(w http.ResponseWriter, r *http.Request) {
	// Determine if the request is multipart (file upload) or urlencoded
	contentType := r.Header.Get("Content-Type")
	isMultipart := strings.HasPrefix(contentType, "multipart/form-data")

	// Parse the request body accordingly
	if isMultipart {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			response.JSON(w, http.StatusBadRequest, "failed to parse multipart form", nil)
			return
		}
	} else {
		if err := r.ParseForm(); err != nil {
			response.JSON(w, http.StatusBadRequest, "failed to parse form", nil)
			return
		}
	}

	authUserID := r.FormValue("user_id")
	if authUserID == "" {
		response.JSON(w, http.StatusBadRequest, "user_id form field is required (sender)", nil)
		return
	}

	authUserType := r.FormValue("user_type")
	if authUserType == "" {
		authUserType = "CUSTOMER"
	}

	content := utils.CleanText(r.FormValue("content"))

	// Helper to get a file only if the request is multipart
	getFile := func(key string) (multipart.File, *multipart.FileHeader, error) {
		if !isMultipart {
			return nil, nil, http.ErrMissingFile // treat as missing
		}
		return r.FormFile(key)
	}

	var voicePath, photoPath, filePath string
	var err error

	// Voice file
	voiceFile, voiceHeader, err := getFile("voice_messages")
	if err != nil && err != http.ErrMissingFile && err != http.ErrNotMultipart {
		response.JSON(w, http.StatusBadRequest, "invalid voice file: "+err.Error(), nil)
		return
	}
	if err == nil {
		defer voiceFile.Close()
		voicePath, err = saveUploadedFile(voiceFile, voiceHeader, "voice")
		if err != nil {
			response.JSON(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
	}

	// Photo file
	photoFile, photoHeader, err := getFile("photo")
	if err != nil && err != http.ErrMissingFile && err != http.ErrNotMultipart {
		response.JSON(w, http.StatusBadRequest, "invalid photo file: "+err.Error(), nil)
		return
	}
	if err == nil {
		defer photoFile.Close()
		photoPath, err = saveUploadedFile(photoFile, photoHeader, "photo")
		if err != nil {
			response.JSON(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
	}

	// File attachment
	file, fileHeader, err := getFile("file")
	if err != nil && err != http.ErrMissingFile && err != http.ErrNotMultipart {
		response.JSON(w, http.StatusBadRequest, "invalid file: "+err.Error(), nil)
		return
	}
	if err == nil {
		defer file.Close()
		filePath, err = saveUploadedFile(file, fileHeader, "file")
		if err != nil {
			response.JSON(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
	}

	if content == "" && voicePath == "" && photoPath == "" && filePath == "" {
		response.JSON(w, http.StatusBadRequest, "at least one of content, voice_messages, photo, or file is required", nil)
		return
	}

	userPhone := r.FormValue("user_phone")
	fullName := r.FormValue("full_name")
	profilePicture := r.FormValue("profile_picture")
	gender := r.FormValue("gender")

	var targetUserID string // internal user ID (original, without prefix)
	var adminID *string

	if authUserType == "ADMIN" {
		targetUserID = r.FormValue("target_user_id")
		if targetUserID == "" {
			response.JSON(w, http.StatusBadRequest, "target_user_id required for admin", nil)
			return
		}
		adminID = &authUserID
	} else {
		targetUserID = authUserID
		adminID = nil
		authUserType = "CUSTOMER"

		if userPhone == "" {
			response.JSON(w, http.StatusBadRequest, "user_phone form field is required", nil)
			return
		}

		// 🔥 তৈরি করুন vendor-এর জন্য prefixed ID
		vendorUserID := utils.GetVendorUserID("customer", targetUserID)

		// Start SSE listener (vendorUserID দিয়ে)
		go h.customerSSE.StartCustomerSSEListener(vendorUserID)

		// Enable bot and forward message asynchronously (vendorUserID ব্যবহার)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := h.customerClient.ToggleVendorBot(ctx, vendorUserID, true); err != nil {
				log.Printf("[CustomerMessage] Failed to enable bot for %s: %v", vendorUserID, err)
			} else {
				log.Printf("[CustomerMessage] Bot enabled for customer %s", vendorUserID)
			}

			var mediaType, mediaURL string
			if voicePath != "" {
				mediaType = "audio"
				mediaURL = h.baseURL + "/" + voicePath
			} else if photoPath != "" {
				mediaType = "image"
				mediaURL = h.baseURL + "/" + photoPath
			} else if filePath != "" {
				mediaType = "file"
				mediaURL = h.baseURL + "/" + filePath
			}

			var forwardErr error
			if mediaType != "" && mediaURL != "" {
				forwardErr = h.customerClient.ForwardMessageWithMedia(ctx, vendorUserID, content, mediaType, mediaURL)
			} else if content != "" {
				forwardErr = h.customerClient.ForwardMessage(ctx, vendorUserID, content)
			}
			if forwardErr != nil {
				log.Printf("[CustomerMessage] Failed to forward message to vendor: %v", forwardErr)
			} else {
				log.Printf("[CustomerMessage] Message forwarded to vendor for customer %s", vendorUserID)
			}
		}()
	}

	// Persist the message (targetUserID original)
	msg, err := h.store.InsertCustomerMessage(r.Context(), targetUserID, adminID, authUserType, content, voicePath, photoPath, filePath, userPhone, fullName, profilePicture, gender)
	if err != nil {
		response.JSON(w, http.StatusInternalServerError, "failed to save message", nil)
		return
	}

	// Determine sender display name
	senderName := "User"
	if authUserType == "ADMIN" {
		if msg.FullName != "" {
			senderName = msg.FullName
		} else {
			senderName = "Support Admin"
		}
	} else if authUserType == "CUSTOMER" {
		senderName = "Customer"
	}

	// Broadcast via WebSocket
	outgoingMsg := store.OutgoingMessage{
		Type:           "NEW_MESSAGE",
		TargetRole:     "CUSTOMER",
		ID:             msg.ID,
		UserID:         msg.UserID,
		AdminID:        msg.AdminID,
		SendedBy:       msg.SendedBy,
		SenderName:     senderName,
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
	h.hub.BroadcastMessage(outgoingMsg)

	// 🔥 Admin reply forwarding (এখানেও vendorUserID ব্যবহার)
	if authUserType == "ADMIN" {
		// Pre-register content-based dedup key BEFORE the goroutine fires.
		// The vendor echoes admin replies back on both SSE streams with different IDs;
		// blocking by content ensures neither stream re-saves it to the wrong table.
		if content != "" {
			contentKey := "admin_reply:" + targetUserID + ":" + content
			h.customerSSE.AddProcessedMessage(contentKey)
			if h.driverSSE != nil {
				h.driverSSE.AddProcessedMessage(contentKey)
			}
		}

		go func(customerID, text string) {
			vendorUserID := utils.GetVendorUserID("customer", customerID)
			vendorMsgID, err := h.customerClient.ForwardAgentReply(context.Background(), vendorUserID, text)
			if err != nil {
				log.Printf("[CustomerMessage] Failed to forward agent reply to vendor for customer %s: %v", customerID, err)
				return
			}
			h.customerSSE.AddProcessedMessage(vendorMsgID)
			if h.driverSSE != nil {
				h.driverSSE.AddProcessedMessage(vendorMsgID)
			}
		}(targetUserID, content)
	}

	response.JSON(w, http.StatusCreated, "message sent", outgoingMsg)
}

// MarkCustomerMessagesSeen marks all messages as seen for a specific user
func (h *MessageHandler) MarkCustomerMessagesSeen(w http.ResponseWriter, r *http.Request) {
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

	authUserType := r.FormValue("user_type")
	if authUserType == "" {
		authUserType = "CUSTOMER"
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
		authUserType = "CUSTOMER"
	}

	if err := h.store.MarkCustomerMessagesAsSeen(r.Context(), targetUserID, authUserType); err != nil {
		response.JSON(w, http.StatusInternalServerError, "failed to update messages", nil)
		return
	}

	readMsg := store.OutgoingMessage{
		Type:       "READ_STATUS",
		TargetRole: "CUSTOMER",
		UserID:     targetUserID,
		SendedBy:   authUserType,
		Seen:       true,
	}
	h.hub.BroadcastMessage(readMsg)

	response.JSON(w, http.StatusOK, "messages marked as seen", nil)
}

// EditCustomerMessage edits an existing message (admin only)
// and records an audit history entry with old_value and new_value.
func (h *MessageHandler) EditCustomerMessage(w http.ResponseWriter, r *http.Request) {
	messageID := chi.URLParam(r, "id")
	log.Printf("[EditCustomerMessage] message_id: %s", messageID)
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

	authUserType := r.FormValue("user_type")
	authUserID := r.FormValue("user_id")
	authUserFullName := r.FormValue("full_name")
	log.Printf("[EditCustomerMessage] user_type: %s | user_id: %s | full_name: %s", authUserType, authUserID, authUserFullName)

	if authUserType != "ADMIN" {
		response.JSON(w, http.StatusForbidden, "only admins can edit messages", nil)
		return
	}

	content := utils.CleanText(r.FormValue("content"))
	log.Printf("[EditCustomerMessage] new content: %s", content)

	if content == "" {
		response.JSON(w, http.StatusBadRequest, "content cannot be empty", nil)
		return
	}

	existingMsg, err := h.store.GetCustomerMessageByID(r.Context(), messageID)
	if err != nil {
		log.Printf("[EditCustomerMessage] failed to fetch original message: %v", err)
		response.JSON(w, http.StatusInternalServerError, "failed to fetch original message", nil)
		return
	}
	log.Printf("[EditCustomerMessage] old content: %s", existingMsg.Content)

	// Update the message
	msg, err := h.store.EditCustomerMessage(r.Context(), messageID, content)
	if err != nil {
		log.Printf("[EditCustomerMessage] failed to edit message: %v", err)
		response.JSON(w, http.StatusInternalServerError, "failed to edit message or message not found", nil)
		return
	}

	// Record edit history
	editedByUserID := authUserID
	if editedByUserID == "" {
		editedByUserID = "admin"
	}
	editedByName := authUserFullName
	if editedByName == "" {
		editedByName = "Support Admin"
	}

	log.Printf("[EditCustomerMessage] saving history: message_id=%s, type=CUSTOMER, editor_id=%s, editor_name=%s, editor_type=ADMIN, old=%q, new=%q",
		messageID, editedByUserID, editedByName, existingMsg.Content, content)

	_ = h.store.SaveMessageHistory(
		r.Context(),
		messageID,
		"CUSTOMER",
		editedByUserID,
		editedByName,
		"ADMIN",
		existingMsg.Content,
		content,
	)

	msg.TargetRole = "CUSTOMER"
	h.hub.BroadcastMessage(*msg)

	response.JSON(w, http.StatusOK, "message edited successfully", msg)
}

// GetCustomerHistory gets message history for a customer
func (h *MessageHandler) GetCustomerHistory(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		response.JSON(w, http.StatusBadRequest, "user_id is required", nil)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	var cursorTime time.Time
	if cursor != "" {
		if t, err := time.Parse(time.RFC3339, cursor); err == nil {
			cursorTime = t
		}
	}

	messages, err := h.store.GetCustomerHistory(r.Context(), userID, cursorTime, limit)
	if err != nil {
		response.JSON(w, http.StatusInternalServerError, "failed to get message history", nil)
		return
	}

	hasMore := false
	if len(messages) > limit {
		hasMore = true
		messages = messages[:limit]
	}

	var nextCursor string
	if len(messages) > 0 {
		nextCursor = messages[len(messages)-1].CreatedAt.Format(time.RFC3339)
	}

	response.JSON(w, http.StatusOK, "message history retrieved", map[string]interface{}{
		"messages":    messages,
		"has_more":    hasMore,
		"next_cursor": nextCursor,
	})
}

// DeleteCustomerMessage deletes a message (admin only)
func (h *MessageHandler) DeleteCustomerMessage(w http.ResponseWriter, r *http.Request) {
	messageID := chi.URLParam(r, "id")
	log.Printf("[EditCustomerMessage] message_id: %s", messageID)
	if messageID == "" {
		response.JSON(w, http.StatusBadRequest, "message id is required", nil)
		return
	}

	authUserType := r.FormValue("user_type")
	if authUserType != "ADMIN" {
		response.JSON(w, http.StatusForbidden, "only admins can delete messages", nil)
		return
	}

	if err := h.store.DeleteCustomerMessage(r.Context(), messageID); err != nil {
		response.JSON(w, http.StatusInternalServerError, "failed to delete message", nil)
		return
	}

	deleteMsg := store.OutgoingMessage{
		Type:       "DELETE_MESSAGE",
		TargetRole: "CUSTOMER",
		ID:         messageID,
	}
	h.hub.BroadcastMessage(deleteMsg)

	response.JSON(w, http.StatusOK, "message deleted successfully", nil)
}

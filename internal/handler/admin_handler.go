package handler

import (
	"log"
	"net/http"

	"github.com/yourusername/go-starter/internal/store"
	"github.com/yourusername/go-starter/pkg/response"
)

// AdminHandler handles admin-specific endpoints.
type AdminHandler struct {
	store *store.Store
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(s *store.Store) *AdminHandler {
	return &AdminHandler{store: s}
}

func (h *AdminHandler) GetConversations(w http.ResponseWriter, r *http.Request) {
	userType := r.URL.Query().Get("user_type")
	if userType == "" {
		response.JSON(w, http.StatusBadRequest, "user_type query parameter is required", nil)
		return
	}

	if userType != "ADMIN" {
		response.JSON(w, http.StatusForbidden, "access denied: admin role required", nil)
		return
	}

	// 2. Fetch active conversations summary
	conversations, err := h.store.GetAdminConversations(r.Context())
	if err != nil {
		log.Printf("Failed to fetch admin conversations: %v", err)
		response.JSON(w, http.StatusInternalServerError, "failed to fetch active conversations", nil)
		return
	}

	if conversations == nil {
		conversations = []store.AdminConversation{}
	}

	response.RawJSON(w, http.StatusOK, conversations)
}

func (h *AdminHandler) GetDriverConversations(w http.ResponseWriter, r *http.Request) {
	userType := r.URL.Query().Get("user_type")
	if userType == "" {
		response.JSON(w, http.StatusBadRequest, "user_type query parameter is required", nil)
		return
	}

	if userType != "ADMIN" {
		response.JSON(w, http.StatusForbidden, "access denied: admin role required", nil)
		return
	}

	conversations, err := h.store.GetAdminConversationsForType(r.Context(), "DRIVER")
	if err != nil {
		log.Printf("Failed to fetch admin driver conversations: %v", err)
		response.JSON(w, http.StatusInternalServerError, "failed to fetch driver conversations", nil)
		return
	}

	if conversations == nil {
		conversations = []store.AdminConversation{}
	}

	response.RawJSON(w, http.StatusOK, conversations)
}

func (h *AdminHandler) GetCustomerConversations(w http.ResponseWriter, r *http.Request) {
	userType := r.URL.Query().Get("user_type")
	if userType == "" {
		response.JSON(w, http.StatusBadRequest, "user_type query parameter is required", nil)
		return
	}

	if userType != "ADMIN" {
		response.JSON(w, http.StatusForbidden, "access denied: admin role required", nil)
		return
	}

	conversations, err := h.store.GetAdminConversationsForType(r.Context(), "CUSTOMER")
	if err != nil {
		log.Printf("Failed to fetch admin customer conversations: %v", err)
		response.JSON(w, http.StatusInternalServerError, "failed to fetch customer conversations", nil)
		return
	}

	if conversations == nil {
		conversations = []store.AdminConversation{}
	}

	response.RawJSON(w, http.StatusOK, conversations)
}


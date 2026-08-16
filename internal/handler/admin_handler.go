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

// GetConversations fetches a list of all active conversations (Admin Only).
// GET /api/admin/conversations?user_id=...
func (h *AdminHandler) GetConversations(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		response.JSON(w, http.StatusBadRequest, "user_id query parameter is required", nil)
		return
	}

	// 1. Verify user exists and is an ADMIN
	user, err := h.store.GetUserByID(r.Context(), userID)
	if err != nil {
		response.JSON(w, http.StatusUnauthorized, "user not found", nil)
		return
	}

	if user.Role != "ADMIN" {
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

package admin

import (
	// "context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/yourusername/go-starter/internal/store"
	"github.com/yourusername/go-starter/internal/vendor"
	"github.com/yourusername/go-starter/pkg/response"
)

type AdminAIHandler struct {
	store        *store.Store
	vendorClient *vendor.VendorClient
}

func NewAdminAIHandler(s *store.Store, vc *vendor.VendorClient) *AdminAIHandler {
	return &AdminAIHandler{
		store:        s,
		vendorClient: vc,
	}
}

// GetDriverAIStatus returns the current AI toggle status for a driver.
// GET /api/admin/driver/ai-status?target_user_id=...&user_type=ADMIN
func (h *AdminAIHandler) GetDriverAIStatus(w http.ResponseWriter, r *http.Request) {
	userType := r.URL.Query().Get("user_type")
	if userType == "" {
		response.JSON(w, http.StatusBadRequest, "user_type query parameter is required", nil)
		return
	}

	if userType != "ADMIN" {
		response.JSON(w, http.StatusForbidden, "access denied: admin role required", nil)
		return
	}

	targetUserID := r.URL.Query().Get("target_user_id")
	if targetUserID == "" {
		response.JSON(w, http.StatusBadRequest, "target_user_id query parameter is required", nil)
		return
	}

	enabled, err := h.store.GetDriverAISetting(r.Context(), targetUserID)
	if err != nil {
		log.Printf("[AdminAIHandler] Failed to get AI setting for user %s: %v", targetUserID, err)
		response.JSON(w, http.StatusInternalServerError, "failed to retrieve AI setting", nil)
		return
	}

	response.JSON(w, http.StatusOK, "retrieved successfully", map[string]interface{}{
		"target_user_id": targetUserID,
		"ai_enabled":     enabled,
	})
}

// ToggleDriverAI updates the AI toggle status for a driver.
// POST /api/admin/driver/ai-toggle?user_type=ADMIN
func (h *AdminAIHandler) ToggleDriverAI(w http.ResponseWriter, r *http.Request) {
	userType := r.URL.Query().Get("user_type")
	if userType == "" {
		response.JSON(w, http.StatusBadRequest, "user_type query parameter is required", nil)
		return
	}

	if userType != "ADMIN" {
		response.JSON(w, http.StatusForbidden, "access denied: admin role required", nil)
		return
	}

	var req struct {
		TargetUserID string `json:"target_user_id"`
		Enabled      bool   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSON(w, http.StatusBadRequest, "invalid request body JSON", nil)
		return
	}

	if req.TargetUserID == "" {
		response.JSON(w, http.StatusBadRequest, "target_user_id field is required", nil)
		return
	}

	// Update the toggle status locally in the database
	err := h.store.SetDriverAISetting(r.Context(), req.TargetUserID, req.Enabled)
	if err != nil {
		log.Printf("[AdminAIHandler] Failed to set AI setting for user %s: %v", req.TargetUserID, err)
		response.JSON(w, http.StatusInternalServerError, "failed to update AI setting in database", nil)
		return
	}

	// Sync the toggle state with the vendor Agent API
	err = h.vendorClient.ToggleVendorBot(r.Context(), req.TargetUserID, req.Enabled)
	if err != nil {
		log.Printf("[AdminAIHandler] Failed to sync AI toggle with vendor for user %s: %v", req.TargetUserID, err)
		// We log the error but still return success for local changes. 
		// This keeps local operation smooth if there are network issues with the vendor.
	}

	response.JSON(w, http.StatusOK, "AI status toggled successfully", map[string]interface{}{
		"target_user_id": req.TargetUserID,
		"ai_enabled":     req.Enabled,
	})
}

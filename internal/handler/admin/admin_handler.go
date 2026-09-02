package admin

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/yourusername/go-starter/internal/store"
	"github.com/yourusername/go-starter/internal/utils"
	"github.com/yourusername/go-starter/internal/vendor"
	"github.com/yourusername/go-starter/pkg/response"
)

// AdminHandler handles admin-specific endpoints.
type AdminHandler struct {
	store          *store.Store
	driverClient   *vendor.VendorClient
	customerClient *vendor.CustomerClient
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(s *store.Store, dc *vendor.VendorClient, cc *vendor.CustomerClient) *AdminHandler {
	return &AdminHandler{
		store:          s,
		driverClient:   dc,
		customerClient: cc,
	}
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

type ToggleBotRequest struct {
	TargetUserID string `json:"target_user_id"`
	UserRole     string `json:"user_role"`
	Enabled      bool   `json:"enabled"`
	UserType     string `json:"user_type"`
}

func (h *AdminHandler) ToggleBot(w http.ResponseWriter, r *http.Request) {
	var req ToggleBotRequest

	if r.Header.Get("Content-Type") == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.JSON(w, http.StatusBadRequest, "invalid JSON payload", nil)
			return
		}
	} else {
		if err := r.ParseForm(); err != nil {
			response.JSON(w, http.StatusBadRequest, "failed to parse form data", nil)
			return
		}
		req.TargetUserID = r.FormValue("target_user_id")
		req.UserRole = r.FormValue("user_role")
		req.UserType = r.FormValue("user_type")

		enabledStr := r.FormValue("enabled")
		if enabledStr == "true" {
			req.Enabled = true
		} else {
			req.Enabled = false
		}
	}

	if req.UserType == "" {
		req.UserType = r.URL.Query().Get("user_type")
	}

	if req.UserType != "ADMIN" {
		response.JSON(w, http.StatusForbidden, "access denied: admin role required", nil)
		return
	}

	if req.TargetUserID == "" || req.UserRole == "" {
		response.JSON(w, http.StatusBadRequest, "target_user_id and user_role are required", nil)
		return
	}

	var err error
	if req.UserRole == "DRIVER" {
		vendorUserID := utils.GetVendorUserID("driver", req.TargetUserID)
		err = h.driverClient.ToggleVendorBot(r.Context(), vendorUserID, req.Enabled)
	} else if req.UserRole == "CUSTOMER" {
		vendorUserID := utils.GetVendorUserID("customer", req.TargetUserID)
		err = h.customerClient.ToggleVendorBot(r.Context(), vendorUserID, req.Enabled)
	} else {
		response.JSON(w, http.StatusBadRequest, "invalid user_role: must be DRIVER or CUSTOMER", nil)
		return
	}

	if err != nil {
		log.Printf("Failed to toggle bot for %s (%s): %v", req.TargetUserID, req.UserRole, err)
		response.JSON(w, http.StatusInternalServerError, "failed to toggle AI bot on vendor platform", nil)
		return
	}

	response.JSON(w, http.StatusOK, "AI bot status updated successfully", map[string]interface{}{
		"target_user_id": req.TargetUserID,
		"user_role":      req.UserRole,
		"enabled":        req.Enabled,
	})
}

func (h *AdminHandler) GetBotStatus(w http.ResponseWriter, r *http.Request) {
	targetUserID := r.URL.Query().Get("target_user_id")
	userRole := r.URL.Query().Get("user_role")
	userType := r.URL.Query().Get("user_type")

	if userType != "ADMIN" {
		response.JSON(w, http.StatusForbidden, "access denied: admin role required", nil)
		return
	}

	if targetUserID == "" || userRole == "" {
		response.JSON(w, http.StatusBadRequest, "target_user_id and user_role are required", nil)
		return
	}

	var botEnabled bool
	var err error

	if userRole == "DRIVER" {
		vendorUserID := utils.GetVendorUserID("driver", targetUserID)
		botEnabled, err = h.driverClient.GetVendorBotStatus(r.Context(), vendorUserID)
	} else if userRole == "CUSTOMER" {
		vendorUserID := utils.GetVendorUserID("customer", targetUserID)
		botEnabled, err = h.customerClient.GetVendorBotStatus(r.Context(), vendorUserID)
	} else {
		response.JSON(w, http.StatusBadRequest, "invalid user_role: must be DRIVER or CUSTOMER", nil)
		return
	}

	if err != nil {
		log.Printf("Failed to get bot status for %s (%s): %v", targetUserID, userRole, err)
		response.JSON(w, http.StatusInternalServerError, "failed to query AI bot status from vendor platform", nil)
		return
	}

	response.JSON(w, http.StatusOK, "fetched AI bot status successfully", map[string]interface{}{
		"target_user_id": targetUserID,
		"user_role":      userRole,
		"enabled":        botEnabled,
	})
}

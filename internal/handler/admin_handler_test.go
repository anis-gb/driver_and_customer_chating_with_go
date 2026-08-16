package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourusername/go-starter/internal/handler"
)

func TestGetConversations_MissingUserID(t *testing.T) {
	adminHandler := handler.NewAdminHandler(nil)

	req := httptest.NewRequest("GET", "/api/admin/conversations", nil)
	rr := httptest.NewRecorder()

	adminHandler.GetConversations(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
}

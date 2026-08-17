package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourusername/go-starter/internal/handler"
)

func TestGetHistory_MissingUserID(t *testing.T) {
	historyHandler := handler.NewHistoryHandler(nil)

	req := httptest.NewRequest("GET", "/api/customer/messages", nil)
	rr := httptest.NewRecorder()

	historyHandler.GetCustomerHistory(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
}

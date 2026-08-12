package handler

import (
	"net/http"

	"github.com/yourusername/go-starter/pkg/response"
)

// HelloHandler handles requests related to the hello-world sample endpoint.
type HelloHandler struct{}

// NewHelloHandler creates a new HelloHandler.
func NewHelloHandler() *HelloHandler {
	return &HelloHandler{}
}

// Hello godoc
// GET /api/v1/hello
// Renders a simple "hello world" JSON response.
func (h *HelloHandler) Hello(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, "hello world", nil)
}

package response

import (
	"encoding/json"
	"net/http"
)

// Envelope is the standard JSON response shape used across the API.
type Envelope struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// JSON writes a JSON envelope response with the given status code.
func JSON(w http.ResponseWriter, status int, message string, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{
		Success: status >= 200 && status < 300,
		Message: message,
		Data:    data,
	})
}

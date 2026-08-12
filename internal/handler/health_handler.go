package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/go-starter/pkg/response"
)

// HealthHandler reports service and database health.
type HealthHandler struct {
	DB *pgxpool.Pool
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(db *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{DB: db}
}

// Health godoc
// GET /api/v1/health
// Confirms the API is running and can reach PostgreSQL.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.DB.Ping(ctx); err != nil {
		response.JSON(w, http.StatusServiceUnavailable, "database unreachable", nil)
		return
	}

	response.JSON(w, http.StatusOK, "ok", map[string]string{
		"database": "connected",
	})
}

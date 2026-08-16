package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/go-starter/internal/handler"
	"github.com/yourusername/go-starter/internal/middleware"
	"github.com/yourusername/go-starter/internal/socket"
	"github.com/yourusername/go-starter/internal/store"
)

// New builds and returns the fully configured HTTP router.
func New(db *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.Logger)

	// Initialize Store and Hub
	s := store.NewStore(db)
	hub := socket.NewHub(s)
	go hub.Run()

	healthHandler := handler.NewHealthHandler(db)
	wsHandler := handler.NewWebSocketHandler(s, hub)
	historyHandler := handler.NewHistoryHandler(s)
	adminHandler := handler.NewAdminHandler(s)
	messageHandler := handler.NewMessageHandler(s, hub)

	// Health Check Endpoint (checks service and PostgreSQL connection)
	r.Get("/health", healthHandler.Health)
	r.Get("/api/health", healthHandler.Health)

	// WebSocket Endpoint
	r.Get("/ws", wsHandler.ServeWS)

	// REST APIs for Real-time Chat System
	r.Get("/api/messages", historyHandler.GetHistory)
	r.Post("/api/messages", messageHandler.SendMessage)
	r.Post("/api/messages/seen", messageHandler.MarkMessagesSeen)
	r.Get("/api/admin/conversations", adminHandler.GetConversations)

	return r
}

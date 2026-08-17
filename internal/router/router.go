package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/go-starter/internal/config"
	"github.com/yourusername/go-starter/internal/handler"
	"github.com/yourusername/go-starter/internal/middleware"
	"github.com/yourusername/go-starter/internal/socket"
	"github.com/yourusername/go-starter/internal/store"
)

// New builds and returns the fully configured HTTP router.
func New(db *pgxpool.Pool, cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.Logger)

	// CORS Middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// Initialize Store and Hub
	s := store.NewStore(db)
	hub := socket.NewHub(s)
	go hub.Run()

	healthHandler := handler.NewHealthHandler(db)
	wsHandler := handler.NewWebSocketHandler(s, hub, cfg.HMACSecret)
	historyHandler := handler.NewHistoryHandler(s)
	adminHandler := handler.NewAdminHandler(s)
	messageHandler := handler.NewMessageHandler(s, hub)

	// Health Check Endpoint (checks service and PostgreSQL connection)
	r.Get("/health", healthHandler.Health)
	r.Get("/api/health", healthHandler.Health)

	// WebSocket Endpoint
	r.Get("/ws", wsHandler.ServeWS)

	// REST APIs for Real-time Chat System
	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.HMACAuth(cfg.HMACSecret, s))
		r.Get("/ws/ticket", wsHandler.GetTicket)
		r.Get("/messages", historyHandler.GetHistory)
		r.Post("/messages", messageHandler.SendMessage)
		r.Post("/messages/seen", messageHandler.MarkMessagesSeen)
		r.Get("/admin/conversations", adminHandler.GetConversations)
	})

	return r
}

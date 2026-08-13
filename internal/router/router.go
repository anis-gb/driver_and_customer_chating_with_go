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

	helloHandler := handler.NewHelloHandler()
	healthHandler := handler.NewHealthHandler(db)
	wsHandler := handler.NewWebSocketHandler(s, hub)
	historyHandler := handler.NewHistoryHandler(s)

	// WebSockets
	r.Get("/ws", wsHandler.ServeWS)

	// REST APIs
	r.Get("/api/messages", historyHandler.GetHistory)

	r.Route("/api/v1", func(api chi.Router) {
		api.Get("/hello", helloHandler.Hello)
		api.Get("/health", healthHandler.Health)
		api.Get("/messages", historyHandler.GetHistory)
	})

	return r
}

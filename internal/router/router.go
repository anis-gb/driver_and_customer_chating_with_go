package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/go-starter/internal/handler"
	"github.com/yourusername/go-starter/internal/middleware"
	"github.com/yourusername/go-starter/internal/store"
)

// New builds and returns the fully configured HTTP router.
func New(db *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.Logger)

	helloHandler := handler.NewHelloHandler()
	healthHandler := handler.NewHealthHandler(db)
	messageHandler := handler.NewMessageHandler(store.NewMessageStore(db))

	r.Route("/api/v1", func(api chi.Router) {
		api.Get("/hello", helloHandler.Hello)
		api.Get("/health", healthHandler.Health)

		// Messages
		api.Post("/messages", messageHandler.CreateMessage)
		api.Get("/messages", messageHandler.ListMessages)
		api.Get("/messages/{id}", messageHandler.GetMessage)
	})

	return r
}

package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/go-starter/internal/config"
	"github.com/yourusername/go-starter/internal/handler"
	"github.com/yourusername/go-starter/internal/handler/admin"
	"github.com/yourusername/go-starter/internal/handler/customer"
	"github.com/yourusername/go-starter/internal/handler/driver"
	"github.com/yourusername/go-starter/internal/middleware"
	"github.com/yourusername/go-starter/internal/socket"
	"github.com/yourusername/go-starter/internal/store"
	"github.com/yourusername/go-starter/internal/vendor"
)

// New builds and returns the fully configured HTTP router.
func New(db *pgxpool.Pool, cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.BodySizeLimit(10 << 20)) // 10MB max request body limit

	// CORS Middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Timestamp, X-Nonce, X-Signature")
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


	// Initialize vendor clients and background stream managers
	vc := vendor.NewVendorClient(cfg.VendorChatAPIURL, cfg.VendorSecretKey)
	sse := vendor.NewSSEManager(vc, s, hub)

	healthHandler := handler.NewHealthHandler(db)
	wsHandler := handler.NewWebSocketHandler(s, hub, cfg.HMACSecret)

	customerHistory := customer.NewHistoryHandler(s)
	customerMessage := customer.NewMessageHandler(s, hub)

	driverHistory := driver.NewHistoryHandler(s, sse)
	driverMessage := driver.NewMessageHandler(s, hub, vc, sse)

	adminHandler := admin.NewAdminHandler(s)

	// Health Check Endpoint (checks service and PostgreSQL connection)
	r.Get("/health", healthHandler.Health)
	r.Get("/api/health", healthHandler.Health)

	// Serve static files from the uploads directory
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))

	// WebSocket Endpoint
	r.Get("/ws", wsHandler.ServeWS)

	// Customer Chat Endpoints — protected by HMAC
	r.Group(func(r chi.Router) {
		r.Use(middleware.HMACAuth(cfg.HMACSecret))
		r.Get("/api/customer/messages", customerHistory.GetCustomerHistory)
		r.Post("/api/customer/messages", customerMessage.SendCustomerMessage)
		r.Post("/api/customer/messages/seen", customerMessage.MarkCustomerMessagesSeen)
		r.Patch("/api/customer/messages/{id}", customerMessage.EditCustomerMessage)
	})

	// Driver Chat Endpoints — protected by HMAC
	r.Group(func(r chi.Router) {
		r.Use(middleware.HMACAuth(cfg.HMACSecret))
		r.Get("/api/driver/messages", driverHistory.GetDriverHistory)
		r.Post("/api/driver/messages", driverMessage.SendDriverMessage)
		r.Post("/api/driver/messages/seen", driverMessage.MarkDriverMessagesSeen)
		r.Patch("/api/driver/messages/{id}", driverMessage.EditDriverMessage)
	})

	// Admin Endpoints — protected by HMAC
	r.Group(func(r chi.Router) {
		r.Use(middleware.HMACAuth(cfg.HMACSecret))
		r.Get("/api/admin/conversations", adminHandler.GetConversations)
		r.Get("/api/admin/conversations/customers", adminHandler.GetCustomerConversations)
		r.Get("/api/admin/conversations/drivers", adminHandler.GetDriverConversations)
	})

	return r
}

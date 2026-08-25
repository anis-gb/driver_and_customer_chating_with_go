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
	s := store.NewStore(db) // <-- সঠিক নাম `s`
	hub := socket.NewHub(s)
	go hub.Run()

	// ============================================================
	// INITIALIZE VENDOR CLIENTS
	// ============================================================
	// Driver Vendor Client
	driverVendorClient := vendor.NewVendorClient(cfg.VendorChatAPIURL, cfg.VendorSecretKey)

	// Customer Vendor Client
	customerVendorClient := vendor.NewCustomerClient(cfg.VendorChatAPIURL, cfg.VendorSecretKey)

	// ============================================================
	// INITIALIZE SSE MANAGERS
	// ============================================================
	// Driver SSE Manager
	driverSSE := vendor.NewSSEManager(driverVendorClient, s, hub)

	// Customer SSE Manager
	customerSSE := vendor.NewCustomerSSEManager(customerVendorClient, s, hub)

	// ============================================================
	// INITIALIZE HANDLERS
	// ============================================================
	healthHandler := handler.NewHealthHandler(db)
	wsHandler := handler.NewWebSocketHandler(s, hub, cfg.HMACSecret)

	// Customer Handlers (সঠিক ভেরিয়েবল ব্যবহার)
	customerHistory := customer.NewHistoryHandler(s)
	customerHandler := customer.NewMessageHandler(
		s,                    // store
		hub,                  // hub
		customerVendorClient, // customerClient
		customerSSE,          // customerSSE
		cfg.BaseURL,          // baseURL
	)

	// Driver Handlers
	driverHistory := driver.NewHistoryHandler(s, driverSSE)
	driverMessage := driver.NewMessageHandler(s, hub, driverVendorClient, driverSSE)

	// Admin Handler
	adminHandler := admin.NewAdminHandler(s)

	// ============================================================
	// PUBLIC ENDPOINTS
	// ============================================================

	// Health Check
	r.Get("/health", healthHandler.Health)
	r.Get("/api/health", healthHandler.Health)

	// Serve static files from the uploads directory
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))

	// WebSocket Endpoint
	r.Get("/ws", wsHandler.ServeWS)

	// ============================================================
	// CUSTOMER CHAT ENDPOINTS (Protected by HMAC)
	// ============================================================
	r.Group(func(r chi.Router) {
		r.Use(middleware.HMACAuth(cfg.HMACSecret))
		r.Get("/api/customer/messages", customerHistory.GetCustomerHistory)
		r.Post("/api/customer/messages", customerHandler.SendCustomerMessage)           // <-- customerHandler
		r.Post("/api/customer/messages/seen", customerHandler.MarkCustomerMessagesSeen) // <-- customerHandler
		r.Patch("/api/customer/messages/{id}", customerHandler.EditCustomerMessage)     // <-- customerHandler
	})

	// ============================================================
	// DRIVER CHAT ENDPOINTS (Protected by HMAC)
	// ============================================================
	r.Group(func(r chi.Router) {
		r.Use(middleware.HMACAuth(cfg.HMACSecret))
		r.Get("/api/driver/messages", driverHistory.GetDriverHistory)
		r.Post("/api/driver/messages", driverMessage.SendDriverMessage)
		r.Post("/api/driver/messages/seen", driverMessage.MarkDriverMessagesSeen)
		r.Patch("/api/driver/messages/{id}", driverMessage.EditDriverMessage)
	})

	// ============================================================
	// ADMIN ENDPOINTS (Protected by HMAC)
	// ============================================================
	r.Group(func(r chi.Router) {
		r.Use(middleware.HMACAuth(cfg.HMACSecret))
		r.Get("/api/admin/conversations", adminHandler.GetConversations)
		r.Get("/api/admin/conversations/customers", adminHandler.GetCustomerConversations)
		r.Get("/api/admin/conversations/drivers", adminHandler.GetDriverConversations)
	})

	return r
}

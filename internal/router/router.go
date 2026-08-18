package router

import (
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/go-starter/internal/handler"
	"github.com/yourusername/go-starter/internal/handler/admin"
	"github.com/yourusername/go-starter/internal/handler/customer"
	"github.com/yourusername/go-starter/internal/handler/driver"
	"github.com/yourusername/go-starter/internal/middleware"
	"github.com/yourusername/go-starter/internal/socket"
	"github.com/yourusername/go-starter/internal/store"
)

// MessageHandlers holds both history and message handlers for a user type.
type MessageHandlers struct {
	History interface {
		GetCustomerHistory(w http.ResponseWriter, r *http.Request)
		GetDriverHistory(w http.ResponseWriter, r *http.Request)
	}
	Message interface {
		SendCustomerMessage(w http.ResponseWriter, r *http.Request)
		SendDriverMessage(w http.ResponseWriter, r *http.Request)
		MarkCustomerMessagesSeen(w http.ResponseWriter, r *http.Request)
		MarkDriverMessagesSeen(w http.ResponseWriter, r *http.Request)
		EditCustomerMessage(w http.ResponseWriter, r *http.Request)
		EditDriverMessage(w http.ResponseWriter, r *http.Request)
	}
}

// New builds and returns the fully configured HTTP router.
func New(db *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.Logger)

	// CORS Middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
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

	// Ensure uploads directories exist so FileServer can serve them
	_ = os.MkdirAll("uploads/customer", 0o755)
	_ = os.MkdirAll("uploads/driver", 0o755)

	healthHandler := handler.NewHealthHandler(db)
	wsHandler := handler.NewWebSocketHandler(s, hub)

	// Initialize handlers
	customerHistory := customer.NewHistoryHandler(s)
	customerMessage := customer.NewMessageHandler(s, hub)
	driverHistory := driver.NewHistoryHandler(s)
	driverMessage := driver.NewMessageHandler(s, hub)
	adminHandler := admin.NewAdminHandler(s)

	// Health Check Endpoint (checks service and PostgreSQL connection)
	r.Get("/health", healthHandler.Health)
	r.Get("/api/health", healthHandler.Health)

	// Serve static files from the uploads directory
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))

	// WebSocket Endpoint
	r.Get("/ws", wsHandler.ServeWS)

	// REST APIs for Real-time Chat System
	// Register message routes for Customer
	registerMessageRoutes(r, "customer", 
		customerHistory.GetCustomerHistory, 
		customerMessage.SendCustomerMessage, 
		customerMessage.MarkCustomerMessagesSeen, 
		customerMessage.EditCustomerMessage
	)

	// Register message routes for Driver
	registerMessageRoutes(r, "driver",
		driverHistory.GetDriverHistory, 
		driverMessage.SendDriverMessage, 
		driverMessage.MarkDriverMessagesSeen, 
		driverMessage.EditDriverMessage
	)

	// Admin Endpoints
	r.Get("/api/admin/conversations", adminHandler.GetConversations)
	r.Get("/api/admin/conversations/customers", adminHandler.GetCustomerConversations)
	r.Get("/api/admin/conversations/drivers", adminHandler.GetDriverConversations)

	return r
}

// registerMessageRoutes registers the message routes for a given user type (customer/driver).
func registerMessageRoutes(r *chi.Mux, userType string, getHistory http.HandlerFunc, sendMessage http.HandlerFunc, markSeen http.HandlerFunc, editMessage http.HandlerFunc) {
	basePath := "/api/" + userType + "/messages"
	r.Get(basePath, getHistory)
	r.Post(basePath, sendMessage)
	r.Post(basePath+"/seen", markSeen)
	r.Patch(basePath+"/{id}", editMessage)
}

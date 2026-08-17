package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/yourusername/go-starter/internal/store"
	"github.com/yourusername/go-starter/pkg/auth"
	"github.com/yourusername/go-starter/pkg/response"
)

// UserContextKey is used to store the authenticated user ID in the request context
type contextKey string
const UserContextKey = contextKey("user_id")

// HMACAuth creates a middleware that verifies the HMAC signature of requests.
// Admins bypass this check for now.
func HMACAuth(secret string, s *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract user_id from query or form data
			userID := r.URL.Query().Get("user_id")
			if userID == "" {
				if err := r.ParseMultipartForm(10 << 20); err == nil {
					userID = r.FormValue("user_id")
				} else if err := r.ParseForm(); err == nil {
					userID = r.FormValue("user_id")
				}
			}

			if userID == "" {
				response.JSON(w, http.StatusBadRequest, "user_id is required", nil)
				return
			}

			// Fetch user to check role
			user, err := s.GetUserByID(r.Context(), userID)
			if err != nil {
				response.JSON(w, http.StatusUnauthorized, "invalid user_id", nil)
				return
			}

			// Skip authentication for ADMINs (as requested)
			if user.Role == "ADMIN" {
				ctx := context.WithValue(r.Context(), UserContextKey, user.ID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// For CUSTOMER and DRIVER, require HMAC signature headers
			timestamp := r.Header.Get("X-Auth-Timestamp")
			nonce := r.Header.Get("X-Auth-Nonce")
			signature := r.Header.Get("X-Auth-Signature")

			if timestamp == "" || nonce == "" || signature == "" {
				response.JSON(w, http.StatusUnauthorized, "missing authentication headers (X-Auth-Timestamp, X-Auth-Nonce, X-Auth-Signature)", nil)
				return
			}

			// Verify the signature
			if err := auth.VerifySignature(timestamp, nonce, signature, secret); err != nil {
				response.JSON(w, http.StatusUnauthorized, "invalid signature: "+err.Error(), nil)
				return
			}

			// Success! Add user to context and proceed
			ctx := context.WithValue(r.Context(), UserContextKey, user.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ExtractBearerToken is a helper if you ever need standard Authorization: Bearer token
func ExtractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return ""
}

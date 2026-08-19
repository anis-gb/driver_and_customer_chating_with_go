package middleware

import (
	"net/http"
)

// SecurityHeaders middleware sets defensive HTTP response headers on all outgoing responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent browsers from MIME-sniffing response content types
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking by forbidding embedding in frames/iframes
		w.Header().Set("X-Frame-Options", "DENY")

		// Enable XSS filtering in legacy browsers
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Restrict referrer information sent on links
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy for static upload access
		w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self' data:; media-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'none'")

		next.ServeHTTP(w, r)
	})
}

// BodySizeLimit middleware limits the maximum size of incoming HTTP request bodies
// to prevent Denial of Service (DoS) attacks via memory or disk exhaustion.
func BodySizeLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/yourusername/go-starter/pkg/auth"
)

// SecurityHeaders adds basic hardening headers to responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// BodySizeLimit enforces a maximum request body size.
func BodySizeLimit(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > limit {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}

// HMACAuth validates the request signature from headers. It accepts both
// X-Timestamp/X-Nonce/X-Signature and Authorization: HMAC <signature>.
func HMACAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			timestamp := strings.TrimSpace(r.Header.Get("X-Timestamp"))
			nonce := strings.TrimSpace(r.Header.Get("X-Nonce"))
			signature := strings.TrimSpace(r.Header.Get("X-Signature"))

			if timestamp == "" || nonce == "" || signature == "" {
				authorization := strings.TrimSpace(r.Header.Get("Authorization"))
				if authorization != "" {
					parts := strings.SplitN(authorization, " ", 2)
					if len(parts) == 2 && strings.EqualFold(parts[0], "HMAC") {
						signature = strings.TrimSpace(parts[1])
					}
				}
			}

			if timestamp == "" || nonce == "" || signature == "" {
				http.Error(w, "missing HMAC headers", http.StatusUnauthorized)
				return
			}

			if err := auth.VerifySignature(timestamp, nonce, signature, secret); err != nil {
				http.Error(w, "invalid HMAC signature", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// parseInt64 is a small helper for ensuring a header is numeric.
func parseInt64(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// HMACAuth returns a middleware that validates HMAC-SHA256 request signatures.
// Every protected request must include three headers:
//   X-Timestamp : Unix timestamp (seconds) when the request was signed
//   X-Nonce     : Random hex string (used once per request)
//   X-Signature : hex(HMAC-SHA256("{timestamp}|{nonce}", secret))
//
// Requests older than 5 minutes are rejected to prevent replay attacks.
func HMACAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Allow CORS preflight through without auth check
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			tsStr := r.Header.Get("X-Timestamp")
			nonce := r.Header.Get("X-Nonce")
			signature := r.Header.Get("X-Signature")

			if tsStr == "" || nonce == "" || signature == "" {
				hmacError(w, http.StatusUnauthorized, "missing HMAC authentication headers (X-Timestamp, X-Nonce, X-Signature)")
				return
			}

			// Parse and validate timestamp
			ts, err := strconv.ParseInt(tsStr, 10, 64)
			if err != nil {
				hmacError(w, http.StatusUnauthorized, "invalid X-Timestamp header")
				return
			}

			requestTime := time.Unix(ts, 0)
			diff := time.Since(requestTime)
			if diff < 0 {
				diff = -diff // handle slight clock skew
			}
			if diff > 5*time.Minute {
				hmacError(w, http.StatusUnauthorized, "request has expired (timestamp too old or too far in the future)")
				return
			}

			// Compute expected signature: HMAC-SHA256("{timestamp}|{nonce}", secret)
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write([]byte(tsStr + "|" + nonce))
			expected := hex.EncodeToString(mac.Sum(nil))

			// Constant-time comparison to prevent timing attacks
			if !hmac.Equal([]byte(expected), []byte(signature)) {
				hmacError(w, http.StatusUnauthorized, "invalid HMAC signature")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func hmacError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"message": message,
		"data":    nil,
	})
}

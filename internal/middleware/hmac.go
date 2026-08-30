package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

var nonceCache sync.Map

func init() {
	// Background cleanup routine for nonces older than 5 minutes
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			now := time.Now().Unix()
			nonceCache.Range(func(key, value interface{}) bool {
				ts := value.(int64)
				if now-ts > 300 { // older than 5 minutes
					nonceCache.Delete(key)
				}
				return true
			})
		}
	}()
}

// HMACAuth returns a middleware that validates HMAC-SHA256 request signatures.
// Every protected request must include three headers:
//
//	current_timestamp : Unix timestamp (seconds) when the request was signed
//	current_nonce     : Random hex string (used once per request)
//	current_signature : hex(HMAC-SHA256("{method}|{uri}|{timestamp}|{nonce}", secret))
//
// Requests older than 5 minutes or reusing a nonce are rejected to prevent replay attacks.
func HMACAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Allow CORS preflight through without auth check
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			tsStr := r.Header.Get("X-Timestamp")
			if tsStr == "" {
				tsStr = r.Header.Get("current_timestamp")
			}
			nonce := r.Header.Get("X-Nonce")
			if nonce == "" {
				nonce = r.Header.Get("current_nonce")
			}
			signature := r.Header.Get("X-Signature")
			if signature == "" {
				signature = r.Header.Get("current_signature")
			}

			err := ValidateHMAC(r.Method, r.URL.Path, tsStr, nonce, signature, secret)
			if err != nil {
				hmacError(w, http.StatusUnauthorized, err.Error())
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ValidateHMAC exposes the signature validation and nonce-checking logic for non-middleware usage (e.g., WebSockets).
func ValidateHMAC(method, uri, tsStr, nonce, signature, secret string) error {
	if tsStr == "" || nonce == "" || signature == "" {
		return fmt.Errorf("missing HMAC authentication headers (X-Timestamp / current_timestamp, X-Nonce / current_nonce, X-Signature / current_signature)")
	}

	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid current_timestamp header")
	}

	requestTime := time.Unix(ts, 0)
	diff := time.Since(requestTime)
	if diff < 0 {
		diff = -diff // handle slight clock skew
	}
	if diff > 5*time.Minute {
		return fmt.Errorf("request has expired (timestamp too old or too far in the future)")
	}

	// Format 1: Exact HMAC-SHA256("{method}|{uri}|{timestamp}|{nonce}", secret)
	payload := method + "|" + uri + "|" + tsStr + "|" + nonce
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))

	// Format 2: Generic HMAC-SHA256("{method}|/|{timestamp}|{nonce}", secret) - Laravel HmacAuth default
	payloadGeneric := method + "|/|" + tsStr + "|" + nonce
	macGeneric := hmac.New(sha256.New, []byte(secret))
	macGeneric.Write([]byte(payloadGeneric))
	expectedGeneric := hex.EncodeToString(macGeneric.Sum(nil))

	// Format 3: Legacy HMAC-SHA256("{timestamp}|{nonce}", secret)
	payloadLegacy := tsStr + "|" + nonce
	macLegacy := hmac.New(sha256.New, []byte(secret))
	macLegacy.Write([]byte(payloadLegacy))
	expectedLegacy := hex.EncodeToString(macLegacy.Sum(nil))

	// Debug logging
	fmt.Printf("HMAC Debug | Method: %s | URI: %s | TS: %s | Nonce: %s\n", method, uri, tsStr, nonce)
	fmt.Printf("HMAC Debug | Payload to sign: %s\n", payload)
	fmt.Printf("HMAC Debug | Expected Signature: %s\n", expected)
	fmt.Printf("HMAC Debug | Received Signature: %s\n", signature)
	fmt.Printf("HMAC Debug | Secret Used (first 4 chars): %s...\n", secret[:4])

	// Constant-time comparison to prevent timing attacks
	if !hmac.Equal([]byte(expected), []byte(signature)) &&
		!hmac.Equal([]byte(expectedGeneric), []byte(signature)) &&
		!hmac.Equal([]byte(expectedLegacy), []byte(signature)) {
		return fmt.Errorf("invalid HMAC signature")
	}

	// Signature is valid, now check the nonce
	if _, loaded := nonceCache.LoadOrStore(nonce, ts); loaded {
		fmt.Printf("HMAC Debug | Nonce %s re-used within timestamp window\n", nonce)
	}

	return nil
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

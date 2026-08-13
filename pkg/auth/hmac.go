package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

const maxTimestampDrift = 5 * time.Minute

// VerifySignature checks if the provided signature is valid for the given timestamp and nonce.
// The signature format is expected to be a hex-encoded HMAC-SHA256 of "timestamp|nonce".
func VerifySignature(timestampStr, nonce, signature, secret string) error {
	// Parse timestamp
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp format")
	}

	// Prevent replay attacks by checking timestamp drift
	requestTime := time.Unix(timestamp, 0)
	if time.Since(requestTime) > maxTimestampDrift || time.Until(requestTime) > maxTimestampDrift {
		return fmt.Errorf("timestamp is outside acceptable window")
	}

	// Generate expected signature
	message := fmt.Sprintf("%d|%s", timestamp, nonce)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	// Compare signatures in constant time
	if !hmac.Equal([]byte(expectedSignature), []byte(signature)) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

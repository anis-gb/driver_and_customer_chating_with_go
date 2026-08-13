package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run generate_ws_url.go <user_id>")
		fmt.Println("Example: go run generate_ws_url.go 44444444-4444-4444-4444-444444444444")
		return
	}

	userID := os.Args[1]

	_ = godotenv.Load()
	secret := os.Getenv("HMAC_SECRET")
	if secret == "" {
		secret = "b9f3f1c8f0a74d4e9a2d8c1e7f6b5a3c_test_private"
	}

	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	// Generate random 16 byte nonce
	nonceBytes := make([]byte, 16)
	rand.Read(nonceBytes)
	nonce := hex.EncodeToString(nonceBytes)

	// Generate signature
	message := fmt.Sprintf("%s|%s", timestamp, nonce)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	signature := hex.EncodeToString(mac.Sum(nil))

	fmt.Println("\n=== PASTE THIS URL INTO POSTMAN WEBSOCKET ===")
	fmt.Printf("ws://localhost:8080/ws?user_id=%s&timestamp=%s&nonce=%s&signature=%s\n", userID, timestamp, nonce, signature)
	fmt.Println("=============================================\n")
}

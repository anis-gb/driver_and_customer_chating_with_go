package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all application configuration values.
type Config struct {
	Port        string
	DatabaseURL string
	Env         string
	HMACSecret  string
	VendorChatAPIURL string
	VendorSecretKey  string
}

// Load reads configuration from a .env file (if present) and environment
// variables. Environment variables always take precedence over .env values.
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, relying on environment variables")
	}

	cfg := &Config{
		Port:             getEnv("PORT", "8080"),
		DatabaseURL:      getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/chat_api?sslmode=disable"),
		Env:              getEnv("APP_ENV", "development"),
		HMACSecret:       getEnv("HMAC_SECRET", "b9f3f1c8f0a74d4e9a2d8c1e7f6b5a3c_test_private"),
		VendorChatAPIURL: getEnv("VENDOR_CHAT_API_URL", "https://fastcom.ascendai.site/app/api/v1/chat"),
		VendorSecretKey:  getEnv("VENDOR_SECRET_KEY", "sk_live_test_secret_key"),
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

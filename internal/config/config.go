package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	DatabaseURL             string
	RedisURL                string
	RedisPassword           string
	JWTSecret               string
	AccessTokenExpiry       time.Duration
	RefreshTokenExpiry      time.Duration
	GRPCPort                string
	HTTPPort                string
	InventoryServiceURL     string
	InventoryServiceHTTPURL string
	CustomerServiceURL      string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if file doesn't exist)
	_ = godotenv.Load()

	accessExpiry, _ := strconv.Atoi(getEnv("ACCESS_TOKEN_EXPIRY", "900"))      // 15 minutes
	refreshExpiry, _ := strconv.Atoi(getEnv("REFRESH_TOKEN_EXPIRY", "604800")) // 7 days

	accessTokenExpiry := time.Duration(accessExpiry) * time.Second
	refreshTokenExpiry := time.Duration(refreshExpiry) * time.Second

	return &Config{
		DatabaseURL:             getEnv("DATABASE_URL", ""),
		RedisURL:                getEnv("REDIS_URL", "localhost:6379"),
		RedisPassword:           getEnv("REDIS_PASSWORD", ""),
		GRPCPort:                getEnv("GRPC_PORT", "50051"),
		HTTPPort:                getEnv("HTTP_PORT", "8088"),
		JWTSecret:               getEnv("JWT_SECRET", "your-secret-key"),
		AccessTokenExpiry:       accessTokenExpiry,
		RefreshTokenExpiry:      refreshTokenExpiry,
		InventoryServiceURL:     getEnv("INVENTORY_SERVICE_URL", "serviceandparts-service:50057"),
		InventoryServiceHTTPURL: getEnv("INVENTORY_SERVICE_HTTP_URL", "http://serviceandparts-service:8087"),
		CustomerServiceURL:      getEnv("CUSTOMER_SERVICE_URL", "http://customer-service:8084"),
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

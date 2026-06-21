package config

import (
	"fmt"
	"os"
	"strconv"
)

// Base holds common configuration shared by all backend services.
type Base struct {
	ServiceName string
	Environment string
	HTTPPort    int
	LogLevel    string
}

// Load reads configuration from environment variables with sensible defaults.
func Load(serviceName string, defaultPort int) (Base, error) {
	port, err := strconv.Atoi(getEnv("HTTP_PORT", strconv.Itoa(defaultPort)))
	if err != nil {
		return Base{}, fmt.Errorf("invalid HTTP_PORT: %w", err)
	}

	return Base{
		ServiceName: serviceName,
		Environment: getEnv("ENVIRONMENT", "development"),
		HTTPPort:    port,
		LogLevel:    getEnv("LOG_LEVEL", "info"),
	}, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

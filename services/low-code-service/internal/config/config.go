package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	ServiceName         string
	Environment         string
	HTTPPort            int
	LogLevel            string
	DatabaseURL         string
	IdentityServiceURL  string
	AdminAuthEnabled    bool
}

func Load() (Config, error) {
	portRaw := os.Getenv("LOW_CODE_SERVICE_PORT")
	if portRaw == "" {
		portRaw = os.Getenv("HTTP_PORT")
	}
	if portRaw == "" {
		portRaw = "8088"
	}

	port, err := strconv.Atoi(portRaw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid LOW_CODE_SERVICE_PORT: %w", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://freight:freight_password@localhost:5432/freight_platform?sslmode=disable"
	}

	return Config{
		ServiceName:        "low-code-service",
		Environment:        getEnv("ENVIRONMENT", "development"),
		HTTPPort:           port,
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		DatabaseURL:        databaseURL,
		IdentityServiceURL: getEnv("IDENTITY_SERVICE_URL", "http://identity-service:8081"),
		AdminAuthEnabled:   getEnvBool("LOW_CODE_ADMIN_AUTH_ENABLED", false),
	}, nil
}

func getEnvBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

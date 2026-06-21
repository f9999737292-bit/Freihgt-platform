package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	ServiceName          string
	Environment          string
	HTTPPort             int
	LogLevel             string
	DatabaseURL          string
	JWTSecret            string
	JWTAccessTokenTTLMin int
}

func Load() (Config, error) {
	portRaw := os.Getenv("IDENTITY_SERVICE_PORT")
	if portRaw == "" {
		portRaw = os.Getenv("HTTP_PORT")
	}
	if portRaw == "" {
		portRaw = "8081"
	}

	port, err := strconv.Atoi(portRaw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid IDENTITY_SERVICE_PORT: %w", err)
	}

	ttlRaw := os.Getenv("JWT_ACCESS_TOKEN_TTL_MINUTES")
	if ttlRaw == "" {
		ttlRaw = "60"
	}
	ttl, err := strconv.Atoi(ttlRaw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid JWT_ACCESS_TOKEN_TTL_MINUTES: %w", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://freight:freight_password@localhost:5432/freight_platform?sslmode=disable"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "dev_secret_change_me"
	}

	return Config{
		ServiceName:          "identity-service",
		Environment:          getEnv("ENVIRONMENT", "development"),
		HTTPPort:             port,
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		DatabaseURL:          databaseURL,
		JWTSecret:            jwtSecret,
		JWTAccessTokenTTLMin: ttl,
	}, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	ServiceName          string
	Environment          string
	HTTPPort             int
	LogLevel             string
	DatabaseURL          string
	JWTSecret                 string
	JWTAccessTokenTTLMin      int
	IntegrationTokenTTLMin    int
	IntegrationOAuthRateLimit   int
	InternalServiceToken        string
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

	integrationTTL := 15
	if raw := strings.TrimSpace(os.Getenv("INTEGRATION_TOKEN_TTL_MINUTES")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			integrationTTL = parsed
		}
	}
	oauthRateLimit := 30
	if raw := strings.TrimSpace(os.Getenv("INTEGRATION_OAUTH_RATE_LIMIT_PER_MIN")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			oauthRateLimit = parsed
		}
	}

	return Config{
		ServiceName:               "identity-service",
		Environment:               getEnv("ENVIRONMENT", "development"),
		HTTPPort:                  port,
		LogLevel:                  getEnv("LOG_LEVEL", "info"),
		DatabaseURL:               databaseURL,
		JWTSecret:                 jwtSecret,
		JWTAccessTokenTTLMin:      ttl,
		IntegrationTokenTTLMin:    integrationTTL,
		IntegrationOAuthRateLimit: oauthRateLimit,
		InternalServiceToken:      getEnv("INTERNAL_SERVICE_TOKEN", ""),
	}, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

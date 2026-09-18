package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/freight-platform/shared-go/clientip"
)

type Config struct {
	ServiceName                 string
	Environment                 string
	HTTPPort                    int
	LogLevel                    string
	DatabaseURL                 string
	JWTSecret                   string
	IntegrationJWTSecret        string
	JWTAccessTokenTTLMin        int
	IntegrationTokenTTLMin      int
	IntegrationOAuthRateLimit   int
	IntegrationOAuthFailedLimit int
	TrustedProxyCIDRs           string
	TrustedProxyNetworks        []*net.IPNet
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
	environment := getEnv("ENVIRONMENT", "development")
	integrationJWTSecret := strings.TrimSpace(os.Getenv("INTEGRATION_JWT_SECRET"))
	if integrationJWTSecret == "" {
		if isProductionEnvironment(environment) {
			return Config{}, fmt.Errorf("INTEGRATION_JWT_SECRET is required in production")
		}
		integrationJWTSecret = "dev_integration_jwt_secret_change_me"
	}
	trustedProxyCIDRs := strings.TrimSpace(os.Getenv("TRUSTED_PROXY_CIDRS"))
	trustedProxyNetworks, err := clientip.ParseTrustedProxyCIDRs(trustedProxyCIDRs)
	if err != nil {
		return Config{}, fmt.Errorf("invalid TRUSTED_PROXY_CIDRS: %w", err)
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
	oauthFailedLimit := 30
	if raw := strings.TrimSpace(os.Getenv("INTEGRATION_OAUTH_FAILED_RATE_LIMIT_PER_MIN")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			oauthFailedLimit = parsed
		}
	}

	return Config{
		ServiceName:                 "identity-service",
		Environment:                 environment,
		HTTPPort:                    port,
		LogLevel:                    getEnv("LOG_LEVEL", "info"),
		DatabaseURL:                 databaseURL,
		JWTSecret:                   jwtSecret,
		IntegrationJWTSecret:        integrationJWTSecret,
		JWTAccessTokenTTLMin:        ttl,
		IntegrationTokenTTLMin:      integrationTTL,
		IntegrationOAuthRateLimit:   oauthRateLimit,
		IntegrationOAuthFailedLimit: oauthFailedLimit,
		TrustedProxyCIDRs:           trustedProxyCIDRs,
		TrustedProxyNetworks:        trustedProxyNetworks,
		InternalServiceToken:        getEnv("INTERNAL_SERVICE_TOKEN", ""),
	}, nil
}

func isProductionEnvironment(environment string) bool {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "production", "prod", "staging":
		return true
	default:
		return false
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

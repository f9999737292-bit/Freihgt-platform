package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	ServiceName string
	Environment string
	HTTPPort    int
	LogLevel    string
	DatabaseURL string
}

func Load() (Config, error) {
	portRaw := os.Getenv("RFX_SERVICE_PORT")
	if portRaw == "" {
		portRaw = os.Getenv("HTTP_PORT")
	}
	if portRaw == "" {
		portRaw = "8084"
	}

	port, err := strconv.Atoi(portRaw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid RFX_SERVICE_PORT: %w", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://freight:freight_password@localhost:5432/freight_platform?sslmode=disable"
	}

	return Config{
		ServiceName: "rfx-service",
		Environment: getEnv("ENVIRONMENT", "development"),
		HTTPPort:    port,
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		DatabaseURL: databaseURL,
	}, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

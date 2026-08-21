package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	ServiceName            string
	Environment            string
	HTTPPort               int
	LogLevel               string
	InternalServiceToken   string
	TransportOrderURL      string
}

func Load() (Config, error) {
	portRaw := os.Getenv("FREIGHT_COST_SERVICE_PORT")
	if portRaw == "" {
		portRaw = os.Getenv("HTTP_PORT")
	}
	if portRaw == "" {
		portRaw = "8092"
	}

	port, err := strconv.Atoi(portRaw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid FREIGHT_COST_SERVICE_PORT: %w", err)
	}

	transportURL := os.Getenv("TRANSPORT_ORDER_SERVICE_URL")
	if transportURL == "" {
		transportURL = "http://transport-order-service:8083"
	}

	return Config{
		ServiceName:          "freight-cost-service",
		Environment:          getEnv("ENVIRONMENT", "development"),
		HTTPPort:             port,
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		InternalServiceToken: os.Getenv("INTERNAL_SERVICE_TOKEN"),
		TransportOrderURL:    transportURL,
	}, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

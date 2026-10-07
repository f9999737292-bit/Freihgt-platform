package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ServiceName   string
	Environment   string
	Port          int
	LogLevel      string
	ShipmentURL   string
	InternalToken string
	SourceTimeout time.Duration
}

func Load() (Config, error) {
	portRaw := strings.TrimSpace(os.Getenv("ANALYTICS_SERVICE_PORT"))
	if portRaw == "" {
		portRaw = "8097"
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil || port <= 0 {
		return Config{}, fmt.Errorf("ANALYTICS_SERVICE_PORT must be a positive integer")
	}
	environment := strings.TrimSpace(os.Getenv("ENVIRONMENT"))
	if environment == "" {
		return Config{}, fmt.Errorf("ENVIRONMENT is required")
	}
	shipmentURL := strings.TrimRight(strings.TrimSpace(os.Getenv("SHIPMENT_SERVICE_URL")), "/")
	if shipmentURL == "" {
		return Config{}, fmt.Errorf("SHIPMENT_SERVICE_URL is required")
	}
	token := strings.TrimSpace(os.Getenv("INTERNAL_SERVICE_TOKEN"))
	if token == "" {
		return Config{}, fmt.Errorf("INTERNAL_SERVICE_TOKEN is required")
	}
	timeout := 5 * time.Second
	if raw := strings.TrimSpace(os.Getenv("SHIPMENT_SOURCE_TIMEOUT")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed <= 0 {
			return Config{}, fmt.Errorf("SHIPMENT_SOURCE_TIMEOUT must be a positive duration")
		}
		timeout = parsed
	}
	return Config{
		ServiceName:   "analytics-service",
		Environment:   environment,
		Port:          port,
		LogLevel:      envDefault("LOG_LEVEL", "info"),
		ShipmentURL:   shipmentURL,
		InternalToken: token,
		SourceTimeout: timeout,
	}, nil
}

func envDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

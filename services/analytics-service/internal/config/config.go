package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ServiceName        string
	Environment        string
	Port               int
	LogLevel           string
	ShipmentURL        string
	GatewayCallerToken string
	SourceCallerToken  string
	SourceTimeout      time.Duration
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
	gatewayToken := strings.TrimSpace(os.Getenv("ANALYTICS_GATEWAY_CALLER_TOKEN"))
	sourceToken := strings.TrimSpace(os.Getenv("ANALYTICS_SOURCE_CALLER_TOKEN"))
	sharedToken := strings.TrimSpace(os.Getenv("INTERNAL_SERVICE_TOKEN"))
	if err := validateCallerSecrets(gatewayToken, sourceToken, sharedToken); err != nil {
		return Config{}, err
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
		ServiceName:        "analytics-service",
		Environment:        environment,
		Port:               port,
		LogLevel:           envDefault("LOG_LEVEL", "info"),
		ShipmentURL:        shipmentURL,
		GatewayCallerToken: gatewayToken,
		SourceCallerToken:  sourceToken,
		SourceTimeout:      timeout,
	}, nil
}

// validateCallerSecrets rejects empty credentials and any pair that would let
// the shared platform token or one hop's secret authorize the other hop.
// The error text does not include secret values.
func validateCallerSecrets(gatewayToken, sourceToken, sharedToken string) error {
	if gatewayToken == "" || sourceToken == "" {
		return fmt.Errorf("ANALYTICS_GATEWAY_CALLER_TOKEN and ANALYTICS_SOURCE_CALLER_TOKEN are required")
	}
	if gatewayToken == sourceToken || (sharedToken != "" && (gatewayToken == sharedToken || sourceToken == sharedToken)) {
		return fmt.Errorf("analytics caller tokens must be distinct from each other and from INTERNAL_SERVICE_TOKEN")
	}
	return nil
}

func envDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

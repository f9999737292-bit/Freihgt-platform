package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ServiceName            string
	Environment            string
	HTTPPort               int
	LogLevel               string
	DatabaseURL            string
	InternalServiceToken   string
	TransportOrderURL      string
	BillingRegisterURL     string
	PaymentServiceURL      string
	AnalyticsProjection    AnalyticsProjectionConfig
}

type AnalyticsProjectionConfig struct {
	Enabled           bool
	DirtyPollInterval time.Duration
	DirtyBatchSize    int
	RebuildInterval   time.Duration
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

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://freight:freight_password@localhost:5432/freight_platform?sslmode=disable"
	}

	transportURL := os.Getenv("TRANSPORT_ORDER_SERVICE_URL")
	if transportURL == "" {
		transportURL = "http://transport-order-service:8083"
	}

	billingURL := os.Getenv("BILLING_REGISTER_SERVICE_URL")
	if billingURL == "" {
		billingURL = "http://billing-register-service:8087"
	}

	paymentURL := os.Getenv("PAYMENT_SERVICE_URL")
	if paymentURL == "" {
		paymentURL = "http://payment-service:8090"
	}

	return Config{
		ServiceName:          "freight-cost-service",
		Environment:          getEnv("ENVIRONMENT", "development"),
		HTTPPort:             port,
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		DatabaseURL:          databaseURL,
		InternalServiceToken: os.Getenv("INTERNAL_SERVICE_TOKEN"),
		TransportOrderURL:    transportURL,
		BillingRegisterURL:   billingURL,
		PaymentServiceURL:    paymentURL,
		AnalyticsProjection:  loadAnalyticsProjectionConfig(),
	}, nil
}

func loadAnalyticsProjectionConfig() AnalyticsProjectionConfig {
	enabled := parseBool(getEnv("FREIGHT_COST_ANALYTICS_PROJECTION_ENABLED", "false"))
	dirtyPoll, err := parseDuration(getEnv("FREIGHT_COST_ANALYTICS_DIRTY_POLL_INTERVAL", "5s"))
	if err != nil {
		dirtyPoll = 5 * time.Second
	}
	rebuildInterval, err := parseDuration(getEnv("FREIGHT_COST_ANALYTICS_REBUILD_INTERVAL", "24h"))
	if err != nil {
		rebuildInterval = 24 * time.Hour
	}
	batchSize, err := strconv.Atoi(getEnv("FREIGHT_COST_ANALYTICS_DIRTY_BATCH_SIZE", "50"))
	if err != nil || batchSize <= 0 {
		batchSize = 50
	}
	return AnalyticsProjectionConfig{
		Enabled:           enabled,
		DirtyPollInterval: dirtyPoll,
		DirtyBatchSize:    batchSize,
		RebuildInterval:   rebuildInterval,
	}
}

func parseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseDuration(raw string) (time.Duration, error) {
	return time.ParseDuration(strings.TrimSpace(raw))
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

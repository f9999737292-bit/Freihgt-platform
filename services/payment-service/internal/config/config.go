package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Config struct {
	ServiceName          string
	Environment          string
	HTTPPort               int
	LogLevel               string
	DatabaseURL            string
	BillingRegisterURL     string
	FreightCostServiceURL  string
	InternalServiceToken   string
	Outbox                 OutboxConfig
}

type OutboxConfig struct {
	Enabled        bool
	PollInterval   time.Duration
	BatchSize      int
	PublishTimeout time.Duration
	LeaseTimeout   time.Duration
	MaxAttempts    int
	WorkerID       string
}

func Load() (Config, error) {
	portRaw := os.Getenv("PAYMENT_SERVICE_PORT")
	if portRaw == "" {
		portRaw = os.Getenv("HTTP_PORT")
	}
	if portRaw == "" {
		portRaw = "8090"
	}

	port, err := strconv.Atoi(portRaw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid PAYMENT_SERVICE_PORT: %w", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://freight:freight_password@localhost:5432/freight_platform?sslmode=disable"
	}

	billingURL := os.Getenv("BILLING_REGISTER_SERVICE_URL")
	if billingURL == "" {
		billingURL = "http://localhost:8087"
	}
	freightCostURL := os.Getenv("FREIGHT_COST_SERVICE_URL")
	if freightCostURL == "" {
		freightCostURL = "http://localhost:8092"
	}

	outbox, err := loadOutboxConfig()
	if err != nil {
		return Config{}, err
	}

	return Config{
		ServiceName:          "payment-service",
		Environment:          getEnv("ENVIRONMENT", "development"),
		HTTPPort:             port,
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		DatabaseURL:          databaseURL,
		BillingRegisterURL:   billingURL,
		FreightCostServiceURL: freightCostURL,
		InternalServiceToken: os.Getenv("INTERNAL_SERVICE_TOKEN"),
		Outbox:               outbox,
	}, nil
}

func loadOutboxConfig() (OutboxConfig, error) {
	enabled := parseBool(getEnv("PAYMENT_OUTBOX_ENABLED", "false"))
	pollInterval, err := parseDuration(getEnv("PAYMENT_OUTBOX_POLL_INTERVAL", "2s"))
	if err != nil {
		return OutboxConfig{}, fmt.Errorf("invalid PAYMENT_OUTBOX_POLL_INTERVAL: %w", err)
	}
	batchSize, err := strconv.Atoi(getEnv("PAYMENT_OUTBOX_BATCH_SIZE", "50"))
	if err != nil {
		return OutboxConfig{}, fmt.Errorf("invalid PAYMENT_OUTBOX_BATCH_SIZE: %w", err)
	}
	publishTimeout, err := parseDuration(getEnv("PAYMENT_OUTBOX_PUBLISH_TIMEOUT", "10s"))
	if err != nil {
		return OutboxConfig{}, fmt.Errorf("invalid PAYMENT_OUTBOX_PUBLISH_TIMEOUT: %w", err)
	}
	leaseTimeout, err := parseDuration(getEnv("PAYMENT_OUTBOX_LEASE_TIMEOUT", "60s"))
	if err != nil {
		return OutboxConfig{}, fmt.Errorf("invalid PAYMENT_OUTBOX_LEASE_TIMEOUT: %w", err)
	}
	maxAttempts, err := strconv.Atoi(getEnv("PAYMENT_OUTBOX_MAX_ATTEMPTS", "5"))
	if err != nil {
		return OutboxConfig{}, fmt.Errorf("invalid PAYMENT_OUTBOX_MAX_ATTEMPTS: %w", err)
	}
	workerID := strings.TrimSpace(os.Getenv("PAYMENT_OUTBOX_WORKER_ID"))
	if workerID == "" {
		host, _ := os.Hostname()
		if host == "" {
			host = "payment-service"
		}
		workerID = host + "-" + uuid.NewString()
	}

	cfg := OutboxConfig{
		Enabled:        enabled,
		PollInterval:   pollInterval,
		BatchSize:      batchSize,
		PublishTimeout: publishTimeout,
		LeaseTimeout:   leaseTimeout,
		MaxAttempts:    maxAttempts,
		WorkerID:       workerID,
	}
	if err := cfg.Validate(); err != nil {
		return OutboxConfig{}, err
	}
	return cfg, nil
}

func (c OutboxConfig) Validate() error {
	if c.BatchSize <= 0 {
		return fmt.Errorf("PAYMENT_OUTBOX_BATCH_SIZE must be > 0")
	}
	if c.MaxAttempts <= 0 {
		return fmt.Errorf("PAYMENT_OUTBOX_MAX_ATTEMPTS must be > 0")
	}
	if c.PublishTimeout <= 0 {
		return fmt.Errorf("PAYMENT_OUTBOX_PUBLISH_TIMEOUT must be > 0")
	}
	if c.LeaseTimeout <= c.PublishTimeout {
		return fmt.Errorf("PAYMENT_OUTBOX_LEASE_TIMEOUT must be greater than PAYMENT_OUTBOX_PUBLISH_TIMEOUT")
	}
	if c.PollInterval <= 0 {
		return fmt.Errorf("PAYMENT_OUTBOX_POLL_INTERVAL must be > 0")
	}
	if strings.TrimSpace(c.WorkerID) == "" {
		return fmt.Errorf("PAYMENT_OUTBOX_WORKER_ID must not be empty")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseDuration(value string) (time.Duration, error) {
	return time.ParseDuration(strings.TrimSpace(value))
}

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
	HTTPPort             int
	LogLevel             string
	DatabaseURL          string
	Outbox               OutboxConfig
	InternalServiceToken string
	Notification         NotificationConfig
	FCM                  FCMConfig
}

type NotificationConfig struct {
	Enabled      bool
	WorkerID     string
	PollInterval time.Duration
	BatchSize    int
	LeaseTimeout time.Duration
	MaxAttempts  int
	RetryBackoff time.Duration
}

type FCMConfig struct {
	ProjectID   string
	AccessToken string
}

type OutboxConfig struct {
	Enabled        bool
	Transport      string
	PollInterval   time.Duration
	BatchSize      int
	PublishTimeout time.Duration
	LeaseTimeout   time.Duration
	MaxAttempts    int
	WorkerID       string
	Kafka          KafkaConfig
}

func Load() (Config, error) {
	portRaw := os.Getenv("SHIPMENT_SERVICE_PORT")
	if portRaw == "" {
		portRaw = os.Getenv("HTTP_PORT")
	}
	if portRaw == "" {
		portRaw = "8085"
	}

	port, err := strconv.Atoi(portRaw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid SHIPMENT_SERVICE_PORT: %w", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://freight:freight_password@localhost:5432/freight_platform?sslmode=disable"
	}

	outbox, err := loadOutboxConfig()
	if err != nil {
		return Config{}, err
	}
	notification, err := loadNotificationConfig()
	if err != nil {
		return Config{}, err
	}

	return Config{
		ServiceName:          "shipment-service",
		Environment:          getEnv("ENVIRONMENT", "development"),
		HTTPPort:             port,
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		DatabaseURL:          databaseURL,
		Outbox:               outbox,
		InternalServiceToken: strings.TrimSpace(os.Getenv("INTERNAL_SERVICE_TOKEN")),
		Notification:         notification,
		FCM: FCMConfig{
			ProjectID:   strings.TrimSpace(os.Getenv("FCM_PROJECT_ID")),
			AccessToken: strings.TrimSpace(os.Getenv("FCM_ACCESS_TOKEN")),
		},
	}, nil
}

func loadNotificationConfig() (NotificationConfig, error) {
	pollInterval, err := parseDuration(getEnv("DRIVER_NOTIFICATION_POLL_INTERVAL", "2s"))
	if err != nil {
		return NotificationConfig{}, err
	}
	leaseTimeout, err := parseDuration(getEnv("DRIVER_NOTIFICATION_LEASE_TIMEOUT", "30s"))
	if err != nil {
		return NotificationConfig{}, err
	}
	retryBackoff, err := parseDuration(getEnv("DRIVER_NOTIFICATION_RETRY_BACKOFF", "5s"))
	if err != nil {
		return NotificationConfig{}, err
	}
	batchSize, _ := strconv.Atoi(getEnv("DRIVER_NOTIFICATION_BATCH_SIZE", "10"))
	maxAttempts, _ := strconv.Atoi(getEnv("DRIVER_NOTIFICATION_MAX_ATTEMPTS", "3"))
	workerID := strings.TrimSpace(os.Getenv("DRIVER_NOTIFICATION_WORKER_ID"))
	if workerID == "" {
		workerID = "driver-notification-" + uuid.NewString()
	}
	return NotificationConfig{
		Enabled:      parseBool(getEnv("DRIVER_NOTIFICATION_ENABLED", "true")),
		WorkerID:     workerID,
		PollInterval: pollInterval,
		BatchSize:    batchSize,
		LeaseTimeout: leaseTimeout,
		MaxAttempts:  maxAttempts,
		RetryBackoff: retryBackoff,
	}, nil
}

func loadOutboxConfig() (OutboxConfig, error) {
	enabled := parseBool(getEnv("SHIPMENT_OUTBOX_ENABLED", "false"))
	pollInterval, err := parseDuration(getEnv("SHIPMENT_OUTBOX_POLL_INTERVAL", "2s"))
	if err != nil {
		return OutboxConfig{}, fmt.Errorf("invalid SHIPMENT_OUTBOX_POLL_INTERVAL: %w", err)
	}
	batchSize, err := strconv.Atoi(getEnv("SHIPMENT_OUTBOX_BATCH_SIZE", "50"))
	if err != nil {
		return OutboxConfig{}, fmt.Errorf("invalid SHIPMENT_OUTBOX_BATCH_SIZE: %w", err)
	}
	publishTimeout, err := parseDuration(getEnv("SHIPMENT_OUTBOX_PUBLISH_TIMEOUT", "10s"))
	if err != nil {
		return OutboxConfig{}, fmt.Errorf("invalid SHIPMENT_OUTBOX_PUBLISH_TIMEOUT: %w", err)
	}
	leaseTimeout, err := parseDuration(getEnv("SHIPMENT_OUTBOX_LEASE_TIMEOUT", "60s"))
	if err != nil {
		return OutboxConfig{}, fmt.Errorf("invalid SHIPMENT_OUTBOX_LEASE_TIMEOUT: %w", err)
	}
	maxAttempts, err := strconv.Atoi(getEnv("SHIPMENT_OUTBOX_MAX_ATTEMPTS", "5"))
	if err != nil {
		return OutboxConfig{}, fmt.Errorf("invalid SHIPMENT_OUTBOX_MAX_ATTEMPTS: %w", err)
	}
	workerID := strings.TrimSpace(os.Getenv("SHIPMENT_OUTBOX_WORKER_ID"))
	if workerID == "" {
		host, _ := os.Hostname()
		if host == "" {
			host = "shipment-service"
		}
		workerID = host + "-" + uuid.NewString()
	}

	kafka, err := loadKafkaConfig()
	if err != nil {
		return OutboxConfig{}, err
	}

	cfg := OutboxConfig{
		Enabled:        enabled,
		Transport:      strings.TrimSpace(os.Getenv("SHIPMENT_OUTBOX_TRANSPORT")),
		PollInterval:   pollInterval,
		BatchSize:      batchSize,
		PublishTimeout: publishTimeout,
		LeaseTimeout:   leaseTimeout,
		MaxAttempts:    maxAttempts,
		WorkerID:       workerID,
		Kafka:          kafka,
	}
	if err := cfg.Validate(); err != nil {
		return OutboxConfig{}, err
	}
	return cfg, nil
}

func (c OutboxConfig) Validate() error {
	if c.BatchSize <= 0 {
		return fmt.Errorf("SHIPMENT_OUTBOX_BATCH_SIZE must be > 0")
	}
	if c.MaxAttempts <= 0 {
		return fmt.Errorf("SHIPMENT_OUTBOX_MAX_ATTEMPTS must be > 0")
	}
	if c.PublishTimeout <= 0 {
		return fmt.Errorf("SHIPMENT_OUTBOX_PUBLISH_TIMEOUT must be > 0")
	}
	if c.LeaseTimeout <= c.PublishTimeout {
		return fmt.Errorf("SHIPMENT_OUTBOX_LEASE_TIMEOUT must be greater than SHIPMENT_OUTBOX_PUBLISH_TIMEOUT")
	}
	if c.PollInterval <= 0 {
		return fmt.Errorf("SHIPMENT_OUTBOX_POLL_INTERVAL must be > 0")
	}
	if strings.TrimSpace(c.WorkerID) == "" {
		return fmt.Errorf("SHIPMENT_OUTBOX_WORKER_ID must not be empty")
	}
	if c.Enabled && strings.TrimSpace(c.Transport) == "" {
		return fmt.Errorf("SHIPMENT_OUTBOX_ENABLED=true requires SHIPMENT_OUTBOX_TRANSPORT")
	}
	if c.Enabled && strings.EqualFold(strings.TrimSpace(c.Transport), "kafka") {
		if err := c.Kafka.ValidateRequired(); err != nil {
			return err
		}
	}
	return nil
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

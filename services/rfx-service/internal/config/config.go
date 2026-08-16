package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ServiceName    string
	Environment    string
	HTTPPort       int
	LogLevel       string
	DatabaseURL    string
	DeadlineWorker DeadlineWorkerConfig
}

type DeadlineWorkerConfig struct {
	Enabled   bool
	Interval  time.Duration
	BatchSize int
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

	deadlineWorker, err := loadDeadlineWorkerConfig()
	if err != nil {
		return Config{}, err
	}

	return Config{
		ServiceName:    "rfx-service",
		Environment:    getEnv("ENVIRONMENT", "development"),
		HTTPPort:       port,
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		DatabaseURL:    databaseURL,
		DeadlineWorker: deadlineWorker,
	}, nil
}

func loadDeadlineWorkerConfig() (DeadlineWorkerConfig, error) {
	intervalSec := intEnv("RFX_DEADLINE_WORKER_INTERVAL_SECONDS", 60)
	if intervalSec <= 0 {
		return DeadlineWorkerConfig{}, fmt.Errorf("RFX_DEADLINE_WORKER_INTERVAL_SECONDS must be positive")
	}
	batchSize := intEnv("RFX_DEADLINE_WORKER_BATCH_SIZE", 50)
	if batchSize <= 0 {
		return DeadlineWorkerConfig{}, fmt.Errorf("RFX_DEADLINE_WORKER_BATCH_SIZE must be positive")
	}
	if batchSize > 500 {
		return DeadlineWorkerConfig{}, fmt.Errorf("RFX_DEADLINE_WORKER_BATCH_SIZE must be <= 500")
	}
	return DeadlineWorkerConfig{
		Enabled:   parseBool(getEnv("RFX_DEADLINE_WORKER_ENABLED", "false")),
		Interval:  time.Duration(intervalSec) * time.Second,
		BatchSize: batchSize,
	}, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func parseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

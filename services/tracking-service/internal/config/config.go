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
	ProviderSecrets        map[string]string
	InternalServiceToken   string
	FreshnessPolicy        FreshnessConfig
	ETAFreshnessPolicy     FreshnessConfig
	TrackingLossDetector   TrackingLossDetectorConfig
}

type FreshnessConfig struct {
	FreshThreshold time.Duration
	StaleThreshold time.Duration
}

type TrackingLossDetectorConfig struct {
	Enabled   bool
	Threshold time.Duration
	Interval  time.Duration
	BatchSize int
}

func Load() (Config, error) {
	portRaw := os.Getenv("TRACKING_SERVICE_PORT")
	if portRaw == "" {
		portRaw = os.Getenv("HTTP_PORT")
	}
	if portRaw == "" {
		portRaw = "8089"
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid TRACKING_SERVICE_PORT: %w", err)
	}

	freshMin := intEnv("TRACKING_FRESH_THRESHOLD_MINUTES", 10)
	staleMin := intEnv("TRACKING_STALE_THRESHOLD_MINUTES", 30)
	if staleMin <= freshMin {
		return Config{}, fmt.Errorf("TRACKING_STALE_THRESHOLD_MINUTES must be greater than TRACKING_FRESH_THRESHOLD_MINUTES")
	}

	etaFreshMin := intEnv("ETA_FRESH_THRESHOLD_MINUTES", 15)
	etaStaleMin := intEnv("ETA_STALE_THRESHOLD_MINUTES", 60)
	if etaStaleMin <= etaFreshMin {
		return Config{}, fmt.Errorf("ETA_STALE_THRESHOLD_MINUTES must be greater than ETA_FRESH_THRESHOLD_MINUTES")
	}

	return Config{
		ServiceName:          "tracking-service",
		Environment:          getEnv("ENVIRONMENT", "development"),
		HTTPPort:             port,
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://freight:freight_password@localhost:5432/freight_platform?sslmode=disable"),
		ProviderSecrets:      parseProviderSecrets(getEnv("TRACKING_PROVIDER_SECRETS", getEnv("TRACKING_GENERIC_PROVIDER_SECRET", "dev_generic_tracking_secret"))),
		InternalServiceToken: getEnv("TRACKING_INTERNAL_SERVICE_TOKEN", getEnv("INTERNAL_SERVICE_TOKEN", "dev_internal_tracking_token")),
		FreshnessPolicy: FreshnessConfig{
			FreshThreshold: time.Duration(freshMin) * time.Minute,
			StaleThreshold: time.Duration(staleMin) * time.Minute,
		},
		ETAFreshnessPolicy: FreshnessConfig{
			FreshThreshold: time.Duration(etaFreshMin) * time.Minute,
			StaleThreshold: time.Duration(etaStaleMin) * time.Minute,
		},
		TrackingLossDetector: loadTrackingLossDetectorConfig(staleMin),
	}, nil
}

func loadTrackingLossDetectorConfig(defaultStaleMinutes int) TrackingLossDetectorConfig {
	thresholdMin := intEnv("CONTROL_TOWER_DRIVER_TRACKING_LOST_AFTER_MINUTES", defaultStaleMinutes)
	if thresholdMin <= 0 {
		thresholdMin = defaultStaleMinutes
	}
	intervalSec := intEnv("TRACKING_LOSS_DETECTOR_INTERVAL_SECONDS", 60)
	if intervalSec <= 0 {
		intervalSec = 60
	}
	batchSize := intEnv("TRACKING_LOSS_DETECTOR_BATCH_SIZE", 100)
	return TrackingLossDetectorConfig{
		Enabled:   parseBool(getEnv("TRACKING_LOSS_DETECTOR_ENABLED", "true")),
		Threshold: time.Duration(thresholdMin) * time.Minute,
		Interval:  time.Duration(intervalSec) * time.Second,
		BatchSize: batchSize,
	}
}

func parseProviderSecrets(raw string) map[string]string {
	out := map[string]string{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return out
	}
	if !strings.Contains(raw, ":") && !strings.Contains(raw, ",") {
		out["generic"] = raw
		return out
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		out[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	return out
}

func parseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
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

func intEnv(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

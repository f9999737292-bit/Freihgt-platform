package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Command         string
	Environment     string
	Commit          string
	GatewayURL      string
	ReadModelURL    string
	PrometheusURL   string
	JWTToken        string
	TenantID        string
	AdminEmail      string
	AdminPassword   string
	CohortManifest  string
	KafkaTopic      string
	KafkaGroup      string
	RpkExec         string
	OutputPath      string
	HTTPTimeout     time.Duration
	MaxRetries      int
	SustainedMismatchMinutes int
	MaxConsumerLag  int64
	RequireMatchRatio float64
}

func LoadConfig() (Config, error) {
	cfg := Config{
		Command:       strings.TrimSpace(os.Getenv("OBSERVATION_COMMAND")),
		Environment:   envOr("OBSERVATION_ENVIRONMENT", "staging"),
		Commit:        strings.TrimSpace(os.Getenv("OBSERVATION_COMMIT")),
		GatewayURL:    strings.TrimRight(envOr("GATEWAY_URL", "http://127.0.0.1:8080"), "/"),
		ReadModelURL:  strings.TrimRight(envOr("READ_MODEL_URL", "http://127.0.0.1:8089"), "/"),
		PrometheusURL: strings.TrimRight(envOr("PROMETHEUS_URL", "http://127.0.0.1:9090"), "/"),
		JWTToken:      strings.TrimSpace(os.Getenv("JWT_TOKEN")),
		TenantID:      strings.TrimSpace(os.Getenv("TENANT_ID")),
		AdminEmail:    strings.TrimSpace(os.Getenv("DEV_ADMIN_EMAIL")),
		AdminPassword: strings.TrimSpace(os.Getenv("DEV_ADMIN_PASSWORD")),
		CohortManifest: strings.TrimSpace(os.Getenv("COHORT_MANIFEST")),
		KafkaTopic:    envOr("CONTROL_TOWER_KAFKA_TOPIC", "shipment.status.v1"),
		KafkaGroup:    envOr("CONTROL_TOWER_KAFKA_GROUP_ID", "control-tower-shipment-status-v1"),
		RpkExec:       envOr("RPK_EXEC", "rpk"),
		OutputPath:    strings.TrimSpace(os.Getenv("OBSERVATION_OUTPUT")),
		HTTPTimeout:   durationEnv("OBSERVATION_HTTP_TIMEOUT_SEC", 15*time.Second),
		MaxRetries:    intEnv("OBSERVATION_MAX_RETRIES", 3),
		SustainedMismatchMinutes: intEnv("OBSERVATION_SUSTAINED_MISMATCH_MIN", 5),
		MaxConsumerLag: int64Env("OBSERVATION_MAX_CONSUMER_LAG", 0),
		RequireMatchRatio: floatEnv("OBSERVATION_REQUIRE_MATCH_RATIO", 1.0),
	}
	if cfg.Command == "" {
		cfg.Command = "gate"
	}
	if cfg.Commit == "" {
		cfg.Commit = strings.TrimSpace(os.Getenv("GIT_COMMIT"))
	}
	if cfg.Commit == "" {
		cfg.Commit = "unknown"
	}
	if cfg.CohortManifest == "" {
		return cfg, fmt.Errorf("COHORT_MANIFEST is required (protected path with alias→tenant mapping)")
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func int64Env(key string, fallback int64) int64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func floatEnv(key string, fallback float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return n
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	sec := intEnv(key, int(fallback/time.Second))
	if sec <= 0 {
		return fallback
	}
	return time.Duration(sec) * time.Second
}

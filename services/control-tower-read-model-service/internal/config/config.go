package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ServiceName string
	Environment string
	HTTPPort    int
	LogLevel    string
	DatabaseURL string
	Consumer    ConsumerConfig
	Kafka       KafkaConfig
}

type ConsumerConfig struct {
	Enabled        bool
	PollTimeout    time.Duration
	ProcessTimeout time.Duration
	CommitTimeout  time.Duration
}

type KafkaConfig struct {
	Brokers       []string
	Topic         string
	GroupID       string
	ClientID      string
	DialTimeout   time.Duration
	TLSEnabled    bool
	TLSCAFile     string
	TLSCertFile   string
	TLSKeyFile    string
	TLSServerName string
	SASLMechanism string
	SASLUsername  string
	SASLPassword  string
}

func Load() (Config, error) {
	portRaw := os.Getenv("CONTROL_TOWER_READ_MODEL_SERVICE_PORT")
	if portRaw == "" {
		portRaw = os.Getenv("CONTROL_TOWER_HTTP_PORT")
	}
	if portRaw == "" {
		portRaw = "8089"
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid CONTROL_TOWER_HTTP_PORT: %w", err)
	}

	databaseURL := os.Getenv("CONTROL_TOWER_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}
	if strings.TrimSpace(databaseURL) == "" {
		return Config{}, fmt.Errorf("CONTROL_TOWER_DATABASE_URL is required")
	}

	consumer, err := loadConsumerConfig()
	if err != nil {
		return Config{}, err
	}
	kafka, err := loadKafkaConfig()
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		ServiceName: "control-tower-read-model-service",
		Environment: getEnv("ENVIRONMENT", "development"),
		HTTPPort:    port,
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		DatabaseURL: databaseURL,
		Consumer:    consumer,
		Kafka:       kafka,
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func loadConsumerConfig() (ConsumerConfig, error) {
	enabled := parseBool(getEnv("CONTROL_TOWER_CONSUMER_ENABLED", "false"))
	pollTimeout, err := parseDuration(getEnv("CONTROL_TOWER_KAFKA_POLL_TIMEOUT", "1s"))
	if err != nil {
		return ConsumerConfig{}, fmt.Errorf("invalid CONTROL_TOWER_KAFKA_POLL_TIMEOUT: %w", err)
	}
	processTimeout, err := parseDuration(getEnv("CONTROL_TOWER_KAFKA_PROCESS_TIMEOUT", "10s"))
	if err != nil {
		return ConsumerConfig{}, fmt.Errorf("invalid CONTROL_TOWER_KAFKA_PROCESS_TIMEOUT: %w", err)
	}
	commitTimeout, err := parseDuration(getEnv("CONTROL_TOWER_KAFKA_COMMIT_TIMEOUT", "5s"))
	if err != nil {
		return ConsumerConfig{}, fmt.Errorf("invalid CONTROL_TOWER_KAFKA_COMMIT_TIMEOUT: %w", err)
	}
	return ConsumerConfig{
		Enabled:        enabled,
		PollTimeout:    pollTimeout,
		ProcessTimeout: processTimeout,
		CommitTimeout:  commitTimeout,
	}, nil
}

func loadKafkaConfig() (KafkaConfig, error) {
	var brokers []string
	for _, part := range strings.Split(strings.TrimSpace(os.Getenv("CONTROL_TOWER_KAFKA_BROKERS")), ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			brokers = append(brokers, part)
		}
	}
	dialTimeout, err := parseDuration(getEnv("CONTROL_TOWER_KAFKA_DIAL_TIMEOUT", "10s"))
	if err != nil {
		return KafkaConfig{}, fmt.Errorf("invalid CONTROL_TOWER_KAFKA_DIAL_TIMEOUT: %w", err)
	}
	return KafkaConfig{
		Brokers:       brokers,
		Topic:         strings.TrimSpace(getEnv("CONTROL_TOWER_KAFKA_TOPIC", "shipment.status.v1")),
		GroupID:       strings.TrimSpace(getEnv("CONTROL_TOWER_KAFKA_GROUP_ID", "control-tower-shipment-status-v1")),
		ClientID:      strings.TrimSpace(getEnv("CONTROL_TOWER_KAFKA_CLIENT_ID", "control-tower-read-model-service")),
		DialTimeout:   dialTimeout,
		TLSEnabled:    parseBool(getEnv("CONTROL_TOWER_KAFKA_TLS_ENABLED", "false")),
		TLSCAFile:     strings.TrimSpace(os.Getenv("CONTROL_TOWER_KAFKA_TLS_CA_FILE")),
		TLSCertFile:   strings.TrimSpace(os.Getenv("CONTROL_TOWER_KAFKA_TLS_CERT_FILE")),
		TLSKeyFile:    strings.TrimSpace(os.Getenv("CONTROL_TOWER_KAFKA_TLS_KEY_FILE")),
		TLSServerName: strings.TrimSpace(os.Getenv("CONTROL_TOWER_KAFKA_TLS_SERVER_NAME")),
		SASLMechanism: strings.ToLower(strings.TrimSpace(os.Getenv("CONTROL_TOWER_KAFKA_SASL_MECHANISM"))),
		SASLUsername:  strings.TrimSpace(os.Getenv("CONTROL_TOWER_KAFKA_SASL_USERNAME")),
		SASLPassword:  os.Getenv("CONTROL_TOWER_KAFKA_SASL_PASSWORD"),
	}, nil
}

func (c Config) Validate() error {
	if c.HTTPPort <= 0 {
		return fmt.Errorf("CONTROL_TOWER_HTTP_PORT must be > 0")
	}
	if c.Consumer.PollTimeout <= 0 || c.Consumer.ProcessTimeout <= 0 || c.Consumer.CommitTimeout <= 0 {
		return fmt.Errorf("consumer timeouts must be > 0")
	}
	if c.Consumer.Enabled {
		if err := c.Kafka.ValidateRequired(); err != nil {
			return err
		}
	}
	return nil
}

func (c KafkaConfig) ValidateRequired() error {
	if len(c.Brokers) == 0 {
		return fmt.Errorf("CONTROL_TOWER_KAFKA_BROKERS must not be empty when consumer enabled")
	}
	if strings.TrimSpace(c.Topic) == "" {
		return fmt.Errorf("CONTROL_TOWER_KAFKA_TOPIC must not be empty")
	}
	if strings.TrimSpace(c.GroupID) == "" {
		return fmt.Errorf("CONTROL_TOWER_KAFKA_GROUP_ID must not be empty")
	}
	if strings.TrimSpace(c.ClientID) == "" {
		return fmt.Errorf("CONTROL_TOWER_KAFKA_CLIENT_ID must not be empty")
	}
	if c.DialTimeout <= 0 {
		return fmt.Errorf("CONTROL_TOWER_KAFKA_DIAL_TIMEOUT must be > 0")
	}
	if err := c.validateTLS(); err != nil {
		return err
	}
	if err := c.validateSASL(); err != nil {
		return err
	}
	return nil
}

func (c KafkaConfig) validateTLS() error {
	if !c.TLSEnabled {
		return nil
	}
	if c.TLSCertFile != "" && c.TLSKeyFile == "" {
		return fmt.Errorf("CONTROL_TOWER_KAFKA_TLS_CERT_FILE requires CONTROL_TOWER_KAFKA_TLS_KEY_FILE")
	}
	if c.TLSKeyFile != "" && c.TLSCertFile == "" {
		return fmt.Errorf("CONTROL_TOWER_KAFKA_TLS_KEY_FILE requires CONTROL_TOWER_KAFKA_TLS_CERT_FILE")
	}
	return nil
}

func (c KafkaConfig) validateSASL() error {
	mech := strings.ToLower(strings.TrimSpace(c.SASLMechanism))
	if mech == "" {
		return nil
	}
	switch mech {
	case "plain", "scram-sha-256", "scram-sha-512":
	default:
		return fmt.Errorf("unsupported CONTROL_TOWER_KAFKA_SASL_MECHANISM %q", c.SASLMechanism)
	}
	if strings.TrimSpace(c.SASLUsername) == "" || strings.TrimSpace(c.SASLPassword) == "" {
		return fmt.Errorf("CONTROL_TOWER_KAFKA_SASL_USERNAME and CONTROL_TOWER_KAFKA_SASL_PASSWORD are required when SASL is set")
	}
	return nil
}

func (c KafkaConfig) ErrorString(err error) string {
	if err == nil {
		return ""
	}
	return strings.ReplaceAll(err.Error(), c.SASLPassword, "***")
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

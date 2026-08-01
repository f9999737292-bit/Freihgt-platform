package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type KafkaConfig struct {
	Brokers       []string
	Topic         string
	ClientID      string
	DialTimeout   time.Duration
	WriteTimeout  time.Duration
	TLSEnabled    bool
	TLSCAFile     string
	TLSCertFile   string
	TLSKeyFile    string
	TLSServerName string
	SASLMechanism string
	SASLUsername  string
	SASLPassword  string
}

func loadKafkaConfig() (KafkaConfig, error) {
	brokersRaw := strings.TrimSpace(os.Getenv("SHIPMENT_KAFKA_BROKERS"))
	var brokers []string
	for _, part := range strings.Split(brokersRaw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			brokers = append(brokers, part)
		}
	}

	dialTimeout, err := parseDuration(getEnv("SHIPMENT_KAFKA_DIAL_TIMEOUT", "10s"))
	if err != nil {
		return KafkaConfig{}, fmt.Errorf("invalid SHIPMENT_KAFKA_DIAL_TIMEOUT: %w", err)
	}
	writeTimeout, err := parseDuration(getEnv("SHIPMENT_KAFKA_WRITE_TIMEOUT", "10s"))
	if err != nil {
		return KafkaConfig{}, fmt.Errorf("invalid SHIPMENT_KAFKA_WRITE_TIMEOUT: %w", err)
	}

	cfg := KafkaConfig{
		Brokers:       brokers,
		Topic:         strings.TrimSpace(getEnv("SHIPMENT_KAFKA_TOPIC", "shipment.status.v1")),
		ClientID:      strings.TrimSpace(getEnv("SHIPMENT_KAFKA_CLIENT_ID", "shipment-service")),
		DialTimeout:   dialTimeout,
		WriteTimeout:  writeTimeout,
		TLSEnabled:    parseBool(getEnv("SHIPMENT_KAFKA_TLS_ENABLED", "false")),
		TLSCAFile:     strings.TrimSpace(os.Getenv("SHIPMENT_KAFKA_TLS_CA_FILE")),
		TLSCertFile:   strings.TrimSpace(os.Getenv("SHIPMENT_KAFKA_TLS_CERT_FILE")),
		TLSKeyFile:    strings.TrimSpace(os.Getenv("SHIPMENT_KAFKA_TLS_KEY_FILE")),
		TLSServerName: strings.TrimSpace(os.Getenv("SHIPMENT_KAFKA_TLS_SERVER_NAME")),
		SASLMechanism: strings.ToLower(strings.TrimSpace(os.Getenv("SHIPMENT_KAFKA_SASL_MECHANISM"))),
		SASLUsername:  strings.TrimSpace(os.Getenv("SHIPMENT_KAFKA_SASL_USERNAME")),
		SASLPassword:  os.Getenv("SHIPMENT_KAFKA_SASL_PASSWORD"),
	}
	return cfg, nil
}

func (c KafkaConfig) ValidateRequired() error {
	if len(c.Brokers) == 0 {
		return fmt.Errorf("SHIPMENT_KAFKA_BROKERS must not be empty when SHIPMENT_OUTBOX_TRANSPORT=kafka")
	}
	if strings.TrimSpace(c.Topic) == "" {
		return fmt.Errorf("SHIPMENT_KAFKA_TOPIC must not be empty when SHIPMENT_OUTBOX_TRANSPORT=kafka")
	}
	if strings.TrimSpace(c.ClientID) == "" {
		return fmt.Errorf("SHIPMENT_KAFKA_CLIENT_ID must not be empty when SHIPMENT_OUTBOX_TRANSPORT=kafka")
	}
	if c.DialTimeout <= 0 {
		return fmt.Errorf("SHIPMENT_KAFKA_DIAL_TIMEOUT must be > 0")
	}
	if c.WriteTimeout <= 0 {
		return fmt.Errorf("SHIPMENT_KAFKA_WRITE_TIMEOUT must be > 0")
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
		return fmt.Errorf("SHIPMENT_KAFKA_TLS_CERT_FILE requires SHIPMENT_KAFKA_TLS_KEY_FILE")
	}
	if c.TLSKeyFile != "" && c.TLSCertFile == "" {
		return fmt.Errorf("SHIPMENT_KAFKA_TLS_KEY_FILE requires SHIPMENT_KAFKA_TLS_CERT_FILE")
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
		return fmt.Errorf("unsupported SHIPMENT_KAFKA_SASL_MECHANISM %q", c.SASLMechanism)
	}
	if strings.TrimSpace(c.SASLUsername) == "" || strings.TrimSpace(c.SASLPassword) == "" {
		return fmt.Errorf("SHIPMENT_KAFKA_SASL_USERNAME and SHIPMENT_KAFKA_SASL_PASSWORD are required when SHIPMENT_KAFKA_SASL_MECHANISM is set")
	}
	return nil
}

func (c KafkaConfig) ErrorString(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	msg = strings.ReplaceAll(msg, c.SASLPassword, "***")
	return msg
}

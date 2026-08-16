package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestKafkaConfigValidationTLSAndSASL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     KafkaConfig
		wantErr string
	}{
		{
			name: "tls disabled no cert required",
			cfg: KafkaConfig{
				Brokers:     []string{"localhost:19092"},
				Topic:       "shipment.status.v1",
				DriverTopic: "driver.events.v1",
				ClientID:     "shipment-service",
				DialTimeout:  time.Second,
				WriteTimeout: time.Second,
			},
		},
		{
			name: "tls ca only allowed",
			cfg: KafkaConfig{
				Brokers:     []string{"localhost:19092"},
				Topic:       "shipment.status.v1",
				DriverTopic: "driver.events.v1",
				ClientID:     "shipment-service",
				DialTimeout:  time.Second,
				WriteTimeout: time.Second,
				TLSEnabled:   true,
				TLSCAFile:    "/etc/ca.pem",
			},
		},
		{
			name: "cert without key",
			cfg: KafkaConfig{
				Brokers:     []string{"localhost:19092"},
				Topic:       "shipment.status.v1",
				DriverTopic: "driver.events.v1",
				ClientID:     "shipment-service",
				DialTimeout:  time.Second,
				WriteTimeout: time.Second,
				TLSEnabled:   true,
				TLSCertFile:  "/etc/cert.pem",
			},
			wantErr: "TLS_KEY_FILE",
		},
		{
			name: "key without cert",
			cfg: KafkaConfig{
				Brokers:     []string{"localhost:19092"},
				Topic:       "shipment.status.v1",
				DriverTopic: "driver.events.v1",
				ClientID:     "shipment-service",
				DialTimeout:  time.Second,
				WriteTimeout: time.Second,
				TLSEnabled:   true,
				TLSKeyFile:   "/etc/key.pem",
			},
			wantErr: "TLS_CERT_FILE",
		},
		{
			name: "unknown sasl mechanism",
			cfg: KafkaConfig{
				Brokers:       []string{"localhost:19092"},
				Topic:         "shipment.status.v1",
				DriverTopic:   "driver.events.v1",
				ClientID:      "shipment-service",
				DialTimeout:   time.Second,
				WriteTimeout:  time.Second,
				SASLMechanism: "oauthbearer",
			},
			wantErr: "unsupported SHIPMENT_KAFKA_SASL_MECHANISM",
		},
		{
			name: "sasl without credentials",
			cfg: KafkaConfig{
				Brokers:       []string{"localhost:19092"},
				Topic:         "shipment.status.v1",
				DriverTopic:   "driver.events.v1",
				ClientID:      "shipment-service",
				DialTimeout:   time.Second,
				WriteTimeout:  time.Second,
				SASLMechanism: "plain",
			},
			wantErr: "SASL_USERNAME",
		},
		{
			name: "empty brokers",
			cfg: KafkaConfig{
				Topic:        "shipment.status.v1",
				ClientID:     "shipment-service",
				DialTimeout:  time.Second,
				WriteTimeout: time.Second,
			},
			wantErr: "SHIPMENT_KAFKA_BROKERS",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.cfg.ValidateRequired()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err=%v want contains %q", err, tc.wantErr)
			}
		})
	}
}

func TestKafkaConfigPasswordNotInErrorString(t *testing.T) {
	t.Parallel()
	cfg := KafkaConfig{
		SASLMechanism: "plain",
		SASLUsername:  "",
		SASLPassword:  "super-secret-password",
	}
	err := cfg.validateSASL()
	if err == nil {
		t.Fatal("expected sasl validation error")
	}
	safe := cfg.ErrorString(err)
	if strings.Contains(safe, "super-secret-password") {
		t.Fatalf("password leaked into error: %s", safe)
	}
}

func TestLoadOutboxKafkaValidationWhenEnabled(t *testing.T) {
	t.Setenv("SHIPMENT_OUTBOX_ENABLED", "true")
	t.Setenv("SHIPMENT_OUTBOX_TRANSPORT", "kafka")
	t.Setenv("SHIPMENT_KAFKA_BROKERS", "")
	t.Setenv("SHIPMENT_KAFKA_TOPIC", "shipment.status.v1")
	t.Setenv("SHIPMENT_KAFKA_DRIVER_TOPIC", "driver.events.v1")
	t.Setenv("SHIPMENT_KAFKA_CLIENT_ID", "shipment-service")

	_, err := loadOutboxConfig()
	if err == nil || !strings.Contains(err.Error(), "SHIPMENT_KAFKA_BROKERS") {
		t.Fatalf("err=%v", err)
	}
}

func TestLoadOutboxDisabledDoesNotRequireKafka(t *testing.T) {
	t.Setenv("SHIPMENT_OUTBOX_ENABLED", "false")
	os.Unsetenv("SHIPMENT_KAFKA_BROKERS")

	cfg, err := loadOutboxConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Enabled {
		t.Fatal("outbox should be disabled")
	}
}

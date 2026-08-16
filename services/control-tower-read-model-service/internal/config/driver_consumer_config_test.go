package config

import (
	"testing"
)

func TestDriverConsumerFeatureFlagOffDoesNotRequireBrokers(t *testing.T) {
	t.Setenv("CONTROL_TOWER_DRIVER_EVENTS_ENABLED", "false")
	t.Setenv("CONTROL_TOWER_DRIVER_KAFKA_BROKERS", "")

	cfg, err := loadDriverConsumerConfig()
	if err != nil {
		t.Fatalf("loadDriverConsumerConfig: %v", err)
	}
	if cfg.Enabled {
		t.Fatal("expected driver consumer disabled")
	}
}

func TestDriverConsumerFeatureFlagOnRequiresBrokersOnValidate(t *testing.T) {
	t.Setenv("CONTROL_TOWER_DRIVER_EVENTS_ENABLED", "true")
	t.Setenv("CONTROL_TOWER_DRIVER_KAFKA_BROKERS", "")
	t.Setenv("CONTROL_TOWER_KAFKA_BROKERS", "")

	cfg, err := loadDriverConsumerConfig()
	if err != nil {
		t.Fatalf("loadDriverConsumerConfig: %v", err)
	}
	if !cfg.Enabled {
		t.Fatal("expected driver consumer enabled")
	}
	if err := cfg.Kafka.ValidateRequired(); err == nil {
		t.Fatal("expected validation error when enabled without driver kafka brokers")
	}
}

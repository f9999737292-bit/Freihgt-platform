package config

import (
	"os"
	"testing"
	"time"
)

func TestDeadlineWorkerDefaultsDisabled(t *testing.T) {
	t.Setenv("RFX_DEADLINE_WORKER_ENABLED", "")
	t.Setenv("RFX_DEADLINE_WORKER_INTERVAL_SECONDS", "")
	t.Setenv("RFX_DEADLINE_WORKER_BATCH_SIZE", "")

	cfg, err := loadDeadlineWorkerConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Enabled {
		t.Fatal("expected worker disabled by default")
	}
	if cfg.Interval != 60*time.Second {
		t.Fatalf("interval=%s", cfg.Interval)
	}
	if cfg.BatchSize != 50 {
		t.Fatalf("batch size=%d", cfg.BatchSize)
	}
}

func TestDeadlineWorkerEnabledFromEnv(t *testing.T) {
	t.Setenv("RFX_DEADLINE_WORKER_ENABLED", "true")
	t.Setenv("RFX_DEADLINE_WORKER_INTERVAL_SECONDS", "15")
	t.Setenv("RFX_DEADLINE_WORKER_BATCH_SIZE", "10")

	cfg, err := loadDeadlineWorkerConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.Enabled {
		t.Fatal("expected worker enabled")
	}
	if cfg.Interval != 15*time.Second || cfg.BatchSize != 10 {
		t.Fatalf("cfg=%+v", cfg)
	}
}

func TestDeadlineWorkerRejectsInvalidInterval(t *testing.T) {
	t.Setenv("RFX_DEADLINE_WORKER_INTERVAL_SECONDS", "0")
	if _, err := loadDeadlineWorkerConfig(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLoadIncludesDeadlineWorkerConfig(t *testing.T) {
	t.Setenv("RFX_DEADLINE_WORKER_ENABLED", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.DeadlineWorker.BatchSize == 0 {
		t.Fatal("expected deadline worker config")
	}
	_ = os.Getenv("DATABASE_URL")
}

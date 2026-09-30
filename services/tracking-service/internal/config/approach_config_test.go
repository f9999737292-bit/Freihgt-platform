package config

import (
	"testing"
)

func TestStopApproachConfigFailClosed(t *testing.T) {
	t.Setenv("TRACKING_STOP_APPROACH_ENABLED", "false")
	t.Setenv("TRACKING_STOP_APPROACH_RADIUS_METERS", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StopApproach.Enabled {
		t.Fatal("approach enabled by default")
	}

	t.Setenv("TRACKING_STOP_APPROACH_ENABLED", "true")
	t.Setenv("TRACKING_STOP_APPROACH_RADIUS_METERS", "")
	if _, err := Load(); err == nil {
		t.Fatal("missing radius was accepted")
	}
	t.Setenv("TRACKING_STOP_APPROACH_RADIUS_METERS", "0")
	if _, err := Load(); err == nil {
		t.Fatal("zero radius was accepted")
	}
	t.Setenv("TRACKING_STOP_APPROACH_RADIUS_METERS", "-5")
	if _, err := Load(); err == nil {
		t.Fatal("negative radius was accepted")
	}
	t.Setenv("TRACKING_STOP_APPROACH_RADIUS_METERS", "250.5")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.StopApproach.Enabled || cfg.StopApproach.RadiusMeters != 250.5 {
		t.Fatalf("config %+v", cfg.StopApproach)
	}
}

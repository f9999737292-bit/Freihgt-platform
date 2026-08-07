package controltowerreadmodel

import (
	"os"
	"testing"
	"time"
)

func TestParseModeValidation(t *testing.T) {
	tests := []struct {
		raw   string
		want  Mode
		isErr bool
	}{
		{"", ModeDisabled, false},
		{"disabled", ModeDisabled, false},
		{"shadow", ModeShadow, false},
		{"primary", ModePrimary, false},
		{"invalid", "", true},
	}
	for _, tc := range tests {
		mode, err := ParseMode(tc.raw)
		if tc.isErr {
			if err == nil {
				t.Fatalf("ParseMode(%q) expected error", tc.raw)
			}
			continue
		}
		if err != nil || mode != tc.want {
			t.Fatalf("ParseMode(%q) = (%q, %v) want %q", tc.raw, mode, err, tc.want)
		}
	}
}

func TestConfigValidateDisabledDoesNotRequireBaseURL(t *testing.T) {
	cfg := Config{Mode: ModeDisabled, Timeout: time.Second}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("disabled config should validate: %v", err)
	}
}

func TestConfigValidateShadowRequiresValidURL(t *testing.T) {
	cfg := Config{Mode: ModeShadow, Timeout: time.Second}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing base URL")
	}

	cfg.BaseURL = "http://control-tower-read-model-service:8089"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid url should pass: %v", err)
	}
}

func TestConfigValidateRejectsCredentialsInURL(t *testing.T) {
	cfg := Config{
		Mode:    ModePrimary,
		BaseURL: "http://user:pass@control-tower-read-model-service:8089",
		Timeout: time.Second,
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for credentials in URL")
	}
}

func TestLoadConfigFromEnvDefaults(t *testing.T) {
	t.Setenv(envMode, "")
	t.Setenv(envBaseURL, "")
	t.Setenv(envTimeout, "")
	t.Setenv(envRequireConsumerRunning, "")

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Mode != ModeDisabled {
		t.Fatalf("mode=%q want disabled", cfg.Mode)
	}
	if cfg.Timeout != 800*time.Millisecond {
		t.Fatalf("timeout=%v want 800ms", cfg.Timeout)
	}
	if !cfg.RequireConsumerRunning {
		t.Fatal("expected require consumer running default true")
	}
}

func TestLoadConfigFromEnvPrimary(t *testing.T) {
	t.Setenv(envMode, "primary")
	t.Setenv(envBaseURL, "http://control-tower-read-model-service:8089")
	t.Setenv(envTimeout, "500ms")
	t.Setenv(envRequireConsumerRunning, "false")

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Mode != ModePrimary {
		t.Fatalf("mode=%q", cfg.Mode)
	}
	if cfg.Timeout != 500*time.Millisecond {
		t.Fatalf("timeout=%v", cfg.Timeout)
	}
	if cfg.RequireConsumerRunning {
		t.Fatal("expected require consumer false")
	}

	_ = os.Unsetenv(envMode)
}

package config

import (
	"strings"
	"testing"
)

func TestLoadRejectsAnalyticsSourceCaller(t *testing.T) {
	const shared = "shared-platform-secret"
	const caller = "analytics-source-secret"
	t.Setenv("ENVIRONMENT", "test")
	t.Setenv("INTERNAL_SERVICE_TOKEN", shared)

	t.Run("empty", func(t *testing.T) {
		t.Setenv("ANALYTICS_SOURCE_CALLER_TOKEN", "")
		_, err := Load()
		if err == nil || strings.Contains(err.Error(), shared) {
			t.Fatal("empty analytics source caller was accepted or the error disclosed a credential")
		}
	})
	t.Run("equal", func(t *testing.T) {
		t.Setenv("ANALYTICS_SOURCE_CALLER_TOKEN", shared)
		_, err := Load()
		if err == nil || strings.Contains(err.Error(), shared) {
			t.Fatal("equal analytics source caller was accepted or the error disclosed a credential")
		}
	})
	t.Run("distinct", func(t *testing.T) {
		t.Setenv("ANALYTICS_SOURCE_CALLER_TOKEN", caller)
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.AnalyticsSourceCallerToken != caller || cfg.InternalServiceToken != shared {
			t.Fatal("caller configuration was not loaded")
		}
	})
}

package config

import (
	"strings"
	"testing"
)

func TestValidateAnalyticsGatewayCaller(t *testing.T) {
	const caller = "gateway-caller-secret"
	const shared = "shared-platform-secret"
	if err := validateAnalyticsGatewayCaller(caller, shared); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, caller, shared string
	}{
		{name: "empty", shared: shared},
		{name: "equal", caller: shared, shared: shared},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAnalyticsGatewayCaller(tt.caller, tt.shared)
			if err == nil {
				t.Fatal("expected rejection")
			}
			if strings.Contains(err.Error(), caller) || strings.Contains(err.Error(), shared) {
				t.Fatal("error disclosed a credential")
			}
		})
	}
}

func TestLoadRejectsMissingAnalyticsGatewayCaller(t *testing.T) {
	t.Setenv("ENVIRONMENT", "test")
	t.Setenv("INTEGRATION_AUTH_ENABLED", "false")
	t.Setenv("INTERNAL_SERVICE_TOKEN", "shared-platform-secret")
	t.Setenv("ANALYTICS_GATEWAY_CALLER_TOKEN", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected rejection")
	}
	if strings.Contains(err.Error(), "shared-platform-secret") {
		t.Fatal("error disclosed a credential")
	}
}

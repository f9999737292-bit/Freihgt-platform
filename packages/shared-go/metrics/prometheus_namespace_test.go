package metrics_test

import (
	"testing"

	"github.com/freight-platform/shared-go/metrics"
)

func TestPrometheusNamespaceReplacesHyphens(t *testing.T) {
	got := metrics.PrometheusNamespace("billing-register-service")
	if got != "billing_register_service" {
		t.Fatalf("PrometheusNamespace() = %q, want billing_register_service", got)
	}
}

func TestPrometheusNamespaceEmptyFallback(t *testing.T) {
	if metrics.PrometheusNamespace("---") != "service" {
		t.Fatal("expected fallback namespace for empty normalization")
	}
}

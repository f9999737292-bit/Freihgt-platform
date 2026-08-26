package observability

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewMetricsRegistersValidDescriptors(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newMetrics(reg, "contract-rate-service")
	if m == nil {
		t.Fatal("expected metrics instance")
	}

	start := time.Now()
	m.ObserveResolution(start, "MATCHED", "CONTRACT", "")
	m.ObserveResolution(start, "AMBIGUOUS", "NONE", "")
	m.ObserveResolution(start, "FAILED", "NONE", "not_found")
	m.IncVersionActivation("ACTIVATED")

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	names := map[string]struct{}{}
	for _, family := range families {
		names[family.GetName()] = struct{}{}
	}

	for _, want := range []string{
		"contract_rate_service_rate_resolution_total",
		"contract_rate_service_rate_resolution_failed_total",
		"contract_rate_service_pricing_source_total",
		"contract_rate_service_pricing_source_failure_total",
		"contract_rate_service_rate_resolution_ambiguous_total",
		"contract_rate_service_rate_resolution_duration_seconds",
		"contract_rate_service_rate_version_activation_total",
	} {
		if _, ok := names[want]; !ok {
			t.Fatalf("missing metric family %q, got %v", want, names)
		}
	}
}

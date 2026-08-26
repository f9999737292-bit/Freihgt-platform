package observability

import (
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/client_golang/prometheus"
)

func TestNewSettlementMetricsRegistersValidDescriptors(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newSettlementMetrics(reg, "billing-register-service")
	if m == nil {
		t.Fatal("expected metrics instance")
	}

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	if len(families) != 1 {
		t.Fatalf("expected 1 metric family, got %d", len(families))
	}
	if families[0].GetName() != "billing_register_service_legacy_settlement_pricing_fallback_total" {
		t.Fatalf("unexpected metric name: %s", families[0].GetName())
	}
	if families[0].GetType() != dto.MetricType_COUNTER {
		t.Fatalf("expected counter, got %v", families[0].GetType())
	}
}

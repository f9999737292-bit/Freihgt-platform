package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewPricingMetricsRegistersValidDescriptors(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newPricingMetrics(reg, "transport-order-service")
	if m == nil {
		t.Fatal("expected metrics instance")
	}
	m.IncSnapshotPersist("OK")
	m.IncSnapshotPersistFailure("db")
	m.IncTOPricingResolution("MATCHED")

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	if len(families) != 3 {
		t.Fatalf("expected 3 metric families, got %d", len(families))
	}

	names := map[string]struct{}{}
	for _, family := range families {
		names[family.GetName()] = struct{}{}
	}

	for _, want := range []string{
		"transport_order_service_snapshot_persist_total",
		"transport_order_service_snapshot_persist_failure_total",
		"transport_order_service_to_pricing_resolution_total",
	} {
		if _, ok := names[want]; !ok {
			t.Fatalf("missing metric family %q, got %v", want, names)
		}
	}
}

package controltower

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
)

func TestShadowMetricsIncrementAfterSummaryRequest(t *testing.T) {
	tenantA := "11111111-1111-1111-1111-111111111111"
	shipmentServer := shipmentCombinedServer(shipmentCombinedConfig{
		items: []map[string]any{
			{"id": "s1", "shipment_number": "SH-1", "status": "IN_TRANSIT"},
		},
		total: 1,
		aggregateBody: `{
			"totalShipments": 1,
			"countedShipments": 1,
			"byStatus": {"IN_TRANSIT": 1},
			"complete": true
		}`,
	})
	defer shipmentServer.Close()

	handler, _ := newReadModelSummaryHandler(t, readModelHandlerConfig{
		testHandlerConfig: testHandlerConfig{
			shipmentURL: shipmentServer.URL,
			identityURL: identityAdminServer().URL,
			authEnabled: true,
		},
		readModelMode: controltowerreadmodel.ModeShadow,
		readModelFn: func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"totalShipments":1,"byStatus":{"IN_TRANSIT":1},"incompleteProjections":0,"freshness":{"consumerRunning":true}}`))
		},
	})

	beforeRM := sumMetric(t, "control_tower_read_model_requests_total")
	beforeCmp := sumMetric(t, "control_tower_read_model_shadow_comparison_total")
	beforeLegacy := sumMetric(t, "control_tower_legacy_status_aggregate_requests_total")

	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}

	if got := sumMetric(t, "control_tower_read_model_requests_total"); got <= beforeRM {
		t.Fatalf("read-model requests metric did not increase: before=%v after=%v", beforeRM, got)
	}
	if got := sumMetric(t, "control_tower_read_model_shadow_comparison_total"); got <= beforeCmp {
		t.Fatalf("shadow comparison metric did not increase: before=%v after=%v", beforeCmp, got)
	}
	if got := sumMetric(t, "control_tower_legacy_status_aggregate_requests_total"); got <= beforeLegacy {
		t.Fatalf("legacy aggregate metric did not increase: before=%v after=%v", beforeLegacy, got)
	}
}

func TestDisabledModeDoesNotIncrementReadModelRequests(t *testing.T) {
	shipmentServer := shipmentCombinedServer(shipmentCombinedConfig{
		items:         []map[string]any{{"id": "s1", "status": "IN_TRANSIT"}},
		total:         1,
		aggregateBody: `{"totalShipments":1,"countedShipments":1,"byStatus":{"IN_TRANSIT":1},"complete":true}`,
	})
	defer shipmentServer.Close()

	handler, _ := newReadModelSummaryHandler(t, readModelHandlerConfig{
		testHandlerConfig: testHandlerConfig{
			shipmentURL: shipmentServer.URL,
			identityURL: identityAdminServer().URL,
			authEnabled: true,
		},
		readModelMode: controltowerreadmodel.ModeDisabled,
	})

	before := sumMetric(t, "control_tower_read_model_requests_total")
	token := signTestToken(t, "secret", "user-a", "11111111-1111-1111-1111-111111111111", "user@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := sumMetric(t, "control_tower_read_model_requests_total"); got != before {
		t.Fatalf("disabled mode must not increment read-model requests: before=%v after=%v", before, got)
	}
}

func sumMetric(t *testing.T, name string) float64 {
	t.Helper()
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	var total float64
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, metric := range mf.GetMetric() {
			if metric.GetCounter() != nil {
				total += metric.GetCounter().GetValue()
			}
		}
	}
	return total
}

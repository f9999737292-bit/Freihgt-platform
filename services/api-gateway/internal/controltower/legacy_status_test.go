package controltower

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/freight-platform/api-gateway/internal/config"
	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
)

func newLegacyStatusTestService(t *testing.T, shipmentURL string, mode controltowerreadmodel.Mode) *Service {
	t.Helper()
	client := NewDownstreamClient(
		&http.Client{Timeout: 5 * time.Second},
		"http://127.0.0.1:1",
		"http://127.0.0.1:1",
		"http://127.0.0.1:1",
		shipmentURL,
		"http://127.0.0.1:1",
		200,
	)
	return NewService(config.Config{
		Services: config.ServiceURLs{Shipment: shipmentURL},
		ControlTower: config.ControlTowerConfig{
			LegacyStatusTimeout: 800 * time.Millisecond,
			ReadModel: controltowerreadmodel.Config{
				Mode: mode,
			},
		},
	}, client, slog.New(slog.DiscardHandler))
}

func testLegacyRequestContext() RequestContext {
	return RequestContext{
		TenantID:  "11111111-1111-1111-1111-111111111111",
		RequestID: "req-legacy-status",
		UserID:    "user-a",
	}
}

func sampleShipments() []rawShipment {
	return []rawShipment{
		{ID: "s1", ShipmentNumber: "SH-1", Status: "IN_TRANSIT"},
		{ID: "s2", ShipmentNumber: "SH-2", Status: "IN_TRANSIT"},
	}
}

func TestResolveLegacyStatusInputUsesFullAggregate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/shipments/status-summary" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{
			"totalShipments": 100,
			"countedShipments": 100,
			"byStatus": {"IN_TRANSIT": 60, "DELIVERED": 40},
			"complete": true
		}`))
	}))
	defer server.Close()

	svc := newLegacyStatusTestService(t, server.URL, controltowerreadmodel.ModeDisabled)
	input := svc.resolveLegacyStatusInput(context.Background(), testLegacyRequestContext(), sampleShipments(), 100)

	if !input.FullAggregateAvailable {
		t.Fatal("expected full aggregate available")
	}
	if input.LimitedDataset {
		t.Fatal("full aggregate must not be limited")
	}
	if input.TotalShipments != 100 || input.ByStatus["DELIVERED"] != 40 {
		t.Fatalf("expected aggregate counts, got %+v", input)
	}
}

func TestResolveLegacyStatusInputFallsBackToPageLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/shipments/status-summary" {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	svc := newLegacyStatusTestService(t, server.URL, controltowerreadmodel.ModePrimary)
	input := svc.resolveLegacyStatusInput(context.Background(), testLegacyRequestContext(), sampleShipments(), 100)

	if input.FullAggregateAvailable {
		t.Fatal("expected full aggregate unavailable after dependency error")
	}
	if !input.LimitedDataset {
		t.Fatal("expected page-limited fallback")
	}
	if input.TotalShipments != 100 || input.CountedShipments != 2 {
		t.Fatalf("expected page counts, got total=%d counted=%d", input.TotalShipments, input.CountedShipments)
	}
	if input.ByStatus["IN_TRANSIT"] != 2 {
		t.Fatalf("byStatus=%v", input.ByStatus)
	}
}

func TestResolveLegacyStatusInputWithoutClientUsesPage(t *testing.T) {
	svc := newLegacyStatusTestService(t, "http://127.0.0.1:1", controltowerreadmodel.ModeDisabled)
	svc.legacyAggregate = nil

	input := svc.resolveLegacyStatusInput(context.Background(), testLegacyRequestContext(), sampleShipments(), 100)

	if input.FullAggregateAvailable {
		t.Fatal("nil aggregate client must not mark full aggregate available")
	}
	if !input.LimitedDataset {
		t.Fatal("expected page-limited input without aggregate client")
	}
	if input.TotalShipments != 100 || input.CountedShipments != 2 {
		t.Fatalf("expected page counts, got total=%d counted=%d", input.TotalShipments, input.CountedShipments)
	}
}

func TestResolveLegacyStatusInputFullPageWhenAggregateMatches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"totalShipments": 2,
			"countedShipments": 2,
			"byStatus": {"IN_TRANSIT": 2},
			"complete": true
		}`))
	}))
	defer server.Close()

	svc := newLegacyStatusTestService(t, server.URL, controltowerreadmodel.ModeShadow)
	input := svc.resolveLegacyStatusInput(context.Background(), testLegacyRequestContext(), sampleShipments(), 2)

	if !input.FullAggregateAvailable || input.LimitedDataset {
		t.Fatalf("expected non-limited full aggregate, got %+v", input)
	}
	if input.TotalShipments != 2 || input.ByStatus["IN_TRANSIT"] != 2 {
		t.Fatalf("unexpected counts: %+v", input)
	}
}

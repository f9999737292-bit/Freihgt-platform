package shipmentevents

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testDownstreamWithStatusHistory(t *testing.T, historyPayload map[string]any) *DownstreamClient {
	t.Helper()
	tenantID := "11111111-1111-1111-1111-111111111111"
	shipmentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	shipmentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Tenant-ID"); got != tenantID {
			t.Fatalf("X-Tenant-ID=%q want %q", got, tenantID)
		}
		if got := r.URL.Query().Get("tenant_id"); got != "" {
			t.Fatalf("tenant_id query must be absent, got %q", got)
		}
		if strings.Contains(r.URL.Path, "/internal/v1/shipments/") {
			if !strings.HasSuffix(r.URL.Path, "/status-history") {
				t.Fatalf("unexpected internal path %q", r.URL.Path)
			}
			_ = json.NewEncoder(w).Encode(historyPayload)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": shipmentID, "tenant_id": tenantID, "shipment_number": "SHP-1", "status": "IN_TRANSIT",
			"created_at": "2026-07-31T10:00:00Z", "updated_at": "2026-07-31T11:00:00Z",
		})
	}))
	t.Cleanup(shipmentServer.Close)

	documentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
	}))
	t.Cleanup(documentServer.Close)

	return NewDownstreamClient(&http.Client{Timeout: 5 * time.Second}, "", shipmentServer.URL, documentServer.URL, "", 200)
}

func TestCanonicalStatusHistoryEventsAreNotDerived(t *testing.T) {
	historyID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	svc := NewService(testServiceConfig(), testDownstreamWithStatusHistory(t, map[string]any{
		"complete": true,
		"items": []any{
			map[string]any{
				"id": historyID, "shipmentId": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				"shipmentVersion": 1, "toStatus": "CARRIER_ASSIGNED",
				"source": "SHIPMENT_SERVICE", "actor": map[string]any{"type": "SYSTEM"},
				"occurredAt": "2026-07-31T10:00:00Z", "recordedAt": "2026-07-31T10:00:00Z",
			},
		},
		"warnings": []any{},
	}))
	resp, err := svc.GetEvents(context.Background(), RequestContext{TenantID: "11111111-1111-1111-1111-111111111111"}, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", ListQuery{Page: 1, Limit: 50, Order: "desc"})
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	if !resp.DataFreshness.ShipmentEventsLoaded {
		t.Fatal("shipmentEventsLoaded must be true when history is available")
	}
	found := false
	for _, event := range resp.Timeline.Items {
		if event.Source == SourceShipmentStatusHistory && event.Type == EventTypeShipmentCreated {
			found = true
			if event.Derived {
				t.Fatal("canonical SHIPMENT_CREATED must not be derived")
			}
			if event.SourceEventID == nil || *event.SourceEventID != historyID {
				t.Fatalf("sourceEventId=%v", event.SourceEventID)
			}
			if event.ID != historyID {
				t.Fatalf("event id=%s want history id", event.ID)
			}
		}
	}
	if !found {
		t.Fatal("expected canonical SHIPMENT_CREATED event")
	}
}

func TestCanonicalGenericTransitionMapsToStatusChanged(t *testing.T) {
	historyID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	svc := NewService(testServiceConfig(), testDownstreamWithStatusHistory(t, map[string]any{
		"complete": true,
		"items": []any{
			map[string]any{
				"id": historyID, "shipmentId": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				"fromStatus": "CARRIER_ASSIGNED", "toStatus": "IN_TRANSIT", "shipmentVersion": 4,
				"source": "SHIPMENT_SERVICE", "actor": map[string]any{"type": "USER", "id": "22222222-2222-2222-2222-222222222222"},
				"occurredAt": "2026-07-31T12:00:00Z", "recordedAt": "2026-07-31T12:00:00Z",
			},
		},
	}))
	resp, err := svc.GetEvents(context.Background(), RequestContext{TenantID: "11111111-1111-1111-1111-111111111111"}, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", ListQuery{Page: 1, Limit: 50})
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	for _, event := range resp.Timeline.Items {
		if event.ID == historyID {
			if event.Type != EventTypeShipmentStatusChanged {
				t.Fatalf("type=%s", event.Type)
			}
			if event.Metadata["fromStatus"] != "CARRIER_ASSIGNED" || event.Metadata["toStatus"] != "IN_TRANSIT" {
				t.Fatalf("metadata=%#v", event.Metadata)
			}
			return
		}
	}
	t.Fatal("expected canonical status changed event")
}

func TestCanonicalCancelledTransition(t *testing.T) {
	historyID := "dddddddd-dddd-dddd-dddd-dddddddddddd"
	svc := NewService(testServiceConfig(), testDownstreamWithStatusHistory(t, map[string]any{
		"complete": true,
		"items": []any{
			map[string]any{
				"id": historyID, "shipmentId": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				"fromStatus": "IN_TRANSIT", "toStatus": "CANCELLED", "shipmentVersion": 5,
				"source": "SHIPMENT_SERVICE", "actor": map[string]any{"type": "USER", "id": "22222222-2222-2222-2222-222222222222"},
				"occurredAt": "2026-07-31T13:00:00Z", "recordedAt": "2026-07-31T13:00:00Z",
			},
		},
	}))
	resp, err := svc.GetEvents(context.Background(), RequestContext{TenantID: "11111111-1111-1111-1111-111111111111"}, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", ListQuery{Page: 1, Limit: 50})
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	for _, event := range resp.Timeline.Items {
		if event.ID == historyID && event.Type != EventTypeShipmentCancelled {
			t.Fatalf("type=%s", event.Type)
		}
	}
}

func TestPartialStatusHistoryAddsWarning(t *testing.T) {
	svc := NewService(testServiceConfig(), testDownstreamWithStatusHistory(t, map[string]any{
		"complete": false,
		"items": []any{
			map[string]any{
				"id": "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", "shipmentId": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				"fromStatus": "CARRIER_ASSIGNED", "toStatus": "IN_TRANSIT", "shipmentVersion": 3,
				"source": "SHIPMENT_SERVICE", "actor": map[string]any{"type": "SYSTEM"},
				"occurredAt": "2026-07-31T12:00:00Z", "recordedAt": "2026-07-31T12:00:00Z",
			},
		},
		"warnings": []any{WarningShipmentStatusHistoryPartial},
	}))
	resp, err := svc.GetEvents(context.Background(), RequestContext{TenantID: "11111111-1111-1111-1111-111111111111"}, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", ListQuery{Page: 1, Limit: 50})
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	if !resp.DataFreshness.Partial {
		t.Fatal("partial must be true for legacy history")
	}
	hasPartial := false
	for _, warning := range resp.DataFreshness.Warnings {
		if warning == WarningShipmentStatusHistoryPartial {
			hasPartial = true
		}
	}
	if !hasPartial {
		t.Fatalf("warnings=%#v", resp.DataFreshness.Warnings)
	}
}

func TestFetchStatusHistoryUsesInternalPathWithoutTenantQuery(t *testing.T) {
	t.Parallel()
	tenantID := "11111111-1111-1111-1111-111111111111"
	shipmentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/internal/v1/shipments/"+shipmentID+"/status-history") {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("tenant_id"); got != "" {
			t.Fatalf("internal request must not include tenant_id query")
		}
		if got := r.Header.Get("X-Tenant-ID"); got != tenantID {
			t.Fatalf("X-Tenant-ID=%q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"complete": true, "items": []any{}, "warnings": []any{}})
	}))
	defer server.Close()

	client := NewDownstreamClient(nil, "", server.URL, "", "", 200)
	result, err := client.FetchStatusHistory(context.Background(), RequestContext{TenantID: tenantID}, shipmentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Failed || result.NotFound {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestMapStatusHistoryEventTypeTable(t *testing.T) {
	t.Parallel()
	if mapStatusHistoryEventType(rawStatusHistory{ToStatus: "CANCELLED", FromStatus: strPtr("IN_TRANSIT")}) != EventTypeShipmentCancelled {
		t.Fatal("CANCELLED mapping failed")
	}
	if mapStatusHistoryEventType(rawStatusHistory{ToStatus: "READY_FOR_BILLING", FromStatus: strPtr("DELIVERED")}) != EventTypeReadyForBilling {
		t.Fatal("READY_FOR_BILLING mapping failed")
	}
	if mapStatusHistoryEventType(rawStatusHistory{ToStatus: "CARRIER_ASSIGNED", FromStatus: nil}) != EventTypeShipmentCreated {
		t.Fatal("initial transition mapping failed")
	}
}

func strPtr(v string) *string { return &v }

func TestCanonicalTransitionRemovesDerivedCreatedDuplicate(t *testing.T) {
	at := time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)
	canonical := []ShipmentTimelineEvent{{
		ID: "hist-1", Type: EventTypeShipmentCreated, Source: SourceShipmentStatusHistory,
		Derived: false, OccurredAt: at, Metadata: map[string]interface{}{"toStatus": "CARRIER_ASSIGNED"},
	}}
	derived := buildDerivedShipmentEventsWithoutStatusDuplicates(rawShipment{
		ID: "ship-1", ShipmentNumber: "SHP-1", Status: "IN_TRANSIT",
		CreatedAt: &at,
	}, canonical)
	for _, event := range derived {
		if event.Type == EventTypeShipmentCreated {
			t.Fatal("derived SHIPMENT_CREATED must be removed when canonical exists")
		}
	}
}

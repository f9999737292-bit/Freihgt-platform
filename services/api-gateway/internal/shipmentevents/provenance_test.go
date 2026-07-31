package shipmentevents

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/freight-platform/api-gateway/internal/config"
)

func testServiceConfig() config.Config {
	return config.Config{
		ProxyTimeoutSeconds: 5,
		ControlTower:        config.ControlTowerConfig{MaxDownstreamFetchLimit: 200},
	}
}

func testDownstreamClient(t *testing.T) *DownstreamClient {
	t.Helper()
	tenantID := "11111111-1111-1111-1111-111111111111"
	shipmentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	shipmentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("tenant_id"); got != tenantID {
			t.Fatalf("shipment request tenant_id=%q want %q", got, tenantID)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": shipmentID, "tenant_id": tenantID, "shipment_number": "SHP-1", "status": "IN_TRANSIT",
			"planned_pickup_at": "2026-08-01T10:00:00Z", "created_at": "2026-07-31T10:00:00Z", "updated_at": "2026-07-31T11:00:00Z",
		})
	}))
	t.Cleanup(shipmentServer.Close)

	documentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
	}))
	t.Cleanup(documentServer.Close)

	return NewDownstreamClient(
		&http.Client{Timeout: 5 * time.Second},
		"", shipmentServer.URL, documentServer.URL, "", 200,
	)
}

func TestDocumentEntityEventIsDerived(t *testing.T) {
	created := time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)
	shipment := rawShipment{ID: "ship-1", ShipmentNumber: "SHP-1"}
	events := buildDocumentEvents(shipment, []rawDocument{{
		ID: "doc-1", DocumentType: "POD", DocumentStatus: "DRAFT", CreatedAt: &created,
	}})
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if !events[0].Derived || events[0].Source != SourceDocumentState {
		t.Fatalf("expected derived document state event, got %#v", events[0])
	}
	if events[0].SourceEventID != nil {
		t.Fatal("derived document event must not set sourceEventId")
	}
}

func TestDocumentSignedWithoutSignedAtNotCreated(t *testing.T) {
	created := time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)
	shipment := rawShipment{ID: "ship-1", ShipmentNumber: "SHP-1"}
	events := buildDocumentEvents(shipment, []rawDocument{{
		ID: "doc-1", DocumentType: "POD", DocumentStatus: "SIGNED", CreatedAt: &created,
	}})
	for _, event := range events {
		if event.Type == EventTypeDocumentSigned {
			t.Fatal("DOCUMENT_SIGNED must not be created without signedAt")
		}
	}
}

func TestDocumentSignedWithSignedAtCreated(t *testing.T) {
	created := time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)
	signed := time.Date(2026, 7, 31, 11, 0, 0, 0, time.UTC)
	shipment := rawShipment{ID: "ship-1", ShipmentNumber: "SHP-1"}
	events := buildDocumentEvents(shipment, []rawDocument{{
		ID: "doc-1", DocumentType: "POD", DocumentStatus: "SIGNED", CreatedAt: &created, SignedAt: &signed,
	}})
	found := false
	for _, event := range events {
		if event.Type == EventTypeDocumentSigned {
			found = true
			if !event.Derived || event.OccurredAt != signed.UTC() {
				t.Fatalf("unexpected signed event: %#v", event)
			}
		}
	}
	if !found {
		t.Fatal("expected DOCUMENT_SIGNED when signedAt is present")
	}
}

func TestDocumentRejectedWithoutRejectedAtNotCreated(t *testing.T) {
	created := time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)
	shipment := rawShipment{ID: "ship-1", ShipmentNumber: "SHP-1"}
	events := buildDocumentEvents(shipment, []rawDocument{{
		ID: "doc-1", DocumentType: "POD", DocumentStatus: "REJECTED", CreatedAt: &created,
	}})
	for _, event := range events {
		if event.Type == EventTypeDocumentRejected {
			t.Fatal("DOCUMENT_REJECTED must not be created without rejectedAt")
		}
	}
}

func TestShipmentCancelledNotCreatedFromUpdatedAt(t *testing.T) {
	updated := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	shipment := rawShipment{
		ID: "ship-1", ShipmentNumber: "SHP-1", Status: "CANCELLED", UpdatedAt: &updated,
	}
	events := buildDerivedShipmentEvents(shipment)
	for _, event := range events {
		if event.Type == EventTypeShipmentCancelled {
			t.Fatal("SHIPMENT_CANCELLED must not be derived from updatedAt")
		}
	}
}

func TestReadyForBillingNotCreatedWithoutTimestamp(t *testing.T) {
	updated := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	shipment := rawShipment{
		ID: "ship-1", ShipmentNumber: "SHP-1", Status: "READY_FOR_BILLING", UpdatedAt: &updated,
	}
	events := buildDerivedShipmentEvents(shipment)
	for _, event := range events {
		if event.Type == EventTypeReadyForBilling || event.Type == EventTypeFinanciallyClosed {
			t.Fatal("billing milestone must not use shipment status without dedicated timestamp")
		}
	}
}

func TestMetadataAllowlistStripsForbiddenFields(t *testing.T) {
	meta := sanitizeMetadata(map[string]interface{}{
		"documentId":   "doc-1",
		"storageUrl":   "https://internal.example/doc",
		"email":        "user@example.com",
		"phone":        "+123",
		"signature":    "sig",
		"errorMessage": "sql error",
	})
	if meta == nil || meta["documentId"] != "doc-1" {
		t.Fatalf("expected allowlisted field, got %#v", meta)
	}
	for _, forbidden := range []string{"storageUrl", "email", "phone", "signature", "errorMessage"} {
		if _, ok := meta[forbidden]; ok {
			t.Fatalf("forbidden metadata key %q present", forbidden)
		}
	}
}

func TestRBACFinanceAndProcurementDenied(t *testing.T) {
	if CanViewShipmentEvents([]string{"FINANCE_MANAGER"}) {
		t.Fatal("FINANCE_MANAGER must be denied")
	}
	if CanViewShipmentEvents([]string{"PROCUREMENT_MANAGER"}) {
		t.Fatal("PROCUREMENT_MANAGER must be denied")
	}
}

func TestRBACTable(t *testing.T) {
	tests := []struct {
		roles []string
		allow bool
	}{
		{[]string{"PLATFORM_ADMIN"}, true},
		{[]string{"CARRIER_DISPATCHER"}, true},
		{[]string{"SHIPPER_ADMIN"}, true},
		{[]string{"SHIPPER_LOGIST"}, true},
		{[]string{"FORWARDER_MANAGER"}, true},
		{[]string{"FINANCE_MANAGER"}, false},
		{[]string{"PROCUREMENT_MANAGER"}, false},
		{[]string{"UNKNOWN_ROLE"}, false},
		{nil, false},
	}
	for _, tt := range tests {
		if got := CanViewShipmentEvents(tt.roles); got != tt.allow {
			t.Fatalf("roles=%v allow=%v got=%v", tt.roles, tt.allow, got)
		}
	}
}

func TestFreshnessDerivedShipmentEventsLoadedFalse(t *testing.T) {
	svc := NewService(testServiceConfig(), testDownstreamClient(t))
	resp, err := svc.GetEvents(t.Context(), RequestContext{TenantID: "11111111-1111-1111-1111-111111111111"}, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", ListQuery{Page: 1, Limit: 50, Order: "desc"})
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	if resp.DataFreshness.ShipmentEventsLoaded {
		t.Fatal("shipmentEventsLoaded must be false when history is derived")
	}
	if !resp.DataFreshness.Partial {
		t.Fatal("partial must be true for derived history")
	}
	hasDerivedWarning := false
	for _, warning := range resp.DataFreshness.Warnings {
		if warning == WarningShipmentHistoryDerived {
			hasDerivedWarning = true
		}
		if strings.Contains(strings.ToLower(warning), "http") || strings.Contains(strings.ToLower(warning), "sql") {
			t.Fatalf("warning must not contain internal text: %q", warning)
		}
	}
	if !hasDerivedWarning {
		t.Fatalf("expected derived warning, got %#v", resp.DataFreshness.Warnings)
	}
}

func TestNoGeolocationRuntimeWarningByDefault(t *testing.T) {
	svc := NewService(testServiceConfig(), testDownstreamClient(t))
	resp, err := svc.GetEvents(t.Context(), RequestContext{TenantID: "11111111-1111-1111-1111-111111111111"}, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", ListQuery{Page: 1, Limit: 50, Order: "desc"})
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	for _, warning := range resp.DataFreshness.Warnings {
		if warning == WarningGeolocationEventsUnavailable || warning == WarningTechnicalEventsUnavailable {
			t.Fatalf("unsupported capability must not emit runtime warning: %q", warning)
		}
	}
}

func TestBillingLookupDoesNotScanRegisters(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	shipmentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	billingCalls := 0

	shipmentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": shipmentID, "tenant_id": tenantID, "shipment_number": "SHP-1", "status": "IN_TRANSIT",
			"created_at": "2026-07-31T10:00:00Z",
		})
	}))
	defer shipmentServer.Close()

	documentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
	}))
	defer documentServer.Close()

	billingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		billingCalls++
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
	}))
	defer billingServer.Close()

	client := NewDownstreamClient(&http.Client{Timeout: 5 * time.Second}, "", shipmentServer.URL, documentServer.URL, billingServer.URL, 200)
	svc := NewService(testServiceConfig(), client)
	resp, err := svc.GetEvents(context.Background(), RequestContext{TenantID: tenantID}, shipmentID, ListQuery{Page: 1, Limit: 50, Order: "desc"})
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	if billingCalls != 0 {
		t.Fatalf("expected zero billing HTTP requests, got %d", billingCalls)
	}
	if resp.DataFreshness.BillingLoaded {
		t.Fatal("billingLoaded must be false without reverse lookup")
	}
	hasBillingWarning := false
	for _, warning := range resp.DataFreshness.Warnings {
		if warning == WarningBillingEventsUnavailable {
			hasBillingWarning = true
		}
	}
	if !hasBillingWarning {
		t.Fatalf("expected billing capability warning, got %#v", resp.DataFreshness.Warnings)
	}
}

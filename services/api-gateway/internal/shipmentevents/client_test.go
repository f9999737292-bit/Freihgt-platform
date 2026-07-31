package shipmentevents

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchShipmentSendsVerifiedTenantHeaderOnly(t *testing.T) {
	t.Parallel()
	tenantID := "11111111-1111-1111-1111-111111111111"
	shipmentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Tenant-ID"); got != tenantID {
			t.Fatalf("X-Tenant-ID=%q want %q", got, tenantID)
		}
		if got := r.URL.Query().Get("tenant_id"); got != "" {
			t.Fatalf("tenant_id query must be absent, got %q", got)
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/shipments/"+shipmentID) {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"` + shipmentID + `","tenant_id":"` + tenantID + `","shipment_number":"SHP-1","status":"IN_TRANSIT","created_at":"2026-07-31T10:00:00Z","updated_at":"2026-07-31T11:00:00Z"}`))
	}))
	defer server.Close()

	client := NewDownstreamClient(nil, "", server.URL, "", "", 200)
	result, err := client.FetchShipment(context.Background(), RequestContext{TenantID: tenantID}, shipmentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NotFound || result.Shipment == nil {
		t.Fatalf("expected shipment, got %#v", result)
	}
}

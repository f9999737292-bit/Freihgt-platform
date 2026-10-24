package transportorderclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestAwardConversionCreatesOrderWithoutPredecessorID(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	eventID := uuid.New()
	lotID := uuid.New()
	orderID := uuid.New()
	snapshotID := uuid.New()
	var seen []map[string]any
	var tenants []string
	var keys []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/transport-orders/from-award-scope" {
			t.Errorf("path=%s", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("body: %v", err)
		}
		seen = append(seen, body)
		tenants = append(tenants, r.Header.Get("X-Tenant-ID"))
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":               orderID.String(),
			"order_number":     "TO-AWD-1",
			"status":           "DRAFT",
			"rate_snapshot_id": snapshotID.String(),
		})
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL, InternalServiceToken: "internal-token"})
	lot := lotID
	input := CreateFromAwardScopeRequest{
		TenantID:              tenantID,
		RfxEventID:            eventID,
		RfxLotID:              &lot,
		OrderNumber:           "TO-AWD-1",
		ShipperCompanyID:      uuid.New(),
		ConsigneeCompanyID:    uuid.New(),
		OriginLocationID:      uuid.New(),
		DestinationLocationID: uuid.New(),
		CargoID:               uuid.New(),
		TransportMode:         "ROAD",
		EquipmentType:         "TILT",
		CarrierCompanyID:      uuid.New(),
		ActorUserID:           uuid.New(),
		ActorCompanyID:        uuid.New(),
		IdempotencyKey:        AwardConversionIdempotencyKey(tenantID, eventID, lotID),
	}
	first, err := client.CreateFromAwardScope(context.Background(), input)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := client.CreateFromAwardScope(context.Background(), input)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first.TransportOrderID != orderID || second.TransportOrderID != orderID {
		t.Fatalf("orders=%s/%s", first.TransportOrderID, second.TransportOrderID)
	}
	if len(seen) != 2 {
		t.Fatalf("calls=%d", len(seen))
	}
	for i, body := range seen {
		if _, ok := body["transport_order_id"]; ok {
			t.Fatalf("call %d carries a predecessor transport_order_id", i)
		}
		if body["rfx_event_id"] != eventID.String() {
			t.Fatalf("event=%v", body["rfx_event_id"])
		}
		if tenants[i] != tenantID.String() {
			t.Fatalf("tenant header=%s", tenants[i])
		}
		if keys[i] != input.IdempotencyKey || keys[0] != keys[1] {
			t.Fatalf("idempotency keys=%v", keys)
		}
	}
}

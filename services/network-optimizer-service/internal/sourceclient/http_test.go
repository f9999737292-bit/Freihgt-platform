package sourceclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/predict"
	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shared-go/lowcode"
)

func TestBNO50ToBNO52PredictionReadsRequireServiceTokenAndTenant(t *testing.T) {
	tenant := uuid.New()
	shipmentID := uuid.New()
	var gotToken, gotTenant string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get(internalauth.HeaderName)
		gotTenant = r.Header.Get(lowcode.HeaderTenantID)
		if gotToken != "platform-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if gotTenant != tenant.String() {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(w, `{"id":"`+shipmentID.String()+`","tenant_id":"`+tenant.String()+`","status":"IN_TRANSIT","version":1,"destination_location_id":"`+uuid.NewString()+`","vehicle_id":null,"destination_latitude":null,"destination_longitude":null,"planned_delivery_at":null,"other_active_vehicle_shipments":0}`)
	}))
	t.Cleanup(upstream.Close)
	client := New(upstream.URL, upstream.URL, "platform-token")
	fact, err := client.Shipment(context.Background(), tenant, shipmentID)
	if err != nil || fact.Status != "IN_TRANSIT" || gotToken != "platform-token" || gotTenant != tenant.String() {
		t.Fatalf("BNO51 err=%v fact=%+v token=%s tenant=%s", err, fact, gotToken, gotTenant)
	}
	missing := New(upstream.URL, upstream.URL, "")
	if _, err := missing.Shipment(context.Background(), tenant, shipmentID); err != predict.ErrUnavailable {
		t.Fatalf("BNO50 err=%v", err)
	}
	foreign, err := client.Shipment(context.Background(), uuid.New(), shipmentID)
	if err != predict.ErrNotFound || foreign.ID != uuid.Nil {
		t.Fatalf("BNO52 err=%v", err)
	}
}

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/service"
)

func TestShipperTrackingContextParticipantProof(t *testing.T) {
	tenantA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tenantB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	shipperA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	shipperB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	other := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	shipmentA := uuid.MustParse("00000000-0000-0000-0000-0000000000a1")
	plannedPickup := time.Date(2026, 10, 11, 8, 0, 0, 0, time.UTC)
	plannedDelivery := time.Date(2026, 10, 12, 18, 0, 0, 0, time.UTC)
	actualPickup := time.Date(2026, 10, 11, 8, 30, 0, 0, time.UTC)
	owned := domain.Shipment{
		ID: shipmentA, TenantID: tenantA, ShipperCompanyID: shipperA, ConsigneeCompanyID: other,
		ShipmentNumber: "SHP-SECRET", Status: domain.ShipmentStatusInTransit,
		PlannedPickupAt: &plannedPickup, PlannedDeliveryAt: &plannedDelivery, ActualPickupAt: &actualPickup,
		CreatedAt: plannedPickup, UpdatedAt: plannedPickup,
	}
	forwarderOfA := uuid.MustParse("00000000-0000-0000-0000-0000000000d1")
	consigneeOfA := uuid.MustParse("00000000-0000-0000-0000-0000000000e1")
	carrierOfA := uuid.MustParse("00000000-0000-0000-0000-0000000000f1")
	rows := []domain.Shipment{
		owned,
		{ID: forwarderOfA, TenantID: tenantA, ShipperCompanyID: shipperB, ConsigneeCompanyID: other, ForwarderCompanyID: &shipperA, Status: domain.ShipmentStatusInTransit, ShipmentNumber: "FWD-SECRET", CreatedAt: plannedPickup, UpdatedAt: plannedPickup},
		{ID: consigneeOfA, TenantID: tenantA, ShipperCompanyID: shipperB, ConsigneeCompanyID: shipperA, Status: domain.ShipmentStatusInTransit, ShipmentNumber: "CNE-SECRET", CreatedAt: plannedPickup, UpdatedAt: plannedPickup},
		{ID: carrierOfA, TenantID: tenantA, ShipperCompanyID: shipperB, ConsigneeCompanyID: other, CarrierCompanyID: &shipperA, Status: domain.ShipmentStatusInTransit, ShipmentNumber: "CAR-SECRET", CreatedAt: plannedPickup, UpdatedAt: plannedPickup},
	}
	lookups := 0
	store := &tenantScopedShipmentService{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			t.Fatal("tenant-wide shipment fetch must not authorize tracking context")
			return nil, nil
		},
		getByShipperFn: func(_ context.Context, id, tenantID, shipperCompanyID uuid.UUID) (*domain.Shipment, error) {
			lookups++
			for _, item := range rows {
				if item.ID == id && item.TenantID == tenantID && item.ShipperCompanyID == shipperCompanyID {
					copy := item
					return &copy, nil
				}
			}
			return nil, apperrors.NotFound("shipment not found")
		},
	}
	handler := NewShipmentHandler(service.NewShipmentService(store, nil, nil))
	auth := internalauth.Config{Token: "tracking-context-token"}
	router := chi.NewRouter()
	router.With(auth.Middleware).Get("/internal/v1/shipments/{shipmentId}/shipper-tracking-context", handler.GetShipperTrackingContext)

	call := func(shipment, company, tenant, token string) *httptest.ResponseRecorder {
		path := "/internal/v1/shipments/" + shipment + "/shipper-tracking-context"
		if company != "" {
			path += "?shipper_company_id=" + company
		}
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if tenant != "" {
			req.Header.Set("X-Tenant-ID", tenant)
		}
		if token != "" {
			req.Header.Set("X-Internal-Service-Token", token)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	t.Run("STC-01 matching shipper", func(t *testing.T) {
		rec := call(shipmentA.String(), shipperA.String(), tenantA.String(), "tracking-context-token")
		if rec.Code != http.StatusOK {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload["shipmentId"] != shipmentA.String() || payload["tenantId"] != tenantA.String() || payload["shipperCompanyId"] != shipperA.String() {
			t.Fatalf("%s", rec.Body.String())
		}
		if payload["status"] != domain.ShipmentStatusInTransit || payload["plannedDeliveryAt"] != "2026-10-12T18:00:00Z" || payload["actualDeliveryAt"] != nil {
			t.Fatalf("%s", rec.Body.String())
		}
		for _, forbidden := range []string{"shipment_number", "SHP-SECRET", "cargo", "forwarder", "consignee", "carrier", "price"} {
			if strings.Contains(rec.Body.String(), forbidden) {
				t.Fatalf("disclosed %s in %s", forbidden, rec.Body.String())
			}
		}
		if lookups != 1 {
			t.Fatalf("lookups=%d", lookups)
		}
	})
	t.Run("STC-02 wrong shipper", func(t *testing.T) {
		rec := call(shipmentA.String(), shipperB.String(), tenantA.String(), "tracking-context-token")
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "SHP-SECRET") || strings.Contains(strings.ToLower(rec.Body.String()), "another company") {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("STC-03 wrong tenant", func(t *testing.T) {
		rec := call(shipmentA.String(), shipperA.String(), tenantB.String(), "tracking-context-token")
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "SHP-SECRET") {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("STC-04 forwarder is not shipper", func(t *testing.T) {
		rec := call(forwarderOfA.String(), shipperA.String(), tenantA.String(), "tracking-context-token")
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "FWD-SECRET") {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("STC-05 consignee is not shipper", func(t *testing.T) {
		rec := call(consigneeOfA.String(), shipperA.String(), tenantA.String(), "tracking-context-token")
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "CNE-SECRET") {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("STC-06 carrier is not shipper", func(t *testing.T) {
		rec := call(carrierOfA.String(), shipperA.String(), tenantA.String(), "tracking-context-token")
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "CAR-SECRET") {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("STC-07 missing company", func(t *testing.T) {
		before := lookups
		rec := call(shipmentA.String(), "", tenantA.String(), "tracking-context-token")
		if rec.Code != http.StatusBadRequest || lookups != before {
			t.Fatalf("%d lookups=%d %s", rec.Code, lookups, rec.Body.String())
		}
	})
	t.Run("STC-08 malformed company", func(t *testing.T) {
		before := lookups
		rec := call(shipmentA.String(), "not-a-uuid", tenantA.String(), "tracking-context-token")
		if rec.Code != http.StatusBadRequest || lookups != before {
			t.Fatalf("%d lookups=%d %s", rec.Code, lookups, rec.Body.String())
		}
	})
	t.Run("STC-09 missing and wrong token", func(t *testing.T) {
		before := lookups
		missing := call(shipmentA.String(), shipperA.String(), tenantA.String(), "")
		wrong := call(shipmentA.String(), shipperA.String(), tenantA.String(), "other-token")
		if missing.Code != http.StatusUnauthorized || wrong.Code != http.StatusUnauthorized || lookups != before {
			t.Fatalf("missing=%d wrong=%d lookups=%d", missing.Code, wrong.Code, lookups)
		}
	})
}

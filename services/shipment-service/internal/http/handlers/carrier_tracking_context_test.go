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

func TestCarrierTrackingContextParticipantProof(t *testing.T) {
	tenantA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tenantB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	carrierA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	carrierB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	shipperA := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	forwarderA := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	consigneeA := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	shipmentA := uuid.MustParse("00000000-0000-0000-0000-0000000000a1")
	deletedID := uuid.MustParse("00000000-0000-0000-0000-0000000000de")
	plannedPickup := time.Date(2026, 10, 11, 8, 0, 0, 0, time.UTC)
	plannedDelivery := time.Date(2026, 10, 12, 18, 0, 0, 0, time.UTC)
	ptr := func(id uuid.UUID) *uuid.UUID { return &id }
	owned := domain.Shipment{
		ID: shipmentA, TenantID: tenantA, ShipperCompanyID: shipperA, ConsigneeCompanyID: consigneeA,
		CarrierCompanyID: ptr(carrierA), ForwarderCompanyID: ptr(forwarderA),
		ShipmentNumber: "SHP-SECRET", Status: domain.ShipmentStatusInTransit,
		PlannedPickupAt: &plannedPickup, PlannedDeliveryAt: &plannedDelivery,
		CreatedAt: plannedPickup, UpdatedAt: plannedPickup,
	}
	rows := []domain.Shipment{owned}
	deleted := map[uuid.UUID]bool{deletedID: true}
	lookups := 0
	tenantWide := 0
	store := &tenantScopedShipmentService{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			tenantWide++
			t.Fatal("tenant-wide shipment fetch must not authorize carrier tracking context")
			return nil, nil
		},
		getByCarrierFn: func(_ context.Context, id, tenantID, carrierCompanyID uuid.UUID) (*domain.Shipment, error) {
			lookups++
			if deleted[id] {
				return nil, apperrors.NotFound("shipment not found")
			}
			for _, item := range rows {
				if item.ID == id && item.TenantID == tenantID && item.CarrierCompanyID != nil && *item.CarrierCompanyID == carrierCompanyID {
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
	router.With(auth.Middleware).Get("/internal/v1/shipments/{shipmentId}/carrier-tracking-context", handler.GetCarrierTrackingContext)

	call := func(shipment, company, tenant, token string) *httptest.ResponseRecorder {
		path := "/internal/v1/shipments/" + shipment + "/carrier-tracking-context"
		if company != "" {
			path += "?carrier_company_id=" + company
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
	denied := func(name, company, tenant, shipment string) {
		t.Run(name, func(t *testing.T) {
			rec := call(shipment, company, tenant, "tracking-context-token")
			body := strings.ToLower(rec.Body.String())
			if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "SHP-SECRET") || strings.Contains(body, "another carrier") || strings.Contains(body, "carrier mismatch") || strings.Contains(rec.Body.String(), carrierB.String()) {
				t.Fatalf("%d %s", rec.Code, rec.Body.String())
			}
		})
	}

	t.Run("matching carrier", func(t *testing.T) {
		rec := call(shipmentA.String(), carrierA.String(), tenantA.String(), "tracking-context-token")
		if rec.Code != http.StatusOK {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload["shipmentId"] != shipmentA.String() || payload["tenantId"] != tenantA.String() || payload["carrierCompanyId"] != carrierA.String() {
			t.Fatalf("%s", rec.Body.String())
		}
		if payload["status"] != domain.ShipmentStatusInTransit || payload["plannedDeliveryAt"] != "2026-10-12T18:00:00Z" {
			t.Fatalf("%s", rec.Body.String())
		}
		for _, forbidden := range []string{"shipment_number", "SHP-SECRET", shipperA.String(), forwarderA.String(), consigneeA.String(), "cargo", "rate", "billing"} {
			if strings.Contains(rec.Body.String(), forbidden) {
				t.Fatalf("disclosed %s in %s", forbidden, rec.Body.String())
			}
		}
		if lookups != 1 || tenantWide != 0 {
			t.Fatalf("lookups=%d tenantWide=%d", lookups, tenantWide)
		}
	})
	denied("wrong carrier", carrierB.String(), tenantA.String(), shipmentA.String())
	denied("wrong tenant", carrierA.String(), tenantB.String(), shipmentA.String())
	denied("shipper is not carrier", shipperA.String(), tenantA.String(), shipmentA.String())
	denied("forwarder is not carrier", forwarderA.String(), tenantA.String(), shipmentA.String())
	denied("consignee is not carrier", consigneeA.String(), tenantA.String(), shipmentA.String())
	denied("deleted shipment", carrierA.String(), tenantA.String(), deletedID.String())

	t.Run("missing company", func(t *testing.T) {
		before := lookups
		rec := call(shipmentA.String(), "", tenantA.String(), "tracking-context-token")
		if rec.Code != http.StatusBadRequest || lookups != before {
			t.Fatalf("%d lookups=%d %s", rec.Code, lookups, rec.Body.String())
		}
	})
	t.Run("malformed company", func(t *testing.T) {
		before := lookups
		rec := call(shipmentA.String(), "not-a-uuid", tenantA.String(), "tracking-context-token")
		if rec.Code != http.StatusBadRequest || lookups != before {
			t.Fatalf("%d lookups=%d %s", rec.Code, lookups, rec.Body.String())
		}
	})
	t.Run("missing and wrong token", func(t *testing.T) {
		before := lookups
		missing := call(shipmentA.String(), carrierA.String(), tenantA.String(), "")
		wrong := call(shipmentA.String(), carrierA.String(), tenantA.String(), "other-token")
		if missing.Code != http.StatusUnauthorized || wrong.Code != http.StatusUnauthorized || lookups != before {
			t.Fatalf("missing=%d wrong=%d lookups=%d", missing.Code, wrong.Code, lookups)
		}
	})
}

package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/tracking-service/internal/domain"
	apperrors "github.com/freight-platform/tracking-service/internal/platform/errors"
	"github.com/freight-platform/tracking-service/internal/service"
)

type httpProver struct {
	calls    int
	tenant   uuid.UUID
	shipper  uuid.UUID
	shipment uuid.UUID
}

func (p *httpProver) Prove(_ context.Context, tenantID, shipmentID, shipperCompanyID uuid.UUID) (service.ShipperTrackingContext, error) {
	p.calls++
	p.tenant, p.shipment, p.shipper = tenantID, shipmentID, shipperCompanyID
	if tenantID != uuid.MustParse("11111111-1111-1111-1111-111111111111") || shipperCompanyID != uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa") {
		return service.ShipperTrackingContext{}, apperrors.NotFound("shipment not found")
	}
	planned := time.Date(2026, 10, 12, 18, 0, 0, 0, time.UTC)
	return service.ShipperTrackingContext{
		ShipmentID: shipmentID, TenantID: tenantID, ShipperCompanyID: shipperCompanyID,
		Status: "in_transit", PlannedDeliveryAt: &planned,
	}, nil
}

type httpTracking struct {
	calls int
}

func (r *httpTracking) GetTrackingSummary(context.Context, uuid.UUID, uuid.UUID) (domain.TrackingSummary, error) {
	r.calls++
	provider := "secret-provider"
	speed := 42.5
	return domain.TrackingSummary{
		ShipmentID: uuid.MustParse("00000000-0000-0000-0000-0000000000a1"), TrackingStatus: domain.TrackingStatusActive,
		Provider: &provider, SpeedKph: &speed, Freshness: domain.FreshnessSummary{Status: domain.FreshnessFresh}, Quality: domain.QualitySummary{Status: domain.QualityGood},
	}, nil
}

func (r *httpTracking) ListLocationHistory(context.Context, uuid.UUID, uuid.UUID, *time.Time, *time.Time, int, int) ([]domain.LocationEvent, int, error) {
	r.calls++
	eventID := "provider-event-secret"
	return []domain.LocationEvent{{
		ID: uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd"), ShipmentID: uuid.MustParse("00000000-0000-0000-0000-0000000000a1"),
		ProviderCode: "secret-provider", ProviderDeviceID: "provider-device-secret", ProviderEventID: &eventID,
		Latitude: 55.7, Longitude: 37.6, RecordedAt: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC), ReceivedAt: time.Date(2026, 10, 10, 12, 0, 1, 0, time.UTC),
		SourceType: domain.SourceDriverMobile, QualityStatus: domain.QualityGood,
	}}, 1, nil
}

type httpETA struct {
	calls   int
	planned service.PlannedTimes
}

func (r *httpETA) GetShipmentETA(_ context.Context, _, shipmentID uuid.UUID, planned service.PlannedTimes) (domain.ShipmentETASummary, error) {
	r.calls++
	r.planned = planned
	provider := "secret-provider"
	confidence := 0.99
	return domain.ShipmentETASummary{ShipmentID: shipmentID, Delivery: &domain.ETATargetSummary{
		Status: domain.ETAStatusAvailable, FreshnessStatus: domain.ETAFreshnessFresh, QualityStatus: domain.ETAQualityGood,
		Provider: &provider, ProviderConfidence: &confidence, ArrivalProjection: domain.ArrivalOnTime,
	}}, nil
}

func (r *httpETA) ListETAHistory(context.Context, uuid.UUID, uuid.UUID, string, *time.Time, *time.Time, int, int) ([]domain.ETAObservation, int, error) {
	r.calls++
	eventID := "eta-provider-event-secret"
	provider := "secret-provider"
	return []domain.ETAObservation{{
		ID: uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"), ShipmentID: uuid.MustParse("00000000-0000-0000-0000-0000000000a1"),
		TargetType: domain.TargetDelivery, EstimatedArrivalAt: time.Date(2026, 10, 12, 19, 0, 0, 0, time.UTC),
		SourceType: domain.ETASourceCalculated, ProviderCode: &provider, ProviderEventID: &eventID,
		SourceObservedAt: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC), ReceivedAt: time.Date(2026, 10, 10, 12, 0, 1, 0, time.UTC),
		QualityStatus: domain.ETAQualityGood,
	}}, 1, nil
}

type httpSlots struct {
	calls     int
	milestone service.SlotMilestoneContext
}

func (r *httpSlots) GetShipmentSlots(_ context.Context, _, shipmentID uuid.UUID, milestone service.SlotMilestoneContext) (domain.ShipmentSlotSummary, error) {
	r.calls++
	r.milestone = milestone
	provider := "secret-provider"
	slotID := "provider-slot-secret"
	return domain.ShipmentSlotSummary{ShipmentID: shipmentID, Delivery: &domain.SlotTargetSummary{
		WindowStatus: "booked", QualityStatus: domain.QualityGood, ArrivalProjection: domain.ArrivalOnTime, ETARelation: "inside",
		Provider: &provider, ProviderSlotID: &slotID,
	}}, nil
}

func (r *httpSlots) ListSlotHistory(context.Context, uuid.UUID, uuid.UUID, string, *time.Time, *time.Time, int, int) ([]domain.SlotRevision, int, error) {
	r.calls++
	slotID := "provider-slot-secret"
	return []domain.SlotRevision{{
		ID: uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff"), ShipmentID: uuid.MustParse("00000000-0000-0000-0000-0000000000a1"),
		SlotType: domain.SlotTypeDelivery, WindowStart: time.Date(2026, 10, 12, 16, 0, 0, 0, time.UTC), WindowEnd: time.Date(2026, 10, 12, 18, 0, 0, 0, time.UTC),
		SlotStatus: "booked", SourceType: domain.SlotSourceShipperAPI, ProviderSlotID: &slotID,
		SourceObservedAt: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC), ReceivedAt: time.Date(2026, 10, 10, 12, 0, 1, 0, time.UTC), QualityStatus: domain.QualityGood,
	}}, 1, nil
}

func shipperRouter(prover *httpProver, tracking *httpTracking, eta *httpETA, slots *httpSlots) http.Handler {
	handler := NewShipperTrackingHandler(service.NewShipperTrackingSource(prover, tracking, eta, slots))
	r := chi.NewRouter()
	r.Get("/v1/shipper/shipments/{shipmentId}/tracking", handler.GetTracking)
	r.Get("/v1/shipper/shipments/{shipmentId}/tracking/locations", handler.ListLocations)
	r.Get("/v1/shipper/shipments/{shipmentId}/eta", handler.GetETA)
	r.Get("/v1/shipper/shipments/{shipmentId}/eta/history", handler.ListETAHistory)
	r.Get("/v1/shipper/shipments/{shipmentId}/slots", handler.GetSlots)
	r.Get("/v1/shipper/shipments/{shipmentId}/slots/history", handler.ListSlotHistory)
	return r
}

func TestShipperTrackingSourceRoutes(t *testing.T) {
	tenantA := "11111111-1111-1111-1111-111111111111"
	tenantB := "22222222-2222-2222-2222-222222222222"
	shipperA := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	shipperB := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	shipmentA := "00000000-0000-0000-0000-0000000000a1"
	paths := []string{
		"/v1/shipper/shipments/" + shipmentA + "/tracking",
		"/v1/shipper/shipments/" + shipmentA + "/tracking/locations",
		"/v1/shipper/shipments/" + shipmentA + "/eta",
		"/v1/shipper/shipments/" + shipmentA + "/eta/history",
		"/v1/shipper/shipments/" + shipmentA + "/slots",
		"/v1/shipper/shipments/" + shipmentA + "/slots/history",
	}
	prover := &httpProver{}
	tracking := &httpTracking{}
	eta := &httpETA{}
	slots := &httpSlots{}
	router := shipperRouter(prover, tracking, eta, slots)
	call := func(path, tenant, company string) *httptest.ResponseRecorder {
		if company != "" {
			sep := "?"
			if strings.Contains(path, "?") {
				sep = "&"
			}
			path += sep + "shipper_company_id=" + company
		}
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if tenant != "" {
			req.Header.Set("X-Tenant-ID", tenant)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	for _, path := range paths {
		rec := call(path, tenantA, shipperA)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %d %s", path, rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		for _, leak := range []string{"providerDeviceId", "provider-device-secret", "providerEventId", "provider-event-secret", "eta-provider-event-secret", "providerSlotId", "provider-slot-secret", "secret-provider", "X-Internal-Service-Token", "dddddddd-dddd-dddd-dddd-dddddddddddd", "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", "ffffffff-ffff-ffff-ffff-ffffffffffff"} {
			if strings.Contains(body, leak) {
				t.Fatalf("%s leaked %s in %s", path, leak, body)
			}
		}
	}
	if prover.calls != len(paths) || tracking.calls != 2 || eta.calls != 3 || slots.calls != 2 {
		t.Fatalf("prover=%d tracking=%d eta=%d slots=%d", prover.calls, tracking.calls, eta.calls, slots.calls)
	}
	if eta.planned.ShipmentStatus != "in_transit" || eta.planned.PlannedDeliveryAt == nil || !eta.planned.PlannedDeliveryAt.Equal(time.Date(2026, 10, 12, 18, 0, 0, 0, time.UTC)) {
		t.Fatalf("planned %+v", eta.planned)
	}
	if slots.milestone.ShipmentStatus != "in_transit" {
		t.Fatalf("milestone %+v", slots.milestone)
	}

	before := prover.calls
	for _, path := range paths {
		foreign := call(path, tenantA, shipperB)
		other := call(path, tenantB, shipperA)
		if foreign.Code != http.StatusNotFound || other.Code != http.StatusNotFound {
			t.Fatalf("%s foreign=%d other=%d", path, foreign.Code, other.Code)
		}
	}
	if tracking.calls != 2 || eta.calls != 3 || slots.calls != 2 {
		t.Fatalf("denied read tracking=%d eta=%d slots=%d", tracking.calls, eta.calls, slots.calls)
	}
	if prover.calls != before+len(paths)*2 {
		t.Fatalf("prover=%d", prover.calls)
	}

	reads := tracking.calls + eta.calls + slots.calls
	for _, path := range paths {
		missing := call(path, tenantA, "")
		malformed := call(path, tenantA, "not-a-uuid")
		if missing.Code != http.StatusBadRequest || malformed.Code != http.StatusBadRequest {
			t.Fatalf("%s missing=%d malformed=%d", path, missing.Code, malformed.Code)
		}
	}
	if tracking.calls+eta.calls+slots.calls != reads || prover.calls != before+len(paths)*2 {
		t.Fatalf("validation called readers or prover")
	}
}

func TestShipperETAAndSlotTamperDenied(t *testing.T) {
	prover := &httpProver{}
	tracking := &httpTracking{}
	eta := &httpETA{}
	slots := &httpSlots{}
	router := shipperRouter(prover, tracking, eta, slots)
	shipmentA := "00000000-0000-0000-0000-0000000000a1"
	base := "shipper_company_id=aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	etaKeys := []string{"plannedPickupAt", "plannedDeliveryAt", "actualPickupAt", "actualDeliveryAt", "shipmentStatus"}
	slotKeys := []string{"shipmentStatus", "actualPickupAt", "actualDeliveryAt", "pickupEtaStatus", "deliveryEtaStatus", "pickupEstimatedArrivalAt", "deliveryEstimatedArrivalAt", "pickupEtaFreshness", "deliveryEtaFreshness", "pickupEtaQuality", "deliveryEtaQuality"}
	for _, key := range etaKeys {
		req := httptest.NewRequest(http.MethodGet, "/v1/shipper/shipments/"+shipmentA+"/eta?"+base+"&"+key+"=tampered", nil)
		req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s %d", key, rec.Code)
		}
	}
	for _, key := range slotKeys {
		req := httptest.NewRequest(http.MethodGet, "/v1/shipper/shipments/"+shipmentA+"/slots?"+base+"&"+key+"=tampered", nil)
		req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s %d", key, rec.Code)
		}
	}
	if prover.calls != 0 || eta.calls != 0 || slots.calls != 0 || tracking.calls != 0 {
		t.Fatalf("tamper reached source prover=%d eta=%d slots=%d tracking=%d", prover.calls, eta.calls, slots.calls, tracking.calls)
	}
}

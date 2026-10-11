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

type carrierHTTPProver struct {
	calls int
	err   error
}

func (p *carrierHTTPProver) Prove(_ context.Context, tenantID, shipmentID, carrierCompanyID uuid.UUID) (service.CarrierTrackingContext, error) {
	p.calls++
	if p.err != nil {
		return service.CarrierTrackingContext{}, p.err
	}
	if tenantID != uuid.MustParse("11111111-1111-1111-1111-111111111111") || carrierCompanyID != uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa") {
		return service.CarrierTrackingContext{}, apperrors.NotFound("shipment not found")
	}
	planned := time.Date(2026, 10, 12, 18, 0, 0, 0, time.UTC)
	return service.CarrierTrackingContext{
		ShipmentID: shipmentID, TenantID: tenantID, CarrierCompanyID: carrierCompanyID,
		Status: "in_transit", PlannedDeliveryAt: &planned,
	}, nil
}

type carrierHTTPTracking struct{ calls int }

func (r *carrierHTTPTracking) GetTrackingSummary(context.Context, uuid.UUID, uuid.UUID) (domain.TrackingSummary, error) {
	r.calls++
	provider := "secret-provider"
	return domain.TrackingSummary{
		ShipmentID: uuid.MustParse("00000000-0000-0000-0000-0000000000a1"), TrackingStatus: domain.TrackingStatusActive,
		Provider: &provider, Freshness: domain.FreshnessSummary{Status: domain.FreshnessFresh}, Quality: domain.QualitySummary{Status: domain.QualityGood},
	}, nil
}

func (r *carrierHTTPTracking) ListLocationHistory(context.Context, uuid.UUID, uuid.UUID, *time.Time, *time.Time, int, int) ([]domain.LocationEvent, int, error) {
	r.calls++
	eventID := "provider-event-secret"
	return []domain.LocationEvent{{
		ID: uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd"), ShipmentID: uuid.MustParse("00000000-0000-0000-0000-0000000000a1"),
		ProviderCode: "secret-provider", ProviderDeviceID: "provider-device-secret", ProviderEventID: &eventID,
		Latitude: 55.7, Longitude: 37.6, RecordedAt: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC), ReceivedAt: time.Date(2026, 10, 10, 12, 0, 1, 0, time.UTC),
		SourceType: domain.SourceDriverMobile, QualityStatus: domain.QualityGood,
	}}, 1, nil
}

type carrierHTTPETA struct {
	calls      int
	planned    service.PlannedTimes
	targetType string
	limit      int
}

func (r *carrierHTTPETA) GetShipmentETA(_ context.Context, _, shipmentID uuid.UUID, planned service.PlannedTimes) (domain.ShipmentETASummary, error) {
	r.calls++
	r.planned = planned
	provider := "secret-provider"
	confidence := 0.99
	return domain.ShipmentETASummary{ShipmentID: shipmentID, Delivery: &domain.ETATargetSummary{
		Status: domain.ETAStatusAvailable, FreshnessStatus: domain.ETAFreshnessFresh, QualityStatus: domain.ETAQualityGood,
		Provider: &provider, ProviderConfidence: &confidence, ArrivalProjection: domain.ArrivalOnTime,
	}}, nil
}

func (r *carrierHTTPETA) ListETAHistory(_ context.Context, _, _ uuid.UUID, targetType string, _, _ *time.Time, limit, _ int) ([]domain.ETAObservation, int, error) {
	r.calls++
	r.targetType = targetType
	r.limit = limit
	eventID := "eta-provider-event-secret"
	provider := "secret-provider"
	return []domain.ETAObservation{{
		ID: uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"), ShipmentID: uuid.MustParse("00000000-0000-0000-0000-0000000000a1"),
		TargetType: targetType, EstimatedArrivalAt: time.Date(2026, 10, 12, 19, 0, 0, 0, time.UTC),
		SourceType: domain.ETASourceCalculated, ProviderCode: &provider, ProviderEventID: &eventID,
		SourceObservedAt: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC), ReceivedAt: time.Date(2026, 10, 10, 12, 0, 1, 0, time.UTC),
		QualityStatus: domain.ETAQualityGood,
	}}, 1, nil
}

type carrierHTTPSlots struct {
	calls     int
	milestone service.SlotMilestoneContext
	slotType  string
}

func (r *carrierHTTPSlots) GetShipmentSlots(_ context.Context, _, shipmentID uuid.UUID, milestone service.SlotMilestoneContext) (domain.ShipmentSlotSummary, error) {
	r.calls++
	r.milestone = milestone
	provider := "secret-provider"
	slotID := "provider-slot-secret"
	return domain.ShipmentSlotSummary{ShipmentID: shipmentID, Delivery: &domain.SlotTargetSummary{
		WindowStatus: "booked", QualityStatus: domain.QualityGood, ArrivalProjection: domain.ArrivalOnTime, ETARelation: "inside",
		Provider: &provider, ProviderSlotID: &slotID,
	}}, nil
}

func (r *carrierHTTPSlots) ListSlotHistory(_ context.Context, _, _ uuid.UUID, slotType string, _, _ *time.Time, _, _ int) ([]domain.SlotRevision, int, error) {
	r.calls++
	r.slotType = slotType
	slotID := "provider-slot-secret"
	return []domain.SlotRevision{{
		ID: uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff"), ShipmentID: uuid.MustParse("00000000-0000-0000-0000-0000000000a1"),
		SlotType: slotType, WindowStart: time.Date(2026, 10, 12, 16, 0, 0, 0, time.UTC), WindowEnd: time.Date(2026, 10, 12, 18, 0, 0, 0, time.UTC),
		SlotStatus: "booked", SourceType: domain.SlotSourceShipperAPI, ProviderSlotID: &slotID,
		SourceObservedAt: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC), ReceivedAt: time.Date(2026, 10, 10, 12, 0, 1, 0, time.UTC), QualityStatus: domain.QualityGood,
	}}, 1, nil
}

func carrierRouter(prover *carrierHTTPProver, tracking *carrierHTTPTracking, eta *carrierHTTPETA, slots *carrierHTTPSlots) http.Handler {
	handler := NewCarrierTrackingHandler(service.NewCarrierTrackingSource(prover, tracking, eta, slots))
	r := chi.NewRouter()
	r.Get("/v1/carrier/shipments/{shipmentId}/tracking", handler.GetTracking)
	r.Get("/v1/carrier/shipments/{shipmentId}/tracking/locations", handler.ListLocations)
	r.Get("/v1/carrier/shipments/{shipmentId}/eta", handler.GetETA)
	r.Get("/v1/carrier/shipments/{shipmentId}/eta/history", handler.ListETAHistory)
	r.Get("/v1/carrier/shipments/{shipmentId}/slots", handler.GetSlots)
	r.Get("/v1/carrier/shipments/{shipmentId}/slots/history", handler.ListSlotHistory)
	return r
}

func TestCarrierTrackingSourceRoutes(t *testing.T) {
	tenantA := "11111111-1111-1111-1111-111111111111"
	tenantB := "22222222-2222-2222-2222-222222222222"
	carrierA := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	carrierB := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	shipmentA := "00000000-0000-0000-0000-0000000000a1"
	paths := []string{
		"/v1/carrier/shipments/" + shipmentA + "/tracking",
		"/v1/carrier/shipments/" + shipmentA + "/tracking/locations",
		"/v1/carrier/shipments/" + shipmentA + "/eta",
		"/v1/carrier/shipments/" + shipmentA + "/eta/history?targetType=pickup&limit=5",
		"/v1/carrier/shipments/" + shipmentA + "/slots",
		"/v1/carrier/shipments/" + shipmentA + "/slots/history?slotType=pickup",
	}
	prover := &carrierHTTPProver{}
	tracking := &carrierHTTPTracking{}
	eta := &carrierHTTPETA{}
	slots := &carrierHTTPSlots{}
	router := carrierRouter(prover, tracking, eta, slots)
	call := func(path, tenant, company string) *httptest.ResponseRecorder {
		if company != "" {
			sep := "?"
			if strings.Contains(path, "?") {
				sep = "&"
			}
			path += sep + "carrier_company_id=" + company
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
		rec := call(path, tenantA, carrierA)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %d %s", path, rec.Code, rec.Body.String())
		}
		for _, leak := range []string{"providerDeviceId", "provider-device-secret", "providerEventId", "provider-event-secret", "eta-provider-event-secret", "providerSlotId", "provider-slot-secret", "secret-provider", "providerConfidence", "X-Internal-Service-Token", "dddddddd-dddd-dddd-dddd-dddddddddddd", "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", "ffffffff-ffff-ffff-ffff-ffffffffffff"} {
			if strings.Contains(rec.Body.String(), leak) {
				t.Fatalf("%s leaked %s in %s", path, leak, rec.Body.String())
			}
		}
	}
	if prover.calls != len(paths) || tracking.calls != 2 || eta.calls != 3 || slots.calls != 2 {
		t.Fatalf("prover=%d tracking=%d eta=%d slots=%d", prover.calls, tracking.calls, eta.calls, slots.calls)
	}
	if eta.targetType != domain.TargetPickup || eta.limit != 5 || slots.slotType != domain.SlotTypePickup {
		t.Fatalf("filters target=%s limit=%d slot=%s", eta.targetType, eta.limit, slots.slotType)
	}
	if eta.planned.ShipmentStatus != "in_transit" || slots.milestone.ShipmentStatus != "in_transit" {
		t.Fatalf("planned %+v milestone %+v", eta.planned, slots.milestone)
	}
	reads := tracking.calls + eta.calls + slots.calls
	for _, path := range paths {
		foreign := call(path, tenantA, carrierB)
		other := call(path, tenantB, carrierA)
		missing := call(path, tenantA, "")
		malformed := call(path, tenantA, "not-a-uuid")
		if foreign.Code != http.StatusNotFound || other.Code != http.StatusNotFound || missing.Code != http.StatusBadRequest || malformed.Code != http.StatusBadRequest {
			t.Fatalf("%s foreign=%d other=%d missing=%d malformed=%d", path, foreign.Code, other.Code, missing.Code, malformed.Code)
		}
	}
	if tracking.calls+eta.calls+slots.calls != reads {
		t.Fatal("denied or invalid request read tracking data")
	}
	stopped := &carrierHTTPProver{err: apperrors.Unavailable("shipment participant check failed")}
	down := carrierRouter(stopped, tracking, eta, slots)
	req := httptest.NewRequest(http.MethodGet, "/v1/carrier/shipments/"+shipmentA+"/tracking?carrier_company_id="+carrierA, nil)
	req.Header.Set("X-Tenant-ID", tenantA)
	rec := httptest.NewRecorder()
	down.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || tracking.calls+eta.calls+slots.calls != reads {
		t.Fatalf("dependency %d reads changed", rec.Code)
	}
	badTarget := call("/v1/carrier/shipments/"+shipmentA+"/eta/history?targetType=execution_stop", tenantA, carrierA)
	if badTarget.Code != http.StatusBadRequest || eta.calls != 3 {
		t.Fatalf("execution_stop %d eta=%d", badTarget.Code, eta.calls)
	}
}

func TestCarrierETAAndSlotTamperDenied(t *testing.T) {
	prover := &carrierHTTPProver{}
	tracking := &carrierHTTPTracking{}
	eta := &carrierHTTPETA{}
	slots := &carrierHTTPSlots{}
	router := carrierRouter(prover, tracking, eta, slots)
	shipmentA := "00000000-0000-0000-0000-0000000000a1"
	base := "carrier_company_id=aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	etaKeys := []string{"plannedPickupAt", "plannedDeliveryAt", "actualPickupAt", "actualDeliveryAt", "shipmentStatus"}
	slotKeys := []string{"shipmentStatus", "actualPickupAt", "actualDeliveryAt", "pickupEtaStatus", "deliveryEtaStatus", "pickupEstimatedArrivalAt", "deliveryEstimatedArrivalAt", "pickupEtaFreshness", "deliveryEtaFreshness", "pickupEtaQuality", "deliveryEtaQuality"}
	for _, key := range etaKeys {
		req := httptest.NewRequest(http.MethodGet, "/v1/carrier/shipments/"+shipmentA+"/eta?"+base+"&"+key+"=tampered", nil)
		req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s %d", key, rec.Code)
		}
	}
	for _, key := range slotKeys {
		req := httptest.NewRequest(http.MethodGet, "/v1/carrier/shipments/"+shipmentA+"/slots?"+base+"&"+key+"=tampered", nil)
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

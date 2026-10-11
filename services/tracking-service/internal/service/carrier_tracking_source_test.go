package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/tracking-service/internal/domain"
	apperrors "github.com/freight-platform/tracking-service/internal/platform/errors"
)

type carrierOrderProver struct {
	calls int
	err   error
	facts CarrierTrackingContext
}

func (p *carrierOrderProver) Prove(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (CarrierTrackingContext, error) {
	p.calls++
	if p.err != nil {
		return CarrierTrackingContext{}, p.err
	}
	return p.facts, nil
}

func TestCarrierTrackingSourceProvesBeforeRead(t *testing.T) {
	tenant := uuid.New()
	shipment := uuid.New()
	carrier := uuid.New()
	planned := time.Date(2026, 10, 12, 18, 0, 0, 0, time.UTC)
	arrival := planned.Add(time.Hour)
	facts := CarrierTrackingContext{
		ShipmentID: shipment, TenantID: tenant, CarrierCompanyID: carrier,
		Status: "in_transit", PlannedDeliveryAt: &planned,
	}
	for _, proveErr := range []error{apperrors.NotFound("shipment not found"), apperrors.Unavailable("shipment participant check failed")} {
		order := []string{}
		prover := &carrierOrderProver{err: proveErr}
		tracking := &orderTracking{order: &order}
		eta := &orderETA{order: &order}
		slots := &orderSlots{order: &order}
		source := NewCarrierTrackingSource(prover, tracking, eta, slots)
		ctx := context.Background()
		if _, err := source.TrackingSummary(ctx, tenant, shipment, carrier); err == nil || tracking.calls != 0 {
			t.Fatalf("summary err=%v calls=%d", err, tracking.calls)
		}
		if _, _, err := source.LocationHistory(ctx, tenant, shipment, carrier, nil, nil, 10, 0); err == nil || tracking.calls != 0 {
			t.Fatalf("locations err=%v calls=%d", err, tracking.calls)
		}
		if _, err := source.ETA(ctx, tenant, shipment, carrier); err == nil || eta.calls != 0 {
			t.Fatalf("eta err=%v calls=%d", err, eta.calls)
		}
		if _, _, err := source.ETAHistory(ctx, tenant, shipment, carrier, domain.TargetDelivery, nil, nil, 10, 0); err == nil || eta.calls != 0 {
			t.Fatalf("eta history err=%v calls=%d", err, eta.calls)
		}
		if _, err := source.Slots(ctx, tenant, shipment, carrier); err == nil || eta.calls != 0 || slots.calls != 0 {
			t.Fatalf("slots err=%v eta=%d slots=%d", err, eta.calls, slots.calls)
		}
		if _, _, err := source.SlotHistory(ctx, tenant, shipment, carrier, domain.SlotTypeDelivery, nil, nil, 10, 0); err == nil || slots.calls != 0 {
			t.Fatalf("slot history err=%v calls=%d", err, slots.calls)
		}
		if prover.calls != 6 {
			t.Fatalf("lookups=%d", prover.calls)
		}
	}

	order := []string{}
	prover := &carrierOrderProver{facts: facts}
	tracking := &orderTracking{order: &order}
	eta := &orderETA{order: &order, summary: domain.ShipmentETASummary{
		ShipmentID: shipment,
		Delivery: &domain.ETATargetSummary{
			Status: domain.ETAStatusAvailable, FreshnessStatus: domain.ETAFreshnessFresh, QualityStatus: domain.ETAQualityGood, EstimatedArrivalAt: &arrival,
		},
	}}
	slots := &orderSlots{order: &order}
	source := NewCarrierTrackingSource(prover, tracking, eta, slots)
	ctx := context.Background()
	before := prover.calls
	if _, err := source.Slots(ctx, tenant, shipment, carrier); err != nil {
		t.Fatal(err)
	}
	if prover.calls-before != 1 {
		t.Fatalf("slot request lookups=%d", prover.calls-before)
	}
	if eta.planned.ShipmentStatus != "in_transit" || eta.planned.PlannedDeliveryAt == nil || !eta.planned.PlannedDeliveryAt.Equal(planned) {
		t.Fatalf("eta used caller facts: %+v", eta.planned)
	}
	if slots.milestone.ShipmentStatus != "in_transit" || !slots.milestone.DeliveryETA.HasUsableETA || slots.milestone.DeliveryETA.EstimatedArrivalAt == nil || !slots.milestone.DeliveryETA.EstimatedArrivalAt.Equal(arrival) {
		t.Fatalf("slot milestone %+v", slots.milestone)
	}
	if _, _, err := source.ETAHistory(ctx, tenant, shipment, carrier, domain.TargetPickup, nil, nil, 20, 1); err != nil {
		t.Fatal(err)
	}
	if prover.calls != 2 || eta.calls != 2 {
		t.Fatalf("prover=%d eta=%d", prover.calls, eta.calls)
	}
}

func TestCarrierTrackingContextClientFailClosed(t *testing.T) {
	tenant := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	shipment := uuid.MustParse("00000000-0000-0000-0000-0000000000a1")
	carrier := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	if _, err := NewCarrierTrackingContextClient("", "token").Prove(context.Background(), tenant, shipment, carrier); !carrierContextUnavailable(err) {
		t.Fatalf("empty url: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Service-Token") != "internal-token" || r.URL.Query().Get("carrier_company_id") != carrier.String() {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"shipmentId": shipment.String(), "tenantId": tenant.String(), "carrierCompanyId": carrier.String(),
			"status": "in_transit", "plannedDeliveryAt": "2026-10-12T18:00:00Z",
		})
	}))
	defer server.Close()
	facts, err := NewCarrierTrackingContextClient(server.URL, "internal-token").Prove(context.Background(), tenant, shipment, carrier)
	if err != nil || facts.Status != "in_transit" || facts.CarrierCompanyID != carrier {
		t.Fatalf("facts=%+v err=%v", facts, err)
	}
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer down.Close()
	if _, err := NewCarrierTrackingContextClient(down.URL, "internal-token").Prove(context.Background(), tenant, shipment, carrier); !carrierContextUnavailable(err) {
		t.Fatalf("dependency: %v", err)
	}
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer foreign.Close()
	if _, err := NewCarrierTrackingContextClient(foreign.URL, "internal-token").Prove(context.Background(), tenant, shipment, carrier); !carrierContextNotFound(err) {
		t.Fatalf("404: %v", err)
	}
}

func carrierContextUnavailable(err error) bool {
	app, ok := err.(*apperrors.AppError)
	return ok && app.Code == apperrors.CodeUnavailable
}

func carrierContextNotFound(err error) bool {
	app, ok := err.(*apperrors.AppError)
	return ok && app.Code == apperrors.CodeNotFound
}

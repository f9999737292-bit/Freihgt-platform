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

type orderProver struct {
	order *[]string
	err   error
	facts ShipperTrackingContext
	calls int
}

func (p *orderProver) Prove(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (ShipperTrackingContext, error) {
	p.calls++
	*p.order = append(*p.order, "prove")
	if p.err != nil {
		return ShipperTrackingContext{}, p.err
	}
	return p.facts, nil
}

type orderTracking struct {
	order *[]string
	calls int
}

func (r *orderTracking) GetTrackingSummary(context.Context, uuid.UUID, uuid.UUID) (domain.TrackingSummary, error) {
	r.calls++
	*r.order = append(*r.order, "tracking")
	return domain.TrackingSummary{TrackingStatus: domain.TrackingStatusActive}, nil
}

func (r *orderTracking) ListLocationHistory(context.Context, uuid.UUID, uuid.UUID, *time.Time, *time.Time, int, int) ([]domain.LocationEvent, int, error) {
	r.calls++
	*r.order = append(*r.order, "locations")
	return []domain.LocationEvent{{Latitude: 1}}, 1, nil
}

type orderETA struct {
	order   *[]string
	calls   int
	planned PlannedTimes
	summary domain.ShipmentETASummary
}

func (r *orderETA) GetShipmentETA(_ context.Context, _, _ uuid.UUID, planned PlannedTimes) (domain.ShipmentETASummary, error) {
	r.calls++
	r.planned = planned
	*r.order = append(*r.order, "eta")
	if r.summary.ShipmentID == uuid.Nil {
		r.summary = domain.ShipmentETASummary{Delivery: &domain.ETATargetSummary{Status: domain.ETAStatusAvailable, FreshnessStatus: domain.ETAFreshnessFresh, QualityStatus: domain.ETAQualityGood}}
	}
	return r.summary, nil
}

func (r *orderETA) ListETAHistory(context.Context, uuid.UUID, uuid.UUID, string, *time.Time, *time.Time, int, int) ([]domain.ETAObservation, int, error) {
	r.calls++
	*r.order = append(*r.order, "eta-history")
	return nil, 0, nil
}

type orderSlots struct {
	order     *[]string
	calls     int
	milestone SlotMilestoneContext
}

func (r *orderSlots) GetShipmentSlots(_ context.Context, _, _ uuid.UUID, milestone SlotMilestoneContext) (domain.ShipmentSlotSummary, error) {
	r.calls++
	r.milestone = milestone
	*r.order = append(*r.order, "slots")
	return domain.ShipmentSlotSummary{}, nil
}

func (r *orderSlots) ListSlotHistory(context.Context, uuid.UUID, uuid.UUID, string, *time.Time, *time.Time, int, int) ([]domain.SlotRevision, int, error) {
	r.calls++
	*r.order = append(*r.order, "slot-history")
	return nil, 0, nil
}

func TestShipperTrackingSourceProvesBeforeRead(t *testing.T) {
	tenant := uuid.New()
	shipment := uuid.New()
	shipper := uuid.New()
	planned := time.Date(2026, 10, 12, 18, 0, 0, 0, time.UTC)
	arrival := planned.Add(time.Hour)
	facts := ShipperTrackingContext{
		ShipmentID: shipment, TenantID: tenant, ShipperCompanyID: shipper,
		Status: "in_transit", PlannedDeliveryAt: &planned,
	}

	denied := []error{apperrors.NotFound("shipment not found"), apperrors.Unavailable("shipment participant check failed")}
	for _, proveErr := range denied {
		order := []string{}
		prover := &orderProver{order: &order, err: proveErr}
		tracking := &orderTracking{order: &order}
		eta := &orderETA{order: &order}
		slots := &orderSlots{order: &order}
		source := NewShipperTrackingSource(prover, tracking, eta, slots)
		ctx := context.Background()
		if _, err := source.TrackingSummary(ctx, tenant, shipment, shipper); err == nil || tracking.calls != 0 {
			t.Fatalf("summary err=%v calls=%d", err, tracking.calls)
		}
		if _, _, err := source.LocationHistory(ctx, tenant, shipment, shipper, nil, nil, 10, 0); err == nil || tracking.calls != 0 {
			t.Fatalf("locations err=%v calls=%d", err, tracking.calls)
		}
		if _, err := source.ETA(ctx, tenant, shipment, shipper); err == nil || eta.calls != 0 {
			t.Fatalf("eta err=%v calls=%d", err, eta.calls)
		}
		if _, _, err := source.ETAHistory(ctx, tenant, shipment, shipper, domain.TargetDelivery, nil, nil, 10, 0); err == nil || eta.calls != 0 {
			t.Fatalf("eta history err=%v calls=%d", err, eta.calls)
		}
		if _, err := source.Slots(ctx, tenant, shipment, shipper); err == nil || eta.calls != 0 || slots.calls != 0 {
			t.Fatalf("slots err=%v eta=%d slots=%d", err, eta.calls, slots.calls)
		}
		if _, _, err := source.SlotHistory(ctx, tenant, shipment, shipper, domain.SlotTypeDelivery, nil, nil, 10, 0); err == nil || slots.calls != 0 {
			t.Fatalf("slot history err=%v calls=%d", err, slots.calls)
		}
	}

	order := []string{}
	prover := &orderProver{order: &order, facts: facts}
	tracking := &orderTracking{order: &order}
	eta := &orderETA{order: &order, summary: domain.ShipmentETASummary{
		ShipmentID: shipment,
		Delivery: &domain.ETATargetSummary{
			Status: domain.ETAStatusAvailable, FreshnessStatus: domain.ETAFreshnessFresh, QualityStatus: domain.ETAQualityGood, EstimatedArrivalAt: &arrival,
		},
	}}
	slots := &orderSlots{order: &order}
	source := NewShipperTrackingSource(prover, tracking, eta, slots)
	ctx := context.Background()
	if _, err := source.TrackingSummary(ctx, tenant, shipment, shipper); err != nil {
		t.Fatal(err)
	}
	if _, _, err := source.LocationHistory(ctx, tenant, shipment, shipper, nil, nil, 10, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := source.ETA(ctx, tenant, shipment, shipper); err != nil {
		t.Fatal(err)
	}
	if eta.planned.ShipmentStatus != "in_transit" || eta.planned.PlannedDeliveryAt == nil || !eta.planned.PlannedDeliveryAt.Equal(planned) {
		t.Fatalf("eta used caller facts: %+v", eta.planned)
	}
	if _, _, err := source.ETAHistory(ctx, tenant, shipment, shipper, domain.TargetDelivery, nil, nil, 10, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := source.Slots(ctx, tenant, shipment, shipper); err != nil {
		t.Fatal(err)
	}
	if slots.milestone.ShipmentStatus != "in_transit" || !slots.milestone.DeliveryETA.HasUsableETA || slots.milestone.DeliveryETA.EstimatedArrivalAt == nil || !slots.milestone.DeliveryETA.EstimatedArrivalAt.Equal(arrival) {
		t.Fatalf("slot milestone %+v", slots.milestone)
	}
	if _, _, err := source.SlotHistory(ctx, tenant, shipment, shipper, domain.SlotTypeDelivery, nil, nil, 10, 0); err != nil {
		t.Fatal(err)
	}
	if prover.calls != 6 {
		t.Fatalf("participant lookups=%d", prover.calls)
	}
	want := []string{"prove", "tracking", "prove", "locations", "prove", "eta", "prove", "eta-history", "prove", "eta", "slots", "prove", "slot-history"}
	if len(order) != len(want) {
		t.Fatalf("order %v", order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order %v", order)
		}
	}
}

func TestShipperTrackingContextClientFailClosed(t *testing.T) {
	tenant := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	shipment := uuid.MustParse("00000000-0000-0000-0000-0000000000a1")
	shipper := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	if _, err := NewShipperTrackingContextClient("", "token").Prove(context.Background(), tenant, shipment, shipper); !shipperContextUnavailable(err) {
		t.Fatalf("empty url: %v", err)
	}
	if _, err := NewShipperTrackingContextClient("http://shipment.internal", "").Prove(context.Background(), tenant, shipment, shipper); !shipperContextUnavailable(err) {
		t.Fatalf("empty token: %v", err)
	}

	var gotTenant, gotToken, gotCompany string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotToken = r.Header.Get("X-Internal-Service-Token")
		gotCompany = r.URL.Query().Get("shipper_company_id")
		switch r.URL.Path {
		case "/internal/v1/shipments/" + shipment.String() + "/shipper-tracking-context":
			if gotCompany == "mismatch" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"shipmentId": shipment.String(), "tenantId": tenant.String(), "shipperCompanyId": shipper.String(),
				"status": "in_transit", "plannedDeliveryAt": "2026-10-12T18:00:00Z",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := NewShipperTrackingContextClient(server.URL, "internal-token")
	facts, err := client.Prove(context.Background(), tenant, shipment, shipper)
	if err != nil || facts.Status != "in_transit" || facts.PlannedDeliveryAt == nil {
		t.Fatalf("facts=%+v err=%v", facts, err)
	}
	if gotTenant != tenant.String() || gotToken != "internal-token" || gotCompany != shipper.String() {
		t.Fatalf("tenant=%s token=%s company=%s", gotTenant, gotToken, gotCompany)
	}
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer foreign.Close()
	if _, err := NewShipperTrackingContextClient(foreign.URL, "internal-token").Prove(context.Background(), tenant, shipment, shipper); !shipperContextNotFound(err) {
		t.Fatalf("404: %v", err)
	}
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer down.Close()
	if _, err := NewShipperTrackingContextClient(down.URL, "internal-token").Prove(context.Background(), tenant, shipment, shipper); !shipperContextUnavailable(err) {
		t.Fatalf("401 dependency: %v", err)
	}
	mismatch := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"shipmentId": uuid.NewString(), "tenantId": tenant.String(), "shipperCompanyId": shipper.String()})
	}))
	defer mismatch.Close()
	if _, err := NewShipperTrackingContextClient(mismatch.URL, "internal-token").Prove(context.Background(), tenant, shipment, shipper); !shipperContextNotFound(err) {
		t.Fatalf("id mismatch: %v", err)
	}
}

func shipperContextUnavailable(err error) bool {
	app, ok := err.(*apperrors.AppError)
	return ok && app.Code == apperrors.CodeUnavailable
}

func shipperContextNotFound(err error) bool {
	app, ok := err.(*apperrors.AppError)
	return ok && app.Code == apperrors.CodeNotFound
}

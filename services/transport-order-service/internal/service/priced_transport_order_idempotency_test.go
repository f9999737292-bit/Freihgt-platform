package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/transport-order-service/internal/domain"
)

type pricedRefStub struct{}

func (pricedRefStub) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (pricedRefStub) Create(context.Context, domain.CreateTransportOrderInput) (*domain.TransportOrder, error) {
	return nil, nil
}
func (pricedRefStub) GetByIDAndTenant(context.Context, uuid.UUID, uuid.UUID) (*domain.TransportOrder, error) {
	return nil, nil
}
func (pricedRefStub) GetCarrierCompanyID(context.Context, uuid.UUID, uuid.UUID) (*uuid.UUID, error) {
	return nil, nil
}
func (pricedRefStub) List(context.Context, domain.ListTransportOrdersFilter) ([]domain.TransportOrder, int, error) {
	return nil, 0, nil
}
func (pricedRefStub) UpdateByIDAndTenant(context.Context, uuid.UUID, uuid.UUID, domain.UpdateTransportOrderInput) (*domain.TransportOrder, error) {
	return nil, nil
}
func (pricedRefStub) UpdateStatusByIDAndTenant(context.Context, uuid.UUID, uuid.UUID, string, string) (*domain.TransportOrder, error) {
	return nil, nil
}

type cargoRefStub struct{}

func (cargoRefStub) Create(context.Context, domain.CreateCargoInput) (*domain.Cargo, error) {
	return nil, nil
}
func (cargoRefStub) GetByIDAndTenant(context.Context, uuid.UUID, uuid.UUID) (*domain.Cargo, error) {
	return nil, nil
}
func (cargoRefStub) ExistsInTenant(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

type locationRefStub struct{}

func (locationRefStub) ExistsInTenant(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

type idempotentPricedStore struct {
	record  *domain.CreateIdempotencyRecord
	result  *domain.PricedTransportOrderResult
	creates int
	tenants []uuid.UUID
}

func (s *idempotentPricedStore) FindCreateIdempotency(context.Context, uuid.UUID, uuid.UUID, string) (*domain.CreateIdempotencyRecord, error) {
	return s.record, nil
}

func (s *idempotentPricedStore) GetPricedResult(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.PricedTransportOrderResult, error) {
	return s.result, nil
}

func (s *idempotentPricedStore) CreatePricedOrder(_ context.Context, in domain.CreatePricedTransportOrderInput, _ domain.RateSnapshot, requestHash string) (*domain.PricedTransportOrderResult, error) {
	s.creates++
	orderID := uuid.New()
	snapshotID := uuid.New()
	s.record = &domain.CreateIdempotencyRecord{
		RequestHash:      requestHash,
		TransportOrderID: orderID,
		RateSnapshotID:   snapshotID,
	}
	s.result = &domain.PricedTransportOrderResult{
		Order:          &domain.TransportOrder{ID: orderID, TenantID: in.TenantID},
		RateSnapshotID: snapshotID,
	}
	return s.result, nil
}

func (s *idempotentPricedStore) WithCreateIdempotencyLock(ctx context.Context, tenantID, _ uuid.UUID, _ string, fn func(context.Context) (*domain.PricedTransportOrderResult, error)) (*domain.PricedTransportOrderResult, error) {
	s.tenants = append(s.tenants, tenantID)
	return fn(ctx)
}

type fixedRateResolver struct{}

func (fixedRateResolver) Resolve(context.Context, domain.CreatePricedTransportOrderInput, time.Time) (domain.ResolveRateResult, error) {
	total := "100.00"
	currency := "RUB"
	return domain.ResolveRateResult{
		PricingSource: "CONTRACT",
		TotalAmount:   &total,
		CurrencyCode:  &currency,
		PricingDate:   "2026-10-24",
		ResolvedAt:    time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC),
	}, nil
}

func TestPricedCreateSameKeyReturnsOneOrder(t *testing.T) {
	t.Parallel()
	store := &idempotentPricedStore{}
	svc := NewPricedTransportOrderService(pricedRefStub{}, cargoRefStub{}, locationRefStub{}, store, fixedRateResolver{}, nil)
	equipment := "TILT"
	tenantID := uuid.New()
	companyID := uuid.New()
	refID := uuid.New()
	input := domain.CreatePricedTransportOrderInput{
		CreateTransportOrderInput: domain.CreateTransportOrderInput{
			TenantID:              tenantID,
			OrderNumber:           "TO-IDEM",
			ShipperCompanyID:      companyID,
			ConsigneeCompanyID:    companyID,
			OriginLocationID:      refID,
			DestinationLocationID: refID,
			CargoID:               refID,
			TransportMode:         domain.TransportModeRoad,
			EquipmentType:         &equipment,
		},
		Actor: domain.InternalActor{
			TenantID:  tenantID,
			UserID:    uuid.New(),
			CompanyID: companyID,
			ActorKind: "BUYER",
		},
		PricingContext: domain.PricingContext{CarrierCompanyID: uuid.New()},
		IdempotencyKey: "same-key",
	}

	first, err := svc.CreatePricedTransportOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := svc.CreatePricedTransportOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first.Order.ID != second.Order.ID || store.creates != 1 {
		t.Fatalf("creates=%d first=%s second=%s", store.creates, first.Order.ID, second.Order.ID)
	}
	if len(store.tenants) != 2 || store.tenants[0] != tenantID || store.tenants[1] != tenantID {
		t.Fatalf("lock tenants=%v", store.tenants)
	}
}

type actorKindResolver struct {
	kind string
}

func (r *actorKindResolver) Resolve(_ context.Context, in domain.CreatePricedTransportOrderInput, _ time.Time) (domain.ResolveRateResult, error) {
	r.kind = in.Actor.ActorKind
	return domain.ResolveRateResult{}, context.Canceled
}

func TestCreateFromAwardScopeForwardsBuyerActor(t *testing.T) {
	t.Parallel()
	resolver := &actorKindResolver{}
	svc := NewPricedTransportOrderService(pricedRefStub{}, cargoRefStub{}, locationRefStub{}, &idempotentPricedStore{}, resolver, nil)
	tenantID := uuid.New()
	buyerID := uuid.New()
	userID := uuid.New()
	refID := uuid.New()
	_, err := svc.CreateFromAwardScope(context.Background(), domain.CreateFromAwardScopeInput{
		TenantID:              tenantID,
		ActorUserID:           userID,
		ActorCompanyID:        buyerID,
		IdempotencyKey:        "award-conv-buyer",
		RfxEventID:            uuid.New(),
		OrderNumber:           "TO-AWD",
		ShipperCompanyID:      buyerID,
		ConsigneeCompanyID:    buyerID,
		OriginLocationID:      refID,
		DestinationLocationID: refID,
		CargoID:               refID,
		TransportMode:         domain.TransportModeRoad,
		EquipmentType:         "TAUTLINER",
		CarrierCompanyID:      uuid.New(),
		CreatedBy:             &userID,
	})
	if err == nil {
		t.Fatal("expected rate resolution to stop the create")
	}
	if resolver.kind != domain.ActorKindBuyer {
		t.Fatalf("actor kind=%q", resolver.kind)
	}
}

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type mockFreightRequestStore struct {
	getTransportOrderFn func(ctx context.Context, id, tenantID uuid.UUID) (string, error)
	createFn            func(ctx context.Context, in domain.CreateFreightRequestFromOrderInput) (*domain.FreightRequest, error)
}

func (m *mockFreightRequestStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (m *mockFreightRequestStore) GetTransportOrder(ctx context.Context, id, tenantID uuid.UUID) (string, error) {
	return m.getTransportOrderFn(ctx, id, tenantID)
}
func (m *mockFreightRequestStore) CreateFromTransportOrder(ctx context.Context, in domain.CreateFreightRequestFromOrderInput) (*domain.FreightRequest, error) {
	return m.createFn(ctx, in)
}
func (m *mockFreightRequestStore) GetByID(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
	return nil, nil
}
func (m *mockFreightRequestStore) List(context.Context, domain.ListFreightRequestsFilter) ([]domain.FreightRequest, int, error) {
	return nil, 0, nil
}
func (m *mockFreightRequestStore) UpdateStatus(context.Context, uuid.UUID, uuid.UUID, string, string) (*domain.FreightRequest, error) {
	return nil, nil
}

func TestFreightRequestServiceCreateFromTransportOrder(t *testing.T) {
	t.Parallel()
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orderID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	shipperID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	userID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	svc := NewFreightRequestServiceWithAuth(&mockFreightRequestStore{
		getTransportOrderFn: func(context.Context, uuid.UUID, uuid.UUID) (string, error) {
			return domain.TransportOrderStatusReadyForSourcing, nil
		},
		createFn: func(_ context.Context, in domain.CreateFreightRequestFromOrderInput) (*domain.FreightRequest, error) {
			return &domain.FreightRequest{
				ID: in.TenantID, FreightRequestNumber: in.FreightRequestNumber, Status: domain.FreightRequestStatusDraft,
			}, nil
		},
	}, &mockMembershipResolver{buyerIDs: []uuid.UUID{shipperID}})
	fr, err := svc.CreateFromTransportOrder(context.Background(), buyerTestActor(tenantID, userID, shipperID), domain.CreateFreightRequestFromOrderInput{
		TenantID: tenantID, TransportOrderID: orderID, FreightRequestNumber: "FR-1",
		RequestType: "MINI_TENDER", ShipperCompanyID: shipperID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fr.Status != domain.FreightRequestStatusDraft {
		t.Fatalf("unexpected status: %s", fr.Status)
	}
}

type mockBidStore struct {
	createFn               func(ctx context.Context, in domain.CreateBidInput) (*domain.Bid, error)
	getByIDFn              func(ctx context.Context, id, tenantID uuid.UUID) (*domain.Bid, error)
	listByFreightRequestFn func(ctx context.Context, freightRequestID, tenantID uuid.UUID) ([]domain.Bid, error)
}

func (m *mockBidStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (m *mockBidStore) CreateBid(ctx context.Context, in domain.CreateBidInput) (*domain.Bid, error) {
	return m.createFn(ctx, in)
}
func (m *mockBidStore) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.Bid, error) {
	return m.getByIDFn(ctx, id, tenantID)
}
func (m *mockBidStore) ListByFreightRequest(ctx context.Context, freightRequestID, tenantID uuid.UUID) ([]domain.Bid, error) {
	if m.listByFreightRequestFn != nil {
		return m.listByFreightRequestFn(ctx, freightRequestID, tenantID)
	}
	return nil, nil
}
func (m *mockBidStore) SubmitBid(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID) (*domain.Bid, error) {
	return nil, nil
}
func (m *mockBidStore) AcceptBid(context.Context, uuid.UUID, uuid.UUID, func(context.Context, pgx.Tx) error) (*domain.Bid, error) {
	return nil, nil
}

type mockFreightRequestStoreForBid struct {
	getByIDFn func(ctx context.Context, id, tenantID uuid.UUID) (*domain.FreightRequest, error)
}

func (m *mockFreightRequestStoreForBid) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (m *mockFreightRequestStoreForBid) GetTransportOrder(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return "", nil
}
func (m *mockFreightRequestStoreForBid) CreateFromTransportOrder(context.Context, domain.CreateFreightRequestFromOrderInput) (*domain.FreightRequest, error) {
	return nil, nil
}
func (m *mockFreightRequestStoreForBid) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.FreightRequest, error) {
	return m.getByIDFn(ctx, id, tenantID)
}
func (m *mockFreightRequestStoreForBid) List(context.Context, domain.ListFreightRequestsFilter) ([]domain.FreightRequest, int, error) {
	return nil, 0, nil
}
func (m *mockFreightRequestStoreForBid) UpdateStatus(context.Context, uuid.UUID, uuid.UUID, string, string) (*domain.FreightRequest, error) {
	return nil, nil
}

func TestBidServiceDuplicateBidConflict(t *testing.T) {
	t.Parallel()
	carrierID := uuid.New()
	svc := NewBidService(&mockBidStore{
		createFn: func(context.Context, domain.CreateBidInput) (*domain.Bid, error) {
			return nil, apperrors.Conflict("record already exists", map[string]any{"detail": "uq_bid_carrier_request"})
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return &domain.FreightRequest{Status: domain.FreightRequestStatusPublished}, nil
		},
	}, &mockActorResolver{kind: domain.ActorKindCarrier, carrierIDs: []uuid.UUID{carrierID}}, nil)
	_, err := svc.CreateBid(context.Background(), domain.ActorContext{TenantID: uuid.New()}, uuid.New(), domain.CreateBidInput{
		TenantID: uuid.New(), CarrierCompanyID: carrierID, BidNumber: "BID-1",
		Items: []domain.CreateBidItemInput{{BaseAmount: 100}},
	})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestBidServiceSubmitOnlyFromDraft(t *testing.T) {
	t.Parallel()
	svc := NewBidService(&mockBidStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
			return &domain.Bid{Status: domain.BidStatusSubmitted, FreightRequestID: uuid.New()}, nil
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return &domain.FreightRequest{Status: domain.FreightRequestStatusPublished}, nil
		},
	}, nil, nil)
	_, err := svc.SubmitBid(context.Background(), domain.ActorContext{TenantID: uuid.New()}, uuid.New())
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestBidServiceAcceptOnlyFromSubmitted(t *testing.T) {
	t.Parallel()
	svc := NewBidService(&mockBidStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
			return &domain.Bid{Status: domain.BidStatusDraft}, nil
		},
	}, &mockFreightRequestStoreForBid{}, nil, nil)
	_, err := svc.AcceptBid(context.Background(), domain.ActorContext{TenantID: uuid.New()}, uuid.New())
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestBidServiceGetByIDPassesTenantToRepository(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	bidID := uuid.New()
	carrierID := uuid.New()
	svc := NewBidService(&mockBidStore{
		getByIDFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Bid, error) {
			if id != bidID || tenant != tenantID {
				t.Fatalf("unexpected scoped lookup id=%s tenant=%s", id, tenant)
			}
			return &domain.Bid{ID: id, TenantID: tenant, BidNumber: "BID-1", CarrierCompanyID: carrierID}, nil
		},
	}, &mockFreightRequestStoreForBid{}, &mockActorResolver{
		kind: domain.ActorKindCarrier, carrierIDs: []uuid.UUID{carrierID},
	}, nil)
	bid, err := svc.GetByID(context.Background(), domain.ActorContext{TenantID: tenantID}, bidID)
	if err != nil || bid.BidNumber != "BID-1" {
		t.Fatalf("unexpected result bid=%v err=%v", bid, err)
	}
}

func TestBidServiceGetByIDNotFound(t *testing.T) {
	t.Parallel()
	svc := NewBidService(&mockBidStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
			return nil, apperrors.NotFound("bid not found")
		},
	}, &mockFreightRequestStoreForBid{}, nil, nil)
	_, err := svc.GetByID(context.Background(), domain.ActorContext{TenantID: uuid.New()}, uuid.New())
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestBidServiceGetByIDForeignSameAsNotFound(t *testing.T) {
	t.Parallel()
	svc := NewBidService(&mockBidStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
			return nil, apperrors.NotFound("bid not found")
		},
	}, &mockFreightRequestStoreForBid{}, nil, nil)
	_, err := svc.GetByID(context.Background(), domain.ActorContext{TenantID: uuid.New()}, uuid.New())
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("foreign tenant must surface as not found, got %v", err)
	}
}

func TestBidServiceGetByIDMissingTenantSkipsRepository(t *testing.T) {
	t.Parallel()
	called := false
	svc := NewBidService(&mockBidStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
			called = true
			return nil, nil
		},
	}, &mockFreightRequestStoreForBid{}, nil, nil)
	_, err := svc.GetByID(context.Background(), domain.ActorContext{}, uuid.New())
	if called {
		t.Fatal("repository must not be called")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

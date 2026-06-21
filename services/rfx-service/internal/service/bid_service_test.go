package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

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
	svc := NewFreightRequestService(&mockFreightRequestStore{
		getTransportOrderFn: func(context.Context, uuid.UUID, uuid.UUID) (string, error) {
			return domain.TransportOrderStatusReadyForSourcing, nil
		},
		createFn: func(_ context.Context, in domain.CreateFreightRequestFromOrderInput) (*domain.FreightRequest, error) {
			return &domain.FreightRequest{
				ID: in.TenantID, FreightRequestNumber: in.FreightRequestNumber, Status: domain.FreightRequestStatusDraft,
			}, nil
		},
	})
	fr, err := svc.CreateFromTransportOrder(context.Background(), domain.CreateFreightRequestFromOrderInput{
		TenantID: tenantID, TransportOrderID: orderID, FreightRequestNumber: "FR-1",
		RequestType: "MINI_TENDER", ShipperCompanyID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fr.Status != domain.FreightRequestStatusDraft {
		t.Fatalf("unexpected status: %s", fr.Status)
	}
}

type mockBidStore struct {
	createFn func(ctx context.Context, in domain.CreateBidInput) (*domain.Bid, error)
	getByIDFn func(ctx context.Context, id, tenantID uuid.UUID) (*domain.Bid, error)
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
func (m *mockBidStore) ListByFreightRequest(context.Context, uuid.UUID, uuid.UUID) ([]domain.Bid, error) {
	return nil, nil
}
func (m *mockBidStore) SubmitBid(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID) (*domain.Bid, error) {
	return nil, nil
}
func (m *mockBidStore) AcceptBid(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
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
	svc := NewBidService(&mockBidStore{
		createFn: func(context.Context, domain.CreateBidInput) (*domain.Bid, error) {
			return nil, apperrors.Conflict("record already exists", map[string]any{"detail": "uq_bid_carrier_request"})
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return &domain.FreightRequest{Status: domain.FreightRequestStatusPublished}, nil
		},
	})
	_, err := svc.CreateBid(context.Background(), uuid.New(), domain.CreateBidInput{
		TenantID: uuid.New(), CarrierCompanyID: uuid.New(), BidNumber: "BID-1",
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
			return &domain.Bid{Status: domain.BidStatusSubmitted}, nil
		},
	}, &mockFreightRequestStoreForBid{})
	_, err := svc.SubmitBid(context.Background(), uuid.New(), uuid.New())
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
	}, &mockFreightRequestStoreForBid{})
	_, err := svc.AcceptBid(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

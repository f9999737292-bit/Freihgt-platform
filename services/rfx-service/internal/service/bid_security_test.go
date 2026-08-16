package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type mockActorResolver struct {
	kind       domain.ActorKind
	carrierIDs []uuid.UUID
}

func (m *mockActorResolver) ResolveActorKind(context.Context, domain.ActorContext) (domain.ActorKind, []uuid.UUID, error) {
	return m.kind, m.carrierIDs, nil
}

func TestBidServiceCarrierCannotViewCompetitorBid(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	carrierA := uuid.New()
	carrierB := uuid.New()
	bidID := uuid.New()

	svc := NewBidService(&mockBidStore{
		getByIDFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Bid, error) {
			return &domain.Bid{ID: id, TenantID: tenant, CarrierCompanyID: carrierB, BidNumber: "BID-1"}, nil
		},
	}, &mockFreightRequestStoreForBid{}, &mockActorResolver{
		kind: domain.ActorKindCarrier, carrierIDs: []uuid.UUID{carrierA},
	}, nil)

	_, err := svc.GetByID(context.Background(), domain.ActorContext{TenantID: tenantID, UserID: uuid.New()}, bidID)
	if err == nil {
		t.Fatal("expected not found for competitor bid")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestBidServiceListBidsFiltersCompetitorBids(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	frID := uuid.New()
	carrierA := uuid.New()
	carrierB := uuid.New()

	svc := NewBidService(&mockBidStore{
		listByFreightRequestFn: func(context.Context, uuid.UUID, uuid.UUID) ([]domain.Bid, error) {
			return []domain.Bid{
				{CarrierCompanyID: carrierA, BidNumber: "A"},
				{CarrierCompanyID: carrierB, BidNumber: "B"},
			}, nil
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return &domain.FreightRequest{Status: domain.FreightRequestStatusPublished}, nil
		},
	}, &mockActorResolver{
		kind: domain.ActorKindCarrier, carrierIDs: []uuid.UUID{carrierA},
	}, nil)

	bids, err := svc.ListBids(context.Background(), domain.ActorContext{TenantID: tenantID}, frID, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bids) != 1 || bids[0].BidNumber != "A" {
		t.Fatalf("expected only own bid, got %+v", bids)
	}
}

func TestBidServiceCreateBidUsesMembershipCarrier(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	carrierID := uuid.New()
	frID := uuid.New()
	deadline := time.Now().UTC().Add(time.Hour)

	svc := NewBidService(&mockBidStore{
		createFn: func(_ context.Context, in domain.CreateBidInput) (*domain.Bid, error) {
			if in.CarrierCompanyID != carrierID {
				t.Fatalf("expected membership carrier %s, got %s", carrierID, in.CarrierCompanyID)
			}
			return &domain.Bid{ID: uuid.New(), CarrierCompanyID: carrierID}, nil
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return &domain.FreightRequest{Status: domain.FreightRequestStatusPublished, ResponseDeadline: &deadline}, nil
		},
	}, &mockActorResolver{
		kind: domain.ActorKindCarrier, carrierIDs: []uuid.UUID{carrierID},
	}, nil)

	_, err := svc.CreateBid(context.Background(), domain.ActorContext{TenantID: tenantID, UserID: uuid.New()}, frID, domain.CreateBidInput{
		TenantID: tenantID, BidNumber: "BID-1", Items: []domain.CreateBidItemInput{{BaseAmount: 100}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

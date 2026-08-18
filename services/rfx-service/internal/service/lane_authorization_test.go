package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func laneTestInput(tenantID, lotID uuid.UUID) domain.CreateRfxLaneInput {
	origin := uuid.New()
	destination := uuid.New()
	return domain.CreateRfxLaneInput{
		TenantID:              tenantID,
		RfxLotID:              lotID,
		OriginLocationID:      origin,
		DestinationLocationID: destination,
		TransportMode:         "ROAD",
	}
}

func TestCreateLaneOwnLotAllowed(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	lotID := uuid.New()
	ownerCompanyID := uuid.New()
	svc := NewRfxService(&mockRfxStore{
		getLotOwnerContextFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.LotOwnerContext, error) {
			if id != lotID || tenant != tenantID {
				t.Fatalf("unexpected lookup lot=%s tenant=%s", id, tenant)
			}
			return &domain.LotOwnerContext{
				LotID: lotID, TenantID: tenantID, RfxEventID: uuid.New(), OwnerCompanyID: ownerCompanyID,
			}, nil
		},
		createLaneFn: func(_ context.Context, in domain.CreateRfxLaneInput) (*domain.RfxLane, error) {
			if in.RfxLotID != lotID {
				t.Fatalf("expected lot %s, got %s", lotID, in.RfxLotID)
			}
			return &domain.RfxLane{ID: uuid.New(), TenantID: tenantID, RfxLotID: lotID}, nil
		},
	}, nil, buyerMembershipResolver(ownerCompanyID))

	_, err := svc.CreateLane(context.Background(), buyerTestActor(tenantID, uuid.New(), ownerCompanyID), lotID, laneTestInput(tenantID, lotID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateLaneCrossCompanyDenied(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	lotID := uuid.New()
	ownerA := uuid.New()
	ownerB := uuid.New()
	svc := NewRfxService(&mockRfxStore{
		getLotOwnerContextFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.LotOwnerContext, error) {
			return &domain.LotOwnerContext{
				LotID: lotID, TenantID: tenantID, RfxEventID: uuid.New(), OwnerCompanyID: ownerA,
			}, nil
		},
		createLaneFn: func(context.Context, domain.CreateRfxLaneInput) (*domain.RfxLane, error) {
			t.Fatal("create lane should not be called")
			return nil, nil
		},
	}, nil, buyerMembershipResolver(ownerB))

	_, err := svc.CreateLane(context.Background(), buyerTestActor(tenantID, uuid.New(), ownerB), lotID, laneTestInput(tenantID, lotID))
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestCreateLaneCrossTenantDenied(t *testing.T) {
	t.Parallel()
	tenantA := uuid.New()
	lotID := uuid.New()
	ownerCompanyID := uuid.New()
	svc := NewRfxService(&mockRfxStore{
		getLotOwnerContextFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.LotOwnerContext, error) {
			if tenant != tenantA {
				return nil, apperrors.NotFound("rfx lot not found")
			}
			// Lot exists only in another tenant; lookup scoped to tenantA yields no row.
			return nil, apperrors.NotFound("rfx lot not found")
		},
		createLaneFn: func(context.Context, domain.CreateRfxLaneInput) (*domain.RfxLane, error) {
			t.Fatal("create lane should not be called")
			return nil, nil
		},
	}, nil, buyerMembershipResolver(ownerCompanyID))

	_, err := svc.CreateLane(context.Background(), buyerTestActor(tenantA, uuid.New(), ownerCompanyID), lotID, laneTestInput(tenantA, lotID))
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestCreateLaneUnknownLotNotFound(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	ownerCompanyID := uuid.New()
	svc := NewRfxService(&mockRfxStore{
		getLotOwnerContextFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.LotOwnerContext, error) {
			return nil, apperrors.NotFound("rfx lot not found")
		},
	}, nil, buyerMembershipResolver(ownerCompanyID))

	_, err := svc.CreateLane(context.Background(), buyerTestActor(tenantID, uuid.New(), ownerCompanyID), uuid.New(), laneTestInput(tenantID, uuid.New()))
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

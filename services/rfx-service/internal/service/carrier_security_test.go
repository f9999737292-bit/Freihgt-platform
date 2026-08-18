package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestGetEventCarrierDeniedWithoutParticipant(t *testing.T) {
	tenantID := uuid.New()
	eventID := uuid.New()
	carrierA := uuid.New()
	carrierB := uuid.New()

	svc := NewRfxService(&mockRfxStore{
		getEventFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
			return &domain.RfxEvent{ID: eventID, TenantID: tenantID, OwnerCompanyID: uuid.New(), Status: domain.RfxStatusPublished}, nil
		},
		participantExistsFn: func(_ context.Context, _ uuid.UUID, companyID, _ uuid.UUID) (bool, error) {
			return companyID == carrierB, nil
		},
	}, nil, &mockActorResolver{
		kind: domain.ActorKindCarrier, carrierIDs: []uuid.UUID{carrierA},
	})

	_, err := svc.GetEvent(context.Background(), domain.ActorContext{TenantID: tenantID, UserID: uuid.New()}, eventID)
	if err == nil {
		t.Fatal("expected not found for non-participant carrier")
	}
	if appErr, ok := err.(*apperrors.AppError); !ok || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestGetResponseCarrierCannotViewCompetitor(t *testing.T) {
	tenantID := uuid.New()
	responseID := uuid.New()
	carrierA := uuid.New()
	carrierB := uuid.New()

	svc := NewRfxService(&mockRfxStore{
		getResponseFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxResponse, error) {
			return &domain.RfxResponse{
				ID: responseID, TenantID: tenantID, ParticipantCompanyID: carrierB, Status: domain.RfxResponseStatusSubmitted,
			}, nil
		},
	}, nil, &mockActorResolver{
		kind: domain.ActorKindCarrier, carrierIDs: []uuid.UUID{carrierA},
	})

	_, err := svc.GetResponse(context.Background(), domain.ActorContext{TenantID: tenantID, UserID: uuid.New()}, responseID)
	if err == nil {
		t.Fatal("expected not found for competitor response")
	}
	if appErr, ok := err.(*apperrors.AppError); !ok || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestListParticipantsDeniedForCarrier(t *testing.T) {
	tenantID := uuid.New()
	eventID := uuid.New()
	svc := NewRfxService(&mockRfxStore{
		getEventFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
			return &domain.RfxEvent{ID: eventID, TenantID: tenantID, OwnerCompanyID: uuid.New()}, nil
		},
	}, nil, &mockActorResolver{
		kind: domain.ActorKindCarrier, carrierIDs: []uuid.UUID{uuid.New()},
	})

	_, err := svc.ListParticipants(context.Background(), domain.ActorContext{TenantID: tenantID, UserID: uuid.New()}, eventID, nil)
	if err == nil {
		t.Fatal("expected forbidden for carrier listing participants")
	}
	if appErr, ok := err.(*apperrors.AppError); !ok || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

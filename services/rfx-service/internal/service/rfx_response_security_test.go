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

func TestRfxServiceCreateResponseRejectsForeignParticipantCompany(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	eventID := uuid.New()
	carrierA := uuid.New()
	carrierB := uuid.New()

	svc := NewRfxService(&mockRfxStore{
		getEventFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
			return &domain.RfxEvent{
				Status:           domain.RfxStatusPublished,
				ResponseDeadline: ptrTime(time.Now().UTC().Add(time.Hour)),
			}, nil
		},
		participantExistsFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (bool, error) {
			return true, nil
		},
	}, nil, &mockActorResolver{
		kind: domain.ActorKindCarrier, carrierIDs: []uuid.UUID{carrierA},
	})

	_, err := svc.CreateResponse(context.Background(), domain.ActorContext{TenantID: tenantID, UserID: uuid.New()}, eventID, domain.CreateRfxResponseInput{
		TenantID:             tenantID,
		ParticipantCompanyID: carrierB,
	})
	if err == nil {
		t.Fatal("expected forbidden for foreign participant company")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestRfxServiceSubmitResponseRejectsForeignResponse(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	responseID := uuid.New()
	carrierA := uuid.New()
	carrierB := uuid.New()
	deadline := time.Now().UTC().Add(time.Hour)

	svc := NewRfxService(&mockRfxStore{
		getResponseFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxResponse, error) {
			return &domain.RfxResponse{
				ID:                   responseID,
				TenantID:             tenantID,
				RfxEventID:           uuid.New(),
				ParticipantCompanyID: carrierB,
				Status:               domain.RfxResponseStatusDraft,
			}, nil
		},
		getEventFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
			return &domain.RfxEvent{ResponseDeadline: &deadline}, nil
		},
	}, nil, &mockActorResolver{
		kind: domain.ActorKindCarrier, carrierIDs: []uuid.UUID{carrierA},
	})

	_, err := svc.SubmitResponse(context.Background(), domain.ActorContext{TenantID: tenantID, UserID: uuid.New()}, responseID)
	if err == nil {
		t.Fatal("expected not found for foreign response")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}

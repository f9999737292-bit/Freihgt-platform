package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestGetCarrierInvitedEventAllowsInvitedCarrier(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	userID := uuid.New()
	carrierID := uuid.New()
	eventID := uuid.New()
	ownResponseID := uuid.New()
	svc := NewRfxService(&mockRfxStore{
		getCarrierInvitedEventFn: func(_ context.Context, gotEvent, gotCompany, gotTenant uuid.UUID) (*domain.CarrierInvitedRfxEvent, error) {
			if gotEvent != eventID || gotCompany != carrierID || gotTenant != tenantID {
				t.Fatalf("unexpected lookup event=%s company=%s tenant=%s", gotEvent, gotCompany, gotTenant)
			}
			return &domain.CarrierInvitedRfxEvent{
				Event:                domain.RfxEvent{ID: eventID, TenantID: tenantID, Title: "Invited"},
				ParticipantStatus:    "INVITED",
				OwnResponseStatus:    domain.CarrierOwnResponseNotStarted,
				OwnResponseID:        &ownResponseID,
				ParticipantCompanyID: carrierID,
				LotCount:             1,
			}, nil
		},
	}, nil, carrierMembershipResolver(carrierID))

	got, err := svc.GetCarrierInvitedEvent(context.Background(), carrierTestActor(tenantID, userID), eventID, carrierID)
	if err != nil {
		t.Fatalf("expected invited carrier to read event: %v", err)
	}
	if got.Event.ID != eventID || got.ParticipantCompanyID != carrierID {
		t.Fatalf("unexpected invited event %+v", got)
	}
}

func TestGetCarrierInvitedEventDeniesForeignCompany(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	userID := uuid.New()
	ownCompany := uuid.New()
	foreignCompany := uuid.New()
	svc := NewRfxService(&mockRfxStore{
		getCarrierInvitedEventFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.CarrierInvitedRfxEvent, error) {
			t.Fatal("repo must not be queried for a foreign company")
			return nil, nil
		},
	}, nil, carrierMembershipResolver(ownCompany))

	_, err := svc.GetCarrierInvitedEvent(context.Background(), carrierTestActor(tenantID, userID), uuid.New(), foreignCompany)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("expected forbidden for foreign company, got %v", err)
	}
}

func TestGetCarrierInvitedEventHidesUninvitedEvent(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	userID := uuid.New()
	carrierID := uuid.New()
	svc := NewRfxService(&mockRfxStore{}, nil, carrierMembershipResolver(carrierID))

	_, err := svc.GetCarrierInvitedEvent(context.Background(), carrierTestActor(tenantID, userID), uuid.New(), carrierID)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found for uninvited event, got %v", err)
	}
}

func TestGetCarrierInvitedEventDeniesBuyer(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	userID := uuid.New()
	owner := uuid.New()
	svc := NewRfxService(&mockRfxStore{
		getCarrierInvitedEventFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.CarrierInvitedRfxEvent, error) {
			t.Fatal("repo must not be queried for a buyer actor")
			return nil, nil
		},
	}, nil, buyerMembershipResolver(owner))

	_, err := svc.GetCarrierInvitedEvent(context.Background(), buyerTestActor(tenantID, userID, owner), uuid.New(), uuid.Nil)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("expected forbidden for buyer actor, got %v", err)
	}
}

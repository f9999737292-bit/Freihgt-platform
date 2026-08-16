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

func TestBuyerCreateForOwnCompanyAllowed(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	userID := uuid.New()
	ownerCompanyID := uuid.New()
	svc := NewRfxService(&mockRfxStore{
		createEventFn: func(_ context.Context, in domain.CreateRfxEventInput) (*domain.RfxEvent, error) {
			if in.OwnerCompanyID != ownerCompanyID {
				t.Fatalf("expected owner %s, got %s", ownerCompanyID, in.OwnerCompanyID)
			}
			return &domain.RfxEvent{ID: uuid.New(), OwnerCompanyID: ownerCompanyID}, nil
		},
	}, nil, buyerMembershipResolver(ownerCompanyID))
	_, err := svc.CreateEvent(context.Background(), buyerTestActor(tenantID, userID, ownerCompanyID), domain.CreateRfxEventInput{
		TenantID: tenantID, OwnerCompanyID: ownerCompanyID, Title: "T", RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuyerCreateForOtherSameTenantCompanyDenied(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	userID := uuid.New()
	ownerA := uuid.New()
	ownerB := uuid.New()
	svc := NewRfxService(&mockRfxStore{}, nil, buyerMembershipResolver(ownerA))
	_, err := svc.CreateEvent(context.Background(), buyerTestActor(tenantID, userID, ownerA), domain.CreateRfxEventInput{
		TenantID: tenantID, OwnerCompanyID: ownerB, Title: "T", RfxType: "SPOT_RFQ", Category: "FREIGHT",
	})
	assertAppErrorCode(t, err, apperrors.CodeForbidden)
}

func TestSameTenantWrongCompanyMutationDenied(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	ownerA := uuid.New()
	ownerB := uuid.New()
	buyerB := buyerTestActor(tenantID, uuid.New(), ownerB)
	event := &domain.RfxEvent{
		ID: uuid.New(), TenantID: tenantID, Status: domain.RfxStatusDraft, OwnerCompanyID: ownerA,
	}
	store := &mockRfxStore{getEventFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
		return event, nil
	}}
	svc := NewRfxService(store, nil, buyerMembershipResolver(ownerB))

	cases := []struct {
		name string
		run  func() error
	}{
		{"publish", func() error { _, err := svc.PublishEvent(context.Background(), buyerB, event.ID); return err }},
		{"cancel", func() error { _, err := svc.CancelEvent(context.Background(), buyerB, event.ID); return err }},
		{"transition", func() error {
			_, err := svc.TransitionEvent(context.Background(), buyerB, event.ID, domain.RfxCommandOpenResponses)
			return err
		}},
		{"add_participant", func() error {
			_, err := svc.AddParticipant(context.Background(), buyerB, event.ID, domain.AddRfxParticipantInput{
				TenantID: tenantID, CompanyID: uuid.New(), ParticipantType: "CARRIER",
			})
			return err
		}},
		{"extend_deadline", func() error {
			deadline := time.Now().UTC().Add(time.Hour)
			_, err := svc.ExtendDeadline(context.Background(), buyerB, event.ID, deadline)
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertAppErrorCode(t, tc.run(), apperrors.CodeNotFound)
		})
	}
}

func TestRBACWithoutOwnerMembershipDenied(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	ownerA := uuid.New()
	ownerB := uuid.New()
	resolver := &mockMembershipResolver{
		kind:     domain.ActorKindBuyer,
		buyerIDs: []uuid.UUID{ownerB},
		roles:    []string{"PROCUREMENT_MANAGER"},
	}
	svc := NewRfxService(&mockRfxStore{
		getEventFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
			return &domain.RfxEvent{Status: domain.RfxStatusDraft, OwnerCompanyID: ownerA}, nil
		},
	}, nil, resolver)
	_, err := svc.PublishEvent(context.Background(), buyerTestActor(tenantID, uuid.New(), ownerB), uuid.New())
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestCompanyMembershipWithoutRBACDenied(t *testing.T) {
	t.Parallel()
	ownerCompanyID := uuid.New()
	resolver := &mockMembershipResolver{
		kind:     domain.ActorKindBuyer,
		buyerIDs: []uuid.UUID{ownerCompanyID},
		roles:    []string{"CARRIER_DISPATCHER"},
	}
	svc := NewRfxService(&mockRfxStore{
		getEventFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
			return &domain.RfxEvent{Status: domain.RfxStatusDraft, OwnerCompanyID: ownerCompanyID}, nil
		},
	}, nil, resolver)
	_, err := svc.PublishEvent(context.Background(), buyerTestActor(uuid.New(), uuid.New(), ownerCompanyID), uuid.New())
	assertAppErrorCode(t, err, apperrors.CodeForbidden)
}

func TestCrossCompanyAwardDenied(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	shipperA := uuid.New()
	shipperB := uuid.New()
	bidID := uuid.New()
	svc := NewBidService(&mockBidStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
			return &domain.Bid{ID: bidID, Status: domain.BidStatusSubmitted, FreightRequestID: uuid.New()}, nil
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return &domain.FreightRequest{Status: domain.FreightRequestStatusPublished, ShipperCompanyID: shipperA}, nil
		},
	}, buyerMembershipResolver(shipperB), nil)
	_, err := svc.AcceptBid(context.Background(), buyerTestActor(tenantID, uuid.New(), shipperB), bidID)
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func assertAppErrorCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("expected code %s, got %v", code, err)
	}
}

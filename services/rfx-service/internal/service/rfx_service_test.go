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

type mockRfxStore struct {
	createEventFn         func(ctx context.Context, in domain.CreateRfxEventInput) (*domain.RfxEvent, error)
	getEventFn            func(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxEvent, error)
	updateStatusFn        func(ctx context.Context, id, tenantID uuid.UUID, expected, newStatus string) (*domain.RfxEvent, error)
	addParticipantFn      func(ctx context.Context, in domain.AddRfxParticipantInput) (*domain.RfxParticipant, error)
	participantExistsFn   func(ctx context.Context, eventID, companyID, tenantID uuid.UUID) (bool, error)
	getResponseFn         func(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxResponse, error)
	getLotOwnerContextFn  func(ctx context.Context, lotID, tenantID uuid.UUID) (*domain.LotOwnerContext, error)
	createLaneFn          func(ctx context.Context, in domain.CreateRfxLaneInput) (*domain.RfxLane, error)
}

func (m *mockRfxStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (m *mockRfxStore) CreateEvent(ctx context.Context, in domain.CreateRfxEventInput) (*domain.RfxEvent, error) {
	if m.createEventFn != nil {
		return m.createEventFn(ctx, in)
	}
	return nil, nil
}
func (m *mockRfxStore) GetEventByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxEvent, error) {
	return m.getEventFn(ctx, id, tenantID)
}
func (m *mockRfxStore) ListEvents(context.Context, domain.ListRfxEventsFilter) ([]domain.RfxEvent, int, error) {
	return nil, 0, nil
}
func (m *mockRfxStore) UpdateEvent(context.Context, uuid.UUID, uuid.UUID, domain.UpdateRfxEventInput) (*domain.RfxEvent, error) {
	return nil, nil
}
func (m *mockRfxStore) UpdateEventStatus(ctx context.Context, id, tenantID uuid.UUID, expected, newStatus string) (*domain.RfxEvent, error) {
	return m.updateStatusFn(ctx, id, tenantID, expected, newStatus)
}
func (m *mockRfxStore) CreateLot(context.Context, domain.CreateRfxLotInput) (*domain.RfxLot, error) {
	return nil, nil
}
func (m *mockRfxStore) ListLotsByEvent(context.Context, uuid.UUID, uuid.UUID) ([]domain.RfxLot, error) {
	return nil, nil
}
func (m *mockRfxStore) GetLotOwnerContext(ctx context.Context, lotID, tenantID uuid.UUID) (*domain.LotOwnerContext, error) {
	if m.getLotOwnerContextFn != nil {
		return m.getLotOwnerContextFn(ctx, lotID, tenantID)
	}
	return nil, apperrors.NotFound("rfx lot not found")
}
func (m *mockRfxStore) CreateLane(ctx context.Context, in domain.CreateRfxLaneInput) (*domain.RfxLane, error) {
	if m.createLaneFn != nil {
		return m.createLaneFn(ctx, in)
	}
	return nil, nil
}
func (m *mockRfxStore) AddParticipant(ctx context.Context, in domain.AddRfxParticipantInput) (*domain.RfxParticipant, error) {
	return m.addParticipantFn(ctx, in)
}
func (m *mockRfxStore) ListParticipants(context.Context, uuid.UUID, uuid.UUID) ([]domain.RfxParticipant, error) {
	return nil, nil
}
func (m *mockRfxStore) ParticipantExists(ctx context.Context, eventID, companyID, tenantID uuid.UUID) (bool, error) {
	if m.participantExistsFn != nil {
		return m.participantExistsFn(ctx, eventID, companyID, tenantID)
	}
	return true, nil
}
func (m *mockRfxStore) CreateResponse(context.Context, domain.CreateRfxResponseInput) (*domain.RfxResponse, error) {
	return nil, nil
}
func (m *mockRfxStore) GetResponseByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxResponse, error) {
	if m.getResponseFn != nil {
		return m.getResponseFn(ctx, id, tenantID)
	}
	return nil, nil
}
func (m *mockRfxStore) SubmitResponse(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID) (*domain.RfxResponse, error) {
	return nil, nil
}
func (m *mockRfxStore) CountLotsByEvent(context.Context, uuid.UUID, uuid.UUID) (int, error) {
	return 0, nil
}
func (m *mockRfxStore) CloseExpiredResponses(context.Context, uuid.UUID, time.Time) (int, error) {
	return 0, nil
}
func (m *mockRfxStore) ListExpiredResponseOpenEvents(context.Context, time.Time, int) ([]domain.ExpiredResponseOpenEvent, error) {
	return nil, nil
}
func (m *mockRfxStore) UpdateEventResponseDeadline(context.Context, uuid.UUID, uuid.UUID, *time.Time, int) (*domain.RfxEvent, error) {
	return nil, nil
}

func TestRfxServicePublishOnlyFromDraft(t *testing.T) {
	t.Parallel()
	svc := NewRfxService(&mockRfxStore{
		getEventFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
			return &domain.RfxEvent{Status: domain.RfxStatusPublished}, nil
		},
	}, nil, nil)
	_, err := svc.PublishEvent(context.Background(), domain.ActorContext{TenantID: uuid.New()}, uuid.New())
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestRfxServiceAddParticipantDuplicateConflict(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	userID := uuid.New()
	ownerCompanyID := uuid.New()
	eventID := uuid.New()
	svc := NewRfxService(&mockRfxStore{
		getEventFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
			return &domain.RfxEvent{Status: domain.RfxStatusDraft, OwnerCompanyID: ownerCompanyID}, nil
		},
		addParticipantFn: func(context.Context, domain.AddRfxParticipantInput) (*domain.RfxParticipant, error) {
			return nil, apperrors.Conflict("record already exists", map[string]any{"detail": "uq_rfx_participant"})
		},
	}, nil, buyerMembershipResolver(ownerCompanyID))
	_, err := svc.AddParticipant(context.Background(), buyerTestActor(tenantID, userID, ownerCompanyID), eventID, domain.AddRfxParticipantInput{
		TenantID: tenantID, CompanyID: uuid.New(), ParticipantType: "CARRIER",
	})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
}

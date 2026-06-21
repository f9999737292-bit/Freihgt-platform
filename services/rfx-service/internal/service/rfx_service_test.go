package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type mockRfxStore struct {
	getEventFn func(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxEvent, error)
	updateStatusFn func(ctx context.Context, id, tenantID uuid.UUID, expected, newStatus string) (*domain.RfxEvent, error)
	addParticipantFn func(ctx context.Context, in domain.AddRfxParticipantInput) (*domain.RfxParticipant, error)
}

func (m *mockRfxStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (m *mockRfxStore) CreateEvent(context.Context, domain.CreateRfxEventInput) (*domain.RfxEvent, error) {
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
func (m *mockRfxStore) CreateLane(context.Context, domain.CreateRfxLaneInput) (*domain.RfxLane, error) {
	return nil, nil
}
func (m *mockRfxStore) AddParticipant(ctx context.Context, in domain.AddRfxParticipantInput) (*domain.RfxParticipant, error) {
	return m.addParticipantFn(ctx, in)
}
func (m *mockRfxStore) ListParticipants(context.Context, uuid.UUID, uuid.UUID) ([]domain.RfxParticipant, error) {
	return nil, nil
}
func (m *mockRfxStore) ParticipantExists(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (m *mockRfxStore) CreateResponse(context.Context, domain.CreateRfxResponseInput) (*domain.RfxResponse, error) {
	return nil, nil
}
func (m *mockRfxStore) GetResponseByID(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxResponse, error) {
	return nil, nil
}
func (m *mockRfxStore) SubmitResponse(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID) (*domain.RfxResponse, error) {
	return nil, nil
}

func TestRfxServicePublishOnlyFromDraft(t *testing.T) {
	t.Parallel()
	svc := NewRfxService(&mockRfxStore{
		getEventFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
			return &domain.RfxEvent{Status: domain.RfxStatusPublished}, nil
		},
	})
	_, err := svc.PublishEvent(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestRfxServiceAddParticipantDuplicateConflict(t *testing.T) {
	t.Parallel()
	svc := NewRfxService(&mockRfxStore{
		getEventFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
			return &domain.RfxEvent{Status: domain.RfxStatusDraft}, nil
		},
		addParticipantFn: func(context.Context, domain.AddRfxParticipantInput) (*domain.RfxParticipant, error) {
			return nil, apperrors.Conflict("record already exists", map[string]any{"detail": "uq_rfx_participant"})
		},
	})
	_, err := svc.AddParticipant(context.Background(), uuid.New(), domain.AddRfxParticipantInput{
		TenantID: uuid.New(), CompanyID: uuid.New(), ParticipantType: "CARRIER",
	})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
}

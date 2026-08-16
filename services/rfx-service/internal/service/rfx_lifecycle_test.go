package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

type mockRfxStoreExtended struct {
	mockRfxStore
	countLotsFn            func(ctx context.Context, eventID, tenantID uuid.UUID) (int, error)
	closeExpiredFn         func(ctx context.Context, tenantID uuid.UUID, now time.Time) (int, error)
	updateDeadlineFn       func(ctx context.Context, id, tenantID uuid.UUID, deadline *time.Time, expectedVersion int) (*domain.RfxEvent, error)
}

func (m *mockRfxStoreExtended) CountLotsByEvent(ctx context.Context, eventID, tenantID uuid.UUID) (int, error) {
	if m.countLotsFn != nil {
		return m.countLotsFn(ctx, eventID, tenantID)
	}
	return 0, nil
}

func (m *mockRfxStoreExtended) CloseExpiredResponses(ctx context.Context, tenantID uuid.UUID, now time.Time) (int, error) {
	if m.closeExpiredFn != nil {
		return m.closeExpiredFn(ctx, tenantID, now)
	}
	return 0, nil
}

func (m *mockRfxStoreExtended) UpdateEventResponseDeadline(ctx context.Context, id, tenantID uuid.UUID, deadline *time.Time, expectedVersion int) (*domain.RfxEvent, error) {
	if m.updateDeadlineFn != nil {
		return m.updateDeadlineFn(ctx, id, tenantID, deadline, expectedVersion)
	}
	return nil, nil
}

func TestRfxServiceTransitionEvent(t *testing.T) {
	t.Parallel()
	eventID := uuid.New()
	tenantID := uuid.New()
	store := &mockRfxStoreExtended{
		mockRfxStore: mockRfxStore{
			getEventFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
				return &domain.RfxEvent{ID: eventID, TenantID: tenantID, Status: domain.RfxStatusPublished, RfxType: "SPOT_RFQ"}, nil
			},
			updateStatusFn: func(_ context.Context, id, tenant uuid.UUID, expected, newStatus string) (*domain.RfxEvent, error) {
				if expected != domain.RfxStatusPublished || newStatus != domain.RfxStatusResponsesOpen {
					t.Fatalf("unexpected transition %s -> %s", expected, newStatus)
				}
				return &domain.RfxEvent{ID: id, TenantID: tenant, Status: newStatus}, nil
			},
		},
	}
	svc := NewRfxService(store, nil)
	event, err := svc.TransitionEvent(context.Background(), domain.ActorContext{TenantID: tenantID}, eventID, domain.RfxCommandOpenResponses)
	if err != nil || event.Status != domain.RfxStatusResponsesOpen {
		t.Fatalf("event=%+v err=%v", event, err)
	}
}

func TestRfxServicePublishRequiresLotsForLongForm(t *testing.T) {
	t.Parallel()
	eventID := uuid.New()
	tenantID := uuid.New()
	store := &mockRfxStoreExtended{
		mockRfxStore: mockRfxStore{
			getEventFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
				return &domain.RfxEvent{ID: eventID, Status: domain.RfxStatusDraft, Title: "T", OwnerCompanyID: uuid.New(), RfxType: "RFQ"}, nil
			},
		},
		countLotsFn: func(context.Context, uuid.UUID, uuid.UUID) (int, error) { return 0, nil },
	}
	svc := NewRfxService(store, nil)
	_, err := svc.PublishEvent(context.Background(), domain.ActorContext{TenantID: tenantID}, eventID)
	if err == nil {
		t.Fatal("expected validation for missing lots")
	}
}

func TestRfxServiceCloseExpiredResponses(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	store := &mockRfxStoreExtended{
		closeExpiredFn: func(_ context.Context, tenant uuid.UUID, now time.Time) (int, error) {
			if tenant != tenantID {
				t.Fatalf("unexpected tenant %s", tenant)
			}
			return 2, nil
		},
	}
	svc := NewRfxService(store, nil)
	count, err := svc.CloseExpiredResponses(context.Background(), tenantID, time.Now().UTC())
	if err != nil || count != 2 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}

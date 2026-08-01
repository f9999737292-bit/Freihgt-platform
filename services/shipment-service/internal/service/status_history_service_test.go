package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type mockStatusHistoryStore struct {
	getFn     func(ctx context.Context, id, tenantID uuid.UUID) (*domain.Shipment, error)
	listFn    func(ctx context.Context, filter domain.ListStatusHistoryFilter) ([]domain.ShipmentStatusHistory, int, error)
	hasInitFn func(ctx context.Context, tenantID, shipmentID uuid.UUID) (bool, error)
}

func (m *mockStatusHistoryStore) GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.Shipment, error) {
	return m.getFn(ctx, id, tenantID)
}
func (m *mockStatusHistoryStore) ListStatusHistory(ctx context.Context, filter domain.ListStatusHistoryFilter) ([]domain.ShipmentStatusHistory, int, error) {
	return m.listFn(ctx, filter)
}
func (m *mockStatusHistoryStore) HasInitialStatusHistory(ctx context.Context, tenantID, shipmentID uuid.UUID) (bool, error) {
	return m.hasInitFn(ctx, tenantID, shipmentID)
}

func TestStatusHistoryServiceCompleteHistory(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	now := time.Now().UTC()
	svc := NewStatusHistoryService(&mockStatusHistoryStore{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{ID: shipmentID, TenantID: tenantID, ShipmentNumber: "SHP-1", Status: domain.ShipmentStatusInTransit}, nil
		},
		listFn: func(_ context.Context, filter domain.ListStatusHistoryFilter) ([]domain.ShipmentStatusHistory, int, error) {
			if filter.TenantID != tenantID || filter.ShipmentID != shipmentID {
				t.Fatalf("unexpected filter tenant=%s shipment=%s", filter.TenantID, filter.ShipmentID)
			}
			return []domain.ShipmentStatusHistory{{
				ID: uuid.New(), TenantID: tenantID, ShipmentID: shipmentID, ShipmentVersion: 1,
				ToStatus: domain.ShipmentStatusCarrierAssigned, OccurredAt: now, RecordedAt: now,
			}}, 1, nil
		},
		hasInitFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) { return true, nil },
	})

	result, err := svc.List(context.Background(), domain.ListStatusHistoryFilter{
		TenantID: tenantID, ShipmentID: shipmentID, Page: 1, Limit: 50, Order: "desc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Complete {
		t.Fatal("expected complete history")
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", result.Warnings)
	}
}

func TestStatusHistoryServicePartialLegacyHistory(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	from := domain.ShipmentStatusCarrierAssigned
	now := time.Now().UTC()
	svc := NewStatusHistoryService(&mockStatusHistoryStore{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{ID: shipmentID, TenantID: tenantID, ShipmentNumber: "SHP-2", Status: domain.ShipmentStatusInTransit}, nil
		},
		listFn: func(context.Context, domain.ListStatusHistoryFilter) ([]domain.ShipmentStatusHistory, int, error) {
			return []domain.ShipmentStatusHistory{{
				ID: uuid.New(), TenantID: tenantID, ShipmentID: shipmentID, ShipmentVersion: 4,
				FromStatus: &from, ToStatus: domain.ShipmentStatusInTransit, OccurredAt: now, RecordedAt: now,
			}}, 1, nil
		},
		hasInitFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) { return false, nil },
	})

	result, err := svc.List(context.Background(), domain.ListStatusHistoryFilter{
		TenantID: tenantID, ShipmentID: shipmentID, Page: 1, Limit: 50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Complete {
		t.Fatal("legacy history must be incomplete")
	}
	if len(result.Warnings) != 1 || result.Warnings[0] != domain.StatusHistoryWarningPartial {
		t.Fatalf("expected partial warning, got %#v", result.Warnings)
	}
}

func TestStatusHistoryServiceForeignShipmentReturns404(t *testing.T) {
	t.Parallel()
	svc := NewStatusHistoryService(&mockStatusHistoryStore{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return nil, apperrors.NotFound("shipment not found")
		},
		listFn: func(context.Context, domain.ListStatusHistoryFilter) ([]domain.ShipmentStatusHistory, int, error) {
			return nil, 0, nil
		},
		hasInitFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) { return false, nil },
	})
	_, err := svc.List(context.Background(), domain.ListStatusHistoryFilter{
		TenantID: uuid.New(), ShipmentID: uuid.New(), Page: 1, Limit: 50,
	})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestParseStatusHistoryListFilterRejectsInvalidLimit(t *testing.T) {
	t.Parallel()
	_, err := ParseStatusHistoryListFilter(uuid.New(), uuid.New(), 1, 500, "desc")
	if err == nil {
		t.Fatal("expected validation error for limit > 200")
	}
}

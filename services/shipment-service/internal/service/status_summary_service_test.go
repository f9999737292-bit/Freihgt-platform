package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/repository"
)

type mockStatusSummaryRepo struct {
	getFn func(ctx context.Context, tenantID uuid.UUID) ([]repository.StatusSummaryRow, error)
}

func (m *mockStatusSummaryRepo) GetStatusSummary(ctx context.Context, tenantID uuid.UUID) ([]repository.StatusSummaryRow, error) {
	return m.getFn(ctx, tenantID)
}

func TestStatusSummaryServiceZeroTenantReturnsValidation(t *testing.T) {
	t.Parallel()
	svc := NewStatusSummaryService(&mockStatusSummaryRepo{
		getFn: func(context.Context, uuid.UUID) ([]repository.StatusSummaryRow, error) {
			t.Fatal("repository must not be called for zero tenant")
			return nil, nil
		},
	})

	_, err := svc.GetStatusSummary(context.Background(), uuid.Nil)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestStatusSummaryServiceCountsByStatus(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	fixedNow := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	svc := NewStatusSummaryService(&mockStatusSummaryRepo{
		getFn: func(_ context.Context, id uuid.UUID) ([]repository.StatusSummaryRow, error) {
			if id != tenantID {
				t.Fatalf("tenant=%s want %s", id, tenantID)
			}
			return []repository.StatusSummaryRow{
				{Status: domain.ShipmentStatusCarrierAssigned, ShipmentCount: 2, TotalShipments: 9},
				{Status: domain.ShipmentStatusInTransit, ShipmentCount: 3, TotalShipments: 9},
				{Status: domain.ShipmentStatusDelivered, ShipmentCount: 4, TotalShipments: 9},
			}, nil
		},
	})
	svc.now = func() time.Time { return fixedNow }

	summary, err := svc.GetStatusSummary(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.TotalShipments != 9 || summary.CountedShipments != 9 {
		t.Fatalf("totals=%d counted=%d want 9/9", summary.TotalShipments, summary.CountedShipments)
	}
	if summary.ByStatus[domain.ShipmentStatusCarrierAssigned] != 2 {
		t.Fatalf("carrier assigned=%d want 2", summary.ByStatus[domain.ShipmentStatusCarrierAssigned])
	}
	if summary.ByStatus[domain.ShipmentStatusInTransit] != 3 {
		t.Fatalf("in transit=%d want 3", summary.ByStatus[domain.ShipmentStatusInTransit])
	}
	if summary.ByStatus[domain.ShipmentStatusDelivered] != 4 {
		t.Fatalf("delivered=%d want 4", summary.ByStatus[domain.ShipmentStatusDelivered])
	}
	if !summary.Complete {
		t.Fatal("expected complete summary")
	}
	if !summary.CalculatedAt.Equal(fixedNow) {
		t.Fatalf("calculatedAt=%v want %v", summary.CalculatedAt, fixedNow)
	}
}

func TestStatusSummaryServiceCompleteFlagWhenCountsMatch(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	svc := NewStatusSummaryService(&mockStatusSummaryRepo{
		getFn: func(context.Context, uuid.UUID) ([]repository.StatusSummaryRow, error) {
			return []repository.StatusSummaryRow{
				{Status: domain.ShipmentStatusCancelled, ShipmentCount: 5, TotalShipments: 5},
			}, nil
		},
	})

	summary, err := svc.GetStatusSummary(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !summary.Complete {
		t.Fatal("expected complete summary when counted matches total")
	}
	if len(summary.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", summary.Warnings)
	}
}

func TestStatusSummaryServiceUnknownStatusMarksIncomplete(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	svc := NewStatusSummaryService(&mockStatusSummaryRepo{
		getFn: func(context.Context, uuid.UUID) ([]repository.StatusSummaryRow, error) {
			return []repository.StatusSummaryRow{
				{Status: domain.ShipmentStatusDelivered, ShipmentCount: 2, TotalShipments: 3},
				{Status: "LEGACY_UNKNOWN", ShipmentCount: 1, TotalShipments: 3},
			}, nil
		},
	})

	summary, err := svc.GetStatusSummary(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Complete {
		t.Fatal("unknown status must mark summary incomplete")
	}
	if len(summary.Warnings) != 1 || summary.Warnings[0] != WarningUnknownShipmentStatus {
		t.Fatalf("warnings=%#v want UNKNOWN_SHIPMENT_STATUS", summary.Warnings)
	}
	if summary.CountedShipments != 2 || summary.TotalShipments != 3 {
		t.Fatalf("counted=%d total=%d want 2/3", summary.CountedShipments, summary.TotalShipments)
	}
	if _, ok := summary.ByStatus["LEGACY_UNKNOWN"]; ok {
		t.Fatal("unknown status must not appear in byStatus")
	}
}

func TestStatusSummaryServiceOnlyUnknownStatuses(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	svc := NewStatusSummaryService(&mockStatusSummaryRepo{
		getFn: func(context.Context, uuid.UUID) ([]repository.StatusSummaryRow, error) {
			return []repository.StatusSummaryRow{
				{Status: "LEGACY_UNKNOWN", ShipmentCount: 2, TotalShipments: 2},
			}, nil
		},
	})

	summary, err := svc.GetStatusSummary(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Complete {
		t.Fatal("only unknown statuses must be incomplete")
	}
	if summary.TotalShipments != 2 || summary.CountedShipments != 0 {
		t.Fatalf("total=%d counted=%d want 2/0", summary.TotalShipments, summary.CountedShipments)
	}
	if len(summary.ByStatus) != 0 {
		t.Fatalf("byStatus=%#v want empty", summary.ByStatus)
	}
}

func TestStatusSummaryServiceOverflowSafeSum(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	svc := NewStatusSummaryService(&mockStatusSummaryRepo{
		getFn: func(context.Context, uuid.UUID) ([]repository.StatusSummaryRow, error) {
			return []repository.StatusSummaryRow{
				{Status: domain.ShipmentStatusInTransit, ShipmentCount: 1_000_000_000, TotalShipments: 2_000_000_000},
				{Status: domain.ShipmentStatusDelivered, ShipmentCount: 1_000_000_000, TotalShipments: 2_000_000_000},
			}, nil
		},
	})

	summary, err := svc.GetStatusSummary(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.TotalShipments != 2_000_000_000 || summary.CountedShipments != 2_000_000_000 {
		t.Fatalf("total=%d counted=%d", summary.TotalShipments, summary.CountedShipments)
	}
	if !summary.Complete {
		t.Fatal("expected complete summary for large counts")
	}
}

func TestStatusSummaryServiceUnknownStatusWarningDoesNotExposeRawValue(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	svc := NewStatusSummaryService(&mockStatusSummaryRepo{
		getFn: func(context.Context, uuid.UUID) ([]repository.StatusSummaryRow, error) {
			return []repository.StatusSummaryRow{
				{Status: "RAW_SECRET_STATUS", ShipmentCount: 1, TotalShipments: 1},
			}, nil
		},
	})

	summary, err := svc.GetStatusSummary(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, warning := range summary.Warnings {
		if warning != WarningUnknownShipmentStatus {
			t.Fatalf("warning=%q must not expose raw status", warning)
		}
	}
}

func TestStatusSummaryServiceEmptyTenantReturnsZeroCounts(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	svc := NewStatusSummaryService(&mockStatusSummaryRepo{
		getFn: func(_ context.Context, id uuid.UUID) ([]repository.StatusSummaryRow, error) {
			if id != tenantID {
				t.Fatalf("tenant=%s want %s", id, tenantID)
			}
			return nil, nil
		},
	})

	summary, err := svc.GetStatusSummary(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.TotalShipments != 0 || summary.CountedShipments != 0 {
		t.Fatalf("totals=%d counted=%d want 0/0", summary.TotalShipments, summary.CountedShipments)
	}
	if len(summary.ByStatus) != 0 {
		t.Fatalf("byStatus=%#v want empty", summary.ByStatus)
	}
	if !summary.Complete {
		t.Fatal("empty tenant summary should be complete")
	}
}

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/transport-order-service/internal/domain"
	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
)

type rateSnapshotReadRepo interface {
	GetOrderByID(ctx context.Context, tenantID, orderID uuid.UUID) (*domain.TransportOrder, error)
	GetSnapshotByTransportOrder(ctx context.Context, tenantID, transportOrderID uuid.UUID) (*domain.RateSnapshot, error)
}

type stubRateSnapshotReadRepo struct {
	order    *domain.TransportOrder
	snapshot *domain.RateSnapshot
	orderErr error
	snapErr  error
}

func (s *stubRateSnapshotReadRepo) GetOrderByID(_ context.Context, _, _ uuid.UUID) (*domain.TransportOrder, error) {
	return s.order, s.orderErr
}

func (s *stubRateSnapshotReadRepo) GetSnapshotByTransportOrder(_ context.Context, _, _ uuid.UUID) (*domain.RateSnapshot, error) {
	return s.snapshot, s.snapErr
}

func TestRateSnapshotReadService_UnpricedOrderReturns409(t *testing.T) {
	tenantID := uuid.New()
	orderID := uuid.New()
	svc := NewRateSnapshotReadService(&stubRateSnapshotReadRepo{
		order: &domain.TransportOrder{ID: orderID, TenantID: tenantID, PricingModelVersion: nil},
	})
	_, err := svc.GetRateSnapshotByTransportOrder(context.Background(), tenantID, orderID)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestRateSnapshotReadService_MissingSnapshotReturns409(t *testing.T) {
	tenantID := uuid.New()
	orderID := uuid.New()
	version := domain.PricingModelVersionSnapshotV1
	svc := NewRateSnapshotReadService(&stubRateSnapshotReadRepo{
		order:   &domain.TransportOrder{ID: orderID, TenantID: tenantID, PricingModelVersion: &version},
		snapErr: apperrors.NotFound("rate snapshot not found"),
	})
	_, err := svc.GetRateSnapshotByTransportOrder(context.Background(), tenantID, orderID)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestRateSnapshotReadService_ReturnsSnapshotTotalAmount(t *testing.T) {
	tenantID := uuid.New()
	orderID := uuid.New()
	version := domain.PricingModelVersionSnapshotV1
	base := decimal.RequireFromString("1000.00")
	total := decimal.RequireFromString("1250.00")
	svc := NewRateSnapshotReadService(&stubRateSnapshotReadRepo{
		order: &domain.TransportOrder{ID: orderID, TenantID: tenantID, PricingModelVersion: &version},
		snapshot: &domain.RateSnapshot{
			TotalAmount: total,
			BaseAmount:  &base,
		},
	})
	got, err := svc.GetRateSnapshotByTransportOrder(context.Background(), tenantID, orderID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.TotalAmount.Equal(total) {
		t.Fatalf("expected total_amount snapshot value, got %s", got.TotalAmount)
	}
}

package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type OperationsAnalyticsSourceStore interface {
	OperationsFoundation(ctx context.Context, tenantID uuid.UUID) (domain.OperationsAnalyticsSourceSnapshot, error)
}

type OperationsAnalyticsSourceService struct {
	store OperationsAnalyticsSourceStore
}

func NewOperationsAnalyticsSourceService(store OperationsAnalyticsSourceStore) *OperationsAnalyticsSourceService {
	return &OperationsAnalyticsSourceService{store: store}
}

func (s *OperationsAnalyticsSourceService) OperationsFoundation(ctx context.Context, tenantID uuid.UUID) (domain.OperationsAnalyticsSourceSnapshot, error) {
	if err := domain.ValidateVerifiedTenant(tenantID); err != nil {
		return domain.OperationsAnalyticsSourceSnapshot{}, err
	}
	if s == nil || s.store == nil {
		return domain.OperationsAnalyticsSourceSnapshot{}, apperrors.Internal("analytics source is not configured", nil)
	}
	snap, err := s.store.OperationsFoundation(ctx, tenantID)
	if err != nil {
		return domain.OperationsAnalyticsSourceSnapshot{}, err
	}
	snap.TenantID = tenantID
	if snap.ShipmentTotal < 0 || snap.OnTimeDeliveryDenominator < 0 || snap.OnTimeDeliveryNumerator < 0 || snap.ReturnCaseCount < 0 || snap.RedirectCaseCount < 0 {
		return domain.OperationsAnalyticsSourceSnapshot{}, apperrors.Internal("analytics source snapshot is inconsistent", nil)
	}
	if snap.OnTimeDeliveryNumerator > snap.OnTimeDeliveryDenominator || snap.OnTimeDeliveryDenominator > snap.ShipmentTotal {
		return domain.OperationsAnalyticsSourceSnapshot{}, apperrors.Internal("analytics source snapshot is inconsistent", nil)
	}
	return snap, nil
}

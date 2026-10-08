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
	if snap.Carriers == nil {
		snap.Carriers = []domain.OperationsAnalyticsCarrierSource{}
	}
	if err := validateOperationsAnalyticsSource(snap); err != nil {
		return domain.OperationsAnalyticsSourceSnapshot{}, err
	}
	return snap, nil
}

func validateOperationsAnalyticsSource(snap domain.OperationsAnalyticsSourceSnapshot) error {
	if snap.ShipmentTotal < 0 || snap.OnTimePickupDenominator < 0 || snap.OnTimePickupNumerator < 0 || snap.OnTimeDeliveryDenominator < 0 || snap.OnTimeDeliveryNumerator < 0 || snap.ReturnCaseCount < 0 || snap.RedirectCaseCount < 0 {
		return apperrors.Internal("analytics source snapshot is inconsistent", nil)
	}
	if snap.OnTimePickupNumerator > snap.OnTimePickupDenominator || snap.OnTimePickupDenominator > snap.ShipmentTotal {
		return apperrors.Internal("analytics source snapshot is inconsistent", nil)
	}
	if snap.OnTimeDeliveryNumerator > snap.OnTimeDeliveryDenominator || snap.OnTimeDeliveryDenominator > snap.ShipmentTotal {
		return apperrors.Internal("analytics source snapshot is inconsistent", nil)
	}
	seen := make(map[uuid.UUID]struct{}, len(snap.Carriers))
	for _, row := range snap.Carriers {
		if row.CarrierCompanyID == uuid.Nil {
			return apperrors.Internal("analytics source snapshot is inconsistent", nil)
		}
		if _, ok := seen[row.CarrierCompanyID]; ok {
			return apperrors.Internal("analytics source snapshot is inconsistent", nil)
		}
		seen[row.CarrierCompanyID] = struct{}{}
		if row.OnTimePickupDenominator < 0 || row.OnTimePickupNumerator < 0 || row.OnTimeDeliveryDenominator < 0 || row.OnTimeDeliveryNumerator < 0 {
			return apperrors.Internal("analytics source snapshot is inconsistent", nil)
		}
		if row.OnTimePickupNumerator > row.OnTimePickupDenominator || row.OnTimeDeliveryNumerator > row.OnTimeDeliveryDenominator {
			return apperrors.Internal("analytics source snapshot is inconsistent", nil)
		}
	}
	return nil
}

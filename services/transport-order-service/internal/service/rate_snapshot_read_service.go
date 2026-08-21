package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/transport-order-service/internal/domain"
	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
)

type rateSnapshotReader interface {
	GetOrderByID(ctx context.Context, tenantID, orderID uuid.UUID) (*domain.TransportOrder, error)
	GetSnapshotByTransportOrder(ctx context.Context, tenantID, transportOrderID uuid.UUID) (*domain.RateSnapshot, error)
}

type RateSnapshotReadService struct {
	repo rateSnapshotReader
}

func NewRateSnapshotReadService(repo rateSnapshotReader) *RateSnapshotReadService {
	return &RateSnapshotReadService{repo: repo}
}

func (s *RateSnapshotReadService) GetRateSnapshotByTransportOrder(ctx context.Context, tenantID, transportOrderID uuid.UUID) (*domain.RateSnapshot, error) {
	order, err := s.repo.GetOrderByID(ctx, tenantID, transportOrderID)
	if err != nil {
		return nil, err
	}
	if order.PricingModelVersion == nil || strings.TrimSpace(*order.PricingModelVersion) != domain.PricingModelVersionSnapshotV1 {
		return nil, apperrors.Conflict("transport order has no pricing snapshot", map[string]any{"field": "pricing_model_version"})
	}
	snapshot, err := s.repo.GetSnapshotByTransportOrder(ctx, tenantID, transportOrderID)
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
			return nil, apperrors.Conflict("pricing snapshot missing for priced transport order", map[string]any{"field": "rate_snapshot"})
		}
		return nil, err
	}
	return snapshot, nil
}

package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/transport-order-service/internal/repository"
)

type TransportOrderAnalyticsDimensionService struct {
	repo *repository.TransportOrderAnalyticsDimensionRepository
}

func NewTransportOrderAnalyticsDimensionService(
	repo *repository.TransportOrderAnalyticsDimensionRepository,
) *TransportOrderAnalyticsDimensionService {
	return &TransportOrderAnalyticsDimensionService{repo: repo}
}

func (s *TransportOrderAnalyticsDimensionService) BatchGetAnalyticsDimensions(
	ctx context.Context,
	tenantID uuid.UUID,
	transportOrderIDs []uuid.UUID,
) ([]repository.TransportOrderAnalyticsDimension, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	var all []repository.TransportOrderAnalyticsDimension
	for _, chunk := range repository.ChunkTransportOrderIDs(transportOrderIDs, repository.MaxAnalyticsDimensionBatchSize) {
		items, err := s.repo.BatchGetByTransportOrderIDs(ctx, tenantID, chunk)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
	}
	return all, nil
}

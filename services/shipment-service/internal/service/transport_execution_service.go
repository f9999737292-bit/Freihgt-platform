package service

import (
	"context"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/repository"
)

type TransportExecutionService struct {
	repo *repository.TransportExecutionRepository
}

func NewTransportExecutionService(repo *repository.TransportExecutionRepository) *TransportExecutionService {
	return &TransportExecutionService{repo: repo}
}

func (s *TransportExecutionService) CreateExecutionProjectionFromActivation(ctx context.Context, cmd domain.ProjectionCommand) (domain.ProjectionResult, error) {
	return s.repo.Project(ctx, cmd)
}

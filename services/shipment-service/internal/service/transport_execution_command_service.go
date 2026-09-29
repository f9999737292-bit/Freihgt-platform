package service

import (
	"context"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/repository"
)

// TransportExecutionCommandService applies stop and action commands in process.
// It is not mounted on HTTP.
type TransportExecutionCommandService struct {
	repo *repository.TransportExecutionCommandRepository
}

func NewTransportExecutionCommandService(repo *repository.TransportExecutionCommandRepository) *TransportExecutionCommandService {
	return &TransportExecutionCommandService{repo: repo}
}

func (s *TransportExecutionCommandService) Execute(ctx context.Context, cmd domain.ExecutionCommand) (domain.ExecutionCommandResult, error) {
	if s == nil || s.repo == nil {
		return domain.ExecutionCommandResult{}, apperrors.Internal("execution command repository is required", nil)
	}
	return s.repo.Execute(ctx, cmd)
}

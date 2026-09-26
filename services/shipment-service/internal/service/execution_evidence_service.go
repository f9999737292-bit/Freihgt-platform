package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type executionEvidenceStore interface {
	GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.Shipment, error)
	ListCargoExecutionEvidence(ctx context.Context, tenantID, shipmentID uuid.UUID) ([]domain.ShipmentCargoEvidence, error)
}

type ExecutionEvidenceService struct {
	store executionEvidenceStore
}

func NewExecutionEvidenceService(store executionEvidenceStore) *ExecutionEvidenceService {
	return &ExecutionEvidenceService{store: store}
}

func (s *ExecutionEvidenceService) ExecutionContext(ctx context.Context, tenantID, shipmentID uuid.UUID) (domain.ShipmentExecutionContext, error) {
	shipment, err := s.load(ctx, tenantID, shipmentID)
	if err != nil {
		return domain.ShipmentExecutionContext{}, err
	}
	return domain.ExecutionContextFromShipment(*shipment), nil
}

func (s *ExecutionEvidenceService) OnboardCargo(ctx context.Context, tenantID, shipmentID uuid.UUID) (domain.OnboardCargoView, error) {
	shipment, err := s.load(ctx, tenantID, shipmentID)
	if err != nil {
		return domain.OnboardCargoView{}, err
	}
	rows, err := s.store.ListCargoExecutionEvidence(ctx, tenantID, shipmentID)
	if err != nil {
		return domain.OnboardCargoView{}, err
	}
	return domain.ResolveOnboardCargo(*shipment, rows), nil
}

func (s *ExecutionEvidenceService) load(ctx context.Context, tenantID, shipmentID uuid.UUID) (*domain.Shipment, error) {
	if err := domain.ValidateVerifiedTenant(tenantID); err != nil {
		return nil, err
	}
	if shipmentID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	return s.store.GetByIDAndTenant(ctx, shipmentID, tenantID)
}

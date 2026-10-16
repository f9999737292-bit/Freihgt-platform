package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

// PODRequirementStore loads one canonical delivery subject and its latest decision.
type PODRequirementStore interface {
	LoadPODRequirement(ctx context.Context, operatingTenantID, executionID, actionID uuid.UUID) (domain.PODRequirementLoad, error)
}

// PODRequirementService maps a missing decision to synthetic UNKNOWN. It does not write.
type PODRequirementService struct {
	store PODRequirementStore
}

func NewPODRequirementService(store PODRequirementStore) *PODRequirementService {
	return &PODRequirementService{store: store}
}

func (s *PODRequirementService) GetPODRequirement(ctx context.Context, operatingTenantID, executionID, actionID uuid.UUID) (domain.PODRequirementView, error) {
	load, err := s.store.LoadPODRequirement(ctx, operatingTenantID, executionID, actionID)
	if err != nil {
		return domain.PODRequirementView{}, err
	}
	if load.Subject.ActionType != domain.ActionTypeDelivery {
		return domain.PODRequirementView{}, apperrors.Conflict(domain.PODReasonActionNotDelivery, map[string]any{"reason": domain.PODReasonActionNotDelivery})
	}
	if load.Decision == nil {
		return domain.SyntheticPODRequirement(load.Subject), nil
	}
	return domain.ViewFromPODDecision(load.Subject, *load.Decision)
}

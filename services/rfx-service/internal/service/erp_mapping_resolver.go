package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
)

type ErpMappingResolver struct {
	repo *repository.ReferenceMappingRepository
}

func NewErpMappingResolver(repo *repository.ReferenceMappingRepository) *ErpMappingResolver {
	return &ErpMappingResolver{repo: repo}
}

func (r *ErpMappingResolver) ResolveCode(
	ctx context.Context,
	tenantID uuid.UUID,
	mappingType string,
	externalCode string,
) (string, domain.ReferenceMappingSet, bool, error) {
	set, err := r.selectActiveSet(ctx, tenantID, mappingType)
	if err != nil {
		return "", domain.ReferenceMappingSet{}, false, err
	}
	canonical, err := r.repo.ResolveCanonicalCode(ctx, set.ID, externalCode)
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
			return "", *set, false, apperrors.Validation("mapping entry not found", map[string]any{
				"field":           mappingType,
				"external_source": externalCode,
			})
		}
		return "", *set, false, err
	}
	return canonical, *set, false, nil
}

func (r *ErpMappingResolver) selectActiveSet(ctx context.Context, tenantID uuid.UUID, mappingType string) (*domain.ReferenceMappingSet, error) {
	set, err := r.selectSetForScope(ctx, &tenantID, mappingType)
	if err == nil {
		return set, nil
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		return nil, err
	}
	if appErr.Code == apperrors.CodeValidation {
		return nil, err
	}
	if appErr.Code != apperrors.CodeNotFound {
		return nil, err
	}
	return r.selectSetForScope(ctx, nil, mappingType)
}

func (r *ErpMappingResolver) selectSetForScope(
	ctx context.Context,
	scopeTenant *uuid.UUID,
	mappingType string,
) (*domain.ReferenceMappingSet, error) {
	sets, err := r.repo.GetSetsAtMaxVersion(ctx, scopeTenant, mappingType)
	if err != nil {
		return nil, err
	}
	if len(sets) == 0 {
		return nil, apperrors.NotFound("reference mapping set not found")
	}
	var active []domain.ReferenceMappingSet
	var retired int
	for _, set := range sets {
		switch set.Status {
		case domain.ReferenceMappingSetStatusActive:
			active = append(active, set)
		case domain.ReferenceMappingSetStatusRetired:
			retired++
		}
	}
	if len(active) > 1 {
		return nil, apperrors.Validation("ambiguous mapping set", map[string]any{
			"field":        mappingType,
			"machine_code": "mapping_ambiguous",
		})
	}
	if len(active) == 1 {
		return &active[0], nil
	}
	if retired > 0 {
		return nil, apperrors.Validation("mapping set retired", map[string]any{
			"field":        mappingType,
			"machine_code": "mapping_retired",
		})
	}
	return nil, apperrors.NotFound("reference mapping set not found")
}

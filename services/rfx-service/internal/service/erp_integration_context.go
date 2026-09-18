package service

import (
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type IntegrationActor struct {
	TenantID    uuid.UUID
	CompanyID   uuid.UUID
	PrincipalID uuid.UUID
	Scopes      []string
}

func (a IntegrationActor) Validate() error {
	if a.TenantID == uuid.Nil {
		return apperrors.Unauthorized("tenant context is required")
	}
	if a.CompanyID == uuid.Nil {
		return apperrors.Unauthorized("company context is required")
	}
	if a.PrincipalID == uuid.Nil {
		return apperrors.Unauthorized("integration principal is required")
	}
	return nil
}

func (a IntegrationActor) HasScopes(required ...string) bool {
	grant := make(map[string]struct{}, len(a.Scopes))
	for _, s := range a.Scopes {
		grant[s] = struct{}{}
	}
	for _, req := range required {
		if _, ok := grant[req]; !ok {
			return false
		}
	}
	return true
}

func (a IntegrationActor) ToAnalysisOwner(companyID uuid.UUID) domain.ImportAnalysis {
	principalID := a.PrincipalID
	return domain.ImportAnalysis{
		TenantID:               a.TenantID,
		ActorCompanyID:         companyID,
		IntegrationPrincipalID: &principalID,
	}
}

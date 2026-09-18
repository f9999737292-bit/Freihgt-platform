package handlers

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/platform/respond"
	"github.com/freight-platform/rfx-service/internal/service"
	"github.com/freight-platform/shared-go/integrationauth"
)

func requireIntegrationActor(w http.ResponseWriter, r *http.Request, requiredScopes ...string) (service.IntegrationActor, bool) {
	if err := rejectClientTenantQuery(r); err != nil {
		respond.Error(w, err)
		return service.IntegrationActor{}, false
	}
	tenantRaw := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	companyRaw := strings.TrimSpace(r.Header.Get("X-Company-ID"))
	principalRaw := strings.TrimSpace(r.Header.Get(integrationauth.HeaderIntegrationPrincipalID))
	if tenantRaw == "" || companyRaw == "" || principalRaw == "" {
		respond.Error(w, apperrors.Unauthorized("integration context is required"))
		return service.IntegrationActor{}, false
	}
	tenantID, err := uuid.Parse(tenantRaw)
	if err != nil {
		respond.Error(w, apperrors.Unauthorized("invalid tenant context"))
		return service.IntegrationActor{}, false
	}
	companyID, err := uuid.Parse(companyRaw)
	if err != nil {
		respond.Error(w, apperrors.Unauthorized("invalid company context"))
		return service.IntegrationActor{}, false
	}
	principalID, err := uuid.Parse(principalRaw)
	if err != nil {
		respond.Error(w, apperrors.Unauthorized("invalid integration principal"))
		return service.IntegrationActor{}, false
	}
	scopesRaw := strings.TrimSpace(r.Header.Get(integrationauth.HeaderIntegrationScopes))
	var scopes []string
	if scopesRaw != "" {
		scopes = strings.Fields(scopesRaw)
	}
	actor := service.IntegrationActor{
		TenantID:    tenantID,
		CompanyID:   companyID,
		PrincipalID: principalID,
		Scopes:      scopes,
	}
	if err := actor.Validate(); err != nil {
		respond.Error(w, err)
		return service.IntegrationActor{}, false
	}
	if len(requiredScopes) > 0 && !actor.HasScopes(requiredScopes...) {
		respond.Error(w, apperrors.Forbidden("missing required scope"))
		return service.IntegrationActor{}, false
	}
	return actor, true
}

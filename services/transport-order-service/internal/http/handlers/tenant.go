package handlers

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/transport-order-service/internal/domain"
	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
)

const headerPlatformAdmin = "X-Platform-Admin"

// resolveVerifiedTenant returns trusted tenant from gateway-set X-Tenant-ID.
func resolveVerifiedTenant(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	if raw == "" {
		return uuid.Nil, apperrors.Unauthorized("tenant context is required")
	}
	tenantID, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperrors.Validation("invalid tenant id", map[string]any{"field": "tenant_id"})
	}
	if tenantID == uuid.Nil {
		return uuid.Nil, apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	return tenantID, nil
}

func parseOrderAccessActor(r *http.Request) (domain.OrderAccessActor, error) {
	companyRaw := strings.TrimSpace(r.Header.Get("X-Company-ID"))
	kind := strings.TrimSpace(r.Header.Get("X-Actor-Kind"))
	if companyRaw == "" || kind == "" {
		return domain.OrderAccessActor{}, apperrors.Validation("actor context is required", map[string]any{"field": "actor"})
	}
	companyID, err := uuid.Parse(companyRaw)
	if err != nil {
		return domain.OrderAccessActor{}, apperrors.Validation("invalid company id", map[string]any{"field": "company_id"})
	}
	isPlatformAdmin := strings.EqualFold(strings.TrimSpace(r.Header.Get(headerPlatformAdmin)), "true")
	return domain.OrderAccessActor{
		CompanyID:       companyID,
		ActorKind:       kind,
		IsPlatformAdmin: isPlatformAdmin,
	}, nil
}

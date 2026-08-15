package handlers

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const headerCarrierCompanyID = "X-Carrier-Company-ID"

// resolveVerifiedTenant returns trusted tenant context for hardened read endpoints.
// Tenant must come from X-Tenant-ID injected by API Gateway after JWT validation.
func resolveVerifiedTenant(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	if raw == "" {
		return uuid.Nil, apperrors.Unauthorized("tenant context is required")
	}
	tenantID, err := domain.ParseUUID(raw, "tenant_id")
	if err != nil {
		return uuid.Nil, err
	}
	if tenantID == uuid.Nil {
		return uuid.Nil, apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	return tenantID, nil
}

func resolveVerifiedUser(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.Header.Get("X-User-ID"))
	if raw == "" {
		return uuid.Nil, apperrors.Unauthorized("user context is required")
	}
	userID, err := domain.ParseUUID(raw, "user_id")
	if err != nil {
		return uuid.Nil, err
	}
	if userID == uuid.Nil {
		return uuid.Nil, apperrors.Validation("user_id is required", map[string]any{"field": "user_id"})
	}
	return userID, nil
}

func resolveOptionalCarrierScope(r *http.Request) (*uuid.UUID, error) {
	raw := strings.TrimSpace(r.Header.Get(headerCarrierCompanyID))
	if raw == "" {
		return nil, nil
	}
	carrierID, err := domain.ParseUUID(raw, "carrier_company_id")
	if err != nil {
		return nil, err
	}
	return &carrierID, nil
}

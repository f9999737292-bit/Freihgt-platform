package handlers

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
)

func resolveVerifiedTenant(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	if raw == "" {
		return uuid.Nil, apperrors.Unauthorized("tenant context is required")
	}
	tenantID, err := domain.ParseUUID(raw, "tenant_id")
	if err != nil {
		return uuid.Nil, apperrors.Validation("invalid tenant_id", map[string]any{"field": "tenant_id"})
	}
	if tenantID == uuid.Nil {
		return uuid.Nil, apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	return tenantID, nil
}

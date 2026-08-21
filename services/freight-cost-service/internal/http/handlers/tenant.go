package handlers

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
)

func ParseTrustedTenant(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	if raw == "" {
		return uuid.Nil, apperrors.Validation("tenant context is required", map[string]any{"field": "tenant_id"})
	}
	tenantID, err := uuid.Parse(raw)
	if err != nil || tenantID == uuid.Nil {
		return uuid.Nil, apperrors.Validation("invalid tenant id", map[string]any{"field": "tenant_id"})
	}
	return tenantID, nil
}

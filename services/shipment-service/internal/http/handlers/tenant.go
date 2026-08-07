package handlers

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

// resolveVerifiedTenant returns the trusted tenant for tenant-bound detail read paths
// (shipments, drivers, vehicles). Tenant must come from X-Tenant-ID set by API Gateway.
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

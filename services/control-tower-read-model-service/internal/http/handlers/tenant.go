package handlers

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
)

var eventIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

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

func resolveVerifiedUser(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.Header.Get("X-User-ID"))
	if raw == "" {
		return uuid.Nil, apperrors.Unauthorized("user context is required")
	}
	userID, err := domain.ParseUUID(raw, "user_id")
	if err != nil {
		return uuid.Nil, apperrors.Validation("invalid user_id", map[string]any{"field": "user_id"})
	}
	if userID == uuid.Nil {
		return uuid.Nil, apperrors.Validation("user_id is required", map[string]any{"field": "user_id"})
	}
	return userID, nil
}

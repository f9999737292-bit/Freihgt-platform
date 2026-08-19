package handlers

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/billing-register-service/internal/domain"
	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

func resolveVerifiedTenant(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	if raw == "" {
		return uuid.Nil, apperrors.Unauthorized("tenant context is required")
	}
	return domain.ParseUUID(raw, "tenant_id")
}

func resolveVerifiedUser(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.Header.Get("X-User-ID"))
	if raw == "" {
		return uuid.Nil, apperrors.Unauthorized("user context is required")
	}
	return domain.ParseUUID(raw, "user_id")
}

func stripUntrustedCompanyHeaders(r *http.Request) {
	r.Header.Del(domain.HeaderCompanyID)
	r.Header.Del(domain.HeaderActorKind)
}

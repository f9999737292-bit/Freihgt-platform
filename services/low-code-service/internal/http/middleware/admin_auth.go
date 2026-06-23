package middleware

import (
	"net/http"

	"github.com/google/uuid"

	sharedauth "github.com/freight-platform/shared-go/auth"
	sharedlowcode "github.com/freight-platform/shared-go/lowcode"

	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
	"github.com/freight-platform/low-code-service/internal/platform/identity"
	"github.com/freight-platform/low-code-service/internal/platform/respond"
)

type AdminAuthConfig struct {
	Enabled     bool
	RoleChecker identity.RoleChecker
}

func RequireLowCodeAdmin(cfg AdminAuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			tenantID, err := tenantIDFromRequest(r)
			if err != nil {
				respond.Error(w, err)
				return
			}

			userID, ok := sharedlowcode.ActorIDFromHeader(r.Header)
			if !ok {
				respond.Error(w, apperrors.Unauthorized("authenticated user is required for low-code admin operations"))
				return
			}

			if cfg.RoleChecker == nil {
				respond.Error(w, apperrors.Internal("admin role checker is not configured", nil))
				return
			}

			roleCodes, err := cfg.RoleChecker.ListUserRoleCodes(r.Context(), userID, tenantID)
			if err != nil {
				respond.Error(w, apperrors.Internal("failed to resolve user roles", err))
				return
			}
			if !sharedauth.HasLowCodeAdminRole(roleCodes) {
				respond.Error(w, apperrors.Forbidden("low-code admin access required"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func tenantIDFromRequest(r *http.Request) (uuid.UUID, error) {
	raw := sharedlowcode.TenantIDFromHeader(r.Header)
	if raw == "" {
		return uuid.Nil, apperrors.TenantRequired()
	}
	tenantID, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperrors.Validation("invalid tenant id", map[string]any{"header": sharedlowcode.HeaderTenantID})
	}
	return tenantID, nil
}

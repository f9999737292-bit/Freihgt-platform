package authcontext

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
	"github.com/freight-platform/company-service/internal/platform/respond"
)

type contextKey struct{}

// Caller carries trusted identity propagated by API Gateway (JWT-derived headers).
type Caller struct {
	TenantID uuid.UUID
	UserID   uuid.UUID
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caller, err := FromRequest(r)
		if err != nil {
			respond.Error(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithCaller(r.Context(), caller)))
	})
}

func FromRequest(r *http.Request) (Caller, error) {
	tenantRaw := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	if tenantRaw == "" {
		return Caller{}, apperrors.Unauthorized("verified tenant context is required")
	}
	tenantID, err := uuid.Parse(tenantRaw)
	if err != nil || tenantID == uuid.Nil {
		return Caller{}, apperrors.Unauthorized("verified tenant context is required")
	}

	userRaw := strings.TrimSpace(r.Header.Get("X-User-ID"))
	if userRaw == "" {
		return Caller{}, apperrors.Unauthorized("verified user context is required")
	}
	userID, err := uuid.Parse(userRaw)
	if err != nil || userID == uuid.Nil {
		return Caller{}, apperrors.Unauthorized("verified user context is required")
	}

	return Caller{TenantID: tenantID, UserID: userID}, nil
}

func WithCaller(ctx context.Context, caller Caller) context.Context {
	return context.WithValue(ctx, contextKey{}, caller)
}

func CallerFromContext(ctx context.Context) (Caller, bool) {
	caller, ok := ctx.Value(contextKey{}).(Caller)
	return caller, ok
}

func MustCaller(ctx context.Context) (Caller, error) {
	caller, ok := CallerFromContext(ctx)
	if !ok {
		return Caller{}, apperrors.Unauthorized("verified caller context is required")
	}
	return caller, nil
}

// RejectMismatchedTenant returns forbidden when client-supplied tenant differs from trusted tenant.
func RejectMismatchedTenant(trusted uuid.UUID, supplied string) error {
	raw := strings.TrimSpace(supplied)
	if raw == "" {
		return nil
	}
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return apperrors.Validation("invalid tenant_id", map[string]any{"field": "tenant_id"})
	}
	if parsed != trusted {
		return apperrors.Forbidden("tenant_id does not match authenticated tenant")
	}
	return nil
}

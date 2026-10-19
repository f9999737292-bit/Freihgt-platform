package userauth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/platform/respond"
	"github.com/freight-platform/rfx-service/internal/platform/security"
)

type principalKey struct{}

// RequireJWT rejects the request unless the bearer token verifies.
// An empty secret fails closed. X-Tenant-ID and X-User-ID are not read.
func RequireJWT(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, err := principalFromRequest(secret, r)
			if err != nil {
				respond.Error(w, apperrors.New(apperrors.CodeAuthenticationRequired, "authentication required"))
				return
			}
			ctx := context.WithValue(r.Context(), principalKey{}, principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// PrincipalFromContext returns the verified user and tenant.
func PrincipalFromContext(ctx context.Context) (security.AccessPrincipal, bool) {
	principal, ok := ctx.Value(principalKey{}).(security.AccessPrincipal)
	if !ok || principal.UserID == uuid.Nil || principal.TenantID == uuid.Nil {
		return security.AccessPrincipal{}, false
	}
	return principal, true
}

func principalFromRequest(secret string, r *http.Request) (security.AccessPrincipal, error) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(header, "Bearer ") {
		return security.AccessPrincipal{}, apperrors.New(apperrors.CodeAuthenticationRequired, "authentication required")
	}
	return security.ParseHS256(secret, strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")), time.Now().UTC())
}

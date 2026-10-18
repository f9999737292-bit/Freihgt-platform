package relationshipauth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
	"github.com/freight-platform/company-service/internal/platform/respond"
	"github.com/freight-platform/company-service/internal/platform/security"
)

type principalKey struct{}

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

func PrincipalFromContext(ctx context.Context) (domain.Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(domain.Principal)
	return principal, ok
}

func principalFromRequest(secret string, r *http.Request) (domain.Principal, error) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(header, "Bearer ") {
		return domain.Principal{}, apperrors.New(apperrors.CodeAuthenticationRequired, "authentication required")
	}
	parsed, err := security.ParseHS256(secret, strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")), time.Now().UTC())
	if err != nil {
		return domain.Principal{}, err
	}
	return domain.Principal{UserID: parsed.UserID, TenantID: parsed.TenantID}, nil
}

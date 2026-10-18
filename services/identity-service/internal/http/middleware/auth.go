package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/identity-service/internal/http/authority"
	apperrors "github.com/freight-platform/identity-service/internal/platform/errors"
	"github.com/freight-platform/identity-service/internal/platform/respond"
	"github.com/freight-platform/identity-service/internal/platform/security"
)

type contextKey string

const (
	claimsContextKey    contextKey = "tokenClaims"
	principalContextKey contextKey = "verifiedPrincipal"
)

func Auth(jwtService *security.JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				respond.Error(w, apperrors.Unauthorized("authorization header is required"))
				return
			}

			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
				respond.Error(w, apperrors.Unauthorized("invalid authorization header"))
				return
			}

			claims, err := jwtService.ParseAccessToken(strings.TrimSpace(parts[1]))
			if err != nil {
				respond.Error(w, apperrors.Unauthorized("invalid token"))
				return
			}

			ctx := context.WithValue(r.Context(), claimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireVerifiedPrincipal applies JWT verification to user and role routes.
// AUTH_ENABLED is not consulted. Identity headers are not read.
func RequireVerifiedPrincipal(jwtService *security.JWTService, lookup authority.Lookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := bearerClaims(jwtService, r)
			if err != nil {
				respond.Error(w, err)
				return
			}
			userID, err := uuid.Parse(strings.TrimSpace(claims.Subject))
			if err != nil || userID == uuid.Nil {
				respond.Error(w, apperrors.AuthenticationRequired())
				return
			}
			tenantID, err := uuid.Parse(strings.TrimSpace(claims.TenantID))
			if err != nil || tenantID == uuid.Nil {
				respond.Error(w, apperrors.TenantContextRequired())
				return
			}
			if lookup == nil {
				respond.Error(w, apperrors.Internal("authority lookup is not configured", nil))
				return
			}
			platformAdmin, roleCodes, err := lookup.LookupPrincipal(r.Context(), userID, tenantID)
			if err != nil {
				respond.Error(w, apperrors.Internal("authority lookup failed", err))
				return
			}
			principal := authority.Principal{
				UserID:        userID,
				TenantID:      tenantID,
				PlatformAdmin: platformAdmin,
				RoleCodes:     roleCodes,
			}
			ctx := context.WithValue(r.Context(), claimsContextKey, claims)
			ctx = context.WithValue(ctx, principalContextKey, principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func PrincipalFromContext(ctx context.Context) (authority.Principal, error) {
	principal, ok := ctx.Value(principalContextKey).(authority.Principal)
	if !ok || principal.UserID == uuid.Nil || principal.TenantID == uuid.Nil {
		return authority.Principal{}, apperrors.AuthenticationRequired()
	}
	return principal, nil
}

func bearerClaims(jwtService *security.JWTService, r *http.Request) (*security.TokenClaims, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return nil, apperrors.AuthenticationRequired()
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return nil, apperrors.AuthenticationRequired()
	}
	if jwtService == nil {
		return nil, apperrors.Internal("jwt service is not configured", nil)
	}
	claims, err := jwtService.ParseAccessToken(strings.TrimSpace(parts[1]))
	if err != nil || claims == nil {
		return nil, apperrors.AuthenticationRequired()
	}
	return claims, nil
}

func UserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	claims, ok := ctx.Value(claimsContextKey).(*security.TokenClaims)
	if !ok || claims == nil || claims.Subject == "" {
		return uuid.Nil, apperrors.Unauthorized("invalid token")
	}
	return uuid.Parse(claims.Subject)
}

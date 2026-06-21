package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/identity-service/internal/platform/errors"
	"github.com/freight-platform/identity-service/internal/platform/respond"
	"github.com/freight-platform/identity-service/internal/platform/security"
)

type contextKey string

const claimsContextKey contextKey = "tokenClaims"

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

func UserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	claims, ok := ctx.Value(claimsContextKey).(*security.TokenClaims)
	if !ok || claims == nil || claims.Subject == "" {
		return uuid.Nil, apperrors.Unauthorized("invalid token")
	}
	return uuid.Parse(claims.Subject)
}

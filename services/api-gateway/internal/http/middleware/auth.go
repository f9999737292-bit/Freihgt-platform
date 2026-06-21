package middleware

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
)

type tokenClaims struct {
	TenantID        string `json:"tenant_id"`
	Email           string `json:"email"`
	PreferredLocale string `json:"preferred_locale"`
	jwt.RegisteredClaims
}

func Auth(enabled bool, jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled || isPublicRoute(r.Method, r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				respond.Error(w, apperrors.Unauthorized("authorization header is required"))
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
				respond.Error(w, apperrors.Unauthorized("invalid authorization header"))
				return
			}

			claims, err := parseToken(strings.TrimSpace(parts[1]), jwtSecret)
			if err != nil {
				respond.Error(w, apperrors.Unauthorized("invalid or expired token"))
				return
			}

			if claims.Subject != "" {
				r.Header.Set("X-User-ID", claims.Subject)
			}
			if claims.TenantID != "" {
				r.Header.Set("X-Tenant-ID", claims.TenantID)
			}
			if claims.Email != "" {
				r.Header.Set("X-User-Email", claims.Email)
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isPublicRoute(method, path string) bool {
	switch {
	case method == http.MethodGet && (path == "/health" || path == "/ready" || path == "/routes" || path == "/metrics"):
		return true
	case method == http.MethodGet && (path == "/docs" || path == "/docs/" || path == "/openapi" || path == "/openapi.yaml" || path == "/openapi.json"):
		return true
	case method == http.MethodGet && strings.HasPrefix(path, "/openapi/"):
		return true
	case method == http.MethodPost && (path == "/api/v1/auth/login" || path == "/api/v1/users"):
		return true
	default:
		return false
	}
}

func parseToken(tokenString, secret string) (*tokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &tokenClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*tokenClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

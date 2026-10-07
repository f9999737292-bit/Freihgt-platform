package http

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/analytics-service/internal/platform/errors"
	"github.com/freight-platform/analytics-service/internal/platform/respond"
)

// Ingress trusts a shared internal token plus the caller name api-gateway.
// S2S_IDENTITY_CRYPTO_BOUND=NO: the caller is not bound cryptographically.

const (
	headerToken      = "X-Internal-Service-Token"
	headerCaller     = "X-Internal-Service-Name"
	headerTenant     = "X-Tenant-ID"
	authorizedCaller = "api-gateway"
)

func requireGateway(token string) func(http.Handler) http.Handler {
	expected := strings.TrimSpace(token)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if expected == "" || !fixedEqual(r.Header.Get(headerToken), expected) {
				respond.Error(w, apperrors.Unauthorized("internal service authentication failed"))
				return
			}
			caller := strings.TrimSpace(r.Header.Get(headerCaller))
			if caller == "" {
				respond.Error(w, apperrors.Forbidden("internal caller is required"))
				return
			}
			if caller != authorizedCaller {
				respond.Error(w, apperrors.Forbidden("caller is not allowed to read analytics"))
				return
			}
			tenantID, err := verifiedTenant(r.Header.Get(headerTenant))
			if err != nil {
				respond.Error(w, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(withTenant(r.Context(), tenantID)))
		})
	}
}

func verifiedTenant(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", apperrors.Unauthorized("tenant context is required")
	}
	parsed, err := uuid.Parse(raw)
	if err != nil || parsed == uuid.Nil {
		return "", apperrors.Validation("tenant_id is required")
	}
	return parsed.String(), nil
}

func fixedEqual(provided, expected string) bool {
	provided = strings.TrimSpace(provided)
	if provided == "" || expected == "" {
		return false
	}
	if len(provided) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

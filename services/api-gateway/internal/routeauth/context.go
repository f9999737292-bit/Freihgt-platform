package routeauth

import (
	"net/http"
	"strings"

	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

type RequestContext struct {
	TenantID  string
	UserID    string
	AuthToken string
	RequestID string
}

func BuildRequestContext(r *http.Request, authEnabled bool, devTenantID string) (RequestContext, error) {
	if authEnabled {
		ac, err := gwmiddleware.MustAuthContext(r.Context())
		if err != nil {
			return RequestContext{}, apperrors.Unauthorized("verified tenant context is required")
		}
		return RequestContext{
			TenantID:  ac.TenantID,
			UserID:    ac.UserID,
			AuthToken: ac.AuthToken,
			RequestID: requestIDFromRequest(r),
		}, nil
	}

	if devTenantID == "" {
		return RequestContext{}, apperrors.Unauthorized("development tenant context is not configured")
	}

	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	return RequestContext{
		TenantID:  devTenantID,
		AuthToken: authHeader,
		RequestID: requestIDFromRequest(r),
	}, nil
}

func requestIDFromRequest(r *http.Request) string {
	if id := sharedmiddleware.RequestIDFromContext(r.Context()); id != "" {
		return id
	}
	return strings.TrimSpace(r.Header.Get(sharedmiddleware.RequestIDHeader))
}

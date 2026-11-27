package handlers

import (
	"net/http"
	"strings"

	"github.com/freight-platform/shared-go/internalauth"

	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
	"github.com/freight-platform/document-service/internal/platform/respond"
)

// DenyUnprovenDocumentWrite closes a document or POD write until a trusted
// authority adapter can name the subject and prove tenant, company, and
// resource scope. X-Tenant-ID, X-User-ID, X-Company-ID, and
// X-Internal-Service-Name are not that proof. AUTH_ENABLED is not read and
// cannot open the route. A confirmed internal token still fails closed,
// because this stage has no resource-authority dependency to call.
func DenyUnprovenDocumentWrite(auth internalauth.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			respond.Error(w, documentWriteDenied(auth, r))
		})
	}
}

func documentWriteDenied(auth internalauth.Config, r *http.Request) error {
	token := strings.TrimSpace(r.Header.Get(internalauth.HeaderName))
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	hasBearer := len(authorization) >= 7 && strings.EqualFold(authorization[:7], "bearer ")
	if token == "" && !hasBearer {
		return apperrors.Unauthorized("document write credential is required")
	}
	if !auth.Authorize(r) {
		return apperrors.Unauthorized("document write credential is not confirmed")
	}
	if strings.TrimSpace(r.Header.Get(HeaderInternalServiceName)) != "" {
		return apperrors.Forbidden("document write caller is not allowed")
	}
	if strings.TrimSpace(r.Header.Get("X-User-ID")) != "" || strings.TrimSpace(r.Header.Get("X-Company-ID")) != "" {
		return apperrors.Forbidden("document write subject is not authorized")
	}
	if strings.TrimSpace(r.Header.Get("X-Tenant-ID")) != "" {
		return apperrors.Forbidden("document write tenant is not authorized")
	}
	return apperrors.Forbidden("document write authority is unavailable")
}

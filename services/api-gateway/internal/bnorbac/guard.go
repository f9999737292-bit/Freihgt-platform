package bnorbac

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/api-gateway/internal/companycontext"
	"github.com/freight-platform/api-gateway/internal/config"
	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
	"github.com/freight-platform/api-gateway/internal/routeauth"
)

type Guard struct {
	client      *companycontext.IdentityClient
	handler     http.Handler
	authEnabled bool
}

func NewGuard(cfg config.Config, handler http.Handler) *Guard {
	httpClient := &http.Client{Timeout: time.Duration(cfg.ProxyTimeoutSeconds) * time.Second}
	return &Guard{
		client:      companycontext.NewIdentityClient(httpClient, cfg.Services.Identity),
		handler:     handler,
		authEnabled: cfg.AuthEnabled,
	}
}

func (g *Guard) WithPolicy(policy Policy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !g.authEnabled {
			respond.Error(w, apperrors.Unauthorized("verified tenant context is required"))
			return
		}
		ac, err := gwmiddleware.MustAuthContext(r.Context())
		if err != nil {
			respond.Error(w, apperrors.Unauthorized("verified tenant context is required"))
			return
		}
		requestedCompany := strings.TrimSpace(ac.RequestedCompanyID)
		if requestedCompany == "" {
			respond.Error(w, apperrors.Validation("X-Company-ID is required", map[string]any{"field": "X-Company-ID"}))
			return
		}
		requestedCompanyID, err := uuid.Parse(requestedCompany)
		if err != nil {
			respond.Error(w, apperrors.Validation("invalid X-Company-ID", map[string]any{"field": "X-Company-ID"}))
			return
		}
		if queryTenant := strings.TrimSpace(r.URL.Query().Get("tenant_id")); queryTenant != "" && !strings.EqualFold(queryTenant, ac.TenantID) {
			respond.Error(w, apperrors.Forbidden("tenant_id does not match authenticated tenant"))
			return
		}

		companycontext.StripUntrustedCompanyHeaders(r.Header)
		r.Header.Del("X-Internal-Service-Token")
		r.Header.Del("X-Platform-Admin")
		r.Header.Del("X-Role")
		r.Header.Del("X-User-Roles")

		reqCtx := routeauth.RequestContext{TenantID: ac.TenantID, UserID: ac.UserID, AuthToken: ac.AuthToken}
		if ac.UserID == "" {
			respond.Error(w, apperrors.Unauthorized("verified user context is required"))
			return
		}
		memberships, err := g.client.ListUserCompanies(r.Context(), reqCtx, ac.UserID)
		if err != nil {
			g.handleIdentityError(w, err)
			return
		}
		var matched *companycontext.UserCompany
		for i := range memberships {
			parsed, parseErr := uuid.Parse(memberships[i].CompanyID)
			if parseErr != nil {
				continue
			}
			if parsed == requestedCompanyID {
				matched = &memberships[i]
				break
			}
		}
		if matched == nil {
			respond.Error(w, apperrors.Forbidden("company does not match authenticated membership"))
			return
		}
		derivedKind, err := companycontext.DeriveActorKind(matched.CompanyType, matched.RoleCodes)
		if err != nil {
			respond.Error(w, err)
			return
		}
		tenantRoles, err := g.client.ListUserTenantRoles(r.Context(), reqCtx, ac.UserID)
		if err != nil {
			g.handleIdentityError(w, err)
			return
		}
		if !policyAllows(policy, matched.RoleCodes, derivedKind, hasPlatformAdmin(tenantRoles)) {
			respond.Error(w, apperrors.Forbidden(policyDenyMessage(policy)))
			return
		}
		r.Header.Set(companycontext.HeaderCompanyID, matched.CompanyID)
		r.Header.Set(companycontext.HeaderActorKind, derivedKind)
		r.Header.Set("X-Tenant-ID", ac.TenantID)
		r.Header.Set("X-User-ID", ac.UserID)
		r.Header.Del("X-User-Email")
		r.Header.Del("X-Platform-Admin")
		g.handler.ServeHTTP(w, r)
	}
}

func (g *Guard) handleIdentityError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, routeauth.ErrIdentityUnauthorized):
		respond.Error(w, apperrors.Unauthorized("invalid or expired token"))
	case errors.Is(err, routeauth.ErrIdentityForbidden):
		respond.Error(w, apperrors.Forbidden("insufficient permission"))
	default:
		respond.Error(w, apperrors.AuthDependencyUnavailable("authentication service is temporarily unavailable"))
	}
}

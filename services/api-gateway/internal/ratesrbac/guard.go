package ratesrbac

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
	devTenantID string
}

func NewGuard(cfg config.Config, handler http.Handler) *Guard {
	httpClient := &http.Client{Timeout: time.Duration(cfg.ProxyTimeoutSeconds) * time.Second}
	return &Guard{
		client:      companycontext.NewIdentityClient(httpClient, cfg.Services.Identity),
		handler:     handler,
		authEnabled: cfg.AuthEnabled,
		devTenantID: strings.TrimSpace(cfg.DevTenantID),
	}
}

func (g *Guard) WithPolicy(policy Policy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		operation := policyMetricName(policy)
		if !g.authEnabled {
			g.handler.ServeHTTP(w, r)
			recordPublicRequest(operation, "pass_dev")
			return
		}

		ac, err := gwmiddleware.MustAuthContext(r.Context())
		if err != nil {
			recordPublicRequest(operation, "unauthorized")
			respond.Error(w, apperrors.Unauthorized("verified tenant context is required"))
			return
		}

		requestedCompany := strings.TrimSpace(ac.RequestedCompanyID)
		if requestedCompany == "" {
			recordPublicRequest(operation, "missing_company")
			recordAuthzDenied(operation, "missing_company")
			respond.Error(w, apperrors.Validation("X-Company-ID is required", map[string]any{"field": "X-Company-ID"}))
			return
		}
		requestedCompanyID, err := uuid.Parse(requestedCompany)
		if err != nil {
			recordPublicRequest(operation, "invalid_company")
			respond.Error(w, apperrors.Validation("invalid X-Company-ID", map[string]any{"field": "X-Company-ID"}))
			return
		}

		companycontext.StripUntrustedCompanyHeaders(r.Header)
		r.Header.Del("X-Internal-Service-Token")

		reqCtx := routeauth.RequestContext{
			TenantID:  ac.TenantID,
			UserID:    ac.UserID,
			AuthToken: ac.AuthToken,
		}
		if ac.UserID == "" {
			recordPublicRequest(operation, "unauthorized")
			respond.Error(w, apperrors.Unauthorized("verified user context is required"))
			return
		}

		memberships, err := g.client.ListUserCompanies(r.Context(), reqCtx, ac.UserID)
		if err != nil {
			recordPublicRequest(operation, "identity_error")
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
			recordPublicRequest(operation, "forbidden_membership")
			recordAuthzDenied(operation, "membership")
			respond.Error(w, apperrors.Forbidden("company does not match authenticated membership"))
			return
		}

		derivedKind, err := companycontext.DeriveActorKind(matched.CompanyType, matched.RoleCodes)
		if err != nil {
			recordPublicRequest(operation, "forbidden_actor")
			recordAuthzDenied(operation, "actor_kind")
			respond.Error(w, err)
			return
		}

		tenantRoles, err := g.client.ListUserTenantRoles(r.Context(), reqCtx, ac.UserID)
		if err != nil {
			recordPublicRequest(operation, "identity_error")
			g.handleIdentityError(w, err)
			return
		}
		isPlatformAdmin := hasPlatformAdmin(tenantRoles)

		if !policyAllows(policy, matched.RoleCodes, derivedKind, isPlatformAdmin) {
			recordPublicRequest(operation, "forbidden_rbac")
			recordAuthzDenied(operation, "rbac")
			respond.Error(w, apperrors.Forbidden(policyDenyMessage(policy)))
			return
		}

		r.Header.Set(companycontext.HeaderCompanyID, matched.CompanyID)
		r.Header.Set(companycontext.HeaderActorKind, derivedKind)

		verified := VerifiedContext{
			TenantID:        ac.TenantID,
			UserID:          ac.UserID,
			AuthToken:       ac.AuthToken,
			CompanyID:       matched.CompanyID,
			ActorKind:       derivedKind,
			CompanyRoles:    append([]string(nil), matched.RoleCodes...),
			IsPlatformAdmin: isPlatformAdmin,
		}
		r = r.WithContext(WithVerifiedContext(r.Context(), verified))

		recordPublicRequest(operation, "pass")
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

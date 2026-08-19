package companycontext

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/api-gateway/internal/config"
	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
	"github.com/freight-platform/api-gateway/internal/routeauth"
)

type Enforcer struct {
	client      *IdentityClient
	authEnabled bool
	devTenantID string
}

func NewEnforcer(cfg config.Config) *Enforcer {
	httpClient := &http.Client{Timeout: time.Duration(cfg.ProxyTimeoutSeconds) * time.Second}
	return &Enforcer{
		client:      NewIdentityClient(httpClient, cfg.Services.Identity),
		authEnabled: cfg.AuthEnabled,
		devTenantID: strings.TrimSpace(cfg.DevTenantID),
	}
}

func (e *Enforcer) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		StripUntrustedCompanyHeaders(r.Header)

		reqCtx, err := e.buildRequestContext(r)
		if err != nil {
			respond.Error(w, err)
			return
		}

		companyRaw := strings.TrimSpace(r.URL.Query().Get("company_id"))
		if companyRaw == "" {
			respond.Error(w, apperrors.Validation("company_id is required", map[string]any{"field": "company_id"}))
			return
		}
		requestedCompanyID, err := uuid.Parse(companyRaw)
		if err != nil {
			respond.Error(w, apperrors.Validation("invalid company_id", map[string]any{"field": "company_id"}))
			return
		}

		if reqCtx.UserID == "" {
			respond.Error(w, apperrors.Unauthorized("verified user context is required"))
			return
		}

		memberships, err := e.client.ListUserCompanies(r.Context(), reqCtx, reqCtx.UserID)
		if err != nil {
			switch {
			case errors.Is(err, routeauth.ErrIdentityUnauthorized):
				respond.Error(w, apperrors.Unauthorized("invalid or expired token"))
			case errors.Is(err, routeauth.ErrIdentityForbidden):
				respond.Error(w, apperrors.Forbidden("insufficient permission"))
			default:
				respond.Error(w, apperrors.AuthDependencyUnavailable("authentication service is temporarily unavailable"))
			}
			return
		}

		var matched *UserCompany
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
			respond.Error(w, apperrors.Forbidden("company_id does not match authenticated membership"))
			return
		}

		derivedKind, err := DeriveActorKind(matched.CompanyType, matched.RoleCodes)
		if err != nil {
			respond.Error(w, err)
			return
		}

		if rawActor := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("actor"))); rawActor != "" && rawActor != derivedKind {
			respond.Error(w, apperrors.Forbidden("actor does not match verified company membership"))
			return
		}

		r.Header.Set(HeaderCompanyID, matched.CompanyID)
		r.Header.Set(HeaderActorKind, derivedKind)
		next.ServeHTTP(w, r)
	})
}

func (e *Enforcer) buildRequestContext(r *http.Request) (routeauth.RequestContext, error) {
	if e.authEnabled {
		ac, err := gwmiddleware.MustAuthContext(r.Context())
		if err != nil {
			return routeauth.RequestContext{}, apperrors.Unauthorized("verified tenant context is required")
		}
		return routeauth.RequestContext{
			TenantID:  ac.TenantID,
			UserID:    ac.UserID,
			AuthToken: ac.AuthToken,
		}, nil
	}
	if e.devTenantID == "" {
		return routeauth.RequestContext{}, apperrors.Unauthorized("development tenant context is not configured")
	}
	return routeauth.RequestContext{
		TenantID:  e.devTenantID,
		UserID:    strings.TrimSpace(r.Header.Get("X-User-ID")),
		AuthToken: strings.TrimSpace(r.Header.Get("Authorization")),
	}, nil
}

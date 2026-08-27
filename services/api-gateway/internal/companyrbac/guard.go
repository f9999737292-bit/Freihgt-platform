package companyrbac

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
		if !g.authEnabled {
			g.handler.ServeHTTP(w, r)
			return
		}

		ac, err := gwmiddleware.MustAuthContext(r.Context())
		if err != nil {
			respond.Error(w, apperrors.Unauthorized("verified tenant context is required"))
			return
		}
		if ac.UserID == "" || ac.TenantID == "" {
			respond.Error(w, apperrors.Unauthorized("verified user context is required"))
			return
		}

		if queryTenant := strings.TrimSpace(r.URL.Query().Get("tenant_id")); queryTenant != "" && !strings.EqualFold(queryTenant, ac.TenantID) {
			respond.Error(w, apperrors.Forbidden("tenant_id does not match authenticated tenant"))
			return
		}

		reqCtx := routeauth.RequestContext{
			TenantID:  ac.TenantID,
			UserID:    ac.UserID,
			AuthToken: ac.AuthToken,
		}

		memberships, err := g.client.ListUserCompanies(r.Context(), reqCtx, ac.UserID)
		if err != nil {
			g.handleIdentityError(w, err)
			return
		}

		tenantRoles, err := g.client.ListUserTenantRoles(r.Context(), reqCtx, ac.UserID)
		if err != nil {
			g.handleIdentityError(w, err)
			return
		}
		isPlatformAdmin := hasPlatformAdmin(tenantRoles)

		var targetCompany uuid.UUID
		var matchedRoles []string
		hasMembership := false

		if policyRequiresCompanyID(policy) {
			targetCompany, err = g.resolveTargetCompanyID(r, policy)
			if err != nil {
				respond.Error(w, err)
				return
			}
			for i := range memberships {
				parsed, parseErr := uuid.Parse(memberships[i].CompanyID)
				if parseErr != nil {
					continue
				}
				if parsed == targetCompany {
					hasMembership = true
					matchedRoles = append([]string(nil), memberships[i].RoleCodes...)
					break
				}
			}
		}

		if !policyAllows(policy, matchedRoles, isPlatformAdmin, hasMembership) {
			respond.Error(w, apperrors.Forbidden(policyDenyMessage(policy)))
			return
		}

		companycontext.StripUntrustedCompanyHeaders(r.Header)
		r.Header.Del("X-Internal-Service-Token")
		r.Header.Set("X-Tenant-ID", ac.TenantID)
		r.Header.Set("X-User-ID", ac.UserID)
		if hasMembership && targetCompany != uuid.Nil {
			r.Header.Set(companycontext.HeaderCompanyID, targetCompany.String())
		}

		g.handler.ServeHTTP(w, r)
	}
}

func (g *Guard) resolveTargetCompanyID(r *http.Request, policy Policy) (uuid.UUID, error) {
	if id, ok := ParseTargetCompanyID(r.URL.Path); ok {
		return id, nil
	}
	if policy == PolicyRead || policy == PolicyUpdate || policy == PolicyDelete {
		return uuid.Nil, apperrors.NotFound("company not found")
	}
	return uuid.Nil, apperrors.Forbidden(policyDenyMessage(policy))
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

package routeauth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/freight-platform/api-gateway/internal/config"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
)

type Policy struct {
	Allow       func(roles []string) bool
	DenyMessage string
}

type Guard struct {
	proxy       http.Handler
	client      *IdentityClient
	authEnabled bool
	devTenantID string
}

func NewGuard(cfg config.Config, proxy http.Handler) *Guard {
	httpClient := &http.Client{Timeout: time.Duration(cfg.ProxyTimeoutSeconds) * time.Second}
	return &Guard{
		proxy:       proxy,
		client:      NewIdentityClient(httpClient, cfg.Services.Identity),
		authEnabled: cfg.AuthEnabled,
		devTenantID: strings.TrimSpace(cfg.DevTenantID),
	}
}

func (g *Guard) WithPolicy(policy Policy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reqCtx, err := BuildRequestContext(r, g.authEnabled, g.devTenantID)
		if err != nil {
			respond.Error(w, err)
			return
		}

		if g.authEnabled {
			if err := g.ensureAccess(r, reqCtx, policy); err != nil {
				respond.Error(w, err)
				return
			}
		}

		g.proxy.ServeHTTP(w, r)
	}
}

func (g *Guard) ensureAccess(r *http.Request, reqCtx RequestContext, policy Policy) error {
	roles, err := g.client.FetchUserRoles(r.Context(), reqCtx)
	if err != nil {
		switch {
		case errors.Is(err, ErrIdentityUnauthorized):
			return apperrors.Unauthorized("invalid or expired token")
		case errors.Is(err, ErrIdentityForbidden):
			return apperrors.Forbidden("insufficient permission")
		default:
			return apperrors.AuthDependencyUnavailable("authentication service is temporarily unavailable")
		}
	}

	if !policy.Allow(roles) {
		message := policy.DenyMessage
		if message == "" {
			message = "access denied"
		}
		return apperrors.Forbidden(message)
	}
	return nil
}

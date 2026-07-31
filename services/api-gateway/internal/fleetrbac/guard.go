package fleetrbac

import (
	"net/http"
	"strings"
	"time"

	"github.com/freight-platform/api-gateway/internal/config"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
)

type Policy int

const (
	PolicyView Policy = iota
	PolicyCreate
	PolicyAssign
)

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
		reqCtx, err := buildRequestContext(r, g.authEnabled, g.devTenantID)
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
		if strings.Contains(err.Error(), "unauthorized") {
			return apperrors.Unauthorized("invalid or expired token")
		}
		return apperrors.AuthDependencyUnavailable("authentication service is temporarily unavailable")
	}

	switch policy {
	case PolicyView:
		if !CanViewFleet(roles) {
			return apperrors.Forbidden("driver and vehicle view access denied")
		}
	case PolicyCreate:
		if !CanCreateFleet(roles) {
			return apperrors.Forbidden("driver and vehicle create access denied")
		}
	case PolicyAssign:
		if !CanAssignFleet(roles) {
			return apperrors.Forbidden("shipment fleet assignment access denied")
		}
	default:
		return apperrors.Forbidden("fleet access denied")
	}
	return nil
}

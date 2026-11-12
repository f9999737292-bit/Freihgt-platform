package analytics

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/api-gateway/internal/analyticsrbac"
	"github.com/freight-platform/api-gateway/internal/companycontext"
	"github.com/freight-platform/api-gateway/internal/config"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
	"github.com/freight-platform/api-gateway/internal/routeauth"
)

type Handler struct {
	log         *slog.Logger
	client      *Client
	companies   *companycontext.IdentityClient
	authEnabled bool
	devTenantID string
}

func NewHandler(log *slog.Logger, cfg config.Config) *Handler {
	if log == nil {
		log = slog.Default()
	}
	timeout := cfg.ProxyTimeoutSeconds
	if timeout <= 0 {
		timeout = 5
	}
	httpClient := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	return &Handler{
		log:         log,
		client:      NewClient(httpClient, cfg.Services.Analytics, cfg.AnalyticsGatewayCallerToken),
		companies:   companycontext.NewIdentityClient(httpClient, cfg.Services.Identity),
		authEnabled: cfg.AuthEnabled,
		devTenantID: cfg.DevTenantID,
	}
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	reqCtx, err := routeauth.BuildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if !h.authEnabled {
		respond.Error(w, apperrors.Forbidden("analytics company scope is not available"))
		return
	}
	if strings.TrimSpace(reqCtx.UserID) == "" {
		respond.Error(w, apperrors.Unauthorized("verified user context is required"))
		return
	}
	scope, err := h.resolveScope(r, reqCtx)
	if err != nil {
		writeIdentityError(w, err)
		return
	}

	status, body, err := h.client.GetKPI(r.Context(), chi.URLParam(r, "kpiId"), reqCtx.TenantID, r.URL.RawQuery, scope)
	if err != nil || status >= 500 {
		h.log.Error("analytics dependency failed")
		respond.JSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{
				"code":    "SERVICE_UNAVAILABLE",
				"message": "analytics service is temporarily unavailable",
				"details": map[string]any{"service": "analytics-service"},
			},
		})
		return
	}
	if len(body) == 0 {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (h *Handler) resolveScope(r *http.Request, reqCtx routeauth.RequestContext) (analyticsrbac.Scope, error) {
	roles, err := h.companies.ListUserTenantRoles(r.Context(), reqCtx, reqCtx.UserID)
	if err != nil {
		return analyticsrbac.Scope{}, err
	}
	if analyticsrbac.HasPlatformAdmin(roles) {
		return analyticsrbac.Scope{Kind: analyticsrbac.ScopeTenant}, nil
	}
	companies, err := h.companies.ListUserCompanies(r.Context(), reqCtx, reqCtx.UserID)
	if err != nil {
		return analyticsrbac.Scope{}, err
	}
	memberships := make([]analyticsrbac.Membership, 0, len(companies))
	for _, company := range companies {
		memberships = append(memberships, analyticsrbac.Membership{
			CompanyID:   company.CompanyID,
			CompanyType: company.CompanyType,
			RoleCodes:   company.RoleCodes,
		})
	}
	scope, err := analyticsrbac.Resolve(nil, memberships)
	if err != nil {
		return analyticsrbac.Scope{}, err
	}
	return scope, nil
}

func writeIdentityError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, routeauth.ErrIdentityUnauthorized):
		respond.Error(w, apperrors.Unauthorized("invalid or expired token"))
	case errors.Is(err, routeauth.ErrIdentityForbidden):
		respond.Error(w, apperrors.Forbidden("insufficient permission"))
	case errors.Is(err, analyticsrbac.ErrScopeUnavailable):
		respond.Error(w, apperrors.Forbidden("analytics company scope is not available"))
	default:
		respond.Error(w, apperrors.AuthDependencyUnavailable("authentication service is temporarily unavailable"))
	}
}

package controltower

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/freight-platform/api-gateway/internal/config"
	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

type Handler struct {
	log         *slog.Logger
	service     *Service
	client      *DownstreamClient
	authEnabled bool
	devTenantID string
}

func NewHandler(log *slog.Logger, cfg config.Config) *Handler {
	httpClient := &http.Client{Timeout: time.Duration(cfg.ProxyTimeoutSeconds) * time.Second}
	client := NewDownstreamClient(
		httpClient,
		cfg.Services.Identity,
		cfg.Services.Company,
		cfg.Services.TransportOrder,
		cfg.Services.Shipment,
		cfg.Services.Document,
		cfg.ControlTower.MaxDownstreamFetchLimit,
	)
	return &Handler{
		log:         log,
		service:     NewService(cfg, client),
		client:      client,
		authEnabled: cfg.AuthEnabled,
		devTenantID: strings.TrimSpace(cfg.DevTenantID),
	}
}

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	started := time.Now()

	query, err := ParseListQuery(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	if h.authEnabled {
		if err := h.ensureAccess(r, reqCtx); err != nil {
			respond.Error(w, err)
			return
		}
	}

	summary, err := h.service.GetSummary(r.Context(), reqCtx, query)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, summary)

	h.log.Info("control_tower_summary_completed",
		slog.String("request_id", requestIDFromRequest(r)),
		slog.String("tenant_id", reqCtx.TenantID),
		slog.String("user_id", reqCtx.UserID),
		slog.Int64("duration_ms", time.Since(started).Milliseconds()),
		slog.Int("shipments_count", len(summary.Shipments.Items)),
		slog.Int("critical_events_count", len(summary.CriticalEvents)),
		slog.Bool("partial", summary.DataFreshness.Partial),
		slog.Any("warning_codes", summary.DataFreshness.Warnings),
	)
}

func (h *Handler) ensureAccess(r *http.Request, reqCtx RequestContext) error {
	roles, err := h.client.FetchUserRoles(r.Context(), reqCtx)
	if err != nil {
		if strings.Contains(err.Error(), "unauthorized") {
			return apperrors.Unauthorized("invalid or expired token")
		}
		return apperrors.AuthDependencyUnavailable("authentication service is temporarily unavailable")
	}
	if !CanAccessControlTower(roles) {
		return apperrors.Forbidden("control tower access denied")
	}
	return nil
}

func buildRequestContext(r *http.Request, authEnabled bool, devTenantID string) (RequestContext, error) {
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

package controltower

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

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
		service:     NewService(cfg, client, log),
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

func (h *Handler) AcknowledgeCriticalEvent(w http.ResponseWriter, r *http.Request) {
	started := time.Now()

	eventID := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "eventId")))
	rawBody, err := readAcknowledgeRequestBody(r)
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"field": "body"}))
		return
	}

	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	if h.authEnabled {
		if err := h.ensureAcknowledgeAccess(r, reqCtx); err != nil {
			respond.Error(w, err)
			return
		}
	}

	result, err := h.service.AcknowledgeCriticalEvent(r.Context(), reqCtx, eventID, rawBody)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, result)

	h.log.Info("control_tower_acknowledge_completed",
		slog.String("request_id", requestIDFromRequest(r)),
		slog.String("tenant_id", reqCtx.TenantID),
		slog.String("user_id", reqCtx.UserID),
		slog.String("event_id", eventID),
		slog.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
}

func (h *Handler) AssignCriticalEvent(w http.ResponseWriter, r *http.Request) {
	h.handleWorkflowMutation(w, r, "assign", h.service.AssignCriticalEvent, h.ensureAssignAccess)
}

func (h *Handler) ResolveCriticalEvent(w http.ResponseWriter, r *http.Request) {
	h.handleWorkflowMutation(w, r, "resolve", h.service.ResolveCriticalEvent, h.ensureResolveAccess)
}

func (h *Handler) ReopenCriticalEvent(w http.ResponseWriter, r *http.Request) {
	h.handleWorkflowMutation(w, r, "reopen", h.service.ReopenCriticalEvent, h.ensureAssignAccess)
}

func (h *Handler) GetCriticalEventActions(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	eventID := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "eventId")))

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

	result, err := h.service.GetCriticalEventActions(r.Context(), reqCtx, eventID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, result)

	h.log.Info("control_tower_actions_list_completed",
		slog.String("request_id", requestIDFromRequest(r)),
		slog.String("tenant_id", reqCtx.TenantID),
		slog.String("user_id", reqCtx.UserID),
		slog.String("event_id", eventID),
		slog.Int("actions_count", len(result.Items)),
		slog.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
}

type workflowMutation func(context.Context, RequestContext, string, []byte) (ControlTowerEventWorkflow, error)
type accessChecker func(*http.Request, RequestContext) error

func (h *Handler) handleWorkflowMutation(
	w http.ResponseWriter,
	r *http.Request,
	action string,
	mutate workflowMutation,
	check accessChecker,
) {
	started := time.Now()
	eventID := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "eventId")))
	rawBody, err := readWorkflowRequestBody(r)
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"field": "body"}))
		return
	}

	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	if h.authEnabled {
		if err := check(r, reqCtx); err != nil {
			respond.Error(w, err)
			return
		}
	}

	result, err := mutate(r.Context(), reqCtx, eventID, rawBody)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, result)

	h.log.Info("control_tower_"+action+"_completed",
		slog.String("request_id", requestIDFromRequest(r)),
		slog.String("tenant_id", reqCtx.TenantID),
		slog.String("user_id", reqCtx.UserID),
		slog.String("event_id", eventID),
		slog.String("status", result.Status),
		slog.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
}

func (h *Handler) ensureAcknowledgeAccess(r *http.Request, reqCtx RequestContext) error {
	return h.ensureRoleAccess(r, reqCtx, CanAcknowledgeControlTower, "acknowledge control tower events denied")
}

func (h *Handler) ensureAssignAccess(r *http.Request, reqCtx RequestContext) error {
	return h.ensureRoleAccess(r, reqCtx, CanAssignControlTower, "assign control tower events denied")
}

func (h *Handler) ensureResolveAccess(r *http.Request, reqCtx RequestContext) error {
	return h.ensureRoleAccess(r, reqCtx, CanResolveControlTower, "resolve control tower events denied")
}

func (h *Handler) ensureRoleAccess(
	r *http.Request,
	reqCtx RequestContext,
	check func([]string) bool,
	deniedMessage string,
) error {
	roles, err := h.client.FetchUserRoles(r.Context(), reqCtx)
	if err != nil {
		if strings.Contains(err.Error(), "unauthorized") {
			return apperrors.Unauthorized("invalid or expired token")
		}
		return apperrors.AuthDependencyUnavailable("authentication service is temporarily unavailable")
	}
	if !check(roles) {
		return apperrors.Forbidden(deniedMessage)
	}
	return nil
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

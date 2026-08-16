package driver

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/api-gateway/internal/config"
	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
	"github.com/freight-platform/api-gateway/internal/document"
	"github.com/freight-platform/api-gateway/internal/driverrbac"
	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
	"github.com/freight-platform/api-gateway/internal/routeauth"
	"github.com/freight-platform/api-gateway/internal/tracking"
	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

type Handler struct {
	log         *slog.Logger
	client      *Client
	tracking    *tracking.Client
	documents   *document.Client
	integrator  *ExceptionIntegrator
	identity    *routeauth.IdentityClient
	authEnabled bool
	devTenantID string
}

func NewHandler(log *slog.Logger, cfg config.Config) *Handler {
	httpClient := &http.Client{Timeout: time.Duration(cfg.ProxyTimeoutSeconds) * time.Second}
	readModel := controltowerreadmodel.NewClient(httpClient, cfg.ControlTower.ReadModel, controltowerreadmodel.NewMetrics())
	return &Handler{
		log: log,
		client: NewClient(httpClient, cfg.Services.Shipment),
		tracking: tracking.NewClient(httpClient, cfg.Services.Tracking, cfg.TrackingInternalToken),
		documents: document.NewClient(httpClient, cfg.Services.Document),
		integrator: NewExceptionIntegrator(
			readModel,
			cfg.ControlTower.ReadModel.Mode.Enabled(),
			cfg.ControlTower.ReadModel.Timeout,
		),
		identity:    routeauth.NewIdentityClient(httpClient, cfg.Services.Identity),
		authEnabled: cfg.AuthEnabled,
		devTenantID: strings.TrimSpace(cfg.DevTenantID),
	}
}

func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, func(ctx RequestContext) (json.RawMessage, int, error) {
		return h.client.GetMe(r.Context(), ctx)
	})
}

func (h *Handler) ListShipments(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, func(ctx RequestContext) (json.RawMessage, int, error) {
		return h.client.ListShipments(r.Context(), ctx, r.URL.RawQuery)
	})
}

func (h *Handler) GetShipment(w http.ResponseWriter, r *http.Request) {
	shipmentID := strings.TrimSpace(chi.URLParam(r, "shipmentId"))
	h.proxy(w, r, func(ctx RequestContext) (json.RawMessage, int, error) {
		return h.client.GetShipment(r.Context(), ctx, shipmentID)
	})
}

func (h *Handler) RecordEvent(w http.ResponseWriter, r *http.Request) {
	shipmentID := strings.TrimSpace(chi.URLParam(r, "shipmentId"))
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		respond.Error(w, err)
		return
	}
	h.proxy(w, r, func(ctx RequestContext) (json.RawMessage, int, error) {
		return h.client.RecordEvent(r.Context(), ctx, shipmentID, body)
	})
}

func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, func(ctx RequestContext) (json.RawMessage, int, error) {
		return h.client.ListTasks(r.Context(), ctx, r.URL.RawQuery)
	})
}

func (h *Handler) GetTask(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	h.proxy(w, r, func(ctx RequestContext) (json.RawMessage, int, error) {
		return h.client.GetTask(r.Context(), ctx, taskID)
	})
}

func (h *Handler) MarkTaskRead(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	h.proxy(w, r, func(ctx RequestContext) (json.RawMessage, int, error) {
		return h.client.MarkTaskRead(r.Context(), ctx, taskID)
	})
}

func (h *Handler) AcknowledgeTask(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	h.proxy(w, r, func(ctx RequestContext) (json.RawMessage, int, error) {
		return h.client.AcknowledgeTask(r.Context(), ctx, taskID)
	})
}

func (h *Handler) SubmitTaskResponse(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		respond.Error(w, err)
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	h.proxy(w, r, func(ctx RequestContext) (json.RawMessage, int, error) {
		return h.client.SubmitTaskResponse(r.Context(), ctx, taskID, body, idem)
	})
}

func (h *Handler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		respond.Error(w, err)
		return
	}
	h.proxy(w, r, func(ctx RequestContext) (json.RawMessage, int, error) {
		return h.client.RegisterDevice(r.Context(), ctx, body)
	})
}

func (h *Handler) RevokeDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(chi.URLParam(r, "deviceId"))
	h.proxy(w, r, func(ctx RequestContext) (json.RawMessage, int, error) {
		return h.client.RevokeDevice(r.Context(), ctx, deviceID)
	})
}

func (h *Handler) ReportException(w http.ResponseWriter, r *http.Request) {
	shipmentID := strings.TrimSpace(chi.URLParam(r, "shipmentId"))
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		respond.Error(w, err)
		return
	}
	reqCtx, err := h.buildRequestContext(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if err := h.ensureDriverAccess(r, reqCtx); err != nil {
		respond.Error(w, err)
		return
	}
	raw, status, err := h.client.ReportException(r.Context(), reqCtx, shipmentID, body)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if status >= 200 && status < 300 {
		h.integrateException(r, reqCtx, raw)
	}
	respond.JSONRaw(w, status, raw)
}

func (h *Handler) IngestLocation(w http.ResponseWriter, r *http.Request) {
	reqCtx, err := h.buildRequestContext(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if err := h.ensureDriverAccess(r, reqCtx); err != nil {
		respond.Error(w, err)
		return
	}
	shipmentID := strings.TrimSpace(chi.URLParam(r, "shipmentId"))
	if _, status, err := h.client.GetShipment(r.Context(), reqCtx, shipmentID); err != nil || status == http.StatusNotFound {
		respond.Error(w, apperrors.NotFound("shipment not found"))
		return
	} else if status >= 400 {
		respond.Error(w, apperrors.ServiceUnavailable("shipment service is temporarily unavailable", "shipment-service"))
		return
	}
	meRaw, status, err := h.client.GetMe(r.Context(), reqCtx)
	if err != nil || status >= 400 {
		respond.Error(w, apperrors.Unauthorized("driver identity is not configured"))
		return
	}
	var me struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(meRaw, &me); err != nil || strings.TrimSpace(me.ID) == "" {
		respond.Error(w, apperrors.Unauthorized("driver identity is not configured"))
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		respond.Error(w, err)
		return
	}
	raw, status, err := h.tracking.IngestDriverLocation(r.Context(), reqCtx.TenantID, reqCtx.RequestID, shipmentID, me.ID, "", body)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSONRaw(w, status, raw)
}

func (h *Handler) proxy(w http.ResponseWriter, r *http.Request, call func(RequestContext) (json.RawMessage, int, error)) {
	reqCtx, err := h.buildRequestContext(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if err := h.ensureDriverAccess(r, reqCtx); err != nil {
		respond.Error(w, err)
		return
	}
	raw, status, err := call(reqCtx)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSONRaw(w, status, raw)
}

func (h *Handler) ReportDelay(w http.ResponseWriter, r *http.Request) {
	shipmentID := strings.TrimSpace(chi.URLParam(r, "shipmentId"))
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		respond.Error(w, err)
		return
	}
	reqCtx, err := h.buildRequestContext(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if err := h.ensureDriverAccess(r, reqCtx); err != nil {
		respond.Error(w, err)
		return
	}
	raw, status, err := h.client.ReportDelay(r.Context(), reqCtx, shipmentID, body)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSONRaw(w, status, raw)
}

func (h *Handler) integrateException(r *http.Request, reqCtx RequestContext, raw json.RawMessage) {
	var payload struct {
		ID         string `json:"id"`
		ShipmentID string `json:"shipmentId"`
		Category   string `json:"category"`
		OccurredAt string `json:"occurredAt"`
		Replayed   bool   `json:"replayed"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		h.log.Warn("driver exception integration skipped: invalid payload")
		return
	}
	occurredAt, err := time.Parse(time.RFC3339, payload.OccurredAt)
	if err != nil {
		occurredAt = time.Now().UTC()
	}
	if err := h.integrator.Integrate(r.Context(), reqCtx, ExceptionIntegrationInput{
		ExceptionID: payload.ID,
		ShipmentID:  payload.ShipmentID,
		Category:    payload.Category,
		OccurredAt:  occurredAt,
		Replayed:    payload.Replayed,
	}); err != nil {
		h.log.Warn("driver exception control tower integration failed", slog.String("error", err.Error()))
	}
}

func (h *Handler) ensureDriverAccess(r *http.Request, reqCtx RequestContext) error {
	if !h.authEnabled {
		return nil
	}
	roles, err := h.identity.FetchUserRoles(r.Context(), routeauth.RequestContext{
		TenantID:  reqCtx.TenantID,
		UserID:    reqCtx.UserID,
		AuthToken: reqCtx.AuthToken,
		RequestID: reqCtx.RequestID,
	})
	if err != nil {
		return err
	}
	if !driverrbac.CanAccessDriverRoutes(roles) {
		return apperrors.Forbidden("driver access denied")
	}
	return nil
}

func (h *Handler) buildRequestContext(r *http.Request) (RequestContext, error) {
	if h.authEnabled {
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
	if h.devTenantID == "" {
		return RequestContext{}, apperrors.Unauthorized("development tenant context is not configured")
	}
	return RequestContext{
		TenantID:  h.devTenantID,
		UserID:    strings.TrimSpace(r.Header.Get("X-User-ID")),
		AuthToken: strings.TrimSpace(r.Header.Get("Authorization")),
		RequestID: requestIDFromRequest(r),
	}, nil
}

func requestIDFromRequest(r *http.Request) string {
	if id := sharedmiddleware.RequestIDFromContext(r.Context()); id != "" {
		return id
	}
	return strings.TrimSpace(r.Header.Get(sharedmiddleware.RequestIDHeader))
}

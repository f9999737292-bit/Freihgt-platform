package tracking

import (
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
	client      *Client
	authEnabled bool
	devTenantID string
}

func NewHandler(log *slog.Logger, cfg config.Config) *Handler {
	httpClient := &http.Client{Timeout: time.Duration(cfg.ProxyTimeoutSeconds) * time.Second}
	return &Handler{
		log: log,
		client: NewClient(
			httpClient,
			cfg.Services.Tracking,
			cfg.TrackingInternalToken,
		),
		authEnabled: cfg.AuthEnabled,
		devTenantID: strings.TrimSpace(cfg.DevTenantID),
	}
}

func (h *Handler) GetCurrent(w http.ResponseWriter, r *http.Request) {
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	shipmentID := strings.TrimSpace(chi.URLParam(r, "shipmentId"))
	if shipmentID == "" {
		respond.Error(w, apperrors.Validation("shipment id is required", nil))
		return
	}
	summary, err := h.client.GetCurrent(r.Context(), reqCtx.TenantID, reqCtx.RequestID, shipmentID)
	if err != nil {
		respond.Error(w, apperrors.ServiceUnavailable("tracking service is temporarily unavailable", "tracking-service"))
		return
	}
	if summary == nil {
		respond.Error(w, apperrors.NotFound("tracking not found"))
		return
	}
	respond.JSON(w, http.StatusOK, summary)
}

func (h *Handler) GetETA(w http.ResponseWriter, r *http.Request) {
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	shipmentID := strings.TrimSpace(chi.URLParam(r, "shipmentId"))
	if shipmentID == "" {
		respond.Error(w, apperrors.Validation("shipment id is required", nil))
		return
	}
	summary, err := h.client.GetETA(r.Context(), reqCtx.TenantID, reqCtx.RequestID, shipmentID, r.URL.RawQuery)
	if err != nil {
		respond.Error(w, apperrors.ServiceUnavailable("tracking service is temporarily unavailable", "tracking-service"))
		return
	}
	if summary == nil {
		respond.Error(w, apperrors.NotFound("eta not found"))
		return
	}
	respond.JSON(w, http.StatusOK, summary)
}

func (h *Handler) ListETAHistory(w http.ResponseWriter, r *http.Request) {
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	shipmentID := strings.TrimSpace(chi.URLParam(r, "shipmentId"))
	if shipmentID == "" {
		respond.Error(w, apperrors.Validation("shipment id is required", nil))
		return
	}
	path := "/v1/shipments/" + shipmentID + "/eta/history"
	body, status, err := h.client.ProxyJSON(r.Context(), reqCtx.TenantID, reqCtx.RequestID, http.MethodGet, path, r.URL.RawQuery)
	if err != nil {
		respond.Error(w, apperrors.ServiceUnavailable("tracking service is temporarily unavailable", "tracking-service"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (h *Handler) GetSlots(w http.ResponseWriter, r *http.Request) {
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	shipmentID := strings.TrimSpace(chi.URLParam(r, "shipmentId"))
	if shipmentID == "" {
		respond.Error(w, apperrors.Validation("shipment id is required", nil))
		return
	}
	summary, err := h.client.GetSlots(r.Context(), reqCtx.TenantID, reqCtx.RequestID, shipmentID, r.URL.RawQuery)
	if err != nil {
		respond.Error(w, apperrors.ServiceUnavailable("tracking service is temporarily unavailable", "tracking-service"))
		return
	}
	if summary == nil {
		respond.Error(w, apperrors.NotFound("slots not found"))
		return
	}
	respond.JSON(w, http.StatusOK, summary)
}

func (h *Handler) ListSlotHistory(w http.ResponseWriter, r *http.Request) {
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	shipmentID := strings.TrimSpace(chi.URLParam(r, "shipmentId"))
	if shipmentID == "" {
		respond.Error(w, apperrors.Validation("shipment id is required", nil))
		return
	}
	path := "/v1/shipments/" + shipmentID + "/slots/history"
	body, status, err := h.client.ProxyJSON(r.Context(), reqCtx.TenantID, reqCtx.RequestID, http.MethodGet, path, r.URL.RawQuery)
	if err != nil {
		respond.Error(w, apperrors.ServiceUnavailable("tracking service is temporarily unavailable", "tracking-service"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (h *Handler) ListLocations(w http.ResponseWriter, r *http.Request) {
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	shipmentID := strings.TrimSpace(chi.URLParam(r, "shipmentId"))
	if shipmentID == "" {
		respond.Error(w, apperrors.Validation("shipment id is required", nil))
		return
	}
	path := "/v1/shipments/" + shipmentID + "/tracking/locations"
	body, status, err := h.client.ProxyJSON(r.Context(), reqCtx.TenantID, reqCtx.RequestID, http.MethodGet, path, r.URL.RawQuery)
	if err != nil {
		respond.Error(w, apperrors.ServiceUnavailable("tracking service is temporarily unavailable", "tracking-service"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

type requestContext struct {
	TenantID  string
	UserID    string
	RequestID string
}

func buildRequestContext(r *http.Request, authEnabled bool, devTenantID string) (requestContext, error) {
	if authEnabled {
		ac, err := gwmiddleware.MustAuthContext(r.Context())
		if err != nil {
			return requestContext{}, apperrors.Unauthorized("verified tenant context is required")
		}
		return requestContext{
			TenantID:  ac.TenantID,
			UserID:    ac.UserID,
			RequestID: requestIDFromRequest(r),
		}, nil
	}
	if devTenantID == "" {
		return requestContext{}, apperrors.Unauthorized("development tenant context is not configured")
	}
	return requestContext{
		TenantID:  devTenantID,
		RequestID: requestIDFromRequest(r),
	}, nil
}

func requestIDFromRequest(r *http.Request) string {
	if id := sharedmiddleware.RequestIDFromContext(r.Context()); id != "" {
		return id
	}
	return strings.TrimSpace(r.Header.Get(sharedmiddleware.RequestIDHeader))
}

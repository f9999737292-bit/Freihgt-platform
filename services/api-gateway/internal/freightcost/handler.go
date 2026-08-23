package freightcost

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/freight-platform/api-gateway/internal/config"
	"github.com/freight-platform/api-gateway/internal/freightcostrbac"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

type Handler struct {
	log    *slog.Logger
	client *Client
}

func NewHandler(log *slog.Logger, cfg config.Config) *Handler {
	httpClient := &http.Client{Timeout: time.Duration(cfg.ProxyTimeoutSeconds) * time.Second}
	return &Handler{
		log: log,
		client: NewClient(
			httpClient,
			cfg.Services.FreightCost,
			cfg.InternalServiceToken,
		),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == "/api/v1/freight-costs" {
		path = "/api/v1/freight-costs"
	}
	if !isAllowlistedPublicPath(r.Method, path) {
		respond.Error(w, apperrors.RouteNotFound("no route found for path"))
		return
	}

	internalPath, ok := mapPublicToInternalPath(path)
	if !ok {
		respond.Error(w, apperrors.RouteNotFound("no route found for path"))
		return
	}

	vc, ok := freightcostrbac.VerifiedContextFromContext(r.Context())
	if !ok {
		respond.Error(w, apperrors.Unauthorized("verified company context is required"))
		return
	}

	requestID := sharedmiddleware.RequestIDFromContext(r.Context())
	if requestID == "" {
		requestID = strings.TrimSpace(r.Header.Get(sharedmiddleware.RequestIDHeader))
	}

	status, respBody, err := h.client.Forward(r.Context(), ForwardInput{
		Method:       r.Method,
		InternalPath: internalPath,
		Query:        r.URL.RawQuery,
		TenantID:     vc.TenantID,
		UserID:       vc.UserID,
		CompanyID:    vc.CompanyID,
		ActorKind:    vc.ActorKind,
		RequestID:    requestID,
	})
	if err != nil {
		respond.Error(w, apperrors.ServiceUnavailable("freight-cost service is temporarily unavailable", "freight-cost-service"))
		return
	}

	if status == http.StatusUnauthorized && looksLikeInternalAuthFailure(respBody) {
		respond.Error(w, apperrors.ServiceUnavailable("freight-cost service is temporarily unavailable", "freight-cost-service"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if len(respBody) > 0 {
		_, _ = w.Write(respBody)
	}
}

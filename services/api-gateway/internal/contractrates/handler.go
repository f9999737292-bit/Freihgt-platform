package contractrates

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/freight-platform/api-gateway/internal/config"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
	"github.com/freight-platform/api-gateway/internal/ratesrbac"
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
			cfg.Services.ContractRate,
			cfg.InternalServiceToken,
		),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	if !isAllowlistedPublicPath(r.Method, path) {
		respond.Error(w, apperrors.RouteNotFound("no route found for path"))
		return
	}

	internalPath, ok := mapPublicToInternalPath(path)
	if !ok {
		respond.Error(w, apperrors.RouteNotFound("no route found for path"))
		return
	}

	vc, ok := ratesrbac.VerifiedContextFromContext(r.Context())
	if !ok {
		respond.Error(w, apperrors.Unauthorized("verified company context is required"))
		return
	}

	var body []byte
	if r.Body != nil && r.Method != http.MethodGet && r.Method != http.MethodDelete {
		raw, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
		if err != nil {
			respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"field": "body"}))
			return
		}
		body, err = validateAndRebuildBody(r.Method, path, raw, vc)
		if err != nil {
			respond.Error(w, err)
			return
		}
	}

	requestID := sharedmiddleware.RequestIDFromContext(r.Context())
	if requestID == "" {
		requestID = strings.TrimSpace(r.Header.Get(sharedmiddleware.RequestIDHeader))
	}

	status, respBody, err := h.client.Forward(r.Context(), ForwardInput{
		Method:       r.Method,
		InternalPath: internalPath,
		Query:        r.URL.RawQuery,
		Body:         body,
		TenantID:     vc.TenantID,
		UserID:       vc.UserID,
		CompanyID:    vc.CompanyID,
		ActorKind:    vc.ActorKind,
		RequestID:    requestID,
	})
	if err != nil {
		respond.Error(w, apperrors.ServiceUnavailable("contract-rate service is temporarily unavailable", "contract-rate-service"))
		return
	}

	if status == http.StatusUnauthorized && looksLikeInternalAuthFailure(respBody) {
		respond.Error(w, apperrors.ServiceUnavailable("contract-rate service is temporarily unavailable", "contract-rate-service"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if len(respBody) > 0 {
		_, _ = w.Write(respBody)
	}
}

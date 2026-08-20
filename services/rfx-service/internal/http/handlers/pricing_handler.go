package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/platform/respond"
	"github.com/freight-platform/rfx-service/internal/service"
)

type PricingHandler struct {
	pricing *service.PricingService
}

func NewPricingHandler(pricing *service.PricingService) *PricingHandler {
	return &PricingHandler{pricing: pricing}
}

func (h *PricingHandler) GetAwardLinkContext(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseTrustedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	linkID, err := uuid.Parse(chi.URLParam(r, "sourceId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid award link id", map[string]any{"field": "sourceId"}))
		return
	}
	ctx, err := h.pricing.GetAwardLinkContext(r.Context(), tenantID, linkID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, ctx)
}

func (h *PricingHandler) GetAwardScopeContext(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseTrustedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid event id", map[string]any{"field": "eventId"}))
		return
	}
	var lotID *uuid.UUID
	if raw := r.URL.Query().Get("lot_id"); raw != "" {
		id, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			respond.Error(w, apperrors.Validation("invalid lot_id", map[string]any{"field": "lot_id"}))
			return
		}
		lotID = &id
	}
	ctx, err := h.pricing.GetAwardScopeContext(r.Context(), tenantID, eventID, lotID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, ctx)
}

func (h *PricingHandler) GetBidContext(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseTrustedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	bidID, err := uuid.Parse(chi.URLParam(r, "bidId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid bid id", map[string]any{"field": "bidId"}))
		return
	}
	ctx, err := h.pricing.GetAcceptedBidContext(r.Context(), tenantID, bidID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, ctx)
}

func parseTrustedTenant(r *http.Request) (uuid.UUID, error) {
	raw := r.Header.Get("X-Tenant-ID")
	if raw == "" {
		return uuid.Nil, apperrors.Unauthorized("tenant context is required")
	}
	tenantID, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperrors.Validation("invalid tenant id", map[string]any{"field": "tenant_id"})
	}
	return tenantID, nil
}

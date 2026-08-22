package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/http/dto"
	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
	"github.com/freight-platform/freight-cost-service/internal/platform/respond"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

type SourceEventHandler struct {
	ingest  *service.IngestService
	rebuild *service.RebuildService
}

func NewSourceEventHandler(ingest *service.IngestService, rebuild *service.RebuildService) *SourceEventHandler {
	return &SourceEventHandler{ingest: ingest, rebuild: rebuild}
}

func (h *SourceEventHandler) PostSourceEvent(w http.ResponseWriter, r *http.Request) {
	tenantID, err := ParseTrustedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var payload dto.SourceEventRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respond.Error(w, apperrors.Validation("invalid json body", map[string]any{"field": "body"}))
		return
	}
	input, err := dto.ToSourceEventInput(payload, tenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.ingest.Ingest(r.Context(), input)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, dto.ToSourceEventResponse(result))
}

func (h *SourceEventHandler) RebuildTransportOrder(w http.ResponseWriter, r *http.Request) {
	tenantID, err := ParseTrustedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	transportOrderID, err := uuid.Parse(chi.URLParam(r, "transportOrderId"))
	if err != nil || transportOrderID == uuid.Nil {
		respond.Error(w, apperrors.Validation("invalid transport order id", map[string]any{"field": "transport_order_id"}))
		return
	}
	result, err := h.rebuild.RebuildTransportOrder(r.Context(), tenantID, transportOrderID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, dto.ToRebuildResponse(result))
}

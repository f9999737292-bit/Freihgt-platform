package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/http/dto"
	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
	"github.com/freight-platform/freight-cost-service/internal/platform/respond"
	"github.com/freight-platform/freight-cost-service/internal/security"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

type CostHandler struct {
	costs *service.CostService
}

func NewCostHandler(costs *service.CostService) *CostHandler {
	return &CostHandler{costs: costs}
}

func (h *CostHandler) GetTransportOrderCostSummary(w http.ResponseWriter, r *http.Request) {
	actor, err := security.ParseTrustedActor(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	transportOrderID, err := uuid.Parse(chi.URLParam(r, "transportOrderId"))
	if err != nil || transportOrderID == uuid.Nil {
		respond.Error(w, apperrors.Validation("invalid transport order id", map[string]any{"field": "transport_order_id"}))
		return
	}

	if _, err := ParseTrustedTenant(r); err != nil {
		respond.Error(w, err)
		return
	}

	summary, err := h.costs.GetCostSummaryByTransportOrder(r.Context(), actor, transportOrderID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, dto.ToCostSummaryResponse(summary))
}

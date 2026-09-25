package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
	"github.com/freight-platform/transport-order-service/internal/platform/respond"
)

type planningReader interface {
	PlanningLocations(ctx context.Context, tenantID, id uuid.UUID) (uuid.UUID, uuid.UUID, error)
}

type PlanningInternalHandler struct {
	orders planningReader
}

func NewPlanningInternalHandler(orders planningReader) *PlanningInternalHandler {
	return &PlanningInternalHandler{orders: orders}
}

func (h *PlanningInternalHandler) GetTransportOrder(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "transportOrderId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid transport order id", map[string]any{"field": "transport_order_id"}))
		return
	}
	origin, destination, err := h.orders.PlanningLocations(r.Context(), tenantID, id)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{
		"id":                      id.String(),
		"tenant_id":               tenantID.String(),
		"origin_location_id":      origin.String(),
		"destination_location_id": destination.String(),
	})
}

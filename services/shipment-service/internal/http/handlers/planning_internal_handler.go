package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
)

type planningReader interface {
	PlanningLocations(ctx context.Context, tenantID, id uuid.UUID) (uuid.UUID, uuid.UUID, error)
}

type PlanningInternalHandler struct {
	shipments planningReader
}

func NewPlanningInternalHandler(shipments planningReader) *PlanningInternalHandler {
	return &PlanningInternalHandler{shipments: shipments}
}

func (h *PlanningInternalHandler) GetShipment(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	id, err := domain.ParseUUID(chi.URLParam(r, "shipmentId"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	origin, destination, err := h.shipments.PlanningLocations(r.Context(), tenantID, id)
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

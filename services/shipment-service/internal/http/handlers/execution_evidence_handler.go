package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
)

type executionEvidenceReader interface {
	ExecutionContext(ctx context.Context, tenantID, shipmentID uuid.UUID) (domain.ShipmentExecutionContext, error)
	OnboardCargo(ctx context.Context, tenantID, shipmentID uuid.UUID) (domain.OnboardCargoView, error)
}

type ExecutionEvidenceHandler struct {
	reader executionEvidenceReader
}

func NewExecutionEvidenceHandler(reader executionEvidenceReader) *ExecutionEvidenceHandler {
	return &ExecutionEvidenceHandler{reader: reader}
}

func (h *ExecutionEvidenceHandler) GetExecutionContext(w http.ResponseWriter, r *http.Request) {
	tenantID, id, ok := h.authorizedShipment(w, r)
	if !ok {
		return
	}
	view, err := h.reader.ExecutionContext(r.Context(), tenantID, id)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"shipment_id":             view.ShipmentID,
		"tenant_id":               view.TenantID,
		"shipment_version":        view.ShipmentVersion,
		"shipment_status":         view.ShipmentStatus,
		"vehicle_id":              view.VehicleID,
		"origin_location_id":      view.OriginLocationID,
		"destination_location_id": view.DestinationLocationID,
		"cargo_id":                view.CargoID,
		"planned_delivery_at":     view.PlannedDeliveryAt,
		"actual_pickup_at":        view.ActualPickupAt,
		"actual_delivery_at":      view.ActualDeliveryAt,
	})
}

func (h *ExecutionEvidenceHandler) GetOnboardCargo(w http.ResponseWriter, r *http.Request) {
	tenantID, id, ok := h.authorizedShipment(w, r)
	if !ok {
		return
	}
	view, err := h.reader.OnboardCargo(r.Context(), tenantID, id)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]map[string]any, 0, len(view.Items))
	for _, item := range view.Items {
		row := map[string]any{
			"cargo_id":          item.CargoID,
			"state":             item.State,
			"state_version":     item.StateVersion,
			"occurred_at":       item.OccurredAt,
			"source":            item.Source,
			"source_event_type": item.SourceEventType,
		}
		if item.DriverID != nil {
			row["driver_id"] = item.DriverID
		}
		items = append(items, row)
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"shipment_id":      view.ShipmentID,
		"shipment_version": view.ShipmentVersion,
		"resolution":       view.Resolution,
		"items":            items,
	})
}

func (h *ExecutionEvidenceHandler) authorizedShipment(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return uuid.Nil, uuid.Nil, false
	}
	id, err := domain.ParseUUID(chi.URLParam(r, "shipmentId"), "id")
	if err != nil {
		respond.Error(w, err)
		return uuid.Nil, uuid.Nil, false
	}
	return tenantID, id, true
}

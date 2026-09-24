package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
)

type vehicleCapabilityReader interface {
	GetByIDAndTenant(ctx context.Context, tenantID, id uuid.UUID) (*domain.Vehicle, error)
}

type VehicleCapabilityHandler struct {
	reader vehicleCapabilityReader
}

func NewVehicleCapabilityHandler(reader vehicleCapabilityReader) *VehicleCapabilityHandler {
	return &VehicleCapabilityHandler{reader: reader}
}

func (h *VehicleCapabilityHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	vehicle, err := h.reader.GetByIDAndTenant(r.Context(), tenantID, id)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"id":                              vehicle.ID,
		"tenant_id":                       vehicle.TenantID,
		"version":                         vehicle.Version,
		"equipment_type":                  vehicle.EquipmentType,
		"combination_type":                vehicle.CombinationType,
		"body_type":                       vehicle.BodyType,
		"loading_access":                  vehicle.LoadingAccess,
		"unloading_access":                vehicle.UnloadingAccess,
		"capacity_weight":                 vehicle.CapacityWeight,
		"capacity_volume":                 vehicle.CapacityVolume,
		"temperature_control_mode":        vehicle.TemperatureControlMode,
		"temperature_capability_min_c":    vehicle.TemperatureCapabilityMinC,
		"temperature_capability_max_c":    vehicle.TemperatureCapabilityMaxC,
		"temperature_zone_count":          vehicle.TemperatureZoneCount,
		"independent_temperature_control": vehicle.IndependentTemperatureControl,
		"container_size":                  vehicle.ContainerSize,
	})
}

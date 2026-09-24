package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
	"github.com/freight-platform/shipment-service/internal/service"
)

type VehicleHandler struct {
	service *service.VehicleService
}

func NewVehicleHandler(svc *service.VehicleService) *VehicleHandler {
	return &VehicleHandler{service: svc}
}

type createVehicleRequest struct {
	CarrierCompanyID              string   `json:"carrier_company_id"`
	PlateNumber                   string   `json:"plate_number"`
	VehicleType                   string   `json:"vehicle_type"`
	EquipmentType                 *string  `json:"equipment_type"`
	CapacityWeight                *float64 `json:"capacity_weight"`
	CapacityVolume                *float64 `json:"capacity_volume"`
	RegistrationCountry           string   `json:"registration_country"`
	CombinationType               *string  `json:"combination_type"`
	BodyType                      *string  `json:"body_type"`
	LoadingAccess                 []string `json:"loading_access"`
	UnloadingAccess               []string `json:"unloading_access"`
	TemperatureControlMode        *string  `json:"temperature_control_mode"`
	TemperatureCapabilityMinC     *float64 `json:"temperature_capability_min_c"`
	TemperatureCapabilityMaxC     *float64 `json:"temperature_capability_max_c"`
	TemperatureZoneCount          *int     `json:"temperature_zone_count"`
	IndependentTemperatureControl *bool    `json:"independent_temperature_control"`
	ContainerSize                 *string  `json:"container_size"`
}

func (h *VehicleHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req createVehicleRequest
	if err := decodeStrictJSON(r, &req); err != nil {
		respond.Error(w, err)
		return
	}
	input, err := parseCreateVehicleRequest(req)
	if err != nil {
		respond.Error(w, err)
		return
	}
	vehicle, err := h.service.Create(r.Context(), tenantID, input)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toVehicleResponse(vehicle))
}

func (h *VehicleHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	vehicle, err := h.service.GetByIDAndTenant(r.Context(), tenantID, id)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toVehicleResponse(vehicle))
}

func (h *VehicleHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	filter := domain.ListVehiclesFilter{
		Limit:  parseLimit(r),
		Offset: parseOffset(r),
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("carrier_company_id")); raw != "" {
		id, err := domain.ParseUUID(raw, "carrier_company_id")
		if err != nil {
			respond.Error(w, err)
			return
		}
		filter.CarrierCompanyID = &id
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		filter.Status = &raw
	}

	vehicles, total, err := h.service.List(r.Context(), tenantID, filter)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]map[string]any, 0, len(vehicles))
	for i := range vehicles {
		items = append(items, toVehicleResponse(&vehicles[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func parseCreateVehicleRequest(req createVehicleRequest) (domain.CreateVehicleInput, error) {
	carrierCompanyID, err := domain.ParseUUID(req.CarrierCompanyID, "carrier_company_id")
	if err != nil {
		return domain.CreateVehicleInput{}, err
	}
	return domain.CreateVehicleInput{
		CarrierCompanyID:              carrierCompanyID,
		PlateNumber:                   req.PlateNumber,
		VehicleType:                   req.VehicleType,
		EquipmentType:                 req.EquipmentType,
		CapacityWeight:                req.CapacityWeight,
		CapacityVolume:                req.CapacityVolume,
		RegistrationCountry:           req.RegistrationCountry,
		CombinationType:               req.CombinationType,
		BodyType:                      req.BodyType,
		LoadingAccess:                 req.LoadingAccess,
		UnloadingAccess:               req.UnloadingAccess,
		TemperatureControlMode:        req.TemperatureControlMode,
		TemperatureCapabilityMinC:     req.TemperatureCapabilityMinC,
		TemperatureCapabilityMaxC:     req.TemperatureCapabilityMaxC,
		TemperatureZoneCount:          req.TemperatureZoneCount,
		IndependentTemperatureControl: req.IndependentTemperatureControl,
		ContainerSize:                 req.ContainerSize,
	}, nil
}

func toVehicleResponse(v *domain.Vehicle) map[string]any {
	return map[string]any{
		"id":                              v.ID.String(),
		"tenant_id":                       v.TenantID.String(),
		"carrier_company_id":              v.CarrierCompanyID.String(),
		"plate_number":                    v.PlateNumber,
		"vehicle_type":                    v.VehicleType,
		"equipment_type":                  v.EquipmentType,
		"capacity_weight":                 v.CapacityWeight,
		"capacity_volume":                 v.CapacityVolume,
		"registration_country":            v.RegistrationCountry,
		"status":                          v.Status,
		"combination_type":                v.CombinationType,
		"body_type":                       v.BodyType,
		"loading_access":                  v.LoadingAccess,
		"unloading_access":                v.UnloadingAccess,
		"temperature_control_mode":        v.TemperatureControlMode,
		"temperature_capability_min_c":    v.TemperatureCapabilityMinC,
		"temperature_capability_max_c":    v.TemperatureCapabilityMaxC,
		"temperature_zone_count":          v.TemperatureZoneCount,
		"independent_temperature_control": v.IndependentTemperatureControl,
		"container_size":                  v.ContainerSize,
		"version":                         v.Version,
	}
}

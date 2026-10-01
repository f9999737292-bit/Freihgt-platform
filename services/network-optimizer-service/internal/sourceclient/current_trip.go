package sourceclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/currenttrip"
	"github.com/freight-platform/network-optimizer-service/internal/predict"
)

func (h *HTTP) ExecutionContext(ctx context.Context, tenant, shipment uuid.UUID) (currenttrip.ShipmentExecution, error) {
	body, err := h.get(ctx, tenant, fmt.Sprintf("%s/internal/v1/shipments/%s/execution-context", h.shipmentURL, shipment))
	if err != nil {
		return currenttrip.ShipmentExecution{}, mapTripErr(err)
	}
	var payload struct {
		ShipmentID            uuid.UUID  `json:"shipment_id"`
		TenantID              uuid.UUID  `json:"tenant_id"`
		ShipmentVersion       int        `json:"shipment_version"`
		ShipmentStatus        string     `json:"shipment_status"`
		VehicleID             *uuid.UUID `json:"vehicle_id"`
		DriverID              *uuid.UUID `json:"driver_id"`
		CarrierCompanyID      *uuid.UUID `json:"carrier_company_id"`
		OriginLocationID      uuid.UUID  `json:"origin_location_id"`
		DestinationLocationID uuid.UUID  `json:"destination_location_id"`
		CargoID               *uuid.UUID `json:"cargo_id"`
		PlannedDeliveryAt     *time.Time `json:"planned_delivery_at"`
		ActualPickupAt        *time.Time `json:"actual_pickup_at"`
		ActualDeliveryAt      *time.Time `json:"actual_delivery_at"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.ShipmentID != shipment || payload.TenantID != tenant {
		return currenttrip.ShipmentExecution{}, currenttrip.ErrUnavailable
	}
	return currenttrip.ShipmentExecution{
		ShipmentID: payload.ShipmentID, TenantID: payload.TenantID, ShipmentVersion: payload.ShipmentVersion,
		ShipmentStatus: payload.ShipmentStatus, VehicleID: payload.VehicleID,
		DriverID: payload.DriverID, CarrierCompanyID: payload.CarrierCompanyID,
		OriginLocationID: payload.OriginLocationID, DestinationLocationID: payload.DestinationLocationID,
		CargoID: payload.CargoID, PlannedDeliveryAt: payload.PlannedDeliveryAt,
		ActualPickupAt: payload.ActualPickupAt, ActualDeliveryAt: payload.ActualDeliveryAt,
	}, nil
}

func (h *HTTP) OnboardCargo(ctx context.Context, tenant, shipment uuid.UUID) (currenttrip.OnboardCargo, error) {
	body, err := h.get(ctx, tenant, fmt.Sprintf("%s/internal/v1/shipments/%s/onboard-cargo", h.shipmentURL, shipment))
	if err != nil {
		return currenttrip.OnboardCargo{}, mapTripErr(err)
	}
	var payload struct {
		ShipmentID      uuid.UUID `json:"shipment_id"`
		ShipmentVersion int       `json:"shipment_version"`
		Resolution      string    `json:"resolution"`
		Items           []struct {
			CargoID         uuid.UUID  `json:"cargo_id"`
			State           string     `json:"state"`
			StateVersion    int        `json:"state_version"`
			OccurredAt      time.Time  `json:"occurred_at"`
			Source          string     `json:"source"`
			SourceEventType string     `json:"source_event_type"`
			DriverID        *uuid.UUID `json:"driver_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.ShipmentID != shipment {
		return currenttrip.OnboardCargo{}, currenttrip.ErrUnavailable
	}
	out := currenttrip.OnboardCargo{ShipmentID: payload.ShipmentID, ShipmentVersion: payload.ShipmentVersion, Resolution: payload.Resolution}
	for _, item := range payload.Items {
		out.Items = append(out.Items, currenttrip.OnboardEvidenceItem{
			CargoID: item.CargoID, State: item.State, StateVersion: item.StateVersion,
			OccurredAt: item.OccurredAt, Source: item.Source, SourceEventType: item.SourceEventType, DriverID: item.DriverID,
		})
	}
	return out, nil
}

func (h *HTTP) CargoPlanningProfile(ctx context.Context, tenant, cargo uuid.UUID) (currenttrip.CargoProfile, error) {
	if h.transportOrderURL == "" {
		return currenttrip.CargoProfile{}, currenttrip.ErrUnavailable
	}
	body, err := h.get(ctx, tenant, fmt.Sprintf("%s/internal/v1/cargoes/%s/planning-profile", h.transportOrderURL, cargo))
	if err != nil {
		return currenttrip.CargoProfile{}, mapTripErr(err)
	}
	var payload struct {
		ID                            uuid.UUID `json:"id"`
		TenantID                      uuid.UUID `json:"tenant_id"`
		Version                       int       `json:"version"`
		CargoTypeCode                 *string   `json:"cargo_type_code"`
		WeightKg                      *float64  `json:"weight_kg"`
		VolumeM3                      *float64  `json:"volume_m3"`
		PalletCount                   *int      `json:"pallet_count"`
		PalletTypeCode                *string   `json:"pallet_type_code"`
		LinearMeters                  *float64  `json:"linear_meters"`
		MaxLoadedHeightMM             *int      `json:"max_loaded_height_mm"`
		Stackable                     *bool     `json:"stackable"`
		Fragile                       *bool     `json:"fragile"`
		PackagingTypeCode             *string   `json:"packaging_type_code"`
		FoodGradeRequired             *bool     `json:"food_grade_required"`
		TemperatureRequired           *bool     `json:"temperature_required"`
		TemperatureMinC               *float64  `json:"temperature_min_c"`
		TemperatureMaxC               *float64  `json:"temperature_max_c"`
		PreferredTemperatureSetpointC *float64  `json:"preferred_temperature_setpoint_c"`
		DangerousGoods                *bool     `json:"dangerous_goods"`
		HazardClasses                 []string  `json:"hazard_classes"`
		OdorEmissionClass             *string   `json:"odor_emission_class"`
		OdorSensitive                 *bool     `json:"odor_sensitive"`
		ContaminationClass            *string   `json:"contamination_class"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.ID != cargo || payload.TenantID != tenant {
		return currenttrip.CargoProfile{}, currenttrip.ErrUnavailable
	}
	return currenttrip.CargoProfile{
		ID: payload.ID, Version: payload.Version, CargoTypeCode: payload.CargoTypeCode,
		WeightKg: payload.WeightKg, VolumeM3: payload.VolumeM3, PalletCount: payload.PalletCount,
		PalletTypeCode: payload.PalletTypeCode, LinearMeters: payload.LinearMeters,
		MaxLoadedHeightMM: payload.MaxLoadedHeightMM, Stackable: payload.Stackable, Fragile: payload.Fragile,
		PackagingTypeCode: payload.PackagingTypeCode, FoodGradeRequired: payload.FoodGradeRequired,
		TemperatureRequired: payload.TemperatureRequired, TemperatureMinC: payload.TemperatureMinC,
		TemperatureMaxC: payload.TemperatureMaxC, PreferredTemperatureSetpointC: payload.PreferredTemperatureSetpointC,
		DangerousGoods: payload.DangerousGoods, HazardClasses: payload.HazardClasses,
		OdorEmissionClass: payload.OdorEmissionClass, OdorSensitive: payload.OdorSensitive,
		ContaminationClass: payload.ContaminationClass,
	}, nil
}

func (h *HTTP) VehicleCapability(ctx context.Context, tenant, id uuid.UUID) (currenttrip.VehicleCapability, error) {
	body, err := h.get(ctx, tenant, fmt.Sprintf("%s/internal/v1/vehicles/%s/capability", h.shipmentURL, id))
	if err != nil {
		return currenttrip.VehicleCapability{}, mapTripErr(err)
	}
	var payload struct {
		ID                            uuid.UUID `json:"id"`
		TenantID                      uuid.UUID `json:"tenant_id"`
		Version                       int       `json:"version"`
		EquipmentType                 *string   `json:"equipment_type"`
		CombinationType               *string   `json:"combination_type"`
		BodyType                      *string   `json:"body_type"`
		EquipmentUnitKind             *string   `json:"equipment_unit_kind"`
		LoadingAccess                 []string  `json:"loading_access"`
		UnloadingAccess               []string  `json:"unloading_access"`
		CapacityWeight                *float64  `json:"capacity_weight"`
		CapacityVolume                *float64  `json:"capacity_volume"`
		PalletPositions               *int      `json:"pallet_positions"`
		UsableLinearMeters            *float64  `json:"usable_linear_meters"`
		InternalLengthMM              *int      `json:"internal_length_mm"`
		InternalWidthMM               *int      `json:"internal_width_mm"`
		InternalHeightMM              *int      `json:"internal_height_mm"`
		TemperatureControlMode        *string   `json:"temperature_control_mode"`
		TemperatureCapabilityMinC     *float64  `json:"temperature_capability_min_c"`
		TemperatureCapabilityMaxC     *float64  `json:"temperature_capability_max_c"`
		TemperatureZoneCount          *int      `json:"temperature_zone_count"`
		IndependentTemperatureControl *bool     `json:"independent_temperature_control"`
		FoodGradeCapability           *bool     `json:"food_grade_capability"`
		ADRCapability                 *bool     `json:"adr_capability"`
		ContainerSize                 *string   `json:"container_size"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.ID != id || payload.TenantID != tenant {
		return currenttrip.VehicleCapability{}, currenttrip.ErrUnavailable
	}
	return currenttrip.VehicleCapability{
		ID: payload.ID, TenantID: payload.TenantID, Version: payload.Version,
		EquipmentType: payload.EquipmentType, CombinationType: payload.CombinationType, BodyType: payload.BodyType,
		EquipmentUnitKind: payload.EquipmentUnitKind, LoadingAccess: payload.LoadingAccess, UnloadingAccess: payload.UnloadingAccess,
		CapacityWeight: payload.CapacityWeight, CapacityVolume: payload.CapacityVolume,
		PalletPositions: payload.PalletPositions, UsableLinearMeters: payload.UsableLinearMeters,
		InternalLengthMM: payload.InternalLengthMM, InternalWidthMM: payload.InternalWidthMM, InternalHeightMM: payload.InternalHeightMM,
		TemperatureControlMode: payload.TemperatureControlMode, TemperatureCapabilityMinC: payload.TemperatureCapabilityMinC,
		TemperatureCapabilityMaxC: payload.TemperatureCapabilityMaxC, TemperatureZoneCount: payload.TemperatureZoneCount,
		IndependentTemperatureControl: payload.IndependentTemperatureControl, FoodGradeCapability: payload.FoodGradeCapability,
		ADRCapability: payload.ADRCapability, ContainerSize: payload.ContainerSize,
	}, nil
}

func (h *HTTP) TrackingState(ctx context.Context, tenant, shipment uuid.UUID) (currenttrip.TrackingPosition, error) {
	raw, err := json.Marshal(map[string]any{"shipmentIds": []string{shipment.String()}})
	if err != nil {
		return currenttrip.TrackingPosition{}, currenttrip.ErrUnavailable
	}
	body, err := h.send(ctx, tenant, http.MethodPost, h.trackingURL+"/internal/v1/tracking/states/lookup", raw)
	if err != nil {
		return currenttrip.TrackingPosition{}, mapTripErr(err)
	}
	var payload struct {
		Items map[string]struct {
			TrackingStatus string `json:"trackingStatus"`
			Freshness      struct {
				Status     string `json:"status"`
				AgeSeconds *int   `json:"ageSeconds"`
			} `json:"freshness"`
			LastKnownPosition *struct {
				Latitude   *float64   `json:"latitude"`
				Longitude  *float64   `json:"longitude"`
				RecordedAt *time.Time `json:"recordedAt"`
			} `json:"lastKnownPosition"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return currenttrip.TrackingPosition{}, currenttrip.ErrUnavailable
	}
	item, ok := payload.Items[shipment.String()]
	if !ok {
		return currenttrip.TrackingPosition{Freshness: "UNKNOWN", TrackingStatus: "UNKNOWN"}, nil
	}
	out := currenttrip.TrackingPosition{TrackingStatus: item.TrackingStatus, Freshness: item.Freshness.Status, AgeSeconds: item.Freshness.AgeSeconds}
	if item.LastKnownPosition != nil {
		out.Latitude = item.LastKnownPosition.Latitude
		out.Longitude = item.LastKnownPosition.Longitude
		out.RecordedAt = item.LastKnownPosition.RecordedAt
	}
	if out.Freshness == "" {
		out.Freshness = "UNKNOWN"
	}
	return out, nil
}

func (h *HTTP) TrackingETA(ctx context.Context, tenant, shipment uuid.UUID) (currenttrip.ETA, error) {
	raw, err := json.Marshal(map[string]any{"shipmentIds": []string{shipment.String()}})
	if err != nil {
		return currenttrip.ETA{}, currenttrip.ErrUnavailable
	}
	body, err := h.send(ctx, tenant, http.MethodPost, h.trackingURL+"/internal/v1/tracking/eta/lookup", raw)
	if err != nil {
		return currenttrip.ETA{}, mapTripErr(err)
	}
	var payload struct {
		Items map[string]struct {
			Status             string     `json:"status"`
			FreshnessStatus    string     `json:"freshnessStatus"`
			AgeSeconds         *int       `json:"ageSeconds"`
			EstimatedArrivalAt *time.Time `json:"estimatedArrivalAt"`
			SourceObservedAt   *time.Time `json:"sourceObservedAt"`
			SourceType         string     `json:"sourceType"`
			Provider           string     `json:"provider"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return currenttrip.ETA{}, currenttrip.ErrUnavailable
	}
	item, ok := payload.Items[shipment.String()]
	if !ok {
		return currenttrip.ETA{Status: "UNKNOWN", FreshnessStatus: "UNKNOWN"}, nil
	}
	return currenttrip.ETA{
		Status: item.Status, FreshnessStatus: item.FreshnessStatus, AgeSeconds: item.AgeSeconds,
		EstimatedArrivalAt: item.EstimatedArrivalAt, SourceObservedAt: item.SourceObservedAt,
		SourceType: item.SourceType, Provider: item.Provider,
	}, nil
}

func mapTripErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, predict.ErrNotFound) {
		return currenttrip.ErrNotFound
	}
	return currenttrip.ErrUnavailable
}

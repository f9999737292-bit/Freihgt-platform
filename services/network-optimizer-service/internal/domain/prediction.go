package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	PredictionMethodRuleBased = "RULE_BASED"
	PredictionRuleVersion     = "bno-predict-0.1b.1"
	UnloadPolicySource        = "BNO_PREDICTION_DEFAULT_UNLOAD_DURATION"

	ReasonETAUnavailable                     = "ETA_UNAVAILABLE"
	ReasonETAStale                           = "ETA_STALE"
	ReasonVehicleUnassigned                  = "VEHICLE_UNASSIGNED"
	ReasonVehicleCapacityUnavailable         = "VEHICLE_CAPACITY_UNAVAILABLE"
	ReasonVehicleFutureAvailabilityAmbiguous = "VEHICLE_FUTURE_AVAILABILITY_AMBIGUOUS"
	ReasonDestinationUnavailable             = "DESTINATION_UNAVAILABLE"
	ReasonUnloadPolicyUnavailable            = "UNLOAD_POLICY_UNAVAILABLE"
	ReasonDeliveryWindowInvalid              = "DELIVERY_WINDOW_INVALID"
	ReasonShipmentNotEligible                = "SHIPMENT_NOT_ELIGIBLE"
	ReasonShipmentCancelled                  = "SHIPMENT_CANCELLED"
	ReasonPredictionSuperseded               = "PREDICTION_SUPERSEDED"
	ReasonPredictionExpired                  = "PREDICTION_EXPIRED"
	ReasonConfidenceBelowFloor               = "CONFIDENCE_BELOW_FLOOR"
	ReasonAvailabilityPolicyUnavailable      = "AVAILABILITY_UNCERTAINTY_POLICY_UNAVAILABLE"
	ReasonETAFreshnessPolicyUnavailable      = "ETA_FRESHNESS_POLICY_UNAVAILABLE"
)

type PredictedCapacity struct {
	ID                            uuid.UUID  `json:"predicted_capacity_id"`
	CapacityID                    uuid.UUID  `json:"capacity_id"`
	OwnerTenantID                 uuid.UUID  `json:"owner_tenant_id"`
	ShipmentID                    uuid.UUID  `json:"shipment_id"`
	ShipmentVersion               int        `json:"shipment_version"`
	ShipmentStatus                string     `json:"shipment_status"`
	VehicleID                     uuid.UUID  `json:"vehicle_id"`
	VehicleVersion                *int       `json:"vehicle_version"`
	DestinationLocationID         uuid.UUID  `json:"destination_location_id"`
	DestinationLatitude           *float64   `json:"destination_latitude"`
	DestinationLongitude          *float64   `json:"destination_longitude"`
	ETA                           time.Time  `json:"eta"`
	ETAObservedAt                 time.Time  `json:"eta_observed_at"`
	DeliveryWindowStart           *time.Time `json:"delivery_window_start"`
	DeliveryWindowEnd             *time.Time `json:"delivery_window_end"`
	PredictedAvailableAt          time.Time  `json:"predicted_available_at"`
	AvailabilityWindowStart       time.Time  `json:"availability_window_start"`
	AvailabilityWindowEnd         time.Time  `json:"availability_window_end"`
	UnloadDurationSeconds         int        `json:"unload_duration_seconds"`
	UnloadPolicySource            string     `json:"unload_policy_source"`
	UncertaintySeconds            int        `json:"uncertainty_seconds"`
	Confidence                    float64    `json:"confidence"`
	PredictionMethod              string     `json:"prediction_method"`
	InputFingerprint              string     `json:"input_fingerprint"`
	RuleVersion                   string     `json:"rule_version"`
	GeneratedAt                   time.Time  `json:"generated_at"`
	SupersedesPredictionID        *uuid.UUID `json:"supersedes_prediction_id"`
	IsCurrent                     bool       `json:"is_current"`
	CombinationType               *string    `json:"combination_type"`
	BodyType                      *string    `json:"body_type"`
	LoadingAccess                 []string   `json:"loading_access"`
	UnloadingAccess               []string   `json:"unloading_access"`
	CapacityWeightKg              *float64   `json:"capacity_weight_kg"`
	CapacityVolumeM3              *float64   `json:"capacity_volume_m3"`
	TemperatureControlMode        *string    `json:"temperature_control_mode"`
	TemperatureCapabilityMinC     *float64   `json:"temperature_capability_min_c"`
	TemperatureCapabilityMaxC     *float64   `json:"temperature_capability_max_c"`
	TemperatureZoneCount          *int       `json:"temperature_zone_count"`
	IndependentTemperatureControl *bool      `json:"independent_temperature_control"`
	CurrentTemperatureSetpointC   *float64   `json:"current_temperature_setpoint_c"`
	LegacyEquipmentType           *string    `json:"legacy_equipment_type"`
	ContainerSize                 *string    `json:"container_size"`
	CapacitySemantics             string     `json:"capacity_semantics"`
	Version                       int        `json:"version"`
	CreatedAt                     time.Time  `json:"created_at"`
}

type fingerprintBody struct {
	RuleVersion            string    `json:"rule_version"`
	TenantID               uuid.UUID `json:"tenant_id"`
	ShipmentID             uuid.UUID `json:"shipment_id"`
	ShipmentVersion        int       `json:"shipment_version"`
	ShipmentStatus         string    `json:"shipment_status"`
	VehicleID              uuid.UUID `json:"vehicle_id"`
	VehicleVersion         *int      `json:"vehicle_version"`
	DestinationLocationID  uuid.UUID `json:"destination_location_id"`
	DestinationLatitude    *float64  `json:"destination_latitude"`
	DestinationLongitude   *float64  `json:"destination_longitude"`
	ETA                    string    `json:"eta"`
	ETAObservedAt          string    `json:"eta_observed_at"`
	DeliveryWindowStart    *string   `json:"delivery_window_start"`
	DeliveryWindowEnd      *string   `json:"delivery_window_end"`
	UnloadDurationSeconds  int       `json:"unload_duration_seconds"`
	UncertaintySeconds     int       `json:"uncertainty_seconds"`
	MaxETAAgeSeconds       int       `json:"max_eta_age_seconds"`
	CombinationType        *string   `json:"combination_type"`
	BodyType               *string   `json:"body_type"`
	LoadingAccess          []string  `json:"loading_access"`
	UnloadingAccess        []string  `json:"unloading_access"`
	CapacityWeightKg       *float64  `json:"capacity_weight_kg"`
	CapacityVolumeM3       *float64  `json:"capacity_volume_m3"`
	TemperatureControlMode *string   `json:"temperature_control_mode"`
	TemperatureMinC        *float64  `json:"temperature_capability_min_c"`
	TemperatureMaxC        *float64  `json:"temperature_capability_max_c"`
	TemperatureZoneCount   *int      `json:"temperature_zone_count"`
	IndependentTemperature *bool     `json:"independent_temperature_control"`
	CurrentSetpointC       *float64  `json:"current_temperature_setpoint_c"`
	LegacyEquipmentType    *string   `json:"legacy_equipment_type"`
	ContainerSize          *string   `json:"container_size"`
}

func (p PredictedCapacity) Fingerprint(maxETAAge time.Duration) (string, error) {
	body := fingerprintBody{
		RuleVersion: p.RuleVersion, TenantID: p.OwnerTenantID, ShipmentID: p.ShipmentID,
		ShipmentVersion: p.ShipmentVersion, ShipmentStatus: p.ShipmentStatus,
		VehicleID: p.VehicleID, VehicleVersion: p.VehicleVersion,
		DestinationLocationID: p.DestinationLocationID,
		DestinationLatitude:   p.DestinationLatitude, DestinationLongitude: p.DestinationLongitude,
		ETA: p.ETA.UTC().Format(time.RFC3339Nano), ETAObservedAt: p.ETAObservedAt.UTC().Format(time.RFC3339Nano),
		UnloadDurationSeconds: p.UnloadDurationSeconds, UncertaintySeconds: p.UncertaintySeconds,
		MaxETAAgeSeconds: int(maxETAAge / time.Second),
		CombinationType:  p.CombinationType, BodyType: p.BodyType,
		LoadingAccess: cloneStrings(p.LoadingAccess), UnloadingAccess: cloneStrings(p.UnloadingAccess),
		CapacityWeightKg: p.CapacityWeightKg, CapacityVolumeM3: p.CapacityVolumeM3,
		TemperatureControlMode: p.TemperatureControlMode,
		TemperatureMinC:        p.TemperatureCapabilityMinC, TemperatureMaxC: p.TemperatureCapabilityMaxC,
		TemperatureZoneCount: p.TemperatureZoneCount, IndependentTemperature: p.IndependentTemperatureControl,
		CurrentSetpointC: p.CurrentTemperatureSetpointC, LegacyEquipmentType: p.LegacyEquipmentType,
		ContainerSize: p.ContainerSize,
	}
	if p.DeliveryWindowStart != nil {
		value := p.DeliveryWindowStart.UTC().Format(time.RFC3339Nano)
		body.DeliveryWindowStart = &value
	}
	if p.DeliveryWindowEnd != nil {
		value := p.DeliveryWindowEnd.UTC().Format(time.RFC3339Nano)
		body.DeliveryWindowEnd = &value
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func ShipmentEligible(status string) bool {
	switch status {
	case "VEHICLE_ASSIGNED", "DRIVER_ASSIGNED", "PICKUP_SLOT_BOOKED", "DELIVERY_SLOT_BOOKED",
		"IN_PICKUP", "LOADED", "IN_TRANSIT", "ARRIVED_AT_CONSIGNEE", "UNLOADING":
		return true
	default:
		return false
	}
}

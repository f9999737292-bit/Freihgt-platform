package predict

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
)

var (
	ErrNotFound    = errors.New("prediction source not found")
	ErrUnavailable = errors.New("prediction source unavailable")
)

type Sources interface {
	Shipment(context.Context, uuid.UUID, uuid.UUID) (ShipmentFact, error)
	Vehicle(context.Context, uuid.UUID, uuid.UUID) (VehicleFact, error)
	ETA(context.Context, uuid.UUID, uuid.UUID) (ETAFact, error)
}

const Method = domain.PredictionMethodRuleBased

type Policy struct {
	Unload          time.Duration
	Uncertainty     time.Duration
	MaxETAAge       time.Duration
	ConfidenceFloor float64
	AutoActivate    bool
	RuleVersion     string
}

func (p Policy) Version() string {
	if p.RuleVersion == "" {
		return domain.PredictionRuleVersion
	}
	return p.RuleVersion
}

type ShipmentFact struct {
	ID                     uuid.UUID
	TenantID               uuid.UUID
	Status                 string
	Version                int
	VehicleID              *uuid.UUID
	DestinationLocationID  uuid.UUID
	DestinationLatitude    *float64
	DestinationLongitude   *float64
	PlannedDeliveryAt      *time.Time
	DeliveryWindowEnd      *time.Time
	OtherActiveAssignments int
}

type VehicleFact struct {
	ID                            uuid.UUID
	Version                       int
	LegacyEquipmentType           *string
	CombinationType               *string
	BodyType                      *string
	LoadingAccess                 []string
	UnloadingAccess               []string
	CapacityWeightKg              *float64
	CapacityVolumeM3              *float64
	TemperatureControlMode        *string
	TemperatureCapabilityMinC     *float64
	TemperatureCapabilityMaxC     *float64
	TemperatureZoneCount          *int
	IndependentTemperatureControl *bool
	CurrentTemperatureSetpointC   *float64
	ContainerSize                 *string
}

type ETAFact struct {
	Present    bool
	Arrival    time.Time
	ObservedAt time.Time
}

type Draft struct {
	Prediction domain.PredictedCapacity
	Reason     string
}

// Evaluate builds one rule-based next-load forecast.
// Payload and volume are the vehicle nominal capability.
// Current-trip residual capacity is not an input and is not computed.
// Confidence is the versioned rule score documented on Score, not a calibrated probability.
func Evaluate(now time.Time, policy Policy, shipment ShipmentFact, vehicle *VehicleFact, eta *ETAFact) Draft {
	if shipment.Status == "CANCELLED" {
		return Draft{Reason: domain.ReasonShipmentCancelled}
	}
	if !domain.ShipmentEligible(shipment.Status) {
		return Draft{Reason: domain.ReasonShipmentNotEligible}
	}
	if shipment.VehicleID == nil || *shipment.VehicleID == uuid.Nil {
		return Draft{Reason: domain.ReasonVehicleUnassigned}
	}
	if shipment.OtherActiveAssignments > 0 {
		return Draft{Reason: domain.ReasonVehicleFutureAvailabilityAmbiguous}
	}
	if shipment.DestinationLocationID == uuid.Nil {
		return Draft{Reason: domain.ReasonDestinationUnavailable}
	}
	if shipment.DeliveryWindowEnd != nil {
		if shipment.PlannedDeliveryAt == nil || !shipment.DeliveryWindowEnd.After(*shipment.PlannedDeliveryAt) {
			return Draft{Reason: domain.ReasonDeliveryWindowInvalid}
		}
	}
	if policy.Unload <= 0 {
		return Draft{Reason: domain.ReasonUnloadPolicyUnavailable}
	}
	if policy.Uncertainty <= 0 {
		return Draft{Reason: domain.ReasonAvailabilityPolicyUnavailable}
	}
	if policy.MaxETAAge <= 0 {
		return Draft{Reason: domain.ReasonETAFreshnessPolicyUnavailable}
	}
	if vehicle == nil || vehicle.ID == uuid.Nil || vehicle.ID != *shipment.VehicleID {
		return Draft{Reason: domain.ReasonVehicleCapacityUnavailable}
	}
	if eta == nil || !eta.Present || eta.Arrival.IsZero() || eta.ObservedAt.IsZero() {
		return Draft{Reason: domain.ReasonETAUnavailable}
	}
	age := now.Sub(eta.ObservedAt)
	if age < 0 || age > policy.MaxETAAge {
		return Draft{Reason: domain.ReasonETAStale}
	}
	body, err := domain.CanonicalBody(vehicle.BodyType, vehicle.LegacyEquipmentType)
	if err != nil {
		return Draft{Reason: domain.ReasonVehicleCapacityUnavailable}
	}
	loading, err := domain.NormalizeAccess(vehicle.LoadingAccess)
	if err != nil {
		return Draft{Reason: domain.ReasonVehicleCapacityUnavailable}
	}
	unloading, err := domain.NormalizeAccess(vehicle.UnloadingAccess)
	if err != nil {
		return Draft{Reason: domain.ReasonVehicleCapacityUnavailable}
	}
	serviceStart := eta.Arrival
	var windowStart *time.Time
	if shipment.PlannedDeliveryAt != nil {
		planned := shipment.PlannedDeliveryAt.UTC()
		windowStart = &planned
		if planned.After(serviceStart) {
			serviceStart = planned
		}
	}
	predictedAt := serviceStart.Add(policy.Unload).UTC()
	availabilityStart := predictedAt.Add(-policy.Uncertainty).UTC()
	availabilityEnd := predictedAt.Add(policy.Uncertainty).UTC()
	confidence := Score(ScoreInput{
		DeliveryWindowKnown: shipment.PlannedDeliveryAt != nil,
		CoordinatesKnown:    shipment.DestinationLatitude != nil && shipment.DestinationLongitude != nil,
		BodyKnown:           body != nil,
		WeightKnown:         vehicle.CapacityWeightKg != nil,
		VolumeKnown:         vehicle.CapacityVolumeM3 != nil,
		ETAAge:              age,
		MaxETAAge:           policy.MaxETAAge,
	})
	version := vehicle.Version
	prediction := domain.PredictedCapacity{
		OwnerTenantID: shipment.TenantID, ShipmentID: shipment.ID,
		ShipmentVersion: shipment.Version, ShipmentStatus: shipment.Status,
		VehicleID: vehicle.ID, VehicleVersion: &version,
		DestinationLocationID: shipment.DestinationLocationID,
		DestinationLatitude:   shipment.DestinationLatitude, DestinationLongitude: shipment.DestinationLongitude,
		ETA: eta.Arrival.UTC(), ETAObservedAt: eta.ObservedAt.UTC(),
		DeliveryWindowStart: windowStart, DeliveryWindowEnd: shipment.DeliveryWindowEnd,
		PredictedAvailableAt: predictedAt, AvailabilityWindowStart: availabilityStart, AvailabilityWindowEnd: availabilityEnd,
		UnloadDurationSeconds: int(policy.Unload / time.Second), UnloadPolicySource: domain.UnloadPolicySource,
		UncertaintySeconds: int(policy.Uncertainty / time.Second), Confidence: confidence,
		PredictionMethod: Method, RuleVersion: policy.Version(), GeneratedAt: now.UTC(),
		IsCurrent: true, CombinationType: vehicle.CombinationType, BodyType: body,
		LoadingAccess: loading, UnloadingAccess: unloading,
		CapacityWeightKg: vehicle.CapacityWeightKg, CapacityVolumeM3: vehicle.CapacityVolumeM3,
		TemperatureControlMode:    vehicle.TemperatureControlMode,
		TemperatureCapabilityMinC: vehicle.TemperatureCapabilityMinC, TemperatureCapabilityMaxC: vehicle.TemperatureCapabilityMaxC,
		TemperatureZoneCount: vehicle.TemperatureZoneCount, IndependentTemperatureControl: vehicle.IndependentTemperatureControl,
		CurrentTemperatureSetpointC: vehicle.CurrentTemperatureSetpointC,
		LegacyEquipmentType:         vehicle.LegacyEquipmentType, ContainerSize: vehicle.ContainerSize,
		CapacitySemantics: domain.CapacitySemanticsNextLoad, Version: 1, CreatedAt: now.UTC(),
	}
	fingerprint, err := prediction.Fingerprint(policy.MaxETAAge)
	if err != nil {
		return Draft{Reason: domain.ReasonVehicleCapacityUnavailable}
	}
	prediction.InputFingerprint = fingerprint
	return Draft{Prediction: prediction}
}

type ScoreInput struct {
	DeliveryWindowKnown bool
	CoordinatesKnown    bool
	BodyKnown           bool
	WeightKnown         bool
	VolumeKnown         bool
	ETAAge              time.Duration
	MaxETAAge           time.Duration
}

// Score is rule version bno-predict-0.1b.1.
// Start at 1. Subtract only these amounts, then clamp to [0, 1]:
//   - 0.15 when no delivery window is known
//   - 0.10 when destination coordinates are missing
//   - 0.05 when normalized body type is unknown
//   - 0.05 when nominal weight is unknown
//   - 0.05 when nominal volume is unknown
//   - 0.10 when ETA age / max age is greater than 0.25 and at most 0.50
//   - 0.20 when ETA age / max age is greater than 0.50
// This is a deterministic rule score, not a calibrated probability.
func Score(in ScoreInput) float64 {
	score := 1.0
	if !in.DeliveryWindowKnown {
		score -= 0.15
	}
	if !in.CoordinatesKnown {
		score -= 0.10
	}
	if !in.BodyKnown {
		score -= 0.05
	}
	if !in.WeightKnown {
		score -= 0.05
	}
	if !in.VolumeKnown {
		score -= 0.05
	}
	if in.MaxETAAge > 0 {
		ratio := float64(in.ETAAge) / float64(in.MaxETAAge)
		if ratio > 0.50 {
			score -= 0.20
		} else if ratio > 0.25 {
			score -= 0.10
		}
	}
	return clamp01(score)
}

func clamp01(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

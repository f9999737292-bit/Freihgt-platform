package currenttrip

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	bnometrics "github.com/freight-platform/network-optimizer-service/internal/platform/metrics"
)

type ShipmentExecutionSource interface {
	ExecutionContext(ctx context.Context, tenant, shipment uuid.UUID) (ShipmentExecution, error)
}

type ShipmentOnboardCargoSource interface {
	OnboardCargo(ctx context.Context, tenant, shipment uuid.UUID) (OnboardCargo, error)
}

type CargoPlanningSource interface {
	CargoPlanningProfile(ctx context.Context, tenant, cargo uuid.UUID) (CargoProfile, error)
}

type VehicleCapabilitySource interface {
	VehicleCapability(ctx context.Context, tenant, vehicle uuid.UUID) (VehicleCapability, error)
}

type TrackingStateSource interface {
	TrackingState(ctx context.Context, tenant, shipment uuid.UUID) (TrackingPosition, error)
}

type TrackingETASource interface {
	TrackingETA(ctx context.Context, tenant, shipment uuid.UUID) (ETA, error)
}

type Provider struct {
	execution    ShipmentExecutionSource
	onboard      ShipmentOnboardCargoSource
	cargo        CargoPlanningSource
	vehicle      VehicleCapabilitySource
	tracking     TrackingStateSource
	eta          TrackingETASource
	equivalences []PalletEquivalence
	now          func() time.Time
}

func NewProvider(
	execution ShipmentExecutionSource,
	onboard ShipmentOnboardCargoSource,
	cargo CargoPlanningSource,
	vehicle VehicleCapabilitySource,
	tracking TrackingStateSource,
	eta TrackingETASource,
) *Provider {
	return &Provider{
		execution: execution, onboard: onboard, cargo: cargo, vehicle: vehicle, tracking: tracking, eta: eta,
		now: func() time.Time { return time.Now().UTC() },
	}
}

func (p *Provider) WithEquivalences(rows []PalletEquivalence) *Provider {
	p.equivalences = append([]PalletEquivalence(nil), rows...)
	return p
}

func (p *Provider) Build(ctx context.Context, tenantID, shipmentID uuid.UUID) (CurrentTripContext, error) {
	result := "built"
	defer func() { bnometrics.CurrentTripContext(result) }()

	execution, err := p.execution.ExecutionContext(ctx, tenantID, shipmentID)
	if err != nil {
		result = "not_found"
		return CurrentTripContext{}, err
	}
	onboard, err := p.onboard.OnboardCargo(ctx, tenantID, shipmentID)
	if err != nil {
		result = "unavailable"
		return CurrentTripContext{}, err
	}
	units := make([]OnboardCargoUnit, 0, len(onboard.Items))
	confirmed := 0
	for _, item := range onboard.Items {
		profile, profileErr := p.cargo.CargoPlanningProfile(ctx, tenantID, item.CargoID)
		if profileErr != nil {
			result = "unavailable"
			return CurrentTripContext{}, profileErr
		}
		if item.State == EvidenceConfirmedOnboard {
			confirmed++
		}
		units = append(units, OnboardCargoUnit{
			CargoID: item.CargoID, CargoVersion: profile.Version, EvidenceState: item.State,
			EvidenceStateVersion: item.StateVersion, EvidenceOccurredAt: item.OccurredAt.UTC(),
			EvidenceSource: item.Source, Profile: profile,
		})
	}
	var capability *VehicleCapability
	vehicleVersion := 0
	if execution.VehicleID != nil {
		loaded, vehicleErr := p.vehicle.VehicleCapability(ctx, tenantID, *execution.VehicleID)
		if vehicleErr != nil {
			result = "unavailable"
			return CurrentTripContext{}, vehicleErr
		}
		capability = &loaded
		vehicleVersion = loaded.Version
	}
	positionFreshness := "UNKNOWN"
	var position *TrackingPosition
	var positionAt *time.Time
	if p.tracking != nil {
		loaded, trackErr := p.tracking.TrackingState(ctx, tenantID, shipmentID)
		if trackErr == nil {
			position = &loaded
			positionFreshness = loaded.Freshness
			positionAt = loaded.RecordedAt
		}
	}
	etaFreshness := "UNKNOWN"
	var eta *ETA
	var etaAt *time.Time
	if p.eta != nil {
		loaded, etaErr := p.eta.TrackingETA(ctx, tenantID, shipmentID)
		if etaErr == nil {
			eta = &loaded
			if loaded.FreshnessStatus != "" {
				etaFreshness = loaded.FreshnessStatus
			}
			etaAt = loaded.SourceObservedAt
		}
	}
	resolution := onboard.Resolution
	if resolution == "" {
		resolution = ResolutionUnproven
	}
	if execution.CargoID == nil && len(onboard.Items) == 0 {
		resolution = ResolutionUnproven
	}
	built := CurrentTripContext{
		ShipmentID: execution.ShipmentID, ShipmentVersion: execution.ShipmentVersion, ShipmentStatus: execution.ShipmentStatus,
		VehicleID: execution.VehicleID, VehicleVersion: vehicleVersion,
		OriginLocationID: execution.OriginLocationID, DestinationLocationID: execution.DestinationLocationID,
		CurrentPosition: position, PositionFreshnessStatus: positionFreshness, PositionObservedAt: positionAt,
		ETA: eta, ETAFreshnessStatus: etaFreshness, ETAObservedAt: etaAt,
		OnboardCargoUnits: units, VehicleCapability: capability,
		ResidualCapacity: residualSnapshot(residualInput{
			Resolution: resolution, Units: units, Vehicle: capability, Equivalences: p.equivalences,
		}),
		BuiltAt: p.now(),
	}
	built.InputFingerprint = inputFingerprint(built)
	recordResidualMetrics(built.ResidualCapacity)
	slog.Info("current trip context built",
		slog.String("result", result),
		slog.Int("shipment_version", built.ShipmentVersion),
		slog.Int("confirmed_onboard_units", confirmed),
	)
	return built, nil
}

func recordResidualMetrics(snapshot ResidualCapacitySnapshot) {
	bnometrics.ResidualDimension("payload", snapshot.Payload.Status)
	bnometrics.ResidualDimension("volume", snapshot.Volume.Status)
	bnometrics.ResidualDimension("pallet_positions", snapshot.PalletPositions.Status)
	bnometrics.ResidualDimension("linear_meters", snapshot.LinearMeters.Status)
	bnometrics.ResidualDimension("height", snapshot.Height.Status)
}

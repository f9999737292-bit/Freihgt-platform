package currenttrip

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNLO03C_ResidualAndContext(t *testing.T) {
	tenant := uuid.New()
	shipmentID := uuid.New()
	vehicleID := uuid.New()
	cargoID := uuid.New()
	occurred := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	weight := 400.0
	volume := 12.0
	linear := 4.0
	height := 2500
	pallets := 2
	palletType := "TYPE_A"
	vehicleWeight := 1000.0
	vehicleVolume := 40.0
	vehicleLinear := 14.0
	vehicleHeight := 2700
	vehiclePallets := 10

	base := func() *fakeSources {
		return &fakeSources{
			execution: ShipmentExecution{
				ShipmentID: shipmentID, TenantID: tenant, ShipmentVersion: 5, ShipmentStatus: "LOADED",
				VehicleID: &vehicleID, OriginLocationID: uuid.New(), DestinationLocationID: uuid.New(), CargoID: &cargoID,
			},
			onboard: OnboardCargo{ShipmentID: shipmentID, ShipmentVersion: 5, Resolution: EvidenceConfirmedOnboard, Items: []OnboardEvidenceItem{{
				CargoID: cargoID, State: EvidenceConfirmedOnboard, StateVersion: 5, OccurredAt: occurred, Source: "DRIVER_OPERATION",
			}}},
			profile: CargoProfile{ID: cargoID, Version: 3, WeightKg: &weight, VolumeM3: &volume, LinearMeters: &linear, MaxLoadedHeightMM: &height, PalletCount: &pallets, PalletTypeCode: &palletType},
			vehicle: VehicleCapability{
				ID: vehicleID, TenantID: tenant, Version: 2,
				CapacityWeight: &vehicleWeight, CapacityVolume: &vehicleVolume, UsableLinearMeters: &vehicleLinear,
				InternalHeightMM: &vehicleHeight, PalletPositions: &vehiclePallets,
			},
			position: TrackingPosition{Freshness: "FRESH", RecordedAt: &occurred},
			eta:      ETA{Status: "AVAILABLE", FreshnessStatus: "FRESH", SourceObservedAt: &occurred, EstimatedArrivalAt: &occurred},
		}
	}
	eq := []PalletEquivalence{{FromCode: "TYPE_A", BasisCode: "BASIS", PositionsEach: 1, OwnershipProven: true}}
	build := func(src *fakeSources) CurrentTripContext {
		t.Helper()
		provider := NewProvider(src, src, src, src, src, src).WithEquivalences(eq)
		provider.now = func() time.Time { return occurred }
		ctx, err := provider.Build(context.Background(), tenant, shipmentID)
		if err != nil {
			t.Fatal(err)
		}
		return ctx
	}

	t.Run("NLO03C_046_KNOWN_WEIGHT_RESIDUAL", func(t *testing.T) {
		got := build(base())
		assertRemaining(t, got.ResidualCapacity.Payload, 600)
	})
	t.Run("NLO03C_047_UNKNOWN_VEHICLE_WEIGHT_RESIDUAL_UNKNOWN", func(t *testing.T) {
		src := base()
		src.vehicle.CapacityWeight = nil
		got := build(src)
		assertUnknown(t, got.ResidualCapacity.Payload)
	})
	t.Run("NLO03C_048_UNKNOWN_ONBOARD_WEIGHT_RESIDUAL_UNKNOWN", func(t *testing.T) {
		src := base()
		src.profile.WeightKg = nil
		got := build(src)
		assertUnknown(t, got.ResidualCapacity.Payload)
	})
	t.Run("NLO03C_049_NO_ONBOARD_EVIDENCE_WEIGHT_NOT_ZERO", func(t *testing.T) {
		src := base()
		src.onboard.Resolution = ResolutionUnproven
		src.onboard.Items = nil
		got := build(src)
		if got.ResidualCapacity.Payload.Status != DimensionUnknown || got.ResidualCapacity.Payload.Remaining != nil || got.ResidualCapacity.Payload.Occupied != nil {
			t.Fatalf("%+v", got.ResidualCapacity.Payload)
		}
		if got.ResidualCapacity.Payload.Reason != ReasonUnproven {
			t.Fatal(got.ResidualCapacity.Payload.Reason)
		}
	})
	t.Run("NLO03C_050_KNOWN_VOLUME_RESIDUAL", func(t *testing.T) {
		assertRemaining(t, build(base()).ResidualCapacity.Volume, 28)
	})
	t.Run("NLO03C_051_UNKNOWN_VOLUME_RESIDUAL_UNKNOWN", func(t *testing.T) {
		src := base()
		src.profile.VolumeM3 = nil
		assertUnknown(t, build(src).ResidualCapacity.Volume)
	})
	t.Run("NLO03C_052_KNOWN_PALLET_RESIDUAL", func(t *testing.T) {
		assertRemaining(t, build(base()).ResidualCapacity.PalletPositions, 8)
	})
	t.Run("NLO03C_053_UNKNOWN_PALLET_COUNT_NOT_ZERO", func(t *testing.T) {
		src := base()
		src.profile.PalletCount = nil
		dim := build(src).ResidualCapacity.PalletPositions
		if dim.Status != DimensionUnknown || dim.Remaining != nil || dim.Reason != ReasonPalletCount {
			t.Fatalf("%+v", dim)
		}
	})
	t.Run("NLO03C_054_UNKNOWN_PALLET_TYPE_WHEN_REQUIRED", func(t *testing.T) {
		src := base()
		src.profile.PalletTypeCode = nil
		if build(src).ResidualCapacity.PalletPositions.Reason != ReasonPalletType {
			t.Fatal(build(src).ResidualCapacity.PalletPositions.Reason)
		}
	})
	t.Run("NLO03C_055_MISSING_PALLET_EQUIVALENCE_UNKNOWN", func(t *testing.T) {
		src := base()
		provider := NewProvider(src, src, src, src, src, src)
		got, err := provider.Build(context.Background(), tenant, shipmentID)
		if err != nil || got.ResidualCapacity.PalletPositions.Status != DimensionUnknown || got.ResidualCapacity.PalletPositions.Reason != ReasonPalletEquivalence {
			t.Fatalf("%v %+v", err, got.ResidualCapacity.PalletPositions)
		}
	})
	t.Run("NLO03C_056_KNOWN_LINEAR_METRES_RESIDUAL", func(t *testing.T) {
		assertRemaining(t, build(base()).ResidualCapacity.LinearMeters, 10)
	})
	t.Run("NLO03C_057_UNKNOWN_LINEAR_METRES_UNKNOWN", func(t *testing.T) {
		src := base()
		src.profile.LinearMeters = nil
		assertUnknown(t, build(src).ResidualCapacity.LinearMeters)
	})
	t.Run("NLO03C_058_HEIGHT_KNOWN_OK", func(t *testing.T) {
		if build(base()).ResidualCapacity.Height.Status != HeightKnownOK {
			t.Fatal(build(base()).ResidualCapacity.Height.Status)
		}
	})
	t.Run("NLO03C_059_HEIGHT_EXCEEDED", func(t *testing.T) {
		src := base()
		tooTall := 3000
		src.profile.MaxLoadedHeightMM = &tooTall
		if build(src).ResidualCapacity.Height.Status != HeightKnownExceeded {
			t.Fatal(build(src).ResidualCapacity.Height.Status)
		}
	})
	t.Run("NLO03C_060_HEIGHT_UNKNOWN", func(t *testing.T) {
		src := base()
		src.vehicle.InternalHeightMM = nil
		if build(src).ResidualCapacity.Height.Status != HeightUnknown {
			t.Fatal(build(src).ResidualCapacity.Height.Status)
		}
	})
	t.Run("NLO03C_061_OCCUPIED_WEIGHT_EXCEEDS_CAPACITY", func(t *testing.T) {
		src := base()
		over := 50.0
		src.vehicle.CapacityWeight = &over
		dim := build(src).ResidualCapacity.Payload
		if dim.Status != DimensionExceeds || dim.Remaining != nil {
			t.Fatalf("%+v", dim)
		}
	})
	t.Run("NLO03C_062_OCCUPIED_VOLUME_EXCEEDS_CAPACITY", func(t *testing.T) {
		src := base()
		over := 1.0
		src.vehicle.CapacityVolume = &over
		dim := build(src).ResidualCapacity.Volume
		if dim.Status != DimensionExceeds || dim.Remaining != nil {
			t.Fatalf("%+v", dim)
		}
	})
	t.Run("NLO03C_063_SERVER_BUILT_CONTEXT", func(t *testing.T) {
		got := build(base())
		if got.ShipmentID != shipmentID || got.InputFingerprint == "" || got.BuiltAt.IsZero() {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("NLO03C_064_CONTEXT_BINDS_SHIPMENT_VERSION", func(t *testing.T) {
		if build(base()).ShipmentVersion != 5 {
			t.Fatal(build(base()).ShipmentVersion)
		}
	})
	t.Run("NLO03C_065_CONTEXT_BINDS_VEHICLE_VERSION", func(t *testing.T) {
		if build(base()).VehicleVersion != 2 {
			t.Fatal(build(base()).VehicleVersion)
		}
	})
	t.Run("NLO03C_066_CONTEXT_BINDS_CARGO_VERSION", func(t *testing.T) {
		got := build(base())
		if len(got.OnboardCargoUnits) != 1 || got.OnboardCargoUnits[0].CargoVersion != 3 {
			t.Fatalf("%+v", got.OnboardCargoUnits)
		}
	})
	t.Run("NLO03C_067_CONTEXT_EVIDENCE_ONLY_CONFIRMED_COUNTS", func(t *testing.T) {
		src := base()
		other := uuid.New()
		otherWeight := 900.0
		src.onboard.Items = append(src.onboard.Items, OnboardEvidenceItem{CargoID: other, State: EvidenceUnloaded, StateVersion: 4, OccurredAt: occurred, Source: "DRIVER_OPERATION"})
		src.extra = map[uuid.UUID]CargoProfile{other: {ID: other, Version: 1, WeightKg: &otherWeight, VolumeM3: &volume, LinearMeters: &linear, MaxLoadedHeightMM: &height}}
		assertRemaining(t, build(src).ResidualCapacity.Payload, 600)
	})
	first := build(base())
	t.Run("NLO03C_068_CONTEXT_FINGERPRINT_DETERMINISTIC", func(t *testing.T) {
		src := base()
		provider := NewProvider(src, src, src, src, src, src).WithEquivalences(eq)
		provider.now = func() time.Time { return occurred.Add(time.Hour) }
		second, err := provider.Build(context.Background(), tenant, shipmentID)
		if err != nil || second.InputFingerprint != first.InputFingerprint || second.BuiltAt.Equal(first.BuiltAt) {
			t.Fatalf("fp %s %s built %s %s err %v", first.InputFingerprint, second.InputFingerprint, first.BuiltAt, second.BuiltAt, err)
		}
	})
	t.Run("NLO03C_069_CONTEXT_FINGERPRINT_CHANGES_ON_EVIDENCE", func(t *testing.T) {
		src := base()
		src.onboard.Items[0].StateVersion = 9
		if build(src).InputFingerprint == first.InputFingerprint {
			t.Fatal("fingerprint unchanged")
		}
	})
	t.Run("NLO03C_070_CONTEXT_FINGERPRINT_CHANGES_ON_VEHICLE_VERSION", func(t *testing.T) {
		src := base()
		src.vehicle.Version = 8
		if build(src).InputFingerprint == first.InputFingerprint {
			t.Fatal("fingerprint unchanged")
		}
	})
	t.Run("NLO03C_071_CONTEXT_FINGERPRINT_CHANGES_ON_CARGO_VERSION", func(t *testing.T) {
		src := base()
		src.profile.Version = 11
		if build(src).InputFingerprint == first.InputFingerprint {
			t.Fatal("fingerprint unchanged")
		}
	})
	t.Run("NLO03C_072_CONTEXT_DOES_NOT_TRUST_CALLER_FACTS", func(t *testing.T) {
		callerRemaining := 1.0
		got := build(base())
		if got.ResidualCapacity.Payload.Remaining == nil || *got.ResidualCapacity.Payload.Remaining == callerRemaining {
			t.Fatalf("trusted caller residual %+v", got.ResidualCapacity.Payload)
		}
	})
}

func assertRemaining(t *testing.T, dim SubtractiveDimension, want float64) {
	t.Helper()
	if dim.Status != DimensionKnown || dim.Remaining == nil || *dim.Remaining != want {
		t.Fatalf("%+v want %v", dim, want)
	}
}

func assertUnknown(t *testing.T, dim SubtractiveDimension) {
	t.Helper()
	if dim.Status != DimensionUnknown || dim.Remaining != nil {
		t.Fatalf("%+v", dim)
	}
}

type fakeSources struct {
	execution ShipmentExecution
	onboard   OnboardCargo
	profile   CargoProfile
	extra     map[uuid.UUID]CargoProfile
	vehicle   VehicleCapability
	position  TrackingPosition
	eta       ETA
}

func (f *fakeSources) ExecutionContext(context.Context, uuid.UUID, uuid.UUID) (ShipmentExecution, error) {
	return f.execution, nil
}
func (f *fakeSources) OnboardCargo(context.Context, uuid.UUID, uuid.UUID) (OnboardCargo, error) {
	return f.onboard, nil
}
func (f *fakeSources) CargoPlanningProfile(_ context.Context, _, cargo uuid.UUID) (CargoProfile, error) {
	if extra, ok := f.extra[cargo]; ok {
		return extra, nil
	}
	return f.profile, nil
}
func (f *fakeSources) VehicleCapability(context.Context, uuid.UUID, uuid.UUID) (VehicleCapability, error) {
	return f.vehicle, nil
}
func (f *fakeSources) TrackingState(context.Context, uuid.UUID, uuid.UUID) (TrackingPosition, error) {
	return f.position, nil
}
func (f *fakeSources) TrackingETA(context.Context, uuid.UUID, uuid.UUID) (ETA, error) {
	return f.eta, nil
}

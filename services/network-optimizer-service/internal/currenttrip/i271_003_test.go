package currenttrip

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestI271_003_OnboardResolutionFingerprint(t *testing.T) {
	unloaded := CurrentTripContext{OnboardCargoResolution: EvidenceUnloaded}
	unproven := CurrentTripContext{OnboardCargoResolution: ResolutionUnproven}
	if inputFingerprint(unloaded) == inputFingerprint(unproven) || inputFingerprint(unloaded) == inputFingerprint(CurrentTripContext{}) {
		t.Fatal("authoritative onboard resolution collapsed in the context fingerprint")
	}

	tenant := uuid.New()
	shipmentID := uuid.New()
	vehicleID := uuid.New()
	src := &fakeSources{
		execution: ShipmentExecution{
			ShipmentID: shipmentID, TenantID: tenant, ShipmentVersion: 1, ShipmentStatus: "IN_TRANSIT",
			VehicleID: &vehicleID, OriginLocationID: uuid.New(), DestinationLocationID: uuid.New(),
		},
		onboard:  OnboardCargo{ShipmentID: shipmentID, ShipmentVersion: 1, Resolution: EvidenceUnloaded},
		vehicle:  VehicleCapability{ID: vehicleID, TenantID: tenant, Version: 1},
		position: TrackingPosition{Freshness: "FRESH"},
		eta:      ETA{Status: "AVAILABLE", FreshnessStatus: "FRESH"},
	}
	built, err := NewProvider(src, src, src, src, src, src).Build(context.Background(), tenant, shipmentID)
	if err != nil {
		t.Fatal(err)
	}
	if built.OnboardCargoResolution != EvidenceUnloaded {
		t.Fatalf("resolution %q", built.OnboardCargoResolution)
	}
}

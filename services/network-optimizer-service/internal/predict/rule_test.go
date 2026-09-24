package predict

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBNO39AndBNO40ConfidenceFormula(t *testing.T) {
	in := ScoreInput{DeliveryWindowKnown: true, CoordinatesKnown: true, BodyKnown: true, WeightKnown: true, VolumeKnown: true, ETAAge: 10 * time.Minute, MaxETAAge: 30 * time.Minute}
	first := Score(in)
	second := Score(in)
	if first != second {
		t.Fatalf("BNO39 %v %v", first, second)
	}
	if first < 0 || first > 1 {
		t.Fatalf("BNO40 %v", first)
	}
	quarter := Score(ScoreInput{DeliveryWindowKnown: true, CoordinatesKnown: true, BodyKnown: true, WeightKnown: true, VolumeKnown: true, ETAAge: 7 * time.Minute, MaxETAAge: 30 * time.Minute})
	half := Score(ScoreInput{DeliveryWindowKnown: true, CoordinatesKnown: true, BodyKnown: true, WeightKnown: true, VolumeKnown: true, ETAAge: 10 * time.Minute, MaxETAAge: 30 * time.Minute})
	late := Score(ScoreInput{DeliveryWindowKnown: true, CoordinatesKnown: true, BodyKnown: true, WeightKnown: true, VolumeKnown: true, ETAAge: 20 * time.Minute, MaxETAAge: 30 * time.Minute})
	if quarter != 1 || math.Abs(half-0.9) > 1e-9 || math.Abs(late-0.8) > 1e-9 {
		t.Fatalf("boundaries quarter=%v half=%v late=%v", quarter, half, late)
	}
	if clamp01(-0.2) != 0 || clamp01(1.2) != 1 || clamp01(0.4) != 0.4 {
		t.Fatal("BNO40 clamp")
	}
}

func TestBNO78EvaluateKeepsNominalCapacity(t *testing.T) {
	weight := 20000.0
	volume := 82.0
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	vehicleID := uuid.New()
	body := "TENT"
	shipment := ShipmentFact{
		ID: uuid.New(), TenantID: uuid.New(), Status: "IN_TRANSIT", Version: 2,
		VehicleID: &vehicleID, DestinationLocationID: uuid.New(),
	}
	vehicle := &VehicleFact{ID: vehicleID, Version: 1, BodyType: &body, CapacityWeightKg: &weight, CapacityVolumeM3: &volume}
	eta := &ETAFact{Present: true, Arrival: now.Add(2 * time.Hour), ObservedAt: now.Add(-time.Minute)}
	draft := Evaluate(now, Policy{Unload: 35 * time.Minute, Uncertainty: 20 * time.Minute, MaxETAAge: 30 * time.Minute}, shipment, vehicle, eta)
	if draft.Reason != "" {
		t.Fatal(draft.Reason)
	}
	if draft.Prediction.CapacityWeightKg == nil || *draft.Prediction.CapacityWeightKg != 20000 || *draft.Prediction.CapacityVolumeM3 != 82 {
		t.Fatalf("BNO78/BNO79 %+v", draft.Prediction.CapacityWeightKg)
	}
	if draft.Prediction.CapacitySemantics != "NEXT_LOAD_FUTURE_CAPACITY" {
		t.Fatal(draft.Prediction.CapacitySemantics)
	}
}

func TestBNO80AmbiguousAssignment(t *testing.T) {
	vehicleID := uuid.New()
	draft := Evaluate(time.Now(), Policy{Unload: time.Minute, Uncertainty: time.Minute, MaxETAAge: time.Hour}, ShipmentFact{
		ID: uuid.New(), Status: "IN_TRANSIT", VehicleID: &vehicleID, DestinationLocationID: uuid.New(), OtherActiveAssignments: 1,
	}, &VehicleFact{ID: vehicleID}, &ETAFact{Present: true, Arrival: time.Now(), ObservedAt: time.Now()})
	if draft.Reason != "VEHICLE_FUTURE_AVAILABILITY_AMBIGUOUS" {
		t.Fatal(draft.Reason)
	}
}

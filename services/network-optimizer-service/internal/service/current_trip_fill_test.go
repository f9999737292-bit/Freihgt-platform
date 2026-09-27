package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/currenttrip"
	"github.com/freight-platform/network-optimizer-service/internal/domain"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

type fillSources struct {
	notFound  bool
	execution currenttrip.ShipmentExecution
	onboard   currenttrip.OnboardCargo
	profiles  map[uuid.UUID]currenttrip.CargoProfile
	vehicle   currenttrip.VehicleCapability
	position  currenttrip.TrackingPosition
	eta       currenttrip.ETA
}

func (f *fillSources) ExecutionContext(context.Context, uuid.UUID, uuid.UUID) (currenttrip.ShipmentExecution, error) {
	if f.notFound {
		return currenttrip.ShipmentExecution{}, currenttrip.ErrNotFound
	}
	return f.execution, nil
}
func (f *fillSources) OnboardCargo(context.Context, uuid.UUID, uuid.UUID) (currenttrip.OnboardCargo, error) {
	return f.onboard, nil
}
func (f *fillSources) CargoPlanningProfile(_ context.Context, _, cargo uuid.UUID) (currenttrip.CargoProfile, error) {
	return f.profiles[cargo], nil
}
func (f *fillSources) VehicleCapability(context.Context, uuid.UUID, uuid.UUID) (currenttrip.VehicleCapability, error) {
	return f.vehicle, nil
}
func (f *fillSources) TrackingState(context.Context, uuid.UUID, uuid.UUID) (currenttrip.TrackingPosition, error) {
	return f.position, nil
}
func (f *fillSources) TrackingETA(context.Context, uuid.UUID, uuid.UUID) (currenttrip.ETA, error) {
	return f.eta, nil
}

type ruleCatalog struct{ rules []compat.Rule }

func (c ruleCatalog) Evaluation(context.Context, uuid.UUID) (compat.Context, error) {
	return compat.Context{Rules: c.rules}, nil
}

func TestNLO03DCurrentTripFill(t *testing.T) {
	t.Run("NLO03D_001_RECOGNIZED_AND_SERVER_BUILT", func(t *testing.T) {
		doc := feasibleFill(t)
		if doc.Pattern != PatternCurrentTripFill || doc.MaxAdditionalLoads == nil || *doc.MaxAdditionalLoads != 1 || doc.ExecutionSupported == nil || *doc.ExecutionSupported {
			t.Fatalf("%+v", doc)
		}
		if doc.ContextSummary == nil || doc.InputFingerprint == "" || doc.ContextSummary.ConfirmedOnboard != 1 {
			t.Fatalf("%+v", doc.ContextSummary)
		}
		if doc.FeasibleCandidateCount != 1 || doc.Candidates[0].ExecutionSupported || len(doc.Candidates[0].Members) != 1 {
			t.Fatalf("%+v", doc.Candidates)
		}
	})
	t.Run("NLO03D_002_FOREIGN_SHIPMENT_NOT_FOUND", func(t *testing.T) {
		w, src, _ := newFill(t)
		src.notFound = true
		_, err := w.svc.SearchConsolidation(context.Background(), w.actor(), ConsolidationCommand{ShipmentID: src.execution.ShipmentID, Pattern: PatternCurrentTripFill})
		var app *apperrors.AppError
		if !errors.As(err, &app) || app.Code != apperrors.CodeNotFound || strings.Contains(app.Message, src.execution.ShipmentID.String()) {
			t.Fatal(err)
		}
	})
	t.Run("NLO03D_003_NO_EVIDENCE_INDETERMINATE", func(t *testing.T) {
		w, src, _ := newFill(t)
		src.onboard.Items = nil
		src.onboard.Resolution = currenttrip.ResolutionUnproven
		doc := searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Status == ConsolidationFeasible || !has(doc.Candidates[0].IndeterminateReasonCodes, currenttrip.ReasonUnproven) {
			t.Fatalf("%+v", doc.Candidates[0])
		}
	})
	t.Run("NLO03D_004_WEIGHT_PASS_FAIL_UNKNOWN", func(t *testing.T) {
		if feasibleFill(t).Candidates[0].Status != ConsolidationFeasible {
			t.Fatal("pass")
		}
		w, src, load := newFill(t)
		heavy := 50000.0
		load.WeightKg = &heavy
		replaceLoad(t, w, load)
		doc := searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Status != ConsolidationHardReject || !has(doc.Candidates[0].Explanation, ReasonPayloadExceeded) {
			t.Fatalf("%+v", doc.Candidates[0])
		}
		w, src, _ = newFill(t)
		src.vehicle.CapacityWeight = nil
		doc = searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Status == ConsolidationFeasible || !has(doc.Candidates[0].IndeterminateReasonCodes, ReasonResidualUnknown) {
			t.Fatalf("%+v", doc.Candidates[0])
		}
	})
	t.Run("NLO03D_005_VOLUME_PASS_FAIL_UNKNOWN", func(t *testing.T) {
		w, src, load := newFill(t)
		vol := 1000.0
		load.VolumeM3 = &vol
		replaceLoad(t, w, load)
		doc := searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Status != ConsolidationHardReject || !has(doc.Candidates[0].Explanation, ReasonVolumeExceeded) {
			t.Fatalf("%+v", doc.Candidates[0])
		}
		w, src, _ = newFill(t)
		src.vehicle.CapacityVolume = nil
		doc = searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Status == ConsolidationFeasible || !has(doc.Candidates[0].IndeterminateReasonCodes, ReasonResidualUnknown) {
			t.Fatalf("%+v", doc.Candidates[0])
		}
	})
	t.Run("NLO03D_006_PALLET_EQUIVALENCE", func(t *testing.T) {
		w, src, load := newFill(t)
		count := 2
		onboardCount := 1
		code := "EUR"
		load.Cargo.PalletCount = &count
		load.Cargo.PalletTypeCode = &code
		replaceLoad(t, w, load)
		profile := src.profiles[src.onboard.Items[0].CargoID]
		profile.PalletCount = &onboardCount
		profile.PalletTypeCode = &code
		src.profiles[src.onboard.Items[0].CargoID] = profile
		w.svc.currentTrip.WithEquivalences([]currenttrip.PalletEquivalence{{FromCode: "EUR", BasisCode: "EUR", PositionsEach: 1, OwnershipProven: true}})
		doc := searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Status != ConsolidationFeasible {
			t.Fatalf("pass %+v", doc.Candidates[0])
		}
		w, src, load = newFill(t)
		load.Cargo.PalletCount = &count
		load.Cargo.PalletTypeCode = &code
		replaceLoad(t, w, load)
		doc = searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Status == ConsolidationFeasible || !has(doc.Candidates[0].IndeterminateReasonCodes, ReasonPalletEquivalence) {
			t.Fatalf("unknown %+v", doc.Candidates[0])
		}
	})
	t.Run("NLO03D_007_LINEAR_AND_HEIGHT", func(t *testing.T) {
		w, src, load := newFill(t)
		linear := 1.0
		load.Cargo.LinearMeters = &linear
		replaceLoad(t, w, load)
		src.profiles[src.onboard.Items[0].CargoID] = profileWithLinear(src.profiles[src.onboard.Items[0].CargoID], 1)
		src.vehicle.UsableLinearMeters = f64(13)
		doc := searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Status != ConsolidationFeasible {
			t.Fatalf("linear pass %+v", doc.Candidates[0])
		}
		linear = 20
		load.Cargo.LinearMeters = &linear
		replaceLoad(t, w, load)
		doc = searchFill(t, w, src.execution.ShipmentID)
		if !has(doc.Candidates[0].Explanation, ReasonLinearExceeded) {
			t.Fatalf("linear fail %+v", doc.Candidates[0])
		}
		w, src, load = newFill(t)
		height := 4000
		load.Cargo.MaxLoadedHeightMM = &height
		replaceLoad(t, w, load)
		doc = searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Status == ConsolidationFeasible || !has(doc.Candidates[0].Explanation, ReasonHeightExceeded) {
			t.Fatalf("height %+v", doc.Candidates[0])
		}
	})
	t.Run("NLO03D_008_TEMPERATURE_ADR_FOOD_ACCESS", func(t *testing.T) {
		w, src, load := newFill(t)
		load.Cargo.TemperatureRequired = boolPtr(true)
		load.Cargo.TemperatureMinC = f64(30)
		load.Cargo.TemperatureMaxC = f64(40)
		replaceLoad(t, w, load)
		doc := searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Status == ConsolidationFeasible || !has(doc.Candidates[0].Explanation, "TEMPERATURE_RANGE_UNSUPPORTED") {
			t.Fatalf("temp %+v", doc.Candidates[0])
		}
		w, src, load = newFill(t)
		load.Cargo.Dangerous = boolPtr(true)
		replaceLoad(t, w, load)
		src.vehicle.ADRCapability = boolPtr(false)
		doc = searchFill(t, w, src.execution.ShipmentID)
		if !has(doc.Candidates[0].Explanation, "ADR_INCOMPATIBLE") {
			t.Fatalf("adr fail %+v", doc.Candidates[0])
		}
		w, src, load = newFill(t)
		load.Cargo.Dangerous = boolPtr(true)
		replaceLoad(t, w, load)
		src.vehicle.ADRCapability = nil
		doc = searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Status == ConsolidationFeasible || !has(doc.Candidates[0].IndeterminateReasonCodes, "ADR_CAPABILITY_UNKNOWN") {
			t.Fatalf("adr unknown %+v", doc.Candidates[0])
		}
		w, src, load = newFill(t)
		src.vehicle.ADRCapability = boolPtr(true)
		src.vehicle.FoodGradeCapability = boolPtr(true)
		load.Cargo.FoodGradeRequired = boolPtr(true)
		replaceLoad(t, w, load)
		doc = searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Status != ConsolidationFeasible {
			t.Fatalf("food pass %+v", doc.Candidates[0])
		}
		w, src, load = newFill(t)
		src.vehicle.FoodGradeCapability = boolPtr(false)
		load.Cargo.FoodGradeRequired = boolPtr(true)
		replaceLoad(t, w, load)
		doc = searchFill(t, w, src.execution.ShipmentID)
		if !has(doc.Candidates[0].Explanation, "FOOD_GRADE_REQUIRED") {
			t.Fatalf("food fail %+v", doc.Candidates[0])
		}
		w, src, load = newFill(t)
		load.Cargo.RequiredLoadingAccess = []string{"REAR"}
		replaceLoad(t, w, load)
		doc = searchFill(t, w, src.execution.ShipmentID)
		if !has(doc.Candidates[0].Explanation, "LOADING_ACCESS_UNSUPPORTED") {
			t.Fatalf("load access %+v", doc.Candidates[0])
		}
		w, src, load = newFill(t)
		load.Cargo.RequiredUnloadingAccess = []string{"SIDE"}
		replaceLoad(t, w, load)
		doc = searchFill(t, w, src.execution.ShipmentID)
		if !has(doc.Candidates[0].Explanation, "UNLOADING_ACCESS_UNSUPPORTED") {
			t.Fatalf("unload access %+v", doc.Candidates[0])
		}
	})
	t.Run("NLO03D_009_CARGO_DENY_AND_SEPARATION", func(t *testing.T) {
		w, src, load := newFill(t)
		code := "CHEM"
		load.Cargo.CargoTypeCode = &code
		replaceLoad(t, w, load)
		w.svc.UseCatalog(ruleCatalog{rules: []compat.Rule{{
			RuleKind: compat.KindCargoCargo, Layer: compat.LayerPlatform, Decision: compat.DecisionDeny,
			LeftSelectorType: "ANY", RightSelectorType: "ANY", ReasonCode: "CARGO_CARGO_HARD_DENY", Severity: compat.SeverityHard,
			RuleSetScope: "SYSTEM",
		}}})
		doc := searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Status == ConsolidationFeasible || !has(doc.Candidates[0].Explanation, "CARGO_CARGO_HARD_DENY") {
			t.Fatalf("deny %+v", doc.Candidates[0])
		}
		sep := "WALL"
		w.svc.UseCatalog(ruleCatalog{rules: []compat.Rule{{
			RuleKind: compat.KindCargoCargo, Layer: compat.LayerPlatform, Decision: compat.DecisionRequireSeparation,
			LeftSelectorType: "ANY", RightSelectorType: "ANY", ReasonCode: "REQUIRE_SEPARATION", RequiredSeparation: &sep,
			RuleSetScope: "SYSTEM",
		}}})
		doc = searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Status == ConsolidationFeasible || !has(doc.Candidates[0].IndeterminateReasonCodes, "REQUIRE_SEPARATION") {
			t.Fatalf("separation %+v", doc.Candidates[0])
		}
	})
	t.Run("NLO03D_010_OCCUPANCY_EXCEEDS", func(t *testing.T) {
		w, src, _ := newFill(t)
		src.profiles[src.onboard.Items[0].CargoID] = profileWithWeight(src.profiles[src.onboard.Items[0].CargoID], 50000)
		doc := searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Status == ConsolidationFeasible || !has(doc.Candidates[0].Explanation, currenttrip.DimensionExceeds) {
			t.Fatalf("%+v", doc.Candidates[0])
		}
	})
	t.Run("NLO03D_011_POSITION_AND_ETA", func(t *testing.T) {
		for _, freshness := range []string{"STALE", "LOST", "UNKNOWN"} {
			w, src, _ := newFill(t)
			src.position.Freshness = freshness
			doc := searchFill(t, w, src.execution.ShipmentID)
			if doc.Candidates[0].Status == ConsolidationFeasible || !has(doc.Candidates[0].IndeterminateReasonCodes, ReasonPositionNotFresh) {
				t.Fatalf("%s %+v", freshness, doc.Candidates[0])
			}
		}
		w, src, _ := newFill(t)
		src.position.Freshness = "FRESH"
		doc := searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Status != ConsolidationFeasible {
			t.Fatalf("fresh %+v", doc.Candidates[0])
		}
		src.eta.FreshnessStatus = "STALE"
		doc = searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Status == ConsolidationFeasible || !has(doc.Candidates[0].IndeterminateReasonCodes, ReasonETANotFresh) {
			t.Fatalf("eta %+v", doc.Candidates[0])
		}
	})
	t.Run("NLO03D_012_WINDOWS", func(t *testing.T) {
		w, src, load := newFill(t)
		past := w.at.Add(-2 * time.Hour)
		load.PickupWindow = span(past, past.Add(time.Minute))
		replaceLoad(t, w, load)
		doc := searchFill(t, w, src.execution.ShipmentID)
		if !has(doc.Candidates[0].Explanation, ReasonPickupWindowMiss) {
			t.Fatalf("pickup %+v", doc.Candidates[0])
		}
		w, src, load = newFill(t)
		load.DeliveryWindow = span(past, past.Add(time.Minute))
		replaceLoad(t, w, load)
		doc = searchFill(t, w, src.execution.ShipmentID)
		if !has(doc.Candidates[0].Explanation, ReasonDeliveryWindowMiss) {
			t.Fatalf("delivery %+v", doc.Candidates[0])
		}
	})
	t.Run("NLO03D_013_ROUTING", func(t *testing.T) {
		doc := feasibleFill(t)
		if doc.Candidates[0].Routing == nil || doc.Candidates[0].Routing.DistanceM != 3000 || doc.Candidates[0].Routing.Provider == "" {
			t.Fatalf("%+v", doc.Candidates[0].Routing)
		}
		w, src, _ := newFill(t)
		w.routes.fail = true
		doc = searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Routing != nil || doc.Candidates[0].Status == ConsolidationFeasible || !has(doc.Candidates[0].IndeterminateReasonCodes, ReasonRoutingUnavailable) {
			t.Fatalf("%+v", doc.Candidates[0])
		}
	})
	t.Run("NLO03D_014_ONE_LOAD_ORDER_FINGERPRINT_PRIVACY", func(t *testing.T) {
		w, src, first := newFill(t)
		second := publishLoad(t, w, src, 50)
		doc := searchFill(t, w, src.execution.ShipmentID)
		if doc.EvaluatedPairCount != 2 || len(doc.Candidates) != 2 {
			t.Fatalf("%+v", doc)
		}
		for _, candidate := range doc.Candidates {
			if len(candidate.Members) != MaxAdditionalLoads {
				t.Fatalf("members %+v", candidate.Members)
			}
		}
		if doc.Candidates[0].Members[0].LoadOpportunityID.String() > doc.Candidates[1].Members[0].LoadOpportunityID.String() && doc.Candidates[0].Status == doc.Candidates[1].Status {
			t.Fatal("order")
		}
		again := searchFill(t, w, src.execution.ShipmentID)
		if again.Candidates[0].Members[0].LoadOpportunityID != doc.Candidates[0].Members[0].LoadOpportunityID {
			t.Fatal("unstable")
		}
		runs, rows, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
		if err != nil || runs.Pattern != PatternCurrentTripFill || len(rows) != 2 {
			t.Fatal(err)
		}
		before := rows[0].CandidateFingerprint
		first.Version = 1
		first.WeightKg = f64(80)
		replaceLoad(t, w, first)
		changed := searchFill(t, w, src.execution.ShipmentID)
		var next string
		for _, candidate := range changed.Candidates {
			if candidate.Members[0].LoadOpportunityID == first.ID {
				_, stored, getErr := w.store.GetConsolidation(context.Background(), w.carrier, changed.SearchID)
				if getErr != nil {
					t.Fatal(getErr)
				}
				for _, row := range stored {
					if row.Members[0].LoadOpportunityID == first.ID {
						next = row.CandidateFingerprint
					}
				}
			}
		}
		if next == "" || next == before {
			t.Fatalf("fingerprint %s %s", before, next)
		}
		_ = second
		w, src, load := newFill(t)
		load.VisibilityScope = domain.VisAnonymized
		replaceLoad(t, w, load)
		doc = searchFill(t, w, src.execution.ShipmentID)
		raw := mustJSON(doc)
		if load.Pickup.LocationID != nil && strings.Contains(raw, load.Pickup.LocationID.String()) {
			t.Fatalf("location leaked %s", raw)
		}
		if strings.Contains(raw, src.execution.ShipmentID.String()) && strings.Contains(raw, "driver") {
			t.Fatal("driver")
		}
	})
	t.Run("NLO03D_015_NO_EXECUTION_SIDE_EFFECTS", func(t *testing.T) {
		raw, err := os.ReadFile("current_trip_fill.go")
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		for _, forbidden := range []string{"InsertShipment", "CreateAssignment", "reservation", "carrier offer", "book slot", "Haversine", "StraightLineKm"} {
			if strings.Contains(text, forbidden) {
				t.Fatal(forbidden)
			}
		}
		doc := feasibleFill(t)
		if doc.Candidates[0].ExecutionSupported || doc.Candidates[0].PlacementCheck == "" {
			t.Fatal(doc.Candidates[0])
		}
	})
	t.Run("NLO03D_017_ODOR_AND_CONTAMINATION", func(t *testing.T) {
		w, src, load := newFill(t)
		load.Cargo.OdorEmissionClass = strPtr("HIGH")
		replaceLoad(t, w, load)
		profile := src.profiles[src.onboard.Items[0].CargoID]
		profile.OdorSensitive = boolPtr(true)
		src.profiles[src.onboard.Items[0].CargoID] = profile
		w.svc.UseCatalog(ruleCatalog{rules: []compat.Rule{{
			RuleKind: compat.KindCargoCargo, Layer: compat.LayerPlatform, Decision: compat.DecisionDeny,
			LeftSelectorType: "ODOR_SENSITIVE", LeftSelectorValue: "true", RightSelectorType: "ODOR_EMISSION_CLASS", RightSelectorValue: "HIGH",
			ReasonCode: "ODOR_COMPATIBILITY_RULE", Severity: compat.SeverityHard, RuleSetScope: "SYSTEM",
		}}})
		doc := searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Status == ConsolidationFeasible || !has(doc.Candidates[0].Explanation, "ODOR_COMPATIBILITY_RULE") {
			t.Fatalf("odor %+v", doc.Candidates[0])
		}
		w, src, load = newFill(t)
		load.Cargo.ContaminationClass = strPtr("HIGH")
		replaceLoad(t, w, load)
		profile = src.profiles[src.onboard.Items[0].CargoID]
		profile.FoodGradeRequired = boolPtr(true)
		src.profiles[src.onboard.Items[0].CargoID] = profile
		src.vehicle.FoodGradeCapability = boolPtr(true)
		w.svc.UseCatalog(ruleCatalog{rules: []compat.Rule{{
			RuleKind: compat.KindCargoCargo, Layer: compat.LayerPlatform, Decision: compat.DecisionDeny,
			LeftSelectorType: "CONTAMINATION_CLASS", LeftSelectorValue: "HIGH", RightSelectorType: "ANY",
			ReasonCode: "CONTAMINATION_COMPATIBILITY_RULE", Severity: compat.SeverityHard, RuleSetScope: "SYSTEM",
		}}})
		doc = searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Status == ConsolidationFeasible || !has(doc.Candidates[0].Explanation, "CONTAMINATION_COMPATIBILITY_RULE") {
			t.Fatalf("contamination %+v", doc.Candidates[0])
		}
	})
	t.Run("NLO03D_018_UNKNOWN_WINDOW_NOT_UNLIMITED", func(t *testing.T) {
		w, src, load := newFill(t)
		load.PickupWindow = domain.TimeWindow{}
		replaceLoad(t, w, load)
		doc := searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Status == ConsolidationFeasible || !has(doc.Candidates[0].IndeterminateReasonCodes, ReasonPickupUnknown) {
			t.Fatalf("%+v", doc.Candidates[0])
		}
	})
	t.Run("NLO03D_016_REHANDLING_CONFLICT", func(t *testing.T) {
		w, src, load := newFill(t)
		load.Pickup.LocationID = &src.execution.DestinationLocationID
		replaceLoad(t, w, load)
		doc := searchFill(t, w, src.execution.ShipmentID)
		if doc.Candidates[0].Status == ConsolidationFeasible || doc.Candidates[0].PlacementCheck != PlacementSequenceConflict || !has(doc.Candidates[0].Explanation, ReasonRehandlingConflict) {
			t.Fatalf("%+v", doc.Candidates[0])
		}
	})
}

func feasibleFill(t *testing.T) ConsolidationResponse {
	t.Helper()
	w, src, _ := newFill(t)
	return searchFill(t, w, src.execution.ShipmentID)
}

func newFill(t *testing.T) (*world, *fillSources, domain.LoadOpportunity) {
	t.Helper()
	w := newWorld(t)
	w.routes.baselineM = 1000
	shipment := uuid.New()
	vehicle := uuid.New()
	cargo := uuid.New()
	dest := uuid.New()
	lat, lon := 55.0, 37.0
	recorded := w.at
	arrival := w.at.Add(4 * time.Hour)
	src := &fillSources{
		execution: currenttrip.ShipmentExecution{
			ShipmentID: shipment, TenantID: w.carrier, ShipmentVersion: 3, ShipmentStatus: "IN_TRANSIT",
			VehicleID: &vehicle, OriginLocationID: uuid.New(), DestinationLocationID: dest,
		},
		onboard: currenttrip.OnboardCargo{ShipmentID: shipment, ShipmentVersion: 3, Resolution: currenttrip.EvidenceConfirmedOnboard, Items: []currenttrip.OnboardEvidenceItem{{
			CargoID: cargo, State: currenttrip.EvidenceConfirmedOnboard, StateVersion: 2, OccurredAt: w.at, Source: "DRIVER_OPERATION",
		}}},
		profiles: map[uuid.UUID]currenttrip.CargoProfile{cargo: {ID: cargo, Version: 4, WeightKg: f64(1000), VolumeM3: f64(2)}},
		vehicle: currenttrip.VehicleCapability{
			ID: vehicle, TenantID: w.carrier, Version: 5, CapacityWeight: f64(20000), CapacityVolume: f64(80),
			InternalHeightMM: intPtr(2700), LoadingAccess: []string{"SIDE"}, UnloadingAccess: []string{"REAR"},
			TemperatureControlMode: strPtr("ACTIVE"), TemperatureCapabilityMinC: f64(-20), TemperatureCapabilityMaxC: f64(20),
			PalletPositions: intPtr(33),
		},
		position: currenttrip.TrackingPosition{Freshness: "FRESH", Latitude: &lat, Longitude: &lon, RecordedAt: &recorded},
		eta:      currenttrip.ETA{FreshnessStatus: "FRESH", EstimatedArrivalAt: &arrival, SourceObservedAt: &recorded},
	}
	w.dir.snaps[dest] = domain.LocationSnapshot{ID: dest, Latitude: f64(56), Longitude: f64(38), CountryCode: "RU", City: "Dest"}
	w.svc.ConfigureCurrentTrip(currenttrip.NewProvider(src, src, src, src, src, src))
	w.svc.UseCatalog(emptyCatalog{})
	win := span(w.at.Add(-time.Hour), w.at.Add(48*time.Hour))
	load := publishLoad(t, w, src, 100)
	load.PickupWindow = win
	load.DeliveryWindow = win
	load.VolumeM3 = f64(1)
	replaceLoad(t, w, load)
	return w, src, load
}

func publishLoad(t *testing.T, w *world, src *fillSources, kg float64) domain.LoadOpportunity {
	t.Helper()
	origin, dest := uuid.New(), uuid.New()
	win := span(w.at.Add(-time.Hour), w.at.Add(48*time.Hour))
	pickup := located(origin, "Pickup")
	pickup.Latitude, pickup.Longitude = f64(55.1), f64(37.1)
	delivery := located(dest, "Drop")
	delivery.Latitude, delivery.Longitude = f64(55.2), f64(37.2)
	load := domain.LoadOpportunity{
		ID: uuid.New(), OwnerTenantID: w.shipper, SourceType: domain.SourceTransportOrder, SourceID: uuid.New(),
		Pickup: pickup, Delivery: delivery, PickupWindow: win, DeliveryWindow: win,
		VisibilityScope: domain.VisMarketplace, Status: domain.LoadPublished, Version: 1,
		CrossShipperConsolidationAllowed: true, WeightKg: f64(kg), VolumeM3: f64(1),
		CreatedAt: w.at, UpdatedAt: w.at,
	}
	if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		return tx.InsertLoad(context.Background(), load)
	}); err != nil {
		t.Fatal(err)
	}
	_ = src
	return load
}

func searchFill(t *testing.T, w *world, shipment uuid.UUID) ConsolidationResponse {
	t.Helper()
	result, err := w.svc.SearchConsolidation(context.Background(), w.actor(), ConsolidationCommand{ShipmentID: shipment, Pattern: PatternCurrentTripFill})
	if err != nil {
		t.Fatal(err)
	}
	var doc ConsolidationResponse
	if err := json.Unmarshal(result.Body, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func profileWithWeight(profile currenttrip.CargoProfile, kg float64) currenttrip.CargoProfile {
	profile.WeightKg = f64(kg)
	return profile
}

func profileWithLinear(profile currenttrip.CargoProfile, meters float64) currenttrip.CargoProfile {
	profile.LinearMeters = f64(meters)
	return profile
}

func TestNLO03DCapacityContext(t *testing.T) {
	t.Run("CURRENT_TRIP_PUBLIC_AND_DB_CAPACITY_ABSENT", func(t *testing.T) {
		w, src, _ := newFill(t)
		result, err := w.svc.SearchConsolidation(context.Background(), w.actor(), ConsolidationCommand{ShipmentID: src.execution.ShipmentID, Pattern: PatternCurrentTripFill})
		if err != nil {
			t.Fatal(err)
		}
		raw := string(result.Body)
		if strings.Contains(raw, `"capacity_id"`) || strings.Contains(raw, `"capacity_version"`) || strings.Contains(raw, "00000000-0000-0000-0000-000000000000") {
			t.Fatalf("%s", raw)
		}
		if !strings.Contains(raw, `"shipment_id"`) || !strings.Contains(raw, `"shipment_version"`) {
			t.Fatalf("%s", raw)
		}
		if strings.Contains(raw, `"compatibility_fingerprints"`) || strings.Contains(raw, `"rule_sets_used"`) || strings.Contains(raw, `"catalog_versions_used"`) {
			t.Fatalf("public response leaked audit provenance: %s", raw)
		}
		var doc ConsolidationResponse
		if err := json.Unmarshal(result.Body, &doc); err != nil {
			t.Fatal(err)
		}
		run, _, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
		if err != nil || run.CapacityID != nil || run.CapacityVersion != nil {
			t.Fatalf("%+v %v", run, err)
		}
	})
	t.Run("PAIRWISE_PUBLIC_AND_DB_CAPACITY_PRESENT", func(t *testing.T) {
		w, cap, _, _ := sameOwnerPair(t)
		result, err := w.svc.SearchConsolidation(context.Background(), w.actor(), ConsolidationCommand{CapacityID: cap.ID, Pattern: PatternSameOriginDestination})
		if err != nil {
			t.Fatal(err)
		}
		raw := string(result.Body)
		if !strings.Contains(raw, cap.ID.String()) || !strings.Contains(raw, `"capacity_version":`+jsonNumber(cap.Version)) {
			t.Fatalf("%s", raw)
		}
		var doc ConsolidationResponse
		if err := json.Unmarshal(result.Body, &doc); err != nil {
			t.Fatal(err)
		}
		if doc.CapacityID == nil || *doc.CapacityID != cap.ID || doc.CapacityVersion == nil || *doc.CapacityVersion != cap.Version {
			t.Fatalf("%+v", doc)
		}
		run, _, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
		if err != nil || run.CapacityID == nil || *run.CapacityID != cap.ID || run.CapacityVersion == nil || *run.CapacityVersion != cap.Version {
			t.Fatalf("%+v %v", run, err)
		}
	})
}

type versionedCatalog struct {
	ruleVersion    int
	catalogVersion int
}

func (c versionedCatalog) Evaluation(context.Context, uuid.UUID) (compat.Context, error) {
	return compat.Context{
		CargoCatalogVersion: c.catalogVersion,
		Rules: []compat.Rule{{
			RuleCode: "NOTE", RuleKind: compat.KindCargoCargo, Layer: compat.LayerPlatform,
			LeftSelectorType: "CARGO_TYPE", LeftSelectorValue: "UNMATCHED", RightSelectorType: "CARGO_TYPE", RightSelectorValue: "UNMATCHED",
			Decision: compat.DecisionAllow, ReasonCode: "NOTE", RuleSetID: "ruleset", RuleSetScope: "SYSTEM", RuleSetVersion: c.ruleVersion,
		}},
		RuleSets:    []compat.RuleSetRef{{ID: "ruleset", Scope: "SYSTEM", Version: c.ruleVersion}},
		CatalogRefs: []compat.CatalogVersionRef{{ID: "cargo-catalog", CatalogKind: "cargo", Scope: "SYSTEM", Version: c.catalogVersion}},
	}, nil
}

func TestNLO03DAuditFingerprint(t *testing.T) {
	t.Run("RULE_AND_CATALOG_VERSION_INVALIDATE_FINGERPRINT", func(t *testing.T) {
		w, src, load := newFill(t)
		w.svc.UseCatalog(versionedCatalog{ruleVersion: 1, catalogVersion: 4})
		first := searchFill(t, w, src.execution.ShipmentID)
		if first.Candidates[0].Status != ConsolidationFeasible || first.Candidates[0].Compatibility != compat.StatusCompatible {
			t.Fatalf("%+v", first.Candidates[0])
		}
		_, rows, err := w.store.GetConsolidation(context.Background(), w.carrier, first.SearchID)
		if err != nil {
			t.Fatal(err)
		}
		before := rows[0].CandidateFingerprint
		trace := string(rows[0].CompatibilityTrace)
		parts := strings.Split(rows[0].CompatibilityFingerprint, "|")
		if len(parts) == 0 || rows[0].CompatibilityFingerprint == "" || rows[0].CompatibilityFingerprint == "NOT_EVALUATED" {
			t.Fatalf("compatibility %s", rows[0].CompatibilityFingerprint)
		}
		for _, part := range parts {
			if part == "" || !strings.Contains(trace, part) {
				t.Fatalf("compatibility %s missing from %s", part, trace)
			}
		}
		if !strings.Contains(trace, `"version":1`) || !strings.Contains(trace, `"version":4`) {
			t.Fatal(trace)
		}
		for _, needle := range []string{`"shipment_id"`, `"shipment_version"`, `"vehicle_id"`, `"confirmed_onboard"`, `"cargo_version"`, `"evidence_state_version"`, `"additional_load_version"`, `"freshness":"FRESH"`, `"residual_capacity"`, `"provenance"`, `"policy"`, `"placement_check"`, `"rule_sets_used"`, `"catalog_versions_used"`, `"routing"`, `"distance_m"`, `"duration_seconds"`} {
			if !strings.Contains(trace, needle) {
				t.Fatalf("missing %s in %s", needle, trace)
			}
		}
		w.svc.UseCatalog(versionedCatalog{ruleVersion: 2, catalogVersion: 4})
		ruled := searchFill(t, w, src.execution.ShipmentID)
		if ruled.Candidates[0].Compatibility != compat.StatusCompatible {
			t.Fatalf("%+v", ruled.Candidates[0])
		}
		_, ruledRows, err := w.store.GetConsolidation(context.Background(), w.carrier, ruled.SearchID)
		if err != nil || ruledRows[0].CandidateFingerprint == before || !strings.Contains(string(ruledRows[0].CompatibilityTrace), `"version":2`) {
			t.Fatal(err)
		}
		w.svc.UseCatalog(versionedCatalog{ruleVersion: 2, catalogVersion: 9})
		cataloged := searchFill(t, w, src.execution.ShipmentID)
		if cataloged.Candidates[0].Compatibility != compat.StatusCompatible {
			t.Fatalf("%+v", cataloged.Candidates[0])
		}
		_, catalogRows, err := w.store.GetConsolidation(context.Background(), w.carrier, cataloged.SearchID)
		if err != nil || catalogRows[0].CandidateFingerprint == ruledRows[0].CandidateFingerprint || !strings.Contains(string(catalogRows[0].CompatibilityTrace), `"version":9`) {
			t.Fatal(err)
		}
		_ = load
	})
	t.Run("POLICY_CHANGES_FINGERPRINT_AND_IDENTICAL_INPUT_IS_STABLE", func(t *testing.T) {
		w, src, _ := newFill(t)
		first := searchFill(t, w, src.execution.ShipmentID)
		again := searchFill(t, w, src.execution.ShipmentID)
		_, firstRows, err := w.store.GetConsolidation(context.Background(), w.carrier, first.SearchID)
		if err != nil {
			t.Fatal(err)
		}
		_, againRows, err := w.store.GetConsolidation(context.Background(), w.carrier, again.SearchID)
		if err != nil || againRows[0].CandidateFingerprint != firstRows[0].CandidateFingerprint {
			t.Fatal(err)
		}
		allowed, err := w.svc.SearchConsolidation(context.Background(), w.actor(), ConsolidationCommand{ShipmentID: src.execution.ShipmentID, Pattern: PatternCurrentTripFill, Policy: PolicyRehandlingAllowed})
		if err != nil {
			t.Fatal(err)
		}
		var doc ConsolidationResponse
		if err := json.Unmarshal(allowed.Body, &doc); err != nil {
			t.Fatal(err)
		}
		_, allowedRows, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
		if err != nil || allowedRows[0].CandidateFingerprint == firstRows[0].CandidateFingerprint || !strings.Contains(string(allowedRows[0].CompatibilityTrace), PolicyRehandlingAllowed) {
			t.Fatalf("%v %s", err, allowedRows[0].CompatibilityTrace)
		}
	})
}

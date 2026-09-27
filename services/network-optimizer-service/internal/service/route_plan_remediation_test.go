package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/currenttrip"
	"github.com/freight-platform/network-optimizer-service/internal/domain"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routeplan"
)

func TestNLO04B_F001_KNOWN_EQUIPMENT_COMPATIBILITY(t *testing.T) {
	t.Run("TEMPERATURE_EQUIPMENT_MISMATCH", func(t *testing.T) {
		w, src, load := newFill(t)
		src.vehicle.TemperatureCapabilityMaxC = f64(0)
		rebindTrip(w, src)
		load.Cargo.TemperatureRequired = boolPtr(true)
		load.Cargo.TemperatureMinC = f64(2)
		load.Cargo.TemperatureMaxC = f64(8)
		replaceLoad(t, w, load)
		expectNoPlan(t, w, src, load)
	})
	t.Run("ADR_REQUIRED_BUT_VEHICLE_NOT_ADR", func(t *testing.T) {
		w, src, load := newFill(t)
		src.vehicle.ADRCapability = boolPtr(false)
		rebindTrip(w, src)
		load.Cargo.Dangerous = boolPtr(true)
		replaceLoad(t, w, load)
		expectNoPlan(t, w, src, load)
	})
	t.Run("FOOD_GRADE_REQUIRED_BUT_UNSUPPORTED", func(t *testing.T) {
		w, src, load := newFill(t)
		src.vehicle.FoodGradeCapability = boolPtr(false)
		rebindTrip(w, src)
		load.Cargo.FoodGradeRequired = boolPtr(true)
		replaceLoad(t, w, load)
		expectNoPlan(t, w, src, load)
	})
	t.Run("BODY_TYPE_MISMATCH", func(t *testing.T) {
		w, src, load := newFill(t)
		src.vehicle.BodyType = strPtr("TENT")
		rebindTrip(w, src)
		load.Cargo.RequiredBodyTypes = []string{"REFRIGERATOR"}
		replaceLoad(t, w, load)
		expectNoPlan(t, w, src, load)
	})
	t.Run("REQUIRED_LOADING_ACCESS_UNSUPPORTED", func(t *testing.T) {
		w, src, load := newFill(t)
		src.vehicle.LoadingAccess = []string{"TOP"}
		rebindTrip(w, src)
		load.Cargo.RequiredLoadingAccess = []string{"SIDE"}
		replaceLoad(t, w, load)
		expectNoPlan(t, w, src, load)
	})
	t.Run("REQUIRED_UNLOADING_ACCESS_UNSUPPORTED", func(t *testing.T) {
		w, src, load := newFill(t)
		src.vehicle.UnloadingAccess = []string{"TOP"}
		rebindTrip(w, src)
		load.Cargo.RequiredUnloadingAccess = []string{"REAR"}
		replaceLoad(t, w, load)
		expectNoPlan(t, w, src, load)
	})
	t.Run("KNOWN_COMPATIBLE_VEHICLE_NOT_INDETERMINATE_FROM_EMPTY_EQUIPMENT", func(t *testing.T) {
		w, src, load := newFill(t)
		load.Cargo.TemperatureRequired = boolPtr(true)
		load.Cargo.TemperatureMinC = f64(2)
		load.Cargo.TemperatureMaxC = f64(8)
		replaceLoad(t, w, load)
		if _, err := planCurrentTrip(w, src, load, "compatible"); err != nil {
			t.Fatal(err)
		}
	})
}

func TestNLO04B_F001_ACCESS_NEEDS_PRESERVED(t *testing.T) {
	w, src, load := newFill(t)
	src.vehicle.LoadingAccess = []string{"TOP"}
	rebindTrip(w, src)
	load.Cargo.RequiredLoadingAccess = []string{"SIDE"}
	load.Cargo.RequiredBodyTypes = []string{"BOX"}
	load.Equipment = []string{"TAIL_LIFT"}
	replaceLoad(t, w, load)
	expectNoPlan(t, w, src, load)
}

func TestNLO04B_F002_UNKNOWN_ROUTING_PROFILE_INDETERMINATE(t *testing.T) {
	w, src, load := newFill(t)
	created := mustPlan(t, w, src, load, "profile")
	if created["result_status"] != routeplan.ResultIndeterminate || !reasonListed(created["reason_codes"], routeplan.ReasonRoutingProfileUnknown) {
		t.Fatalf("%v", created["result_status"])
	}
	if created["result_status"] == routeplan.ResultFeasible {
		t.Fatal("feasible with unknown routing profile")
	}
}

func TestNLO04B_F002_PROVIDER_DEFAULT_AUDIT(t *testing.T) {
	w, src, load := newFill(t)
	created := mustPlan(t, w, src, load, "defaults")
	for _, leg := range created["legs"].([]any) {
		if leg.(map[string]any)["provider_default_used"] != true {
			t.Fatalf("%v", leg)
		}
	}
}

func TestNLO04B_F004_REAL_CATALOG_DEPENDENCY(t *testing.T) {
	w, src, load := newFill(t)
	load.OwnerTenantID = w.carrier
	load.ConsolidationAllowed = true
	replaceLoad(t, w, load)
	w.svc.UseCatalog(versionedCatalog{ruleVersion: 4, catalogVersion: 9})
	created := mustPlan(t, w, src, load, "catalog-a")
	catalogA, ruleA, algorithm := dependencyPrints(t, created)
	if catalogA == routeplan.AlgorithmPolicyVersion || ruleA == routeplan.AlgorithmPolicyVersion || algorithm != routeplan.AlgorithmPolicyVersion {
		t.Fatalf("catalog %s rule %s algorithm %s", catalogA, ruleA, algorithm)
	}
	w.svc.UseCatalog(versionedCatalog{ruleVersion: 5, catalogVersion: 10})
	other := mustPlan(t, w, src, load, "catalog-b")
	catalogB, ruleB, _ := dependencyPrints(t, other)
	if catalogA == catalogB || ruleA == ruleB {
		t.Fatal("catalog or rule version did not change the dependency fingerprint")
	}
}

func TestNLO04B_F005_ANONYMIZED_POST_PRIVACY(t *testing.T) {
	doc, secretLat, secretID, owner := anonymizedPlan(t, "privacy-post")
	raw, _ := json.Marshal(doc)
	assertAnonymized(t, string(raw), secretLat, secretID, owner)
}

func TestNLO04B_F005_ANONYMIZED_GET_PRIVACY(t *testing.T) {
	w, src, load, secretLat, secretID := anonymizedWorld(t)
	created := mustPlan(t, w, src, load, "privacy-get")
	got, err := w.svc.GetRoutePlan(context.Background(), Actor{TenantID: w.carrier, UserID: uuid.New()}, uuid.MustParse(created["id"].(string)))
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Body) != string(planJSON(created)) && !sameProjection(t, got.Body, planJSON(created)) {
		t.Fatal("post and get projections differ")
	}
	assertAnonymized(t, string(got.Body), secretLat, secretID, w.shipper)
}

func TestNLO04B_F005_PUBLIC_SUBJECT_SNAPSHOT(t *testing.T) {
	w, src, load, secretLat, secretID := anonymizedWorld(t)
	created := mustPlan(t, w, src, load, "snapshot")
	raw := planJSON(created)
	if !strings.Contains(string(raw), "public_subject_snapshot") {
		t.Fatal("public snapshot missing")
	}
	assertAnonymized(t, string(raw), secretLat, secretID, w.shipper)
	var graph repository.RoutePlanGraph
	err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		var getErr error
		graph, getErr = tx.GetRoutePlan(context.Background(), w.carrier, uuid.MustParse(created["id"].(string)))
		return getErr
	})
	if err != nil {
		t.Fatal(err)
	}
	foundExact := false
	for _, stop := range graph.Stops {
		if stop.StopRole == routeplan.RoleCargo && stop.Latitude == secretLat && stop.LocationID != nil && *stop.LocationID == secretID {
			foundExact = true
		}
	}
	if !foundExact {
		t.Fatal("internal cargo point was not stored exactly")
	}
	for _, action := range graph.Actions {
		if action.SubjectID == load.ID && !strings.Contains(string(action.PublicSubjectSnapshot), load.ID.String()) {
			t.Fatal("snapshot missing load id")
		}
		if strings.Contains(string(action.PublicSubjectSnapshot), secretID.String()) || strings.Contains(string(action.PublicSubjectSnapshot), w.shipper.String()) {
			t.Fatal("snapshot leaked exact geography or owner")
		}
	}
}

func TestNLO04B_F007_EXISTING_CAPACITY_EXCEEDED(t *testing.T) {
	t.Run("CURRENT_TRIP_PAYLOAD_ALREADY_EXCEEDED", func(t *testing.T) {
		w, src, load := newFill(t)
		profile := src.profiles[src.onboard.Items[0].CargoID]
		profile.WeightKg = f64(50000)
		src.profiles[src.onboard.Items[0].CargoID] = profile
		src.vehicle.CapacityWeight = f64(1000)
		rebindTrip(w, src)
		expectNoPlan(t, w, src, load)
		expectNoRows(t, w)
	})
	t.Run("CURRENT_TRIP_VOLUME_ALREADY_EXCEEDED", func(t *testing.T) {
		w, src, load := newFill(t)
		profile := src.profiles[src.onboard.Items[0].CargoID]
		profile.VolumeM3 = f64(90)
		src.profiles[src.onboard.Items[0].CargoID] = profile
		src.vehicle.CapacityVolume = f64(10)
		rebindTrip(w, src)
		expectNoPlan(t, w, src, load)
		expectNoRows(t, w)
	})
	t.Run("CURRENT_TRIP_HEIGHT_ALREADY_EXCEEDED", func(t *testing.T) {
		w, src, load := newFill(t)
		profile := src.profiles[src.onboard.Items[0].CargoID]
		profile.MaxLoadedHeightMM = intPtr(3000)
		src.profiles[src.onboard.Items[0].CargoID] = profile
		src.vehicle.InternalHeightMM = intPtr(2000)
		rebindTrip(w, src)
		expectNoPlan(t, w, src, load)
		expectNoRows(t, w)
	})
	t.Run("UNKNOWN_REMAINS_INDETERMINATE", func(t *testing.T) {
		w, src, load := newFill(t)
		src.vehicle.CapacityWeight = nil
		src.vehicle.CapacityVolume = nil
		rebindTrip(w, src)
		created := mustPlan(t, w, src, load, "unknown-capacity")
		if created["result_status"] != routeplan.ResultIndeterminate {
			t.Fatalf("%v", created["result_status"])
		}
	})
}

func rebindTrip(w *world, src *fillSources) {
	w.svc.ConfigureCurrentTrip(currenttrip.NewProvider(src, src, src, src, src, src))
}

func planCurrentTrip(w *world, src *fillSources, load domain.LoadOpportunity, key string) (map[string]any, error) {
	body := `{"planning_mode":"CURRENT_TRIP","shipment_id":"` + src.execution.ShipmentID.String() + `","candidate_load_ids":["` + load.ID.String() + `"]}`
	var cmd RoutePlanCommand
	if err := json.Unmarshal([]byte(body), &cmd); err != nil {
		return nil, err
	}
	cmd.Raw = []byte(body)
	w.svc.UseRouting(&countingRoute{})
	w.svc.SetClock(func() time.Time { return w.at })
	result, err := w.svc.EvaluateRoutePlan(context.Background(), Actor{TenantID: w.carrier, UserID: uuid.New()}, key, cmd)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(result.Body, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func mustPlan(t *testing.T, w *world, src *fillSources, load domain.LoadOpportunity, key string) map[string]any {
	t.Helper()
	doc, err := planCurrentTrip(w, src, load, key)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func expectNoPlan(t *testing.T, w *world, src *fillSources, load domain.LoadOpportunity) {
	t.Helper()
	_, err := planCurrentTrip(w, src, load, "reject-"+uuid.NewString())
	var app *apperrors.AppError
	if !errors.As(err, &app) || app.Message != routeplan.ResultNoPlan {
		t.Fatalf("%v", err)
	}
}

func expectNoRows(t *testing.T, w *world) {
	t.Helper()
	events, err := w.store.ListOutbox(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.EventName == routePlanEvaluatedEvent {
			t.Fatal("event emitted for rejected plan")
		}
	}
}

func anonymizedWorld(t *testing.T) (*world, *fillSources, domain.LoadOpportunity, float64, uuid.UUID) {
	t.Helper()
	w, src, load := newFill(t)
	secretLat := 12.345678
	secretID := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	load.VisibilityScope = domain.VisAnonymized
	load.CrossShipperConsolidationAllowed = true
	load.Pickup = domain.Place{LocationID: &secretID, Latitude: &secretLat, Longitude: f64(77.654321), CountryCode: "RU", Region: "Secret", City: "Hidden"}
	load.Delivery = domain.Place{LocationID: &secretID, Latitude: &secretLat, Longitude: f64(77.654321), CountryCode: "RU", Region: "Secret", City: "Hidden"}
	replaceLoad(t, w, load)
	return w, src, load, secretLat, secretID
}

func anonymizedPlan(t *testing.T, key string) (map[string]any, float64, uuid.UUID, uuid.UUID) {
	t.Helper()
	w, src, load, secretLat, secretID := anonymizedWorld(t)
	return mustPlan(t, w, src, load, key), secretLat, secretID, w.shipper
}

func assertAnonymized(t *testing.T, raw string, secretLat float64, secretID, owner uuid.UUID) {
	t.Helper()
	if strings.Contains(raw, secretID.String()) || strings.Contains(raw, owner.String()) || strings.Contains(raw, "12.345678") || strings.Contains(raw, "77.654321") {
		t.Fatalf("exact geography or owner leaked in %s", raw)
	}
	_ = secretLat
}

func reasonListed(value any, code string) bool {
	codes, _ := value.([]any)
	for _, item := range codes {
		if item == code {
			return true
		}
	}
	return false
}

func dependencyPrints(t *testing.T, doc map[string]any) (string, string, string) {
	t.Helper()
	var catalog, rules, algorithm string
	for _, item := range doc["dependencies"].([]any) {
		row := item.(map[string]any)
		switch row["dependency_kind"] {
		case "CATALOG":
			catalog, _ = row["fingerprint"].(string)
		case "RULE_SET":
			rules, _ = row["fingerprint"].(string)
		case "ALGORITHM_POLICY":
			algorithm, _ = row["fingerprint"].(string)
		}
	}
	if catalog == "" || rules == "" || algorithm == "" {
		t.Fatalf("dependencies %+v", doc["dependencies"])
	}
	return catalog, rules, algorithm
}

func planJSON(doc map[string]any) []byte {
	raw, _ := json.Marshal(doc)
	return raw
}

func sameProjection(t *testing.T, got, want []byte) bool {
	t.Helper()
	var left, right map[string]any
	if err := json.Unmarshal(got, &left); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want, &right); err != nil {
		t.Fatal(err)
	}
	leftStops, _ := json.Marshal(left["stops"])
	rightStops, _ := json.Marshal(right["stops"])
	return string(leftStops) == string(rightStops)
}

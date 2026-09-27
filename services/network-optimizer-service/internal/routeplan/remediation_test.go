package routeplan

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

func TestNLO04B_F001_KNOWN_EQUIPMENT_COMPATIBILITY(t *testing.T) {
	body := "REFRIGERATOR"
	mode := "ACTIVE"
	minC, maxC := -20.0, 20.0
	yes := true
	no := false
	known := compat.Equipment{
		BodyType: &body, LoadingAccess: []string{"SIDE", "REAR"}, UnloadingAccess: []string{"REAR"},
		TemperatureControlMode: &mode, TemperatureMinC: &minC, TemperatureMaxC: &maxC,
		FoodGradeCapability: &yes, ADRCapability: &yes,
	}
	temp := compat.Cargo{ID: "temp", TemperatureRequired: &yes, TemperatureMinC: f64p(2), TemperatureMaxC: f64p(8)}
	hot := compat.Equipment{TemperatureControlMode: &mode, TemperatureMinC: f64p(-25), TemperatureMaxC: f64p(0)}
	assertHard(t, "TEMPERATURE_EQUIPMENT_MISMATCH", hot, temp, compat.AccessNeed{})
	assertFeasible(t, "TEMPERATURE_KNOWN_EQUIPMENT_NOT_INDETERMINATE", known, temp, compat.AccessNeed{})
	_, _, emptyErr := (Planner{}).Plan(context.Background(), equippedInput(compat.Equipment{}, temp, compat.AccessNeed{}))
	if emptyErr == nil {
		t.Fatal("empty equipment produced a feasible plan")
	}

	adrCargo := compat.Cargo{ID: "adr", DangerousGoods: &yes}
	assertHard(t, "ADR_REQUIRED_BUT_VEHICLE_NOT_ADR", compat.Equipment{ADRCapability: &no}, adrCargo, compat.AccessNeed{})
	food := compat.Cargo{ID: "food", FoodGradeRequired: &yes}
	assertHard(t, "FOOD_GRADE_REQUIRED_BUT_UNSUPPORTED", compat.Equipment{FoodGradeCapability: &no}, food, compat.AccessNeed{})
	assertHard(t, "BODY_TYPE_MISMATCH", compat.Equipment{BodyType: strp("TENT")}, compat.Cargo{ID: "body"}, compat.AccessNeed{RequiredBodyTypes: []string{"REFRIGERATOR"}})
	assertHard(t, "REQUIRED_LOADING_ACCESS_UNSUPPORTED", compat.Equipment{LoadingAccess: []string{"TOP"}}, compat.Cargo{ID: "load"}, compat.AccessNeed{RequiredLoadingAccess: []string{"SIDE"}})
	assertHard(t, "REQUIRED_UNLOADING_ACCESS_UNSUPPORTED", compat.Equipment{UnloadingAccess: []string{"TOP"}}, compat.Cargo{ID: "unload"}, compat.AccessNeed{RequiredUnloadingAccess: []string{"REAR"}})
}

func TestNLO04B_F001_ACCESS_NEEDS_PRESERVED(t *testing.T) {
	var seen compat.AccessNeed
	load := sampleLoad()
	load.PickupAction.AccessNeed = compat.AccessNeed{RequiredLoadingAccess: []string{"SIDE"}, RequiredBodyTypes: []string{"REFRIGERATOR"}}
	load.DeliveryAction.AccessNeed = load.PickupAction.AccessNeed
	_, _, err := (Planner{}).Plan(context.Background(), baseInput(load, func(items []compat.GroupageItem) compat.Result {
		if len(items) > 0 {
			seen = items[0].AccessNeed
		}
		return compat.Result{Status: compat.StatusCompatible}
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(seen.RequiredLoadingAccess) != 1 || seen.RequiredLoadingAccess[0] != "SIDE" || len(seen.RequiredBodyTypes) != 1 {
		t.Fatalf("access need dropped %#v", seen)
	}
}

func TestNLO04B_F002_UNKNOWN_ROUTING_PROFILE_INDETERMINATE(t *testing.T) {
	load := sampleLoad()
	in := baseInput(load, compatibleGroupage)
	in.VehicleProfile = routing.VehicleProfile{}
	in.ServiceDurationSeconds = intPtr(0)
	out, _, err := (Planner{}).Plan(context.Background(), in)
	if err != nil || out.ResultStatus != ResultIndeterminate || !contains(out.ReasonCodes, ReasonRoutingProfileUnknown) {
		t.Fatalf("%s %v %v", out.ResultStatus, out.ReasonCodes, err)
	}
	for _, leg := range out.Legs {
		if !leg.ProviderDefaultUsed {
			t.Fatal("unknown profile claimed a known routing proof")
		}
	}
	in.VehicleProfile = completeProfile()
	known, _, err := (Planner{}).Plan(context.Background(), in)
	if err != nil || known.ResultStatus != ResultFeasible {
		t.Fatalf("complete profile %s %v %v", known.ResultStatus, known.ReasonCodes, err)
	}
	for _, leg := range known.Legs {
		if leg.ProviderDefaultUsed {
			t.Fatal("complete profile marked provider defaults")
		}
	}
}

func TestNLO04B_F002_PROVIDER_DEFAULT_AUDIT(t *testing.T) {
	load := sampleLoad()
	in := baseInput(load, compatibleGroupage)
	out, _, err := (Planner{}).Plan(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	for _, leg := range out.Legs {
		if !leg.ProviderDefaultUsed || leg.ResponseFingerprint == "" {
			t.Fatalf("%+v", leg)
		}
	}
}

func TestNLO04B_F003_SEQUENCE_LOCAL_ROUTING_FAILURE(t *testing.T) {
	first := sampleLoad()
	second := loadAt("00000000-0000-0000-0000-0000000000d1", "00000000-0000-0000-0000-0000000000d2", "00000000-0000-0000-0000-0000000000d3", 5)
	second.Pickup = canonical(uuid.MustParse("00000000-0000-0000-0000-0000000000d2"), 40, 40)
	in := baseInput(first, compatibleGroupage)
	in.Loads = []Load{first, second}
	in.VehicleProfile = completeProfile()
	in.ServiceDurationSeconds = intPtr(0)
	notFound := &selectiveRoute{code: routing.ErrRouteNotFound}
	in.Route = notFound
	out, _, err := (Planner{}).Plan(context.Background(), in)
	if err != nil || out.ResultStatus != ResultFeasible {
		t.Fatalf("first failure aborted search %v %s %v", err, out.ResultStatus, out.ReasonCodes)
	}
	if notFound.failed == 0 || notFound.calls <= notFound.failed {
		t.Fatalf("calls %d failed %d", notFound.calls, notFound.failed)
	}
	for _, leg := range out.Legs {
		if leg.DistanceM == 0 || leg.DurationSeconds == 0 {
			t.Fatalf("failed sequence persisted %+v", leg)
		}
	}
	timeout := &selectiveRoute{code: routing.ErrTimeout}
	in.Route = timeout
	if _, _, err := (Planner{}).Plan(context.Background(), in); err != nil {
		t.Fatal(err)
	}
}

func TestNLO04B_F003_ALL_ROUTING_FAILURE(t *testing.T) {
	load := sampleLoad()
	in := baseInput(load, compatibleGroupage)
	in.Route = &selectiveRoute{code: routing.ErrRouteNotFound, all: true}
	_, _, err := (Planner{}).Plan(context.Background(), in)
	var search *SearchError
	if !errors.As(err, &search) || search.Code != ResultRoutingDown {
		t.Fatalf("got %v", err)
	}
}

func TestNLO04B_F004_FINGERPRINT_LOAD_VERSION(t *testing.T) {
	first := planned(t, 1, nil)
	second := planned(t, 2, nil)
	if EvaluationFingerprint(first) == EvaluationFingerprint(second) {
		t.Fatal("load version did not change the evaluation fingerprint")
	}
}

func TestNLO04B_F004_FINGERPRINT_WINDOWS(t *testing.T) {
	window := time.Unix(10, 0).UTC()
	same := planned(t, 1, nil)
	other := planned(t, 1, &window)
	if EvaluationFingerprint(same) == EvaluationFingerprint(other) {
		t.Fatal("pickup window did not change the evaluation fingerprint")
	}
}

func TestNLO04B_F004_FINGERPRINT_CONTEXT(t *testing.T) {
	out := planned(t, 1, nil)
	other := out
	other.ContextFingerprint = "trip-context-2"
	if EvaluationFingerprint(out) == EvaluationFingerprint(other) {
		t.Fatal("context fingerprint ignored")
	}
}

func TestNLO04B_F004_FINGERPRINT_CATALOG_RULES(t *testing.T) {
	out := planned(t, 1, nil)
	other := out
	other.CatalogFingerprint = "catalog-b"
	other.RuleFingerprint = "rules-b"
	if EvaluationFingerprint(out) == EvaluationFingerprint(other) {
		t.Fatal("catalog or rule fingerprint ignored")
	}
	policy := out
	policy.ReasonCodes = append([]string{}, out.ReasonCodes...)
	if EvaluationFingerprint(policy) == "" {
		t.Fatal("empty fingerprint")
	}
}

func TestNLO04B_F006_CACHE_EXPIRY(t *testing.T) {
	route := &scriptRoute{}
	in := Input{Clock: time.Unix(0, 0), Route: route, cache: map[string]Leg{}, VehicleProfile: completeProfile()}
	budget := NewBudget(func() time.Time { return time.Unix(0, 0) })
	from := Stop{Point: anchor(1, 1)}
	to := Stop{Point: canonical(uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), 2, 2)}
	depart := in.Clock
	if _, err := routeLeg(context.Background(), in, budget, from, to, &depart); err != nil {
		t.Fatal(err)
	}
	if _, err := routeLeg(context.Background(), in, budget, from, to, &depart); err != nil {
		t.Fatal(err)
	}
	if route.calls != 1 {
		t.Fatalf("unexpired cache calls %d", route.calls)
	}
	in.Clock = time.Unix(3, 0)
	if _, err := routeLeg(context.Background(), in, budget, from, to, &depart); err != nil {
		t.Fatal(err)
	}
	if route.calls != 2 {
		t.Fatalf("expired cache calls %d", route.calls)
	}
}

func TestNLO04B_F006_RESPONSE_PROOF_FINGERPRINT(t *testing.T) {
	route := &scriptRoute{}
	in := Input{Clock: time.Unix(0, 0), Route: route, cache: map[string]Leg{}, VehicleProfile: completeProfile()}
	budget := NewBudget(func() time.Time { return time.Unix(0, 0) })
	from := Stop{Point: anchor(1, 1)}
	to := Stop{Point: canonical(uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"), 3, 3)}
	depart := in.Clock
	first, err := routeLeg(context.Background(), in, budget, from, to, &depart)
	if err != nil {
		t.Fatal(err)
	}
	key := LegKey(from.Point.Fingerprint(), to.Point.Fingerprint(), in.VehicleProfile.Hash(), routing.RouteFastest, first.TrafficMode, first.DepartureBucket)
	if first.RequestFingerprint == "" || first.RequestFingerprint == key || first.ResponseFingerprint == first.RequestFingerprint {
		t.Fatalf("fingerprints collapsed %s %s %s", key, first.RequestFingerprint, first.ResponseFingerprint)
	}
	again := ProofFingerprint(first)
	if again != first.ResponseFingerprint {
		t.Fatal("same response proof changed")
	}
	shifted := first
	shifted.DistanceM += 10
	if ProofFingerprint(shifted) == first.ResponseFingerprint {
		t.Fatal("distance change kept the same response proof")
	}
}

func TestNLO04B_F008_FIRST_KNOWN_WINDOW_HARD_REJECT(t *testing.T) {
	lateEnd := time.Unix(1001, 0)
	load := sampleLoad()
	load.PickupAction.WindowEnd = &lateEnd
	in := baseInput(load, compatibleGroupage)
	in.Clock = time.Unix(1000, 0)
	in.ServiceDurationSeconds = nil
	if _, _, err := (Planner{}).Plan(context.Background(), in); err == nil {
		t.Fatal("known late first stop was not rejected")
	}
	far := time.Unix(100000, 0)
	load.PickupAction.WindowEnd = &far
	in.Loads = []Load{load}
	out, _, err := (Planner{}).Plan(context.Background(), in)
	if err != nil || out.ResultStatus != ResultIndeterminate || !contains(out.ReasonCodes, ReasonServiceDurationUnknown) {
		t.Fatalf("in-window unknown duration %s %v %v", out.ResultStatus, out.ReasonCodes, err)
	}
	downstream := sampleLoad()
	past := time.Unix(1, 0)
	downstream.DeliveryAction.WindowEnd = &past
	in.Loads = []Load{downstream}
	down, _, err := (Planner{}).Plan(context.Background(), in)
	if err != nil || down.ResultStatus != ResultIndeterminate {
		t.Fatalf("downstream window falsely hard-rejected %v %s", err, down.ResultStatus)
	}
	early := time.Unix(5000, 0)
	known := sampleLoad()
	known.PickupAction.WindowStart = &early
	in.Loads = []Load{known}
	in.ServiceDurationSeconds = intPtr(60)
	in.VehicleProfile = completeProfile()
	waited, _, err := (Planner{}).Plan(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	var pickup *Stop
	for i := range waited.Stops {
		if waited.Stops[i].Role == RoleCargo && waited.Stops[i].Depart != nil {
			pickup = &waited.Stops[i]
			break
		}
	}
	if pickup == nil || !pickup.Depart.Equal(early.Add(60*time.Second)) {
		t.Fatalf("early wait depart %+v", pickup)
	}
}

type selectiveRoute struct {
	calls  int
	failed int
	code   error
	all    bool
}

func (s *selectiveRoute) Route(_ context.Context, req routing.RouteRequest) (routing.RouteResult, error) {
	s.calls++
	if s.all || (req.Origin.Latitude == 1 && req.Destination.Latitude == 40) {
		s.failed++
		return routing.RouteResult{}, s.code
	}
	return routing.RouteResult{
		Provider: "script", DistanceM: 1000, DurationSeconds: 100,
		CalculatedAt: time.Unix(1, 0), ExpiresAt: time.Unix(2, 0),
	}, nil
}

func (s *selectiveRoute) Matrix(context.Context, routing.MatrixRequest) (routing.MatrixResult, error) {
	return routing.MatrixResult{}, routing.ErrInvalidResponse
}

func assertHard(t *testing.T, name string, equipment compat.Equipment, cargo compat.Cargo, access compat.AccessNeed) {
	t.Helper()
	_, _, err := (Planner{}).Plan(context.Background(), equippedInput(equipment, cargo, access))
	var search *SearchError
	if !errors.As(err, &search) || search.Code != ResultNoPlan {
		t.Fatalf("%s: %v", name, err)
	}
}

func assertFeasible(t *testing.T, name string, equipment compat.Equipment, cargo compat.Cargo, access compat.AccessNeed) {
	t.Helper()
	out, _, err := (Planner{}).Plan(context.Background(), equippedInput(equipment, cargo, access))
	if err != nil || out.ResultStatus != ResultFeasible {
		t.Fatalf("%s: %s %v %v", name, out.ResultStatus, out.ReasonCodes, err)
	}
}

func equippedInput(equipment compat.Equipment, cargo compat.Cargo, access compat.AccessNeed) Input {
	load := sampleLoad()
	load.PickupAction.Cargo = cargo
	load.DeliveryAction.Cargo = cargo
	load.PickupAction.AccessNeed = access
	load.DeliveryAction.AccessNeed = access
	in := baseInput(load, func(items []compat.GroupageItem) compat.Result {
		return compat.EvaluateGroupageItems(equipment, items, compat.Context{})
	})
	in.VehicleProfile = completeProfile()
	in.ServiceDurationSeconds = intPtr(0)
	return in
}

func sampleLoad() Load {
	return loadAt("00000000-0000-0000-0000-0000000000c1", "00000000-0000-0000-0000-0000000000c2", "00000000-0000-0000-0000-0000000000c3", 10)
}

func baseInput(load Load, groupage func([]compat.GroupageItem) compat.Result) Input {
	return Input{
		Mode: ModeDepotStart, Start: Stop{Role: RoleStart, Point: canonical(uuid.MustParse("00000000-0000-0000-0000-0000000000c4"), 1, 1)},
		Loads: []Load{load}, Initial: knownCapacity(100), Clock: time.Unix(0, 0),
		Route: &scriptRoute{}, Groupage: groupage, ServiceDurationSeconds: intPtr(0),
	}
}

func planned(t *testing.T, version int, pickupStart *time.Time) Outcome {
	t.Helper()
	load := sampleLoad()
	load.Version = version
	load.PickupAction.SubjectVersion = version
	load.DeliveryAction.SubjectVersion = version
	load.PickupAction.WindowStart = pickupStart
	in := baseInput(load, compatibleGroupage)
	in.VehicleProfile = completeProfile()
	out, _, err := (Planner{}).Plan(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func f64p(v float64) *float64 { return &v }
func strp(v string) *string   { return &v }

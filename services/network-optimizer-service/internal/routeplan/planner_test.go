package routeplan

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

func TestPointFingerprintStable(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	observed := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	anchor := Point{Kind: PointAnchor, Latitude: 55.75, Longitude: 37.61, Source: SourceTracking, ObservedAt: &observed}
	again := Point{Kind: PointAnchor, Latitude: 55.75, Longitude: 37.61, Source: SourceTracking, ObservedAt: &observed}
	if anchor.Fingerprint() != again.Fingerprint() {
		t.Fatal("anchor fingerprint drifted")
	}
	canonical := Point{Kind: PointCanonical, LocationID: &id, Latitude: 55.75, Longitude: 37.61, Source: SourceCanonical}
	if canonical.Fingerprint() == anchor.Fingerprint() {
		t.Fatal("canonical and anchor fingerprints collided")
	}
	shifted := anchor
	later := observed.Add(time.Second)
	shifted.ObservedAt = &later
	if shifted.Fingerprint() == anchor.Fingerprint() {
		t.Fatal("observed_at ignored")
	}
	moved := anchor
	moved.Latitude = 55.7500001
	if moved.Fingerprint() == anchor.Fingerprint() {
		t.Fatal("coordinate tolerance applied")
	}
}

func TestLegKeyChanges(t *testing.T) {
	base := LegKey("a", "b", "veh", routing.RouteFastest, routing.TrafficStatistical, "2026-09-27T10:00:00Z")
	if base == LegKey("z", "b", "veh", routing.RouteFastest, routing.TrafficStatistical, "2026-09-27T10:00:00Z") {
		t.Fatal("start fingerprint ignored")
	}
	if base == LegKey("a", "b", "veh", routing.RouteFastest, routing.TrafficStatistical, "2026-09-27T10:15:00Z") {
		t.Fatal("departure bucket ignored")
	}
	if base == LegKey("a", "b", "other", routing.RouteFastest, routing.TrafficStatistical, "2026-09-27T10:00:00Z") {
		t.Fatal("vehicle profile ignored")
	}
	if base == LegKey("a", "b", "veh", routing.RouteShortest, routing.TrafficStatistical, "2026-09-27T10:00:00Z") {
		t.Fatal("route mode ignored")
	}
	if base == LegKey("a", "b", "veh", routing.RouteFastest, routing.TrafficCurrent, "2026-09-27T10:00:00Z") {
		t.Fatal("traffic mode ignored")
	}
}

func TestBudgetStopsBeforeExceeding(t *testing.T) {
	b := NewBudget(func() time.Time { return time.Unix(0, 0) })
	for i := 0; i < MaxSequenceCandidates; i++ {
		if err := b.openSequence(); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.openSequence(); err == nil || b.SequenceCandidates != MaxSequenceCandidates {
		t.Fatalf("sequences %d err %v", b.SequenceCandidates, err)
	}
	b = NewBudget(nil)
	for i := 0; i < MaxRouteLegEvaluations; i++ {
		if err := b.openLeg(); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.openLeg(); err == nil || b.RouteLegEvaluations != MaxRouteLegEvaluations {
		t.Fatalf("legs %d err %v", b.RouteLegEvaluations, err)
	}
	b = NewBudget(nil)
	b.RouteLegEvaluations = MaxRoutingProviderCalls
	for i := 0; i < MaxRoutingProviderCalls; i++ {
		if err := b.openProvider(); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.openProvider(); err == nil || b.RoutingProviderCalls != MaxRoutingProviderCalls {
		t.Fatalf("calls %d err %v", b.RoutingProviderCalls, err)
	}
	b = NewBudget(nil)
	for i := 0; i < MaxGroupageEvaluations; i++ {
		if err := b.openGroupage(); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.openGroupage(); err == nil || b.GroupageEvaluations != MaxGroupageEvaluations {
		t.Fatalf("groupage %d err %v", b.GroupageEvaluations, err)
	}
	now := time.Unix(0, 0)
	b = NewBudget(func() time.Time { return now })
	now = now.Add(TimeBudget + time.Second)
	if err := b.before(BudgetTime); err == nil || b.BlockedReason != BudgetTime {
		t.Fatal(err)
	}
}

func TestBoundedInsertion(t *testing.T) {
	start := anchor(55, 37)
	endID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	end := Stop{Role: RoleEnd, Point: canonical(endID, 48, 2)}
	loadA := loadAt("00000000-0000-0000-0000-0000000000a1", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb1", endID.String(), 10)
	loadB := loadAt("00000000-0000-0000-0000-0000000000a2", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb2", endID.String(), 10)
	routes := &scriptRoute{}
	in := Input{
		Mode: ModeCurrentTrip, Start: Stop{Role: RoleStart, Point: start}, End: &end,
		Loads: []Load{loadB, loadA}, Initial: knownCapacity(1000),
		Clock: time.Date(2026, 9, 27, 8, 0, 0, 0, time.UTC), Route: routes,
		Groupage: compatibleGroupage, ServiceDurationSeconds: intPtr(0), VehicleProfile: completeProfile(),
	}
	first, budget, err := Planner{}.Plan(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if first.Stops[0].Role != RoleStart || first.Stops[len(first.Stops)-1].Role != RoleEnd {
		t.Fatalf("anchors moved %+v", roles(first.Stops))
	}
	if first.PickupOrdinal <= 1 || first.DeliveryOrdinal < first.PickupOrdinal {
		t.Fatalf("ordinals pickup %d delivery %d", first.PickupOrdinal, first.DeliveryOrdinal)
	}
	second, _, err := Planner{}.Plan(context.Background(), in)
	if err != nil || EvaluationFingerprint(first) != EvaluationFingerprint(second) {
		t.Fatalf("fingerprint drifted %v", err)
	}
	if budget.RoutingProviderCalls > budget.RouteLegEvaluations {
		t.Fatalf("calls %d legs %d", budget.RoutingProviderCalls, budget.RouteLegEvaluations)
	}
	if ExistingFutureRouteLoadSource != "NONE" {
		t.Fatal(ExistingFutureRouteLoadSource)
	}
}

func TestSharedStopsAndAnchorIsolation(t *testing.T) {
	pickup := uuid.MustParse("cccccccc-cccc-cccc-cccc-ccccccccccc1")
	delivery := uuid.MustParse("cccccccc-cccc-cccc-cccc-ccccccccccc2")
	start := anchor(1, 1)
	end := Stop{Role: RoleEnd, Point: canonical(delivery, 3, 3)}
	a := loadAt("00000000-0000-0000-0000-0000000000b1", pickup.String(), delivery.String(), 1)
	b := loadAt("00000000-0000-0000-0000-0000000000b2", pickup.String(), delivery.String(), 1)
	out, _, err := Planner{}.Plan(context.Background(), Input{
		Mode: ModeDepotStart, Start: Stop{Role: RoleStart, Point: start}, End: &end,
		Loads: []Load{a, b}, Initial: knownCapacity(100), Clock: time.Unix(0, 0),
		Route: &scriptRoute{}, Groupage: compatibleGroupage, ServiceDurationSeconds: intPtr(0), VehicleProfile: completeProfile(),
	})
	if err != nil {
		t.Fatal(err)
	}
	cargoStops := 0
	for _, stop := range out.Stops {
		if stop.Role == RoleStart {
			for _, action := range stop.Actions {
				t.Fatalf("start absorbed %s", action.Type)
			}
			if stop.Point.Kind != PointAnchor || stop.Point.LocationID != nil {
				t.Fatal("anchor mutated")
			}
		}
		if stop.Role == RoleCargo && sameLocation(stop.Point, a.Pickup) {
			cargoStops++
			if len(stop.Actions) != 2 {
				t.Fatalf("shared pickup actions %d", len(stop.Actions))
			}
		}
	}
	if cargoStops != 1 {
		t.Fatalf("cargo stops %d", cargoStops)
	}
	deliveries := 0
	for _, stop := range out.Stops {
		if stop.Role == RoleEnd {
			for _, action := range stop.Actions {
				if action.Type == ActionDelivery {
					deliveries++
				}
			}
		}
	}
	if deliveries != 2 {
		t.Fatalf("shared deliveries %d", deliveries)
	}
}

func TestFeasibleBeatsIndeterminateAndOrdinals(t *testing.T) {
	far := uuid.MustParse("dddddddd-dddd-dddd-dddd-ddddddddddd1")
	near := uuid.MustParse("dddddddd-dddd-dddd-dddd-ddddddddddd2")
	destA := uuid.MustParse("dddddddd-dddd-dddd-dddd-ddddddddddd3")
	destB := uuid.MustParse("dddddddd-dddd-dddd-dddd-ddddddddddd4")
	short := loadAt("00000000-0000-0000-0000-0000000000c1", near.String(), destA.String(), 1)
	long := loadAt("00000000-0000-0000-0000-0000000000c2", far.String(), destB.String(), 1)
	routes := &scriptRoute{}
	groupage := func(items []compat.GroupageItem) compat.Result {
		if len(items) > 1 {
			return compat.Result{Status: compat.StatusIndeterminate, IndeterminateReasons: []compat.Reason{{ReasonCode: "OVERLAP_UNKNOWN"}}}
		}
		return compat.Result{Status: compat.StatusCompatible}
	}
	out, _, err := Planner{}.Plan(context.Background(), Input{
		Mode: ModeDepotStart, Start: Stop{Role: RoleStart, Point: anchor(0, 0)},
		Loads: []Load{short, long}, Initial: knownCapacity(100), Clock: time.Unix(100, 0),
		Route: routes, Groupage: groupage, ServiceDurationSeconds: intPtr(0), VehicleProfile: completeProfile(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.ResultStatus != ResultFeasible {
		t.Fatalf("status %s reasons %v", out.ResultStatus, out.ReasonCodes)
	}
	if out.ExecutionSupported {
		t.Fatal("execution supported")
	}
}

func TestUnknownDurationIndeterminate(t *testing.T) {
	dest := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeee1")
	load := loadAt("00000000-0000-0000-0000-0000000000d1", "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeee2", dest.String(), 5)
	out, _, err := Planner{}.Plan(context.Background(), Input{
		Mode: ModeCurrentTrip, Start: Stop{Role: RoleStart, Point: anchor(10, 10)},
		End:   &Stop{Role: RoleEnd, Point: canonical(dest, 11, 11)},
		Loads: []Load{load}, Initial: knownCapacity(50), Clock: time.Unix(0, 0),
		Route: &scriptRoute{}, Groupage: compatibleGroupage,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.ResultStatus != ResultIndeterminate || !contains(out.ReasonCodes, ReasonServiceDurationUnknown) {
		t.Fatalf("%s %v", out.ResultStatus, out.ReasonCodes)
	}
	for _, leg := range out.Legs {
		if leg.DurationSeconds < 0 || leg.DistanceM < 0 {
			t.Fatal("negative road metric")
		}
	}
	if len(out.Legs) > 1 && out.Legs[1].TrafficMode != routing.TrafficCurrent {
		t.Fatalf("assumed duration traffic %s", out.Legs[1].TrafficMode)
	}
}

func TestLateWindowAndEmptyIntersection(t *testing.T) {
	dest := uuid.MustParse("ffffffff-ffff-ffff-ffff-fffffffffff1")
	past := time.Unix(1, 0)
	load := loadAt("00000000-0000-0000-0000-0000000000e1", "ffffffff-ffff-ffff-ffff-fffffffffff2", dest.String(), 5)
	load.DeliveryAction.WindowEnd = &past
	_, _, err := Planner{}.Plan(context.Background(), Input{
		Mode: ModeCurrentTrip, Start: Stop{Role: RoleStart, Point: anchor(0, 0)},
		End:   &Stop{Role: RoleEnd, Point: canonical(dest, 1, 1), Actions: []Action{load.DeliveryAction}},
		Loads: []Load{}, Initial: knownCapacity(10), Clock: time.Unix(1000, 0),
		Route: &scriptRoute{}, Groupage: compatibleGroupage, ServiceDurationSeconds: intPtr(60),
		BaseSubjects: 1,
	})
	if err == nil {
		t.Fatal("late window persisted")
	}
	left := time.Unix(10, 0)
	right := time.Unix(20, 0)
	early := time.Unix(1, 0)
	stop := Stop{Actions: []Action{{WindowStart: &right, WindowEnd: &left}, {WindowStart: &early, WindowEnd: &right}}}
	if _, _, ok := stopWindow(stop); ok {
		t.Fatal("empty intersection accepted")
	}
}

func TestCacheHitDoesNotRecallProvider(t *testing.T) {
	dest := uuid.MustParse("12121212-1212-1212-1212-121212121212")
	load := loadAt("00000000-0000-0000-0000-0000000000f1", "13131313-1313-1313-1313-131313131313", dest.String(), 4)
	routes := &scriptRoute{}
	in := Input{
		Mode: ModeDepotStart, Start: Stop{Role: RoleStart, Point: anchor(5, 5)},
		Loads: []Load{load}, Initial: knownCapacity(20), Clock: time.Unix(0, 0),
		Route: routes, Groupage: compatibleGroupage, ServiceDurationSeconds: intPtr(0), VehicleProfile: completeProfile(),
	}
	if _, _, err := (Planner{}).Plan(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	calls := routes.calls
	if _, budget, err := (Planner{}).Plan(context.Background(), in); err != nil || budget.RoutingProviderCalls != calls {
		t.Fatalf("cross request cache calls %d then %d err %v", calls, budget.RoutingProviderCalls, err)
	}
	routes.calls = 0
	out, budget, err := Planner{}.Plan(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if budget.RouteLegEvaluations < budget.RoutingProviderCalls || routes.calls != budget.RoutingProviderCalls {
		t.Fatalf("legs %d calls %d provider %d", budget.RouteLegEvaluations, budget.RoutingProviderCalls, routes.calls)
	}
	if len(out.Legs) == 0 || out.Legs[0].Provider == "" {
		t.Fatal("missing provider")
	}
}

func TestLoadLimitAndNoPermutation(t *testing.T) {
	loads := []Load{
		loadAt("00000000-0000-0000-0000-000000000011", "14141414-1414-1414-1414-141414141411", "15151515-1515-1515-1515-151515151511", 1),
		loadAt("00000000-0000-0000-0000-000000000012", "14141414-1414-1414-1414-141414141412", "15151515-1515-1515-1515-151515151512", 1),
		loadAt("00000000-0000-0000-0000-000000000013", "14141414-1414-1414-1414-141414141413", "15151515-1515-1515-1515-151515151513", 1),
	}
	_, _, err := Planner{}.Plan(context.Background(), Input{Loads: loads, BaseSubjects: 2, Start: Stop{Role: RoleStart, Point: anchor(0, 0)}})
	if err == nil || err.(*SearchError).Budget != BudgetLoads {
		t.Fatal(err)
	}
}

type scriptRoute struct {
	calls    int
	duration map[float64]int
}

func (s *scriptRoute) Route(_ context.Context, req routing.RouteRequest) (routing.RouteResult, error) {
	s.calls++
	duration := 100
	if s.duration != nil {
		if value, ok := s.duration[req.Destination.Latitude]; ok {
			duration = value
		}
	}
	return routing.RouteResult{
		Provider: "script", DistanceM: duration * 10, DurationSeconds: duration,
		CalculatedAt: time.Unix(1, 0), ExpiresAt: time.Unix(2, 0),
		TrafficMode: req.TrafficMode, RouteMode: req.RouteMode,
	}, nil
}

func (s *scriptRoute) Matrix(context.Context, routing.MatrixRequest) (routing.MatrixResult, error) {
	return routing.MatrixResult{}, routing.ErrInvalidResponse
}

func compatibleGroupage([]compat.GroupageItem) compat.Result {
	return compat.Result{Status: compat.StatusCompatible}
}

func anchor(lat, lon float64) Point {
	observed := time.Unix(50, 0).UTC()
	return Point{Kind: PointAnchor, Latitude: lat, Longitude: lon, Source: SourceTracking, ObservedAt: &observed}
}

func canonical(id uuid.UUID, lat, lon float64) Point {
	return Point{Kind: PointCanonical, LocationID: &id, Latitude: lat, Longitude: lon, Source: SourceCanonical}
}

func loadAt(id, pickup, delivery string, weight float64) Load {
	pickupID := uuid.MustParse(pickup)
	deliveryID := uuid.MustParse(delivery)
	w := weight
	return Load{
		ID: uuid.MustParse(id), Version: 1,
		Pickup: canonical(pickupID, 20, 20), Delivery: canonical(deliveryID, 30, 30),
		PickupAction:   Action{Type: ActionPickup, SubjectType: SubjectLoad, SubjectID: uuid.MustParse(id), SubjectVersion: 1, WeightKg: &w, Cargo: compat.Cargo{ID: id}},
		DeliveryAction: Action{Type: ActionDelivery, SubjectType: SubjectLoad, SubjectID: uuid.MustParse(id), SubjectVersion: 1, WeightKg: &w, Cargo: compat.Cargo{ID: id}},
	}
}

func knownCapacity(value float64) Capacity {
	v := value
	return Capacity{
		Payload: Dimension{Status: DimKnown, Value: &v}, Volume: Dimension{Status: DimKnown, Value: &v},
		Pallets: Dimension{Status: DimKnown, Value: &v}, Linear: Dimension{Status: DimKnown, Value: &v},
		Height: Dimension{Status: DimKnown, Value: &v}, Temperature: "UNALLOCATED",
	}
}

func intPtr(v int) *int { return &v }

func completeProfile() routing.VehicleProfile {
	gross, height, width, length, axle := 18000.0, 4.0, 2.55, 16.5, 8000.0
	danger := false
	return routing.VehicleProfile{
		GrossWeightKg: &gross, HeightM: &height, WidthM: &width, LengthM: &length, AxleLoadKg: &axle, DangerousCargo: &danger,
	}
}

func roles(stops []Stop) []string {
	out := make([]string, len(stops))
	for i, stop := range stops {
		out[i] = stop.Role
	}
	return out
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestServiceDurationPolicyVersionChangesFingerprint(t *testing.T) {
	id := uuid.New()
	version := 1
	next := 2
	seconds := 90
	base := Outcome{ResultStatus: ResultFeasible, ServiceDurationKnown: true, ServiceDurationPolicyID: &id, ServiceDurationPolicyVersion: &version}
	changed := base
	changed.ServiceDurationPolicyVersion = &next
	if EvaluationFingerprint(base) == EvaluationFingerprint(changed) {
		t.Fatal("policy version did not change the evaluation fingerprint")
	}
	legacy := Outcome{ResultStatus: ResultFeasible, ServiceDurationKnown: true, ServiceDurationSeconds: &seconds}
	if EvaluationFingerprint(base) == EvaluationFingerprint(legacy) {
		t.Fatal("policy identity did not change the evaluation fingerprint")
	}
	stopSeconds := 900
	withStop := base
	withStop.Stops = []Stop{{Service: &stopSeconds}}
	if EvaluationFingerprint(base) == EvaluationFingerprint(withStop) {
		t.Fatal("resolved stop duration did not change the evaluation fingerprint")
	}
}

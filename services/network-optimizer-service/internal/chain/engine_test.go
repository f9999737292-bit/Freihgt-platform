package chain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/routeplan"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

func TestCHAIN01DeterministicOrderAndServerSearchID(t *testing.T) {
	clock := fixedClock()
	eng := engine(t)
	eng.Repository = &MemoryRepository{}
	leftReq := request(clock, func() time.Time { return clock }, twoLoads(t, clock), &latProvider{}, "key-a")
	rightReq := request(clock, func() time.Time { return clock }, twoLoads(t, clock), &latProvider{}, "key-b")
	left, err := eng.Search(context.Background(), leftReq)
	if err != nil {
		t.Fatal(err)
	}
	right, err := eng.Search(context.Background(), rightReq)
	if err != nil {
		t.Fatal(err)
	}
	if left.SearchID == uuid.Nil || right.SearchID == uuid.Nil || left.SearchID == right.SearchID {
		t.Fatalf("search ids %s %s", left.SearchID, right.SearchID)
	}
	if left.GlobalOptimum || right.GlobalOptimum {
		t.Fatal("global optimum claimed")
	}
	if len(left.Candidates) == 0 || len(right.Candidates) == 0 {
		t.Fatal("missing candidate")
	}
	if left.Candidates[0].SelectedOrder != right.Candidates[0].SelectedOrder {
		t.Fatalf("order drifted %s %s", left.Candidates[0].SelectedOrder, right.Candidates[0].SelectedOrder)
	}
}

func TestCHAIN02Caps(t *testing.T) {
	clock := fixedClock()
	loads := make([]VisibleLoad, 10)
	for i := range loads {
		loads[i] = sampleLoad(i+1, float64(i+1), clock)
	}
	wide := &latProvider{}
	result := run(t, clock, loads, wide, "")
	if len(result.FirstLoadIDs) != FirstLoadCandidatesMax {
		t.Fatalf("first loads %d", len(result.FirstLoadIDs))
	}
	for _, id := range result.FirstLoadIDs {
		n := loadNumber(t, id)
		if n > FirstLoadCandidatesMax {
			t.Fatalf("kept farther load %d", n)
		}
	}
	if result.MaxSecondsPerFirst > SecondLoadCandidatesPerFirstMax {
		t.Fatalf("seconds %d", result.MaxSecondsPerFirst)
	}
	if result.PairsRetained > PairCandidatesMax {
		t.Fatalf("pairs %d", result.PairsRetained)
	}
	if result.Evaluations > RoutePlanEvaluationsMax {
		t.Fatalf("evaluations %d", result.Evaluations)
	}
	if result.PlannerInvocations > RoutePlanEvaluationsMax*OrdersPerPair {
		t.Fatalf("invocations %d", result.PlannerInvocations)
	}
	if result.ProviderCalls > ProviderCallsMax || wide.calls > ProviderCallsMax {
		t.Fatalf("calls result %d provider %d", result.ProviderCalls, wide.calls)
	}
	if result.MaxLoadsPerPlan > TotalLoads {
		t.Fatalf("loads per plan %d", result.MaxLoadsPerPlan)
	}
	for _, order := range result.OrdersEvaluated {
		if order != OrderLowerFirst && order != OrderHigherFirst {
			t.Fatalf("unexpected order %s", order)
		}
	}
}

func TestCHAIN02SecondAndPairCaps(t *testing.T) {
	clock := fixedClock()
	var loads []VisibleLoad
	for i := 1; i <= 4; i++ {
		loads = append(loads, sampleLoad(i, float64(i), clock))
	}
	result := run(t, clock, loads, &latProvider{}, "")
	if result.MaxSecondsPerFirst != SecondLoadCandidatesPerFirstMax {
		t.Fatalf("seconds per first %d", result.MaxSecondsPerFirst)
	}
	if result.PairsRetained != PairCandidatesMax {
		t.Fatalf("pairs %d", result.PairsRetained)
	}
	if result.Evaluations > RoutePlanEvaluationsMax {
		t.Fatalf("evaluations %d", result.Evaluations)
	}
	three := run(t, clock, loads[:3], &latProvider{}, "")
	if three.PairsRetained < RoutePlanEvaluationsMax {
		t.Fatalf("pairs available %d", three.PairsRetained)
	}
	if three.Evaluations != RoutePlanEvaluationsMax {
		t.Fatalf("evaluations %d status %s calls %d", three.Evaluations, three.Status, three.ProviderCalls)
	}
}

func TestCHAIN03ProviderCapAndWallClock(t *testing.T) {
	clock := fixedClock()
	loads := make([]VisibleLoad, 30)
	for i := range loads {
		loads[i] = sampleLoad(i+1, float64(i+1), clock)
	}
	capped := &latProvider{}
	result := run(t, clock, loads, capped, "")
	if capped.calls != ProviderCallsMax || result.ProviderCalls != ProviderCallsMax {
		t.Fatalf("calls provider %d result %d", capped.calls, result.ProviderCalls)
	}
	assertExhausted(t, result)
	for _, candidate := range result.Candidates {
		if candidate.Feasible {
			t.Fatal("unfinished candidate feasible")
		}
	}

	current := clock
	var calls int
	slow := &latProvider{advance: func() {
		calls++
		if calls >= 2 {
			current = clock.Add(WallClockBudget + time.Second)
		}
	}}
	timed := runAt(t, func() time.Time { return current }, loads, slow, "")
	if slow.calls != 2 {
		t.Fatalf("wall clock allowed %d calls", slow.calls)
	}
	assertExhausted(t, timed)
	if timed.ProviderCalls >= ProviderCallsMax {
		t.Fatal("wall clock waited for the call cap")
	}
}

func TestCHAIN03NoCallAfterDeadlineAndInheritedTimeout(t *testing.T) {
	clock := fixedClock()
	reads := 0
	now := func() time.Time {
		reads++
		if reads == 1 {
			return clock
		}
		return clock.Add(WallClockBudget)
	}
	blocked := &latProvider{}
	result := runAt(t, now, []VisibleLoad{sampleLoad(1, 1, clock)}, blocked, "")
	if blocked.calls != 0 || result.ProviderCalls != 0 {
		t.Fatalf("call after deadline provider %d result %d", blocked.calls, result.ProviderCalls)
	}
	assertExhausted(t, result)

	reads = 0
	inheritedNow := func() time.Time {
		reads++
		if reads == 1 {
			return clock
		}
		return clock.Add(18 * time.Second)
	}
	inherited := &latProvider{}
	timed := runAt(t, inheritedNow, []VisibleLoad{sampleLoad(1, 1, clock), sampleLoad(2, 2, clock)}, inherited, "")
	if len(timed.CallTimeouts) == 0 {
		t.Fatal("no provider timeout recorded")
	}
	if timed.CallTimeouts[0] != 2*time.Second {
		t.Fatalf("timeout %s", timed.CallTimeouts[0])
	}
	for _, timeout := range timed.CallTimeouts {
		if timeout > ProviderCallTimeout {
			t.Fatalf("timeout %s exceeds 5s", timeout)
		}
	}
}

func TestCHAIN03CancellationStopsWorkWithoutRetry(t *testing.T) {
	clock := fixedClock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	blocked := &latProvider{}
	result, err := engine(t).Search(ctx, request(clock, func() time.Time { return clock }, twoLoads(t, clock), blocked, ""))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusCancelled || result.SearchComplete || result.NoRetry != true || blocked.calls != 0 {
		t.Fatalf("status %s calls %d", result.Status, blocked.calls)
	}

	ctx, cancel = context.WithCancel(context.Background())
	once := &latProvider{cancel: cancel}
	stopped, err := engine(t).Search(ctx, request(clock, func() time.Time { return clock }, twoLoads(t, clock), once, ""))
	if err != nil {
		t.Fatal(err)
	}
	if stopped.Status != StatusCancelled || once.calls != 1 || stopped.NoRetry != true {
		t.Fatalf("status %s calls %d retry %v", stopped.Status, once.calls, stopped.NoRetry)
	}
}

func TestCHAIN04Fixed48HourHorizon(t *testing.T) {
	clock := fixedClock()
	inside := sampleLoad(1, 1, clock)
	outside := sampleLoad(2, 2, clock)
	outside.PickupStart = clock.Add(Horizon)
	outside.PickupEnd = clock.Add(Horizon + time.Hour)
	outside.DeliveryStart = clock.Add(Horizon + 2*time.Hour)
	outside.DeliveryEnd = clock.Add(Horizon + 3*time.Hour)
	result := run(t, clock, []VisibleLoad{inside, outside}, &latProvider{}, "")
	if !containsID(result.FirstLoadIDs, inside.ID) || containsID(result.FirstLoadIDs, outside.ID) {
		t.Fatalf("horizon ids %v", result.FirstLoadIDs)
	}
	if result.HorizonEnd.Sub(result.HorizonStart) != Horizon {
		t.Fatalf("horizon %s", result.HorizonEnd.Sub(result.HorizonStart))
	}

	from := clock.Add(10 * time.Hour)
	manual := sampleLoad(3, 1, clock)
	manual.PickupStart = clock.Add(12 * time.Hour)
	manual.PickupEnd = clock.Add(14 * time.Hour)
	manual.DeliveryStart = clock.Add(15 * time.Hour)
	manual.DeliveryEnd = clock.Add(20 * time.Hour)
	early := sampleLoad(4, 2, clock)
	early.PickupStart = clock
	early.PickupEnd = from.Add(-time.Minute)
	req := request(clock, func() time.Time { return clock }, []VisibleLoad{manual, early}, &latProvider{}, "")
	req.Seed.Source = SeedManualAvailable
	req.Seed.AvailableFrom = &from
	manualResult, err := engine(t).Search(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !containsID(manualResult.FirstLoadIDs, manual.ID) || containsID(manualResult.FirstLoadIDs, early.ID) {
		t.Fatalf("manual horizon %v", manualResult.FirstLoadIDs)
	}

	late := sampleLoad(5, 3, clock)
	late.PickupStart = from.Add(60 * time.Hour)
	late.PickupEnd = from.Add(61 * time.Hour)
	late.DeliveryStart = from.Add(62 * time.Hour)
	late.DeliveryEnd = from.Add(63 * time.Hour)
	until := from.Add(72 * time.Hour)
	req.Loads = []VisibleLoad{late}
	req.Seed.AvailableUntil = &until
	req.Route = &latProvider{}
	stretched, err := engine(t).Search(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if containsID(stretched.FirstLoadIDs, late.ID) || stretched.HorizonEnd.After(from.Add(Horizon)) {
		t.Fatalf("horizon expanded to %s", stretched.HorizonEnd)
	}

	tripLoad := sampleLoad(6, 1, clock)
	tripLate := sampleLoad(7, 2, clock)
	tripLate.PickupStart = clock.Add(50 * time.Hour)
	tripLate.PickupEnd = clock.Add(51 * time.Hour)
	tripLate.DeliveryStart = clock.Add(52 * time.Hour)
	tripLate.DeliveryEnd = clock.Add(53 * time.Hour)
	predictedStart := clock.Add(40 * time.Hour)
	trip := request(clock, func() time.Time { return clock }, []VisibleLoad{tripLoad, tripLate}, &latProvider{}, "")
	trip.Seed.Source = SeedCurrentTripResidual
	trip.Seed.AvailableFrom = &predictedStart
	tripResult, err := engine(t).Search(context.Background(), trip)
	if err != nil {
		t.Fatal(err)
	}
	if !containsID(tripResult.FirstLoadIDs, tripLoad.ID) || containsID(tripResult.FirstLoadIDs, tripLate.ID) {
		t.Fatalf("current trip used predicted start %v horizon %s", tripResult.FirstLoadIDs, tripResult.HorizonStart)
	}
	if !tripResult.HorizonStart.Equal(clock) {
		t.Fatal(tripResult.HorizonStart)
	}
}

func TestCHAIN05TenantScopedMemoryLookup(t *testing.T) {
	clock := fixedClock()
	repo := &MemoryRepository{}
	eng := engine(t)
	eng.Repository = repo
	req := request(clock, func() time.Time { return clock }, twoLoads(t, clock), &latProvider{}, "same")
	if _, err := eng.Search(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.Lookup(req.TenantID, "same"); !ok {
		t.Fatal("owner lookup missed")
	}
	if _, ok := repo.Lookup(uuid.MustParse("99999999-9999-9999-9999-999999999999"), "same"); ok {
		t.Fatal("foreign tenant read its chain search")
	}
}

func TestCHAIN06And07VisibilityGates(t *testing.T) {
	clock := fixedClock()
	foreign := sampleLoad(1, 1, clock)
	foreign.OwnerTenantID = uuid.MustParse("88888888-8888-8888-8888-888888888888")
	foreign.CrossShipperConsolidationAllowed = false
	foreign.Private.ShipperName = "SECRET_SHIPPER"
	result := run(t, clock, []VisibleLoad{foreign, sampleLoad(2, 2, clock)}, &latProvider{}, "")
	if containsID(result.FirstLoadIDs, foreign.ID) {
		t.Fatal("foreign load entered a pair")
	}
	if result.ReasonCounts[ReasonConsolidation] != 1 {
		t.Fatalf("counts %v", result.ReasonCounts)
	}

	unknown := sampleLoad(3, 1, clock)
	unknown.ReferenceUnavailable = true
	unknown.Private.CustomerName = "SECRET_CUSTOMER"
	blocked := run(t, clock, []VisibleLoad{unknown}, &latProvider{}, "")
	if len(blocked.Candidates) != 0 || blocked.ReasonCounts[ReasonReferenceContext] != 1 {
		t.Fatalf("reference result %+v", blocked.ReasonCounts)
	}
	for _, candidate := range blocked.Candidates {
		if candidate.Feasible {
			t.Fatal("unknown reference context was feasible")
		}
	}
}

func TestCHAIN08Stacking(t *testing.T) {
	if reject, _ := stackingDecision(false, nil, nil); reject {
		t.Fatal("non-overlap rejected")
	}
	clock := fixedClock()
	left := sharedLoad(1, clock)
	right := sharedLoad(2, clock)
	unknown := run(t, clock, []VisibleLoad{left, right}, &latProvider{}, "")
	if feasible(unknown) || !hasReason(unknown, ReasonStackingUnknown) {
		t.Fatalf("unknown stacking status %s feasible %v candidates %+v counts %v", unknown.Status, feasible(unknown), unknown.Candidates, unknown.ReasonCounts)
	}
	no := false
	left.Stackable = &no
	right.Stackable = &no
	rejected := run(t, clock, []VisibleLoad{left, right}, &latProvider{}, "")
	if feasible(rejected) || !hasReason(rejected, ReasonStackingRejected) {
		t.Fatal("false stackable accepted")
	}
	yes := true
	left.Stackable = &yes
	right.Stackable = &yes
	left.WeightKg = 60
	right.WeightKg = 60
	tightReq := request(clock, func() time.Time { return clock }, []VisibleLoad{left, right}, &latProvider{}, "")
	tightReq.Seed.Capacity = known(100)
	tight, err := engine(t).Search(context.Background(), tightReq)
	if err != nil {
		t.Fatal(err)
	}
	if feasible(tight) {
		t.Fatal("stackable created extra pallet space")
	}
	left.WeightKg = 1
	right.WeightKg = 1
	allowed := run(t, clock, []VisibleLoad{left, right}, &latProvider{}, "")
	if !feasible(allowed) {
		t.Fatalf("known stackable overlap rejected status %s", allowed.Status)
	}
}

func TestCHAIN09And10CapacityIsNotZero(t *testing.T) {
	clock := fixedClock()
	loads := twoLoads(t, clock)
	req := request(clock, func() time.Time { return clock }, loads, &latProvider{}, "")
	req.Seed.Capacity.Payload.Known = false
	result, err := engine(t).Search(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if feasible(result) {
		t.Fatal("unknown capacity became a feasible chain")
	}
}

func TestCHAIN11GroupageHardReject(t *testing.T) {
	clock := fixedClock()
	req := request(clock, func() time.Time { return clock }, sharedLoads(clock), &latProvider{}, "")
	req.Groupage = func([]compat.GroupageItem) compat.Result {
		return compat.Result{Status: compat.StatusIncompatible, HardRejects: []compat.Reason{{ReasonCode: "GROUPAGE_HARD"}}}
	}
	yes := true
	req.Loads[0].Stackable = &yes
	req.Loads[1].Stackable = &yes
	result, err := engine(t).Search(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if feasible(result) {
		t.Fatal("hard groupage incompatibility stayed feasible")
	}
}

func TestCHAIN12WindowAndCHAIN13ServiceDuration(t *testing.T) {
	clock := fixedClock()
	loads := twoLoads(t, clock)
	loads[0].PickupEnd = clock.Add(time.Second)
	loads[1].PickupEnd = clock.Add(time.Second)
	late := &latProvider{duration: 5000}
	result := run(t, clock, loads, late, "")
	if feasible(result) {
		t.Fatal("late arrival stayed feasible")
	}

	unknownService := request(clock, func() time.Time { return clock }, twoLoads(t, clock), &latProvider{}, "")
	unknownService.ServiceDurationSeconds = nil
	serviceResult, err := engine(t).Search(context.Background(), unknownService)
	if err != nil {
		t.Fatal(err)
	}
	if feasible(serviceResult) || serviceResult.ExecutionSupported {
		t.Fatal("unknown service duration became executable")
	}
}

func TestCHAIN14UnknownRoadIsNotZero(t *testing.T) {
	clock := fixedClock()
	near := sampleLoad(1, 1, clock)
	far := sampleLoad(2, 9, clock)
	routes := &latProvider{fail: true, failLat: 1}
	result := run(t, clock, []VisibleLoad{near, far}, routes, "")
	if containsID(result.FirstLoadIDs, near.ID) {
		t.Fatal("failed road entered as zero distance")
	}
	if !containsID(result.FirstLoadIDs, far.ID) {
		t.Fatal("measured farther load missing")
	}
	if routes.failCalls != 1 {
		t.Fatalf("retries %d", routes.failCalls)
	}
}

func TestCHAIN15OneCandidateTwoOrders(t *testing.T) {
	clock := fixedClock()
	result := run(t, clock, twoLoads(t, clock), &latProvider{}, "")
	if result.PairsRetained != 1 || len(result.Candidates) != 1 {
		t.Fatalf("pairs %d candidates %d", result.PairsRetained, len(result.Candidates))
	}
	candidate := result.Candidates[0]
	if candidate.RoutePlanID != nil {
		t.Fatal("route plan id manufactured before persistence")
	}
	if len(candidate.OrdersEvaluated) != OrdersPerPair {
		t.Fatalf("orders %v", candidate.OrdersEvaluated)
	}
	if candidate.OrdersEvaluated[0] != OrderLowerFirst || candidate.OrdersEvaluated[1] != OrderHigherFirst {
		t.Fatalf("order sequence %v", candidate.OrdersEvaluated)
	}
	if len(candidate.Loads) != TotalLoads {
		t.Fatalf("loads %d", len(candidate.Loads))
	}
}

func TestCHAIN16And22VersionAndIdempotency(t *testing.T) {
	clock := fixedClock()
	repo := &MemoryRepository{}
	eng := engine(t)
	eng.Repository = repo
	loads := twoLoads(t, clock)
	routes := &latProvider{}
	req := request(clock, func() time.Time { return clock }, loads, routes, "replay")
	first, err := eng.Search(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	routes.calls = 0
	second, err := eng.Search(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if second.SearchID != first.SearchID || routes.calls != 0 {
		t.Fatalf("replay id %s calls %d", second.SearchID, routes.calls)
	}
	changed := req
	changed.Loads = append([]VisibleLoad(nil), req.Loads...)
	changed.Loads[0].Version = 2
	changed.Route = &latProvider{}
	if _, err := eng.Search(context.Background(), changed); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatal(err)
	}
	fresh := changed
	fresh.IdempotencyKey = "other"
	third, err := eng.Search(context.Background(), fresh)
	if err != nil {
		t.Fatal(err)
	}
	if third.SearchID == first.SearchID {
		t.Fatal("version change reused the search id")
	}
	if len(third.Candidates) == 1 && len(first.Candidates) == 1 && third.Candidates[0].CandidateID == first.Candidates[0].CandidateID {
		t.Fatal("version change reused the candidate id")
	}
}

func TestCHAIN17ServiceDurationChangesRequestHash(t *testing.T) {
	clock := fixedClock()
	base := request(clock, func() time.Time { return clock }, twoLoads(t, clock), &latProvider{}, "")
	changed := base
	seconds := 90
	changed.ServiceDurationSeconds = &seconds
	if requestHash(base) == requestHash(changed) {
		t.Fatal("service duration policy did not change the request hash")
	}
	if !strings.Contains(PolicyVersion, "BOUNDED_TWO_LOAD") {
		t.Fatal(PolicyVersion)
	}
}

func TestCHAIN19LexicographicObjective(t *testing.T) {
	lowEmpty := Score{EmptyRoadMeters: 10, DurationSeconds: 90, MinLoadID: "b", MaxLoadID: "d", Order: OrderHigherFirst}
	highEmpty := Score{EmptyRoadMeters: 20, DurationSeconds: 5, MinLoadID: "a", MaxLoadID: "c", Order: OrderLowerFirst}
	if !Better(lowEmpty, highEmpty) {
		t.Fatal("longer empty road won")
	}
	shorter := lowEmpty
	shorter.DurationSeconds = 40
	longer := lowEmpty
	longer.DurationSeconds = 80
	if !Better(shorter, longer) {
		t.Fatal("duration tie-break ignored")
	}
	priced := int64(500)
	missing := lowEmpty
	present := lowEmpty
	present.Revenue = &priced
	present.MinLoadID = "z"
	ranked := OmitShared([]Score{missing, present})
	if ranked[0].Revenue != nil || ranked[1].Revenue != nil {
		t.Fatal("missing revenue compared as a number")
	}
	if Better(ranked[1], ranked[0]) {
		t.Fatal("priced load won because the other revenue was missing")
	}
	leftRevenue := int64(10)
	rightRevenue := int64(40)
	left := lowEmpty
	right := lowEmpty
	left.Revenue = &leftRevenue
	right.Revenue = &rightRevenue
	right.MinLoadID = "z"
	if !Better(right, left) {
		t.Fatal("known revenue did not rank descending")
	}
	lowUse := 0.2
	highUse := 0.8
	less := lowEmpty
	more := lowEmpty
	less.Utilization = &lowUse
	more.Utilization = &highUse
	more.MinLoadID = "z"
	if !Better(more, less) {
		t.Fatal("known utilization did not rank descending")
	}
}

func TestCHAIN20PrivacyProjection(t *testing.T) {
	clock := fixedClock()
	loads := twoLoads(t, clock)
	loads[0].Private = PrivateFacts{
		ShipperName: "SECRET_SHIPPER", CustomerName: "SECRET_CUSTOMER", ContractRate: "SECRET_RATE",
		Revenue: int64Ptr(424242), Currency: "EUR", RevenueAuthoritative: true,
		InternalShipmentID: uuidPtr(uuid.MustParse("77777777-7777-7777-7777-777777777777")),
		ExactAddress:       "SECRET_ADDRESS",
	}
	loads[0].PublicationLabel = "VISIBLE_AREA"
	result := run(t, clock, loads, &latProvider{}, "")
	body, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, secret := range []string{"SECRET_SHIPPER", "SECRET_CUSTOMER", "SECRET_RATE", "SECRET_ADDRESS", "77777777-7777-7777-7777-777777777777", "424242"} {
		if strings.Contains(text, secret) {
			t.Fatalf("projected %s", secret)
		}
	}
	if !strings.Contains(text, "VISIBLE_AREA") {
		t.Fatal("publication label dropped")
	}
	for _, candidate := range result.Candidates {
		if candidate.RoutePlanID != nil {
			t.Fatal(candidate.RoutePlanID)
		}
	}
}

func TestCHAIN21ExecutionRemainsDisabled(t *testing.T) {
	clock := fixedClock()
	result := run(t, clock, twoLoads(t, clock), &latProvider{}, "")
	if result.ExecutionSupported || result.GlobalOptimum || !result.NoRetry {
		t.Fatalf("%+v", result)
	}
	if feasible(result) && result.Candidates[0].RoutePlanID != nil {
		t.Fatal("execution identity created")
	}
}

func TestPredictedCapacityExcludedAndSeedsAccepted(t *testing.T) {
	clock := fixedClock()
	predicted := request(clock, func() time.Time { return clock }, twoLoads(t, clock), &latProvider{}, "")
	predicted.Seed.Source = SeedPredictedCapacity
	rejected, err := engine(t).Search(context.Background(), predicted)
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Status != StatusRejected || rejected.ProviderCalls != 0 || rejected.ReasonCounts[ReasonPredictedCapacity] != 1 {
		t.Fatalf("%+v", rejected)
	}
	manual := request(clock, func() time.Time { return clock }, twoLoads(t, clock), &latProvider{}, "")
	manual.Seed.Source = SeedManualAvailable
	manualResult, err := engine(t).Search(context.Background(), manual)
	if err != nil || manualResult.Status == StatusRejected {
		t.Fatal(manualResult.Status, err)
	}
	trip := request(clock, func() time.Time { return clock }, twoLoads(t, clock), &latProvider{}, "")
	trip.Seed.Source = SeedCurrentTripResidual
	tripResult, err := engine(t).Search(context.Background(), trip)
	if err != nil || tripResult.Status == StatusRejected {
		t.Fatal(tripResult.Status, err)
	}
}

func TestClientChainIDRejected(t *testing.T) {
	clock := fixedClock()
	req := request(clock, func() time.Time { return clock }, twoLoads(t, clock), &latProvider{}, "")
	id := uuid.New()
	req.ClientChainID = &id
	result, err := engine(t).Search(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusRejected || result.ProviderCalls != 0 {
		t.Fatalf("%+v", result)
	}
}

func TestBudgetConstants(t *testing.T) {
	if TotalLoads != 2 || Horizon != 48*time.Hour || FirstLoadCandidatesMax != 8 || SecondLoadCandidatesPerFirstMax != 2 {
		t.Fatal("shape constants")
	}
	if PairCandidatesMax != 4 || RoutePlanEvaluationsMax != 2 || ProviderCallsMax != 24 || WallClockBudget != 20*time.Second {
		t.Fatal("budget constants")
	}
	if ProviderCallTimeout != 5*time.Second || ProviderRetryCount != 0 || SequencesPerOrderMax != 16 || LegsPerOrderMax != 6 {
		t.Fatal("call constants")
	}
}

type latProvider struct {
	calls     int
	fail      bool
	failLat   float64
	failCalls int
	duration  int
	advance   func()
	cancel    context.CancelFunc
}

func (p *latProvider) Route(ctx context.Context, req routing.RouteRequest) (routing.RouteResult, error) {
	p.calls++
	if p.advance != nil {
		p.advance()
	}
	if p.cancel != nil {
		p.cancel()
		return routing.RouteResult{}, context.Canceled
	}
	if p.fail && req.Destination.Latitude == p.failLat {
		p.failCalls++
		return routing.RouteResult{}, routing.ErrRouteNotFound
	}
	duration := 100
	if p.duration != 0 {
		duration = p.duration
	}
	distance := int(req.Destination.Latitude * 1000)
	if distance <= 0 {
		distance = 1000
	}
	return routing.RouteResult{
		Provider: "script", DistanceM: distance, DurationSeconds: duration,
		CalculatedAt: time.Unix(1, 0).UTC(), ExpiresAt: time.Unix(1, 0).UTC().Add(time.Hour),
		TrafficMode: req.TrafficMode, RouteMode: req.RouteMode,
	}, nil
}

func (p *latProvider) Matrix(context.Context, routing.MatrixRequest) (routing.MatrixResult, error) {
	return routing.MatrixResult{}, routing.ErrInvalidResponse
}

func fixedClock() time.Time {
	return time.Date(2026, 10, 14, 8, 0, 0, 0, time.UTC)
}

func engine(t *testing.T) Engine {
	t.Helper()
	n := 0
	return Engine{NewID: func() uuid.UUID {
		n++
		return uuid.MustParse(fmt.Sprintf("30000000-0000-0000-0000-%012d", n))
	}}
}

func run(t *testing.T, clock time.Time, loads []VisibleLoad, routes routing.Provider, key string) Result {
	t.Helper()
	return runAt(t, func() time.Time { return clock }, loads, routes, key)
}

func runAt(t *testing.T, now func() time.Time, loads []VisibleLoad, routes routing.Provider, key string) Result {
	t.Helper()
	result, err := engine(t).Search(context.Background(), request(fixedClock(), now, loads, routes, key))
	if err != nil {
		t.Fatal(err)
	}
	if result.ProviderCalls > ProviderCallsMax {
		t.Fatalf("provider calls %d", result.ProviderCalls)
	}
	return result
}

func request(clock time.Time, now func() time.Time, loads []VisibleLoad, routes routing.Provider, key string) Request {
	service := 0
	return Request{
		TenantID:       uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		IdempotencyKey: key, Seed: Seed{Source: SeedManualAvailable, Start: anchor(0), Capacity: known(1000)},
		Loads: loads, VehicleProfile: profile(), ServiceDurationSeconds: &service, Now: now, Route: routes,
	}
}

func twoLoads(t *testing.T, clock time.Time) []VisibleLoad {
	t.Helper()
	return []VisibleLoad{sampleLoad(1, 2, clock), sampleLoad(2, 4, clock)}
}

func sampleLoad(n int, lat float64, clock time.Time) VisibleLoad {
	return VisibleLoad{
		ID: loadID(n), Version: 1, OwnerTenantID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ConsolidationAllowed: true,
		Pickup:               cargoPoint(1000+n, lat), Delivery: cargoPoint(2000+n, lat+20),
		PickupStart: clock.Add(time.Hour), PickupEnd: clock.Add(10 * time.Hour),
		DeliveryStart: clock.Add(2 * time.Hour), DeliveryEnd: clock.Add(30 * time.Hour),
		WeightKg: 1, PublicationLabel: "VISIBLE_AREA",
	}
}

func sharedLoad(n int, clock time.Time) VisibleLoad {
	load := sampleLoad(n, 5, clock)
	load.Pickup = cargoPoint(5000, 5)
	load.Delivery = cargoPoint(6000, 8)
	return load
}

func sharedLoads(clock time.Time) []VisibleLoad {
	return []VisibleLoad{sharedLoad(1, clock), sharedLoad(2, clock)}
}

func loadID(n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("10000000-0000-0000-0000-%012d", n))
}

func cargoPoint(n int, lat float64) routeplan.Point {
	id := uuid.MustParse(fmt.Sprintf("20000000-0000-0000-0000-%012d", n))
	return routeplan.Point{Kind: routeplan.PointCanonical, LocationID: &id, Latitude: lat, Longitude: 10, Source: routeplan.SourceCanonical}
}

func anchor(lat float64) routeplan.Point {
	observed := time.Unix(50, 0).UTC()
	return routeplan.Point{Kind: routeplan.PointAnchor, Latitude: lat, Longitude: 10, Source: routeplan.SourceTracking, ObservedAt: &observed}
}

func known(value float64) Capacity {
	dim := Dimension{Known: true, Value: value}
	return Capacity{Payload: dim, Volume: dim, Pallets: dim, Linear: dim, Height: dim}
}

func profile() routing.VehicleProfile {
	gross, height, width, length, axle := 18000.0, 4.0, 2.55, 16.5, 8000.0
	danger := false
	return routing.VehicleProfile{GrossWeightKg: &gross, HeightM: &height, WidthM: &width, LengthM: &length, AxleLoadKg: &axle, DangerousCargo: &danger}
}

func containsID(ids []uuid.UUID, want uuid.UUID) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func loadNumber(t *testing.T, id uuid.UUID) int {
	t.Helper()
	var n int
	if _, err := fmt.Sscanf(id.String()[len(id.String())-12:], "%d", &n); err != nil {
		t.Fatal(err)
	}
	return n
}

func feasible(result Result) bool {
	for _, candidate := range result.Candidates {
		if candidate.Feasible {
			return true
		}
	}
	return false
}

func hasReason(result Result, reason string) bool {
	for _, candidate := range result.Candidates {
		for _, code := range candidate.ReasonCodes {
			if code == reason {
				return true
			}
		}
	}
	return false
}

func int64Ptr(v int64) *int64 { return &v }

func uuidPtr(id uuid.UUID) *uuid.UUID { return &id }

func assertExhausted(t *testing.T, result Result) {
	t.Helper()
	if result.Status != StatusBudgetExhausted || result.SearchComplete || result.GlobalOptimum || !result.NoRetry {
		t.Fatalf("status %s complete %v optimum %v", result.Status, result.SearchComplete, result.GlobalOptimum)
	}
}

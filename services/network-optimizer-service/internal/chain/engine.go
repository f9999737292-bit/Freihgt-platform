package chain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/routeplan"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

// Engine is the bounded two-load chain search. It does not persist to PostgreSQL,
// expose an HTTP route, or hand a plan to TMS execution.
type Engine struct {
	NewID      func() uuid.UUID
	Repository Repository
}

func (e Engine) Search(ctx context.Context, req Request) (Result, error) {
	result := baseResult()
	if req.ClientChainID != nil {
		result.Status = StatusRejected
		result.ReasonCounts[ReasonClientChainID] = 1
		return result, nil
	}
	if req.Seed.Source == SeedPredictedCapacity {
		result.Status = StatusRejected
		result.ReasonCounts[ReasonPredictedCapacity] = 1
		return result, nil
	}
	if req.Seed.Source != SeedManualAvailable && req.Seed.Source != SeedCurrentTripResidual {
		result.Status = StatusRejected
		return result, nil
	}
	if req.Seed.OnboardSubjects+TotalLoads > routeplan.MaxRouteLoadSubjects {
		result.Status = StatusRejected
		result.ReasonCounts[routeplan.ResultLoadLimit] = 1
		return result, nil
	}
	now := req.Now
	if now == nil {
		now = time.Now
	}
	started := now()
	hash := requestHash(req)
	if e.Repository != nil && req.IdempotencyKey != "" {
		claim, err := e.Repository.Claim(ctx, req.TenantID, req.IdempotencyKey, hash)
		if err != nil || claim.Conflict {
			result.Status = StatusIdempotencyConflict
			return result, ErrIdempotencyConflict
		}
		if claim.Replay {
			return claim.Result, nil
		}
	}
	if err := ctx.Err(); err != nil {
		result.Status = StatusCancelled
		result.SearchID = resolveSearchID(e, req)
		return e.finish(ctx, req, hash, nil, result)
	}
	horizonStart, horizonEnd, ok := resolveHorizon(req, started)
	if !ok {
		result.Status = StatusRejected
		result.ReasonCounts[ReasonWindowNotAbsolute] = 1
		result.SearchID = resolveSearchID(e, req)
		return e.finish(ctx, req, hash, nil, result)
	}
	result.SearchID = resolveSearchID(e, req)
	result.HorizonStart = horizonStart
	result.HorizonEnd = horizonEnd
	base := req.Route
	if base == nil {
		base = unavailableProvider{}
	}
	deadline := effectiveComputeDeadline(started, req.ComputeDeadline)
	limited := &limitedProvider{
		base: base, now: now, deadline: deadline,
		durableAbsolute: req.ComputeDeadline != nil && !req.ComputeDeadline.IsZero(),
	}
	if !started.Before(deadline) {
		result.Status = StatusBudgetExhausted
		result.SearchComplete = false
		return e.finish(ctx, req, hash, limited, result)
	}
	loads := eligibleLoads(req.TenantID, req.Loads, horizonStart, horizonEnd, result.ReasonCounts)
	sort.Slice(loads, func(i, j int) bool { return loads[i].ID.String() < loads[j].ID.String() })

	var pool []measured
	cancelled := false
	exhausted := false
	for _, load := range loads {
		if err := ctx.Err(); err != nil {
			cancelled = true
			break
		}
		meters, err := measure(ctx, limited, req, req.Seed.Start, load.Pickup)
		if errors.Is(err, context.Canceled) {
			cancelled = true
			break
		}
		if errors.Is(err, routing.ErrSearchBudget) {
			exhausted = true
			break
		}
		if err != nil {
			result.ReasonCounts[ReasonRoadUnknown]++
			continue
		}
		pool = append(pool, measured{load: load, deadhead: meters})
	}
	sort.Slice(pool, func(i, j int) bool {
		if pool[i].deadhead != pool[j].deadhead {
			return pool[i].deadhead < pool[j].deadhead
		}
		return pool[i].load.ID.String() < pool[j].load.ID.String()
	})
	catalog := append([]measured(nil), pool...)
	if len(pool) > FirstLoadCandidatesMax {
		result.ReasonCounts["SHORTLIST_PRUNED"] += len(pool) - FirstLoadCandidatesMax
		pool = pool[:FirstLoadCandidatesMax]
	}
	for _, item := range pool {
		result.FirstLoadIDs = append(result.FirstLoadIDs, item.load.ID)
	}

	var directed []directedPair
	if !cancelled && !exhausted {
		directed, cancelled, exhausted = e.secondLoads(ctx, req, limited, pool, catalog, &result)
	}
	pairs := dedupePairs(directed)
	if len(pairs) > PairCandidatesMax {
		result.ReasonCounts["PAIR_PRUNED"] += len(pairs) - PairCandidatesMax
		pairs = pairs[:PairCandidatesMax]
	}
	result.PairsRetained = len(pairs)
	built := make([]builtPair, len(pairs))
	for i := range pairs {
		built[i] = builtPair{pair: pairs[i], id: e.nextID()}
	}
	if !cancelled && !exhausted {
		cancelled, exhausted = e.evaluate(ctx, req, limited, now, started, horizonEnd, built, &result)
	}
	result.Candidates = rankCandidates(built)
	switch {
	case cancelled:
		result.Status = StatusCancelled
	case exhausted:
		result.Status = StatusBudgetExhausted
	default:
		result.Status = StatusSearchComplete
		result.SearchComplete = true
	}
	return e.finish(ctx, req, hash, limited, result)
}

func (e Engine) secondLoads(ctx context.Context, req Request, limited *limitedProvider, firsts, catalog []measured, result *Result) ([]directedPair, bool, bool) {
	var directed []directedPair
	for _, first := range firsts {
		if err := ctx.Err(); err != nil {
			return directed, true, false
		}
		var others []measured
		for _, other := range catalog {
			if other.load.ID != first.load.ID {
				others = append(others, other)
			}
		}
		sort.Slice(others, func(i, j int) bool { return others[i].load.ID.String() < others[j].load.ID.String() })
		var links []linkMeasure
		complete := true
		for _, other := range others {
			meters, err := measure(ctx, limited, req, first.load.Delivery, other.load.Pickup)
			if errors.Is(err, context.Canceled) {
				return directed, true, false
			}
			if errors.Is(err, routing.ErrSearchBudget) {
				complete = false
				break
			}
			if err != nil {
				result.ReasonCounts[ReasonRoadUnknown]++
				continue
			}
			links = append(links, linkMeasure{load: other.load, meters: meters})
		}
		if !complete {
			return directed, false, true
		}
		sort.Slice(links, func(i, j int) bool {
			if links[i].meters != links[j].meters {
				return links[i].meters < links[j].meters
			}
			return links[i].load.ID.String() < links[j].load.ID.String()
		})
		if len(links) > SecondLoadCandidatesPerFirstMax {
			links = links[:SecondLoadCandidatesPerFirstMax]
		}
		if len(links) > result.MaxSecondsPerFirst {
			result.MaxSecondsPerFirst = len(links)
		}
		for _, link := range links {
			directed = append(directed, directedPair{first: first, second: link.load, link: link.meters})
		}
	}
	return directed, false, false
}

func (e Engine) evaluate(ctx context.Context, req Request, limited *limitedProvider, now func() time.Time, started, horizonEnd time.Time, pairs []builtPair, result *Result) (bool, bool) {
	limit := RoutePlanEvaluationsMax
	if len(pairs) < limit {
		limit = len(pairs)
	}
	for i := 0; i < limit; i++ {
		if err := ctx.Err(); err != nil {
			return true, false
		}
		if !now().Before(limited.deadline) || limited.exhausted || limited.calls >= ProviderCallsMax {
			return false, true
		}
		result.Evaluations++
		orders := []struct {
			code   string
			first  VisibleLoad
			second VisibleLoad
		}{
			{OrderLowerFirst, pairs[i].pair.low, pairs[i].pair.high},
			{OrderHigherFirst, pairs[i].pair.high, pairs[i].pair.low},
		}
		var scores []Score
		var reasons []string
		for _, order := range orders {
			if err := ctx.Err(); err != nil {
				return true, false
			}
			if !now().Before(limited.deadline) || limited.exhausted || limited.calls >= ProviderCallsMax {
				reasons = append(reasons, ReasonUnfinished)
				return false, true
			}
			outcome, finished, stop, errKind := e.planOrder(ctx, req, limited, now, started, order.first, order.second)
			result.PlannerInvocations++
			result.OrdersEvaluated = append(result.OrdersEvaluated, order.code)
			pairs[i].orders = append(pairs[i].orders, order.code)
			if result.MaxLoadsPerPlan < TotalLoads {
				result.MaxLoadsPerPlan = TotalLoads
			}
			if errKind == stopCancel {
				commitPair(&pairs[i], scores, reasons)
				return true, false
			}
			if errKind == stopBudget {
				reasons = append(reasons, ReasonUnfinished)
				commitPair(&pairs[i], scores, reasons)
				return false, true
			}
			if !finished || stop {
				reasons = append(reasons, ReasonUnfinished)
				continue
			}
			score, feasible, orderReasons := scoreOrder(req, limited, ctx, outcome, order.first, order.second, horizonEnd, order.code)
			reasons = append(reasons, orderReasons...)
			if !feasible {
				continue
			}
			scores = append(scores, score)
		}
		commitPair(&pairs[i], scores, reasons)
	}
	return false, false
}

func commitPair(pair *builtPair, scores []Score, reasons []string) {
	if len(scores) == 0 {
		pair.feasible = false
		pair.reasons = unique(reasons)
		return
	}
	scores = OmitShared(scores)
	sort.SliceStable(scores, func(a, b int) bool { return Better(scores[a], scores[b]) })
	best := scores[0]
	pair.score = &best
	pair.feasible = true
	pair.reasons = nil
}

const (
	stopNone = iota
	stopCancel
	stopBudget
	stopInner
)

func (e Engine) planOrder(ctx context.Context, req Request, limited *limitedProvider, now func() time.Time, started time.Time, first, second VisibleLoad) (routeplan.Outcome, bool, bool, int) {
	remaining := limited.deadline.Sub(now())
	if remaining <= 0 || limited.exhausted || limited.calls >= ProviderCallsMax {
		return routeplan.Outcome{}, false, true, stopBudget
	}
	if remaining > routeplan.TimeBudget {
		remaining = routeplan.TimeBudget
	}
	budget := routeplan.NewBudget(now)
	budget.SequenceCeiling = SequencesPerOrderMax
	budget.LegCeiling = LegsPerOrderMax
	budget.TimeCeiling = remaining
	mode := routeplan.ModeDepotStart
	if req.Seed.Source == SeedCurrentTripResidual {
		mode = routeplan.ModeCurrentTrip
	}
	groupage := req.Groupage
	if groupage == nil {
		groupage = func([]compat.GroupageItem) compat.Result {
			return compat.Result{Status: compat.StatusCompatible}
		}
	}
	outcome, _, err := (routeplan.Planner{}).Plan(ctx, routeplan.Input{
		Mode:                         mode,
		Start:                        routeplan.Stop{Role: routeplan.RoleStart, Point: req.Seed.Start},
		Loads:                        []routeplan.Load{toPlanLoad(first), toPlanLoad(second)},
		BaseSubjects:                 req.Seed.OnboardSubjects,
		Initial:                      toCapacity(req.Seed.Capacity),
		VehicleProfile:               req.VehicleProfile,
		Clock:                        started,
		PreserveInsertionOrder:       true,
		ServiceDurationSeconds:       req.ServiceDurationSeconds,
		ActionServiceSeconds:         req.ActionServiceSeconds,
		ServiceDurationPolicyID:      req.ServiceDurationPolicyID,
		ServiceDurationPolicyVersion: req.ServiceDurationPolicyVersion,
		Groupage:                     groupage,
		Now:                          now,
		Budget:                       budget,
		Route:                        limited,
	})
	if err == nil {
		return outcome, true, false, stopNone
	}
	if errors.Is(err, context.Canceled) {
		return outcome, false, true, stopCancel
	}
	if errors.Is(err, routing.ErrSearchBudget) {
		return outcome, false, true, stopBudget
	}
	var searchErr *routeplan.SearchError
	if errors.As(err, &searchErr) && searchErr.Code == routeplan.ResultBudget {
		if !now().Before(limited.deadline) || limited.exhausted {
			return outcome, false, true, stopBudget
		}
		return outcome, false, true, stopInner
	}
	return outcome, true, false, stopNone
}

func scoreOrder(req Request, limited *limitedProvider, ctx context.Context, outcome routeplan.Outcome, first, second VisibleLoad, horizonEnd time.Time, order string) (Score, bool, []string) {
	var reasons []string
	if outcome.ResultStatus != routeplan.ResultFeasible {
		reasons = append(reasons, ReasonNotFeasible)
		reasons = append(reasons, outcome.ReasonCodes...)
		return Score{}, false, reasons
	}
	if overlaps(outcome.Stops, first.ID, second.ID) {
		reject, reason := stackingDecision(true, first.Stackable, second.Stackable)
		if reject {
			return Score{}, false, []string{reason}
		}
	}
	if !provesHorizon(first, outcome.Stops, horizonEnd) || !provesHorizon(second, outcome.Stops, horizonEnd) {
		return Score{}, false, []string{ReasonHorizonCompletion}
	}
	empty, known := emptyRoad(outcome, first.ID, second.ID)
	if !known {
		return Score{}, false, []string{ReasonRoadUnknown}
	}
	duration := 0
	for _, leg := range outcome.Legs {
		if leg.DurationSeconds < 0 {
			return Score{}, false, []string{ReasonRoadUnknown}
		}
		duration += leg.DurationSeconds
	}
	low, high := first, second
	if high.ID.String() < low.ID.String() {
		low, high = high, low
	}
	score := Score{
		EmptyRoadMeters: empty,
		DurationSeconds: duration,
		Utilization:     utilization(toCapacity(req.Seed.Capacity), outcome.Snapshots),
		Revenue:         pairRevenue(low, high),
		MinLoadID:       low.ID.String(),
		MaxLoadID:       high.ID.String(),
		Order:           order,
	}
	if req.Seed.Target != nil && len(outcome.Stops) > 0 {
		meters, err := measure(ctx, limited, req, outcome.Stops[len(outcome.Stops)-1].Point, *req.Seed.Target)
		if errors.Is(err, context.Canceled) || errors.Is(err, routing.ErrSearchBudget) {
			return score, false, []string{ReasonUnfinished}
		}
		if err == nil {
			score.TargetMeters = &meters
		}
	}
	return score, true, nil
}

func rankCandidates(pairs []builtPair) []ProjectedCandidate {
	var feasible []builtPair
	var rest []builtPair
	var scores []Score
	for _, pair := range pairs {
		if pair.feasible && pair.score != nil {
			feasible = append(feasible, pair)
			scores = append(scores, *pair.score)
		} else {
			rest = append(rest, pair)
		}
	}
	scores = OmitShared(scores)
	for i := range feasible {
		copied := scores[i]
		feasible[i].score = &copied
	}
	sort.SliceStable(feasible, func(i, j int) bool { return Better(*feasible[i].score, *feasible[j].score) })
	sort.SliceStable(rest, func(i, j int) bool {
		return feasibleKey(rest[i]) < feasibleKey(rest[j])
	})
	out := make([]ProjectedCandidate, 0, len(pairs))
	for _, pair := range append(feasible, rest...) {
		out = append(out, projectPair(pair))
	}
	return out
}

func projectPair(pair builtPair) ProjectedCandidate {
	item := ProjectedCandidate{
		CandidateID:     pair.id,
		Loads:           []ProjectedLoad{projectLoad(pair.pair.low), projectLoad(pair.pair.high)},
		OrdersEvaluated: append([]string(nil), pair.orders...),
		Feasible:        pair.feasible,
		ReasonCodes:     append([]string(nil), pair.reasons...),
		RoutePlanID:     nil,
	}
	if pair.score != nil && pair.feasible {
		empty := pair.score.EmptyRoadMeters
		duration := pair.score.DurationSeconds
		item.EmptyRoadMeters = &empty
		item.DurationSeconds = &duration
		item.SelectedOrder = pair.score.Order
		if pair.score.Utilization == nil {
			item.OmittedComponents = append(item.OmittedComponents, OmitUtilization)
		} else {
			u := *pair.score.Utilization
			item.Utilization = &u
		}
		if pair.score.TargetMeters == nil {
			item.OmittedComponents = append(item.OmittedComponents, OmitTarget)
		} else {
			t := *pair.score.TargetMeters
			item.TargetMeters = &t
		}
		if pair.score.Revenue == nil {
			item.OmittedComponents = append(item.OmittedComponents, OmitRevenue)
		} else {
			r := *pair.score.Revenue
			item.RevenueAmount = &r
			// Currency is a non-private code when revenue is authoritative on both loads.
			if pair.pair.low.Private.Currency != "" {
				cur := pair.pair.low.Private.Currency
				item.RevenueCurrency = &cur
			}
		}
	}
	return item
}

func eligibleLoads(carrier uuid.UUID, loads []VisibleLoad, start, end time.Time, counts map[string]int) []VisibleLoad {
	var out []VisibleLoad
	for _, load := range loads {
		if load.ReferenceUnavailable || load.OwnerTenantID == uuid.Nil {
			counts[ReasonReferenceContext]++
			continue
		}
		if load.OwnerTenantID == carrier {
			if !load.ConsolidationAllowed {
				counts[ReasonConsolidation]++
				continue
			}
		} else if !load.CrossShipperConsolidationAllowed {
			counts[ReasonConsolidation]++
			continue
		}
		ok, reason := insideHorizon(load, start, end)
		if !ok {
			counts[reason]++
			continue
		}
		out = append(out, load)
	}
	return out
}

func measure(ctx context.Context, limited *limitedProvider, req Request, from, to routeplan.Point) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	result, err := limited.Route(ctx, routing.RouteRequest{
		Origin:         routing.Point{Latitude: from.Latitude, Longitude: from.Longitude},
		Destination:    routing.Point{Latitude: to.Latitude, Longitude: to.Longitude},
		VehicleProfile: req.VehicleProfile,
		RouteMode:      routing.RouteFastest,
		TrafficMode:    routing.TrafficCurrent,
	})
	if err != nil {
		return 0, err
	}
	if result.Provider == "" || result.DistanceM < 0 || result.DurationSeconds < 0 {
		return 0, routing.ErrRouteNotFound
	}
	return result.DistanceM, nil
}

func dedupePairs(directed []directedPair) []orderedPair {
	seen := map[string]bool{}
	var out []orderedPair
	for _, item := range directed {
		low, high := item.first.load, item.second
		if high.ID.String() < low.ID.String() {
			low, high = high, low
		}
		key := low.ID.String() + ":" + high.ID.String() + ":" + fmt.Sprint(low.Version) + ":" + fmt.Sprint(high.Version)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, orderedPair{low: low, high: high, deadhead: item.first.deadhead, link: item.link})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].deadhead != out[j].deadhead {
			return out[i].deadhead < out[j].deadhead
		}
		if out[i].low.ID.String() != out[j].low.ID.String() {
			return out[i].low.ID.String() < out[j].low.ID.String()
		}
		if out[i].link != out[j].link {
			return out[i].link < out[j].link
		}
		return out[i].high.ID.String() < out[j].high.ID.String()
	})
	return out
}

func emptyRoad(outcome routeplan.Outcome, left, right uuid.UUID) (int, bool) {
	if len(outcome.Stops) == 0 || len(outcome.Legs) != len(outcome.Stops)-1 {
		return 0, false
	}
	on := map[uuid.UUID]bool{}
	apply := func(stop routeplan.Stop) {
		for _, action := range stop.Actions {
			if action.SubjectID != left && action.SubjectID != right {
				continue
			}
			if action.Type == routeplan.ActionPickup {
				on[action.SubjectID] = true
			}
			if action.Type == routeplan.ActionDelivery {
				delete(on, action.SubjectID)
			}
		}
	}
	total := 0
	apply(outcome.Stops[0])
	for i, leg := range outcome.Legs {
		if leg.DistanceM < 0 {
			return 0, false
		}
		if !on[left] && !on[right] {
			total += leg.DistanceM
		}
		apply(outcome.Stops[i+1])
	}
	return total, true
}

func overlaps(stops []routeplan.Stop, left, right uuid.UUID) bool {
	onLeft, onRight := false, false
	for _, stop := range stops {
		for _, action := range stop.Actions {
			if action.Type == routeplan.ActionPickup {
				if action.SubjectID == left {
					onLeft = true
				}
				if action.SubjectID == right {
					onRight = true
				}
			}
			if onLeft && onRight {
				return true
			}
			if action.Type == routeplan.ActionDelivery {
				if action.SubjectID == left {
					onLeft = false
				}
				if action.SubjectID == right {
					onRight = false
				}
			}
		}
	}
	return false
}

func provesHorizon(load VisibleLoad, stops []routeplan.Stop, end time.Time) bool {
	if !load.DeliveryEnd.After(end) {
		return true
	}
	for _, stop := range stops {
		for _, action := range stop.Actions {
			if action.SubjectID == load.ID && action.Type == routeplan.ActionDelivery {
				return stop.Depart != nil && !stop.Depart.After(end)
			}
		}
	}
	return false
}

func utilization(initial routeplan.Capacity, snapshots []routeplan.Snapshot) *float64 {
	type dimFn func(routeplan.Capacity) routeplan.Dimension
	fns := []dimFn{
		func(c routeplan.Capacity) routeplan.Dimension { return c.Payload },
		func(c routeplan.Capacity) routeplan.Dimension { return c.Volume },
		func(c routeplan.Capacity) routeplan.Dimension { return c.Pallets },
		func(c routeplan.Capacity) routeplan.Dimension { return c.Linear },
		func(c routeplan.Capacity) routeplan.Dimension { return c.Height },
	}
	if len(snapshots) == 0 {
		return nil
	}
	maxUsed := 0.0
	for _, fn := range fns {
		base := fn(initial)
		if base.Status != routeplan.DimKnown || base.Value == nil || *base.Value <= 0 {
			return nil
		}
		for _, snap := range snapshots {
			current := fn(snap.Capacity)
			if current.Status != routeplan.DimKnown || current.Value == nil {
				return nil
			}
			used := (*base.Value - *current.Value) / *base.Value
			if used > maxUsed {
				maxUsed = used
			}
		}
	}
	return &maxUsed
}

func pairRevenue(left, right VisibleLoad) *int64 {
	if !left.Private.RevenueAuthoritative || !right.Private.RevenueAuthoritative {
		return nil
	}
	if left.Private.Revenue == nil || right.Private.Revenue == nil {
		return nil
	}
	if left.Private.Currency == "" || left.Private.Currency != right.Private.Currency {
		return nil
	}
	sum := *left.Private.Revenue + *right.Private.Revenue
	return &sum
}

func toPlanLoad(load VisibleLoad) routeplan.Load {
	weight := load.WeightKg
	cargo := compat.Cargo{ID: load.ID.String(), Stackable: load.Stackable, WeightKg: &weight}
	pickupStart, pickupEnd := load.PickupStart, load.PickupEnd
	deliveryStart, deliveryEnd := load.DeliveryStart, load.DeliveryEnd
	return routeplan.Load{
		ID: load.ID, Version: load.Version, OwnerTenantID: load.OwnerTenantID,
		Pickup: load.Pickup, Delivery: load.Delivery,
		PickupAction: routeplan.Action{
			Type: routeplan.ActionPickup, SubjectType: routeplan.SubjectLoad,
			SubjectID: load.ID, SubjectVersion: load.Version, WeightKg: &weight,
			WindowStart: &pickupStart, WindowEnd: &pickupEnd, Cargo: cargo,
		},
		DeliveryAction: routeplan.Action{
			Type: routeplan.ActionDelivery, SubjectType: routeplan.SubjectLoad,
			SubjectID: load.ID, SubjectVersion: load.Version, WeightKg: &weight,
			WindowStart: &deliveryStart, WindowEnd: &deliveryEnd, Cargo: cargo,
		},
	}
}

func toCapacity(capacity Capacity) routeplan.Capacity {
	return routeplan.Capacity{
		Payload: toDim(capacity.Payload), Volume: toDim(capacity.Volume), Pallets: toDim(capacity.Pallets),
		Linear: toDim(capacity.Linear), Height: toDim(capacity.Height), Temperature: "UNALLOCATED",
	}
}

func toDim(dim Dimension) routeplan.Dimension {
	if !dim.Known {
		return routeplan.Dimension{Status: routeplan.DimUnknown}
	}
	value := dim.Value
	return routeplan.Dimension{Status: routeplan.DimKnown, Value: &value}
}

// effectiveComputeDeadline returns the absolute compute stop.
// When ComputeDeadline is set (durable I2B), it governs; otherwise I0 uses started+20s.
func effectiveComputeDeadline(started time.Time, absolute *time.Time) time.Time {
	fresh := started.UTC().Add(WallClockBudget)
	if absolute == nil || absolute.IsZero() {
		return fresh
	}
	persisted := absolute.UTC()
	// Never reopen a fresh window past the persisted durable deadline.
	if persisted.Before(fresh) {
		return persisted
	}
	return fresh
}

func resolveSearchID(e Engine, req Request) uuid.UUID {
	if req.DurableSearchID != nil && *req.DurableSearchID != uuid.Nil {
		return *req.DurableSearchID
	}
	return e.nextID()
}

func resolveHorizon(req Request, started time.Time) (time.Time, time.Time, bool) {
	if req.PersistedHorizonStart != nil && req.PersistedHorizonEnd != nil {
		start := req.PersistedHorizonStart.UTC()
		end := req.PersistedHorizonEnd.UTC()
		if start.IsZero() || end.IsZero() {
			return time.Time{}, time.Time{}, false
		}
		return start, end, true
	}
	return horizonBounds(req.Seed, started)
}

func requestHash(req Request) string {
	loads := append([]VisibleLoad(nil), req.Loads...)
	sort.Slice(loads, func(i, j int) bool { return loads[i].ID.String() < loads[j].ID.String() })
	sum := sha256.New()
	fmt.Fprintf(sum, "policy=%s\nseed=%s\nonboard=%d\n", PolicyVersion, req.Seed.Source, req.Seed.OnboardSubjects)
	writeDim := func(name string, dim Dimension) {
		fmt.Fprintf(sum, "%s=%t:%g\n", name, dim.Known, dim.Value)
	}
	writeDim("payload", req.Seed.Capacity.Payload)
	writeDim("volume", req.Seed.Capacity.Volume)
	writeDim("pallets", req.Seed.Capacity.Pallets)
	writeDim("linear", req.Seed.Capacity.Linear)
	writeDim("height", req.Seed.Capacity.Height)
	if req.ServiceDurationSeconds != nil {
		fmt.Fprintf(sum, "service=%d\n", *req.ServiceDurationSeconds)
	}
	for _, load := range loads {
		fmt.Fprintf(sum, "load=%s:%d\n", load.ID, load.Version)
	}
	return hex.EncodeToString(sum.Sum(nil))
}

func (e Engine) nextID() uuid.UUID {
	if e.NewID != nil {
		return e.NewID()
	}
	return uuid.New()
}

func (e Engine) finish(ctx context.Context, req Request, hash string, limited *limitedProvider, result Result) (Result, error) {
	result.NoRetry = true
	result.ExecutionSupported = false
	result.GlobalOptimum = false
	result.PolicyVersion = PolicyVersion
	if limited != nil {
		result.ProviderCalls = limited.calls
		result.CallTimeouts = append([]time.Duration(nil), limited.timeouts...)
	}
	if result.Status == StatusBudgetExhausted || result.Status == StatusCancelled || result.Status == StatusRejected {
		result.SearchComplete = false
	}
	if e.Repository != nil && req.IdempotencyKey != "" && result.SearchID != uuid.Nil {
		if err := e.Repository.Commit(ctx, req.TenantID, req.IdempotencyKey, hash, result); err != nil && !errors.Is(err, ErrIdempotencyConflict) {
			return result, err
		}
	}
	return result, nil
}

func baseResult() Result {
	return Result{
		NoRetry: true, ExecutionSupported: false, GlobalOptimum: false,
		PolicyVersion: PolicyVersion, ReasonCounts: map[string]int{},
	}
}

func unique(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func feasibleKey(pair builtPair) string {
	return pair.pair.low.ID.String() + pair.pair.high.ID.String()
}

type measured struct {
	load     VisibleLoad
	deadhead int
}

type linkMeasure struct {
	load   VisibleLoad
	meters int
}

type directedPair struct {
	first  measured
	second VisibleLoad
	link   int
}

type orderedPair struct {
	low, high VisibleLoad
	deadhead  int
	link      int
}

type builtPair struct {
	pair     orderedPair
	id       uuid.UUID
	orders   []string
	reasons  []string
	score    *Score
	feasible bool
}

type unavailableProvider struct{}

func (unavailableProvider) Route(context.Context, routing.RouteRequest) (routing.RouteResult, error) {
	return routing.RouteResult{}, routing.ErrProviderUnavailable
}

func (unavailableProvider) Matrix(context.Context, routing.MatrixRequest) (routing.MatrixResult, error) {
	return routing.MatrixResult{}, routing.ErrProviderUnavailable
}

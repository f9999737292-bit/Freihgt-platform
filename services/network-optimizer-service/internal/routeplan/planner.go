package routeplan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

type Dimension struct {
	Status string
	Value  *float64
}

type Capacity struct {
	Payload     Dimension
	Volume      Dimension
	Pallets     Dimension
	Linear      Dimension
	Height      Dimension
	Temperature string
}

type Action struct {
	Type            string
	SubjectType     string
	SubjectID       uuid.UUID
	SubjectVersion  int
	WeightKg        *float64
	VolumeM3        *float64
	Pallets         *float64
	LinearMeters    *float64
	HeightMM        *float64
	WindowStart     *time.Time
	WindowEnd       *time.Time
	ShipmentID      *uuid.UUID
	ShipmentVersion *int
	EvidenceState   string
	EvidenceVersion *int
	EvidenceAt      *time.Time
	Cargo           compat.Cargo
}

type Stop struct {
	Role    string
	Point   Point
	Actions []Action
	Arrival *time.Time
	Depart  *time.Time
	Service *int
}

type Load struct {
	ID             uuid.UUID
	Version        int
	OwnerTenantID  uuid.UUID
	Pickup         Point
	Delivery       Point
	PickupAction   Action
	DeliveryAction Action
}

type Leg struct {
	FromIndex           int
	ToIndex             int
	FromFingerprint     string
	ToFingerprint       string
	DistanceM           int
	DurationSeconds     int
	Provider            string
	RequestFingerprint  string
	ResponseFingerprint string
	VehicleProfileHash  string
	RouteMode           string
	TrafficMode         string
	DepartureBucket     string
	CalculatedAt        time.Time
	ExpiresAt           time.Time
}

type Snapshot struct {
	SequenceOrdinal    int
	AfterStopIndex     int
	AfterActionOrdinal int
	Capacity           Capacity
}

type Outcome struct {
	Stops              []Stop
	Legs               []Leg
	Snapshots          []Snapshot
	ResultStatus       string
	ReasonCodes        []string
	PickupOrdinal      int
	DeliveryOrdinal    int
	DurationSeconds    int
	ExecutionSupported bool
}

type Input struct {
	Mode                   string
	Start                  Stop
	End                    *Stop
	Loads                  []Load
	BaseSubjects           int
	Initial                Capacity
	Onboard                []compat.GroupageItem
	VehicleProfile         routing.VehicleProfile
	Clock                  time.Time
	ServiceDurationSeconds *int
	ReferenceUnavailable   bool
	Route                  routing.Provider
	Groupage               func([]compat.GroupageItem) compat.Result
	Now                    func() time.Time
	Budget                 *Budget
	cache                  map[string]Leg
}

type Planner struct{}

func (Planner) Plan(ctx context.Context, in Input) (Outcome, *Budget, error) {
	budget := in.Budget
	if budget == nil {
		budget = NewBudget(in.Now)
	}
	if in.BaseSubjects+len(in.Loads) > MaxRouteLoadSubjects || len(in.Loads) > MaxAdditionalLoads {
		return Outcome{}, budget, &SearchError{Code: ResultLoadLimit, Budget: BudgetLoads, Detail: ReasonBudgetExceeded}
	}
	if in.cache == nil {
		in.cache = map[string]Leg{}
	}
	loads := append([]Load(nil), in.Loads...)
	sort.Slice(loads, func(i, j int) bool { return loads[i].ID.String() < loads[j].ID.String() })
	parent := []Stop{cloneStop(in.Start)}
	if in.End != nil {
		parent = append(parent, cloneStop(*in.End))
	}
	if err := validateAnchors(parent); err != nil {
		return Outcome{}, budget, err
	}
	winner := parent
	var last Outcome
	found := false
	for _, load := range loads {
		next, ok, err := insertLoad(ctx, in, budget, winner, load)
		if err != nil {
			return Outcome{}, budget, err
		}
		if !ok {
			return Outcome{}, budget, &SearchError{Code: ResultNoPlan, Detail: ReasonCanonicalCargo}
		}
		winner = next.Stops
		last = next
		found = true
	}
	if !found {
		return Outcome{}, budget, &SearchError{Code: ResultNoPlan}
	}
	last.ExecutionSupported = false
	last.ReasonCodes = uniqueSorted(last.ReasonCodes)
	return last, budget, nil
}

func insertLoad(ctx context.Context, in Input, budget *Budget, parent []Stop, load Load) (Outcome, bool, error) {
	if !canonicalCargo(load.Pickup) || !canonicalCargo(load.Delivery) {
		return Outcome{}, false, nil
	}
	sequences := enumerate(parent, load)
	var best *Outcome
	blocked := ""
	for _, seq := range sequences {
		if len(seq) > MaxStops {
			blocked = BudgetStops
			continue
		}
		if actionsExceed(seq) {
			blocked = BudgetActions
			continue
		}
		outcome, err := evaluate(ctx, in, budget, seq, load.ID)
		if err != nil {
			return Outcome{}, false, err
		}
		if outcome == nil {
			continue
		}
		if best == nil || better(*outcome, *best) {
			copyOutcome := *outcome
			best = &copyOutcome
		}
	}
	if best == nil {
		if blocked != "" {
			budget.BlockedReason = blocked
			return Outcome{}, false, &SearchError{Code: ResultBudget, Budget: blocked, Detail: ReasonBudgetExceeded}
		}
		return Outcome{}, false, nil
	}
	return *best, true, nil
}

func better(next, current Outcome) bool {
	nextOK := next.ResultStatus == ResultFeasible
	currentOK := current.ResultStatus == ResultFeasible
	if nextOK != currentOK {
		return nextOK
	}
	if next.DurationSeconds != current.DurationSeconds {
		return next.DurationSeconds < current.DurationSeconds
	}
	if next.PickupOrdinal != current.PickupOrdinal {
		return next.PickupOrdinal < current.PickupOrdinal
	}
	if next.DeliveryOrdinal != current.DeliveryOrdinal {
		return next.DeliveryOrdinal < current.DeliveryOrdinal
	}
	return false
}

func enumerate(parent []Stop, load Load) [][]Stop {
	if sameLocation(load.Pickup, load.Delivery) {
		if matches := matching(parent, load.Pickup); len(matches) > 0 {
			var out [][]Stop
			for _, index := range matches {
				built, ok := apply(parent, load, slot{attach: index, insert: -1}, slot{attach: index, insert: -1})
				if ok {
					out = append(out, built)
				}
			}
			return out
		}
		return enumerateSameNewStop(parent, load)
	}
	pickupSlots := placements(parent, load.Pickup, false)
	deliverySlots := placements(parent, load.Delivery, true)
	var out [][]Stop
	for _, pickup := range pickupSlots {
		for _, delivery := range deliverySlots {
			built, ok := apply(parent, load, pickup, delivery)
			if ok {
				out = append(out, built)
			}
		}
	}
	return out
}

type slot struct {
	attach int
	insert int
}

func placements(parent []Stop, point Point, delivery bool) []slot {
	matches := matching(parent, point)
	if len(matches) > 0 {
		var slots []slot
		for _, index := range matches {
			if len(parent[index].Actions) >= MaxActionsPerStop {
				continue
			}
			slots = append(slots, slot{attach: index, insert: -1})
		}
		return slots
	}
	endBound := len(parent)
	if len(parent) > 0 && parent[len(parent)-1].Role == RoleEnd {
		endBound = len(parent) - 1
	}
	var slots []slot
	for i := 1; i <= endBound; i++ {
		if !delivery && i > endBound {
			continue
		}
		slots = append(slots, slot{attach: -1, insert: i})
	}
	return slots
}

func enumerateSameNewStop(parent []Stop, load Load) [][]Stop {
	if len(matching(parent, load.Pickup)) > 0 {
		return nil
	}
	endBound := len(parent)
	if len(parent) > 0 && parent[len(parent)-1].Role == RoleEnd {
		endBound = len(parent) - 1
	}
	var out [][]Stop
	for i := 1; i <= endBound; i++ {
		stop := Stop{Role: RoleCargo, Point: load.Pickup, Actions: []Action{load.PickupAction, load.DeliveryAction}}
		built := insertAt(parent, i, stop)
		if validateAnchors(built) == nil {
			out = append(out, built)
		}
	}
	return out
}

func apply(parent []Stop, load Load, pickup, delivery slot) ([]Stop, bool) {
	switch {
	case pickup.attach >= 0 && delivery.attach >= 0:
		if pickup.attach > delivery.attach {
			return nil, false
		}
		built := cloneStops(parent)
		if pickup.attach == delivery.attach {
			if len(built[pickup.attach].Actions)+2 > MaxActionsPerStop {
				return nil, false
			}
			built[pickup.attach].Actions = append(built[pickup.attach].Actions, load.PickupAction, load.DeliveryAction)
			return built, validateAnchors(built) == nil
		}
		if len(built[pickup.attach].Actions)+1 > MaxActionsPerStop || len(built[delivery.attach].Actions)+1 > MaxActionsPerStop {
			return nil, false
		}
		built[pickup.attach].Actions = append(built[pickup.attach].Actions, load.PickupAction)
		built[delivery.attach].Actions = append(built[delivery.attach].Actions, load.DeliveryAction)
		return built, validateAnchors(built) == nil
	case pickup.attach >= 0 && delivery.insert >= 0:
		if delivery.insert <= pickup.attach {
			return nil, false
		}
		built := cloneStops(parent)
		if len(built[pickup.attach].Actions)+1 > MaxActionsPerStop {
			return nil, false
		}
		built[pickup.attach].Actions = append(built[pickup.attach].Actions, load.PickupAction)
		built = insertAt(built, delivery.insert, Stop{Role: RoleCargo, Point: load.Delivery, Actions: []Action{load.DeliveryAction}})
		return built, validateAnchors(built) == nil
	case pickup.insert >= 0 && delivery.attach >= 0:
		if pickup.insert > delivery.attach {
			return nil, false
		}
		built := insertAt(parent, pickup.insert, Stop{Role: RoleCargo, Point: load.Pickup, Actions: []Action{load.PickupAction}})
		deliveryIndex := delivery.attach
		if pickup.insert <= delivery.attach {
			deliveryIndex++
		}
		if len(built[deliveryIndex].Actions)+1 > MaxActionsPerStop {
			return nil, false
		}
		built[deliveryIndex].Actions = append(built[deliveryIndex].Actions, load.DeliveryAction)
		return built, validateAnchors(built) == nil
	default:
		if pickup.insert > delivery.insert {
			return nil, false
		}
		built := insertAt(parent, pickup.insert, Stop{Role: RoleCargo, Point: load.Pickup, Actions: []Action{load.PickupAction}})
		deliveryIndex := delivery.insert
		if pickup.insert <= delivery.insert {
			deliveryIndex++
		}
		built = insertAt(built, deliveryIndex, Stop{Role: RoleCargo, Point: load.Delivery, Actions: []Action{load.DeliveryAction}})
		return built, validateAnchors(built) == nil
	}
}

func evaluate(ctx context.Context, in Input, budget *Budget, stops []Stop, loadID uuid.UUID) (*Outcome, error) {
	if err := budget.openSequence(); err != nil {
		return nil, err
	}
	if err := validateAnchors(stops); err != nil {
		return nil, nil
	}
	reasons := []string{}
	if emptyIntersection(stops) {
		return nil, nil
	}
	if in.ServiceDurationSeconds == nil {
		reasons = append(reasons, ReasonServiceDurationUnknown)
	}
	if in.ReferenceUnavailable {
		reasons = append(reasons, ReasonMultiPartyContext)
	}
	onboard := append([]compat.GroupageItem(nil), in.Onboard...)
	capacity := in.Initial
	snapshots := []Snapshot{{SequenceOrdinal: 0, AfterStopIndex: 0, AfterActionOrdinal: 0, Capacity: capacity}}
	seq := 1
	var legs []Leg
	total := 0
	departure := &in.Clock
	durationKnown := in.ServiceDurationSeconds != nil
	for i := range stops {
		if i > 0 {
			if err := budget.before(BudgetTime); err != nil {
				return nil, err
			}
			item := groupageItems(onboard)
			if in.Groupage != nil && len(item) > 0 {
				if err := budget.openGroupage(); err != nil {
					return nil, err
				}
				result := in.Groupage(item)
				if result.Status == compat.StatusIncompatible || len(result.HardRejects) > 0 {
					return nil, nil
				}
				if result.Status == compat.StatusIndeterminate || len(result.IndeterminateReasons) > 0 {
					if len(result.IndeterminateReasons) == 0 {
						reasons = append(reasons, ReasonCapacityUnknown)
					}
					for _, reason := range result.IndeterminateReasons {
						reasons = append(reasons, reason.ReasonCode)
					}
				}
			}
			leg, err := routeLeg(ctx, in, budget, stops[i-1], stops[i], departure)
			if err != nil {
				return nil, err
			}
			legs = append(legs, leg)
			total += leg.DurationSeconds
			if durationKnown && departure != nil {
				arrival := departure.Add(time.Duration(leg.DurationSeconds) * time.Second)
				windowStart, windowEnd, ok := stopWindow(stops[i])
				if !ok {
					return nil, nil
				}
				if windowEnd != nil && arrival.After(*windowEnd) {
					return nil, nil
				}
				if windowStart != nil && arrival.Before(*windowStart) {
					arrival = *windowStart
				}
				stops[i].Arrival = &arrival
				service := *in.ServiceDurationSeconds
				stops[i].Service = &service
				depart := arrival.Add(time.Duration(service) * time.Second)
				stops[i].Depart = &depart
				departure = &depart
			} else if stops[i].Role != RoleEnd {
				departure = nil
			}
		}
		for a := range stops[i].Actions {
			action := stops[i].Actions[a]
			var hard bool
			var unknown bool
			capacity, hard, unknown = applyAction(capacity, action)
			if hard {
				return nil, nil
			}
			if unknown {
				reasons = append(reasons, ReasonCapacityUnknown)
			}
			if action.Type == ActionPickup {
				onboard = append(onboard, compat.GroupageItem{Cargo: action.Cargo})
			} else {
				onboard = removeCargo(onboard, action.SubjectID)
			}
			snapshots = append(snapshots, Snapshot{
				SequenceOrdinal: seq, AfterStopIndex: i, AfterActionOrdinal: a + 1, Capacity: capacity,
			})
			seq++
		}
	}
	status := ResultFeasible
	if len(reasons) > 0 || !durationKnown {
		status = ResultIndeterminate
		if !durationKnown {
			reasons = append(reasons, ReasonServiceDurationUnknown)
		}
	}
	pickup, delivery := ordinals(stops, loadID)
	return &Outcome{
		Stops: stops, Legs: legs, Snapshots: snapshots, ResultStatus: status,
		ReasonCodes: uniqueSorted(reasons), PickupOrdinal: pickup, DeliveryOrdinal: delivery,
		DurationSeconds: total, ExecutionSupported: false,
	}, nil
}

func routeLeg(ctx context.Context, in Input, budget *Budget, from, to Stop, departure *time.Time) (Leg, error) {
	if err := budget.openLeg(); err != nil {
		return Leg{}, err
	}
	traffic := routing.TrafficCurrent
	if departure != nil {
		traffic = routing.TrafficStatistical
	}
	profile := in.VehicleProfile.Hash()
	key := LegKey(from.Point.Fingerprint(), to.Point.Fingerprint(), profile, routing.RouteFastest, traffic, DepartureBucket(traffic, departure))
	if cached, ok := in.cache[key]; ok {
		return cached, nil
	}
	if err := budget.openProvider(); err != nil {
		return Leg{}, err
	}
	if in.Route == nil {
		return Leg{}, &SearchError{Code: ResultRoutingDown}
	}
	result, err := in.Route.Route(ctx, routing.RouteRequest{
		Origin:      routing.Point{Latitude: from.Point.Latitude, Longitude: from.Point.Longitude},
		Destination: routing.Point{Latitude: to.Point.Latitude, Longitude: to.Point.Longitude},
		DepartureAt: departure, VehicleProfile: in.VehicleProfile,
		RouteMode: routing.RouteFastest, TrafficMode: traffic,
	})
	if err != nil || result.DistanceM < 0 || result.DurationSeconds < 0 || result.Provider == "" {
		return Leg{}, &SearchError{Code: ResultRoutingDown}
	}
	response := routing.Fingerprint(result.Provider, routing.RouteRequest{
		Origin:      routing.Point{Latitude: from.Point.Latitude, Longitude: from.Point.Longitude},
		Destination: routing.Point{Latitude: to.Point.Latitude, Longitude: to.Point.Longitude},
		DepartureAt: departure, VehicleProfile: in.VehicleProfile,
		RouteMode: routing.RouteFastest, TrafficMode: traffic,
	})
	leg := Leg{
		FromFingerprint: from.Point.Fingerprint(), ToFingerprint: to.Point.Fingerprint(),
		DistanceM: result.DistanceM, DurationSeconds: result.DurationSeconds,
		Provider: result.Provider, RequestFingerprint: key, ResponseFingerprint: response,
		VehicleProfileHash: profile, RouteMode: routing.RouteFastest, TrafficMode: traffic,
		DepartureBucket: DepartureBucket(traffic, departure),
		CalculatedAt:    result.CalculatedAt, ExpiresAt: result.ExpiresAt,
	}
	if in.cache == nil {
		in.cache = map[string]Leg{}
	}
	in.cache[key] = leg
	return leg, nil
}

func applyAction(capacity Capacity, action Action) (Capacity, bool, bool) {
	unknown := false
	hard := false
	consume := action.Type == ActionPickup
	capacity.Payload, hard, unknown = moveDim(capacity.Payload, action.WeightKg, consume, hard, unknown)
	capacity.Volume, hard, unknown = moveDim(capacity.Volume, action.VolumeM3, consume, hard, unknown)
	capacity.Pallets, hard, unknown = moveDim(capacity.Pallets, action.Pallets, consume, hard, unknown)
	capacity.Linear, hard, unknown = moveDim(capacity.Linear, action.LinearMeters, consume, hard, unknown)
	if action.HeightMM != nil {
		if capacity.Height.Status != DimKnown || capacity.Height.Value == nil {
			unknown = true
		} else if action.Type == ActionPickup && *action.HeightMM > *capacity.Height.Value {
			hard = true
		}
	}
	return capacity, hard, unknown
}

func moveDim(dim Dimension, delta *float64, consume, hard, unknown bool) (Dimension, bool, bool) {
	if delta == nil || hard {
		return dim, hard, unknown
	}
	if dim.Status != DimKnown || dim.Value == nil {
		return dim, hard, true
	}
	next := *dim.Value
	if consume {
		next -= *delta
	} else {
		next += *delta
	}
	if next < 0 {
		return dim, true, unknown
	}
	value := next
	dim.Value = &value
	dim.Status = DimKnown
	return dim, hard, unknown
}

func canonicalCargo(point Point) bool {
	return point.Kind == PointCanonical && point.LocationID != nil && point.Source == SourceCanonical
}

func sameLocation(left, right Point) bool {
	return left.LocationID != nil && right.LocationID != nil && *left.LocationID == *right.LocationID
}

func matching(stops []Stop, point Point) []int {
	if !canonicalCargo(point) {
		return nil
	}
	var indexes []int
	for i, stop := range stops {
		if stop.Role == RoleStart || stop.Point.Kind == PointAnchor || stop.Point.LocationID == nil {
			continue
		}
		if stop.Role != RoleCargo && stop.Role != RoleEnd {
			continue
		}
		if *stop.Point.LocationID == *point.LocationID {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func validateAnchors(stops []Stop) error {
	if len(stops) == 0 || stops[0].Role != RoleStart {
		return &SearchError{Code: ResultNoPlan, Detail: ReasonAnchorOrder}
	}
	end := -1
	for i, stop := range stops {
		if stop.Role == RoleEnd {
			end = i
		}
		if stop.Role == RoleStart && i != 0 {
			return &SearchError{Code: ResultNoPlan, Detail: ReasonAnchorOrder}
		}
	}
	if end >= 0 && end != len(stops)-1 {
		return &SearchError{Code: ResultNoPlan, Detail: ReasonAnchorOrder}
	}
	return nil
}

func emptyIntersection(stops []Stop) bool {
	for _, stop := range stops {
		if _, _, ok := stopWindow(stop); !ok {
			return true
		}
	}
	return false
}

func stopWindow(stop Stop) (*time.Time, *time.Time, bool) {
	var start *time.Time
	var end *time.Time
	for _, action := range stop.Actions {
		if action.WindowStart != nil && (start == nil || action.WindowStart.After(*start)) {
			value := *action.WindowStart
			start = &value
		}
		if action.WindowEnd != nil && (end == nil || action.WindowEnd.Before(*end)) {
			value := *action.WindowEnd
			end = &value
		}
	}
	if start != nil && end != nil && start.After(*end) {
		return nil, nil, false
	}
	return start, end, true
}

func ordinals(stops []Stop, loadID uuid.UUID) (int, int) {
	pickup, delivery := 0, 0
	for i, stop := range stops {
		for _, action := range stop.Actions {
			if action.SubjectID != loadID || action.SubjectType != SubjectLoad {
				continue
			}
			if action.Type == ActionPickup {
				pickup = i + 1
			}
			if action.Type == ActionDelivery {
				delivery = i + 1
			}
		}
	}
	return pickup, delivery
}

func actionsExceed(stops []Stop) bool {
	for _, stop := range stops {
		if len(stop.Actions) > MaxActionsPerStop {
			return true
		}
	}
	return false
}

func groupageItems(items []compat.GroupageItem) []compat.GroupageItem {
	out := append([]compat.GroupageItem(nil), items...)
	sort.Slice(out, func(i, j int) bool { return out[i].Cargo.ID < out[j].Cargo.ID })
	return out
}

func removeCargo(items []compat.GroupageItem, id uuid.UUID) []compat.GroupageItem {
	out := make([]compat.GroupageItem, 0, len(items))
	for _, item := range items {
		if item.Cargo.ID == id.String() {
			continue
		}
		out = append(out, item)
	}
	return out
}

func insertAt(stops []Stop, index int, stop Stop) []Stop {
	cloned := cloneStops(stops)
	if index < 0 || index > len(cloned) {
		return cloned
	}
	cloned = append(cloned, Stop{})
	copy(cloned[index+1:], cloned[index:])
	cloned[index] = cloneStop(stop)
	return cloned
}

func cloneStops(stops []Stop) []Stop {
	out := make([]Stop, len(stops))
	for i, stop := range stops {
		out[i] = cloneStop(stop)
	}
	return out
}

func cloneStop(stop Stop) Stop {
	stop.Actions = append([]Action(nil), stop.Actions...)
	return stop
}

func uniqueSorted(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func EvaluationFingerprint(outcome Outcome) string {
	var b strings.Builder
	b.WriteString(outcome.ResultStatus)
	b.WriteString("|")
	for _, code := range uniqueSorted(outcome.ReasonCodes) {
		b.WriteString(code)
		b.WriteString(",")
	}
	for i, stop := range outcome.Stops {
		b.WriteString("#")
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString(stop.Role)
		b.WriteString(stop.Point.Fingerprint())
		for j, action := range stop.Actions {
			b.WriteString(action.Type)
			b.WriteString(action.SubjectType)
			b.WriteString(action.SubjectID.String())
			b.WriteString(strconv.Itoa(j + 1))
		}
	}
	for _, leg := range outcome.Legs {
		b.WriteString(leg.FromFingerprint)
		b.WriteString(leg.ToFingerprint)
		b.WriteString(strconv.Itoa(leg.DistanceM))
		b.WriteString(strconv.Itoa(leg.DurationSeconds))
		b.WriteString(leg.TrafficMode)
		b.WriteString(leg.DepartureBucket)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

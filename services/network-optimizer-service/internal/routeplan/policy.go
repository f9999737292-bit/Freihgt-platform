package routeplan

import "time"

const (
	AlgorithmPolicyVersion = "BOUNDED_INCREMENTAL_HEURISTIC/v0.4b"
	RoutingPolicyVersion   = "road-routing/v1"
	BudgetPolicyVersion    = "nlo-0.4b-budgets/v1"

	ModeCurrentTrip = "CURRENT_TRIP"
	ModeDepotStart  = "DEPOT_START"

	StatusEvaluated = "EVALUATED"

	ResultFeasible      = "FEASIBLE_PLAN_FOUND"
	ResultIndeterminate = "INDETERMINATE_PLAN_FOUND"
	ResultNoPlan        = "NO_PLAN_FOUND_WITHIN_POLICY"
	ResultBudget        = "SEARCH_BUDGET_EXHAUSTED"
	ResultRoutingDown   = "ROUTING_UNAVAILABLE"
	ResultStartUnknown  = "ROUTE_START_POSITION_UNKNOWN"
	ResultLoadLimit     = "PLAN_LOAD_LIMIT_EXCEEDED"

	RoleStart = "START"
	RoleCargo = "CARGO"
	RoleEnd   = "END"

	PointCanonical = "CANONICAL_LOCATION"
	PointAnchor    = "POSITION_ANCHOR"

	SourceCanonical = "CANONICAL_LOCATION"
	SourceTracking  = "TRACKING_POSITION"
	SourceCapacity  = "CAPACITY_POSITION"

	ActionPickup   = "PICKUP"
	ActionDelivery = "DELIVERY"

	SubjectLoad  = "LOAD_OPPORTUNITY"
	SubjectCargo = "SHIPMENT_CARGO"

	DimKnown   = "KNOWN"
	DimUnknown = "UNKNOWN"

	ReasonBudgetExceeded         = "SEARCH_BUDGET_EXCEEDED"
	ReasonServiceDurationUnknown = "SERVICE_DURATION_UNKNOWN"
	ReasonStopWindow             = "STOP_WINDOW_VIOLATION"
	ReasonWindowIntersection     = "STOP_WINDOW_INTERSECTION_EMPTY"
	ReasonCapacityExceeded       = "CAPACITY_EXCEEDED"
	ReasonCapacityUnknown        = "CAPACITY_UNKNOWN"
	ReasonCanonicalCargo         = "CANONICAL_CARGO_LOCATION_REQUIRED"
	ReasonMultiPartyContext      = "MULTI_PARTY_REFERENCE_CONTEXT_UNAVAILABLE"
	ReasonAnchorOrder            = "ANCHOR_ORDER_VIOLATION"
	ReasonRoutingProfileUnknown  = "ROUTING_VEHICLE_PROFILE_UNKNOWN"

	BudgetSequences = "sequences"
	BudgetRouting   = "routing"
	BudgetGroupage  = "groupage"
	BudgetTime      = "time"
	BudgetStops     = "stops"
	BudgetActions   = "actions"
	BudgetLoads     = "loads"

	MaxRouteLoadSubjects    = 4
	MaxAdditionalLoads      = 2
	MaxStops                = 8
	MaxActionsPerStop       = 4
	MaxSequenceCandidates   = 64
	MaxRouteLegEvaluations  = 300
	MaxRoutingProviderCalls = 300
	MaxGroupageEvaluations  = 600
	TimeBudget              = 5 * time.Second
)

// Future shipment multi-stop execution is NLO-0.4D.
// This wave has no authoritative future-route load list.
const ExistingFutureRouteLoadSource = "NONE"

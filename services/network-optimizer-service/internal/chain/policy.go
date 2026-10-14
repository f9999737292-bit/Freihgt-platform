package chain

import "time"

const (
	PolicyVersion = "NLO-0.9C-I0/BOUNDED_TWO_LOAD"

	TotalLoads = 2
	Horizon    = 48 * time.Hour

	FirstLoadCandidatesMax          = 8
	SecondLoadCandidatesPerFirstMax = 2
	PairCandidatesMax               = 4
	RoutePlanEvaluationsMax         = 2
	OrdersPerPair                   = 2
	SequencesPerOrderMax            = 16
	LegsPerOrderMax                 = 6
	ProviderCallsMax                = 24
	WallClockBudget                 = 20 * time.Second
	ProviderCallTimeout             = 5 * time.Second
	ProviderRetryCount              = 0

	OrderLowerFirst  = "L1_BEFORE_L2"
	OrderHigherFirst = "L2_BEFORE_L1"

	StatusSearchComplete      = "SEARCH_COMPLETE"
	StatusBudgetExhausted     = "CHAIN_BUDGET_EXHAUSTED"
	StatusCancelled           = "CANCELLED"
	StatusRejected            = "REJECTED"
	StatusIdempotencyConflict = "IDEMPOTENCY_CONFLICT"

	SeedManualAvailable     = "MANUAL_AVAILABLE_CAPACITY"
	SeedCurrentTripResidual = "CURRENT_TRIP_RESIDUAL"
	SeedPredictedCapacity   = "PREDICTED_CAPACITY"

	ReasonPredictedCapacity = "PREDICTED_CAPACITY_EXCLUDED"
	ReasonClientChainID     = "CLIENT_CHAIN_ID_REJECTED"
	ReasonOutsideHorizon    = "OUTSIDE_HORIZON"
	ReasonWindowNotAbsolute = "WINDOW_NOT_ABSOLUTE"
	ReasonConsolidation     = "CONSOLIDATION_NOT_ALLOWED"
	ReasonReferenceContext  = "MULTI_PARTY_REFERENCE_CONTEXT_UNAVAILABLE"
	ReasonRoadUnknown       = "ROAD_DISTANCE_UNKNOWN"
	ReasonStackingUnknown   = "STACKING_UNKNOWN"
	ReasonStackingRejected  = "STACKING_REJECTED"
	ReasonNotFeasible       = "NOT_FEASIBLE"
	ReasonUnfinished        = "EVALUATION_UNFINISHED"
	ReasonHorizonCompletion = "HORIZON_COMPLETION_UNPROVEN"
	ReasonCapacityUnknown   = "CAPACITY_UNKNOWN"
)

// Omitted objective components are absent. They are not numeric zero.
const (
	OmitUtilization = "KNOWN_CAPACITY_UTILIZATION"
	OmitTarget      = "TARGET_ROAD_DISTANCE"
	OmitRevenue     = "AUTHORITATIVE_REVENUE"
)

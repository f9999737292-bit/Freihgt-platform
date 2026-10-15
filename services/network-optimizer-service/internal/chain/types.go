package chain

import (
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/routeplan"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

// Dimension is known only when Known is true. An unknown value is not zero.
type Dimension struct {
	Known bool
	Value float64
}

type Capacity struct {
	Payload Dimension
	Volume  Dimension
	Pallets Dimension
	Linear  Dimension
	Height  Dimension
}

type Seed struct {
	Source          string
	AvailableFrom   *time.Time
	AvailableUntil  *time.Time
	Start           routeplan.Point
	Target          *routeplan.Point
	Capacity        Capacity
	OnboardSubjects int
}

// PrivateFacts are held for ranking and then dropped from the projection.
type PrivateFacts struct {
	ShipperName          string
	CustomerName         string
	ContractRate         string
	Revenue              *int64
	Currency             string
	RevenueAuthoritative bool
	InternalShipmentID   *uuid.UUID
	ExactAddress         string
}

type VisibleLoad struct {
	ID                               uuid.UUID
	Version                          int
	OwnerTenantID                    uuid.UUID
	ConsolidationAllowed             bool
	CrossShipperConsolidationAllowed bool
	ReferenceUnavailable             bool
	Pickup                           routeplan.Point
	Delivery                         routeplan.Point
	PickupStart                      time.Time
	PickupEnd                        time.Time
	DeliveryStart                    time.Time
	DeliveryEnd                      time.Time
	Stackable                        *bool
	WeightKg                         float64
	PublicationLabel                 string
	Private                          PrivateFacts
}

type Request struct {
	TenantID               uuid.UUID
	IdempotencyKey         string
	ClientChainID          *uuid.UUID
	Seed                   Seed
	Loads                  []VisibleLoad
	VehicleProfile         routing.VehicleProfile
	ServiceDurationSeconds *int
	// ActionServiceSeconds maps routeplan action types (PICKUP/DELIVERY) to
	// claim-time stored durations. When set, it supersedes ServiceDurationSeconds.
	ActionServiceSeconds         map[string]int
	ServiceDurationPolicyID      *uuid.UUID
	ServiceDurationPolicyVersion *int
	// ComputeDeadline is an absolute wall-clock stop for durable I2B execution.
	// When set, the engine uses it instead of started+WallClockBudget so a late
	// start cannot reopen a fresh 20s window past the persisted claim deadline.
	ComputeDeadline *time.Time
	Groupage        func([]compat.GroupageItem) compat.Result
	Now             func() time.Time
	Route           routing.Provider
}

type Result struct {
	SearchID           uuid.UUID            `json:"chain_search_id"`
	Status             string               `json:"status"`
	SearchComplete     bool                 `json:"search_complete"`
	GlobalOptimum      bool                 `json:"global_optimum"`
	NoRetry            bool                 `json:"no_retry"`
	ExecutionSupported bool                 `json:"execution_supported"`
	PolicyVersion      string               `json:"policy_version"`
	Candidates         []ProjectedCandidate `json:"candidates"`
	Evaluations        int                  `json:"route_plan_evaluations"`
	PlannerInvocations int                  `json:"planner_invocations"`
	ProviderCalls      int                  `json:"provider_calls"`
	MaxLoadsPerPlan    int                  `json:"max_loads_per_plan"`
	FirstLoadIDs       []uuid.UUID          `json:"first_load_ids,omitempty"`
	MaxSecondsPerFirst int                  `json:"max_seconds_per_first"`
	PairsRetained      int                  `json:"pairs_retained"`
	ReasonCounts       map[string]int       `json:"reason_counts,omitempty"`
	HorizonStart       time.Time            `json:"horizon_start"`
	HorizonEnd         time.Time            `json:"horizon_end"`
	CallTimeouts       []time.Duration      `json:"-"`
	OrdersEvaluated    []string             `json:"orders_evaluated,omitempty"`
}

package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Chain search statuses frozen by NLO-0.9C-I1.
const (
	ChainSearchPending   = "PENDING"
	ChainSearchRunning   = "RUNNING"
	ChainSearchCompleted = "COMPLETED"
	ChainSearchCancelled = "CANCELLED"
	ChainSearchFailed    = "FAILED"
)

const (
	ChainSeedManualAvailableCapacity = "MANUAL_AVAILABLE_CAPACITY"
	ChainSeedCurrentTripResidual     = "CURRENT_TRIP_RESIDUAL"
)

const (
	ChainFeasible   = "FEASIBLE"
	ChainInfeasible = "INFEASIBLE"
	ChainUnfinished = "UNFINISHED"
)

const (
	ChainOrderL1BeforeL2 = "L1_BEFORE_L2"
	ChainOrderL2BeforeL1 = "L2_BEFORE_L1"
)

// ServiceDurationSnapshot is the immutable duration values used by one chain run.
// Unknown is distinct from a known zero.
type ServiceDurationSnapshot struct {
	State                 string     `json:"state"` // KNOWN or UNKNOWN
	PolicyID              *uuid.UUID `json:"policy_id,omitempty"`
	PolicyVersion         *int       `json:"policy_version,omitempty"`
	PickupKnown           bool       `json:"pickup_known"`
	PickupSeconds         *int       `json:"pickup_seconds,omitempty"`
	DeliveryKnown         bool       `json:"delivery_known"`
	DeliverySeconds       *int       `json:"delivery_seconds,omitempty"`
	FingerprintProvenance string     `json:"fingerprint_provenance,omitempty"`
}

const (
	ServiceDurationKnown   = "KNOWN"
	ServiceDurationUnknown = "UNKNOWN"
)

// ChainSearchRun is the durable search-run row for NLO-0.9C chain persistence.
type ChainSearchRun struct {
	ID                           uuid.UUID
	TenantID                     uuid.UUID
	Status                       string
	CreatedAt                    time.Time
	StartedAt                    *time.Time
	CompletedAt                  *time.Time
	CancelledAt                  *time.Time
	RequestFingerprint           string
	PolicyVersion                string
	AlgorithmVersion             string
	CatalogFingerprint           string
	RoutingProviderVersion       string
	DeadlineAt                   time.Time
	OwnerToken                   uuid.UUID
	ClaimExpiresAt               time.Time
	ServiceDurationPolicyID      *uuid.UUID
	ServiceDurationPolicyVersion *int
	ServiceDurationSnapshot      ServiceDurationSnapshot
	HorizonStart                 time.Time
	HorizonEnd                   time.Time
	SeedSource                   string
	SeedCapacityID               *uuid.UUID
	SeedCapacityVersion          *int
	SeedShipmentID               *uuid.UUID
	SeedShipmentVersion          *int
	CandidateCount               int
	EvaluatedPairCount           int
	ProviderCallCount            int
	SearchComplete               bool
	GlobalOptimum                bool
	ExecutionSupported           bool
	FailureCode                  *string
	CancelReasonClass            *string
	ReasonCounts                 map[string]int
}

// ChainCandidate is one unordered pair shortlist row.
type ChainCandidate struct {
	CandidateID          uuid.UUID
	SearchID             uuid.UUID
	TenantID             uuid.UUID
	Ordinal              int
	FirstLoadID          uuid.UUID
	FirstLoadVersion     int
	SecondLoadID         uuid.UUID
	SecondLoadVersion    int
	FeasibilityStatus    string
	SelectedOrder        *string
	EmptyRoadDistanceM   *float64
	TotalDurationSeconds *int
	Utilization          *float64
	TargetDistanceM      *float64
	RevenueAmount        *int64
	RevenueCurrency      *string
	OmittedComponents    []string
	ReasonCodes          []string
	RoutePlanID          *uuid.UUID
	CandidateFingerprint string
	CreatedAt            time.Time
}

// ChainSearchIdempotency maps a tenant idempotency key to one search fingerprint.
type ChainSearchIdempotency struct {
	TenantID           uuid.UUID
	IdempotencyKey     string
	RequestFingerprint string
	SearchID           uuid.UUID
	CreatedAt          time.Time
}

// ChainSearchStore is the durable chain-search persistence port (I2A foundation).
// It does not implement I2B claim/recovery orchestration.
type ChainSearchStore interface {
	WithinChainSearch(context.Context, func(ChainSearchTx) error) error
}

// ChainSearchTx is a transaction-scoped chain persistence surface for I2A/I2B.
// Mutation paths are intentionally separated so deadline and claim-expiry gates
// cannot be confused or bypassed with optional parameters.
type ChainSearchTx interface {
	// InsertChainSearchRun creates a PENDING claim. Status is forced to PENDING,
	// owner_token is generated internally, and claim timestamps come from one DB clock seed.
	InsertChainSearchRun(context.Context, ChainSearchRun) (ChainSearchRun, error)
	GetChainSearchRun(context.Context, uuid.UUID, uuid.UUID) (ChainSearchRun, error)
	LockChainSearchRun(context.Context, uuid.UUID, uuid.UUID) error

	// MarkChainSearchRunning is the only PENDING -> RUNNING / compute-start path.
	// Requires exact owner_token and clock_timestamp() before deadline_at.
	MarkChainSearchRunning(ctx context.Context, tenant, searchID, ownerToken uuid.UUID) (bool, error)

	// OwnerPersistChainSearchRun applies owner persistence/terminal writes under the frozen
	// transition graph, exact owner_token, and clock_timestamp() before claim_expires_at.
	OwnerPersistChainSearchRun(ctx context.Context, run ChainSearchRun, ownerToken uuid.UUID, expectedStatus string) (bool, error)

	// RecoverExpiredChainSearchClaim is an explicit recovery CAS primitive for I2B.
	// It only terminalizes expired PENDING/RUNNING rows to FAILED/CHAIN_INTERRUPTED
	// when clock_timestamp() >= claim_expires_at.
	RecoverExpiredChainSearchClaim(ctx context.Context, tenant, searchID uuid.UUID) (bool, error)

	InsertChainCandidate(context.Context, ChainCandidate) error
	ListChainCandidates(context.Context, uuid.UUID, uuid.UUID) ([]ChainCandidate, error)
	InsertChainSearchIdempotency(context.Context, ChainSearchIdempotency) error
	GetChainSearchIdempotency(context.Context, uuid.UUID, string) (ChainSearchIdempotency, error)
}

// Failure code for abandoned-claim recovery (I1).
const ChainInterruptedFailureCode = "CHAIN_INTERRUPTED"

func marshalServiceDurationSnapshot(snapshot ServiceDurationSnapshot) ([]byte, error) {
	if snapshot.State == "" {
		snapshot.State = ServiceDurationUnknown
	}
	return json.Marshal(snapshot)
}

func unmarshalServiceDurationSnapshot(raw []byte) (ServiceDurationSnapshot, error) {
	var snapshot ServiceDurationSnapshot
	if len(raw) == 0 {
		return ServiceDurationSnapshot{State: ServiceDurationUnknown}, nil
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return ServiceDurationSnapshot{}, err
	}
	if snapshot.State == "" {
		snapshot.State = ServiceDurationUnknown
	}
	return snapshot, nil
}

func marshalReasonCounts(counts map[string]int) ([]byte, error) {
	if counts == nil {
		counts = map[string]int{}
	}
	return json.Marshal(counts)
}

func unmarshalReasonCounts(raw []byte) (map[string]int, error) {
	if len(raw) == 0 {
		return map[string]int{}, nil
	}
	out := map[string]int{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]int{}
	}
	return out, nil
}

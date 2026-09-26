package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ConsolidationRun struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	CapacityID         uuid.UUID
	CapacityVersion    int
	Pattern            string
	StartedAt          time.Time
	CompletedAt        time.Time
	Status             string
	CandidateLimit     *int
	PoolLoadCount      int
	EvaluatedPairCount int
	CreatedAt          time.Time
}

type ConsolidationMember struct {
	Ordinal           int
	LoadOpportunityID uuid.UUID
	LoadVersion       int
	LoadOwnerTenantID uuid.UUID
	CreatedAt         time.Time
}

type ConsolidationCandidate struct {
	ID                       uuid.UUID
	SearchRunID              uuid.UUID
	TenantID                 uuid.UUID
	CapacityID               uuid.UUID
	Pattern                  string
	Status                   string
	ExecutionSupported       bool
	CompatibilityStatus      string
	CompatibilityFingerprint string
	CandidateFingerprint     string
	PickupOverlapStart       *time.Time
	PickupOverlapEnd         *time.Time
	DeliveryOverlapStart     *time.Time
	DeliveryOverlapEnd       *time.Time
	PlacementCheck           string
	HardRejectReasons        []string
	IndeterminateReasonCodes []string
	Conditions               []byte
	Warnings                 []byte
	CapacityUsage            []byte
	CompatibilityTrace       []byte
	CreatedAt                time.Time
	Members                  []ConsolidationMember
}

type ConsolidationStore interface {
	SaveConsolidation(ctx context.Context, run ConsolidationRun, candidates []ConsolidationCandidate) error
	GetConsolidation(ctx context.Context, tenant, id uuid.UUID) (ConsolidationRun, []ConsolidationCandidate, error)
}

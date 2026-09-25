package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type SearchRun struct {
	ID                         uuid.UUID
	TenantID                   uuid.UUID
	CapacityID                 uuid.UUID
	CapacityVersion            int
	EffectivePolicyFingerprint string
	StartedAt                  time.Time
	CompletedAt                time.Time
	RoutingProvider            string
	Status                     string
	ScoreProfileCode           string
	ScoreProfileVersion        int
	ScoreProfileFingerprint    string
	ScoringAlgorithmVersion    string
	RankingCurrency            *string
}

type StoredCandidate struct {
	ID                       uuid.UUID
	SearchRunID              uuid.UUID
	TenantID                 uuid.UUID
	CapacityID               uuid.UUID
	LoadOpportunityID        uuid.UUID
	LoadVersion              int
	Eligibility              string
	RejectReasons            []string
	RoadDeadheadKm           *float64
	RoadDeadheadMinutes      *float64
	WaitingMinutes           *float64
	CompatibilityStatus      string
	CompatibilityFingerprint string
	PolicyFingerprint        string
	CreatedAt                time.Time
	Rank                     *int
	ScoreStatus              string
	ScoreTotal               *int
	ScoreEvidenceBps         *int
	ScoreFingerprint         *string
	ScoreComponents          []byte
	UnrankedReasonCodes      []string
}

type SearchStore interface {
	SaveSearch(ctx context.Context, run SearchRun, candidates []StoredCandidate) error
	GetSearch(ctx context.Context, tenant, id uuid.UUID) (SearchRun, []StoredCandidate, error)
}

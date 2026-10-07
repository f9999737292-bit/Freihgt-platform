package domain

import "time"

// Routing-stage bounds. CandidateDiscoveryCap stays 1000 and is not changed here.
// CandidateFinalEvaluationCap is frozen policy. Scoring and persistence enforcement
// stays a later wave.
const (
	CandidateRoutingCap         = 25
	MatrixDimensionLimit        = 25
	MaxMatrixCallsPerSearch     = 4
	MaxRouteCallsPerSearch      = 2
	MaxProviderCallsTotal       = 6
	ProviderRequestTimeout      = 5 * time.Second
	SearchHardBudget            = 30 * time.Second
	SearchParallelism           = 1
	ProviderRetryCount          = 0
	CandidateFinalEvaluationCap = 25
)

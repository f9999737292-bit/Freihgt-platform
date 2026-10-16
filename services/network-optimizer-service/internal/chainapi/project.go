package chainapi

import (
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

// SearchResource is the public chain-search document. Internal fencing and
// private commercial fields are not present.
type SearchResource struct {
	ChainSearchID      uuid.UUID         `json:"chain_search_id"`
	Status             string            `json:"status"`
	SearchComplete     bool              `json:"search_complete"`
	CandidateCount     int               `json:"candidate_count"`
	EvaluatedPairCount int               `json:"evaluated_pair_count"`
	ProviderCallCount  int               `json:"provider_call_count"`
	GlobalOptimum      bool              `json:"global_optimum"`
	ExecutionSupported bool              `json:"execution_supported"`
	FailureCode        *string           `json:"failure_code,omitempty"`
	CancelReasonClass  *string           `json:"cancel_reason_class,omitempty"`
	ReasonCounts       map[string]int    `json:"reason_counts"`
	CreatedAt          time.Time         `json:"created_at"`
	StartedAt          *time.Time        `json:"started_at,omitempty"`
	CompletedAt        *time.Time        `json:"completed_at,omitempty"`
	CancelledAt        *time.Time        `json:"cancelled_at,omitempty"`
	HorizonStart       time.Time         `json:"horizon_start"`
	HorizonEnd         time.Time         `json:"horizon_end"`
	Seed               PublicSeed        `json:"seed"`
	Candidates         []PublicCandidate `json:"candidates,omitempty"`
}

type PublicSeed struct {
	Type            string     `json:"type"`
	CapacityID      *uuid.UUID `json:"capacity_id,omitempty"`
	CapacityVersion *int       `json:"capacity_version,omitempty"`
	ShipmentID      *uuid.UUID `json:"shipment_id,omitempty"`
	ShipmentVersion *int       `json:"shipment_version,omitempty"`
}

type PublicCandidate struct {
	ChainCandidateID     uuid.UUID  `json:"chain_candidate_id"`
	Ordinal              int        `json:"ordinal"`
	FirstLoadID          uuid.UUID  `json:"first_load_id"`
	FirstLoadVersion     int        `json:"first_load_version"`
	SecondLoadID         uuid.UUID  `json:"second_load_id"`
	SecondLoadVersion    int        `json:"second_load_version"`
	FeasibilityStatus    string     `json:"feasibility_status"`
	SelectedOrder        *string    `json:"selected_order,omitempty"`
	EmptyRoadDistanceM   *float64   `json:"empty_road_distance_m,omitempty"`
	TotalDurationSeconds *int       `json:"total_duration_seconds,omitempty"`
	Utilization          *float64   `json:"utilization,omitempty"`
	TargetDistanceM      *float64   `json:"target_distance_m,omitempty"`
	RevenueAmount        *int64     `json:"revenue_amount,omitempty"`
	RevenueCurrency      *string    `json:"revenue_currency,omitempty"`
	OmittedComponents    []string   `json:"omitted_components"`
	ReasonCodes          []string   `json:"reason_codes"`
	RoutePlanID          *uuid.UUID `json:"route_plan_id,omitempty"`
	ExecutionSupported   bool       `json:"execution_supported"`
}

type CandidateList struct {
	Candidates []PublicCandidate `json:"candidates"`
}

func projectSearch(run repository.ChainSearchRun, candidates []repository.ChainCandidate, includeCandidates bool) SearchResource {
	out := SearchResource{
		ChainSearchID:      run.ID,
		Status:             run.Status,
		SearchComplete:     run.SearchComplete,
		CandidateCount:     run.CandidateCount,
		EvaluatedPairCount: run.EvaluatedPairCount,
		ProviderCallCount:  run.ProviderCallCount,
		GlobalOptimum:      false,
		ExecutionSupported: false,
		FailureCode:        run.FailureCode,
		CancelReasonClass:  run.CancelReasonClass,
		ReasonCounts:       run.ReasonCounts,
		CreatedAt:          run.CreatedAt.UTC(),
		StartedAt:          utcPtr(run.StartedAt),
		CompletedAt:        utcPtr(run.CompletedAt),
		CancelledAt:        utcPtr(run.CancelledAt),
		HorizonStart:       run.HorizonStart.UTC(),
		HorizonEnd:         run.HorizonEnd.UTC(),
		Seed:               projectSeed(run),
	}
	if out.ReasonCounts == nil {
		out.ReasonCounts = map[string]int{}
	}
	if includeCandidates {
		if run.Status == repository.ChainSearchCompleted {
			out.Candidates = projectCandidates(candidates)
		} else {
			out.Candidates = []PublicCandidate{}
		}
	}
	return out
}

func projectSeed(run repository.ChainSearchRun) PublicSeed {
	switch run.SeedSource {
	case repository.ChainSeedManualAvailableCapacity:
		return PublicSeed{Type: PublicSeedManualCapacity, CapacityID: run.SeedCapacityID, CapacityVersion: run.SeedCapacityVersion}
	case repository.ChainSeedCurrentTripResidual:
		return PublicSeed{Type: PublicSeedCurrentTrip, ShipmentID: run.SeedShipmentID, ShipmentVersion: run.SeedShipmentVersion}
	default:
		return PublicSeed{Type: run.SeedSource}
	}
}

func projectCandidates(rows []repository.ChainCandidate) []PublicCandidate {
	out := make([]PublicCandidate, 0, len(rows))
	for _, row := range rows {
		item := PublicCandidate{
			ChainCandidateID:     row.CandidateID,
			Ordinal:              row.Ordinal,
			FirstLoadID:          row.FirstLoadID,
			FirstLoadVersion:     row.FirstLoadVersion,
			SecondLoadID:         row.SecondLoadID,
			SecondLoadVersion:    row.SecondLoadVersion,
			FeasibilityStatus:    row.FeasibilityStatus,
			SelectedOrder:        row.SelectedOrder,
			EmptyRoadDistanceM:   row.EmptyRoadDistanceM,
			TotalDurationSeconds: row.TotalDurationSeconds,
			Utilization:          row.Utilization,
			TargetDistanceM:      row.TargetDistanceM,
			OmittedComponents:    append([]string(nil), row.OmittedComponents...),
			ReasonCodes:          append([]string(nil), row.ReasonCodes...),
			RoutePlanID:          row.RoutePlanID,
			ExecutionSupported:   false,
		}
		if row.RevenueAmount != nil && row.RevenueCurrency != nil && *row.RevenueCurrency != "" {
			item.RevenueAmount = row.RevenueAmount
			item.RevenueCurrency = row.RevenueCurrency
		}
		if item.OmittedComponents == nil {
			item.OmittedComponents = []string{}
		}
		if item.ReasonCodes == nil {
			item.ReasonCodes = []string{}
		}
		out = append(out, item)
	}
	return out
}

func utcPtr(in *time.Time) *time.Time {
	if in == nil {
		return nil
	}
	value := in.UTC()
	return &value
}

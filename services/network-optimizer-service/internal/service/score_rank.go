package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

func (s *Service) activeScoreProfile(ctx context.Context, code string) (domain.ScoreProfile, error) {
	if s.scoreProfiles == nil {
		return domain.ScoreProfile{}, apperrors.Validation("score profile store is unavailable", map[string]any{"reason": "SCORE_PROFILE_STORE_UNAVAILABLE"})
	}
	profile, err := s.scoreProfiles.ActiveScoreProfile(ctx, code)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrScoringAlgorithmUnsupported):
			return domain.ScoreProfile{}, apperrors.Validation("scoring algorithm version is not supported", map[string]any{"reason": "SCORING_ALGORITHM_UNSUPPORTED"})
		case errors.Is(err, domain.ErrScoreProfileInvalid):
			return domain.ScoreProfile{}, apperrors.Validation("active score profile is invalid", map[string]any{"reason": "SCORE_PROFILE_INVALID"})
		case errors.Is(err, repository.ErrNotFound):
			return domain.ScoreProfile{}, apperrors.Validation("objective profile is not executable", map[string]any{
				"reason": "OBJECTIVE_PROFILE_NOT_EXECUTABLE",
				"detail": "PLANNING_COST_PROVIDER_NOT_IMPLEMENTED",
			})
		default:
			return domain.ScoreProfile{}, apperrors.Validation("active score profile is invalid", map[string]any{"reason": "SCORE_PROFILE_INVALID"})
		}
	}
	if err := domain.ValidateActiveScoreProfile(profile); err != nil {
		if errors.Is(err, domain.ErrScoringAlgorithmUnsupported) {
			return domain.ScoreProfile{}, apperrors.Validation("scoring algorithm version is not supported", map[string]any{"reason": "SCORING_ALGORITHM_UNSUPPORTED"})
		}
		return domain.ScoreProfile{}, apperrors.Validation("active score profile is invalid", map[string]any{"reason": "SCORE_PROFILE_INVALID"})
	}
	return profile, nil
}

func scoreEligible(rows []evaluatedLoad, profile domain.ScoreProfile, policy domain.NextLoadSearchPolicy, prediction *domain.PredictedCapacity, capacity domain.Capacity) (ranked, unranked int) {
	facts := make([]domain.ScoreFacts, 0, len(rows))
	index := make([]int, 0, len(rows))
	for i := range rows {
		if len(rows[i].reasons) > 0 {
			rows[i].score.Status = domain.ScoreNotApplicable
			continue
		}
		facts = append(facts, scoreFacts(rows[i], policy, prediction, capacity))
		index = append(index, i)
	}
	scored := domain.ScorePool(profile, facts, policy.RankingCurrency)
	return assignPoolScores(rows, index, scored)
}

// assignPoolScores writes a ScorePool result onto eligible rows and ranks only
// rows that have a total. It does not treat a missing component as zero.
func assignPoolScores(rows []evaluatedLoad, index []int, scored map[uuid.UUID]domain.CandidateScore) (ranked, unranked int) {
	rankedIDs := make([]int, 0)
	for _, i := range index {
		rows[i].score = scored[rows[i].load.ID]
		rows[i].rank = nil
		if rows[i].score.Status == domain.ScoreRanked {
			rankedIDs = append(rankedIDs, i)
			ranked++
		} else {
			rows[i].score.Status = domain.ScoreUnranked
			unranked++
		}
	}
	sort.Slice(rankedIDs, func(a, b int) bool {
		left, right := rows[rankedIDs[a]], rows[rankedIDs[b]]
		if *left.score.Total != *right.score.Total {
			return *left.score.Total > *right.score.Total
		}
		if *left.score.EvidenceBps != *right.score.EvidenceBps {
			return *left.score.EvidenceBps > *right.score.EvidenceBps
		}
		if valueOr(left.roadKm, math.MaxFloat64) != valueOr(right.roadKm, math.MaxFloat64) {
			return valueOr(left.roadKm, math.MaxFloat64) < valueOr(right.roadKm, math.MaxFloat64)
		}
		return left.load.ID.String() < right.load.ID.String()
	})
	for n, i := range rankedIDs {
		rank := n + 1
		rows[i].rank = &rank
	}
	return ranked, unranked
}

// projectPublicRevenue rebuilds the public score when an anonymized load has a
// hidden amount. That amount stays in the stored private score. It is omitted
// from the public pool instead of being inserted as zero, so published
// normalization, totals, and ranks cannot be reversed into the hidden rate.
func projectPublicRevenue(rows []evaluatedLoad, profile domain.ScoreProfile, policy domain.NextLoadSearchPolicy, prediction *domain.PredictedCapacity, capacity domain.Capacity) []evaluatedLoad {
	if !profileHasComponent(profile, domain.ComponentRevenue) || !anonymizedRevenueInPool(rows) {
		return rows
	}
	cloned := append([]evaluatedLoad(nil), rows...)
	facts := make([]domain.ScoreFacts, 0, len(cloned))
	index := make([]int, 0, len(cloned))
	for i := range cloned {
		if len(cloned[i].reasons) > 0 {
			continue
		}
		fact := scoreFacts(cloned[i], policy, prediction, capacity)
		if cloned[i].load.VisibilityScope == domain.VisAnonymized {
			fact.Revenue = nil
			fact.RevenueCurrency = ""
		}
		facts = append(facts, fact)
		index = append(index, i)
	}
	assignPoolScores(cloned, index, domain.ScorePool(profile, facts, policy.RankingCurrency))
	return cloned
}

func profileHasComponent(profile domain.ScoreProfile, code string) bool {
	for _, component := range profile.Components {
		if component.Code == code {
			return true
		}
	}
	return false
}

func anonymizedRevenueInPool(rows []evaluatedLoad) bool {
	for _, row := range rows {
		if len(row.reasons) > 0 || row.load.VisibilityScope != domain.VisAnonymized || row.load.Commercial.Amount == nil {
			continue
		}
		return true
	}
	return false
}

func scoreFacts(row evaluatedLoad, policy domain.NextLoadSearchPolicy, prediction *domain.PredictedCapacity, capacity domain.Capacity) domain.ScoreFacts {
	fact := domain.ScoreFacts{
		LoadID: row.load.ID, DeadheadKm: row.roadKm, WaitingKnown: row.waitingKnown, WaitingMinutes: row.waiting,
		SlackKnown: row.slackKnown, SlackMinutes: row.slackMinutes, TargetKm: row.deliveryToTarget,
		CapacityID: capacity.ID, CapacityVersion: capacity.Version, LoadVersion: row.load.Version,
		CompatibilityFingerprint: row.fingerprint, PolicyFingerprint: policy.Fingerprint(),
	}
	if row.usage != nil {
		fact.Utilization = domain.UtilizationRatio(
			row.usage.WeightUsed, row.usage.WeightAvailable,
			row.usage.VolumeUsed, row.usage.VolumeAvailable,
			row.usage.PalletsUsed, row.usage.PalletsAvailable,
			row.usage.LinearMetersUsed, row.usage.LinearMetersAvailable,
		)
	}
	if row.load.Commercial.Amount != nil {
		fact.Revenue = row.load.Commercial.Amount
		fact.RevenueCurrency = row.load.Commercial.Currency
	}
	if prediction != nil && capacity.Source == domain.SourceCurrentShipmentPrediction {
		confidence := prediction.Confidence
		uncertainty := prediction.UncertaintySeconds
		fact.Confidence = &confidence
		fact.UncertaintySeconds = &uncertainty
	}
	return fact
}

func publicComponents(load domain.LoadOpportunity, components []domain.ScoreComponentResult) []domain.ScoreComponentResult {
	out := append([]domain.ScoreComponentResult(nil), components...)
	anonymized := load.VisibilityScope == domain.VisAnonymized
	showAmount := load.MarketplaceView().Commercial != nil && load.MarketplaceView().Commercial.Amount != nil
	for i := range out {
		switch out[i].Code {
		case domain.ComponentDeadheadEfficiency:
			if anonymized {
				bucket := ""
				if out[i].RawValue != nil {
					bucket = domain.DeadheadBucket(*out[i].RawValue)
				}
				out[i].RawValue = nil
				out[i].RawUnit = ""
				out[i].Explanation = "Road deadhead bucket " + bucket + " compared within the feasible pool"
			}
		case domain.ComponentTargetProximity:
			if anonymized {
				out[i].RawValue = nil
				out[i].RawUnit = ""
				out[i].Explanation = "Delivery to the explicit target is comparatively near or far within the feasible pool"
			}
		case domain.ComponentRevenue:
			// Anonymized loads never keep a raw rate or the points derived from it.
			if anonymized || !showAmount {
				out[i].RawValue = nil
				out[i].RawUnit = ""
				if anonymized {
					out[i].NormalizedPoints = nil
					out[i].WeightedPoints = nil
				}
				if anonymized && out[i].Status == domain.ComponentAvailable {
					out[i].Status = domain.ComponentUnavailable
					out[i].ReasonCode = domain.ReasonCommercialUnavailable
					out[i].Explanation = "Published commercial amount is absent"
				} else if out[i].Status == domain.ComponentAvailable {
					out[i].Explanation = "Published commercial amount compared within the feasible pool"
				}
			}
		}
	}
	return out
}

func marshalComponents(components []domain.ScoreComponentResult) ([]byte, error) {
	if components == nil {
		components = []domain.ScoreComponentResult{}
	}
	return json.Marshal(components)
}

func responseRows(rows []evaluatedLoad, limit *int) []evaluatedLoad {
	ranked := make([]evaluatedLoad, 0)
	unranked := make([]evaluatedLoad, 0)
	for _, row := range rows {
		if len(row.reasons) > 0 {
			continue
		}
		if row.score.Status == domain.ScoreRanked && row.rank != nil {
			ranked = append(ranked, row)
		} else {
			unranked = append(unranked, row)
		}
	}
	sort.Slice(ranked, func(i, j int) bool { return *ranked[i].rank < *ranked[j].rank })
	sort.Slice(unranked, func(i, j int) bool { return unranked[i].load.ID.String() < unranked[j].load.ID.String() })
	if limit != nil {
		if *limit <= 0 {
			return []evaluatedLoad{}
		}
		if *limit < len(ranked) {
			return ranked[:*limit]
		}
		return ranked
	}
	return append(ranked, unranked...)
}

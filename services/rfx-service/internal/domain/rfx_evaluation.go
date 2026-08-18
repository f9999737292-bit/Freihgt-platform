package domain

import (
	"math"
	"sort"

	"github.com/google/uuid"
)

const (
	ParticipantStatusShortlisted = "SHORTLISTED"
	ParticipantStatusAwarded     = "AWARDED"
	ParticipantStatusNotAwarded  = "NOT_AWARDED"
)

type EvaluationCandidate struct {
	ResponseID           uuid.UUID
	ParticipantCompanyID uuid.UUID
	TotalAmount          float64
	CurrencyCode         string
	CommercialScore      float64
	ManualScore          *float64
	TotalScore           float64
	Rank                 int
	Comparable           bool
	ParticipantStatus    string
}

func IsResponseEligibleForEvaluation(status string) bool {
	return status == RfxResponseStatusSubmitted
}

func ComputeCommercialScores(candidates []EvaluationCandidate) []EvaluationCandidate {
	if len(candidates) == 0 {
		return candidates
	}
	currency := candidates[0].CurrencyCode
	lowest := math.MaxFloat64
	allComparable := true
	for _, c := range candidates {
		if c.CurrencyCode != currency || c.TotalAmount <= 0 {
			allComparable = false
			break
		}
		if c.TotalAmount < lowest {
			lowest = c.TotalAmount
		}
	}
	out := make([]EvaluationCandidate, len(candidates))
	copy(out, candidates)
	if !allComparable || lowest == math.MaxFloat64 || lowest <= 0 {
		for i := range out {
			out[i].Comparable = false
			out[i].CommercialScore = 0
			out[i].TotalScore = manualOrZero(out[i].ManualScore)
		}
		return out
	}
	for i := range out {
		out[i].Comparable = true
		out[i].CommercialScore = roundMoney((lowest / out[i].TotalAmount) * 100)
		out[i].TotalScore = combineScores(out[i].CommercialScore, out[i].ManualScore)
	}
	return out
}

func combineScores(commercial float64, manual *float64) float64 {
	if manual == nil {
		return commercial
	}
	return roundMoney(commercial*0.7 + (*manual)*0.3)
}

func CombineScoresPublic(commercial float64, manual *float64) float64 {
	return combineScores(commercial, manual)
}

func manualOrZero(manual *float64) float64 {
	if manual == nil {
		return 0
	}
	return *manual
}

func RankEvaluationCandidates(candidates []EvaluationCandidate) []EvaluationCandidate {
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].TotalScore == candidates[j].TotalScore {
			return candidates[i].TotalAmount < candidates[j].TotalAmount
		}
		return candidates[i].TotalScore > candidates[j].TotalScore
	})
	rank := 1
	for i := range candidates {
		if !candidates[i].Comparable && candidates[i].TotalScore == 0 {
			candidates[i].Rank = 0
			continue
		}
		if i > 0 && candidates[i].TotalScore == candidates[i-1].TotalScore {
			candidates[i].Rank = candidates[i-1].Rank
		} else {
			candidates[i].Rank = rank
		}
		rank++
	}
	return candidates
}

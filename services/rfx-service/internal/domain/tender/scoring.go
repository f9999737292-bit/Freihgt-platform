package tender

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Scoring factors (allow-listed; no arbitrary expressions).
const (
	FactorPrice        = "PRICE"
	FactorSLA          = "SLA"
	FactorCarrierKPI   = "CARRIER_KPI"
	FactorCapacity     = "CAPACITY"
	FactorReliability  = "RELIABILITY"
	FactorTransitTime  = "TRANSIT_TIME"
)

var AllowedFactors = map[string]struct{}{
	FactorPrice: {}, FactorSLA: {}, FactorCarrierKPI: {}, FactorCapacity: {},
	FactorReliability: {}, FactorTransitTime: {},
}

const (
	WeightSumTarget   = 100.0
	WeightSumTolerance = 0.01
	ScoreMin          = 0.0
	ScoreMax          = 100.0
)

type ScoringFactorWeight struct {
	Factor string  `json:"factor"`
	Weight float64 `json:"weight"`
}

type ScoringTemplateSnapshot struct {
	VersionNumber int                   `json:"version_number"`
	Factors       []ScoringFactorWeight `json:"factors"`
}

func ValidateScoringTemplate(factors []ScoringFactorWeight) error {
	if len(factors) == 0 {
		return fmt.Errorf("at least one scoring factor is required")
	}
	seen := make(map[string]struct{}, len(factors))
	sum := 0.0
	for _, f := range factors {
		factor := strings.TrimSpace(strings.ToUpper(f.Factor))
		if _, ok := AllowedFactors[factor]; !ok {
			return fmt.Errorf("unsupported scoring factor: %s", f.Factor)
		}
		if _, dup := seen[factor]; dup {
			return fmt.Errorf("duplicate scoring factor: %s", factor)
		}
		seen[factor] = struct{}{}
		if math.IsNaN(f.Weight) || f.Weight < 0 {
			return fmt.Errorf("invalid weight for factor %s", factor)
		}
		sum += f.Weight
	}
	if math.Abs(sum-WeightSumTarget) > WeightSumTolerance {
		return fmt.Errorf("factor weights must sum to 100, got %.4f", sum)
	}
	return nil
}

// BidCandidate is the normalized input for scoring/allocation engines.
type BidCandidate struct {
	CarrierCompanyID string
	LotID            string
	PriceAmount      float64
	CurrencyCode     string
	CapacityUnits    float64
	TransitHours     float64
	SLAScoreInput    float64
	CarrierKPIInput  float64
	ReliabilityInput float64
	CarrierActive    bool
	RevisionNumber   int
	BidRevisionID    string
}

type FactorContribution struct {
	Factor       string  `json:"factor"`
	Weight       float64 `json:"weight"`
	RawScore     float64 `json:"raw_score"`
	Contribution float64 `json:"contribution"`
}

type CarrierScoreResult struct {
	CarrierCompanyID string               `json:"carrier_company_id"`
	LotID            string               `json:"lot_id,omitempty"`
	BidRevisionID    string               `json:"bid_revision_id,omitempty"`
	TotalScore       float64              `json:"total_score"`
	Contributions    []FactorContribution `json:"contributions"`
	PriceScore       float64              `json:"price_score"`
	SLAScore         float64              `json:"sla_score"`
	CarrierKPIScore  float64              `json:"carrier_kpi_score"`
	CapacityScore    float64              `json:"capacity_score"`
	ReliabilityScore float64              `json:"reliability_score"`
	TransitScore     float64              `json:"transit_time_score"`
}

func roundScore(v float64) float64 {
	return math.Round(v*10000) / 10000
}

func clampScore(v float64) float64 {
	if v < ScoreMin {
		return ScoreMin
	}
	if v > ScoreMax {
		return ScoreMax
	}
	return roundScore(v)
}

func normalizePriceScores(candidates []BidCandidate) map[string]float64 {
	out := make(map[string]float64, len(candidates))
	lowest := 0.0
	first := true
	for _, c := range candidates {
		if c.PriceAmount <= 0 {
			continue
		}
		if first || c.PriceAmount < lowest {
			lowest = c.PriceAmount
			first = false
		}
	}
	if first || lowest <= 0 {
		for _, c := range candidates {
			out[keyForCandidate(c)] = 0
		}
		return out
	}
	for _, c := range candidates {
		k := keyForCandidate(c)
		if c.PriceAmount <= 0 {
			out[k] = 0
			continue
		}
		out[k] = clampScore((lowest / c.PriceAmount) * ScoreMax)
	}
	return out
}

func normalizeTransitScores(candidates []BidCandidate) map[string]float64 {
	out := make(map[string]float64, len(candidates))
	best := 0.0
	first := true
	for _, c := range candidates {
		if c.TransitHours <= 0 {
			continue
		}
		if first || c.TransitHours < best {
			best = c.TransitHours
			first = false
		}
	}
	if first || best <= 0 {
		for _, c := range candidates {
			out[keyForCandidate(c)] = 0
		}
		return out
	}
	for _, c := range candidates {
		k := keyForCandidate(c)
		if c.TransitHours <= 0 {
			out[k] = 0
			continue
		}
		out[k] = clampScore((best / c.TransitHours) * ScoreMax)
	}
	return out
}

func normalizeCapacityScores(candidates []BidCandidate, requiredVolume float64) map[string]float64 {
	out := make(map[string]float64, len(candidates))
	for _, c := range candidates {
		k := keyForCandidate(c)
		if c.CapacityUnits <= 0 {
			out[k] = 0
			continue
		}
		if requiredVolume > 0 {
			ratio := c.CapacityUnits / requiredVolume
			if ratio >= 1 {
				out[k] = ScoreMax
			} else {
				out[k] = clampScore(ratio * ScoreMax)
			}
			continue
		}
		out[k] = clampScore(math.Min(c.CapacityUnits, ScoreMax))
	}
	return out
}

func keyForCandidate(c BidCandidate) string {
	if c.LotID != "" {
		return c.LotID + ":" + c.CarrierCompanyID
	}
	return c.CarrierCompanyID
}

// ScoreCandidates computes deterministic weighted scores for qualified candidates only.
func ScoreCandidates(template ScoringTemplateSnapshot, candidates []BidCandidate, requiredVolume float64) ([]CarrierScoreResult, error) {
	if err := ValidateScoringTemplate(template.Factors); err != nil {
		return nil, err
	}
	priceScores := normalizePriceScores(candidates)
	transitScores := normalizeTransitScores(candidates)
	capacityScores := normalizeCapacityScores(candidates, requiredVolume)

	weightByFactor := make(map[string]float64, len(template.Factors))
	for _, f := range template.Factors {
		weightByFactor[strings.ToUpper(strings.TrimSpace(f.Factor))] = f.Weight
	}

	results := make([]CarrierScoreResult, 0, len(candidates))
	for _, c := range candidates {
		k := keyForCandidate(c)
		priceScore := priceScores[k]
		slaScore := clampScore(c.SLAScoreInput)
		kpiScore := clampScore(c.CarrierKPIInput)
		capScore := capacityScores[k]
		relScore := clampScore(c.ReliabilityInput)
		transitScore := transitScores[k]

		rawByFactor := map[string]float64{
			FactorPrice:       priceScore,
			FactorSLA:         slaScore,
			FactorCarrierKPI:  kpiScore,
			FactorCapacity:    capScore,
			FactorReliability: relScore,
			FactorTransitTime: transitScore,
		}

		contributions := make([]FactorContribution, 0, len(template.Factors))
		total := 0.0
		for _, fw := range template.Factors {
			factor := strings.ToUpper(strings.TrimSpace(fw.Factor))
			raw := rawByFactor[factor]
			contribution := roundScore(raw * fw.Weight / WeightSumTarget)
			total += contribution
			contributions = append(contributions, FactorContribution{
				Factor:       factor,
				Weight:       fw.Weight,
				RawScore:     raw,
				Contribution: contribution,
			})
		}

		results = append(results, CarrierScoreResult{
			CarrierCompanyID: c.CarrierCompanyID,
			LotID:            c.LotID,
			BidRevisionID:    c.BidRevisionID,
			TotalScore:       roundScore(total),
			Contributions:    contributions,
			PriceScore:       priceScore,
			SLAScore:         slaScore,
			CarrierKPIScore:  kpiScore,
			CapacityScore:    capScore,
			ReliabilityScore: relScore,
			TransitScore:     transitScore,
		})
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].TotalScore != results[j].TotalScore {
			return results[i].TotalScore > results[j].TotalScore
		}
		if results[i].PriceScore != results[j].PriceScore {
			return results[i].PriceScore > results[j].PriceScore
		}
		return results[i].CarrierCompanyID < results[j].CarrierCompanyID
	})
	return results, nil
}

package tender

import (
	"fmt"
	"math"
	"sort"
)

const (
	StrategyWinnerTakesMost  = "WINNER_TAKES_MOST"
	StrategyDualSource       = "DUAL_SOURCE"
	StrategyDiversified      = "DIVERSIFIED"
	StrategyEqualSplit       = "EQUAL_SPLIT"
	StrategyScoreWeighted    = "SCORE_WEIGHTED"
	StrategyCapacityWeighted = "CAPACITY_WEIGHTED"
	StrategyManual           = "MANUAL"
)

type AllocationConstraints struct {
	MinSuppliers        int     `json:"min_suppliers"`
	MaxSuppliers        int     `json:"max_suppliers"`
	MinSharePct         float64 `json:"min_share_pct"`
	MaxSharePct         float64 `json:"max_share_pct"`
	TotalVolume         float64 `json:"total_volume"`
	MaxCarrierSharePct  float64 `json:"max_carrier_share_pct"`
}

type ManualShare struct {
	CarrierCompanyID string  `json:"carrier_company_id"`
	SharePct         float64 `json:"share_pct"`
}

type AllocationConfig struct {
	Strategy     string          `json:"strategy"`
	RankShares   []float64       `json:"rank_shares,omitempty"`
	ManualShares []ManualShare   `json:"manual_shares,omitempty"`
	Constraints  AllocationConstraints `json:"constraints"`
}

type AllocationLine struct {
	CarrierCompanyID      string  `json:"carrier_company_id"`
	LotID                 string  `json:"lot_id,omitempty"`
	Score                 float64 `json:"score"`
	BaseSharePct          float64 `json:"base_share_pct"`
	BalanceAdjustmentPct  float64 `json:"balance_adjustment_pct"`
	ProposedSharePct      float64 `json:"proposed_share_pct"`
	CommittedCapacity     float64 `json:"committed_capacity"`
	ProposedVolume        float64 `json:"proposed_volume"`
}

type AllocationOutcome struct {
	Status  string           `json:"status"`
	Reasons []string         `json:"reasons,omitempty"`
	Lines   []AllocationLine `json:"lines,omitempty"`
	Summary AllocationSummary `json:"summary"`
}

type AllocationSummary struct {
	ExpectedCost       float64 `json:"expected_cost"`
	WeightedScore      float64 `json:"weighted_score"`
	SupplierCount      int     `json:"supplier_count"`
	MaxConcentrationPct float64 `json:"max_concentration_pct"`
	CapacityCoveragePct float64 `json:"capacity_coverage_pct"`
}

const (
	AllocationStatusComputed   = "COMPUTED"
	AllocationStatusInfeasible = "INFEASIBLE"
)

func roundPct(v float64) float64 {
	return math.Round(v*10000) / 10000
}

func roundVol(v float64) float64 {
	return math.Round(v*1000) / 1000
}

func redistributeShares(shares map[string]float64, order []string, maxShare float64) map[string]float64 {
	if maxShare <= 0 {
		return shares
	}
	out := make(map[string]float64, len(shares))
	excess := 0.0
	for _, id := range order {
		s := shares[id]
		if s > maxShare {
			excess += s - maxShare
			out[id] = maxShare
		} else {
			out[id] = s
		}
	}
	for excess > 0.0001 {
		remaining := 0
		for _, id := range order {
			if out[id] < maxShare-0.0001 {
				remaining++
			}
		}
		if remaining == 0 {
			break
		}
		delta := excess / float64(remaining)
		for _, id := range order {
			if out[id] >= maxShare-0.0001 {
				continue
			}
			room := maxShare - out[id]
			add := math.Min(delta, room)
			out[id] += add
			excess -= add
		}
	}
	return out
}

func applyCapacityLimits(lines []AllocationLine, totalVolume float64) []AllocationLine {
	out := make([]AllocationLine, len(lines))
	copy(out, lines)
	for i := range out {
		if out[i].CommittedCapacity > 0 && out[i].ProposedVolume > out[i].CommittedCapacity {
			out[i].ProposedVolume = out[i].CommittedCapacity
		}
	}
	if totalVolume <= 0 {
		return out
	}
	sumVol := 0.0
	for _, l := range out {
		sumVol += l.ProposedVolume
	}
	if sumVol <= totalVolume || sumVol == 0 {
		return out
	}
	scale := totalVolume / sumVol
	for i := range out {
		out[i].ProposedVolume = roundVol(out[i].ProposedVolume * scale)
	}
	return out
}

func buildSummary(lines []AllocationLine, candidates []BidCandidate, prices map[string]float64) AllocationSummary {
	priceByCarrier := make(map[string]float64, len(candidates))
	capByCarrier := make(map[string]float64, len(candidates))
	for _, c := range candidates {
		priceByCarrier[c.CarrierCompanyID] = c.PriceAmount
		capByCarrier[c.CarrierCompanyID] = c.CapacityUnits
	}
	expectedCost := 0.0
	weightedScore := 0.0
	totalShare := 0.0
	maxConc := 0.0
	totalCap := 0.0
	totalVol := 0.0
	for _, l := range lines {
		if l.ProposedSharePct <= 0 {
			continue
		}
		price := prices[l.CarrierCompanyID]
		expectedCost += price * l.ProposedVolume
		weightedScore += l.Score * l.ProposedSharePct / 100
		totalShare += l.ProposedSharePct
		if l.ProposedSharePct > maxConc {
			maxConc = l.ProposedSharePct
		}
		totalCap += capByCarrier[l.CarrierCompanyID]
		totalVol += l.ProposedVolume
	}
	coverage := 0.0
	if totalVol > 0 && totalCap > 0 {
		coverage = math.Min(100, (totalCap/totalVol)*100)
	}
	return AllocationSummary{
		ExpectedCost:        roundScore(expectedCost),
		WeightedScore:       roundScore(weightedScore),
		SupplierCount:       len(lines),
		MaxConcentrationPct: roundPct(maxConc),
		CapacityCoveragePct: roundPct(coverage),
	}
}

func ComputeAllocation(
	cfg AllocationConfig,
	scores []CarrierScoreResult,
	candidates []BidCandidate,
	balanceAdjustments map[string]float64,
) AllocationOutcome {
	constraints := cfg.Constraints
	if len(scores) == 0 {
		return AllocationOutcome{Status: AllocationStatusInfeasible, Reasons: []string{"no_scored_candidates"}}
	}
	if constraints.MinSuppliers > 0 && len(scores) < constraints.MinSuppliers {
		return AllocationOutcome{
			Status:  AllocationStatusInfeasible,
			Reasons: []string{fmt.Sprintf("minimum_suppliers:%d qualified:%d", constraints.MinSuppliers, len(scores))},
		}
	}

	capByCarrier := make(map[string]float64, len(candidates))
	priceByCarrier := make(map[string]float64, len(candidates))
	for _, c := range candidates {
		capByCarrier[c.CarrierCompanyID] = c.CapacityUnits
		priceByCarrier[c.CarrierCompanyID] = c.PriceAmount
	}

	shareByCarrier := make(map[string]float64)
	order := make([]string, 0, len(scores))
	for _, s := range scores {
		order = append(order, s.CarrierCompanyID)
	}

	switch cfg.Strategy {
	case StrategyManual:
		for _, ms := range cfg.ManualShares {
			shareByCarrier[ms.CarrierCompanyID] = ms.SharePct
		}
	case StrategyEqualSplit:
		if len(scores) == 0 {
			break
		}
		each := 100.0 / float64(len(scores))
		for _, s := range scores {
			shareByCarrier[s.CarrierCompanyID] = each
		}
	case StrategyScoreWeighted:
		sumScore := 0.0
		for _, s := range scores {
			sumScore += s.TotalScore
		}
		if sumScore <= 0 {
			return AllocationOutcome{Status: AllocationStatusInfeasible, Reasons: []string{"zero_total_score"}}
		}
		for _, s := range scores {
			shareByCarrier[s.CarrierCompanyID] = (s.TotalScore / sumScore) * 100
		}
	case StrategyCapacityWeighted:
		sumCap := 0.0
		for _, s := range scores {
			sumCap += capByCarrier[s.CarrierCompanyID]
		}
		if sumCap <= 0 {
			return AllocationOutcome{Status: AllocationStatusInfeasible, Reasons: []string{"zero_total_capacity"}}
		}
		for _, s := range scores {
			shareByCarrier[s.CarrierCompanyID] = (capByCarrier[s.CarrierCompanyID] / sumCap) * 100
		}
	default:
		rankShares := cfg.RankShares
		if len(rankShares) == 0 {
			switch cfg.Strategy {
			case StrategyWinnerTakesMost:
				rankShares = []float64{70, 20, 10}
			case StrategyDualSource:
				rankShares = []float64{70, 30}
			case StrategyDiversified:
				rankShares = []float64{50, 30, 20}
			default:
				rankShares = []float64{100}
			}
		}
		if constraints.MinSuppliers > len(rankShares) && cfg.Strategy == StrategyDiversified {
			return AllocationOutcome{
				Status:  AllocationStatusInfeasible,
				Reasons: []string{fmt.Sprintf("rank_shares_require:%d qualified:%d", len(rankShares), len(scores))},
			}
		}
		for i, s := range scores {
			if i >= len(rankShares) {
				break
			}
			shareByCarrier[s.CarrierCompanyID] = rankShares[i]
		}
	}

	sumShares := 0.0
	for _, v := range shareByCarrier {
		sumShares += v
	}
	if math.Abs(sumShares-100) > 0.05 && sumShares > 0 {
		for id, v := range shareByCarrier {
			shareByCarrier[id] = roundPct(v / sumShares * 100)
		}
	}

	if constraints.MaxCarrierSharePct > 0 {
		shareByCarrier = redistributeShares(shareByCarrier, order, constraints.MaxCarrierSharePct)
	}

	lines := make([]AllocationLine, 0, len(shareByCarrier))
	for _, s := range scores {
		base := shareByCarrier[s.CarrierCompanyID]
		if base <= 0 {
			continue
		}
		adj := 0.0
		if balanceAdjustments != nil {
			adj = balanceAdjustments[s.CarrierCompanyID]
		}
		proposed := roundPct(base + adj)
		if proposed < 0 {
			proposed = 0
		}
		if constraints.MinSharePct > 0 && proposed > 0 && proposed < constraints.MinSharePct {
			proposed = constraints.MinSharePct
		}
		if constraints.MaxSharePct > 0 && proposed > constraints.MaxSharePct {
			proposed = constraints.MaxSharePct
		}
		vol := 0.0
		if constraints.TotalVolume > 0 {
			vol = roundVol(constraints.TotalVolume * proposed / 100)
		}
		cap := capByCarrier[s.CarrierCompanyID]
		if cap > 0 && vol > cap {
			vol = cap
		}
		lines = append(lines, AllocationLine{
			CarrierCompanyID:     s.CarrierCompanyID,
			LotID:                s.LotID,
			Score:                s.TotalScore,
			BaseSharePct:         roundPct(base),
			BalanceAdjustmentPct: roundPct(adj),
			ProposedSharePct:     proposed,
			CommittedCapacity:    cap,
			ProposedVolume:       vol,
		})
	}

	if constraints.MaxSuppliers > 0 && len(lines) > constraints.MaxSuppliers {
		sort.SliceStable(lines, func(i, j int) bool {
			return lines[i].ProposedSharePct > lines[j].ProposedSharePct
		})
		lines = lines[:constraints.MaxSuppliers]
	}

	lines = applyCapacityLimits(lines, constraints.TotalVolume)

	finalSum := 0.0
	for _, l := range lines {
		finalSum += l.ProposedSharePct
	}
	if finalSum > 100.01 {
		return AllocationOutcome{
			Status:  AllocationStatusInfeasible,
			Reasons: []string{fmt.Sprintf("share_sum_exceeds_100:%.4f", finalSum)},
		}
	}

	return AllocationOutcome{
		Status:  AllocationStatusComputed,
		Lines:   lines,
		Summary: buildSummary(lines, candidates, priceByCarrier),
	}
}

package tender

import (
	"fmt"
	"math"
)

const (
	BalanceUnderallocated = "UNDERALLOCATED"
	BalanceBalanced       = "BALANCED"
	BalanceOverallocated  = "OVERALLOCATED"
)

type QuotaTarget struct {
	CarrierCompanyID string  `json:"carrier_company_id"`
	TargetSharePct   float64 `json:"target_share_pct"`
}

type QuotaBalancePolicy struct {
	TolerancePct      float64 `json:"tolerance_pct"`
	CarryBalance      bool    `json:"carry_balance"`
	MaxCorrectionPct  float64 `json:"max_correction_pct"`
	PeriodType        string  `json:"period_type"`
}

type QuotaPosition struct {
	CarrierCompanyID string  `json:"carrier_company_id"`
	TargetSharePct   float64 `json:"target_share_pct"`
	ActualSharePct   float64 `json:"actual_share_pct"`
	BalancePP        float64 `json:"balance_pp"`
	Status           string  `json:"status"`
	NextAdjustment   float64 `json:"next_adjustment_pct"`
}

func ValidateQuotaTargets(targets []QuotaTarget) error {
	sum := 0.0
	for _, t := range targets {
		if t.TargetSharePct < 0 {
			return fmt.Errorf("negative target share for carrier %s", t.CarrierCompanyID)
		}
		sum += t.TargetSharePct
	}
	if math.Abs(sum-100) > 0.05 {
		return fmt.Errorf("quota targets must sum to 100, got %.4f", sum)
	}
	return nil
}

func ComputeQuotaPositions(policy QuotaBalancePolicy, targets []QuotaTarget, actualByCarrier map[string]float64) ([]QuotaPosition, error) {
	if err := ValidateQuotaTargets(targets); err != nil {
		return nil, err
	}
	out := make([]QuotaPosition, 0, len(targets))
	for _, t := range targets {
		actual := actualByCarrier[t.CarrierCompanyID]
		balance := roundPct(t.TargetSharePct - actual)
		status := BalanceBalanced
		if balance > policy.TolerancePct {
			status = BalanceUnderallocated
		} else if balance < -policy.TolerancePct {
			status = BalanceOverallocated
		}
		adj := 0.0
		if policy.CarryBalance {
			adj = balance
			if adj > policy.MaxCorrectionPct {
				adj = policy.MaxCorrectionPct
			}
			if adj < -policy.MaxCorrectionPct {
				adj = -policy.MaxCorrectionPct
			}
		}
		out = append(out, QuotaPosition{
			CarrierCompanyID: t.CarrierCompanyID,
			TargetSharePct:   roundPct(t.TargetSharePct),
			ActualSharePct:   roundPct(actual),
			BalancePP:        balance,
			Status:           status,
			NextAdjustment:   roundPct(adj),
		})
	}
	return out, nil
}

// BalanceAdjustmentsForAllocation returns bounded correction map for qualified carriers only.
// Qualification must be enforced separately — underallocated but disqualified carriers get 0 adjustment applied at allocation time.
func BalanceAdjustmentsForAllocation(positions []QuotaPosition, qualified map[string]struct{}) map[string]float64 {
	out := make(map[string]float64, len(positions))
	for _, p := range positions {
		if _, ok := qualified[p.CarrierCompanyID]; !ok {
			continue
		}
		out[p.CarrierCompanyID] = p.NextAdjustment
	}
	return out
}

// ApplyQualificationOverrideBalance zeroes allocation adjustment for disqualified carriers.
func ApplyQualificationOverrideBalance(lines []AllocationLine, qualified map[string]struct{}) []AllocationLine {
	out := make([]AllocationLine, len(lines))
	copy(out, lines)
	for i, l := range out {
		key := l.CarrierCompanyID
		if l.LotID != "" {
			key = l.LotID + ":" + l.CarrierCompanyID
		}
		if _, ok := qualified[key]; !ok {
			out[i].BalanceAdjustmentPct = 0
			out[i].BaseSharePct = 0
			out[i].ProposedSharePct = 0
			out[i].ProposedVolume = 0
		}
	}
	return out
}

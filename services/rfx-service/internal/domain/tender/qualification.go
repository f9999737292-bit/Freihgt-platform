package tender

import (
	"fmt"
	"strings"
)

const (
	QualificationQualified    = "QUALIFIED"
	QualificationDisqualified = "DISQUALIFIED"
)

type QualificationRules struct {
	MinimumSLAScore    float64 `json:"minimum_sla_score"`
	MinimumCapacity    float64 `json:"minimum_capacity"`
	RequireCarrierActive bool  `json:"require_carrier_active"`
}

type QualificationResult struct {
	CarrierCompanyID string   `json:"carrier_company_id"`
	LotID            string   `json:"lot_id,omitempty"`
	Result           string   `json:"result"`
	Reasons          []string `json:"reasons"`
}

func EvaluateQualification(rules QualificationRules, candidates []BidCandidate) []QualificationResult {
	out := make([]QualificationResult, 0, len(candidates))
	for _, c := range candidates {
		reasons := make([]string, 0, 4)
		if rules.RequireCarrierActive && !c.CarrierActive {
			reasons = append(reasons, "carrier_not_active")
		}
		if rules.MinimumSLAScore > 0 && c.SLAScoreInput < rules.MinimumSLAScore {
			reasons = append(reasons, fmt.Sprintf("sla_below_minimum:%.2f", rules.MinimumSLAScore))
		}
		if rules.MinimumCapacity > 0 && c.CapacityUnits < rules.MinimumCapacity {
			reasons = append(reasons, fmt.Sprintf("capacity_below_minimum:%.2f", rules.MinimumCapacity))
		}
		if c.PriceAmount <= 0 {
			reasons = append(reasons, "missing_price")
		}
		result := QualificationQualified
		if len(reasons) > 0 {
			result = QualificationDisqualified
		}
		out = append(out, QualificationResult{
			CarrierCompanyID: c.CarrierCompanyID,
			LotID:            c.LotID,
			Result:           result,
			Reasons:          reasons,
		})
	}
	return out
}

func QualifiedCarrierSet(results []QualificationResult) map[string]struct{} {
	set := make(map[string]struct{})
	for _, r := range results {
		if r.Result != QualificationQualified {
			continue
		}
		key := r.CarrierCompanyID
		if r.LotID != "" {
			key = r.LotID + ":" + r.CarrierCompanyID
		}
		set[key] = struct{}{}
	}
	return set
}

func IsQualified(qualified map[string]struct{}, c BidCandidate) bool {
	key := c.CarrierCompanyID
	if c.LotID != "" {
		key = c.LotID + ":" + c.CarrierCompanyID
	}
	_, ok := qualified[key]
	return ok
}

func FilterQualifiedCandidates(qualified map[string]struct{}, candidates []BidCandidate) []BidCandidate {
	out := make([]BidCandidate, 0, len(candidates))
	for _, c := range candidates {
		if IsQualified(qualified, c) {
			out = append(out, c)
		}
	}
	return out
}

func DisqualificationReasonText(reasons []string) string {
	return strings.Join(reasons, "; ")
}

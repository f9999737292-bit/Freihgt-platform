package domain

import (
	"strings"
)
func ViewScopeForActorKind(actorKind string) CostViewScope {
	switch strings.ToUpper(strings.TrimSpace(actorKind)) {
	case "CARRIER":
		return CostViewScopeCarrierReceivable
	default:
		return CostViewScopeBuyerCost
	}
}

func ApplyViewScope(scope CostViewScope, summary *CostSummary) *CostSummary {
	if summary == nil {
		return nil
	}
	if scope == CostViewScopeBuyerCost {
		return summary
	}
	filtered := *summary
	filtered.AccruedAmount = nil
	filtered.ForecastExposure = nil
	filtered.CurrentVarianceAmount = nil
	filtered.FinalVarianceAmount = nil
	return &filtered
}

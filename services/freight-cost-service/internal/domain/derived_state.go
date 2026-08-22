package domain

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// ComputeDerivedStateFingerprint hashes canonical projection inputs that drive variance/forecast.
// Derived variance amounts are excluded because they are deterministic outputs of these inputs.
func ComputeDerivedStateFingerprint(projection *CostSummaryProjection, proposed ProposedAccessorialInput) string {
	if projection == nil {
		return ""
	}
	parts := []string{
		decimalFingerprint(projection.PlannedAmount),
		decimalFingerprint(projection.AccruedAmount),
		decimalFingerprint(projection.CurrentActualAmount),
		decimalFingerprint(projection.FinalActualAmount),
		projection.CurrencyCode,
		projection.SettlementStatus,
		fmt.Sprintf("%d", projection.OpenDisputeCount),
		string(projection.BillingReconciliationStatus),
		proposed.SourceStatus,
		decimalFingerprint(proposed.TotalExVAT),
	}
	return EvidenceFingerprint(parts...)
}

func decimalFingerprint(value *decimal.Decimal) string {
	if value == nil {
		return "NULL"
	}
	return value.StringFixed(MoneyScale)
}

// ApplyDerivedStateRevision bumps projection_revision only when canonical derived inputs changed.
func ApplyDerivedStateRevision(projection *CostSummaryProjection, proposed ProposedAccessorialInput) bool {
	if projection == nil {
		return false
	}
	newFingerprint := ComputeDerivedStateFingerprint(projection, proposed)
	if newFingerprint == "" {
		return false
	}
	if projection.DerivedStateFingerprint != nil && *projection.DerivedStateFingerprint == newFingerprint {
		return false
	}
	projection.ProjectionRevision++
	fp := newFingerprint
	projection.DerivedStateFingerprint = &fp
	return true
}

func FormatDecimalPtr(value *decimal.Decimal) string {
	if value == nil {
		return ""
	}
	return value.StringFixed(MoneyScale)
}

func JoinFingerprintParts(parts ...string) string {
	return strings.Join(parts, "|")
}

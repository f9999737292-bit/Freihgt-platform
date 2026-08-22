package service

import (
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/client/billing_register"
	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/provider"
)

type canonicalReconciliationContext struct {
	stored      *domain.CostSummaryProjection
	snapshot    *provider.RateSnapshotFact
	settlement  *billing_register.SettlementFact
	billingLink *billing_register.BillingLinkFact
	cursors     []domain.SourceCursor
}

func detectCanonicalReconciliationFindings(input canonicalReconciliationContext) []domain.ReconciliationFinding {
	if input.stored == nil {
		return nil
	}
	var findings []domain.ReconciliationFinding
	projection := input.stored

	if input.snapshot == nil {
		if projection.PlannedAmount != nil {
			findings = append(findings, newCanonicalFinding(projection, domain.FindingMissingPlannedFact, "planned", 0, projection.ProjectionRevision, nil))
		}
	} else if projection.PlannedAmount == nil {
		findings = append(findings, newCanonicalFinding(projection, domain.FindingMissingPlannedFact, "planned", 0, projection.ProjectionRevision, nil))
	} else if !decimalEqualPtr(projection.PlannedAmount, &input.snapshot.TotalAmount) {
		findings = append(findings, newCanonicalFinding(projection, domain.FindingProjectionDrift, "planned_amount", 1, projection.ProjectionRevision, map[string]any{
			"canonical_source": "transport_rate_snapshot",
		}))
	}

	if input.settlement == nil {
		if projection.SettlementLinked && (projection.AccruedAmount != nil || projection.CurrentActualAmount != nil) {
			findings = append(findings, newCanonicalFinding(projection, domain.FindingProjectionDrift, "settlement_missing", 0, projection.ProjectionRevision, nil))
		}
	} else {
		if input.settlement.CurrencyCode != "" && projection.CurrencyCode != "" &&
			!stringsEqualFold(projection.CurrencyCode, input.settlement.CurrencyCode) {
			findings = append(findings, newCanonicalFinding(projection, domain.FindingCurrencyDrift, "settlement_currency", 0, projection.ProjectionRevision, map[string]any{
				"stored": projection.CurrencyCode, "canonical": input.settlement.CurrencyCode,
			}))
		}
		if input.snapshot != nil && input.snapshot.CurrencyCode != "" && projection.CurrencyCode != "" &&
			!stringsEqualFold(projection.CurrencyCode, input.snapshot.CurrencyCode) {
			findings = append(findings, newCanonicalFinding(projection, domain.FindingCurrencyDrift, "snapshot_currency", 0, projection.ProjectionRevision, map[string]any{
				"stored": projection.CurrencyCode, "canonical": input.snapshot.CurrencyCode,
			}))
		}

		expectedAccrual := input.settlement.AccrualAmountExVAT
		if expectedAccrual == nil && projection.AccruedAmount != nil {
			findings = append(findings, newCanonicalFinding(projection, domain.FindingProjectionDrift, "accrual_amount", input.settlement.Version, projection.ProjectionRevision, nil))
		} else if expectedAccrual != nil && (projection.AccruedAmount == nil || !decimalEqualPtr(projection.AccruedAmount, expectedAccrual)) {
			findings = append(findings, newCanonicalFinding(projection, domain.FindingProjectionDrift, "accrual_amount", input.settlement.Version, projection.ProjectionRevision, nil))
		}

		financial := domain.SettlementFinancialInput{
			Status:           input.settlement.Status,
			OpenDisputeCount: input.settlement.OpenDisputeCount,
			TotalWithoutVAT:  input.settlement.TotalWithoutVAT,
		}
		expectedCurrent := domain.CurrentActualAmount(financial)
		if (expectedCurrent == nil) != (projection.CurrentActualAmount == nil) ||
			(expectedCurrent != nil && projection.CurrentActualAmount != nil && !decimalEqualPtr(projection.CurrentActualAmount, expectedCurrent)) {
			findings = append(findings, newCanonicalFinding(projection, domain.FindingProjectionDrift, "current_actual_amount", input.settlement.Version, projection.ProjectionRevision, nil))
		}
		expectedFinal := domain.FinalActualAmount(financial)
		if (expectedFinal == nil) != (projection.FinalActualAmount == nil) ||
			(expectedFinal != nil && projection.FinalActualAmount != nil && !decimalEqualPtr(projection.FinalActualAmount, expectedFinal)) {
			findings = append(findings, newCanonicalFinding(projection, domain.FindingProjectionDrift, "final_actual_amount", input.settlement.Version, projection.ProjectionRevision, nil))
		}

		for _, cursor := range input.cursors {
			if cursor.SourceService == domain.SourceServiceBillingRegister &&
				cursor.SourceType == domain.SourceTypeFreightSettlement &&
				cursor.SourceID == input.settlement.SettlementID &&
				cursor.LastSourceRevision < input.settlement.Version {
				findings = append(findings, newCanonicalFinding(projection, domain.FindingStaleCursor, cursor.EntryKind, input.settlement.Version, cursor.LastSourceRevision, map[string]any{
					"source_type": cursor.SourceType,
					"entry_kind":  cursor.EntryKind,
				}))
			}
		}

		if projection.PlannedAmount == nil {
			findings = append(findings, newCanonicalFinding(projection, domain.FindingMissingPlannedFact, "planned", 0, projection.ProjectionRevision, nil))
		}
		if projection.PlannedAmount != nil && projection.AccruedAmount == nil && input.settlement.AccrualAmountExVAT != nil {
			findings = append(findings, newCanonicalFinding(projection, domain.FindingMissingAccrualFact, "accrual", 0, projection.ProjectionRevision, nil))
		}
		if projection.FinalActualAmount == nil && input.settlement.Status == domain.SettlementStatusReadyForPayment && input.settlement.OpenDisputeCount == 0 {
			findings = append(findings, newCanonicalFinding(projection, domain.FindingMissingFinalActual, "final_actual", 0, projection.ProjectionRevision, nil))
		}
	}

	if input.billingLink != nil {
		if input.billingLink.BillingLinkState == domain.BillingLinkStateLinked && projection.BillingRegisterAmount == nil {
			findings = append(findings, newCanonicalFinding(projection, domain.FindingBillingLinkMismatch, "billing_link", input.billingLink.BillingLinkRevision, projection.ProjectionRevision, nil))
		}
		if input.billingLink.BillingLinkState == domain.BillingLinkStateUnlinked && projection.SettlementLinked {
			findings = append(findings, newCanonicalFinding(projection, domain.FindingOrphanBillingLink, "billing_link_orphan", 0, projection.ProjectionRevision, nil))
		}
	} else if projection.BillingReconciliationStatus == domain.BillingReconciliationMismatch {
		findings = append(findings, newCanonicalFinding(projection, domain.FindingBillingLinkMismatch, "billing_link", 0, projection.ProjectionRevision, map[string]any{
			"status": string(projection.BillingReconciliationStatus),
		}))
	}

	return dedupeFindings(findings)
}

func buildDriverContextFromCanonical(
	settlement *billing_register.SettlementFact,
	snapshot *provider.RateSnapshotFact,
) domain.DriverAttributionContext {
	ctx := domain.DriverAttributionContext{}
	if settlement != nil {
		for _, item := range settlement.ApprovedAccessorials {
			ctx.ApprovedAccessorials = append(ctx.ApprovedAccessorials, domain.ApprovedAccessorialEvidence{
				AccessorialID: item.AccessorialID,
				ChargeCode:    item.ChargeCode,
				Amount:        item.Amount,
			})
		}
		if settlement.BaseFreightAmount != nil {
			base := settlement.BaseFreightAmount.Round(domain.MoneyScale)
			ctx.BaseFreightAmount = &base
		}
	}
	if ctx.BaseFreightAmount == nil && snapshot != nil {
		total := snapshot.TotalAmount.Round(domain.MoneyScale)
		ctx.BaseFreightAmount = &total
	}
	return ctx
}

func newCanonicalFinding(
	projection *domain.CostSummaryProjection,
	kind, referenceKey string,
	expectedRevision, observedRevision int64,
	details map[string]any,
) domain.ReconciliationFinding {
	if details == nil {
		details = map[string]any{}
	}
	return domain.ReconciliationFinding{
		TenantID:              projection.TenantID,
		TransportOrderID:      projection.TransportOrderID,
		FindingID:             domain.DeriveFindingID(projection.TenantID, projection.TransportOrderID, kind, referenceKey),
		FindingKind:           kind,
		Status:                domain.FindingStatusOpen,
		ExpectedRevision:      int64Ptr(expectedRevision),
		ObservedRevision:      int64Ptr(observedRevision),
		CanonicalReferenceKey: referenceKey,
		DetailsJSON:           details,
	}
}

func dedupeFindings(findings []domain.ReconciliationFinding) []domain.ReconciliationFinding {
	if len(findings) == 0 {
		return nil
	}
	seen := make(map[uuid.UUID]struct{}, len(findings))
	out := make([]domain.ReconciliationFinding, 0, len(findings))
	for _, f := range findings {
		if _, ok := seen[f.FindingID]; ok {
			continue
		}
		seen[f.FindingID] = struct{}{}
		out = append(out, f)
	}
	return out
}

func decimalEqualPtr(a *decimal.Decimal, b *decimal.Decimal) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Round(domain.MoneyScale).Equal(b.Round(domain.MoneyScale))
}

func stringsEqualFold(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func int64Ptr(v int64) *int64 {
	return &v
}

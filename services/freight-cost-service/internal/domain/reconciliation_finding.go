package domain

import (
	"strconv"

	"github.com/google/uuid"
)

var NamespaceFreightCostReconciliationFinding = uuid.MustParse("f0a1b2c3-d4e5-6789-abcd-ef0123456792")

const (
	FindingStatusOpen     = "OPEN"
	FindingStatusResolved = "RESOLVED"
	FindingStatusReopened = "REOPENED"

	FindingProjectionDrift      = "PROJECTION_DRIFT"
	FindingMissingPlannedFact   = "MISSING_PLANNED_FACT"
	FindingMissingAccrualFact   = "MISSING_ACCRUAL_FACT"
	FindingMissingFinalActual   = "MISSING_FINAL_ACTUAL"
	FindingStaleCursor          = "STALE_CURSOR"
	FindingBillingLinkMismatch  = "BILLING_LINK_MISMATCH"
	FindingOrphanBillingLink    = "ORPHAN_BILLING_LINK"
	FindingOrphanPaymentLink    = "ORPHAN_PAYMENT_LINK"
	FindingCurrencyDrift        = "CURRENCY_DRIFT"
	FindingDuplicateEconomic    = "DUPLICATE_ECONOMIC_FACT"
)

type ReconciliationFinding struct {
	TenantID              uuid.UUID
	TransportOrderID      uuid.UUID
	FindingID             uuid.UUID
	FindingKind           string
	Status                string
	ExpectedRevision      *int64
	ObservedRevision      *int64
	CanonicalReferenceKey string
	DetailsJSON           map[string]any
}

func DeriveFindingID(
	tenantID, transportOrderID uuid.UUID,
	findingKind, canonicalReferenceKey string,
) uuid.UUID {
	key := tenantID.String() + "|" + transportOrderID.String() + "|" + findingKind + "|" + canonicalReferenceKey
	return uuid.NewSHA1(NamespaceFreightCostReconciliationFinding, []byte(key))
}

func DetectReconciliationFindings(projection *CostSummaryProjection) []ReconciliationFinding {
	if projection == nil {
		return nil
	}
	var findings []ReconciliationFinding
	if projection.PlannedAmount == nil {
		findings = append(findings, newFinding(projection, FindingMissingPlannedFact, "planned", 0, projection.ProjectionRevision, nil))
	}
	if projection.PlannedAmount != nil && projection.AccruedAmount == nil && projection.SettlementLinked {
		findings = append(findings, newFinding(projection, FindingMissingAccrualFact, "accrual", 0, projection.ProjectionRevision, nil))
	}
	if projection.FinalActualAmount == nil && projection.SettlementStatus == SettlementStatusReadyForPayment && projection.OpenDisputeCount == 0 {
		findings = append(findings, newFinding(projection, FindingMissingFinalActual, "final_actual", 0, projection.ProjectionRevision, nil))
	}
	if projection.BillingReconciliationStatus == BillingReconciliationMismatch {
		findings = append(findings, newFinding(projection, FindingBillingLinkMismatch, "billing_link", 0, projection.ProjectionRevision, map[string]any{
			"status": string(projection.BillingReconciliationStatus),
		}))
	}
	if projection.BillingReconciliationStatus == BillingReconciliationUnlinked && projection.SettlementLinked {
		findings = append(findings, newFinding(projection, FindingOrphanBillingLink, "billing_link_orphan", 0, projection.ProjectionRevision, nil))
	}
	return findings
}

func newFinding(
	projection *CostSummaryProjection,
	kind, referenceKey string,
	expectedRevision, observedRevision int64,
	details map[string]any,
) ReconciliationFinding {
	if details == nil {
		details = map[string]any{}
	}
	if observedRevision > 0 {
		details["observed_revision"] = strconv.FormatInt(observedRevision, 10)
	}
	return ReconciliationFinding{
		TenantID:              projection.TenantID,
		TransportOrderID:      projection.TransportOrderID,
		FindingID:             DeriveFindingID(projection.TenantID, projection.TransportOrderID, kind, referenceKey),
		FindingKind:           kind,
		Status:                FindingStatusOpen,
		ExpectedRevision:      int64Ptr(expectedRevision),
		ObservedRevision:      int64Ptr(observedRevision),
		CanonicalReferenceKey: referenceKey,
		DetailsJSON:           details,
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}

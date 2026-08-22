package domain

import (
	"strconv"

	"github.com/google/uuid"
)

var NamespaceFreightCostReconciliationFinding = uuid.MustParse("f0a1b2c3-d4e5-6789-abcd-ef0123456792")

const (
	FindingStatusOpen      = "OPEN"
	FindingStatusResolved  = "RESOLVED"
	FindingStatusReopened  = "REOPENED"

	FindingProjectionDrift     = "PROJECTION_DRIFT"
	FindingMissingPlannedFact  = "MISSING_PLANNED_FACT"
	FindingStaleCursor         = "STALE_CURSOR"
	FindingBillingLinkMismatch = "BILLING_LINK_MISMATCH"
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
	expectedRevision, observedRevision int64,
) uuid.UUID {
	key := tenantID.String() + "|" + transportOrderID.String() + "|" + findingKind + "|" +
		canonicalReferenceKey + "|" + strconv.FormatInt(expectedRevision, 10) + "|" + strconv.FormatInt(observedRevision, 10)
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
	if projection.BillingReconciliationStatus == BillingReconciliationMismatch {
		findings = append(findings, newFinding(projection, FindingBillingLinkMismatch, "billing_link", 0, projection.ProjectionRevision, map[string]any{
			"status": string(projection.BillingReconciliationStatus),
		}))
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
	return ReconciliationFinding{
		TenantID:              projection.TenantID,
		TransportOrderID:      projection.TransportOrderID,
		FindingID:             DeriveFindingID(projection.TenantID, projection.TransportOrderID, kind, referenceKey, expectedRevision, observedRevision),
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

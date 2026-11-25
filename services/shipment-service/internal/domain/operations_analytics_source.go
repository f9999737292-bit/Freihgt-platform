package domain

import (
	"github.com/google/uuid"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

// OperationsAnalyticsCarrierSource is one carrier's source counts.
// Late counts are not stored here; analytics derives them from these facts.
type OperationsAnalyticsCarrierSource struct {
	CarrierCompanyID          uuid.UUID `json:"carrierCompanyId"`
	OnTimePickupDenominator   int64     `json:"onTimePickupDenominator"`
	OnTimePickupNumerator     int64     `json:"onTimePickupNumerator"`
	OnTimeDeliveryDenominator int64     `json:"onTimeDeliveryDenominator"`
	OnTimeDeliveryNumerator   int64     `json:"onTimeDeliveryNumerator"`
}

// OperationsAnalyticsSourceSnapshot is a tenant-scoped source fact aggregate.
// It is not a KPI response: freshness, definition versions, and KPI identifiers
// stay outside shipment-service. sourceObservedAt is omitted because no single
// watermark covers every row in this snapshot.
type OperationsAnalyticsSourceSnapshot struct {
	TenantID                  uuid.UUID                          `json:"tenantId"`
	ShipmentTotal             int64                              `json:"shipmentTotal"`
	OnTimePickupDenominator   int64                              `json:"onTimePickupDenominator"`
	OnTimePickupNumerator     int64                              `json:"onTimePickupNumerator"`
	OnTimeDeliveryDenominator int64                              `json:"onTimeDeliveryDenominator"`
	OnTimeDeliveryNumerator   int64                              `json:"onTimeDeliveryNumerator"`
	ReturnCaseCount           int64                              `json:"returnCaseCount"`
	RedirectCaseCount         int64                              `json:"redirectCaseCount"`
	Carriers                  []OperationsAnalyticsCarrierSource `json:"carriers"`
	// DispositionScope is TENANT only when the case counts were read for the
	// whole tenant. Company-scoped reads leave the counts at zero and mark
	// UNAVAILABLE: a disposition case has no company column of its own.
	DispositionScope string `json:"dispositionScope,omitempty"`
}

const (
	AnalyticsScopeTenant  = "TENANT"
	AnalyticsScopeShipper = "SHIPPER"
	AnalyticsScopeCarrier = "CARRIER"

	DispositionScopeTenant      = "TENANT"
	DispositionScopeUnavailable = "UNAVAILABLE"
)

// AnalyticsReadScope is the server-resolved population for one source read.
// CompanyID is set only for SHIPPER and CARRIER.
type AnalyticsReadScope struct {
	Kind      string
	CompanyID uuid.UUID
}

// BindDispositionScope records whether case counts are a tenant measurement.
// A company-scoped snapshot cannot carry a non-zero case count: that number
// would be an unverified company link.
func BindDispositionScope(snap *OperationsAnalyticsSourceSnapshot, scope AnalyticsReadScope) error {
	if snap == nil {
		return apperrors.Internal("analytics source snapshot is inconsistent", nil)
	}
	switch scope.Kind {
	case AnalyticsScopeTenant:
		if scope.CompanyID != uuid.Nil {
			return apperrors.Forbidden("analytics company scope is not available")
		}
		if snap.DispositionScope == "" {
			snap.DispositionScope = DispositionScopeTenant
		}
		if snap.DispositionScope != DispositionScopeTenant {
			return apperrors.Internal("analytics source snapshot is inconsistent", nil)
		}
		return nil
	case AnalyticsScopeShipper, AnalyticsScopeCarrier:
		if scope.CompanyID == uuid.Nil {
			return apperrors.Forbidden("analytics company scope is not available")
		}
		if snap.ReturnCaseCount != 0 || snap.RedirectCaseCount != 0 {
			return apperrors.Internal("analytics source snapshot is inconsistent", nil)
		}
		if snap.DispositionScope != "" && snap.DispositionScope != DispositionScopeUnavailable {
			return apperrors.Internal("analytics source snapshot is inconsistent", nil)
		}
		snap.DispositionScope = DispositionScopeUnavailable
		return nil
	default:
		return apperrors.Forbidden("analytics company scope is not available")
	}
}

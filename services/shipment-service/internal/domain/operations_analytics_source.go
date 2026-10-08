package domain

import "github.com/google/uuid"

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
}

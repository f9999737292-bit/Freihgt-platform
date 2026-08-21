package provider

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type RateSnapshotFact struct {
	TransportOrderID    uuid.UUID
	TenantID            uuid.UUID
	BuyerCompanyID      uuid.UUID
	CarrierCompanyID    uuid.UUID
	SnapshotID          uuid.UUID
	CurrencyCode        string
	TotalAmount         decimal.Decimal
	PricingSource       string
	PricingModelVersion string
	ResolvedAt          time.Time
}

type TransportOrderPricingProvider interface {
	GetRateSnapshot(ctx context.Context, tenantID, transportOrderID uuid.UUID) (*RateSnapshotFact, error)
}

type SettlementCostFact struct {
	TransportOrderID uuid.UUID
	Status           string
	OpenDisputeCount int
	TotalWithoutVAT  *decimal.Decimal
	CurrencyCode     string
}

type SettlementCostProvider interface {
	GetByTransportOrder(ctx context.Context, tenantID, transportOrderID uuid.UUID) (*SettlementCostFact, error)
}

type BillingRegisterFact struct {
	BillingRegisterID uuid.UUID
	TotalExVAT        decimal.Decimal
	CurrencyCode      string
}

type BillingAmountProvider interface {
	GetRegisterSnapshot(ctx context.Context, tenantID, billingRegisterID uuid.UUID) (*BillingRegisterFact, error)
}

type PaymentObligationFact struct {
	BillingRegisterID uuid.UUID
	PaidAmount        decimal.Decimal
	CurrencyCode      string
}

type PaymentCostProvider interface {
	GetObligationByBillingRegister(ctx context.Context, tenantID, billingRegisterID uuid.UUID) (*PaymentObligationFact, error)
}

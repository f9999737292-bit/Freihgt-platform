package provider

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type SettlementAccessorialLine struct {
	AccessorialID uuid.UUID
	ChargeCode    string
	Amount        decimal.Decimal
	Status        string
	CurrencyCode  string
}

type SettlementAccessorialBatchItem struct {
	TransportOrderID         uuid.UUID
	SettlementID             uuid.UUID
	BuyerCompanyID           uuid.UUID
	CurrencyCode             string
	ApprovedAccessorialTotal decimal.Decimal
	Accessorials             []SettlementAccessorialLine
}

type SettlementAccessorialReader interface {
	BatchGetSettlementsByTransportOrder(
		ctx context.Context,
		tenantID uuid.UUID,
		transportOrderIDs []uuid.UUID,
	) (map[uuid.UUID]SettlementAccessorialBatchItem, error)
}

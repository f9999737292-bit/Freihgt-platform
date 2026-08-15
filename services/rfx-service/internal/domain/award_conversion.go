package domain

import "github.com/google/uuid"

type AwardConversionContext struct {
	RfxEventID       uuid.UUID
	RfxType          string
	FreightReqID     *uuid.UUID
	TransportOrderID *uuid.UUID
	PrimaryCarrierID uuid.UUID
	PrimarySharePct  float64
	ExpectedCost     float64
	CurrencyCode     string
}

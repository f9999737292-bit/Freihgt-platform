package domain

import (
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const (
	AwardTransportOrderSourceSystem = "rfx_award"
)

type RfxAwardTransportOrder struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	RfxEventID       uuid.UUID
	RfxAwardID       uuid.UUID
	RfxResponseID    uuid.UUID
	RfxLotID         uuid.UUID
	RfxLaneID        uuid.UUID
	TransportOrderID uuid.UUID
	CarrierCompanyID uuid.UUID
	BuyerCompanyID   uuid.UUID
	Amount           float64
	CurrencyCode     string
	ConvertedBy      *uuid.UUID
	ConvertedAt      time.Time
	OrderNumber      string
	OrderStatus      string
	Version          int
}

type ConvertAwardTransportOrdersResult struct {
	Created bool
	Items   []RfxAwardTransportOrder
}

type AwardConversionScope struct {
	RfxLotID  uuid.UUID
	RfxLaneID uuid.UUID
	Amount    float64
	Currency  string
	LotNumber string
}

func ValidateAwardConversionEventStatus(status string) error {
	if status != RfxStatusAwarded {
		return apperrors.Validation("rfx event must be AWARDED to convert to transport orders", map[string]any{
			"field":  "status",
			"status": status,
		})
	}
	return nil
}

func ValidateAwardConversionResponse(award *RfxAward, response *RfxResponse) error {
	if response.Status != RfxResponseStatusSubmitted {
		return apperrors.Validation("winning response must be SUBMITTED", map[string]any{"field": "status"})
	}
	if response.ID != award.RfxResponseID {
		return apperrors.Validation("response is not the awarded winner", map[string]any{"field": "response_id"})
	}
	if response.ParticipantCompanyID != award.CarrierCompanyID {
		return apperrors.Validation("response carrier does not match award", map[string]any{"field": "carrier_company_id"})
	}
	return nil
}

func BuildAwardConversionScopes(lotCount int, lots []RfxLot, lines []RfxResponseOfferLine, eventCurrency string) ([]AwardConversionScope, error) {
	if !ResponseOfferComplete(lotCount, lines) {
		return nil, apperrors.Validation("response commercial offer is incomplete", map[string]any{"field": "offer_lines"})
	}
	_, currency, currencyOK := ResponseCommercialSummary(lines, eventCurrency)
	if !currencyOK || currency == "" {
		return nil, apperrors.Validation("response is not commercially comparable", map[string]any{"field": "currency_code"})
	}

	if lotCount == 0 {
		if len(lines) != 1 || lines[0].RfxLotID != uuid.Nil {
			return nil, apperrors.Validation("event-level offer must contain exactly one line", map[string]any{"field": "offer_lines"})
		}
		return []AwardConversionScope{{
			Amount:   lines[0].Amount,
			Currency: currency,
		}}, nil
	}

	scopes := make([]AwardConversionScope, 0, len(lots))
	for _, lot := range lots {
		var matched *RfxResponseOfferLine
		for i := range lines {
			if lines[i].RfxLotID == lot.ID {
				matched = &lines[i]
				break
			}
		}
		if matched == nil {
			return nil, apperrors.Validation("missing offer line for lot", map[string]any{"field": "offer_lines", "rfx_lot_id": lot.ID.String()})
		}
		scopes = append(scopes, AwardConversionScope{
			RfxLotID:  lot.ID,
			Amount:    matched.Amount,
			Currency:  NormalizeCurrencyCode(matched.CurrencyCode),
			LotNumber: lot.LotNumber,
		})
	}
	return scopes, nil
}

package domain

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const (
	PricingSourceRFQAward = "RFQ_AWARD"
	PricingSourceSpotBid  = "SPOT_BID"
)

const (
	ComponentBreakdownAvailable   = "AVAILABLE"
	ComponentBreakdownUnavailable = "UNAVAILABLE"
)

type PricingContextComponent struct {
	ComponentType     string  `json:"component_type"`
	CalculationMethod string  `json:"calculation_method"`
	Amount            *string `json:"amount,omitempty"`
	PercentValue      *string `json:"percent_value,omitempty"`
	UnitCode          *string `json:"unit_code,omitempty"`
}

type NormalizedPricingContext struct {
	TenantID                 uuid.UUID                 `json:"tenant_id"`
	SourceType               string                    `json:"source_type"`
	SourceID                 uuid.UUID                 `json:"source_id"`
	BuyerCompanyID           uuid.UUID                 `json:"buyer_company_id"`
	CarrierCompanyID         uuid.UUID                 `json:"carrier_company_id"`
	OriginLocationID         uuid.UUID                 `json:"origin_location_id"`
	DestinationLocationID    uuid.UUID                 `json:"destination_location_id"`
	EquipmentType            string                    `json:"equipment_type"`
	TransportMode            string                    `json:"transport_mode"`
	CurrencyCode             string                    `json:"currency_code"`
	TotalAmount              string                    `json:"total_amount"`
	BaseAmount               *string                   `json:"base_amount"`
	Components               []PricingContextComponent `json:"components"`
	ComponentBreakdownStatus string                    `json:"component_breakdown_status"`
	SourceStatus             string                    `json:"source_status"`
	RfxEventID               *uuid.UUID                `json:"rfx_event_id,omitempty"`
	RfxLotID                 *uuid.UUID                `json:"rfx_lot_id,omitempty"`
	AwardLinkID              *uuid.UUID                `json:"award_link_id,omitempty"`
	BidID                    *uuid.UUID                `json:"bid_id,omitempty"`
}

func AggregateOnlyPricingContext(
	tenantID, sourceID, buyerID, carrierID, originID, destID uuid.UUID,
	sourceType, equipment, mode, currency, totalAmount, sourceStatus string,
) NormalizedPricingContext {
	return NormalizedPricingContext{
		TenantID:                 tenantID,
		SourceType:               sourceType,
		SourceID:                 sourceID,
		BuyerCompanyID:           buyerID,
		CarrierCompanyID:         carrierID,
		OriginLocationID:         originID,
		DestinationLocationID:    destID,
		EquipmentType:            strings.TrimSpace(equipment),
		TransportMode:            normalizePricingTransportMode(mode),
		CurrencyCode:             NormalizeCurrencyCode(currency),
		TotalAmount:              totalAmount,
		BaseAmount:               nil,
		Components:               []PricingContextComponent{},
		ComponentBreakdownStatus: ComponentBreakdownUnavailable,
		SourceStatus:             sourceStatus,
	}
}

func normalizePricingTransportMode(mode string) string {
	mode = strings.ToUpper(strings.TrimSpace(mode))
	if mode == "" {
		return "ROAD"
	}
	return mode
}

func ValidateNormalizedPricingContext(ctx NormalizedPricingContext) error {
	if ctx.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if ctx.BuyerCompanyID == uuid.Nil || ctx.CarrierCompanyID == uuid.Nil {
		return apperrors.Validation("buyer and carrier company ids are required", nil)
	}
	if ctx.OriginLocationID == uuid.Nil || ctx.DestinationLocationID == uuid.Nil {
		return apperrors.Validation("origin and destination location ids are required", map[string]any{"code": "MISSING_PRICING_CONTEXT"})
	}
	if strings.TrimSpace(ctx.EquipmentType) == "" {
		return apperrors.Validation("equipment_type is required", map[string]any{"code": "MISSING_PRICING_CONTEXT"})
	}
	if strings.TrimSpace(ctx.CurrencyCode) == "" {
		return apperrors.Validation("currency_code is required", map[string]any{"code": "MISSING_PRICING_CONTEXT"})
	}
	if strings.TrimSpace(ctx.TotalAmount) == "" {
		return apperrors.Validation("total_amount is required", map[string]any{"code": "MISSING_PRICING_CONTEXT"})
	}
	if ctx.ComponentBreakdownStatus == ComponentBreakdownUnavailable {
		if ctx.BaseAmount != nil || len(ctx.Components) > 0 {
			return apperrors.Validation("aggregate-only pricing context must not include component breakdown", nil)
		}
	}
	return nil
}

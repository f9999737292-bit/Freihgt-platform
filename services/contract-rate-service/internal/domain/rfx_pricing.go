package domain

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
)

const (
	ReasonSourceNotFound         = "SOURCE_NOT_FOUND"
	ReasonSourceForbidden        = "SOURCE_FORBIDDEN"
	ReasonInvalidPricingSource   = "INVALID_PRICING_SOURCE"
	ReasonPricingSourceMismatch  = "PRICING_SOURCE_MISMATCH"
)

type RFxPricingContext struct {
	TenantID                 uuid.UUID
	SourceType               string
	SourceID                 uuid.UUID
	BuyerCompanyID           uuid.UUID
	CarrierCompanyID         uuid.UUID
	OriginLocationID         uuid.UUID
	DestinationLocationID    uuid.UUID
	EquipmentType            string
	TransportMode            string
	CurrencyCode             string
	TotalAmount              string
	BaseAmount               *string
	ComponentBreakdownStatus string
	Components               []ResolvedComponent
	RfxEventID               *uuid.UUID
	RfxLotID                 *uuid.UUID
	AwardLinkID              *uuid.UUID
	BidID                    *uuid.UUID
	SourceStatus             string
}

type RFxPricingSourceProvider interface {
	GetAwardLinkPricingContext(ctx context.Context, tenantID, linkID uuid.UUID) (RFxPricingContext, error)
	GetAwardScopePricingContext(ctx context.Context, tenantID, eventID uuid.UUID, lotID *uuid.UUID) (RFxPricingContext, error)
	GetAcceptedBidPricingContext(ctx context.Context, tenantID, bidID uuid.UUID) (RFxPricingContext, error)
}

func ValidateRFxContextAgainstRequest(req ResolveRateRequest, ctx RFxPricingContext) error {
	if ctx.TenantID != req.TenantID {
		return apperrors.Forbidden("pricing source tenant mismatch", map[string]any{"code": ReasonSourceForbidden})
	}
	if ctx.BuyerCompanyID != req.BuyerCompanyID {
		return apperrors.Validation("pricing source buyer mismatch", map[string]any{"code": ReasonPricingSourceMismatch, "field": "buyer_company_id"})
	}
	if ctx.CarrierCompanyID != req.CarrierCompanyID && req.CarrierCompanyID != uuid.Nil {
		return apperrors.Validation("pricing source carrier mismatch", map[string]any{"code": ReasonPricingSourceMismatch, "field": "carrier_company_id"})
	}
	if ctx.OriginLocationID != req.OriginLocationID {
		return apperrors.Validation("pricing source origin mismatch", map[string]any{"code": ReasonPricingSourceMismatch, "field": "origin_location_id"})
	}
	if ctx.DestinationLocationID != req.DestinationLocationID {
		return apperrors.Validation("pricing source destination mismatch", map[string]any{"code": ReasonPricingSourceMismatch, "field": "destination_location_id"})
	}
	if strings.TrimSpace(ctx.EquipmentType) != strings.TrimSpace(req.EquipmentType) {
		return apperrors.Validation("pricing source equipment mismatch", map[string]any{"code": ReasonPricingSourceMismatch, "field": "equipment_type"})
	}
	if !strings.EqualFold(strings.TrimSpace(ctx.TransportMode), req.TransportMode) {
		return apperrors.Validation("pricing source transport mode mismatch", map[string]any{"code": ReasonPricingSourceMismatch, "field": "transport_mode"})
	}
	if req.CurrencyCode != nil && !strings.EqualFold(ctx.CurrencyCode, *req.CurrencyCode) {
		return apperrors.Validation("pricing source currency mismatch", map[string]any{"code": ReasonPricingSourceMismatch, "field": "currency_code"})
	}
	return nil
}

func BuildRFxResolveResult(req ResolveRateRequest, ctx RFxPricingContext, now time.Time) (ResolveRateResult, error) {
	if err := ValidateRFxContextAgainstRequest(req, ctx); err != nil {
		return ResolveRateResult{}, err
	}
	total := ctx.TotalAmount
	currency := ctx.CurrencyCode
	result := ResolveRateResult{
		Status:                   ResolveStatusMatched,
		PricingSource:            ctx.SourceType,
		OriginLocationID:         &req.OriginLocationID,
		DestinationLocationID:    &req.DestinationLocationID,
		EquipmentType:            &req.EquipmentType,
		TransportMode:            &req.TransportMode,
		CurrencyCode:             &currency,
		TotalAmount:              &total,
		BaseAmount:               ctx.BaseAmount,
		Components:               ctx.Components,
		ComponentBreakdownStatus: ctx.ComponentBreakdownStatus,
		PricingDate:              req.PricingDate.Format("2006-01-02"),
		ResolvedAt:               now,
		ResolverVersion:          ResolverVersion,
	}
	if ctx.AwardLinkID != nil {
		result.AwardLinkID = ctx.AwardLinkID
	}
	if ctx.BidID != nil {
		result.BidID = ctx.BidID
	}
	if ctx.RfxEventID != nil {
		result.RfxEventID = ctx.RfxEventID
	}
	if ctx.RfxLotID != nil {
		result.RfxLotID = ctx.RfxLotID
	}
	result.CarrierCompanyID = &ctx.CarrierCompanyID
	result.BuyerCompanyID = &ctx.BuyerCompanyID
	if _, err := decimal.NewFromString(total); err != nil {
		return ResolveRateResult{}, apperrors.Validation("invalid pricing source total amount", map[string]any{"code": ReasonInvalidPricingSource})
	}
	return result, nil
}

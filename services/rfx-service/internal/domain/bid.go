package domain

import (
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const (
	BidStatusDraft     = "DRAFT"
	BidStatusSubmitted = "SUBMITTED"
	BidStatusAccepted  = "ACCEPTED"
	BidStatusRejected  = "REJECTED"
)

type Bid struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	FreightRequestID    uuid.UUID
	CarrierCompanyID    uuid.UUID
	BidNumber           string
	Status              string
	TotalAmount         float64
	CurrencyCode        *string
	VATRate             *float64
	VATAmount           float64
	TotalAmountWithVAT  float64
	ValidUntil          *time.Time
	SubmittedAt         *time.Time
	Items               []BidItem
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Version             int
}

type BidItem struct {
	ID               uuid.UUID
	BidID            uuid.UUID
	Description      *string
	BaseAmount       float64
	FuelSurcharge    float64
	TollAmount       float64
	ExtraCharges     float64
	AmountWithoutVAT float64
	VATRate          *float64
	VATAmount        float64
	AmountWithVAT    float64
	Comment          *string
}

type CreateBidItemInput struct {
	Description   *string
	BaseAmount    float64
	FuelSurcharge float64
	TollAmount    float64
	ExtraCharges  float64
	VATRate       *float64
	Comment       *string
}

type CreateBidInput struct {
	TenantID         uuid.UUID
	FreightRequestID uuid.UUID
	CarrierCompanyID uuid.UUID
	BidNumber        string
	CurrencyCode     *string
	VATRate          *float64
	ValidUntil       *time.Time
	Items            []CreateBidItemInput
}

type CalculatedBidItem struct {
	Item             CreateBidItemInput
	AmountWithoutVAT float64
	VATAmount        float64
	AmountWithVAT    float64
}

type CalculatedBidTotals struct {
	TotalAmount        float64
	VATAmount          float64
	TotalAmountWithVAT float64
	Items              []CalculatedBidItem
}

func ValidateCreateBidInput(in CreateBidInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.FreightRequestID == uuid.Nil {
		return apperrors.Validation("freight_request_id is required", map[string]any{"field": "freight_request_id"})
	}
	if in.CarrierCompanyID == uuid.Nil {
		return apperrors.Validation("carrier_company_id is required", map[string]any{"field": "carrier_company_id"})
	}
	if strings.TrimSpace(in.BidNumber) == "" {
		return apperrors.Validation("bid_number is required", map[string]any{"field": "bid_number"})
	}
	if len(in.Items) == 0 {
		return apperrors.Validation("at least one bid item is required", map[string]any{"field": "items"})
	}
	for i, item := range in.Items {
		for _, check := range []struct {
			value float64
			field string
		}{
			{item.BaseAmount, "base_amount"},
			{item.FuelSurcharge, "fuel_surcharge"},
			{item.TollAmount, "toll_amount"},
			{item.ExtraCharges, "extra_charges"},
		} {
			if err := ValidateNonNegativeAmount(check.value, check.field); err != nil {
				return apperrors.Validation(err.Error(), map[string]any{"field": "items", "index": i})
			}
		}
	}
	return ValidateFutureDeadline(in.ValidUntil, "valid_until")
}

func CalculateBidTotals(items []CreateBidItemInput, defaultVATRate *float64) CalculatedBidTotals {
	result := CalculatedBidTotals{Items: make([]CalculatedBidItem, 0, len(items))}
	for _, item := range items {
		amountWithoutVAT := round2(item.BaseAmount + item.FuelSurcharge + item.TollAmount + item.ExtraCharges)
		vatRate := defaultVATRate
		if item.VATRate != nil {
			vatRate = item.VATRate
		}
		vatAmount := 0.0
		if vatRate != nil {
			vatAmount = round2(amountWithoutVAT * (*vatRate) / 100)
		}
		amountWithVAT := round2(amountWithoutVAT + vatAmount)
		result.Items = append(result.Items, CalculatedBidItem{
			Item:             item,
			AmountWithoutVAT: amountWithoutVAT,
			VATAmount:        vatAmount,
			AmountWithVAT:    amountWithVAT,
		})
		result.TotalAmount = round2(result.TotalAmount + amountWithoutVAT)
		result.VATAmount = round2(result.VATAmount + vatAmount)
		result.TotalAmountWithVAT = round2(result.TotalAmountWithVAT + amountWithVAT)
	}
	return result
}

func ValidateSubmitBid(status string) error {
	if status != BidStatusDraft {
		return apperrors.Validation("bid can only be submitted from DRAFT status", map[string]any{"field": "status", "status": status})
	}
	return nil
}

func ValidateAcceptBid(status string) error {
	if status != BidStatusSubmitted {
		return apperrors.Validation("bid can only be accepted from SUBMITTED status", map[string]any{"field": "status", "status": status})
	}
	return nil
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

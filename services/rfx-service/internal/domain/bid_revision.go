package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type BidRevision struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	BidID            uuid.UUID
	RevisionNumber   int
	IsActive         bool
	TotalAmount      *float64
	CurrencyCode     string
	CapacityUnits    *float64
	TransitHours     *float64
	SLAScoreInput    *float64
	CarrierKPIInput  *float64
	ReliabilityInput *float64
	Comment          *string
	SubmittedAt      *time.Time
	IdempotencyKey   *string
	CreatedAt        time.Time
	CarrierCompanyID uuid.UUID
	FreightRequestID uuid.UUID
}

type SubmitBidRevisionInput struct {
	TenantID         uuid.UUID
	BidID            uuid.UUID
	CarrierCompanyID uuid.UUID
	TotalAmount      float64
	CurrencyCode     string
	CapacityUnits    float64
	TransitHours     float64
	SLAScoreInput    float64
	CarrierKPIInput  float64
	ReliabilityInput float64
	Comment          *string
	IdempotencyKey   *string
	SubmittedBy      *uuid.UUID
	Items            []CreateBidItemInput
	VATRate          *float64
}

func ValidateSubmitBidRevisionInput(in SubmitBidRevisionInput) error {
	if in.TenantID == uuid.Nil || in.BidID == uuid.Nil {
		return apperrors.Validation("tenant_id and bid_id are required", map[string]any{})
	}
	if in.CarrierCompanyID == uuid.Nil {
		return apperrors.Validation("carrier_company_id is required", map[string]any{"field": "carrier_company_id"})
	}
	if in.TotalAmount < 0 {
		return apperrors.Validation("total_amount cannot be negative", map[string]any{"field": "total_amount"})
	}
	if strings.TrimSpace(in.CurrencyCode) == "" {
		in.CurrencyCode = "RUB"
	}
	return nil
}

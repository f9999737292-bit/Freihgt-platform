package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

const ActStatusDraft = "DRAFT"

type Act struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	RegisterID         uuid.UUID
	ActNumber          string
	ActDate            time.Time
	SellerCompanyID    uuid.UUID
	BuyerCompanyID     uuid.UUID
	ServiceDescription *string
	TotalAmount        float64
	CurrencyCode       string
	Status             string
	DocumentID         *uuid.UUID
	CreatedAt          time.Time
}

type CreateActInput struct {
	TenantID           uuid.UUID
	ActNumber          string
	ActDate            time.Time
	SellerCompanyID    uuid.UUID
	BuyerCompanyID     uuid.UUID
	ServiceDescription *string
}

func ValidateCreateActInput(in CreateActInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if strings.TrimSpace(in.ActNumber) == "" {
		return apperrors.Validation("act_number is required", map[string]any{"field": "act_number"})
	}
	if in.SellerCompanyID == uuid.Nil {
		return apperrors.Validation("seller_company_id is required", map[string]any{"field": "seller_company_id"})
	}
	if in.BuyerCompanyID == uuid.Nil {
		return apperrors.Validation("buyer_company_id is required", map[string]any{"field": "buyer_company_id"})
	}
	return nil
}

package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

const UPDStatusDraft = "DRAFT"

var allowedUPDFunctionCodes = map[string]struct{}{
	"СЧФ": {}, "СЧФДОП": {}, "ДОП": {},
}

type UPDDocument struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	RegisterID       uuid.UUID
	UPDNumber        string
	UPDDate          time.Time
	SellerCompanyID  uuid.UUID
	BuyerCompanyID   uuid.UUID
	FunctionCode     string
	AmountWithoutVAT float64
	VATRate          *float64
	VATAmount        float64
	AmountWithVAT    float64
	Status           string
	DocumentID       *uuid.UUID
	CreatedAt        time.Time
}

type CreateUPDInput struct {
	TenantID        uuid.UUID
	UPDNumber       string
	UPDDate         time.Time
	SellerCompanyID uuid.UUID
	BuyerCompanyID  uuid.UUID
	FunctionCode    string
}

func ValidateCreateUPDInput(in CreateUPDInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if strings.TrimSpace(in.UPDNumber) == "" {
		return apperrors.Validation("upd_number is required", map[string]any{"field": "upd_number"})
	}
	if strings.TrimSpace(in.FunctionCode) == "" {
		return apperrors.Validation("function_code is required", map[string]any{"field": "function_code"})
	}
	if _, ok := allowedUPDFunctionCodes[strings.TrimSpace(in.FunctionCode)]; !ok {
		return apperrors.Validation("invalid function_code", map[string]any{"field": "function_code", "value": in.FunctionCode})
	}
	if in.SellerCompanyID == uuid.Nil {
		return apperrors.Validation("seller_company_id is required", map[string]any{"field": "seller_company_id"})
	}
	if in.BuyerCompanyID == uuid.Nil {
		return apperrors.Validation("buyer_company_id is required", map[string]any{"field": "buyer_company_id"})
	}
	return nil
}

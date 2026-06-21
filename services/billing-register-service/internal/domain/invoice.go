package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

const InvoiceStatusDraft = "DRAFT"

type Invoice struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	RegisterID      uuid.UUID
	InvoiceNumber   string
	InvoiceDate     time.Time
	SellerCompanyID uuid.UUID
	BuyerCompanyID  uuid.UUID
	TotalAmount     float64
	CurrencyCode    string
	Status          string
	DocumentID      *uuid.UUID
	CreatedAt       time.Time
}

type CreateInvoiceInput struct {
	TenantID        uuid.UUID
	InvoiceNumber   string
	InvoiceDate     time.Time
	SellerCompanyID uuid.UUID
	BuyerCompanyID  uuid.UUID
}

func ValidateCreateInvoiceInput(in CreateInvoiceInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if strings.TrimSpace(in.InvoiceNumber) == "" {
		return apperrors.Validation("invoice_number is required", map[string]any{"field": "invoice_number"})
	}
	if in.SellerCompanyID == uuid.Nil {
		return apperrors.Validation("seller_company_id is required", map[string]any{"field": "seller_company_id"})
	}
	if in.BuyerCompanyID == uuid.Nil {
		return apperrors.Validation("buyer_company_id is required", map[string]any{"field": "buyer_company_id"})
	}
	return nil
}

package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

const VATInvoiceStatusDraft = "DRAFT"

type VATInvoice struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	RegisterID       uuid.UUID
	VATInvoiceNumber string
	VATInvoiceDate   time.Time
	SellerCompanyID  uuid.UUID
	BuyerCompanyID   uuid.UUID
	AmountWithoutVAT float64
	VATRate          *float64
	VATAmount        float64
	AmountWithVAT    float64
	Status           string
	DocumentID       *uuid.UUID
	CreatedAt        time.Time
}

type CreateVATInvoiceInput struct {
	TenantID         uuid.UUID
	VATInvoiceNumber string
	VATInvoiceDate   time.Time
	SellerCompanyID  uuid.UUID
	BuyerCompanyID   uuid.UUID
}

func ValidateCreateVATInvoiceInput(in CreateVATInvoiceInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if strings.TrimSpace(in.VATInvoiceNumber) == "" {
		return apperrors.Validation("vat_invoice_number is required", map[string]any{"field": "vat_invoice_number"})
	}
	if in.SellerCompanyID == uuid.Nil {
		return apperrors.Validation("seller_company_id is required", map[string]any{"field": "seller_company_id"})
	}
	if in.BuyerCompanyID == uuid.Nil {
		return apperrors.Validation("buyer_company_id is required", map[string]any{"field": "buyer_company_id"})
	}
	return nil
}

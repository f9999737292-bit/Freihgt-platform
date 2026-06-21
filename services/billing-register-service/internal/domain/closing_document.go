package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

const (
	ClosingPackageTypeInvoiceOnly        = "INVOICE_ONLY"
	ClosingPackageTypeActPlusVATInvoice  = "ACT_PLUS_VAT_INVOICE"
	ClosingPackageTypeUPD                = "UPD"
	ClosingPackageTypeCustom             = "CUSTOM"
	ClosingPackageStatusDraft            = "DRAFT"
)

var allowedPackageTypes = map[string]struct{}{
	ClosingPackageTypeInvoiceOnly:       {},
	ClosingPackageTypeActPlusVATInvoice: {},
	ClosingPackageTypeUPD:               {},
	ClosingPackageTypeCustom:            {},
}

type ClosingDocumentPackage struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	RegisterID    uuid.UUID
	PackageNumber string
	PackageType   string
	Status        string
	CreatedAt     time.Time
}

type CreateClosingDocumentPackageInput struct {
	TenantID      uuid.UUID
	PackageNumber string
	PackageType   string
}

func ValidateCreateClosingDocumentPackageInput(in CreateClosingDocumentPackageInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if strings.TrimSpace(in.PackageNumber) == "" {
		return apperrors.Validation("package_number is required", map[string]any{"field": "package_number"})
	}
	if _, ok := allowedPackageTypes[strings.TrimSpace(in.PackageType)]; !ok {
		return apperrors.Validation("invalid package_type", map[string]any{"field": "package_type", "value": in.PackageType})
	}
	return nil
}

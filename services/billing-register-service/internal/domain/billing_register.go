package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

const (
	RegisterStatusDraft                  = "DRAFT"
	RegisterStatusCalculated             = "CALCULATED"
	RegisterStatusUnderReview              = "UNDER_REVIEW"
	RegisterStatusApproved               = "APPROVED"
	RegisterStatusClosingDocumentsCreated = "CLOSING_DOCUMENTS_CREATED"
	RegisterStatusSentToEDO              = "SENT_TO_EDO"
	RegisterStatusSignedByCounterparty   = "SIGNED_BY_COUNTERPARTY"
	RegisterStatusPaid                   = "PAID"
	RegisterStatusClosed                 = "CLOSED"
	RegisterStatusCancelled              = "CANCELLED"

	ShipmentStatusReadyForBilling    = "READY_FOR_BILLING"
	ShipmentStatusDocumentsCompleted = "DOCUMENTS_COMPLETED"
)

type BillingRegister struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	RegisterNumber      string
	CustomerCompanyID   uuid.UUID
	ContractorCompanyID uuid.UUID
	ContractID          *uuid.UUID
	PeriodFrom          time.Time
	PeriodTo            time.Time
	CurrencyCode        string
	VATRate             *float64
	Status              string
	TotalWithoutVAT     float64
	VATAmount           float64
	TotalWithVAT        float64
	CreatedAt           time.Time
	ApprovedAt          *time.Time
	ApprovedBy          *uuid.UUID
	UpdatedAt           time.Time
	Version             int
}

type CreateBillingRegisterInput struct {
	TenantID            uuid.UUID
	RegisterNumber      string
	CustomerCompanyID   uuid.UUID
	ContractorCompanyID uuid.UUID
	ContractID          *uuid.UUID
	PeriodFrom          time.Time
	PeriodTo            time.Time
	CurrencyCode        string
	VATRate             *float64
}

type ListBillingRegistersFilter struct {
	TenantID            uuid.UUID
	CustomerCompanyID   *uuid.UUID
	ContractorCompanyID *uuid.UUID
	Status              *string
	PeriodFrom          *time.Time
	PeriodTo            *time.Time
	Limit               int
	Offset              int
}

type TenantActionInput struct {
	TenantID uuid.UUID
}

type ApproveRegisterInput struct {
	TenantID   uuid.UUID
	ApprovedBy uuid.UUID
}

func ValidateCreateBillingRegisterInput(in CreateBillingRegisterInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if strings.TrimSpace(in.RegisterNumber) == "" {
		return apperrors.Validation("register_number is required", map[string]any{"field": "register_number"})
	}
	if in.CustomerCompanyID == uuid.Nil {
		return apperrors.Validation("customer_company_id is required", map[string]any{"field": "customer_company_id"})
	}
	if in.ContractorCompanyID == uuid.Nil {
		return apperrors.Validation("contractor_company_id is required", map[string]any{"field": "contractor_company_id"})
	}
	if in.PeriodTo.Before(in.PeriodFrom) {
		return apperrors.Validation("period_to cannot be earlier than period_from", map[string]any{"field": "period_to"})
	}
	if in.VATRate != nil {
		if err := ValidateNonNegativeAmount(*in.VATRate, "vat_rate"); err != nil {
			return err
		}
	}
	return nil
}

func ValidateListBillingRegistersFilter(f ListBillingRegistersFilter) error {
	if f.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if f.Limit == 0 {
		return apperrors.Validation("limit must be greater than 0", map[string]any{"field": "limit"})
	}
	return ValidateListPagination(f.Limit, f.Offset)
}

func ValidateTenantActionInput(in TenantActionInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	return nil
}

func ValidateApproveRegisterInput(in ApproveRegisterInput) error {
	if err := ValidateTenantActionInput(TenantActionInput{TenantID: in.TenantID}); err != nil {
		return err
	}
	if in.ApprovedBy == uuid.Nil {
		return apperrors.Validation("approved_by is required", map[string]any{"field": "approved_by"})
	}
	return nil
}

func ValidateAddItemRegisterStatus(status string) error {
	if status != RegisterStatusDraft && status != RegisterStatusCalculated {
		return apperrors.Validation("items can only be added to DRAFT or CALCULATED register", map[string]any{
			"field": "status", "status": status,
		})
	}
	return nil
}

func ValidateDeleteItemRegisterStatus(status string) error {
	return ValidateAddItemRegisterStatus(status)
}

func ValidateCalculateRegisterStatus(status string) error {
	if status != RegisterStatusDraft && status != RegisterStatusCalculated {
		return apperrors.Validation("register can only be calculated in DRAFT or CALCULATED status", map[string]any{
			"field": "status", "status": status,
		})
	}
	return nil
}

func ValidateApproveRegisterStatus(status string) error {
	if status != RegisterStatusCalculated {
		return apperrors.Validation("register can only be approved from CALCULATED status", map[string]any{
			"field": "status", "status": status,
		})
	}
	return nil
}

func ValidateApproveRegisterTotals(totalWithVAT float64) error {
	if totalWithVAT <= 0 {
		return apperrors.Validation("total_with_vat must be greater than 0", map[string]any{"field": "total_with_vat"})
	}
	return nil
}

func ValidateClosingDocumentRegisterStatus(status string) error {
	if status != RegisterStatusApproved {
		return apperrors.Validation("closing documents can only be created for APPROVED register", map[string]any{
			"field": "status", "status": status,
		})
	}
	return nil
}

func ValidateCreateClosingDocumentRegisterStatus(status string) error {
	if status != RegisterStatusApproved && status != RegisterStatusClosingDocumentsCreated {
		return apperrors.Validation("documents can only be created for APPROVED or CLOSING_DOCUMENTS_CREATED register", map[string]any{
			"field": "status", "status": status,
		})
	}
	return nil
}

func ValidateMarkSentToEDOStatus(status string) error {
	if status != RegisterStatusClosingDocumentsCreated {
		return apperrors.Validation("register can only be marked SENT_TO_EDO from CLOSING_DOCUMENTS_CREATED status", map[string]any{
			"field": "status", "status": status,
		})
	}
	return nil
}

func ValidateMarkSignedStatus(status string) error {
	if status != RegisterStatusSentToEDO && status != RegisterStatusClosingDocumentsCreated {
		return apperrors.Validation("register can only be marked SIGNED_BY_COUNTERPARTY from SENT_TO_EDO or CLOSING_DOCUMENTS_CREATED status", map[string]any{
			"field": "status", "status": status,
		})
	}
	return nil
}

func ValidateMarkPaidStatus(status string) error {
	if status != RegisterStatusSignedByCounterparty {
		return apperrors.Validation("register can only be marked PAID from SIGNED_BY_COUNTERPARTY status", map[string]any{
			"field": "status", "status": status,
		})
	}
	return nil
}

func ValidateCloseRegisterStatus(status string) error {
	if status != RegisterStatusPaid {
		return apperrors.Validation("register can only be closed from PAID status", map[string]any{
			"field": "status", "status": status,
		})
	}
	return nil
}

func ValidateShipmentForBilling(status string) error {
	if status != ShipmentStatusReadyForBilling && status != ShipmentStatusDocumentsCompleted {
		return apperrors.Validation("shipment must be in READY_FOR_BILLING or DOCUMENTS_COMPLETED status", map[string]any{
			"field": "shipment_id", "status": status,
		})
	}
	return nil
}

func NormalizeCurrencyCode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "RUB"
	}
	return strings.ToUpper(value)
}

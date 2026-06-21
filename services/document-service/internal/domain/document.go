package domain

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

const (
	DocumentStatusDraft             = "DRAFT"
	DocumentStatusReadyForSigning   = "READY_FOR_SIGNING"
	DocumentStatusSigningInProgress = "SIGNING_IN_PROGRESS"
	DocumentStatusSigned            = "SIGNED"
	DocumentStatusSentToOperator    = "SENT_TO_OPERATOR"
	DocumentStatusAccepted          = "ACCEPTED"
	DocumentStatusRejected          = "REJECTED"
	DocumentStatusArchived          = "ARCHIVED"
	DocumentStatusCancelled         = "CANCELLED"

	RelatedEntityTypeShipment        = "SHIPMENT"
	RelatedEntityTypeTransportOrder  = "TRANSPORT_ORDER"
	RelatedEntityTypeBillingRegister = "BILLING_REGISTER"
	RelatedEntityTypeInvoice         = "INVOICE"
	RelatedEntityTypeCompany         = "COMPANY"
)

var allowedDocumentTypes = map[string]struct{}{
	"ETRN": {}, "EPD": {}, "WAYBILL": {}, "POD": {}, "DISCREPANCY_ACT": {},
	"CLAIM": {}, "INVOICE": {}, "VAT_INVOICE": {}, "ACT": {}, "UPD": {}, "ECMR": {},
}

var allowedRelatedEntityTypes = map[string]struct{}{
	RelatedEntityTypeShipment:        {},
	RelatedEntityTypeTransportOrder:  {},
	RelatedEntityTypeBillingRegister: {},
	RelatedEntityTypeInvoice:         {},
	RelatedEntityTypeCompany:         {},
}

var allowedLegalLanguages = map[string]struct{}{
	"ru-RU": {}, "en-US": {}, "zh-CN": {},
}

type Document struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	DocumentNumber    string
	DocumentType      string
	DocumentStatus    string
	OwnerCompanyID    uuid.UUID
	RelatedEntityType *string
	RelatedEntityID   *uuid.UUID
	LegalLanguage     string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Version           int
}

type CreateDocumentInput struct {
	TenantID          uuid.UUID
	DocumentNumber    string
	DocumentType      string
	OwnerCompanyID    uuid.UUID
	RelatedEntityType *string
	RelatedEntityID   *uuid.UUID
	LegalLanguage     string
	PayloadJSON       json.RawMessage
}

type ListDocumentsFilter struct {
	TenantID          uuid.UUID
	DocumentType      *string
	DocumentStatus    *string
	RelatedEntityType *string
	RelatedEntityID   *uuid.UUID
	Limit             int
	Offset            int
}

type ReadyForSigningInput struct {
	TenantID uuid.UUID
}

type CancelDocumentInput struct {
	TenantID uuid.UUID
	Reason   string
}

type ArchiveDocumentInput struct {
	TenantID uuid.UUID
}

func ValidateCreateDocumentInput(in CreateDocumentInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if strings.TrimSpace(in.DocumentNumber) == "" {
		return apperrors.Validation("document_number is required", map[string]any{"field": "document_number"})
	}
	if err := ValidateDocumentType(in.DocumentType); err != nil {
		return err
	}
	if in.OwnerCompanyID == uuid.Nil {
		return apperrors.Validation("owner_company_id is required", map[string]any{"field": "owner_company_id"})
	}
	if strings.TrimSpace(in.LegalLanguage) == "" {
		return apperrors.Validation("legal_language is required", map[string]any{"field": "legal_language"})
	}
	if err := ValidateLegalLanguage(in.LegalLanguage); err != nil {
		return err
	}
	if in.RelatedEntityType != nil {
		if err := ValidateRelatedEntityType(*in.RelatedEntityType); err != nil {
			return err
		}
	}
	return nil
}

func ValidateDocumentType(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return apperrors.Validation("document_type is required", map[string]any{"field": "document_type"})
	}
	if _, ok := allowedDocumentTypes[value]; !ok {
		return apperrors.Validation("invalid document_type", map[string]any{"field": "document_type", "value": value})
	}
	return nil
}

func ValidateRelatedEntityType(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return apperrors.Validation("invalid related_entity_type", map[string]any{"field": "related_entity_type"})
	}
	if _, ok := allowedRelatedEntityTypes[value]; !ok {
		return apperrors.Validation("invalid related_entity_type", map[string]any{"field": "related_entity_type", "value": value})
	}
	return nil
}

func ValidateLegalLanguage(value string) error {
	value = strings.TrimSpace(value)
	if _, ok := allowedLegalLanguages[value]; !ok {
		return apperrors.Validation("legal_language must be ru-RU, en-US or zh-CN", map[string]any{"field": "legal_language", "value": value})
	}
	return nil
}

func ValidateListDocumentsFilter(f ListDocumentsFilter) error {
	if f.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if f.Limit == 0 {
		return apperrors.Validation("limit must be greater than 0", map[string]any{"field": "limit"})
	}
	return ValidateListPagination(f.Limit, f.Offset)
}

func ValidateCreateVersionStatus(status string) error {
	if status == DocumentStatusSigned || status == DocumentStatusArchived {
		return apperrors.Validation("cannot create new version for SIGNED or ARCHIVED document", map[string]any{
			"field":  "document_status",
			"status": status,
		})
	}
	if status != DocumentStatusDraft && status != DocumentStatusRejected {
		return apperrors.Validation("new version can only be created for DRAFT or REJECTED document", map[string]any{
			"field":  "document_status",
			"status": status,
		})
	}
	return nil
}

func ValidateReadyForSigningStatus(status string) error {
	if status != DocumentStatusDraft {
		return apperrors.Validation("document can only be moved to READY_FOR_SIGNING from DRAFT status", map[string]any{
			"field":  "document_status",
			"status": status,
		})
	}
	return nil
}

func ValidateCreateSigningSessionStatus(status string) error {
	if status != DocumentStatusReadyForSigning {
		return apperrors.Validation("signing session can only be created for READY_FOR_SIGNING document", map[string]any{
			"field":  "document_status",
			"status": status,
		})
	}
	return nil
}

func ValidateCancelDocumentStatus(status string) error {
	if status == DocumentStatusSigned || status == DocumentStatusArchived {
		return apperrors.Validation("cannot cancel SIGNED or ARCHIVED document", map[string]any{
			"field":  "document_status",
			"status": status,
		})
	}
	return nil
}

func ValidateArchiveDocumentStatus(status string) error {
	if status != DocumentStatusSigned && status != DocumentStatusAccepted {
		return apperrors.Validation("document can only be archived from SIGNED or ACCEPTED status", map[string]any{
			"field":  "document_status",
			"status": status,
		})
	}
	return nil
}

func ValidateSigningDocumentStatus(status string) error {
	if status == DocumentStatusCancelled || status == DocumentStatusArchived {
		return apperrors.Validation("cannot sign CANCELLED or ARCHIVED document", map[string]any{
			"field":  "document_status",
			"status": status,
		})
	}
	return nil
}

func NormalizeLegalLanguage(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "ru-RU"
	}
	return value
}

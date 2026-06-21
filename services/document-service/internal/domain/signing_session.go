package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

const (
	SigningSessionStatusCreated    = "CREATED"
	SigningSessionStatusInProgress = "IN_PROGRESS"
	SigningSessionStatusCompleted  = "COMPLETED"
	SigningSessionStatusExpired    = "EXPIRED"
	SigningSessionStatusCancelled  = "CANCELLED"
	SigningSessionStatusFailed     = "FAILED"

	SignatureTypeSimpleElectronic     = "SIMPLE_ELECTRONIC"
	SignatureTypeEnhancedUnqualified  = "ENHANCED_UNQUALIFIED"
	SignatureTypeEnhancedQualified    = "ENHANCED_QUALIFIED"

	SignatureVerificationValid = "VALID"
)

var allowedSignatureTypes = map[string]struct{}{
	SignatureTypeSimpleElectronic:    {},
	SignatureTypeEnhancedUnqualified: {},
	SignatureTypeEnhancedQualified:   {},
}

type SigningSession struct {
	ID                    uuid.UUID
	TenantID              uuid.UUID
	DocumentID            uuid.UUID
	Status                string
	RequiredSignersCount  int
	CompletedSignersCount int
	CreatedAt             time.Time
	ExpiresAt             *time.Time
}

type Signature struct {
	ID                     uuid.UUID
	TenantID               uuid.UUID
	SigningSessionID       uuid.UUID
	DocumentID             uuid.UUID
	SignerUserID           *uuid.UUID
	SignerCompanyID        *uuid.UUID
	SignatureType          string
	SignaturePayloadPath   *string
	CertificateFingerprint *string
	SignedAt               *time.Time
	VerificationStatus     string
	CreatedAt              time.Time
}

type CreateSigningSessionInput struct {
	TenantID             uuid.UUID
	RequiredSignersCount int
	ExpiresAt            *time.Time
}

type AddSignatureInput struct {
	TenantID               uuid.UUID
	SignerUserID           uuid.UUID
	SignerCompanyID        uuid.UUID
	SignatureType          string
	SignaturePayloadPath   *string
	CertificateFingerprint *string
}

func ValidateCreateSigningSessionInput(in CreateSigningSessionInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.RequiredSignersCount <= 0 {
		return apperrors.Validation("required_signers_count must be greater than 0", map[string]any{"field": "required_signers_count"})
	}
	return nil
}

func ValidateAddSignatureInput(in AddSignatureInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.SignerUserID == uuid.Nil {
		return apperrors.Validation("signer_user_id is required", map[string]any{"field": "signer_user_id"})
	}
	if in.SignerCompanyID == uuid.Nil {
		return apperrors.Validation("signer_company_id is required", map[string]any{"field": "signer_company_id"})
	}
	return ValidateSignatureType(in.SignatureType)
}

func ValidateSignatureType(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return apperrors.Validation("signature_type is required", map[string]any{"field": "signature_type"})
	}
	if _, ok := allowedSignatureTypes[value]; !ok {
		return apperrors.Validation("invalid signature_type", map[string]any{"field": "signature_type", "value": value})
	}
	return nil
}

func ValidateSigningSessionForSignature(status string) error {
	if status != SigningSessionStatusCreated && status != SigningSessionStatusInProgress {
		return apperrors.Validation("signatures can only be added to CREATED or IN_PROGRESS signing session", map[string]any{
			"field":  "status",
			"status": status,
		})
	}
	return nil
}

func ResolveSigningAfterSignature(required, completed int) (sessionStatus, documentStatus string) {
	if completed >= required {
		return SigningSessionStatusCompleted, DocumentStatusSigned
	}
	return SigningSessionStatusInProgress, DocumentStatusSigningInProgress
}

package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

const (
	AccessorialStatusProposed = "PROPOSED"
	AccessorialStatusApproved = "APPROVED"
	AccessorialStatusRejected = "REJECTED"
	AccessorialStatusDisputed = "DISPUTED"
)

type SettlementAccessorial struct {
	ID                    uuid.UUID
	TenantID              uuid.UUID
	SettlementID          uuid.UUID
	ChargeCode            string
	Description           *string
	Amount                float64
	CurrencyCode          string
	Status                string
	SubmittedBy           uuid.UUID
	SubmittedByCompanyID  uuid.UUID
	EvidenceDocumentID    *uuid.UUID
	EvidenceType          *string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type ProposeAccessorialInput struct {
	SettlementActorInput
	ChargeCode         string
	Description        *string
	Amount             float64
	EvidenceDocumentID *uuid.UUID
	EvidenceType       *string
}

func ValidateProposeAccessorialInput(in ProposeAccessorialInput) error {
	if err := ValidateSettlementActor(in.SettlementActorInput); err != nil {
		return err
	}
	if strings.TrimSpace(in.ChargeCode) == "" {
		return apperrors.Validation("charge_code is required", map[string]any{"field": "charge_code"})
	}
	return ValidateNonNegativeAmount(in.Amount, "amount")
}

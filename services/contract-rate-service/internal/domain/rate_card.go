package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
)

type RateCard struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	ContractID  uuid.UUID
	Name        string
	Description *string
	CreatedAt   time.Time
	CreatedBy   *uuid.UUID
	UpdatedAt   time.Time
	UpdatedBy   *uuid.UUID
	Version     int
}

type RateCardVersion struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	RateCardID         uuid.UUID
	VersionNumber      int
	ValidFrom          time.Time
	ValidTo            *time.Time
	Status             string
	SupersedesVersionID *uuid.UUID
	CreatedAt          time.Time
	CreatedBy          *uuid.UUID
	ActivatedAt        *time.Time
	ActivatedBy        *uuid.UUID
	Version            int
}

type CreateRateCardInput struct {
	TenantID    uuid.UUID
	ContractID  uuid.UUID
	Name        string
	Description *string
	Actor       ActorInput
}

type UpdateRateCardInput struct {
	Name        *string
	Description *string
	Actor       ActorInput
}

type CreateRateVersionInput struct {
	TenantID   uuid.UUID
	RateCardID uuid.UUID
	ValidFrom  time.Time
	ValidTo    *time.Time
	Actor      ActorInput
}

type UpdateRateVersionInput struct {
	ValidFrom *time.Time
	ValidTo   *time.Time
	Actor     ActorInput
}

func ValidateCreateRateCardInput(in CreateRateCardInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.ContractID == uuid.Nil {
		return apperrors.Validation("contract_id is required", map[string]any{"field": "contract_id"})
	}
	if strings.TrimSpace(in.Name) == "" {
		return apperrors.Validation("name is required", map[string]any{"field": "name"})
	}
	return nil
}

func ValidateCreateRateVersionInput(in CreateRateVersionInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.RateCardID == uuid.Nil {
		return apperrors.Validation("rate_card_id is required", map[string]any{"field": "rate_card_id"})
	}
	if in.ValidFrom.IsZero() {
		return apperrors.Validation("valid_from is required", map[string]any{"field": "valid_from"})
	}
	return ValidateDateRange(in.ValidFrom, in.ValidTo)
}

func ValidateUpdateRateVersionInput(current *RateCardVersion, patch UpdateRateVersionInput) error {
	if current == nil {
		return apperrors.NotFound("rate version not found")
	}
	if current.Status != RateVersionStatusDraft {
		return apperrors.Validation("only DRAFT rate versions can be updated", map[string]any{"status": current.Status})
	}
	validFrom := current.ValidFrom
	if patch.ValidFrom != nil {
		validFrom = *patch.ValidFrom
	}
	validTo := current.ValidTo
	if patch.ValidTo != nil {
		validTo = patch.ValidTo
	}
	return ValidateDateRange(validFrom, validTo)
}

func ValidateDiscardRateVersion(current *RateCardVersion) error {
	if current == nil {
		return apperrors.NotFound("rate version not found")
	}
	if current.Status != RateVersionStatusDraft {
		return apperrors.Validation("only DRAFT rate versions can be discarded", map[string]any{"status": current.Status})
	}
	return nil
}

func ValidateRateCardParentContract(contract *TransportContract) error {
	if contract == nil {
		return apperrors.NotFound("contract not found")
	}
	if IsTerminalContractStatus(contract.Status) {
		return apperrors.Validation("terminal contract cannot own new rate cards", map[string]any{"status": contract.Status})
	}
	return nil
}

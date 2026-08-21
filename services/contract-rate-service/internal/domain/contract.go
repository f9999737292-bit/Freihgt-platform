package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
)

type TransportContract struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	BuyerCompanyID    uuid.UUID
	CarrierCompanyID  uuid.UUID
	ContractNumber    string
	ExternalReference *string
	Name              string
	Description       *string
	Status            string
	ValidFrom         time.Time
	ValidTo           *time.Time
	CurrencyCode      string
	CreatedAt         time.Time
	CreatedBy         *uuid.UUID
	UpdatedAt         time.Time
	UpdatedBy         *uuid.UUID
	ActivatedAt       *time.Time
	ActivatedBy       *uuid.UUID
	TerminatedAt      *time.Time
	TerminatedBy      *uuid.UUID
	TerminationReason *string
	Version           int
}

type CreateContractInput struct {
	TenantID          uuid.UUID
	BuyerCompanyID    uuid.UUID
	CarrierCompanyID  uuid.UUID
	ContractNumber    string
	ExternalReference *string
	Name              string
	Description       *string
	ValidFrom         time.Time
	ValidTo           *time.Time
	CurrencyCode      string
	Actor             ActorInput
}

type UpdateContractInput struct {
	ExternalReference *string
	Name              *string
	Description       *string
	ValidTo           NullableDatePatch
	Actor             ActorInput
}

type PatchContractMetadataInput struct {
	Description       *string
	ExternalReference *string
	Actor             ActorInput
}

func ValidateCreateContractInput(in CreateContractInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.BuyerCompanyID == uuid.Nil {
		return apperrors.Validation("buyer_company_id is required", map[string]any{"field": "buyer_company_id"})
	}
	if in.CarrierCompanyID == uuid.Nil {
		return apperrors.Validation("carrier_company_id is required", map[string]any{"field": "carrier_company_id"})
	}
	if in.BuyerCompanyID == in.CarrierCompanyID {
		return apperrors.Validation("buyer and carrier must differ", map[string]any{"field": "carrier_company_id"})
	}
	if strings.TrimSpace(in.ContractNumber) == "" {
		return apperrors.Validation("contract_number is required", map[string]any{"field": "contract_number"})
	}
	if strings.TrimSpace(in.Name) == "" {
		return apperrors.Validation("name is required", map[string]any{"field": "name"})
	}
	if in.ValidFrom.IsZero() {
		return apperrors.Validation("valid_from is required", map[string]any{"field": "valid_from"})
	}
	if err := ValidateDateRange(in.ValidFrom, in.ValidTo); err != nil {
		return err
	}
	if err := ValidateCurrencyCode(in.CurrencyCode); err != nil {
		return err
	}
	return nil
}

func ValidateDateRange(validFrom time.Time, validTo *time.Time) error {
	if validTo != nil && validTo.Before(validFrom) {
		return apperrors.Validation("valid_to must be on or after valid_from", map[string]any{"field": "valid_to"})
	}
	return nil
}

func NormalizeContractNumber(value string) string {
	return strings.TrimSpace(value)
}

func IsTerminalContractStatus(status string) bool {
	switch status {
	case ContractStatusTerminated, ContractStatusExpired, ContractStatusCancelled:
		return true
	default:
		return false
	}
}

func ContractEligibleForResolution(status string, onDate time.Time, validTo *time.Time) bool {
	if status != ContractStatusActive {
		return false
	}
	if validTo != nil && onDate.After(*validTo) {
		return false
	}
	return true
}

func ShouldExpireContract(status string, onDate time.Time, validTo *time.Time) bool {
	if status != ContractStatusActive && status != ContractStatusSuspended {
		return false
	}
	if validTo == nil {
		return false
	}
	return onDate.After(*validTo)
}

func ValidateDraftContractUpdate(current *TransportContract, patch UpdateContractInput) error {
	if current == nil {
		return apperrors.NotFound("contract not found")
	}
	if current.Status != ContractStatusDraft {
		return apperrors.Validation("only DRAFT contracts can be fully updated", map[string]any{"field": "status"})
	}
	validTo := current.ValidTo
	if patch.ValidTo.Present {
		validTo = ApplyNullableDatePatch(current.ValidTo, patch.ValidTo)
	}
	return ValidateDateRange(current.ValidFrom, validTo)
}

func ValidateMetadataPatch(current *TransportContract, patch PatchContractMetadataInput) error {
	if current == nil {
		return apperrors.NotFound("contract not found")
	}
	switch current.Status {
	case ContractStatusActive, ContractStatusSuspended:
		return nil
	default:
		return apperrors.Validation("contract metadata cannot be edited in current status", map[string]any{"status": current.Status})
	}
}

func ValidateImmutableFieldsMutation(current *TransportContract) error {
	return apperrors.Validation("immutable contract fields cannot be changed after activation", map[string]any{"field": "status"})
}

func ValidateContractMutationAllowed(current *TransportContract) error {
	if current == nil {
		return apperrors.NotFound("contract not found")
	}
	if IsTerminalContractStatus(current.Status) {
		return apperrors.Validation("terminal contract cannot be mutated", map[string]any{"status": current.Status})
	}
	return nil
}

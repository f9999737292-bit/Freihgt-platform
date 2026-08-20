package domain

import (
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
)

func ApplyLazyExpiration(contract *TransportContract, onDate time.Time) (bool, error) {
	if contract == nil {
		return false, apperrors.NotFound("contract not found")
	}
	if !ShouldExpireContract(contract.Status, onDate, contract.ValidTo) {
		return false, nil
	}
	contract.Status = ContractStatusExpired
	return true, nil
}

func ValidateActivateContract(contract *TransportContract, onDate time.Time) error {
	if contract == nil {
		return apperrors.NotFound("contract not found")
	}
	if contract.Status == ContractStatusActive {
		return nil
	}
	if contract.Status != ContractStatusDraft {
		return apperrors.Validation("only DRAFT contracts can be activated", map[string]any{"status": contract.Status})
	}
	if ShouldExpireContract(ContractStatusActive, onDate, contract.ValidTo) {
		return apperrors.Validation("contract validity window has already expired", map[string]any{"field": "valid_to"})
	}
	return nil
}

func ValidateSuspendContract(contract *TransportContract) error {
	if contract == nil {
		return apperrors.NotFound("contract not found")
	}
	if contract.Status == ContractStatusSuspended {
		return nil
	}
	if contract.Status != ContractStatusActive {
		return apperrors.Validation("only ACTIVE contracts can be suspended", map[string]any{"status": contract.Status})
	}
	return nil
}

func ValidateReactivateContract(contract *TransportContract, onDate time.Time) error {
	if contract == nil {
		return apperrors.NotFound("contract not found")
	}
	if contract.Status == ContractStatusActive {
		return nil
	}
	if contract.Status == ContractStatusExpired {
		return apperrors.Validation("expired contracts cannot be reactivated", map[string]any{"status": contract.Status})
	}
	if contract.Status != ContractStatusSuspended {
		return apperrors.Validation("only SUSPENDED contracts can be reactivated", map[string]any{"status": contract.Status})
	}
	if ShouldExpireContract(ContractStatusActive, onDate, contract.ValidTo) {
		return apperrors.Validation("contract validity window has expired", map[string]any{"field": "valid_to"})
	}
	return nil
}

func ValidateTerminateContract(contract *TransportContract) error {
	if contract == nil {
		return apperrors.NotFound("contract not found")
	}
	if contract.Status == ContractStatusTerminated {
		return nil
	}
	if contract.Status != ContractStatusActive && contract.Status != ContractStatusSuspended {
		return apperrors.Validation("only ACTIVE or SUSPENDED contracts can be terminated", map[string]any{"status": contract.Status})
	}
	return nil
}

func ValidateCancelContract(contract *TransportContract) error {
	if contract == nil {
		return apperrors.NotFound("contract not found")
	}
	if contract.Status == ContractStatusCancelled {
		return nil
	}
	if contract.Status != ContractStatusDraft {
		return apperrors.Validation("only DRAFT contracts can be cancelled", map[string]any{"status": contract.Status})
	}
	return nil
}

func TransitionActivate(contract *TransportContract, at time.Time, actorUserID uuid.UUID) {
	contract.Status = ContractStatusActive
	contract.ActivatedAt = &at
	contract.ActivatedBy = &actorUserID
}

func TransitionSuspend(contract *TransportContract) {
	contract.Status = ContractStatusSuspended
}

func TransitionReactivate(contract *TransportContract) {
	contract.Status = ContractStatusActive
}

func TransitionTerminate(contract *TransportContract, at time.Time, actorUserID uuid.UUID, reason *string) {
	contract.Status = ContractStatusTerminated
	contract.TerminatedAt = &at
	contract.TerminatedBy = &actorUserID
	contract.TerminationReason = reason
}

func TransitionCancel(contract *TransportContract) {
	contract.Status = ContractStatusCancelled
}

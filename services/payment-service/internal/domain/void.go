package domain

import (
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
)

const (
	AuditAllocationVoided     = "ALLOCATION_VOIDED"
	AuditPaymentReallocated   = "PAYMENT_REALLOCATED"
	AuditObligationReallocated = "OBLIGATION_REALLOCATED"
	AuditPaymentVoided        = "PAYMENT_VOIDED"
)

type VoidAllocationInput struct {
	TenantID       uuid.UUID
	AllocationID   uuid.UUID
	Reason         string
	ActorUserID    uuid.UUID
	ActorCompanyID uuid.UUID
	ActorKind      string
}

type VoidPaymentInput struct {
	TenantID       uuid.UUID
	PaymentID      uuid.UUID
	Reason         string
	ActorUserID    uuid.UUID
	ActorCompanyID uuid.UUID
	ActorKind      string
}

func ValidateVoidReason(reason string) (string, error) {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return "", apperrors.Validation("reason is required", map[string]any{"field": "reason"})
	}
	if len(trimmed) > 255 {
		return "", apperrors.Validation("reason must be at most 255 characters", map[string]any{"field": "reason"})
	}
	return trimmed, nil
}

func ValidateAllocationVoidFinality(payment *Payment, obligation *PaymentObligation) error {
	if payment == nil || obligation == nil {
		return apperrors.Internal("payment and obligation are required", nil)
	}
	if payment.Status == PaymentStatusReconciled {
		return apperrors.Conflict("reconciled payment allocations cannot be voided", nil)
	}
	if payment.Status == PaymentStatusVoided {
		return apperrors.Conflict("payment is voided", nil)
	}
	if obligation.Status == ObligationStatusPaid {
		return apperrors.Conflict("paid obligation allocations cannot be voided", map[string]any{"obligation_status": obligation.Status})
	}
	if obligation.Status == ObligationStatusCancelled || obligation.Status == ObligationStatusVoided {
		return apperrors.Conflict("obligation is not reversible", map[string]any{"obligation_status": obligation.Status})
	}
	return nil
}

func ValidatePaymentVoidPreconditions(payment *Payment, activeAllocationCount int, activeAllocationSum decimal.Decimal) error {
	if payment == nil {
		return apperrors.Internal("payment is required", nil)
	}
	if payment.Status == PaymentStatusVoided {
		return nil
	}
	if payment.Status == PaymentStatusReconciled {
		return apperrors.Conflict("reconciled payment cannot be voided", nil)
	}
	if payment.Status != PaymentStatusReceived {
		return apperrors.Conflict("payment cannot be voided in current status", map[string]any{"status": payment.Status})
	}
	if activeAllocationCount > 0 || !activeAllocationSum.IsZero() {
		return apperrors.Conflict("payment has active allocations", nil)
	}
	if !payment.AllocatedAmount.IsZero() || !payment.UnallocatedAmount.Equal(payment.Amount) {
		return apperrors.Conflict("payment allocation state is inconsistent", nil)
	}
	return nil
}

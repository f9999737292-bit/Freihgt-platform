package domain

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
)

const (
	PaymentSourceManual = "MANUAL"

	PaymentStatusReceived           = "RECEIVED"
	PaymentStatusPartiallyAllocated = "PARTIALLY_ALLOCATED"
	PaymentStatusFullyAllocated     = "FULLY_ALLOCATED"
	PaymentStatusReconciled           = "RECONCILED"
	PaymentStatusVoided               = "VOIDED"
)

func DerivePaymentAllocationStatus(amount, allocated decimal.Decimal) (string, error) {
	amount = RoundMoney(amount)
	allocated = RoundMoney(allocated)
	if allocated.IsNegative() {
		return "", apperrors.Conflict("allocated amount cannot be negative", nil)
	}
	if allocated.GreaterThan(amount) {
		return "", apperrors.Conflict("allocated amount cannot exceed payment amount", nil)
	}
	switch {
	case allocated.IsZero():
		return PaymentStatusReceived, nil
	case allocated.LessThan(amount):
		return PaymentStatusPartiallyAllocated, nil
	case MoneyEqual(allocated, amount):
		return PaymentStatusFullyAllocated, nil
	default:
		return "", apperrors.Conflict("invalid payment allocation state", nil)
	}
}

func DeriveUnallocated(amount, allocated decimal.Decimal) decimal.Decimal {
	return RoundMoney(amount.Sub(allocated))
}

func ValidateReconcilePayment(status string) error {
	if status != PaymentStatusFullyAllocated {
		return apperrors.Conflict("payment must be fully allocated before reconciliation", map[string]any{"status": status})
	}
	return nil
}

func ValidateManualPaymentSource(source string) error {
	if source != "" && source != PaymentSourceManual {
		return apperrors.Forbidden("manual payment endpoint accepts server-controlled MANUAL source only")
	}
	return nil
}

func ValidatePaymentActorForCreate(payer, payee, actorCompany uuid.UUID, actorKind string) error {
	switch actorKind {
	case PaymentActorBuyer:
		if payer != actorCompany {
			return apperrors.Forbidden("buyer can only create payments where buyer is payer")
		}
	case PaymentActorCarrier:
		if payee != actorCompany {
			return apperrors.Forbidden("carrier can only create payments where carrier is payee")
		}
	default:
		return apperrors.Forbidden("verified actor context is required")
	}
	return nil
}

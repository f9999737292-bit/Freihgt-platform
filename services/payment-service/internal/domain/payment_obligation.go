package domain

import (
	"github.com/shopspring/decimal"

	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
)

const (
	ObligationSourceBillingRegister = "BILLING_REGISTER"

	ObligationStatusOpen          = "OPEN"
	ObligationStatusPartiallyPaid = "PARTIALLY_PAID"
	ObligationStatusPaid          = "PAID"
	ObligationStatusCancelled     = "CANCELLED"
	ObligationStatusVoided        = "VOIDED"
)

func DeriveObligationStatus(original, paid decimal.Decimal) (string, error) {
	original = RoundMoney(original)
	paid = RoundMoney(paid)
	if paid.IsNegative() {
		return "", apperrors.Conflict("paid amount cannot be negative", nil)
	}
	if paid.GreaterThan(original) {
		return "", apperrors.Conflict("paid amount cannot exceed original amount", map[string]any{
			"paid_amount": paid.StringFixed(MoneyScale), "original_amount": original.StringFixed(MoneyScale),
		})
	}
	switch {
	case paid.IsZero():
		return ObligationStatusOpen, nil
	case paid.LessThan(original):
		return ObligationStatusPartiallyPaid, nil
	case MoneyEqual(paid, original):
		return ObligationStatusPaid, nil
	default:
		return "", apperrors.Conflict("invalid obligation paid state", nil)
	}
}

func DeriveOutstanding(original, paid decimal.Decimal) decimal.Decimal {
	return RoundMoney(original.Sub(paid))
}

func IsObligationOverdue(status string, dueDate *string, today string, outstanding decimal.Decimal) bool {
	if dueDate == nil || *dueDate == "" {
		return false
	}
	if outstanding.LessThanOrEqual(decimal.Zero) {
		return false
	}
	if status != ObligationStatusOpen && status != ObligationStatusPartiallyPaid {
		return false
	}
	return *dueDate < today
}

func ValidateObligationDueDateMutation(status string) error {
	switch status {
	case ObligationStatusPaid, ObligationStatusCancelled, ObligationStatusVoided:
		return apperrors.Conflict("due date cannot be changed in terminal financial state", map[string]any{"status": status})
	default:
		return nil
	}
}

func ValidateRegisterStatusForObligationEnsure(registerStatus string) error {
	switch registerStatus {
	case "SIGNED_BY_COUNTERPARTY", "PAID", "CLOSED":
		return nil
	default:
		return apperrors.Conflict("billing register must be signed by counterparty before obligation creation", map[string]any{
			"register_status": registerStatus,
		})
	}
}

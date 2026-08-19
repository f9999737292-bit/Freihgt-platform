package domain

import (
	"github.com/shopspring/decimal"

	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
)

const AuditPaymentReconciled = "PAYMENT_RECONCILED"

type ReconciliationSnapshot struct {
	ActiveAllocationCount         int
	ActiveAllocationSum           decimal.Decimal
	InvalidTenantCount            int
	InvalidCurrencyCount          int
	MissingObligationCount        int
	InvalidObligationTenantCount  int
	InvalidPartyCount             int
	InvalidObligationStateCount   int
	NonPositiveAmountCount        int
}

func (s ReconciliationSnapshot) HasRelationalViolations() bool {
	return s.InvalidTenantCount > 0 ||
		s.InvalidCurrencyCount > 0 ||
		s.MissingObligationCount > 0 ||
		s.InvalidObligationTenantCount > 0 ||
		s.InvalidPartyCount > 0 ||
		s.InvalidObligationStateCount > 0 ||
		s.NonPositiveAmountCount > 0
}

func ValidateReconciliationSnapshotIntegrity(snapshot ReconciliationSnapshot) error {
	if snapshot.HasRelationalViolations() {
		return apperrors.Conflict("active allocation relational integrity violation", map[string]any{
			"invalid_tenant_count":             snapshot.InvalidTenantCount,
			"invalid_currency_count":           snapshot.InvalidCurrencyCount,
			"missing_obligation_count":         snapshot.MissingObligationCount,
			"invalid_obligation_tenant_count":  snapshot.InvalidObligationTenantCount,
			"invalid_party_count":              snapshot.InvalidPartyCount,
			"invalid_obligation_state_count":   snapshot.InvalidObligationStateCount,
			"non_positive_amount_count":        snapshot.NonPositiveAmountCount,
		})
	}
	return nil
}

func ValidateNoVoidMetadata(p *Payment) error {
	if p == nil {
		return apperrors.Internal("payment is required", nil)
	}
	if p.VoidedAt != nil {
		return apperrors.Conflict("payment void metadata present", map[string]any{"field": "voided_at"})
	}
	if p.VoidedBy != nil {
		return apperrors.Conflict("payment void metadata present", map[string]any{"field": "voided_by"})
	}
	if p.VoidReason != nil {
		return apperrors.Conflict("payment void metadata present", map[string]any{"field": "void_reason"})
	}
	return nil
}

func ValidateFirstReconcileInvariants(p *Payment, snapshot ReconciliationSnapshot) error {
	if p == nil {
		return apperrors.Internal("payment is required", nil)
	}
	if !p.Amount.IsPositive() {
		return apperrors.Conflict("payment amount must be positive", nil)
	}
	if p.Status != PaymentStatusFullyAllocated {
		return apperrors.Conflict("payment must be fully allocated before reconciliation", map[string]any{"status": p.Status})
	}
	if p.Status == PaymentStatusVoided {
		return apperrors.Conflict("voided payment cannot be reconciled", nil)
	}
	if err := ValidateNoVoidMetadata(p); err != nil {
		return err
	}
	if p.ReconciledAt != nil || p.ReconciledBy != nil {
		return apperrors.Conflict("payment reconciliation metadata already present", nil)
	}
	if snapshot.ActiveAllocationCount <= 0 {
		return apperrors.Conflict("payment has no active allocations", nil)
	}
	if err := ValidateReconciliationSnapshotIntegrity(snapshot); err != nil {
		return err
	}
	if !MoneyEqual(snapshot.ActiveAllocationSum, p.Amount) {
		return apperrors.Conflict("active allocation sum does not equal payment amount", map[string]any{
			"active_allocation_sum": snapshot.ActiveAllocationSum.StringFixed(MoneyScale),
			"payment_amount":        p.Amount.StringFixed(MoneyScale),
		})
	}
	if !MoneyEqual(p.AllocatedAmount, p.Amount) {
		return apperrors.Conflict("stored allocated amount does not equal payment amount", map[string]any{
			"allocated_amount": p.AllocatedAmount.StringFixed(MoneyScale),
			"payment_amount":   p.Amount.StringFixed(MoneyScale),
		})
	}
	if !MoneyEqual(p.AllocatedAmount, snapshot.ActiveAllocationSum) {
		return apperrors.Conflict("stored allocated amount does not equal active allocation sum", map[string]any{
			"allocated_amount":      p.AllocatedAmount.StringFixed(MoneyScale),
			"active_allocation_sum": snapshot.ActiveAllocationSum.StringFixed(MoneyScale),
		})
	}
	if !p.UnallocatedAmount.IsZero() {
		return apperrors.Conflict("payment unallocated amount must be zero", map[string]any{
			"unallocated_amount": p.UnallocatedAmount.StringFixed(MoneyScale),
		})
	}
	return nil
}

func ValidateReconciledIntegrity(p *Payment, snapshot ReconciliationSnapshot) error {
	if p == nil {
		return apperrors.Internal("payment is required", nil)
	}
	if p.Status != PaymentStatusReconciled {
		return apperrors.Conflict("payment is not reconciled", map[string]any{"status": p.Status})
	}
	if p.ReconciledAt == nil || p.ReconciledBy == nil {
		return apperrors.Conflict("reconciled payment metadata is incomplete", nil)
	}
	if err := ValidateNoVoidMetadata(p); err != nil {
		return err
	}
	if snapshot.ActiveAllocationCount <= 0 {
		return apperrors.Conflict("reconciled payment has no active allocations", nil)
	}
	if err := ValidateReconciliationSnapshotIntegrity(snapshot); err != nil {
		return err
	}
	if !MoneyEqual(snapshot.ActiveAllocationSum, p.Amount) {
		return apperrors.Conflict("reconciled payment active allocation sum mismatch", nil)
	}
	if !MoneyEqual(p.AllocatedAmount, p.Amount) {
		return apperrors.Conflict("reconciled payment stored allocated amount mismatch", nil)
	}
	if !p.UnallocatedAmount.IsZero() {
		return apperrors.Conflict("reconciled payment unallocated amount must be zero", nil)
	}
	return nil
}

func ValidateAllocateAgainstReconciled(status string) error {
	if status == PaymentStatusReconciled {
		return apperrors.Conflict("reconciled payment cannot be allocated", nil)
	}
	return nil
}

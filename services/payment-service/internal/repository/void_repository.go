package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/payment-service/internal/domain"
	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
)

const allocationSelectCols = `
	id, tenant_id, payment_id, obligation_id, allocated_amount, currency_code,
	created_by, created_at, voided_at, voided_by, void_reason`

type VoidAllocationResult struct {
	Allocation *domain.PaymentAllocation
	Payment    *domain.Payment
	Obligation *domain.PaymentObligation
}

func scanAllocation(row pgx.Row) (*domain.PaymentAllocation, error) {
	var alloc domain.PaymentAllocation
	var amount string
	if err := row.Scan(
		&alloc.ID, &alloc.TenantID, &alloc.PaymentID, &alloc.ObligationID,
		&amount, &alloc.CurrencyCode, &alloc.CreatedBy, &alloc.CreatedAt,
		&alloc.VoidedAt, &alloc.VoidedBy, &alloc.VoidReason,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("payment allocation not found")
		}
		return nil, mapDBError(err)
	}
	parsed, err := decimal.NewFromString(amount)
	if err != nil {
		return nil, apperrors.Internal("invalid allocation amount", err)
	}
	alloc.AllocatedAmount = parsed
	return &alloc, nil
}

func (r *PaymentRepository) GetAllocationByID(ctx context.Context, tenantID, allocationID uuid.UUID) (*domain.PaymentAllocation, error) {
	query := `SELECT ` + allocationSelectCols + `
		FROM billing.payment_allocations WHERE id = $1 AND tenant_id = $2`
	return scanAllocation(r.pool.QueryRow(ctx, query, allocationID, tenantID))
}

func (r *PaymentRepository) getAllocationForUpdate(ctx context.Context, tx pgx.Tx, tenantID, allocationID uuid.UUID) (*domain.PaymentAllocation, error) {
	query := `SELECT ` + allocationSelectCols + `
		FROM billing.payment_allocations WHERE id = $1 AND tenant_id = $2 FOR UPDATE`
	return scanAllocation(tx.QueryRow(ctx, query, allocationID, tenantID))
}

// lockPaymentAndObligationForUpdate preserves the same UUID lock ordering as Allocate().
func (r *PaymentRepository) lockPaymentAndObligationForUpdate(ctx context.Context, tx pgx.Tx, tenantID, paymentID, obligationID uuid.UUID) (*domain.Payment, *domain.PaymentObligation, error) {
	firstID, secondID := obligationID, paymentID
	firstIsObligation := true
	if paymentID.String() < obligationID.String() {
		firstID, secondID = paymentID, obligationID
		firstIsObligation = false
	}
	var payment *domain.Payment
	var obligation *domain.PaymentObligation
	var err error
	if firstIsObligation {
		obligation, err = r.getObligationForUpdate(ctx, tx, tenantID, firstID)
		if err != nil {
			return nil, nil, err
		}
		payment, err = r.getPaymentForUpdate(ctx, tx, tenantID, secondID)
	} else {
		payment, err = r.getPaymentForUpdate(ctx, tx, tenantID, firstID)
		if err != nil {
			return nil, nil, err
		}
		obligation, err = r.getObligationForUpdate(ctx, tx, tenantID, secondID)
	}
	if err != nil {
		return nil, nil, err
	}
	return payment, obligation, nil
}

func sumActiveAllocationsForPaymentTx(ctx context.Context, tx pgx.Tx, paymentID uuid.UUID) (decimal.Decimal, int, error) {
	const query = `
		SELECT COALESCE(SUM(allocated_amount), 0), COUNT(*)
		FROM billing.payment_allocations
		WHERE payment_id = $1 AND voided_at IS NULL`
	var sumRaw string
	var count int
	if err := tx.QueryRow(ctx, query, paymentID).Scan(&sumRaw, &count); err != nil {
		return decimal.Zero, 0, mapDBError(err)
	}
	sum, err := decimal.NewFromString(sumRaw)
	if err != nil {
		return decimal.Zero, 0, apperrors.Internal("invalid allocation sum", err)
	}
	return domain.RoundMoney(sum), count, nil
}

func sumActiveAllocationsForObligationTx(ctx context.Context, tx pgx.Tx, obligationID uuid.UUID) (decimal.Decimal, error) {
	const query = `
		SELECT COALESCE(SUM(allocated_amount), 0)
		FROM billing.payment_allocations
		WHERE obligation_id = $1 AND voided_at IS NULL`
	var sumRaw string
	if err := tx.QueryRow(ctx, query, obligationID).Scan(&sumRaw); err != nil {
		return decimal.Zero, mapDBError(err)
	}
	sum, err := decimal.NewFromString(sumRaw)
	if err != nil {
		return decimal.Zero, apperrors.Internal("invalid allocation sum", err)
	}
	return domain.RoundMoney(sum), nil
}

func (r *PaymentRepository) VoidAllocation(ctx context.Context, in domain.VoidAllocationInput) (*VoidAllocationResult, error) {
	reason, err := domain.ValidateVoidReason(in.Reason)
	if err != nil {
		return nil, err
	}
	var result *VoidAllocationResult
	err = r.withTx(ctx, func(tx pgx.Tx) error {
		identity, err := r.GetAllocationByID(ctx, in.TenantID, in.AllocationID)
		if err != nil {
			return err
		}
		payment, obligation, err := r.lockPaymentAndObligationForUpdate(ctx, tx, in.TenantID, identity.PaymentID, identity.ObligationID)
		if err != nil {
			return err
		}
		alloc, err := r.getAllocationForUpdate(ctx, tx, in.TenantID, in.AllocationID)
		if err != nil {
			return err
		}
		if alloc.VoidedAt != nil {
			result = &VoidAllocationResult{Allocation: alloc, Payment: payment, Obligation: obligation}
			return nil
		}
		if err := domain.ValidateAllocationVoidFinality(payment, obligation); err != nil {
			return err
		}
		now := time.Now().UTC()
		const updateAlloc = `
			UPDATE billing.payment_allocations
			SET voided_at = $1, voided_by = $2, void_reason = $3
			WHERE id = $4 AND tenant_id = $5 AND voided_at IS NULL
			RETURNING ` + allocationSelectCols
		updatedAlloc, err := scanAllocation(tx.QueryRow(ctx, updateAlloc, now, in.ActorUserID, reason, alloc.ID, in.TenantID))
		if err != nil {
			return err
		}

		newPaid, err := sumActiveAllocationsForObligationTx(ctx, tx, obligation.ID)
		if err != nil {
			return err
		}
		newOutstanding := domain.DeriveOutstanding(obligation.OriginalAmount, newPaid)
		obligationStatus, err := domain.DeriveObligationStatus(obligation.OriginalAmount, newPaid)
		if err != nil {
			return err
		}

		newAllocated, _, err := sumActiveAllocationsForPaymentTx(ctx, tx, payment.ID)
		if err != nil {
			return err
		}
		newUnallocated := domain.DeriveUnallocated(payment.Amount, newAllocated)
		paymentStatus, err := domain.DerivePaymentAllocationStatus(payment.Amount, newAllocated)
		if err != nil {
			return err
		}

		const updateObligation = `
			UPDATE billing.payment_obligations
			SET paid_amount = $1, outstanding_amount = $2, status = $3, version = version + 1, updated_at = now()
			WHERE id = $4 AND tenant_id = $5 AND version = $6
			RETURNING ` + obligationSelectCols
		updatedObligation, err := scanObligation(tx.QueryRow(ctx, updateObligation,
			newPaid.StringFixed(domain.MoneyScale), newOutstanding.StringFixed(domain.MoneyScale),
			obligationStatus, obligation.ID, in.TenantID, obligation.Version))
		if err != nil {
			return err
		}

		const updatePayment = `
			UPDATE billing.payments
			SET allocated_amount = $1, unallocated_amount = $2, status = $3, version = version + 1, updated_at = now()
			WHERE id = $4 AND tenant_id = $5 AND version = $6
			RETURNING ` + paymentSelectCols
		updatedPayment, err := scanPayment(tx.QueryRow(ctx, updatePayment,
			newAllocated.StringFixed(domain.MoneyScale), newUnallocated.StringFixed(domain.MoneyScale),
			paymentStatus, payment.ID, in.TenantID, payment.Version))
		if err != nil {
			return err
		}

		if r.simulateAllocationVoidAuditFailure {
			return errors.New("simulated allocation void audit failure")
		}
		if err := r.insertAuditTx(ctx, tx, in.TenantID, "PAYMENT_ALLOCATION", updatedAlloc.ID, domain.AuditAllocationVoided,
			&in.ActorUserID, &in.ActorCompanyID, map[string]any{
				"payment_id": updatedAlloc.PaymentID.String(), "obligation_id": updatedAlloc.ObligationID.String(),
				"allocated_amount": updatedAlloc.AllocatedAmount.StringFixed(domain.MoneyScale),
				"reason": reason,
			}); err != nil {
			return err
		}
		if err := r.insertAuditTx(ctx, tx, in.TenantID, "PAYMENT", updatedPayment.ID, domain.AuditPaymentReallocated,
			&in.ActorUserID, &in.ActorCompanyID, map[string]any{
				"allocated_amount": newAllocated.StringFixed(domain.MoneyScale),
				"unallocated_amount": newUnallocated.StringFixed(domain.MoneyScale),
				"status": paymentStatus,
			}); err != nil {
			return err
		}
		if err := r.insertAuditTx(ctx, tx, in.TenantID, "PAYMENT_OBLIGATION", updatedObligation.ID, domain.AuditObligationReallocated,
			&in.ActorUserID, &in.ActorCompanyID, map[string]any{
				"paid_amount": newPaid.StringFixed(domain.MoneyScale),
				"outstanding_amount": newOutstanding.StringFixed(domain.MoneyScale),
				"status": obligationStatus,
			}); err != nil {
			return err
		}

		result = &VoidAllocationResult{Allocation: updatedAlloc, Payment: updatedPayment, Obligation: updatedObligation}
		return nil
	})
	return result, err
}

func (r *PaymentRepository) VoidPayment(ctx context.Context, in domain.VoidPaymentInput) (*domain.Payment, error) {
	reason, err := domain.ValidateVoidReason(in.Reason)
	if err != nil {
		return nil, err
	}
	var result *domain.Payment
	err = r.withTx(ctx, func(tx pgx.Tx) error {
		payment, err := r.getPaymentForUpdate(ctx, tx, in.TenantID, in.PaymentID)
		if err != nil {
			return err
		}
		if payment.Status == domain.PaymentStatusVoided {
			result = payment
			return nil
		}
		activeSum, activeCount, err := sumActiveAllocationsForPaymentTx(ctx, tx, payment.ID)
		if err != nil {
			return err
		}
		if err := domain.ValidatePaymentVoidPreconditions(payment, activeCount, activeSum); err != nil {
			return err
		}
		now := time.Now().UTC()
		const updatePayment = `
			UPDATE billing.payments
			SET status = $1, voided_at = $2, voided_by = $3, void_reason = $4, version = version + 1, updated_at = now()
			WHERE id = $5 AND tenant_id = $6 AND version = $7
			RETURNING ` + paymentSelectCols
		updated, err := scanPayment(tx.QueryRow(ctx, updatePayment,
			domain.PaymentStatusVoided, now, in.ActorUserID, reason,
			payment.ID, in.TenantID, payment.Version))
		if err != nil {
			return err
		}
		if r.simulatePaymentVoidAuditFailure {
			return errors.New("simulated payment void audit failure")
		}
		if err := r.insertAuditTx(ctx, tx, in.TenantID, "PAYMENT", updated.ID, domain.AuditPaymentVoided,
			&in.ActorUserID, &in.ActorCompanyID, map[string]any{"reason": reason}); err != nil {
			return err
		}
		result = updated
		return nil
	})
	return result, err
}

func (r *PaymentRepository) SimulateAllocationVoidAuditFailureForTest(ctx context.Context, in domain.VoidAllocationInput) error {
	r.simulateAllocationVoidAuditFailure = true
	defer func() { r.simulateAllocationVoidAuditFailure = false }()
	_, err := r.VoidAllocation(ctx, in)
	return err
}

func (r *PaymentRepository) SimulatePaymentVoidAuditFailureForTest(ctx context.Context, in domain.VoidPaymentInput) error {
	r.simulatePaymentVoidAuditFailure = true
	defer func() { r.simulatePaymentVoidAuditFailure = false }()
	_, err := r.VoidPayment(ctx, in)
	return err
}

func (r *PaymentRepository) CountAuditEvents(ctx context.Context, tenantID uuid.UUID, entityType string, entityID uuid.UUID, eventType string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM billing.payment_audit_events
		WHERE tenant_id = $1 AND entity_type = $2 AND entity_id = $3 AND event_type = $4`,
		tenantID, entityType, entityID, eventType,
	).Scan(&count)
	return count, mapDBError(err)
}

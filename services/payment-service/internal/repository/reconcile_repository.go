package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/payment-service/internal/domain"
	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
)

func (r *PaymentRepository) loadReconciliationSnapshotTx(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	payment *domain.Payment,
) (domain.ReconciliationSnapshot, error) {
	const query = `
		SELECT
			COUNT(a.id),
			COALESCE(SUM(a.allocated_amount), 0),
			COUNT(*) FILTER (WHERE a.tenant_id <> $2),
			COUNT(*) FILTER (WHERE a.currency_code <> $3),
			COUNT(*) FILTER (WHERE o.id IS NULL),
			COUNT(*) FILTER (WHERE o.id IS NOT NULL AND o.tenant_id <> $2),
			COUNT(*) FILTER (WHERE o.id IS NOT NULL AND (o.payer_company_id <> $4 OR o.payee_company_id <> $5 OR o.currency_code <> $3)),
			COUNT(*) FILTER (WHERE o.status IN ('CANCELLED', 'VOIDED')),
			COUNT(*) FILTER (WHERE a.allocated_amount <= 0)
		FROM billing.payment_allocations a
		LEFT JOIN billing.payment_obligations o ON o.id = a.obligation_id
		WHERE a.payment_id = $1 AND a.voided_at IS NULL`

	var snapshot domain.ReconciliationSnapshot
	var sum string
	if err := tx.QueryRow(ctx, query,
		payment.ID, tenantID, payment.CurrencyCode, payment.PayerCompanyID, payment.PayeeCompanyID,
	).Scan(
		&snapshot.ActiveAllocationCount,
		&sum,
		&snapshot.InvalidTenantCount,
		&snapshot.InvalidCurrencyCount,
		&snapshot.MissingObligationCount,
		&snapshot.InvalidObligationTenantCount,
		&snapshot.InvalidPartyCount,
		&snapshot.InvalidObligationStateCount,
		&snapshot.NonPositiveAmountCount,
	); err != nil {
		return snapshot, mapDBError(err)
	}
	activeSum, err := decimal.NewFromString(sum)
	if err != nil {
		return snapshot, apperrors.Internal("invalid active allocation sum", err)
	}
	snapshot.ActiveAllocationSum = domain.RoundMoney(activeSum)
	return snapshot, nil
}

func (r *PaymentRepository) ReconcilePayment(ctx context.Context, tenantID, paymentID uuid.UUID, actor domain.PaymentActorInput) (*domain.Payment, error) {
	var result *domain.Payment
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		p, err := r.getPaymentForUpdate(ctx, tx, tenantID, paymentID)
		if err != nil {
			return err
		}
		if err := domain.ValidatePaymentAccess(p.PayerCompanyID, p.PayeeCompanyID, actor.ActorCompanyID, actor.ActorKind); err != nil {
			return err
		}

		snapshot, err := r.loadReconciliationSnapshotTx(ctx, tx, tenantID, p)
		if err != nil {
			return err
		}

		if p.Status == domain.PaymentStatusReconciled {
			if err := domain.ValidateReconciledIntegrity(p, snapshot); err != nil {
				return err
			}
			result = p
			return nil
		}

		if p.Status == domain.PaymentStatusVoided {
			return apperrors.Conflict("voided payment cannot be reconciled", nil)
		}
		if err := domain.ValidateFirstReconcileInvariants(p, snapshot); err != nil {
			return err
		}

		reconciledAt := time.Now().UTC()
		const query = `
			UPDATE billing.payments
			SET status = $1, reconciled_at = $2, reconciled_by = $3, version = version + 1, updated_at = $2
			WHERE id = $4 AND tenant_id = $5 AND version = $6
			RETURNING ` + paymentSelectCols
		updated, err := scanPayment(tx.QueryRow(ctx, query,
			domain.PaymentStatusReconciled, reconciledAt, actor.ActorUserID, paymentID, tenantID, p.Version))
		if err != nil {
			return err
		}
		result = updated

		if r.simulateReconcileAuditFailure {
			return apperrors.Internal("simulated reconcile audit failure", nil)
		}

		return r.insertAuditTx(ctx, tx, tenantID, "PAYMENT", paymentID, domain.AuditPaymentReconciled,
			&actor.ActorUserID, &actor.ActorCompanyID, map[string]any{
				"status":                  domain.PaymentStatusReconciled,
				"amount":                  updated.Amount.StringFixed(domain.MoneyScale),
				"allocated_amount":        updated.AllocatedAmount.StringFixed(domain.MoneyScale),
				"unallocated_amount":      updated.UnallocatedAmount.StringFixed(domain.MoneyScale),
				"active_allocation_sum":   snapshot.ActiveAllocationSum.StringFixed(domain.MoneyScale),
				"active_allocation_count": snapshot.ActiveAllocationCount,
			})
	})
	return result, err
}

func (r *PaymentRepository) SimulateReconcileAuditFailureForTest(ctx context.Context, tenantID, paymentID uuid.UUID, actor domain.PaymentActorInput) error {
	r.simulateReconcileAuditFailure = true
	defer func() { r.simulateReconcileAuditFailure = false }()
	_, err := r.ReconcilePayment(ctx, tenantID, paymentID, actor)
	return err
}

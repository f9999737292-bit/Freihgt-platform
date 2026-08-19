package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/payment-service/internal/domain"
	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
)

type PaymentRepository struct {
	pool                           *pgxpool.Pool
	simulateObligationAuditFailure bool
	simulatePaymentAuditFailure    bool
}

func NewPaymentRepository(pool *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{pool: pool}
}

func (r *PaymentRepository) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return mapDBError(err)
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return mapDBError(tx.Commit(ctx))
}

func scanObligation(row pgx.Row) (*domain.PaymentObligation, error) {
	var o domain.PaymentObligation
	var original, paid, outstanding string
	var dueDate *time.Time
	var blocked *string
	if err := row.Scan(
		&o.ID, &o.TenantID, &o.ObligationNumber, &o.PayerCompanyID, &o.PayeeCompanyID,
		&o.SourceType, &o.SourceID, &o.CurrencyCode, &original, &paid, &outstanding,
		&dueDate, &o.Status, &blocked, &o.Version, &o.CreatedAt, &o.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("payment obligation not found")
		}
		return nil, mapDBError(err)
	}
	var err error
	o.OriginalAmount, err = decimal.NewFromString(original)
	if err != nil {
		return nil, apperrors.Internal("invalid original_amount", err)
	}
	o.PaidAmount, err = decimal.NewFromString(paid)
	if err != nil {
		return nil, apperrors.Internal("invalid paid_amount", err)
	}
	o.OutstandingAmount, err = decimal.NewFromString(outstanding)
	if err != nil {
		return nil, apperrors.Internal("invalid outstanding_amount", err)
	}
	o.DueDate = dueDate
	o.BlockedReason = blocked
	return &o, nil
}

const obligationSelectCols = `
	id, tenant_id, obligation_number, payer_company_id, payee_company_id,
	source_type, source_id, currency_code, original_amount, paid_amount, outstanding_amount,
	due_date, status, blocked_reason, version, created_at, updated_at`

func (r *PaymentRepository) GetObligationByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.PaymentObligation, error) {
	query := `SELECT ` + obligationSelectCols + ` FROM billing.payment_obligations WHERE id = $1 AND tenant_id = $2`
	return scanObligation(r.pool.QueryRow(ctx, query, id, tenantID))
}

func (r *PaymentRepository) GetObligationBySource(ctx context.Context, tenantID uuid.UUID, sourceType string, sourceID uuid.UUID) (*domain.PaymentObligation, error) {
	query := `SELECT ` + obligationSelectCols + `
		FROM billing.payment_obligations
		WHERE tenant_id = $1 AND source_type = $2 AND source_id = $3`
	return scanObligation(r.pool.QueryRow(ctx, query, tenantID, sourceType, sourceID))
}

func (r *PaymentRepository) EnsureObligationForBillingRegister(ctx context.Context, tenantID, registerID uuid.UUID, snap *BillingRegisterSnapshot) (*domain.PaymentObligation, error) {
	if err := domain.ValidateRegisterStatusForObligationEnsure(snap.Status); err != nil {
		return nil, err
	}
	existing, err := r.GetObligationBySource(ctx, tenantID, domain.ObligationSourceBillingRegister, registerID)
	if err == nil {
		return existing, nil
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		return nil, err
	}

	original := snap.TotalWithVAT
	if original.LessThanOrEqual(decimal.Zero) {
		return nil, apperrors.Validation("register total must be greater than zero for obligation", map[string]any{"field": "total_with_vat"})
	}
	outstanding := domain.DeriveOutstanding(original, decimal.Zero)
	number := obligationNumber(snap.RegisterNumber, registerID)

	var obligation *domain.PaymentObligation
	err = r.withTx(ctx, func(tx pgx.Tx) error {
		if existingTx, lookupErr := r.getObligationBySourceTx(ctx, tx, tenantID, domain.ObligationSourceBillingRegister, registerID); lookupErr == nil {
			obligation = existingTx
			return nil
		} else {
			var lookupAppErr *apperrors.AppError
			if !errors.As(lookupErr, &lookupAppErr) || lookupAppErr.Code != apperrors.CodeNotFound {
				return lookupErr
			}
		}

		const insert = `
			INSERT INTO billing.payment_obligations (
				tenant_id, obligation_number, payer_company_id, payee_company_id,
				source_type, source_id, currency_code, original_amount, paid_amount, outstanding_amount, status
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT (tenant_id, source_type, source_id) DO NOTHING
			RETURNING ` + obligationSelectCols

		row := tx.QueryRow(ctx, insert,
			tenantID, number, snap.CustomerCompanyID, snap.ContractorCompanyID,
			domain.ObligationSourceBillingRegister, registerID, snap.CurrencyCode,
			original.StringFixed(domain.MoneyScale), decimal.Zero.StringFixed(domain.MoneyScale),
			outstanding.StringFixed(domain.MoneyScale), domain.ObligationStatusOpen,
		)
		inserted, insertErr := scanObligation(row)
		if insertErr != nil {
			if errors.Is(insertErr, pgx.ErrNoRows) {
				fetched, fetchErr := r.getObligationBySourceTx(ctx, tx, tenantID, domain.ObligationSourceBillingRegister, registerID)
				if fetchErr != nil {
					return fetchErr
				}
				obligation = fetched
				return nil
			}
			return insertErr
		}
		if r.simulateObligationAuditFailure {
			return errors.New("simulated obligation audit failure")
		}
		if auditErr := r.insertAuditTx(ctx, tx, tenantID, "PAYMENT_OBLIGATION", inserted.ID, "OBLIGATION_CREATED", nil, nil, map[string]any{
			"source_type": domain.ObligationSourceBillingRegister, "source_id": registerID.String(),
			"original_amount": original.StringFixed(domain.MoneyScale),
		}); auditErr != nil {
			return auditErr
		}
		obligation = inserted
		return nil
	})
	return obligation, err
}

func (r *PaymentRepository) getObligationBySourceTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, sourceType string, sourceID uuid.UUID) (*domain.PaymentObligation, error) {
	query := `SELECT ` + obligationSelectCols + `
		FROM billing.payment_obligations
		WHERE tenant_id = $1 AND source_type = $2 AND source_id = $3`
	return scanObligation(tx.QueryRow(ctx, query, tenantID, sourceType, sourceID))
}

func (r *PaymentRepository) SimulateObligationAuditFailureForTest(ctx context.Context, tenantID, registerID uuid.UUID, snap *BillingRegisterSnapshot) error {
	r.simulateObligationAuditFailure = true
	defer func() { r.simulateObligationAuditFailure = false }()
	_, err := r.EnsureObligationForBillingRegister(ctx, tenantID, registerID, snap)
	return err
}

func (r *PaymentRepository) SimulatePaymentAuditFailureForTest(ctx context.Context, in domain.CreateManualPaymentInput) error {
	r.simulatePaymentAuditFailure = true
	defer func() { r.simulatePaymentAuditFailure = false }()
	_, err := r.CreateManualPayment(ctx, in)
	return err
}

func (r *PaymentRepository) UpdateObligationDueDate(ctx context.Context, tenantID, obligationID uuid.UUID, dueDate *time.Time, actor domain.PaymentActorInput) (*domain.PaymentObligation, error) {
	var result *domain.PaymentObligation
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		o, err := r.getObligationForUpdate(ctx, tx, tenantID, obligationID)
		if err != nil {
			return err
		}
		if err := domain.ValidatePaymentAccess(o.PayerCompanyID, o.PayeeCompanyID, actor.ActorCompanyID, actor.ActorKind); err != nil {
			return err
		}
		if err := domain.ValidateObligationDueDateMutation(o.Status); err != nil {
			return err
		}
		before := obligationAuditPayload(o)
		const query = `
			UPDATE billing.payment_obligations
			SET due_date = $1, version = version + 1, updated_at = now()
			WHERE id = $2 AND tenant_id = $3 AND version = $4
			RETURNING ` + obligationSelectCols
		updated, err := scanObligation(tx.QueryRow(ctx, query, optionalDate(dueDate), obligationID, tenantID, o.Version))
		if err != nil {
			return err
		}
		result = updated
		after := obligationAuditPayload(updated)
		return r.insertAuditTx(ctx, tx, tenantID, "PAYMENT_OBLIGATION", obligationID, "DUE_DATE_UPDATED",
			&actor.ActorUserID, &actor.ActorCompanyID, map[string]any{"before": before, "after": after})
	})
	return result, err
}

func obligationAuditPayload(o *domain.PaymentObligation) map[string]any {
	payload := map[string]any{
		"status": o.Status, "due_date": nil,
		"paid_amount": o.PaidAmount.StringFixed(domain.MoneyScale),
		"outstanding_amount": o.OutstandingAmount.StringFixed(domain.MoneyScale),
	}
	if o.DueDate != nil {
		payload["due_date"] = o.DueDate.Format("2006-01-02")
	}
	return payload
}

func (r *PaymentRepository) getObligationForUpdate(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) (*domain.PaymentObligation, error) {
	query := `SELECT ` + obligationSelectCols + `
		FROM billing.payment_obligations WHERE id = $1 AND tenant_id = $2 FOR UPDATE`
	return scanObligation(tx.QueryRow(ctx, query, id, tenantID))
}

func scanPayment(row pgx.Row) (*domain.Payment, error) {
	var p domain.Payment
	var amount, allocated, unallocated string
	var valueDate, reconciledAt, voidedAt *time.Time
	var reference, externalRef, externalID, voidReason *string
	var reconciledBy, voidedBy *uuid.UUID
	if err := row.Scan(
		&p.ID, &p.TenantID, &p.PaymentNumber, &p.PayerCompanyID, &p.PayeeCompanyID,
		&amount, &p.CurrencyCode, &p.PaymentDate, &valueDate, &reference, &externalRef,
		&p.Source, &externalID, &p.Status, &allocated, &unallocated, &p.CreatedBy,
		&reconciledAt, &reconciledBy, &voidedAt, &voidedBy, &voidReason,
		&p.Version, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("payment not found")
		}
		return nil, mapDBError(err)
	}
	var err error
	p.Amount, err = decimal.NewFromString(amount)
	if err != nil {
		return nil, apperrors.Internal("invalid payment amount", err)
	}
	p.AllocatedAmount, err = decimal.NewFromString(allocated)
	if err != nil {
		return nil, apperrors.Internal("invalid allocated amount", err)
	}
	p.UnallocatedAmount, err = decimal.NewFromString(unallocated)
	if err != nil {
		return nil, apperrors.Internal("invalid unallocated amount", err)
	}
	p.ValueDate = valueDate
	p.Reference = reference
	p.ExternalReference = externalRef
	p.ExternalID = externalID
	p.ReconciledAt = reconciledAt
	p.ReconciledBy = reconciledBy
	p.VoidedAt = voidedAt
	p.VoidedBy = voidedBy
	p.VoidReason = voidReason
	return &p, nil
}

const paymentSelectCols = `
	id, tenant_id, payment_number, payer_company_id, payee_company_id,
	amount, currency_code, payment_date, value_date, reference, external_reference,
	source, external_id, status, allocated_amount, unallocated_amount, created_by,
	reconciled_at, reconciled_by, voided_at, voided_by, void_reason,
	version, created_at, updated_at`

func (r *PaymentRepository) GetPaymentByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Payment, error) {
	query := `SELECT ` + paymentSelectCols + ` FROM billing.payments WHERE id = $1 AND tenant_id = $2`
	return scanPayment(r.pool.QueryRow(ctx, query, id, tenantID))
}

func (r *PaymentRepository) CreateManualPayment(ctx context.Context, in domain.CreateManualPaymentInput) (*domain.Payment, error) {
	unallocated := domain.DeriveUnallocated(in.Amount, decimal.Zero)
	number := fmt.Sprintf("PAY-%s", uuid.New().String()[:8])
	var payment *domain.Payment
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		const insert = `
			INSERT INTO billing.payments (
				tenant_id, payment_number, payer_company_id, payee_company_id,
				amount, currency_code, payment_date, reference, external_reference,
				source, external_id, status, allocated_amount, unallocated_amount, created_by
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
			RETURNING ` + paymentSelectCols
		created, err := scanPayment(tx.QueryRow(ctx, insert,
			in.TenantID, number, in.PayerCompanyID, in.PayeeCompanyID,
			in.Amount.StringFixed(domain.MoneyScale), domain.NormalizeCurrencyCode(in.CurrencyCode),
			in.PaymentDate, optionalString(in.Reference), optionalString(in.ExternalReference),
			domain.PaymentSourceManual, optionalString(in.ExternalID),
			domain.PaymentStatusReceived, decimal.Zero.StringFixed(domain.MoneyScale),
			unallocated.StringFixed(domain.MoneyScale), in.CreatedBy,
		))
		if err != nil {
			return err
		}
		if r.simulatePaymentAuditFailure {
			return errors.New("simulated payment audit failure")
		}
		if auditErr := r.insertAuditTx(ctx, tx, in.TenantID, "PAYMENT", created.ID, "PAYMENT_CREATED", &in.CreatedBy, &in.PayerCompanyID, map[string]any{
			"amount": created.Amount.StringFixed(domain.MoneyScale), "source": domain.PaymentSourceManual,
		}); auditErr != nil {
			return auditErr
		}
		payment = created
		return nil
	})
	return payment, err
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
		if err := domain.ValidateReconcilePayment(p.Status); err != nil {
			return err
		}
		const query = `
			UPDATE billing.payments
			SET status = $1, reconciled_at = now(), reconciled_by = $2, version = version + 1, updated_at = now()
			WHERE id = $3 AND tenant_id = $4 AND version = $5
			RETURNING ` + paymentSelectCols
		updated, err := scanPayment(tx.QueryRow(ctx, query,
			domain.PaymentStatusReconciled, actor.ActorUserID, paymentID, tenantID, p.Version))
		if err != nil {
			return err
		}
		result = updated
		return r.insertAuditTx(ctx, tx, tenantID, "PAYMENT", paymentID, "PAYMENT_RECONCILED",
			&actor.ActorUserID, &actor.ActorCompanyID, map[string]any{"status": domain.PaymentStatusReconciled})
	})
	return result, err
}

func (r *PaymentRepository) getPaymentForUpdate(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) (*domain.Payment, error) {
	query := `SELECT ` + paymentSelectCols + ` FROM billing.payments WHERE id = $1 AND tenant_id = $2 FOR UPDATE`
	return scanPayment(tx.QueryRow(ctx, query, id, tenantID))
}

type AllocateResult struct {
	Payment    *domain.Payment
	Obligation *domain.PaymentObligation
	Allocation *domain.PaymentAllocation
}

func (r *PaymentRepository) Allocate(ctx context.Context, in domain.CreateAllocationInput) (*AllocateResult, error) {
	var result *AllocateResult
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		firstID, secondID := in.ObligationID, in.PaymentID
		firstIsObligation := true
		if in.PaymentID.String() < in.ObligationID.String() {
			firstID, secondID = in.PaymentID, in.ObligationID
			firstIsObligation = false
		}

		var obligation *domain.PaymentObligation
		var payment *domain.Payment
		var err error

		if firstIsObligation {
			obligation, err = r.getObligationForUpdate(ctx, tx, in.TenantID, firstID)
			if err != nil {
				return err
			}
			payment, err = r.getPaymentForUpdate(ctx, tx, in.TenantID, secondID)
		} else {
			payment, err = r.getPaymentForUpdate(ctx, tx, in.TenantID, firstID)
			if err != nil {
				return err
			}
			obligation, err = r.getObligationForUpdate(ctx, tx, in.TenantID, secondID)
		}
		if err != nil {
			return err
		}
		if payment.Status == domain.PaymentStatusVoided {
			return apperrors.Conflict("payment is voided", nil)
		}
		if obligation.Status == domain.ObligationStatusCancelled || obligation.Status == domain.ObligationStatusVoided {
			return apperrors.Conflict("obligation is not allocatable", map[string]any{"status": obligation.Status})
		}
		if err := domain.ValidateAllocationParties(
			payment.PayerCompanyID, payment.PayeeCompanyID,
			obligation.PayerCompanyID, obligation.PayeeCompanyID,
			payment.CurrencyCode, obligation.CurrencyCode,
			in.AllocatedAmount, payment.UnallocatedAmount, obligation.OutstandingAmount,
		); err != nil {
			return err
		}

		const insertAlloc = `
			INSERT INTO billing.payment_allocations (
				tenant_id, payment_id, obligation_id, allocated_amount, currency_code, created_by
			) VALUES ($1,$2,$3,$4,$5,$6)
			RETURNING id, tenant_id, payment_id, obligation_id, allocated_amount, currency_code, created_by, created_at, voided_at`
		var alloc domain.PaymentAllocation
		var allocAmount string
		if err := tx.QueryRow(ctx, insertAlloc,
			in.TenantID, in.PaymentID, in.ObligationID,
			in.AllocatedAmount.StringFixed(domain.MoneyScale), payment.CurrencyCode, in.CreatedBy,
		).Scan(&alloc.ID, &alloc.TenantID, &alloc.PaymentID, &alloc.ObligationID,
			&allocAmount, &alloc.CurrencyCode, &alloc.CreatedBy, &alloc.CreatedAt, &alloc.VoidedAt); err != nil {
			return mapDBError(err)
		}
		alloc.AllocatedAmount, err = decimal.NewFromString(allocAmount)
		if err != nil {
			return apperrors.Internal("invalid allocation amount", err)
		}

		newPaid := domain.RoundMoney(obligation.PaidAmount.Add(in.AllocatedAmount))
		newOutstanding := domain.DeriveOutstanding(obligation.OriginalAmount, newPaid)
		obligationStatus, err := domain.DeriveObligationStatus(obligation.OriginalAmount, newPaid)
		if err != nil {
			return err
		}

		newAllocated := domain.RoundMoney(payment.AllocatedAmount.Add(in.AllocatedAmount))
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

		if err := r.insertAuditTx(ctx, tx, in.TenantID, "PAYMENT_ALLOCATION", alloc.ID, "ALLOCATION_CREATED",
			&in.CreatedBy, &in.ActorCompanyID, map[string]any{
				"payment_id": in.PaymentID.String(), "obligation_id": in.ObligationID.String(),
				"allocated_amount": in.AllocatedAmount.StringFixed(domain.MoneyScale),
			}); err != nil {
			return err
		}
		if err := r.insertAuditTx(ctx, tx, in.TenantID, "PAYMENT_OBLIGATION", obligation.ID, "OBLIGATION_ALLOCATED",
			&in.CreatedBy, &in.ActorCompanyID, map[string]any{
				"paid_amount": newPaid.StringFixed(domain.MoneyScale),
				"status": obligationStatus,
			}); err != nil {
			return err
		}

		result = &AllocateResult{Payment: updatedPayment, Obligation: updatedObligation, Allocation: &alloc}
		return nil
	})
	return result, err
}

func (r *PaymentRepository) insertAudit(ctx context.Context, tenantID uuid.UUID, entityType string, entityID uuid.UUID, eventType string, userID, companyID *uuid.UUID, payload map[string]any) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		return r.insertAuditTx(ctx, tx, tenantID, entityType, entityID, eventType, userID, companyID, payload)
	})
}

func (r *PaymentRepository) insertAuditTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, entityType string, entityID uuid.UUID, eventType string, userID, companyID *uuid.UUID, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return apperrors.Internal("audit payload encode failed", err)
	}
	const query = `
		INSERT INTO billing.payment_audit_events (tenant_id, entity_type, entity_id, event_type, actor_user_id, actor_company_id, payload)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`
	_, err = tx.Exec(ctx, query, tenantID, entityType, entityID, eventType, optionalUUID(userID), optionalUUID(companyID), raw)
	return mapDBError(err)
}

func (r *PaymentRepository) ListObligations(ctx context.Context, tenantID uuid.UUID, actor domain.PaymentActorInput, limit, offset int) ([]domain.PaymentObligation, error) {
	query := `SELECT ` + obligationSelectCols + ` FROM billing.payment_obligations WHERE tenant_id = $1`
	args := []any{tenantID}
	switch actor.ActorKind {
	case domain.PaymentActorBuyer:
		query += ` AND payer_company_id = $2`
		args = append(args, actor.ActorCompanyID)
	case domain.PaymentActorCarrier:
		query += ` AND payee_company_id = $2`
		args = append(args, actor.ActorCompanyID)
	}
	query += ` ORDER BY created_at DESC LIMIT $3 OFFSET $4`
	args = append(args, limit, offset)
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	result := make([]domain.PaymentObligation, 0)
	for rows.Next() {
		o, err := scanObligation(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *o)
	}
	return result, rows.Err()
}

func (r *PaymentRepository) ListPayments(ctx context.Context, tenantID uuid.UUID, actor domain.PaymentActorInput, limit, offset int) ([]domain.Payment, error) {
	query := `SELECT ` + paymentSelectCols + ` FROM billing.payments WHERE tenant_id = $1`
	args := []any{tenantID}
	switch actor.ActorKind {
	case domain.PaymentActorBuyer:
		query += ` AND payer_company_id = $2`
		args = append(args, actor.ActorCompanyID)
	case domain.PaymentActorCarrier:
		query += ` AND payee_company_id = $2`
		args = append(args, actor.ActorCompanyID)
	}
	query += ` ORDER BY created_at DESC LIMIT $3 OFFSET $4`
	args = append(args, limit, offset)
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	result := make([]domain.Payment, 0)
	for rows.Next() {
		p, err := scanPayment(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *p)
	}
	return result, rows.Err()
}

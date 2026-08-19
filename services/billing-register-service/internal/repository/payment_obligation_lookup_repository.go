package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/billing-register-service/internal/domain"
	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

type PaymentObligationSnapshot struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	SourceID          uuid.UUID
	Status            string
	OriginalAmount    decimal.Decimal
	PaidAmount        decimal.Decimal
	OutstandingAmount decimal.Decimal
}

type PaymentObligationLookupRepository struct {
	pool *pgxpool.Pool
}

func NewPaymentObligationLookupRepository(pool *pgxpool.Pool) *PaymentObligationLookupRepository {
	return &PaymentObligationLookupRepository{pool: pool}
}

func (r *PaymentObligationLookupRepository) GetByBillingRegister(ctx context.Context, tenantID, registerID uuid.UUID) (*PaymentObligationSnapshot, error) {
	const query = `
		SELECT id, tenant_id, source_id, status, original_amount, paid_amount, outstanding_amount
		FROM billing.payment_obligations
		WHERE tenant_id = $1 AND source_type = 'BILLING_REGISTER' AND source_id = $2`
	var snap PaymentObligationSnapshot
	var original, paid, outstanding string
	err := r.pool.QueryRow(ctx, query, tenantID, registerID).Scan(
		&snap.ID, &snap.TenantID, &snap.SourceID, &snap.Status, &original, &paid, &outstanding,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("payment obligation not found")
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	var parseErr error
	snap.OriginalAmount, parseErr = decimal.NewFromString(original)
	if parseErr != nil {
		return nil, apperrors.Internal("invalid obligation original_amount", parseErr)
	}
	snap.PaidAmount, parseErr = decimal.NewFromString(paid)
	if parseErr != nil {
		return nil, apperrors.Internal("invalid obligation paid_amount", parseErr)
	}
	snap.OutstandingAmount, parseErr = decimal.NewFromString(outstanding)
	if parseErr != nil {
		return nil, apperrors.Internal("invalid obligation outstanding_amount", parseErr)
	}
	return &snap, nil
}

func ValidateObligationPaidForRegisterSync(snap *PaymentObligationSnapshot) error {
	if snap.Status != "PAID" {
		return apperrors.Conflict("payment obligation is not PAID", map[string]any{"obligation_status": snap.Status})
	}
	if !snap.PaidAmount.Equal(snap.OriginalAmount) {
		return apperrors.Conflict("payment obligation paid_amount does not match original_amount", nil)
	}
	if !snap.OutstandingAmount.IsZero() {
		return apperrors.Conflict("payment obligation outstanding_amount must be zero", nil)
	}
	return nil
}

func (r *PaymentObligationLookupRepository) ValidateRegisterPaidPreconditions(ctx context.Context, tenantID, registerID uuid.UUID) error {
	snap, err := r.GetByBillingRegister(ctx, tenantID, registerID)
	if err != nil {
		return err
	}
	return ValidateObligationPaidForRegisterSync(snap)
}

// EnsureRegisterPaidInvariant documents the frozen projection invariant.
func EnsureRegisterPaidInvariant(reg *domain.BillingRegister, snap *PaymentObligationSnapshot) error {
	if reg.Status != domain.RegisterStatusPaid {
		return nil
	}
	return ValidateObligationPaidForRegisterSync(snap)
}

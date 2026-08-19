package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/payment-service/internal/domain"
	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
)

type BillingRegisterSnapshot struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	CustomerCompanyID   uuid.UUID
	ContractorCompanyID uuid.UUID
	CurrencyCode        string
	TotalWithVAT        decimal.Decimal
	Status              string
	RegisterNumber      string
}

type BillingRegisterLookupRepository struct {
	pool *pgxpool.Pool
}

func NewBillingRegisterLookupRepository(pool *pgxpool.Pool) *BillingRegisterLookupRepository {
	return &BillingRegisterLookupRepository{pool: pool}
}

func (r *BillingRegisterLookupRepository) GetSnapshot(ctx context.Context, tenantID, registerID uuid.UUID) (*BillingRegisterSnapshot, error) {
	const query = `
		SELECT id, tenant_id, customer_company_id, contractor_company_id, currency_code,
		       total_with_vat, status, register_number
		FROM billing.billing_registers
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
	var snap BillingRegisterSnapshot
	var total string
	if err := r.pool.QueryRow(ctx, query, registerID, tenantID).Scan(
		&snap.ID, &snap.TenantID, &snap.CustomerCompanyID, &snap.ContractorCompanyID,
		&snap.CurrencyCode, &total, &snap.Status, &snap.RegisterNumber,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("billing register not found")
		}
		return nil, mapDBError(err)
	}
	amount, err := decimal.NewFromString(total)
	if err != nil {
		return nil, apperrors.Internal("invalid register total", err)
	}
	snap.TotalWithVAT = domain.RoundMoney(amount)
	snap.CurrencyCode = domain.NormalizeCurrencyCode(snap.CurrencyCode)
	return &snap, nil
}

func obligationNumber(registerNumber string, registerID uuid.UUID) string {
	base := strings.TrimSpace(registerNumber)
	if base == "" {
		base = registerID.String()[:8]
	}
	return fmt.Sprintf("PO-%s", base)
}

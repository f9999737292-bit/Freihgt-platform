package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/payment-service/internal/domain"
	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
)

type InternalObligationRead struct {
	ObligationID      uuid.UUID
	TenantID          uuid.UUID
	BillingRegisterID uuid.UUID
	TransportOrderID  uuid.UUID
	BuyerCompanyID    uuid.UUID
	CarrierCompanyID  uuid.UUID
	Version           int
	OriginalAmount    string
	PaidAmount        string
	CurrencyCode      string
	Status            string
	TaxBasis          string
	UpdatedAt         time.Time
}

func (r *PaymentRepository) GetInternalObligationByBillingRegister(
	ctx context.Context,
	tenantID, billingRegisterID uuid.UUID,
) (*InternalObligationRead, error) {
	const query = `
		SELECT
			po.id,
			po.tenant_id,
			po.source_id,
			po.version,
			po.original_amount::text,
			po.paid_amount::text,
			po.currency_code,
			po.status,
			po.updated_at,
			po.payer_company_id,
			po.payee_company_id,
			fs.transport_order_id
		FROM billing.payment_obligations po
		JOIN billing.billing_registers br
			ON br.id = po.source_id AND br.tenant_id = po.tenant_id AND br.deleted_at IS NULL
		LEFT JOIN LATERAL (
			SELECT fs2.transport_order_id
			FROM billing.billing_register_items bri
			JOIN billing.freight_settlements fs2
				ON fs2.id = bri.settlement_id AND fs2.tenant_id = bri.tenant_id
			WHERE bri.register_id = br.id
			  AND bri.tenant_id = br.tenant_id
			ORDER BY bri.created_at ASC
			LIMIT 1
		) fs ON TRUE
		WHERE po.tenant_id = $1
		  AND po.source_type = $2
		  AND po.source_id = $3`

	var read InternalObligationRead
	var transportOrderID *uuid.UUID
	if err := r.pool.QueryRow(ctx, query, tenantID, domain.ObligationSourceBillingRegister, billingRegisterID).Scan(
		&read.ObligationID,
		&read.TenantID,
		&read.BillingRegisterID,
		&read.Version,
		&read.OriginalAmount,
		&read.PaidAmount,
		&read.CurrencyCode,
		&read.Status,
		&read.UpdatedAt,
		&read.BuyerCompanyID,
		&read.CarrierCompanyID,
		&transportOrderID,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("payment obligation not found")
		}
		return nil, mapDBError(err)
	}
	if transportOrderID == nil || *transportOrderID == uuid.Nil {
		return nil, apperrors.NotFound("payment obligation not found")
	}
	read.TransportOrderID = *transportOrderID
	read.TaxBasis = domain.TaxBasisWithVAT
	read.CurrencyCode = domain.NormalizeCurrencyCode(read.CurrencyCode)
	read.UpdatedAt = read.UpdatedAt.UTC()
	return &read, nil
}

package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func (r *TenderRepository) LoadAwardConversionContext(ctx context.Context, proposalID, tenantID uuid.UUID) (domain.AwardConversionContext, error) {
	var out domain.AwardConversionContext
	err := r.pool.QueryRow(ctx, `
		SELECT e.id, e.rfx_type, fr.id, fr.transport_order_id
		FROM rfx.award_proposals p
		JOIN rfx.rfx_events e ON e.id = p.rfx_event_id
		LEFT JOIN rfx.freight_requests fr ON fr.rfx_event_id = e.id AND fr.tenant_id = p.tenant_id AND fr.deleted_at IS NULL
		WHERE p.id = $1 AND p.tenant_id = $2
	`, proposalID, tenantID).Scan(&out.RfxEventID, &out.RfxType, &out.FreightReqID, &out.TransportOrderID)
	if err != nil {
		return out, mapDBError(err)
	}

	err = r.pool.QueryRow(ctx, `
		SELECT carrier_company_id, share_pct, expected_cost, COALESCE(currency_code, 'RUB')
		FROM rfx.award_proposal_lines
		WHERE award_proposal_id = $1 AND tenant_id = $2
		ORDER BY share_pct DESC, carrier_company_id ASC
		LIMIT 1
	`, proposalID, tenantID).Scan(&out.PrimaryCarrierID, &out.PrimarySharePct, &out.ExpectedCost, &out.CurrencyCode)
	if err != nil {
		return out, mapDBError(err)
	}
	return out, nil
}

func (r *TenderRepository) GetAwardConversionByKey(ctx context.Context, tenantID uuid.UUID, idempotencyKey string) (*uuid.UUID, *uuid.UUID, error) {
	var shipmentID uuid.UUID
	var awardID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT award_id, transport_order_id
		FROM rfx.award_transport_orders
		WHERE tenant_id = $1 AND idempotency_key = $2
	`, tenantID, idempotencyKey).Scan(&awardID, &shipmentID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, mapDBError(err)
	}
	return &awardID, &shipmentID, nil
}

func (r *TenderRepository) SaveAwardTransportOrder(ctx context.Context, tenantID, awardID, transportOrderID, carrierID uuid.UUID, idempotencyKey string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO rfx.award_transport_orders (tenant_id, award_id, transport_order_id, carrier_company_id, idempotency_key)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (tenant_id, idempotency_key) DO NOTHING
	`, tenantID, awardID, transportOrderID, carrierID, idempotencyKey)
	return mapDBError(err)
}

func (r *TenderRepository) GetAwardByProposal(ctx context.Context, proposalID, tenantID uuid.UUID) (uuid.UUID, error) {
	var awardID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT id FROM rfx.awards WHERE award_proposal_id = $1 AND tenant_id = $2
	`, proposalID, tenantID).Scan(&awardID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return uuid.Nil, apperrors.NotFound("award not found")
		}
		return uuid.Nil, mapDBError(err)
	}
	return awardID, nil
}

package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type PricingRepository struct {
	pool *pgxpool.Pool
}

func NewPricingRepository(pool *pgxpool.Pool) *PricingRepository {
	return &PricingRepository{pool: pool}
}

func (r *PricingRepository) GetAwardLinkContext(ctx context.Context, tenantID, linkID uuid.UUID) (domain.NormalizedPricingContext, error) {
	const query = `
		SELECT l.id, l.tenant_id, l.rfx_event_id, l.rfx_lot_id, l.buyer_company_id, l.carrier_company_id,
			trim(l.amount::text), l.currency_code,
			COALESCE(ln.origin_location_id, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(ln.destination_location_id, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(ln.equipment_type, ''), COALESCE(ln.transport_mode, 'ROAD')
		FROM rfx.rfx_award_transport_orders l
		LEFT JOIN rfx.rfx_lanes ln ON ln.id = l.rfx_lane_id AND ln.tenant_id = l.tenant_id
		WHERE l.id = $1 AND l.tenant_id = $2`
	row := r.pool.QueryRow(ctx, query, linkID, tenantID)
	return scanAwardPricingRow(row, linkID)
}

func (r *PricingRepository) GetAwardScopeContext(ctx context.Context, tenantID, eventID uuid.UUID, lotID *uuid.UUID) (domain.NormalizedPricingContext, error) {
	const eventQuery = `
		SELECT e.id, e.tenant_id, e.owner_company_id, a.carrier_company_id,
			trim(ol.amount::text), COALESCE(ol.currency_code, e.currency_code, 'RUB'),
			COALESCE(ln.origin_location_id, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(ln.destination_location_id, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(ln.equipment_type, ''), COALESCE(ln.transport_mode, 'ROAD'),
			ol.rfx_lot_id
		FROM rfx.rfx_events e
		JOIN rfx.rfx_awards a ON a.rfx_event_id = e.id AND a.tenant_id = e.tenant_id
		JOIN rfx.rfx_responses resp ON resp.id = a.rfx_response_id AND resp.tenant_id = e.tenant_id
		JOIN rfx.rfx_response_offer_lines ol ON ol.rfx_response_id = resp.id AND ol.tenant_id = e.tenant_id
		LEFT JOIN rfx.rfx_lots lot ON lot.id = ol.rfx_lot_id AND lot.tenant_id = e.tenant_id
		LEFT JOIN rfx.rfx_lanes ln ON ln.rfx_lot_id = lot.id AND ln.tenant_id = e.tenant_id
		WHERE e.id = $1 AND e.tenant_id = $2
		  AND ($3::uuid IS NULL OR ol.rfx_lot_id = $3)
		ORDER BY ln.created_at NULLS LAST
		LIMIT 1`
	var lotParam any
	if lotID != nil && *lotID != uuid.Nil {
		lotParam = *lotID
	}
	var (
		tenantOut, eventOut, buyerID, carrierID, originID, destID uuid.UUID
		amount, currency, equipment, mode                       string
		scopeLotID                                                *uuid.UUID
	)
	err := r.pool.QueryRow(ctx, eventQuery, eventID, tenantID, lotParam).Scan(
		&eventOut, &tenantOut, &buyerID, &carrierID, &amount, &currency,
		&originID, &destID, &equipment, &mode, &scopeLotID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NormalizedPricingContext{}, apperrors.NotFound("award pricing scope not found")
	}
	if err != nil {
		return domain.NormalizedPricingContext{}, mapDBError(err)
	}
	if tenantOut != tenantID {
		return domain.NormalizedPricingContext{}, apperrors.NotFound("award pricing scope not found")
	}
	sourceID := eventID
	if scopeLotID != nil && *scopeLotID != uuid.Nil {
		sourceID = *scopeLotID
	}
	ctxOut := domain.AggregateOnlyPricingContext(
		tenantID, sourceID, buyerID, carrierID, originID, destID,
		domain.PricingSourceRFQAward, equipment, mode, currency, formatDecimalAmount(amount), domain.RfxStatusAwarded,
	)
	ctxOut.RfxEventID = &eventOut
	if scopeLotID != nil && *scopeLotID != uuid.Nil {
		id := *scopeLotID
		ctxOut.RfxLotID = &id
	}
	if strings.TrimSpace(ctxOut.EquipmentType) == "" {
		ctxOut.EquipmentType = "TAUTLINER"
	}
	return ctxOut, domain.ValidateNormalizedPricingContext(ctxOut)
}

func (r *PricingRepository) GetAcceptedBidContext(ctx context.Context, tenantID, bidID uuid.UUID) (domain.NormalizedPricingContext, error) {
	const query = `
		SELECT b.id, b.tenant_id, b.status, b.carrier_company_id,
			trim(b.total_amount::text), COALESCE(b.currency_code, fr.currency_code, 'RUB'),
			fr.shipper_company_id,
			COALESCE(to_order.origin_location_id, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(to_order.destination_location_id, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(to_order.equipment_type, ''), COALESCE(to_order.transport_mode, 'ROAD')
		FROM rfx.bids b
		JOIN rfx.freight_requests fr ON fr.id = b.freight_request_id AND fr.tenant_id = b.tenant_id
		LEFT JOIN transport.transport_orders to_order
			ON to_order.id = fr.transport_order_id AND to_order.tenant_id = fr.tenant_id
		WHERE b.id = $1 AND b.tenant_id = $2 AND b.deleted_at IS NULL`
	var (
		id, tenantOut, carrierID, buyerID, originID, destID uuid.UUID
		status, amount, currency, equipment, mode           string
	)
	err := r.pool.QueryRow(ctx, query, bidID, tenantID).Scan(
		&id, &tenantOut, &status, &carrierID, &amount, &currency, &buyerID,
		&originID, &destID, &equipment, &mode,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NormalizedPricingContext{}, apperrors.NotFound("bid not found")
	}
	if err != nil {
		return domain.NormalizedPricingContext{}, mapDBError(err)
	}
	if tenantOut != tenantID {
		return domain.NormalizedPricingContext{}, apperrors.NotFound("bid not found")
	}
	if status != domain.BidStatusAccepted {
		return domain.NormalizedPricingContext{}, apperrors.Validation("bid is not ACCEPTED", map[string]any{"code": "INVALID_PRICING_SOURCE", "status": status})
	}
	ctxOut := domain.AggregateOnlyPricingContext(
		tenantID, bidID, buyerID, carrierID, originID, destID,
		domain.PricingSourceSpotBid, equipment, mode, currency, formatDecimalAmount(amount), status,
	)
	ctxOut.BidID = &bidID
	return ctxOut, domain.ValidateNormalizedPricingContext(ctxOut)
}

func scanAwardPricingRow(row pgx.Row, linkID uuid.UUID) (domain.NormalizedPricingContext, error) {
	var (
		id, tenantID, eventID, buyerID, carrierID, originID, destID uuid.UUID
		lotID                                                       *uuid.UUID
		amount, currency, equipment, mode                           string
	)
	err := row.Scan(&id, &tenantID, &eventID, &lotID, &buyerID, &carrierID, &amount, &currency, &originID, &destID, &equipment, &mode)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NormalizedPricingContext{}, apperrors.NotFound("award link not found")
	}
	if err != nil {
		return domain.NormalizedPricingContext{}, mapDBError(err)
	}
	sourceID := linkID
	ctxOut := domain.AggregateOnlyPricingContext(
		tenantID, sourceID, buyerID, carrierID, originID, destID,
		domain.PricingSourceRFQAward, equipment, mode, currency, formatDecimalAmount(amount), domain.RfxStatusAwarded,
	)
	ctxOut.AwardLinkID = &linkID
	ctxOut.RfxEventID = &eventID
	if lotID != nil && *lotID != uuid.Nil {
		idCopy := *lotID
		ctxOut.RfxLotID = &idCopy
	}
	return ctxOut, domain.ValidateNormalizedPricingContext(ctxOut)
}

func formatDecimalAmount(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, ".") {
		parts := strings.SplitN(raw, ".", 2)
		frac := parts[1]
		if len(frac) < 2 {
			frac = frac + strings.Repeat("0", 2-len(frac))
		} else if len(frac) > 2 {
			frac = frac[:2]
		}
		return parts[0] + "." + frac
	}
	return raw + ".00"
}

func formatPricingScopeKey(eventID uuid.UUID, lotID *uuid.UUID) string {
	if lotID == nil || *lotID == uuid.Nil {
		return fmt.Sprintf("event:%s", eventID)
	}
	return fmt.Sprintf("lot:%s", lotID.String())
}

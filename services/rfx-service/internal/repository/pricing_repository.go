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
			ln.origin_location_id, ln.destination_location_id,
			COALESCE(ln.equipment_type, ''), COALESCE(ln.transport_mode, '')
		FROM rfx.rfx_award_transport_orders l
		LEFT JOIN rfx.rfx_lanes ln ON ln.id = l.rfx_lane_id AND ln.tenant_id = l.tenant_id
		WHERE l.id = $1 AND l.tenant_id = $2`
	row := r.pool.QueryRow(ctx, query, linkID, tenantID)
	return scanAwardPricingRow(row, linkID)
}

func (r *PricingRepository) GetAwardScopeContext(ctx context.Context, tenantID, eventID uuid.UUID, lotID *uuid.UUID) (domain.NormalizedPricingContext, error) {
	scopeLotID, err := r.resolveAwardScopeLotID(ctx, tenantID, eventID, lotID)
	if err != nil {
		return domain.NormalizedPricingContext{}, err
	}
	laneCount, err := r.countLanesForLot(ctx, tenantID, scopeLotID)
	if err != nil {
		return domain.NormalizedPricingContext{}, err
	}
	if laneCount != 1 {
		return domain.NormalizedPricingContext{}, apperrors.Validation("award pricing scope is ambiguous", map[string]any{"code": "PRICING_SOURCE_AMBIGUOUS"})
	}

	const scopeQuery = `
		SELECT e.id, e.tenant_id, e.owner_company_id, a.carrier_company_id,
			trim(ol.amount::text),
			NULLIF(trim(COALESCE(ol.currency_code, '')), ''),
			NULLIF(trim(COALESCE(e.currency_code, '')), ''),
			ln.origin_location_id, ln.destination_location_id,
			NULLIF(trim(COALESCE(ln.equipment_type, '')), ''),
			NULLIF(trim(COALESCE(ln.transport_mode, '')), ''),
			ol.rfx_lot_id
		FROM rfx.rfx_events e
		JOIN rfx.rfx_awards a ON a.rfx_event_id = e.id AND a.tenant_id = e.tenant_id
		JOIN rfx.rfx_responses resp ON resp.id = a.rfx_response_id AND resp.tenant_id = e.tenant_id
		JOIN rfx.rfx_response_offer_lines ol ON ol.rfx_response_id = resp.id AND ol.tenant_id = e.tenant_id AND ol.rfx_lot_id = $3
		JOIN rfx.rfx_lanes ln ON ln.rfx_lot_id = ol.rfx_lot_id AND ln.tenant_id = e.tenant_id
		WHERE e.id = $1 AND e.tenant_id = $2`
	var (
		eventOut, tenantOut, buyerID, carrierID, originID, destID uuid.UUID
		amount, lineCurrency, eventCurrency, equipment, mode       *string
		scopeLotOut                                                uuid.UUID
	)
	err = r.pool.QueryRow(ctx, scopeQuery, eventID, tenantID, scopeLotID).Scan(
		&eventOut, &tenantOut, &buyerID, &carrierID, &amount,
		&lineCurrency, &eventCurrency,
		&originID, &destID, &equipment, &mode, &scopeLotOut,
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
	currency, err := resolveAuthoritativeCurrency(lineCurrency, eventCurrency)
	if err != nil {
		return domain.NormalizedPricingContext{}, err
	}
	equipmentType, err := requirePricingEquipment(equipment)
	if err != nil {
		return domain.NormalizedPricingContext{}, err
	}
	transportMode := domain.NormalizePricingTransportMode(derefString(mode))
	if originID == uuid.Nil || destID == uuid.Nil {
		return domain.NormalizedPricingContext{}, apperrors.Validation("origin and destination location ids are required", map[string]any{"code": "MISSING_PRICING_CONTEXT"})
	}
	ctxOut := domain.AggregateOnlyPricingContext(
		tenantID, scopeLotOut, buyerID, carrierID, originID, destID,
		domain.PricingSourceRFQAward, equipmentType, transportMode, currency, formatDecimalAmount(derefString(amount)), domain.RfxStatusAwarded,
	)
	ctxOut.RfxEventID = &eventOut
	id := scopeLotOut
	ctxOut.RfxLotID = &id
	return ctxOut, domain.ValidateNormalizedPricingContext(ctxOut)
}

func (r *PricingRepository) resolveAwardScopeLotID(ctx context.Context, tenantID, eventID uuid.UUID, lotID *uuid.UUID) (uuid.UUID, error) {
	if lotID != nil && *lotID != uuid.Nil {
		const verifyQuery = `
			SELECT 1
			FROM rfx.rfx_awards a
			JOIN rfx.rfx_responses resp ON resp.id = a.rfx_response_id AND resp.tenant_id = a.tenant_id
			JOIN rfx.rfx_response_offer_lines ol ON ol.rfx_response_id = resp.id AND ol.tenant_id = a.tenant_id
			WHERE a.rfx_event_id = $1 AND a.tenant_id = $2 AND ol.rfx_lot_id = $3
			LIMIT 1`
		var ok int
		err := r.pool.QueryRow(ctx, verifyQuery, eventID, tenantID, *lotID).Scan(&ok)
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, apperrors.NotFound("award pricing scope not found")
		}
		if err != nil {
			return uuid.Nil, mapDBError(err)
		}
		return *lotID, nil
	}

	const countQuery = `
		SELECT COUNT(DISTINCT ol.rfx_lot_id)
		FROM rfx.rfx_awards a
		JOIN rfx.rfx_responses resp ON resp.id = a.rfx_response_id AND resp.tenant_id = a.tenant_id
		JOIN rfx.rfx_response_offer_lines ol ON ol.rfx_response_id = resp.id AND ol.tenant_id = a.tenant_id
		WHERE a.rfx_event_id = $1 AND a.tenant_id = $2`
	var lotCount int
	if err := r.pool.QueryRow(ctx, countQuery, eventID, tenantID).Scan(&lotCount); err != nil {
		return uuid.Nil, mapDBError(err)
	}
	switch lotCount {
	case 0:
		return uuid.Nil, apperrors.NotFound("award pricing scope not found")
	case 1:
		const singleLotQuery = `
			SELECT DISTINCT ol.rfx_lot_id
			FROM rfx.rfx_awards a
			JOIN rfx.rfx_responses resp ON resp.id = a.rfx_response_id AND resp.tenant_id = a.tenant_id
			JOIN rfx.rfx_response_offer_lines ol ON ol.rfx_response_id = resp.id AND ol.tenant_id = a.tenant_id
			WHERE a.rfx_event_id = $1 AND a.tenant_id = $2
			LIMIT 1`
		var resolved uuid.UUID
		if err := r.pool.QueryRow(ctx, singleLotQuery, eventID, tenantID).Scan(&resolved); err != nil {
			return uuid.Nil, mapDBError(err)
		}
		return resolved, nil
	default:
		return uuid.Nil, apperrors.Validation("award pricing scope is ambiguous", map[string]any{"code": "PRICING_SOURCE_AMBIGUOUS"})
	}
}

func (r *PricingRepository) countLanesForLot(ctx context.Context, tenantID, lotID uuid.UUID) (int, error) {
	const query = `SELECT COUNT(*) FROM rfx.rfx_lanes WHERE tenant_id = $1 AND rfx_lot_id = $2`
	var count int
	if err := r.pool.QueryRow(ctx, query, tenantID, lotID).Scan(&count); err != nil {
		return 0, mapDBError(err)
	}
	return count, nil
}

func (r *PricingRepository) GetAcceptedBidContext(ctx context.Context, tenantID, bidID uuid.UUID) (domain.NormalizedPricingContext, error) {
	const query = `
		SELECT b.id, b.tenant_id, b.status, b.carrier_company_id,
			trim(b.total_amount::text),
			NULLIF(trim(COALESCE(b.currency_code, '')), ''),
			NULLIF(trim(COALESCE(fr.currency_code, '')), ''),
			fr.shipper_company_id,
			to_order.origin_location_id, to_order.destination_location_id,
			NULLIF(trim(COALESCE(to_order.equipment_type, '')), ''),
			NULLIF(trim(COALESCE(to_order.transport_mode, '')), '')
		FROM rfx.bids b
		JOIN rfx.freight_requests fr ON fr.id = b.freight_request_id AND fr.tenant_id = b.tenant_id
		LEFT JOIN transport.transport_orders to_order
			ON to_order.id = fr.transport_order_id AND to_order.tenant_id = fr.tenant_id
		WHERE b.id = $1 AND b.tenant_id = $2 AND b.deleted_at IS NULL`
	var (
		id, tenantOut, carrierID, buyerID, originID, destID uuid.UUID
		status, amount                                      string
		bidCurrency, requestCurrency, equipment, mode       *string
	)
	err := r.pool.QueryRow(ctx, query, bidID, tenantID).Scan(
		&id, &tenantOut, &status, &carrierID, &amount, &bidCurrency, &requestCurrency, &buyerID,
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
	currency, err := resolveAuthoritativeCurrency(bidCurrency, requestCurrency)
	if err != nil {
		return domain.NormalizedPricingContext{}, err
	}
	equipmentType, err := requirePricingEquipment(equipment)
	if err != nil {
		return domain.NormalizedPricingContext{}, err
	}
	if originID == uuid.Nil || destID == uuid.Nil {
		return domain.NormalizedPricingContext{}, apperrors.Validation("origin and destination location ids are required", map[string]any{"code": "MISSING_PRICING_CONTEXT"})
	}
	ctxOut := domain.AggregateOnlyPricingContext(
		tenantID, bidID, buyerID, carrierID, originID, destID,
		domain.PricingSourceSpotBid, equipmentType, domain.NormalizePricingTransportMode(derefString(mode)), currency, formatDecimalAmount(amount), status,
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
	if originID == uuid.Nil || destID == uuid.Nil {
		return domain.NormalizedPricingContext{}, apperrors.Validation("origin and destination location ids are required", map[string]any{"code": "MISSING_PRICING_CONTEXT"})
	}
	currency = strings.TrimSpace(currency)
	if currency == "" {
		return domain.NormalizedPricingContext{}, apperrors.Validation("currency_code is required", map[string]any{"code": "MISSING_PRICING_CONTEXT"})
	}
	equipmentType, err := requirePricingEquipment(&equipment)
	if err != nil {
		return domain.NormalizedPricingContext{}, err
	}
	sourceID := linkID
	ctxOut := domain.AggregateOnlyPricingContext(
		tenantID, sourceID, buyerID, carrierID, originID, destID,
		domain.PricingSourceRFQAward, equipmentType, domain.NormalizePricingTransportMode(mode), currency, formatDecimalAmount(amount), domain.RfxStatusAwarded,
	)
	ctxOut.AwardLinkID = &linkID
	ctxOut.RfxEventID = &eventID
	if lotID != nil && *lotID != uuid.Nil {
		idCopy := *lotID
		ctxOut.RfxLotID = &idCopy
	}
	return ctxOut, domain.ValidateNormalizedPricingContext(ctxOut)
}

func resolveAuthoritativeCurrency(primary, fallback *string) (string, error) {
	if primary != nil && strings.TrimSpace(*primary) != "" {
		return domain.NormalizeCurrencyCode(*primary), nil
	}
	if fallback != nil && strings.TrimSpace(*fallback) != "" {
		return domain.NormalizeCurrencyCode(*fallback), nil
	}
	return "", apperrors.Validation("currency_code is required", map[string]any{"code": "MISSING_PRICING_CONTEXT"})
}

func requirePricingEquipment(raw *string) (string, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return "", apperrors.Validation("equipment_type is required", map[string]any{"code": "MISSING_PRICING_CONTEXT"})
	}
	return strings.TrimSpace(*raw), nil
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
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

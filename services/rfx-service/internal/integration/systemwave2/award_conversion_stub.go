//go:build integration

package systemwave2

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/transportorderclient"
)

type awardConversionStub struct {
	pool        *pgxpool.Pool
	pricingRepo *repository.PricingRepository
}

func newAwardConversionStub(pool *pgxpool.Pool) *awardConversionStub {
	return &awardConversionStub{pool: pool, pricingRepo: repository.NewPricingRepository(pool)}
}

func (s *awardConversionStub) CreateFromAwardScope(ctx context.Context, in transportorderclient.CreateFromAwardScopeRequest) (transportorderclient.CreateFromAwardScopeResponse, error) {
	const existingQuery = `
		SELECT transport_order_id, rate_snapshot_id
		FROM transport.transport_order_create_idempotency
		WHERE tenant_id = $1 AND actor_company_id = $2 AND idempotency_key = $3`
	var orderID, snapshotID uuid.UUID
	err := s.pool.QueryRow(ctx, existingQuery, in.TenantID, in.ActorCompanyID, in.IdempotencyKey).Scan(&orderID, &snapshotID)
	if err == nil {
		var orderNumber, status string
		if err := s.pool.QueryRow(ctx, `
			SELECT order_number, status FROM transport.transport_orders
			WHERE id = $1 AND tenant_id = $2`, orderID, in.TenantID).Scan(&orderNumber, &status); err != nil {
			return transportorderclient.CreateFromAwardScopeResponse{}, err
		}
		return transportorderclient.CreateFromAwardScopeResponse{
			TransportOrderID: orderID,
			RateSnapshotID:   snapshotID,
			OrderNumber:      orderNumber,
			OrderStatus:      status,
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return transportorderclient.CreateFromAwardScopeResponse{}, err
	}

	pricing, err := s.pricingRepo.GetAwardScopeContext(ctx, in.TenantID, in.RfxEventID, in.RfxLotID)
	if err != nil {
		return transportorderclient.CreateFromAwardScopeResponse{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return transportorderclient.CreateFromAwardScopeResponse{}, err
	}
	defer tx.Rollback(ctx)

	const insertOrder = `
		INSERT INTO transport.transport_orders (
			tenant_id, order_number, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id,
			transport_mode, equipment_type, status, pricing_model_version,
			source_system, external_reference
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'DRAFT','SNAPSHOT_V1',$10,$11)
		RETURNING id`
	equipment := strings.TrimSpace(in.EquipmentType)
	if equipment == "" {
		equipment = "TAUTLINER"
	}
	if err := tx.QueryRow(ctx, insertOrder,
		in.TenantID, in.OrderNumber, in.ShipperCompanyID, in.ConsigneeCompanyID,
		in.OriginLocationID, in.DestinationLocationID, in.CargoID,
		in.TransportMode, equipment, in.SourceSystem, in.ExternalReference,
	).Scan(&orderID); err != nil {
		return transportorderclient.CreateFromAwardScopeResponse{}, err
	}

	const insertSnapshot = `
		INSERT INTO transport.transport_order_rate_snapshots (
			tenant_id, transport_order_id, buyer_company_id, carrier_company_id,
			pricing_source, rfx_event_id, rfx_lot_id,
			origin_location_id, destination_location_id, equipment_type, transport_mode,
			currency_code, component_breakdown_status, components, accessorial_rules,
			base_amount, total_amount, pricing_date, resolved_at, resolved_by_service,
			resolver_version, resolution_request_hash
		) VALUES (
			$1,$2,$3,$4,'RFQ_AWARD',$5,$6,$7,$8,$9,$10,$11,'UNAVAILABLE','[]'::jsonb,'[]'::jsonb,
			NULL,$12, CURRENT_DATE, now(), 'integration-stub', 'v2.0C', $13
		) RETURNING id`
	requestHash := hashRequestKey(in.IdempotencyKey)
	if err := tx.QueryRow(ctx, insertSnapshot,
		in.TenantID, orderID, pricing.BuyerCompanyID, pricing.CarrierCompanyID,
		in.RfxEventID, in.RfxLotID,
		pricing.OriginLocationID, pricing.DestinationLocationID, equipment, pricing.TransportMode,
		pricing.CurrencyCode, pricing.TotalAmount, requestHash,
	).Scan(&snapshotID); err != nil {
		return transportorderclient.CreateFromAwardScopeResponse{}, err
	}

	const insertIdempotency = `
		INSERT INTO transport.transport_order_create_idempotency (
			tenant_id, actor_company_id, idempotency_key, request_hash,
			transport_order_id, rate_snapshot_id
		) VALUES ($1,$2,$3,$4,$5,$6)`
	if _, err := tx.Exec(ctx, insertIdempotency,
		in.TenantID, in.ActorCompanyID, in.IdempotencyKey, requestHash, orderID, snapshotID,
	); err != nil {
		return transportorderclient.CreateFromAwardScopeResponse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return transportorderclient.CreateFromAwardScopeResponse{}, err
	}
	return transportorderclient.CreateFromAwardScopeResponse{
		TransportOrderID: orderID,
		RateSnapshotID:   snapshotID,
		OrderNumber:      in.OrderNumber,
		OrderStatus:      "DRAFT",
	}, nil
}

func hashRequestKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

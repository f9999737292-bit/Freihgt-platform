package repository

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/transport-order-service/internal/domain"
	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
)

type PricedOrderRepository struct {
	pool *pgxpool.Pool
}

func NewPricedOrderRepository(pool *pgxpool.Pool) *PricedOrderRepository {
	return &PricedOrderRepository{pool: pool}
}

func (r *PricedOrderRepository) WithCreateIdempotencyLock(
	ctx context.Context,
	tenantID, actorCompanyID uuid.UUID,
	idempotencyKey string,
	fn func(context.Context) (*domain.PricedTransportOrderResult, error),
) (*domain.PricedTransportOrderResult, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer conn.Release()

	key1, key2 := idempotencyAdvisoryLockKeys(tenantID, actorCompanyID, idempotencyKey)
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1, $2)", key1, key2); err != nil {
		return nil, mapDBError(err)
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1, $2)", key1, key2)

	return fn(ctx)
}

func idempotencyAdvisoryLockKeys(tenantID, actorCompanyID uuid.UUID, key string) (int32, int32) {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", tenantID, actorCompanyID, strings.TrimSpace(key))))
	return int32(binary.BigEndian.Uint32(sum[0:4])), int32(binary.BigEndian.Uint32(sum[4:8]))
}

func (r *PricedOrderRepository) FindCreateIdempotency(
	ctx context.Context,
	tenantID, actorCompanyID uuid.UUID,
	idempotencyKey string,
) (*domain.CreateIdempotencyRecord, error) {
	const query = `
		SELECT request_hash, transport_order_id, rate_snapshot_id
		FROM transport.transport_order_create_idempotency
		WHERE tenant_id = $1 AND actor_company_id = $2 AND idempotency_key = $3`
	var record domain.CreateIdempotencyRecord
	err := r.pool.QueryRow(ctx, query, tenantID, actorCompanyID, strings.TrimSpace(idempotencyKey)).Scan(
		&record.RequestHash, &record.TransportOrderID, &record.RateSnapshotID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return &record, nil
}

func (r *PricedOrderRepository) findCreateIdempotencyTx(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, actorCompanyID uuid.UUID,
	idempotencyKey string,
) (*domain.CreateIdempotencyRecord, error) {
	const query = `
		SELECT request_hash, transport_order_id, rate_snapshot_id
		FROM transport.transport_order_create_idempotency
		WHERE tenant_id = $1 AND actor_company_id = $2 AND idempotency_key = $3`
	var record domain.CreateIdempotencyRecord
	err := tx.QueryRow(ctx, query, tenantID, actorCompanyID, strings.TrimSpace(idempotencyKey)).Scan(
		&record.RequestHash, &record.TransportOrderID, &record.RateSnapshotID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return &record, nil
}

func (r *PricedOrderRepository) GetPricedResult(ctx context.Context, tenantID, orderID, snapshotID uuid.UUID) (*domain.PricedTransportOrderResult, error) {
	order, err := r.getOrderByID(ctx, tenantID, orderID)
	if err != nil {
		return nil, err
	}
	snapshot, err := r.getSnapshotByID(ctx, tenantID, snapshotID)
	if err != nil {
		return nil, err
	}
	return &domain.PricedTransportOrderResult{
		Order:          order,
		RateSnapshot:   snapshot,
		RateSnapshotID: snapshot.ID,
	}, nil
}

func (r *PricedOrderRepository) CreatePricedOrder(
	ctx context.Context,
	in domain.CreatePricedTransportOrderInput,
	snapshot domain.RateSnapshot,
	requestHash string,
) (*domain.PricedTransportOrderResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer tx.Rollback(ctx)

	const existingQuery = `
		SELECT request_hash, transport_order_id, rate_snapshot_id
		FROM transport.transport_order_create_idempotency
		WHERE tenant_id = $1 AND actor_company_id = $2 AND idempotency_key = $3`
	var (
		existingHash  string
		existingOrder uuid.UUID
		existingSnap  uuid.UUID
	)
	err = tx.QueryRow(ctx, existingQuery, in.TenantID, in.Actor.CompanyID, strings.TrimSpace(in.IdempotencyKey)).Scan(
		&existingHash, &existingOrder, &existingSnap,
	)
	if err == nil {
		if existingHash != requestHash {
			return nil, apperrors.Conflict("idempotency key reused with different request payload", map[string]any{"field": "idempotency_key"})
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, mapDBError(err)
		}
		return r.GetPricedResult(ctx, in.TenantID, existingOrder, existingSnap)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, mapDBError(err)
	}

	const insertOrder = `
		INSERT INTO transport.transport_orders (
			tenant_id, order_number, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id,
			requested_pickup_date, requested_delivery_date,
			transport_mode, equipment_type, status, pricing_model_version,
			source_system, external_reference
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		RETURNING id, tenant_id, order_number, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id,
			requested_pickup_date, requested_delivery_date,
			transport_mode, equipment_type, status, pricing_model_version,
			source_system, external_reference,
			created_at, updated_at, version`
	orderRow := tx.QueryRow(ctx, insertOrder,
		in.TenantID,
		strings.TrimSpace(in.OrderNumber),
		in.ShipperCompanyID,
		in.ConsigneeCompanyID,
		in.OriginLocationID,
		in.DestinationLocationID,
		in.CargoID,
		optionalDate(in.RequestedPickupDate),
		optionalDate(in.RequestedDeliveryDate),
		domain.NormalizeTransportMode(in.TransportMode),
		optionalString(in.EquipmentType),
		domain.TransportOrderStatusDraft,
		domain.PricingModelVersionSnapshotV1,
		optionalString(in.SourceSystem),
		optionalString(in.ExternalReference),
	)
	order, err := scanTransportOrderWithPricing(orderRow)
	if err != nil {
		if pgErr := conflictFromError(err); pgErr != nil {
			return nil, pgErr
		}
		return nil, mapDBError(err)
	}

	snapshot.TransportOrderID = order.ID
	snapshot.TenantID = in.TenantID
	createdSnapshot, err := insertSnapshotTx(ctx, tx, snapshot)
	if err != nil {
		return nil, err
	}

	const insertIdempotency = `
		INSERT INTO transport.transport_order_create_idempotency (
			tenant_id, actor_company_id, idempotency_key, request_hash,
			transport_order_id, rate_snapshot_id
		) VALUES ($1,$2,$3,$4,$5,$6)`
	if _, err := tx.Exec(ctx, insertIdempotency,
		in.TenantID, in.Actor.CompanyID, strings.TrimSpace(in.IdempotencyKey), requestHash,
		order.ID, createdSnapshot.ID,
	); err != nil {
		if pgErr := conflictFromError(err); pgErr != nil {
			if existing, lookupErr := r.findCreateIdempotencyTx(ctx, tx, in.TenantID, in.Actor.CompanyID, in.IdempotencyKey); lookupErr == nil && existing != nil {
				if existing.RequestHash != requestHash {
					return nil, apperrors.Conflict("idempotency key reused with different request payload", map[string]any{"field": "idempotency_key"})
				}
				if commitErr := tx.Commit(ctx); commitErr != nil {
					return nil, mapDBError(commitErr)
				}
				return r.GetPricedResult(ctx, in.TenantID, existing.TransportOrderID, existing.RateSnapshotID)
			}
		}
		return nil, mapDBError(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, mapDBError(err)
	}
	return &domain.PricedTransportOrderResult{
		Order:          order,
		RateSnapshot:   createdSnapshot,
		RateSnapshotID: createdSnapshot.ID,
	}, nil
}

func (r *PricedOrderRepository) getOrderByID(ctx context.Context, tenantID, orderID uuid.UUID) (*domain.TransportOrder, error) {
	const query = `
		SELECT id, tenant_id, order_number, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id,
			requested_pickup_date, requested_delivery_date,
			transport_mode, equipment_type, status, pricing_model_version,
			source_system, external_reference,
			created_at, updated_at, version
		FROM transport.transport_orders
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
	row := r.pool.QueryRow(ctx, query, orderID, tenantID)
	order, err := scanTransportOrderWithPricing(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("transport order not found")
	}
	return order, err
}

func (r *PricedOrderRepository) getSnapshotByID(ctx context.Context, tenantID, snapshotID uuid.UUID) (*domain.RateSnapshot, error) {
	const query = `
		SELECT id, tenant_id, transport_order_id, buyer_company_id, carrier_company_id,
			pricing_source, award_link_id, rfx_event_id, rfx_lot_id, bid_id, manual_spot_audit_id,
			contract_id, rate_card_id, rate_version_id, rate_line_id,
			contract_number, rate_card_name, rate_version_number,
			origin_location_id, destination_location_id, equipment_type, transport_mode,
			currency_code, component_breakdown_status, components, accessorial_rules,
			base_amount, total_amount, pricing_date, resolved_at, resolved_by_service,
			resolver_version, resolution_request_hash, created_at
		FROM transport.transport_order_rate_snapshots
		WHERE id = $1 AND tenant_id = $2`
	row := r.pool.QueryRow(ctx, query, snapshotID, tenantID)
	snapshot, err := scanRateSnapshot(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("rate snapshot not found")
	}
	return snapshot, err
}

func insertSnapshotTx(ctx context.Context, tx pgx.Tx, snapshot domain.RateSnapshot) (*domain.RateSnapshot, error) {
	const query = `
		INSERT INTO transport.transport_order_rate_snapshots (
			tenant_id, transport_order_id, buyer_company_id, carrier_company_id,
			pricing_source, award_link_id, rfx_event_id, rfx_lot_id, bid_id, manual_spot_audit_id,
			contract_id, rate_card_id, rate_version_id, rate_line_id,
			contract_number, rate_card_name, rate_version_number,
			origin_location_id, destination_location_id, equipment_type, transport_mode,
			currency_code, component_breakdown_status, components, accessorial_rules,
			base_amount, total_amount, pricing_date, resolved_at, resolved_by_service,
			resolver_version, resolution_request_hash
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32
		)
		RETURNING id, created_at`
	var id uuid.UUID
	var createdAt time.Time
	if err := tx.QueryRow(ctx, query,
		snapshot.TenantID,
		snapshot.TransportOrderID,
		snapshot.BuyerCompanyID,
		snapshot.CarrierCompanyID,
		snapshot.PricingSource,
		optionalUUID(snapshot.AwardLinkID),
		optionalUUID(snapshot.RfxEventID),
		optionalUUID(snapshot.RfxLotID),
		optionalUUID(snapshot.BidID),
		optionalUUID(snapshot.ManualSpotAuditID),
		optionalUUID(snapshot.ContractID),
		optionalUUID(snapshot.RateCardID),
		optionalUUID(snapshot.RateVersionID),
		optionalUUID(snapshot.RateLineID),
		optionalString(snapshot.ContractNumber),
		optionalString(snapshot.RateCardName),
		optionalInt(snapshot.RateVersionNumber),
		snapshot.OriginLocationID,
		snapshot.DestinationLocationID,
		snapshot.EquipmentType,
		snapshot.TransportMode,
		snapshot.CurrencyCode,
		snapshot.ComponentBreakdownStatus,
		snapshot.Components,
		snapshot.AccessorialRules,
		optionalDecimal(snapshot.BaseAmount),
		snapshot.TotalAmount.StringFixed(domain.MoneyScale),
		snapshot.PricingDate,
		snapshot.ResolvedAt,
		snapshot.ResolvedByService,
		snapshot.ResolverVersion,
		snapshot.ResolutionRequestHash,
	).Scan(&id, &createdAt); err != nil {
		return nil, mapDBError(err)
	}
	out := snapshot
	out.ID = id
	out.CreatedAt = createdAt
	return &out, nil
}

func scanTransportOrderWithPricing(row pgx.Row) (*domain.TransportOrder, error) {
	var order domain.TransportOrder
	err := row.Scan(
		&order.ID,
		&order.TenantID,
		&order.OrderNumber,
		&order.ShipperCompanyID,
		&order.ConsigneeCompanyID,
		&order.OriginLocationID,
		&order.DestinationLocationID,
		&order.CargoID,
		&order.RequestedPickupDate,
		&order.RequestedDeliveryDate,
		&order.TransportMode,
		&order.EquipmentType,
		&order.Status,
		&order.PricingModelVersion,
		&order.SourceSystem,
		&order.ExternalReference,
		&order.CreatedAt,
		&order.UpdatedAt,
		&order.Version,
	)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func scanRateSnapshot(row pgx.Row) (*domain.RateSnapshot, error) {
	var snapshot domain.RateSnapshot
	var baseText *string
	var totalText string
	err := row.Scan(
		&snapshot.ID,
		&snapshot.TenantID,
		&snapshot.TransportOrderID,
		&snapshot.BuyerCompanyID,
		&snapshot.CarrierCompanyID,
		&snapshot.PricingSource,
		&snapshot.AwardLinkID,
		&snapshot.RfxEventID,
		&snapshot.RfxLotID,
		&snapshot.BidID,
		&snapshot.ManualSpotAuditID,
		&snapshot.ContractID,
		&snapshot.RateCardID,
		&snapshot.RateVersionID,
		&snapshot.RateLineID,
		&snapshot.ContractNumber,
		&snapshot.RateCardName,
		&snapshot.RateVersionNumber,
		&snapshot.OriginLocationID,
		&snapshot.DestinationLocationID,
		&snapshot.EquipmentType,
		&snapshot.TransportMode,
		&snapshot.CurrencyCode,
		&snapshot.ComponentBreakdownStatus,
		&snapshot.Components,
		&snapshot.AccessorialRules,
		&baseText,
		&totalText,
		&snapshot.PricingDate,
		&snapshot.ResolvedAt,
		&snapshot.ResolvedByService,
		&snapshot.ResolverVersion,
		&snapshot.ResolutionRequestHash,
		&snapshot.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if baseText != nil && strings.TrimSpace(*baseText) != "" {
		base, err := decimal.NewFromString(strings.TrimSpace(*baseText))
		if err != nil {
			return nil, err
		}
		snapshot.BaseAmount = &base
	}
	total, err := decimal.NewFromString(strings.TrimSpace(totalText))
	if err != nil {
		return nil, err
	}
	snapshot.TotalAmount = total
	return &snapshot, nil
}

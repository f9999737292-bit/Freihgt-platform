package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type AwardConversionPrepared struct {
	Scope             domain.AwardConversionScope
	LaneID            uuid.UUID
	CargoID           uuid.UUID
	OriginLocationID  uuid.UUID
	DestinationLocationID uuid.UUID
	OrderNumber       string
	ExternalReference string
	EquipmentType     *string
	TransportMode     string
}

type AwardConversionLinkInput struct {
	Scope            domain.AwardConversionScope
	LaneID           uuid.UUID
	TransportOrderID uuid.UUID
	OrderNumber      string
	OrderStatus      string
}

func (r *RfxRepository) PrepareAwardConversion(
	ctx context.Context,
	event *domain.RfxEvent,
	award *domain.RfxAward,
	scope domain.AwardConversionScope,
	convertedBy uuid.UUID,
) (*AwardConversionPrepared, error) {
	lane, err := r.resolveConversionLane(ctx, scope.RfxLotID, event.TenantID)
	if err != nil {
		return nil, err
	}
	if lane.OriginLocationID == nil || lane.DestinationLocationID == nil {
		return nil, apperrors.Validation("lane route is incomplete", map[string]any{"field": "route"})
	}
	originID := *lane.OriginLocationID
	destID := *lane.DestinationLocationID
	if err := r.ensureTransportLocationsExist(ctx, event.TenantID, originID, destID); err != nil {
		return nil, err
	}
	cargoID, err := r.createPlaceholderCargo(ctx, event.TenantID, convertedBy)
	if err != nil {
		return nil, err
	}
	orderNumber := buildAwardOrderNumber(event.RfxNumber, scope.LotNumber)
	externalRef := fmt.Sprintf("%s:%s", award.ID.String(), scopeKey(scope.RfxLotID))
	transportMode := lane.TransportMode
	if strings.TrimSpace(transportMode) == "" {
		transportMode = "ROAD"
	}
	equipmentType := lane.EquipmentType
	if equipmentType == nil || strings.TrimSpace(*equipmentType) == "" {
		defaultEquip := "TAUTLINER"
		equipmentType = &defaultEquip
	}
	return &AwardConversionPrepared{
		Scope:                 scope,
		LaneID:                lane.ID,
		CargoID:               cargoID,
		OriginLocationID:      originID,
		DestinationLocationID: destID,
		OrderNumber:           orderNumber,
		ExternalReference:     externalRef,
		EquipmentType:         equipmentType,
		TransportMode:         transportMode,
	}, nil
}

func (r *RfxRepository) LinkAwardTransportOrdersTransactional(
	ctx context.Context,
	event *domain.RfxEvent,
	award *domain.RfxAward,
	response *domain.RfxResponse,
	links []AwardConversionLinkInput,
	convertedBy uuid.UUID,
	preCommit func(context.Context, pgx.Tx) error,
) (*domain.ConvertAwardTransportOrdersResult, error) {
	var result domain.ConvertAwardTransportOrdersResult
	if len(links) == 0 {
		existing, err := r.ListAwardTransportOrdersByEvent(ctx, event.ID, event.TenantID)
		if err != nil {
			return nil, err
		}
		result.Items = existing
		return &result, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer tx.Rollback(ctx)

	const lockAward = `
		SELECT id FROM rfx.rfx_awards
		WHERE rfx_event_id = $1 AND tenant_id = $2
		FOR UPDATE
	`
	var lockedAwardID uuid.UUID
	if err := tx.QueryRow(ctx, lockAward, event.ID, event.TenantID).Scan(&lockedAwardID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("rfx award not found")
		}
		return nil, mapDBError(err)
	}
	if lockedAwardID != award.ID {
		return nil, apperrors.Validation("award mismatch for event", map[string]any{"field": "award_id"})
	}

	existing, err := listAwardTransportOrdersTx(ctx, tx, event.ID, event.TenantID)
	if err != nil {
		return nil, err
	}
	existingByLot := map[uuid.UUID]domain.RfxAwardTransportOrder{}
	var existingEventLevel *domain.RfxAwardTransportOrder
	for i := range existing {
		item := existing[i]
		if item.RfxLotID == uuid.Nil {
			copyItem := item
			existingEventLevel = &copyItem
			continue
		}
		existingByLot[item.RfxLotID] = item
	}

	createdAny := false
	out := make([]domain.RfxAwardTransportOrder, 0, len(existing)+len(links))
	out = append(out, existing...)

	for _, link := range links {
		if link.Scope.RfxLotID != uuid.Nil {
			if _, ok := existingByLot[link.Scope.RfxLotID]; ok {
				continue
			}
		} else if existingEventLevel != nil {
			continue
		}
		item, err := linkAwardTransportOrderTx(ctx, tx, event, award, response, link, convertedBy)
		if err != nil {
			return nil, err
		}
		createdAny = true
		out = append(out, *item)
		if link.Scope.RfxLotID != uuid.Nil {
			existingByLot[link.Scope.RfxLotID] = *item
		} else {
			copyItem := *item
			existingEventLevel = &copyItem
		}
	}

	if preCommit != nil {
		if err := preCommit(ctx, tx); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, mapDBError(err)
	}
	result.Created = createdAny
	result.Items = out
	return &result, nil
}

func linkAwardTransportOrderTx(
	ctx context.Context,
	tx pgx.Tx,
	event *domain.RfxEvent,
	award *domain.RfxAward,
	response *domain.RfxResponse,
	link AwardConversionLinkInput,
	convertedBy uuid.UUID,
) (*domain.RfxAwardTransportOrder, error) {
	var lotID *uuid.UUID
	if link.Scope.RfxLotID != uuid.Nil {
		id := link.Scope.RfxLotID
		lotID = &id
	}
	var laneID *uuid.UUID
	if link.LaneID != uuid.Nil {
		id := link.LaneID
		laneID = &id
	}
	const insertLink = `
		INSERT INTO rfx.rfx_award_transport_orders (
			tenant_id, rfx_event_id, rfx_award_id, rfx_response_id, rfx_lot_id, rfx_lane_id,
			transport_order_id, carrier_company_id, buyer_company_id, amount, currency_code, converted_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, converted_at, version
	`
	var linkID uuid.UUID
	var convertedAt time.Time
	var version int
	if err := tx.QueryRow(ctx, insertLink,
		event.TenantID,
		event.ID,
		award.ID,
		response.ID,
		lotID,
		laneID,
		link.TransportOrderID,
		award.CarrierCompanyID,
		event.OwnerCompanyID,
		link.Scope.Amount,
		domain.NormalizeCurrencyCode(link.Scope.Currency),
		convertedBy,
	).Scan(&linkID, &convertedAt, &version); err != nil {
		return nil, mapDBError(err)
	}
	item := &domain.RfxAwardTransportOrder{
		ID:               linkID,
		TenantID:         event.TenantID,
		RfxEventID:       event.ID,
		RfxAwardID:       award.ID,
		RfxResponseID:    response.ID,
		RfxLotID:         link.Scope.RfxLotID,
		RfxLaneID:        link.LaneID,
		TransportOrderID: link.TransportOrderID,
		CarrierCompanyID: award.CarrierCompanyID,
		BuyerCompanyID:   event.OwnerCompanyID,
		Amount:           link.Scope.Amount,
		CurrencyCode:     domain.NormalizeCurrencyCode(link.Scope.Currency),
		ConvertedBy:      &convertedBy,
		ConvertedAt:      convertedAt,
		OrderNumber:      link.OrderNumber,
		OrderStatus:      link.OrderStatus,
		Version:          version,
	}
	return item, nil
}

func (r *RfxRepository) resolveConversionLane(ctx context.Context, lotID, tenantID uuid.UUID) (*domain.RfxLane, error) {
	if lotID == uuid.Nil {
		return nil, apperrors.Validation("route mapping unavailable without lots/lanes", map[string]any{"field": "route"})
	}
	const query = `
		SELECT id, tenant_id, rfx_lot_id, origin_location_id, destination_location_id,
			transport_mode, equipment_type, estimated_volume, volume_unit, required_service_level
		FROM rfx.rfx_lanes
		WHERE rfx_lot_id = $1 AND tenant_id = $2
		ORDER BY created_at
		LIMIT 1
	`
	row := r.pool.QueryRow(ctx, query, lotID, tenantID)
	lane, err := scanRfxLane(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Validation("lot has no lanes for route mapping", map[string]any{"field": "route", "rfx_lot_id": lotID.String()})
		}
		return nil, mapDBError(err)
	}
	return lane, nil
}

func (r *RfxRepository) ensureTransportLocationsExist(ctx context.Context, tenantID, originID, destID uuid.UUID) error {
	const query = `
		SELECT COUNT(*) FROM transport.locations
		WHERE tenant_id = $1 AND id = ANY($2::uuid[]) AND deleted_at IS NULL
	`
	var count int
	if err := r.pool.QueryRow(ctx, query, tenantID, []uuid.UUID{originID, destID}).Scan(&count); err != nil {
		return mapDBError(err)
	}
	if count != 2 {
		return apperrors.Validation("lane locations must exist in transport catalog", map[string]any{"field": "route"})
	}
	return nil
}

func (r *RfxRepository) createPlaceholderCargo(ctx context.Context, tenantID uuid.UUID, createdBy uuid.UUID) (uuid.UUID, error) {
	const query = `
		INSERT INTO transport.cargoes (tenant_id, cargo_type, description, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	var cargoID uuid.UUID
	if err := r.pool.QueryRow(ctx, query, tenantID, "GENERAL", "RFx award conversion placeholder", createdBy).Scan(&cargoID); err != nil {
		return uuid.Nil, mapDBError(err)
	}
	return cargoID, nil
}

func (r *RfxRepository) ListAwardTransportOrdersByEvent(ctx context.Context, eventID, tenantID uuid.UUID) ([]domain.RfxAwardTransportOrder, error) {
	const query = `
		SELECT l.id, l.tenant_id, l.rfx_event_id, l.rfx_award_id, l.rfx_response_id,
			l.rfx_lot_id, l.rfx_lane_id, l.transport_order_id, l.carrier_company_id,
			l.buyer_company_id, l.amount, l.currency_code, l.converted_by, l.converted_at,
			l.version, o.order_number, o.status
		FROM rfx.rfx_award_transport_orders l
		JOIN transport.transport_orders o ON o.id = l.transport_order_id AND o.tenant_id = l.tenant_id
		WHERE l.rfx_event_id = $1 AND l.tenant_id = $2
		ORDER BY l.converted_at, l.rfx_lot_id NULLS FIRST
	`
	rows, err := r.db().Query(ctx, query, eventID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	items := make([]domain.RfxAwardTransportOrder, 0)
	for rows.Next() {
		item, err := scanAwardTransportOrder(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func listAwardTransportOrdersTx(ctx context.Context, tx pgx.Tx, eventID, tenantID uuid.UUID) ([]domain.RfxAwardTransportOrder, error) {
	const query = `
		SELECT l.id, l.tenant_id, l.rfx_event_id, l.rfx_award_id, l.rfx_response_id,
			l.rfx_lot_id, l.rfx_lane_id, l.transport_order_id, l.carrier_company_id,
			l.buyer_company_id, l.amount, l.currency_code, l.converted_by, l.converted_at,
			l.version, o.order_number, o.status
		FROM rfx.rfx_award_transport_orders l
		JOIN transport.transport_orders o ON o.id = l.transport_order_id AND o.tenant_id = l.tenant_id
		WHERE l.rfx_event_id = $1 AND l.tenant_id = $2
		ORDER BY l.converted_at, l.rfx_lot_id NULLS FIRST
	`
	rows, err := tx.Query(ctx, query, eventID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	items := make([]domain.RfxAwardTransportOrder, 0)
	for rows.Next() {
		item, err := scanAwardTransportOrder(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func buildAwardOrderNumber(rfxNumber, lotNumber string) string {
	base := strings.TrimSpace(rfxNumber)
	if base == "" {
		base = "RFX"
	}
	if strings.TrimSpace(lotNumber) == "" {
		return base + "-TO"
	}
	return base + "-" + strings.TrimSpace(lotNumber) + "-TO"
}

func scopeKey(lotID uuid.UUID) string {
	if lotID == uuid.Nil {
		return "event"
	}
	return lotID.String()
}

func scanAwardTransportOrder(row pgx.Row) (*domain.RfxAwardTransportOrder, error) {
	var item domain.RfxAwardTransportOrder
	var lotID *uuid.UUID
	var laneID *uuid.UUID
	var convertedBy *uuid.UUID
	err := row.Scan(
		&item.ID,
		&item.TenantID,
		&item.RfxEventID,
		&item.RfxAwardID,
		&item.RfxResponseID,
		&lotID,
		&laneID,
		&item.TransportOrderID,
		&item.CarrierCompanyID,
		&item.BuyerCompanyID,
		&item.Amount,
		&item.CurrencyCode,
		&convertedBy,
		&item.ConvertedAt,
		&item.Version,
		&item.OrderNumber,
		&item.OrderStatus,
	)
	if err != nil {
		return nil, err
	}
	if lotID != nil {
		item.RfxLotID = *lotID
	}
	if laneID != nil {
		item.RfxLaneID = *laneID
	}
	item.ConvertedBy = convertedBy
	return &item, nil
}

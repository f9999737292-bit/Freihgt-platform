package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type OrderExecutionRepository struct {
	pool *pgxpool.Pool
}

func NewOrderExecutionRepository(pool *pgxpool.Pool) *OrderExecutionRepository {
	return &OrderExecutionRepository{pool: pool}
}

func (r *OrderExecutionRepository) GetAwardLinkByTransportOrderID(ctx context.Context, tenantID, transportOrderID uuid.UUID) (*domain.AwardTransportOrderLink, error) {
	const query = `
		SELECT id, tenant_id, rfx_event_id, rfx_award_id, rfx_response_id, rfx_lot_id,
			transport_order_id, carrier_company_id, buyer_company_id,
			amount::float8, currency_code, converted_at
		FROM rfx.rfx_award_transport_orders
		WHERE tenant_id = $1 AND transport_order_id = $2
	`
	var link domain.AwardTransportOrderLink
	var lotID *uuid.UUID
	err := r.pool.QueryRow(ctx, query, tenantID, transportOrderID).Scan(
		&link.ID, &link.TenantID, &link.RfxEventID, &link.RfxAwardID, &link.RfxResponseID, &lotID,
		&link.TransportOrderID, &link.CarrierCompanyID, &link.BuyerCompanyID,
		&link.Amount, &link.CurrencyCode, &link.ConvertedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("award transport order link not found")
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	link.RfxLotID = lotID
	return &link, nil
}

func (r *OrderExecutionRepository) GetShipmentByTransportOrderID(ctx context.Context, tenantID, transportOrderID uuid.UUID) (*domain.Shipment, error) {
	const query = `
		SELECT id, tenant_id, shipment_number, transport_order_id,
			shipper_company_id, consignee_company_id, carrier_company_id, forwarder_company_id,
			driver_id, vehicle_id, origin_location_id, destination_location_id, cargo_id,
			transport_mode, status, planned_pickup_at, planned_delivery_at,
			actual_pickup_at, actual_delivery_at, created_at, updated_at, version
		FROM transport.shipments
		WHERE tenant_id = $1 AND transport_order_id = $2 AND deleted_at IS NULL
		ORDER BY created_at ASC
		LIMIT 1
	`
	shipment, err := scanShipment(r.pool.QueryRow(ctx, query, tenantID, transportOrderID))
	if err != nil {
		return nil, err
	}
	return shipment, nil
}

func (r *OrderExecutionRepository) GetTransportOrderMeta(ctx context.Context, tenantID, orderID uuid.UUID) (orderNumber, status string, err error) {
	const query = `
		SELECT order_number, status
		FROM transport.transport_orders
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`
	err = r.pool.QueryRow(ctx, query, orderID, tenantID).Scan(&orderNumber, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", apperrors.NotFound("transport order not found")
	}
	if err != nil {
		return "", "", mapDBError(err)
	}
	return orderNumber, status, nil
}

type ExecuteAwardOrderParams struct {
	TenantID              uuid.UUID
	TransportOrderID      uuid.UUID
	CarrierCompanyID      uuid.UUID
	ShipmentNumber        string
	PlannedPickupAt       *time.Time
	PlannedDeliveryAt     *time.Time
	Transition            domain.StatusTransitionContext
	ImplicitCarrierAccept bool
}

type ExecuteAwardOrderResult struct {
	Shipment      *domain.Shipment
	OrderStatus   string
	Created       bool
	AwardLink     domain.AwardTransportOrderLink
	OrderNumber   string
}

func (r *OrderExecutionRepository) ExecuteAwardOrder(ctx context.Context, params ExecuteAwardOrderParams) (*ExecuteAwardOrderResult, error) {
	var result *ExecuteAwardOrderResult
	err := measureDB("order_execution_repository", "execute_award_order", func() error {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return mapDBError(err)
		}
		defer tx.Rollback(ctx)

		link, err := lockAwardLink(ctx, tx, params.TenantID, params.TransportOrderID)
		if err != nil {
			return err
		}
		if link.CarrierCompanyID != params.CarrierCompanyID {
			return apperrors.Forbidden("carrier company is not assigned to this transport order")
		}

		orderNumber, orderStatus, err := getTransportOrderForUpdate(ctx, tx, params.TenantID, params.TransportOrderID)
		if err != nil {
			return err
		}

		if existing, findErr := getShipmentByTransportOrderTx(ctx, tx, params.TenantID, params.TransportOrderID); findErr == nil {
			result = &ExecuteAwardOrderResult{
				Shipment:    existing,
				OrderStatus: orderStatus,
				Created:     false,
				AwardLink:   *link,
				OrderNumber: orderNumber,
			}
			return tx.Commit(ctx)
		} else if !isNotFound(findErr) {
			return findErr
		}

		switch orderStatus {
		case domain.TransportOrderStatusDraft:
			orderStatus, err = updateTransportOrderStatusTx(ctx, tx, params.TenantID, params.TransportOrderID, orderStatus, domain.TransportOrderStatusReadyForSourcing)
			if err != nil {
				return err
			}
		case domain.TransportOrderStatusReadyForSourcing, domain.TransportOrderStatusAssigned:
			// allowed
		case domain.TransportOrderStatusConverted:
			if existing, findErr := getShipmentByTransportOrderTx(ctx, tx, params.TenantID, params.TransportOrderID); findErr == nil {
				result = &ExecuteAwardOrderResult{
					Shipment:    existing,
					OrderStatus: orderStatus,
					Created:     false,
					AwardLink:   *link,
					OrderNumber: orderNumber,
				}
				return tx.Commit(ctx)
			}
			return apperrors.Conflict("transport order already converted but shipment is missing", nil)
		default:
			return apperrors.Validation("transport order is not eligible for execution", map[string]any{
				"field":  "transport_order_id",
				"status": orderStatus,
			})
		}

		orderSnapshot, err := getTransportOrderSnapshotTx(ctx, tx, params.TenantID, params.TransportOrderID)
		if err != nil {
			return err
		}

		initialStatus := domain.ShipmentStatusCarrierAssigned
		if params.ImplicitCarrierAccept {
			initialStatus = domain.ShipmentStatusAcceptedByCarrier
		}
		shipment, err := insertShipmentTx(ctx, tx, CreateShipmentParams{
			TenantID:              params.TenantID,
			ShipmentNumber:        params.ShipmentNumber,
			TransportOrderID:      params.TransportOrderID,
			ShipperCompanyID:      orderSnapshot.ShipperCompanyID,
			ConsigneeCompanyID:    orderSnapshot.ConsigneeCompanyID,
			CarrierCompanyID:      params.CarrierCompanyID,
			OriginLocationID:      orderSnapshot.OriginLocationID,
			DestinationLocationID: orderSnapshot.DestinationLocationID,
			CargoID:               orderSnapshot.CargoID,
			TransportMode:         orderSnapshot.TransportMode,
			PlannedPickupAt:       params.PlannedPickupAt,
			PlannedDeliveryAt:     params.PlannedDeliveryAt,
		}, initialStatus, params.Transition)
		if err != nil {
			var appErr *apperrors.AppError
			if errors.As(err, &appErr) && appErr.Code == apperrors.CodeConflict {
				if existing, findErr := getShipmentByTransportOrderTx(ctx, tx, params.TenantID, params.TransportOrderID); findErr == nil {
					currentStatus, statusErr := getTransportOrderStatusTx(ctx, tx, params.TenantID, params.TransportOrderID)
					if statusErr != nil {
						return statusErr
					}
					result = &ExecuteAwardOrderResult{
						Shipment:    existing,
						OrderStatus: currentStatus,
						Created:     false,
						AwardLink:   *link,
						OrderNumber: orderNumber,
					}
					return tx.Commit(ctx)
				}
			}
			return err
		}

		orderStatus, err = updateTransportOrderStatusTx(ctx, tx, params.TenantID, params.TransportOrderID, orderStatus, domain.TransportOrderStatusConverted)
		if err != nil {
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			return mapDBError(err)
		}
		result = &ExecuteAwardOrderResult{
			Shipment:    shipment,
			OrderStatus: orderStatus,
			Created:     true,
			AwardLink:   *link,
			OrderNumber: orderNumber,
		}
		return nil
	})
	return result, err
}

func (r *OrderExecutionRepository) ListCarrierTransportOrders(ctx context.Context, filter domain.ListCarrierTransportOrdersFilter) ([]domain.CarrierTransportOrderListItem, int, error) {
	var items []domain.CarrierTransportOrderListItem
	var total int
	err := measureDB("order_execution_repository", "list_carrier_transport_orders", func() error {
		countQuery := `
			SELECT COUNT(*)
			FROM rfx.rfx_award_transport_orders l
			JOIN transport.transport_orders o ON o.id = l.transport_order_id AND o.tenant_id = l.tenant_id AND o.deleted_at IS NULL
			WHERE l.tenant_id = $1 AND l.carrier_company_id = $2
		`
		if err := r.pool.QueryRow(ctx, countQuery, filter.TenantID, filter.CarrierCompanyID).Scan(&total); err != nil {
			return mapDBError(err)
		}

		listQuery := `
			SELECT l.id, l.tenant_id, l.rfx_event_id, l.rfx_award_id, l.rfx_response_id, l.rfx_lot_id,
				l.transport_order_id, l.carrier_company_id, l.buyer_company_id,
				l.amount::float8, l.currency_code, l.converted_at,
				o.order_number, o.status,
				s.id, s.status
			FROM rfx.rfx_award_transport_orders l
			JOIN transport.transport_orders o ON o.id = l.transport_order_id AND o.tenant_id = l.tenant_id AND o.deleted_at IS NULL
			LEFT JOIN transport.shipments s ON s.transport_order_id = l.transport_order_id AND s.tenant_id = l.tenant_id AND s.deleted_at IS NULL
			WHERE l.tenant_id = $1 AND l.carrier_company_id = $2
			ORDER BY l.converted_at DESC
			LIMIT $3 OFFSET $4
		`
		rows, err := r.pool.Query(ctx, listQuery, filter.TenantID, filter.CarrierCompanyID, filter.Limit, filter.Offset)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		result := make([]domain.CarrierTransportOrderListItem, 0)
		for rows.Next() {
			var item domain.CarrierTransportOrderListItem
			var lotID *uuid.UUID
			var shipmentID *uuid.UUID
			var shipmentStatus *string
			if err := rows.Scan(
				&item.Link.ID, &item.Link.TenantID, &item.Link.RfxEventID, &item.Link.RfxAwardID, &item.Link.RfxResponseID, &lotID,
				&item.Link.TransportOrderID, &item.Link.CarrierCompanyID, &item.Link.BuyerCompanyID,
				&item.Link.Amount, &item.Link.CurrencyCode, &item.Link.ConvertedAt,
				&item.OrderNumber, &item.OrderStatus,
				&shipmentID, &shipmentStatus,
			); err != nil {
				return mapDBError(err)
			}
			item.Link.RfxLotID = lotID
			item.ShipmentID = shipmentID
			item.ShipmentStatus = shipmentStatus
			result = append(result, item)
		}
		if err := rows.Err(); err != nil {
			return mapDBError(err)
		}
		items = result
		return nil
	})
	return items, total, err
}

func (r *OrderExecutionRepository) ListBuyerTransportOrders(ctx context.Context, filter domain.ListBuyerTransportOrdersFilter) ([]domain.BuyerTransportOrderListItem, int, error) {
	var items []domain.BuyerTransportOrderListItem
	var total int
	err := measureDB("order_execution_repository", "list_buyer_transport_orders", func() error {
		countQuery := `
			SELECT COUNT(*)
			FROM rfx.rfx_award_transport_orders l
			JOIN transport.transport_orders o ON o.id = l.transport_order_id AND o.tenant_id = l.tenant_id AND o.deleted_at IS NULL
			WHERE l.tenant_id = $1 AND l.buyer_company_id = $2
		`
		if err := r.pool.QueryRow(ctx, countQuery, filter.TenantID, filter.BuyerCompanyID).Scan(&total); err != nil {
			return mapDBError(err)
		}

		listQuery := `
			SELECT l.id, l.tenant_id, l.rfx_event_id, l.rfx_award_id, l.rfx_response_id, l.rfx_lot_id,
				l.transport_order_id, l.carrier_company_id, l.buyer_company_id,
				l.amount::float8, l.currency_code, l.converted_at,
				o.order_number, o.status,
				s.id, s.status
			FROM rfx.rfx_award_transport_orders l
			JOIN transport.transport_orders o ON o.id = l.transport_order_id AND o.tenant_id = l.tenant_id AND o.deleted_at IS NULL
			LEFT JOIN transport.shipments s ON s.transport_order_id = l.transport_order_id AND s.tenant_id = l.tenant_id AND s.deleted_at IS NULL
			WHERE l.tenant_id = $1 AND l.buyer_company_id = $2
			ORDER BY l.converted_at DESC
			LIMIT $3 OFFSET $4
		`
		rows, err := r.pool.Query(ctx, listQuery, filter.TenantID, filter.BuyerCompanyID, filter.Limit, filter.Offset)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		result := make([]domain.BuyerTransportOrderListItem, 0)
		for rows.Next() {
			var item domain.BuyerTransportOrderListItem
			var lotID *uuid.UUID
			var shipmentID *uuid.UUID
			var shipmentStatus *string
			if err := rows.Scan(
				&item.Link.ID, &item.Link.TenantID, &item.Link.RfxEventID, &item.Link.RfxAwardID, &item.Link.RfxResponseID, &lotID,
				&item.Link.TransportOrderID, &item.Link.CarrierCompanyID, &item.Link.BuyerCompanyID,
				&item.Link.Amount, &item.Link.CurrencyCode, &item.Link.ConvertedAt,
				&item.OrderNumber, &item.OrderStatus,
				&shipmentID, &shipmentStatus,
			); err != nil {
				return mapDBError(err)
			}
			item.Link.RfxLotID = lotID
			item.ShipmentID = shipmentID
			item.ShipmentStatus = shipmentStatus
			result = append(result, item)
		}
		if err := rows.Err(); err != nil {
			return mapDBError(err)
		}
		items = result
		return nil
	})
	return items, total, err
}

func (r *OrderExecutionRepository) ListShipmentMilestones(ctx context.Context, tenantID, shipmentID uuid.UUID, limit int) ([]domain.ShipmentStatusHistory, error) {
	if limit <= 0 {
		limit = 100
	}
	const query = `
		SELECT id, tenant_id, shipment_id, shipment_version, from_status, to_status,
			reason_code, source, actor_type, actor_id, correlation_id, occurred_at, recorded_at
		FROM transport.shipment_status_history
		WHERE tenant_id = $1 AND shipment_id = $2
		ORDER BY occurred_at ASC, shipment_version ASC
		LIMIT $3
	`
	rows, err := r.pool.Query(ctx, query, tenantID, shipmentID, limit)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	items := make([]domain.ShipmentStatusHistory, 0)
	for rows.Next() {
		var h domain.ShipmentStatusHistory
		if err := rows.Scan(
			&h.ID, &h.TenantID, &h.ShipmentID, &h.ShipmentVersion, &h.FromStatus, &h.ToStatus,
			&h.ReasonCode, &h.Source, &h.ActorType, &h.ActorID, &h.CorrelationID, &h.OccurredAt, &h.RecordedAt,
		); err != nil {
			return nil, mapDBError(err)
		}
		items = append(items, h)
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}
	return items, nil
}

func (r *OrderExecutionRepository) ListShipmentPODDocuments(ctx context.Context, tenantID, shipmentID uuid.UUID) ([]domain.PODDocumentSummary, error) {
	const query = `
		SELECT id, document_number, document_status, created_at
		FROM documents.documents
		WHERE tenant_id = $1 AND related_entity_type = 'SHIPMENT' AND related_entity_id = $2
			AND document_type = 'POD' AND deleted_at IS NULL
		ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, tenantID, shipmentID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	items := make([]domain.PODDocumentSummary, 0)
	for rows.Next() {
		var item domain.PODDocumentSummary
		if err := rows.Scan(&item.ID, &item.DocumentNumber, &item.Status, &item.CreatedAt); err != nil {
			return nil, mapDBError(err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}
	return items, nil
}

func lockAwardLink(ctx context.Context, tx pgx.Tx, tenantID, transportOrderID uuid.UUID) (*domain.AwardTransportOrderLink, error) {
	const query = `
		SELECT id, tenant_id, rfx_event_id, rfx_award_id, rfx_response_id, rfx_lot_id,
			transport_order_id, carrier_company_id, buyer_company_id,
			amount::float8, currency_code, converted_at
		FROM rfx.rfx_award_transport_orders
		WHERE tenant_id = $1 AND transport_order_id = $2
		FOR UPDATE
	`
	var link domain.AwardTransportOrderLink
	var lotID *uuid.UUID
	err := tx.QueryRow(ctx, query, tenantID, transportOrderID).Scan(
		&link.ID, &link.TenantID, &link.RfxEventID, &link.RfxAwardID, &link.RfxResponseID, &lotID,
		&link.TransportOrderID, &link.CarrierCompanyID, &link.BuyerCompanyID,
		&link.Amount, &link.CurrencyCode, &link.ConvertedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("award transport order link not found")
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	link.RfxLotID = lotID
	return &link, nil
}

func getTransportOrderForUpdate(ctx context.Context, tx pgx.Tx, tenantID, orderID uuid.UUID) (orderNumber, status string, err error) {
	const query = `
		SELECT order_number, status
		FROM transport.transport_orders
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		FOR UPDATE
	`
	err = tx.QueryRow(ctx, query, orderID, tenantID).Scan(&orderNumber, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", apperrors.NotFound("transport order not found")
	}
	if err != nil {
		return "", "", mapDBError(err)
	}
	return orderNumber, status, nil
}

func getTransportOrderStatusTx(ctx context.Context, tx pgx.Tx, tenantID, orderID uuid.UUID) (string, error) {
	const query = `SELECT status FROM transport.transport_orders WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
	var status string
	err := tx.QueryRow(ctx, query, orderID, tenantID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperrors.NotFound("transport order not found")
	}
	if err != nil {
		return "", mapDBError(err)
	}
	return status, nil
}

func getTransportOrderSnapshotTx(ctx context.Context, tx pgx.Tx, tenantID, orderID uuid.UUID) (*domain.TransportOrderSnapshot, error) {
	const query = `
		SELECT id, tenant_id, status, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id, transport_mode
		FROM transport.transport_orders
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`
	var order domain.TransportOrderSnapshot
	err := tx.QueryRow(ctx, query, orderID, tenantID).Scan(
		&order.ID, &order.TenantID, &order.Status,
		&order.ShipperCompanyID, &order.ConsigneeCompanyID,
		&order.OriginLocationID, &order.DestinationLocationID,
		&order.CargoID, &order.TransportMode,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("transport order not found")
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return &order, nil
}

func updateTransportOrderStatusTx(ctx context.Context, tx pgx.Tx, tenantID, orderID uuid.UUID, expectedStatus, newStatus string) (string, error) {
	const query = `
		UPDATE transport.transport_orders
		SET status = $1, version = version + 1, updated_at = now()
		WHERE id = $2 AND tenant_id = $3 AND deleted_at IS NULL AND status = $4
		RETURNING status
	`
	var status string
	err := tx.QueryRow(ctx, query, newStatus, orderID, tenantID, expectedStatus).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperrors.Conflict("transport order status changed concurrently", map[string]any{
			"expected_status": expectedStatus,
			"target_status":   newStatus,
		})
	}
	if err != nil {
		return "", mapDBError(err)
	}
	return status, nil
}

func getShipmentByTransportOrderTx(ctx context.Context, tx pgx.Tx, tenantID, transportOrderID uuid.UUID) (*domain.Shipment, error) {
	const query = `
		SELECT id, tenant_id, shipment_number, transport_order_id,
			shipper_company_id, consignee_company_id, carrier_company_id, forwarder_company_id,
			driver_id, vehicle_id, origin_location_id, destination_location_id, cargo_id,
			transport_mode, status, planned_pickup_at, planned_delivery_at,
			actual_pickup_at, actual_delivery_at, created_at, updated_at, version
		FROM transport.shipments
		WHERE tenant_id = $1 AND transport_order_id = $2 AND deleted_at IS NULL
		ORDER BY created_at ASC
		LIMIT 1
	`
	shipment, err := scanShipment(tx.QueryRow(ctx, query, tenantID, transportOrderID))
	if err != nil {
		return nil, err
	}
	return shipment, nil
}

func insertShipmentTx(ctx context.Context, tx pgx.Tx, params CreateShipmentParams, initialStatus string, transition domain.StatusTransitionContext) (*domain.Shipment, error) {
	const query = `
		INSERT INTO transport.shipments (
			tenant_id, shipment_number, transport_order_id,
			shipper_company_id, consignee_company_id, carrier_company_id, forwarder_company_id,
			origin_location_id, destination_location_id, cargo_id, transport_mode,
			status, planned_pickup_at, planned_delivery_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, tenant_id, shipment_number, transport_order_id,
			shipper_company_id, consignee_company_id, carrier_company_id, forwarder_company_id,
			driver_id, vehicle_id, origin_location_id, destination_location_id, cargo_id,
			transport_mode, status, planned_pickup_at, planned_delivery_at,
			actual_pickup_at, actual_delivery_at, created_at, updated_at, version
	`
	shipment, err := scanShipment(tx.QueryRow(ctx, query,
		params.TenantID,
		strings.TrimSpace(params.ShipmentNumber),
		params.TransportOrderID,
		params.ShipperCompanyID,
		params.ConsigneeCompanyID,
		params.CarrierCompanyID,
		optionalUUID(params.ForwarderCompanyID),
		params.OriginLocationID,
		params.DestinationLocationID,
		optionalUUID(params.CargoID),
		params.TransportMode,
		initialStatus,
		optionalTime(params.PlannedPickupAt),
		optionalTime(params.PlannedDeliveryAt),
	))
	if err != nil {
		return nil, mapDBError(err)
	}

	write := statusHistoryWriteFromShipmentTransition(
		shipment,
		nil,
		shipment.Status,
		transition,
	)
	if err := insertStatusHistoryAndOutbox(ctx, tx, write); err != nil {
		return nil, err
	}
	return shipment, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isNotFound(err error) bool {
	for err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}

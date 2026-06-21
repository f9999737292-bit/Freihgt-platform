package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type ShipmentRepository struct {
	pool *pgxpool.Pool
}

func NewShipmentRepository(pool *pgxpool.Pool) *ShipmentRepository {
	return &ShipmentRepository{pool: pool}
}

func (r *ShipmentRepository) CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM core.companies
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		)
	`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, companyID, tenantID).Scan(&exists); err != nil {
		return false, mapDBError(err)
	}
	return exists, nil
}

func (r *ShipmentRepository) GetTransportOrder(ctx context.Context, id, tenantID uuid.UUID) (*domain.TransportOrderSnapshot, error) {
	const query = `
		SELECT id, tenant_id, status, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id, transport_mode
		FROM transport.transport_orders
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`
	var order domain.TransportOrderSnapshot
	err := r.pool.QueryRow(ctx, query, id, tenantID).Scan(
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

func (r *ShipmentRepository) GetBid(ctx context.Context, id, tenantID uuid.UUID) (*domain.BidSnapshot, error) {
	const query = `
		SELECT id, tenant_id, status, carrier_company_id
		FROM rfx.bids
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`
	var bid domain.BidSnapshot
	err := r.pool.QueryRow(ctx, query, id, tenantID).Scan(
		&bid.ID, &bid.TenantID, &bid.Status, &bid.CarrierCompanyID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("bid not found")
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return &bid, nil
}

type CreateShipmentParams struct {
	TenantID              uuid.UUID
	ShipmentNumber        string
	TransportOrderID      uuid.UUID
	ShipperCompanyID      uuid.UUID
	ConsigneeCompanyID    uuid.UUID
	CarrierCompanyID      uuid.UUID
	ForwarderCompanyID    *uuid.UUID
	OriginLocationID      uuid.UUID
	DestinationLocationID uuid.UUID
	CargoID               *uuid.UUID
	TransportMode         string
	PlannedPickupAt       *time.Time
	PlannedDeliveryAt     *time.Time
}

func (r *ShipmentRepository) CreateShipment(ctx context.Context, params CreateShipmentParams) (*domain.Shipment, error) {
	var result *domain.Shipment
	err := measureDB("shipment_repository", "create_shipment", func() error {
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
		row := r.pool.QueryRow(ctx, query,
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
			domain.ShipmentStatusCarrierAssigned,
			optionalTime(params.PlannedPickupAt),
			optionalTime(params.PlannedDeliveryAt),
		)
		shipment, err := scanShipment(row)
		if err != nil {
			return err
		}
		result = shipment
		return nil
	})
	return result, err
}

func (r *ShipmentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Shipment, error) {
	var result *domain.Shipment
	err := measureDB("shipment_repository", "get_shipment", func() error {
		const query = `
		SELECT id, tenant_id, shipment_number, transport_order_id,
			shipper_company_id, consignee_company_id, carrier_company_id, forwarder_company_id,
			driver_id, vehicle_id, origin_location_id, destination_location_id, cargo_id,
			transport_mode, status, planned_pickup_at, planned_delivery_at,
			actual_pickup_at, actual_delivery_at, created_at, updated_at, version
		FROM transport.shipments
		WHERE id = $1 AND deleted_at IS NULL
	`
		shipment, err := scanShipment(r.pool.QueryRow(ctx, query, id))
		if err != nil {
			return err
		}
		result = shipment
		return nil
	})
	return result, err
}

func (r *ShipmentRepository) GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.Shipment, error) {
	var result *domain.Shipment
	err := measureDB("shipment_repository", "get_shipment", func() error {
		const query = `
		SELECT id, tenant_id, shipment_number, transport_order_id,
			shipper_company_id, consignee_company_id, carrier_company_id, forwarder_company_id,
			driver_id, vehicle_id, origin_location_id, destination_location_id, cargo_id,
			transport_mode, status, planned_pickup_at, planned_delivery_at,
			actual_pickup_at, actual_delivery_at, created_at, updated_at, version
		FROM transport.shipments
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`
		shipment, err := scanShipment(r.pool.QueryRow(ctx, query, id, tenantID))
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NotFound("shipment not found")
		}
		if err != nil {
			return err
		}
		result = shipment
		return nil
	})
	return result, err
}

func (r *ShipmentRepository) List(ctx context.Context, filter domain.ListShipmentsFilter) ([]domain.Shipment, int, error) {
	var shipments []domain.Shipment
	var total int
	err := measureDB("shipment_repository", "list_shipments", func() error {
		args := []any{filter.TenantID}
		where := []string{"tenant_id = $1", "deleted_at IS NULL"}

		if filter.ShipperCompanyID != nil {
			args = append(args, *filter.ShipperCompanyID)
			where = append(where, fmt.Sprintf("shipper_company_id = $%d", len(args)))
		}
		if filter.ConsigneeCompanyID != nil {
			args = append(args, *filter.ConsigneeCompanyID)
			where = append(where, fmt.Sprintf("consignee_company_id = $%d", len(args)))
		}
		if filter.CarrierCompanyID != nil {
			args = append(args, *filter.CarrierCompanyID)
			where = append(where, fmt.Sprintf("carrier_company_id = $%d", len(args)))
		}
		if filter.Status != nil {
			args = append(args, *filter.Status)
			where = append(where, fmt.Sprintf("status = $%d", len(args)))
		}

		whereClause := strings.Join(where, " AND ")
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM transport.shipments WHERE %s", whereClause)
		if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
			return mapDBError(err)
		}

		args = append(args, filter.Limit, filter.Offset)
		listQuery := fmt.Sprintf(`
		SELECT id, tenant_id, shipment_number, transport_order_id,
			shipper_company_id, consignee_company_id, carrier_company_id, forwarder_company_id,
			driver_id, vehicle_id, origin_location_id, destination_location_id, cargo_id,
			transport_mode, status, planned_pickup_at, planned_delivery_at,
			actual_pickup_at, actual_delivery_at, created_at, updated_at, version
		FROM transport.shipments
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, len(args)-1, len(args))

		rows, err := r.pool.Query(ctx, listQuery, args...)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		items := make([]domain.Shipment, 0)
		for rows.Next() {
			shipment, err := scanShipmentRows(rows)
			if err != nil {
				return err
			}
			items = append(items, *shipment)
		}
		if err := rows.Err(); err != nil {
			return mapDBError(err)
		}
		shipments = items
		return nil
	})
	return shipments, total, err
}

func (r *ShipmentRepository) AssignDriver(ctx context.Context, id, tenantID, driverID uuid.UUID, newStatus string, expectedVersion int) (*domain.Shipment, error) {
	var result *domain.Shipment
	err := measureDB("shipment_repository", "assign_driver", func() error {
		const query = `
		UPDATE transport.shipments
		SET driver_id = $1, status = $2, version = version + 1, updated_at = now()
		WHERE id = $3 AND tenant_id = $4 AND deleted_at IS NULL AND version = $5
		RETURNING id, tenant_id, shipment_number, transport_order_id,
			shipper_company_id, consignee_company_id, carrier_company_id, forwarder_company_id,
			driver_id, vehicle_id, origin_location_id, destination_location_id, cargo_id,
			transport_mode, status, planned_pickup_at, planned_delivery_at,
			actual_pickup_at, actual_delivery_at, created_at, updated_at, version
	`
		shipment, err := scanShipmentUpdate(r.pool.QueryRow(ctx, query, driverID, newStatus, id, tenantID, expectedVersion))
		if err != nil {
			return err
		}
		result = shipment
		return nil
	})
	return result, err
}

func (r *ShipmentRepository) AssignVehicle(ctx context.Context, id, tenantID, vehicleID uuid.UUID, newStatus string, expectedVersion int) (*domain.Shipment, error) {
	var result *domain.Shipment
	err := measureDB("shipment_repository", "assign_vehicle", func() error {
		const query = `
		UPDATE transport.shipments
		SET vehicle_id = $1, status = $2, version = version + 1, updated_at = now()
		WHERE id = $3 AND tenant_id = $4 AND deleted_at IS NULL AND version = $5
		RETURNING id, tenant_id, shipment_number, transport_order_id,
			shipper_company_id, consignee_company_id, carrier_company_id, forwarder_company_id,
			driver_id, vehicle_id, origin_location_id, destination_location_id, cargo_id,
			transport_mode, status, planned_pickup_at, planned_delivery_at,
			actual_pickup_at, actual_delivery_at, created_at, updated_at, version
	`
		shipment, err := scanShipmentUpdate(r.pool.QueryRow(ctx, query, vehicleID, newStatus, id, tenantID, expectedVersion))
		if err != nil {
			return err
		}
		result = shipment
		return nil
	})
	return result, err
}

func (r *ShipmentRepository) UpdateStatus(ctx context.Context, id, tenantID uuid.UUID, newStatus string, actualPickupAt, actualDeliveryAt *time.Time, expectedVersion int) (*domain.Shipment, error) {
	var result *domain.Shipment
	err := measureDB("shipment_repository", "update_shipment_status", func() error {
		const query = `
		UPDATE transport.shipments
		SET status = $1,
			actual_pickup_at = COALESCE($2, actual_pickup_at),
			actual_delivery_at = COALESCE($3, actual_delivery_at),
			version = version + 1,
			updated_at = now()
		WHERE id = $4 AND tenant_id = $5 AND deleted_at IS NULL AND version = $6
		RETURNING id, tenant_id, shipment_number, transport_order_id,
			shipper_company_id, consignee_company_id, carrier_company_id, forwarder_company_id,
			driver_id, vehicle_id, origin_location_id, destination_location_id, cargo_id,
			transport_mode, status, planned_pickup_at, planned_delivery_at,
			actual_pickup_at, actual_delivery_at, created_at, updated_at, version
	`
		shipment, err := scanShipmentUpdate(r.pool.QueryRow(ctx, query,
			newStatus,
			optionalTime(actualPickupAt),
			optionalTime(actualDeliveryAt),
			id, tenantID, expectedVersion,
		))
		if err != nil {
			return err
		}
		result = shipment
		return nil
	})
	return result, err
}

func (r *ShipmentRepository) Accept(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) (*domain.Shipment, error) {
	var result *domain.Shipment
	err := measureDB("shipment_repository", "accept_shipment", func() error {
		const query = `
		UPDATE transport.shipments
		SET status = $1, version = version + 1, updated_at = now()
		WHERE id = $2 AND tenant_id = $3 AND deleted_at IS NULL AND version = $4
		RETURNING id, tenant_id, shipment_number, transport_order_id,
			shipper_company_id, consignee_company_id, carrier_company_id, forwarder_company_id,
			driver_id, vehicle_id, origin_location_id, destination_location_id, cargo_id,
			transport_mode, status, planned_pickup_at, planned_delivery_at,
			actual_pickup_at, actual_delivery_at, created_at, updated_at, version
	`
		shipment, err := scanShipmentUpdate(r.pool.QueryRow(ctx, query, domain.ShipmentStatusAcceptedByCarrier, id, tenantID, expectedVersion))
		if err != nil {
			return err
		}
		result = shipment
		return nil
	})
	return result, err
}

func (r *ShipmentRepository) Cancel(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) (*domain.Shipment, error) {
	var result *domain.Shipment
	err := measureDB("shipment_repository", "cancel_shipment", func() error {
		const query = `
		UPDATE transport.shipments
		SET status = $1, version = version + 1, updated_at = now()
		WHERE id = $2 AND tenant_id = $3 AND deleted_at IS NULL AND version = $4
		RETURNING id, tenant_id, shipment_number, transport_order_id,
			shipper_company_id, consignee_company_id, carrier_company_id, forwarder_company_id,
			driver_id, vehicle_id, origin_location_id, destination_location_id, cargo_id,
			transport_mode, status, planned_pickup_at, planned_delivery_at,
			actual_pickup_at, actual_delivery_at, created_at, updated_at, version
	`
		shipment, err := scanShipmentUpdate(r.pool.QueryRow(ctx, query, domain.ShipmentStatusCancelled, id, tenantID, expectedVersion))
		if err != nil {
			return err
		}
		result = shipment
		return nil
	})
	return result, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanShipment(row rowScanner) (*domain.Shipment, error) {
	var s domain.Shipment
	err := row.Scan(
		&s.ID, &s.TenantID, &s.ShipmentNumber, &s.TransportOrderID,
		&s.ShipperCompanyID, &s.ConsigneeCompanyID, &s.CarrierCompanyID, &s.ForwarderCompanyID,
		&s.DriverID, &s.VehicleID, &s.OriginLocationID, &s.DestinationLocationID, &s.CargoID,
		&s.TransportMode, &s.Status, &s.PlannedPickupAt, &s.PlannedDeliveryAt,
		&s.ActualPickupAt, &s.ActualDeliveryAt, &s.CreatedAt, &s.UpdatedAt, &s.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("shipment not found")
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return &s, nil
}

func scanShipmentRows(rows pgx.Rows) (*domain.Shipment, error) {
	var s domain.Shipment
	err := rows.Scan(
		&s.ID, &s.TenantID, &s.ShipmentNumber, &s.TransportOrderID,
		&s.ShipperCompanyID, &s.ConsigneeCompanyID, &s.CarrierCompanyID, &s.ForwarderCompanyID,
		&s.DriverID, &s.VehicleID, &s.OriginLocationID, &s.DestinationLocationID, &s.CargoID,
		&s.TransportMode, &s.Status, &s.PlannedPickupAt, &s.PlannedDeliveryAt,
		&s.ActualPickupAt, &s.ActualDeliveryAt, &s.CreatedAt, &s.UpdatedAt, &s.Version,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &s, nil
}

func scanShipmentUpdate(row rowScanner) (*domain.Shipment, error) {
	shipment, err := scanShipment(row)
	if err != nil {
		return nil, err
	}
	return shipment, nil
}

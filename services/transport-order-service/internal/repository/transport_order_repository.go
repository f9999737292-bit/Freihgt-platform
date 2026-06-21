package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/transport-order-service/internal/domain"
	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
)

type TransportOrderRepository struct {
	pool *pgxpool.Pool
}

func NewTransportOrderRepository(pool *pgxpool.Pool) *TransportOrderRepository {
	return &TransportOrderRepository{pool: pool}
}

func (r *TransportOrderRepository) CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error) {
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

func (r *TransportOrderRepository) Create(ctx context.Context, in domain.CreateTransportOrderInput) (*domain.TransportOrder, error) {
	var result *domain.TransportOrder
	err := measureDB("transport_order_repository", "create_transport_order", func() error {
		const query = `
		INSERT INTO transport.transport_orders (
			tenant_id, order_number, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id,
			requested_pickup_date, requested_delivery_date,
			transport_mode, equipment_type, status, source_system, external_reference
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, tenant_id, order_number, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id,
			requested_pickup_date, requested_delivery_date,
			transport_mode, equipment_type, status, source_system, external_reference,
			created_at, updated_at, version
	`
		row := r.pool.QueryRow(ctx, query,
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
			optionalString(in.SourceSystem),
			optionalString(in.ExternalReference),
		)
		order, err := scanTransportOrder(row)
		if err != nil {
			if pgErr := conflictFromError(err); pgErr != nil {
				return pgErr
			}
			return mapDBError(err)
		}
		result = order
		return nil
	})
	return result, err
}

func (r *TransportOrderRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.TransportOrder, error) {
	var result *domain.TransportOrder
	err := measureDB("transport_order_repository", "get_transport_order", func() error {
		const query = `
		SELECT id, tenant_id, order_number, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id,
			requested_pickup_date, requested_delivery_date,
			transport_mode, equipment_type, status, source_system, external_reference,
			created_at, updated_at, version
		FROM transport.transport_orders
		WHERE id = $1 AND deleted_at IS NULL
	`
		row := r.pool.QueryRow(ctx, query, id)
		order, err := scanTransportOrder(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperrors.NotFound("transport order not found")
			}
			return mapDBError(err)
		}
		result = order
		return nil
	})
	return result, err
}

func (r *TransportOrderRepository) List(ctx context.Context, filter domain.ListTransportOrdersFilter) ([]domain.TransportOrder, int, error) {
	var items []domain.TransportOrder
	var total int
	err := measureDB("transport_order_repository", "list_transport_orders", func() error {
		where := strings.Builder{}
		where.WriteString(" FROM transport.transport_orders WHERE tenant_id = $1 AND deleted_at IS NULL")
		args := []any{filter.TenantID}
		argIdx := 2

		if filter.ShipperCompanyID != nil {
			where.WriteString(fmt.Sprintf(" AND shipper_company_id = $%d", argIdx))
			args = append(args, *filter.ShipperCompanyID)
			argIdx++
		}
		if filter.ConsigneeCompanyID != nil {
			where.WriteString(fmt.Sprintf(" AND consignee_company_id = $%d", argIdx))
			args = append(args, *filter.ConsigneeCompanyID)
			argIdx++
		}
		if filter.Status != nil {
			where.WriteString(fmt.Sprintf(" AND status = $%d", argIdx))
			args = append(args, *filter.Status)
			argIdx++
		}

		countQuery := "SELECT COUNT(*)" + where.String()
		if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
			return mapDBError(err)
		}

		listQuery := `
		SELECT id, tenant_id, order_number, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id,
			requested_pickup_date, requested_delivery_date,
			transport_mode, equipment_type, status, source_system, external_reference,
			created_at, updated_at, version
	` + where.String() + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
		args = append(args, filter.Limit, filter.Offset)

		rows, err := r.pool.Query(ctx, listQuery, args...)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		orders := make([]domain.TransportOrder, 0)
		for rows.Next() {
			order, err := scanTransportOrder(rows)
			if err != nil {
				return mapDBError(err)
			}
			orders = append(orders, *order)
		}
		if err := rows.Err(); err != nil {
			return mapDBError(err)
		}
		items = orders
		return nil
	})
	return items, total, err
}

func (r *TransportOrderRepository) Update(ctx context.Context, id uuid.UUID, in domain.UpdateTransportOrderInput) (*domain.TransportOrder, error) {
	var result *domain.TransportOrder
	err := measureDB("transport_order_repository", "update_transport_order", func() error {
		current, err := r.GetByID(ctx, id)
		if err != nil {
			return err
		}

		pickup := current.RequestedPickupDate
		if in.RequestedPickupDate != nil {
			pickup = in.RequestedPickupDate
		}
		delivery := current.RequestedDeliveryDate
		if in.RequestedDeliveryDate != nil {
			delivery = in.RequestedDeliveryDate
		}
		equipment := current.EquipmentType
		if in.EquipmentType != nil {
			equipment = stringPtr(strings.TrimSpace(*in.EquipmentType))
		}
		mode := current.TransportMode
		if in.TransportMode != nil {
			mode = domain.NormalizeTransportMode(*in.TransportMode)
		}

		const query = `
		UPDATE transport.transport_orders SET
			requested_pickup_date = $2,
			requested_delivery_date = $3,
			equipment_type = $4,
			transport_mode = $5,
			updated_at = now(),
			version = version + 1
		WHERE id = $1 AND deleted_at IS NULL AND status = $6 AND version = $7
		RETURNING id, tenant_id, order_number, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id,
			requested_pickup_date, requested_delivery_date,
			transport_mode, equipment_type, status, source_system, external_reference,
			created_at, updated_at, version
	`
		row := r.pool.QueryRow(ctx, query,
			id,
			optionalDate(pickup),
			optionalDate(delivery),
			optionalString(equipment),
			mode,
			domain.TransportOrderStatusDraft,
			current.Version,
		)
		order, err := scanTransportOrder(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperrors.Conflict("transport order was updated by another request", map[string]any{"field": "version"})
			}
			return mapDBError(err)
		}
		result = order
		return nil
	})
	return result, err
}

func (r *TransportOrderRepository) UpdateStatus(ctx context.Context, id uuid.UUID, expectedStatus, newStatus string) (*domain.TransportOrder, error) {
	operation := transportOrderStatusOperation(expectedStatus, newStatus)
	var result *domain.TransportOrder
	err := measureDB("transport_order_repository", operation, func() error {
		current, err := r.GetByID(ctx, id)
		if err != nil {
			return err
		}

		const query = `
		UPDATE transport.transport_orders SET
			status = $3,
			updated_at = now(),
			version = version + 1
		WHERE id = $1 AND deleted_at IS NULL AND status = $2 AND version = $4
		RETURNING id, tenant_id, order_number, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id,
			requested_pickup_date, requested_delivery_date,
			transport_mode, equipment_type, status, source_system, external_reference,
			created_at, updated_at, version
	`
		row := r.pool.QueryRow(ctx, query, id, expectedStatus, newStatus, current.Version)
		order, err := scanTransportOrder(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperrors.Conflict("transport order was updated by another request", map[string]any{"field": "version"})
			}
			return mapDBError(err)
		}
		result = order
		return nil
	})
	return result, err
}

func transportOrderStatusOperation(expectedStatus, newStatus string) string {
	if expectedStatus == domain.TransportOrderStatusDraft && newStatus == domain.TransportOrderStatusReadyForSourcing {
		return "submit_transport_order"
	}
	if newStatus == domain.TransportOrderStatusCancelled {
		return "cancel_transport_order"
	}
	return "update_transport_order_status"
}

func scanTransportOrder(row pgx.Row) (*domain.TransportOrder, error) {
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

func optionalDate(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func conflictFromError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return apperrors.Conflict("order_number already exists", map[string]any{"field": "order_number"})
	}
	return nil
}

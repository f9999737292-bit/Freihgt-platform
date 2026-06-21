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

type FreightRequestRepository struct {
	pool *pgxpool.Pool
}

func NewFreightRequestRepository(pool *pgxpool.Pool) *FreightRequestRepository {
	return &FreightRequestRepository{pool: pool}
}

func (r *FreightRequestRepository) CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error) {
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

func (r *FreightRequestRepository) GetTransportOrder(ctx context.Context, id, tenantID uuid.UUID) (string, error) {
	const query = `
		SELECT status
		FROM transport.transport_orders
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`
	var status string
	if err := r.pool.QueryRow(ctx, query, id, tenantID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperrors.NotFound("transport order not found")
		}
		return "", mapDBError(err)
	}
	return status, nil
}

func (r *FreightRequestRepository) CreateFromTransportOrder(ctx context.Context, in domain.CreateFreightRequestFromOrderInput) (*domain.FreightRequest, error) {
	var result *domain.FreightRequest
	err := measureDB("freight_request_repository", "create_freight_request", func() error {
		const query = `
		INSERT INTO rfx.freight_requests (
			tenant_id, freight_request_number, transport_order_id, request_type,
			shipper_company_id, response_deadline, currency_code, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, tenant_id, freight_request_number, transport_order_id, request_type,
			shipper_company_id, status, response_deadline, currency_code,
			created_at, updated_at, version
	`
		row := r.pool.QueryRow(ctx, query,
			in.TenantID,
			strings.TrimSpace(in.FreightRequestNumber),
			in.TransportOrderID,
			strings.TrimSpace(in.RequestType),
			in.ShipperCompanyID,
			in.ResponseDeadline,
			optionalString(in.CurrencyCode),
			domain.FreightRequestStatusDraft,
		)
		request, err := scanFreightRequest(row)
		if err != nil {
			return mapDBError(err)
		}
		result = request
		return nil
	})
	return result, err
}

func (r *FreightRequestRepository) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.FreightRequest, error) {
	var result *domain.FreightRequest
	err := measureDB("freight_request_repository", "get_freight_request", func() error {
		const query = `
		SELECT id, tenant_id, freight_request_number, transport_order_id, request_type,
			shipper_company_id, status, response_deadline, currency_code,
			created_at, updated_at, version
		FROM rfx.freight_requests
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`
		row := r.pool.QueryRow(ctx, query, id, tenantID)
		request, err := scanFreightRequest(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperrors.NotFound("freight request not found")
			}
			return mapDBError(err)
		}
		result = request
		return nil
	})
	return result, err
}

func (r *FreightRequestRepository) List(ctx context.Context, filter domain.ListFreightRequestsFilter) ([]domain.FreightRequest, int, error) {
	var requests []domain.FreightRequest
	var total int
	err := measureDB("freight_request_repository", "list_freight_requests", func() error {
		where := strings.Builder{}
		where.WriteString(" FROM rfx.freight_requests WHERE tenant_id = $1 AND deleted_at IS NULL")
		args := []any{filter.TenantID}
		argIdx := 2

		if filter.RequestType != nil {
			where.WriteString(fmt.Sprintf(" AND request_type = $%d", argIdx))
			args = append(args, *filter.RequestType)
			argIdx++
		}
		if filter.Status != nil {
			where.WriteString(fmt.Sprintf(" AND status = $%d", argIdx))
			args = append(args, *filter.Status)
			argIdx++
		}
		if filter.ShipperCompanyID != nil {
			where.WriteString(fmt.Sprintf(" AND shipper_company_id = $%d", argIdx))
			args = append(args, *filter.ShipperCompanyID)
			argIdx++
		}

		countQuery := "SELECT COUNT(*)" + where.String()
		if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
			return mapDBError(err)
		}

		listQuery := `
		SELECT id, tenant_id, freight_request_number, transport_order_id, request_type,
			shipper_company_id, status, response_deadline, currency_code,
			created_at, updated_at, version
	` + where.String() + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
		args = append(args, filter.Limit, filter.Offset)

		rows, err := r.pool.Query(ctx, listQuery, args...)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		requests = make([]domain.FreightRequest, 0)
		for rows.Next() {
			request, err := scanFreightRequest(rows)
			if err != nil {
				return mapDBError(err)
			}
			requests = append(requests, *request)
		}
		if err := rows.Err(); err != nil {
			return mapDBError(err)
		}
		return nil
	})
	return requests, total, err
}

func (r *FreightRequestRepository) UpdateStatus(ctx context.Context, id, tenantID uuid.UUID, expectedStatus, newStatus string) (*domain.FreightRequest, error) {
	operation := "update_freight_request_status"
	if newStatus == domain.FreightRequestStatusPublished {
		operation = "publish_freight_request"
	}

	var result *domain.FreightRequest
	err := measureDB("freight_request_repository", operation, func() error {
		current, err := r.GetByID(ctx, id, tenantID)
		if err != nil {
			return err
		}

		const query = `
		UPDATE rfx.freight_requests SET
			status = $4,
			updated_at = now(),
			version = version + 1
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL AND status = $3 AND version = $5
		RETURNING id, tenant_id, freight_request_number, transport_order_id, request_type,
			shipper_company_id, status, response_deadline, currency_code,
			created_at, updated_at, version
	`
		row := r.pool.QueryRow(ctx, query, id, tenantID, expectedStatus, newStatus, current.Version)
		request, err := scanFreightRequest(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperrors.Conflict("freight request was updated by another request", map[string]any{"field": "version"})
			}
			return mapDBError(err)
		}
		result = request
		return nil
	})
	return result, err
}

func scanFreightRequest(row pgx.Row) (*domain.FreightRequest, error) {
	var request domain.FreightRequest
	err := row.Scan(
		&request.ID,
		&request.TenantID,
		&request.FreightRequestNumber,
		&request.TransportOrderID,
		&request.RequestType,
		&request.ShipperCompanyID,
		&request.Status,
		&request.ResponseDeadline,
		&request.CurrencyCode,
		&request.CreatedAt,
		&request.UpdatedAt,
		&request.Version,
	)
	if err != nil {
		return nil, err
	}
	return &request, nil
}

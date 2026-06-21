package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/transport-order-service/internal/domain"
	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
)

type LocationRepository struct {
	pool *pgxpool.Pool
}

func NewLocationRepository(pool *pgxpool.Pool) *LocationRepository {
	return &LocationRepository{pool: pool}
}

func (r *LocationRepository) CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error) {
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

func (r *LocationRepository) Create(ctx context.Context, in domain.CreateLocationInput) (*domain.Location, error) {
	var result *domain.Location
	err := measureDB("location_repository", "create_location", func() error {
		const query = `
		INSERT INTO transport.locations (
			tenant_id, company_id, location_type, name, country_code,
			region, city, address_line, postal_code, lat, lon, timezone, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, 'ACTIVE')
		RETURNING id, tenant_id, company_id, location_type, name, country_code,
			region, city, address_line, postal_code, lat, lon, timezone, status,
			created_at, updated_at, version
	`
		row := r.pool.QueryRow(ctx, query,
			in.TenantID,
			optionalUUID(in.CompanyID),
			strings.TrimSpace(in.LocationType),
			strings.TrimSpace(in.Name),
			domain.NormalizeCountryCode(in.CountryCode),
			optionalString(in.Region),
			optionalString(in.City),
			optionalString(in.AddressLine),
			optionalString(in.PostalCode),
			optionalFloat(in.Lat),
			optionalFloat(in.Lon),
			domain.NormalizeTimezone(in.Timezone),
		)
		location, err := scanLocation(row)
		if err != nil {
			return mapDBError(err)
		}
		result = location
		return nil
	})
	return result, err
}

func (r *LocationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Location, error) {
	var result *domain.Location
	err := measureDB("location_repository", "get_location", func() error {
		const query = `
		SELECT id, tenant_id, company_id, location_type, name, country_code,
			region, city, address_line, postal_code, lat, lon, timezone, status,
			created_at, updated_at, version
		FROM transport.locations
		WHERE id = $1 AND deleted_at IS NULL
	`
		row := r.pool.QueryRow(ctx, query, id)
		location, err := scanLocation(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperrors.NotFound("location not found")
			}
			return mapDBError(err)
		}
		result = location
		return nil
	})
	return result, err
}

func (r *LocationRepository) ExistsInTenant(ctx context.Context, id, tenantID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM transport.locations
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		)
	`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, id, tenantID).Scan(&exists); err != nil {
		return false, mapDBError(err)
	}
	return exists, nil
}

func (r *LocationRepository) List(ctx context.Context, filter domain.ListLocationsFilter) ([]domain.Location, int, error) {
	var items []domain.Location
	var total int
	err := measureDB("location_repository", "list_locations", func() error {
		where := strings.Builder{}
		where.WriteString(" FROM transport.locations WHERE tenant_id = $1 AND deleted_at IS NULL")
		args := []any{filter.TenantID}
		argIdx := 2

		if filter.CompanyID != nil {
			where.WriteString(fmt.Sprintf(" AND company_id = $%d", argIdx))
			args = append(args, *filter.CompanyID)
			argIdx++
		}
		if filter.LocationType != nil {
			where.WriteString(fmt.Sprintf(" AND location_type = $%d", argIdx))
			args = append(args, *filter.LocationType)
			argIdx++
		}
		if filter.Search != nil && strings.TrimSpace(*filter.Search) != "" {
			where.WriteString(fmt.Sprintf(" AND (name ILIKE $%d OR city ILIKE $%d OR address_line ILIKE $%d)", argIdx, argIdx, argIdx))
			args = append(args, "%"+strings.TrimSpace(*filter.Search)+"%")
			argIdx++
		}

		countQuery := "SELECT COUNT(*)" + where.String()
		if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
			return mapDBError(err)
		}

		listQuery := `
		SELECT id, tenant_id, company_id, location_type, name, country_code,
			region, city, address_line, postal_code, lat, lon, timezone, status,
			created_at, updated_at, version
	` + where.String() + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
		args = append(args, filter.Limit, filter.Offset)

		rows, err := r.pool.Query(ctx, listQuery, args...)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		locations := make([]domain.Location, 0)
		for rows.Next() {
			location, err := scanLocation(rows)
			if err != nil {
				return mapDBError(err)
			}
			locations = append(locations, *location)
		}
		if err := rows.Err(); err != nil {
			return mapDBError(err)
		}
		items = locations
		return nil
	})
	return items, total, err
}

func scanLocation(row pgx.Row) (*domain.Location, error) {
	var location domain.Location
	err := row.Scan(
		&location.ID,
		&location.TenantID,
		&location.CompanyID,
		&location.LocationType,
		&location.Name,
		&location.CountryCode,
		&location.Region,
		&location.City,
		&location.AddressLine,
		&location.PostalCode,
		&location.Lat,
		&location.Lon,
		&location.Timezone,
		&location.Status,
		&location.CreatedAt,
		&location.UpdatedAt,
		&location.Version,
	)
	if err != nil {
		return nil, err
	}
	return &location, nil
}

func optionalUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

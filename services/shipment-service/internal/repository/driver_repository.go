package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type DriverRepository struct {
	pool *pgxpool.Pool
}

func NewDriverRepository(pool *pgxpool.Pool) *DriverRepository {
	return &DriverRepository{pool: pool}
}

func (r *DriverRepository) CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error) {
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

func (r *DriverRepository) Create(ctx context.Context, in domain.CreateDriverInput) (*domain.Driver, error) {
	var result *domain.Driver
	err := measureDB("driver_repository", "create_driver", func() error {
		const query = `
		INSERT INTO transport.drivers (
			tenant_id, carrier_company_id, user_id, full_name, phone,
			license_number, license_country, preferred_locale, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, tenant_id, carrier_company_id, user_id, full_name, phone,
			license_number, license_country, preferred_locale, status
	`
		row := r.pool.QueryRow(ctx, query,
			in.TenantID,
			in.CarrierCompanyID,
			optionalUUID(in.UserID),
			strings.TrimSpace(in.FullName),
			optionalString(in.Phone),
			optionalString(in.LicenseNumber),
			domain.NormalizeCountryCode(in.LicenseCountry),
			domain.NormalizeTimezone(in.PreferredLocale),
			domain.DriverStatusActive,
		)
		driver, err := scanDriver(row)
		if err != nil {
			return err
		}
		result = driver
		return nil
	})
	return result, err
}

func (r *DriverRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Driver, error) {
	const query = `
		SELECT id, tenant_id, carrier_company_id, user_id, full_name, phone,
			license_number, license_country, preferred_locale, status
		FROM transport.drivers
		WHERE id = $1 AND deleted_at IS NULL
	`
	return scanDriver(r.pool.QueryRow(ctx, query, id))
}

func (r *DriverRepository) List(ctx context.Context, filter domain.ListDriversFilter) ([]domain.Driver, int, error) {
	var drivers []domain.Driver
	var total int
	err := measureDB("driver_repository", "list_drivers", func() error {
		args := []any{filter.TenantID}
		where := []string{"tenant_id = $1", "deleted_at IS NULL"}

		if filter.CarrierCompanyID != nil {
			args = append(args, *filter.CarrierCompanyID)
			where = append(where, fmt.Sprintf("carrier_company_id = $%d", len(args)))
		}
		if filter.Status != nil {
			args = append(args, *filter.Status)
			where = append(where, fmt.Sprintf("status = $%d", len(args)))
		}

		whereClause := strings.Join(where, " AND ")
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM transport.drivers WHERE %s", whereClause)
		if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
			return mapDBError(err)
		}

		args = append(args, filter.Limit, filter.Offset)
		listQuery := fmt.Sprintf(`
		SELECT id, tenant_id, carrier_company_id, user_id, full_name, phone,
			license_number, license_country, preferred_locale, status
		FROM transport.drivers
		WHERE %s
		ORDER BY full_name ASC
		LIMIT $%d OFFSET $%d
	`, whereClause, len(args)-1, len(args))

		rows, err := r.pool.Query(ctx, listQuery, args...)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		items := make([]domain.Driver, 0)
		for rows.Next() {
			driver, err := scanDriverRows(rows)
			if err != nil {
				return err
			}
			items = append(items, *driver)
		}
		if err := rows.Err(); err != nil {
			return mapDBError(err)
		}
		drivers = items
		return nil
	})
	return drivers, total, err
}

func scanDriver(row pgx.Row) (*domain.Driver, error) {
	var d domain.Driver
	err := row.Scan(
		&d.ID, &d.TenantID, &d.CarrierCompanyID, &d.UserID, &d.FullName, &d.Phone,
		&d.LicenseNumber, &d.LicenseCountry, &d.PreferredLocale, &d.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("driver not found")
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return &d, nil
}

func scanDriverRows(rows pgx.Rows) (*domain.Driver, error) {
	var d domain.Driver
	err := rows.Scan(
		&d.ID, &d.TenantID, &d.CarrierCompanyID, &d.UserID, &d.FullName, &d.Phone,
		&d.LicenseNumber, &d.LicenseCountry, &d.PreferredLocale, &d.Status,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &d, nil
}

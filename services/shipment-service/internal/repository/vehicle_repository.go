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

type VehicleRepository struct {
	pool *pgxpool.Pool
}

func NewVehicleRepository(pool *pgxpool.Pool) *VehicleRepository {
	return &VehicleRepository{pool: pool}
}

func (r *VehicleRepository) CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error) {
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

func (r *VehicleRepository) Create(ctx context.Context, in domain.CreateVehicleInput) (*domain.Vehicle, error) {
	var result *domain.Vehicle
	err := measureDB("vehicle_repository", "create_vehicle", func() error {
		const query = `
		INSERT INTO transport.vehicles (
			tenant_id, carrier_company_id, plate_number, vehicle_type, equipment_type,
			capacity_weight, capacity_volume, registration_country, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, tenant_id, carrier_company_id, plate_number, vehicle_type, equipment_type,
			capacity_weight, capacity_volume, registration_country, status
	`
		row := r.pool.QueryRow(ctx, query,
			in.TenantID,
			in.CarrierCompanyID,
			strings.TrimSpace(in.PlateNumber),
			domain.NormalizeVehicleType(in.VehicleType),
			optionalString(in.EquipmentType),
			optionalFloat(in.CapacityWeight),
			optionalFloat(in.CapacityVolume),
			domain.NormalizeCountryCode(in.RegistrationCountry),
			domain.VehicleStatusActive,
		)
		vehicle, err := scanVehicle(row)
		if err != nil {
			return err
		}
		result = vehicle
		return nil
	})
	return result, err
}

func (r *VehicleRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Vehicle, error) {
	const query = `
		SELECT id, tenant_id, carrier_company_id, plate_number, vehicle_type, equipment_type,
			capacity_weight, capacity_volume, registration_country, status
		FROM transport.vehicles
		WHERE id = $1 AND deleted_at IS NULL
	`
	return scanVehicle(r.pool.QueryRow(ctx, query, id))
}

func (r *VehicleRepository) List(ctx context.Context, filter domain.ListVehiclesFilter) ([]domain.Vehicle, int, error) {
	var vehicles []domain.Vehicle
	var total int
	err := measureDB("vehicle_repository", "list_vehicles", func() error {
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
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM transport.vehicles WHERE %s", whereClause)
		if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
			return mapDBError(err)
		}

		args = append(args, filter.Limit, filter.Offset)
		listQuery := fmt.Sprintf(`
		SELECT id, tenant_id, carrier_company_id, plate_number, vehicle_type, equipment_type,
			capacity_weight, capacity_volume, registration_country, status
		FROM transport.vehicles
		WHERE %s
		ORDER BY plate_number ASC
		LIMIT $%d OFFSET $%d
	`, whereClause, len(args)-1, len(args))

		rows, err := r.pool.Query(ctx, listQuery, args...)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		items := make([]domain.Vehicle, 0)
		for rows.Next() {
			vehicle, err := scanVehicleRows(rows)
			if err != nil {
				return err
			}
			items = append(items, *vehicle)
		}
		if err := rows.Err(); err != nil {
			return mapDBError(err)
		}
		vehicles = items
		return nil
	})
	return vehicles, total, err
}

func scanVehicle(row pgx.Row) (*domain.Vehicle, error) {
	var v domain.Vehicle
	err := row.Scan(
		&v.ID, &v.TenantID, &v.CarrierCompanyID, &v.PlateNumber, &v.VehicleType, &v.EquipmentType,
		&v.CapacityWeight, &v.CapacityVolume, &v.RegistrationCountry, &v.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("vehicle not found")
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return &v, nil
}

func scanVehicleRows(rows pgx.Rows) (*domain.Vehicle, error) {
	var v domain.Vehicle
	err := rows.Scan(
		&v.ID, &v.TenantID, &v.CarrierCompanyID, &v.PlateNumber, &v.VehicleType, &v.EquipmentType,
		&v.CapacityWeight, &v.CapacityVolume, &v.RegistrationCountry, &v.Status,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &v, nil
}

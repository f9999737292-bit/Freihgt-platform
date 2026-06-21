package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/transport-order-service/internal/domain"
	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
)

type CargoRepository struct {
	pool *pgxpool.Pool
}

func NewCargoRepository(pool *pgxpool.Pool) *CargoRepository {
	return &CargoRepository{pool: pool}
}

func (r *CargoRepository) Create(ctx context.Context, in domain.CreateCargoInput) (*domain.Cargo, error) {
	var result *domain.Cargo
	err := measureDB("cargo_repository", "create_cargo", func() error {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return mapDBError(err)
		}
		defer tx.Rollback(ctx)

		const insertCargo = `
		INSERT INTO transport.cargoes (
			tenant_id, cargo_type, description, gross_weight, net_weight, volume,
			temperature_min, temperature_max, dangerous_goods_flag, customs_required
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, tenant_id, cargo_type, description, gross_weight, net_weight, volume,
			temperature_min, temperature_max, dangerous_goods_flag, customs_required,
			created_at, updated_at, version
	`
		row := tx.QueryRow(ctx, insertCargo,
			in.TenantID,
			strings.TrimSpace(in.CargoType),
			optionalString(in.Description),
			optionalFloat(in.GrossWeight),
			optionalFloat(in.NetWeight),
			optionalFloat(in.Volume),
			optionalFloat(in.TemperatureMin),
			optionalFloat(in.TemperatureMax),
			in.DangerousGoodsFlag,
			in.CustomsRequired,
		)

		cargo, err := scanCargo(row)
		if err != nil {
			return mapDBError(err)
		}

		const insertItem = `
		INSERT INTO transport.cargo_items (
			cargo_id, sku, name, quantity, unit, weight, volume, package_type, hazard_class
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, cargo_id, sku, name, quantity, unit, weight, volume, package_type, hazard_class
	`

		items := make([]domain.CargoItem, 0, len(in.Items))
		for _, item := range in.Items {
			itemRow := tx.QueryRow(ctx, insertItem,
				cargo.ID,
				optionalString(item.SKU),
				strings.TrimSpace(item.Name),
				item.Quantity,
				strings.TrimSpace(item.Unit),
				optionalFloat(item.Weight),
				optionalFloat(item.Volume),
				optionalString(item.PackageType),
				optionalString(item.HazardClass),
			)
			scanned, err := scanCargoItem(itemRow)
			if err != nil {
				return mapDBError(err)
			}
			items = append(items, *scanned)
		}
		cargo.Items = items

		if err := tx.Commit(ctx); err != nil {
			return mapDBError(err)
		}
		result = cargo
		return nil
	})
	return result, err
}

func (r *CargoRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Cargo, error) {
	var result *domain.Cargo
	err := measureDB("cargo_repository", "get_cargo", func() error {
		const query = `
		SELECT id, tenant_id, cargo_type, description, gross_weight, net_weight, volume,
			temperature_min, temperature_max, dangerous_goods_flag, customs_required,
			created_at, updated_at, version
		FROM transport.cargoes
		WHERE id = $1 AND deleted_at IS NULL
	`
		row := r.pool.QueryRow(ctx, query, id)
		cargo, err := scanCargo(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperrors.NotFound("cargo not found")
			}
			return mapDBError(err)
		}

		items, err := r.listItems(ctx, id)
		if err != nil {
			return err
		}
		cargo.Items = items
		result = cargo
		return nil
	})
	return result, err
}

func (r *CargoRepository) ExistsInTenant(ctx context.Context, id, tenantID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM transport.cargoes
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		)
	`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, id, tenantID).Scan(&exists); err != nil {
		return false, mapDBError(err)
	}
	return exists, nil
}

func (r *CargoRepository) listItems(ctx context.Context, cargoID uuid.UUID) ([]domain.CargoItem, error) {
	const query = `
		SELECT id, cargo_id, sku, name, quantity, unit, weight, volume, package_type, hazard_class
		FROM transport.cargo_items
		WHERE cargo_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.pool.Query(ctx, query, cargoID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	items := make([]domain.CargoItem, 0)
	for rows.Next() {
		item, err := scanCargoItem(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func scanCargo(row pgx.Row) (*domain.Cargo, error) {
	var (
		cargo     domain.Cargo
		createdAt time.Time
		updatedAt time.Time
	)
	err := row.Scan(
		&cargo.ID,
		&cargo.TenantID,
		&cargo.CargoType,
		&cargo.Description,
		&cargo.GrossWeight,
		&cargo.NetWeight,
		&cargo.Volume,
		&cargo.TemperatureMin,
		&cargo.TemperatureMax,
		&cargo.DangerousGoodsFlag,
		&cargo.CustomsRequired,
		&createdAt,
		&updatedAt,
		&cargo.Version,
	)
	if err != nil {
		return nil, err
	}
	cargo.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	cargo.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return &cargo, nil
}

func scanCargoItem(row pgx.Row) (*domain.CargoItem, error) {
	var item domain.CargoItem
	if err := row.Scan(
		&item.ID,
		&item.CargoID,
		&item.SKU,
		&item.Name,
		&item.Quantity,
		&item.Unit,
		&item.Weight,
		&item.Volume,
		&item.PackageType,
		&item.HazardClass,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

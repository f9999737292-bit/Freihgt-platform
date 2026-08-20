package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
)

type RateComponentRepository struct {
	pool      *pgxpool.Pool
	rateLines *RateLineRepository
	rateCards *RateCardRepository
	audit     *AuditRepository
}

func NewRateComponentRepository(pool *pgxpool.Pool, rateLines *RateLineRepository, rateCards *RateCardRepository, audit *AuditRepository) *RateComponentRepository {
	return &RateComponentRepository{pool: pool, rateLines: rateLines, rateCards: rateCards, audit: audit}
}

func (r *RateComponentRepository) Create(ctx context.Context, in domain.CreateRateComponentInput, correlationID *string) (*domain.RateComponent, error) {
	if err := domain.ValidateCreateRateComponentInput(in); err != nil {
		return nil, err
	}
	var created domain.RateComponent
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		line, err := r.rateLines.getForUpdate(ctx, tx, in.TenantID, in.RateLineID)
		if err != nil {
			return err
		}
		version, err := r.rateCards.getVersionForUpdate(ctx, tx, in.TenantID, line.RateCardVersionID)
		if err != nil {
			return err
		}
		if err := domain.ValidateDraftVersionMutation(version); err != nil {
			return err
		}
		const query = `
			INSERT INTO contract_rate.rate_component (
				tenant_id, rate_line_id, component_type, calculation_method,
				amount, percent_value, unit_code, created_by, updated_by
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$8)
			RETURNING id, tenant_id, rate_line_id, component_type, calculation_method,
				amount, percent_value, unit_code, created_at, created_by, updated_at, updated_by`
		row := tx.QueryRow(ctx, query, in.TenantID, in.RateLineID, in.ComponentType, in.CalculationMethod,
			decimalPtr(in.Amount), decimalPtr(in.PercentValue), optionalString(in.UnitCode), in.Actor.ActorUserID)
		if err := scanRateComponent(row, &created); err != nil {
			return err
		}
		return r.audit.InsertTx(ctx, tx, auditFromActor(domain.AuditEntityRateComponent, created.ID, domain.AuditActionRateComponentCreated, in.Actor, correlationID, nil))
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (r *RateComponentRepository) Update(ctx context.Context, tenantID, componentID uuid.UUID, patch domain.UpdateRateComponentInput, correlationID *string) (*domain.RateComponent, error) {
	var updated domain.RateComponent
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		current, err := r.getForUpdate(ctx, tx, tenantID, componentID)
		if err != nil {
			return err
		}
		line, err := r.rateLines.getForUpdate(ctx, tx, tenantID, current.RateLineID)
		if err != nil {
			return err
		}
		version, err := r.rateCards.getVersionForUpdate(ctx, tx, tenantID, line.RateCardVersionID)
		if err != nil {
			return err
		}
		if err := domain.ValidateDraftVersionMutation(version); err != nil {
			return err
		}
		amount := current.Amount
		if patch.Amount != nil {
			amount = patch.Amount
		}
		percent := current.PercentValue
		if patch.PercentValue != nil {
			percent = patch.PercentValue
		}
		unit := current.UnitCode
		if patch.UnitCode != nil {
			unit = patch.UnitCode
		}
		if err := domain.ValidateUpdateRateComponentInput(current, patch); err != nil {
			return err
		}
		const query = `
			UPDATE contract_rate.rate_component SET
				amount = $3, percent_value = $4, unit_code = $5,
				updated_by = $6, updated_at = now()
			WHERE tenant_id = $1 AND id = $2
			RETURNING id, tenant_id, rate_line_id, component_type, calculation_method,
				amount, percent_value, unit_code, created_at, created_by, updated_at, updated_by`
		row := tx.QueryRow(ctx, query, tenantID, componentID, decimalPtr(amount), decimalPtr(percent), optionalString(unit), patch.Actor.ActorUserID)
		if err := scanRateComponent(row, &updated); err != nil {
			return err
		}
		return r.audit.InsertTx(ctx, tx, auditFromActor(domain.AuditEntityRateComponent, updated.ID, domain.AuditActionRateComponentUpdated, patch.Actor, correlationID, nil))
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *RateComponentRepository) Delete(ctx context.Context, tenantID, componentID uuid.UUID, actor domain.ActorInput, correlationID *string) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		current, err := r.getForUpdate(ctx, tx, tenantID, componentID)
		if err != nil {
			return err
		}
		line, err := r.rateLines.getForUpdate(ctx, tx, tenantID, current.RateLineID)
		if err != nil {
			return err
		}
		version, err := r.rateCards.getVersionForUpdate(ctx, tx, tenantID, line.RateCardVersionID)
		if err != nil {
			return err
		}
		if err := domain.ValidateDraftVersionMutation(version); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `DELETE FROM contract_rate.rate_component WHERE tenant_id = $1 AND id = $2`, tenantID, componentID)
		if err != nil {
			return mapDBError(err)
		}
		if tag.RowsAffected() == 0 {
			return apperrors.NotFound("rate component not found")
		}
		return r.audit.InsertTx(ctx, tx, auditFromActor(domain.AuditEntityRateComponent, componentID, domain.AuditActionRateComponentDeleted, actor, correlationID, nil))
	})
}

func (r *RateComponentRepository) GetByIDAndTenant(ctx context.Context, tenantID, componentID uuid.UUID) (*domain.RateComponent, error) {
	const query = `
		SELECT id, tenant_id, rate_line_id, component_type, calculation_method,
			amount, percent_value, unit_code, created_at, created_by, updated_at, updated_by
		FROM contract_rate.rate_component
		WHERE tenant_id = $1 AND id = $2`
	var item domain.RateComponent
	if err := scanRateComponent(r.pool.QueryRow(ctx, query, tenantID, componentID), &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *RateComponentRepository) ListByLine(ctx context.Context, tenantID, lineID uuid.UUID) ([]domain.RateComponent, error) {
	const query = `
		SELECT id, tenant_id, rate_line_id, component_type, calculation_method,
			amount, percent_value, unit_code, created_at, created_by, updated_at, updated_by
		FROM contract_rate.rate_component
		WHERE tenant_id = $1 AND rate_line_id = $2
		ORDER BY component_type ASC`
	rows, err := r.pool.Query(ctx, query, tenantID, lineID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	items := make([]domain.RateComponent, 0)
	for rows.Next() {
		var item domain.RateComponent
		if err := scanRateComponent(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *RateComponentRepository) ListByVersion(ctx context.Context, tenantID, versionID uuid.UUID) (map[uuid.UUID][]domain.RateComponent, error) {
	const query = `
		SELECT c.id, c.tenant_id, c.rate_line_id, c.component_type, c.calculation_method,
			c.amount, c.percent_value, c.unit_code, c.created_at, c.created_by, c.updated_at, c.updated_by
		FROM contract_rate.rate_component c
		JOIN contract_rate.rate_line l ON l.tenant_id = c.tenant_id AND l.id = c.rate_line_id
		WHERE c.tenant_id = $1 AND l.rate_card_version_id = $2`
	rows, err := r.pool.Query(ctx, query, tenantID, versionID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	result := map[uuid.UUID][]domain.RateComponent{}
	for rows.Next() {
		var item domain.RateComponent
		if err := scanRateComponent(rows, &item); err != nil {
			return nil, err
		}
		result[item.RateLineID] = append(result[item.RateLineID], item)
	}
	return result, rows.Err()
}

func (r *RateComponentRepository) getForUpdate(ctx context.Context, tx pgx.Tx, tenantID, componentID uuid.UUID) (*domain.RateComponent, error) {
	const query = `
		SELECT id, tenant_id, rate_line_id, component_type, calculation_method,
			amount, percent_value, unit_code, created_at, created_by, updated_at, updated_by
		FROM contract_rate.rate_component WHERE tenant_id = $1 AND id = $2 FOR UPDATE`
	var item domain.RateComponent
	if err := scanRateComponent(tx.QueryRow(ctx, query, tenantID, componentID), &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *RateComponentRepository) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return mapDBError(err)
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return mapDBError(tx.Commit(ctx))
}

func scanRateComponent(row scannable, out *domain.RateComponent) error {
	var amount, percent *decimal.Decimal
	var unit *string
	if err := row.Scan(
		&out.ID, &out.TenantID, &out.RateLineID, &out.ComponentType, &out.CalculationMethod,
		&amount, &percent, &unit, &out.CreatedAt, &out.CreatedBy, &out.UpdatedAt, &out.UpdatedBy,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NotFound("rate component not found")
		}
		return mapDBError(err)
	}
	out.Amount = amount
	out.PercentValue = percent
	out.UnitCode = unit
	return nil
}

func decimalPtr(v *decimal.Decimal) any {
	if v == nil {
		return nil
	}
	return *v
}

package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
)

type RateLineRepository struct {
	pool      *pgxpool.Pool
	rateCards *RateCardRepository
	locations *LocationRepository
	audit     *AuditRepository
}

func NewRateLineRepository(pool *pgxpool.Pool, rateCards *RateCardRepository, locations *LocationRepository, audit *AuditRepository) *RateLineRepository {
	return &RateLineRepository{pool: pool, rateCards: rateCards, locations: locations, audit: audit}
}

func (r *RateLineRepository) Create(ctx context.Context, in domain.CreateRateLineInput, correlationID *string) (*domain.RateLine, error) {
	if err := domain.ValidateCreateRateLineInput(in); err != nil {
		return nil, err
	}
	equipment, _ := domain.NormalizeEquipmentType(in.EquipmentType)
	mode, _ := domain.NormalizeTransportMode(in.TransportMode)
	var created domain.RateLine
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		version, err := r.rateCards.getVersionForUpdate(ctx, tx, in.TenantID, in.RateCardVersionID)
		if err != nil {
			return err
		}
		if err := domain.ValidateDraftVersionMutation(version); err != nil {
			return err
		}
		ok, err := r.locations.ExistsAllInTenant(ctx, in.TenantID, []uuid.UUID{in.OriginLocationID, in.DestinationLocationID})
		if err != nil {
			return err
		}
		if !ok {
			return apperrors.NotFound("location not found")
		}
		const query = `
			INSERT INTO contract_rate.rate_line (
				tenant_id, rate_card_version_id, origin_location_id, destination_location_id,
				equipment_type, transport_mode, created_by, updated_by
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$7)
			RETURNING id, tenant_id, rate_card_version_id, origin_location_id, destination_location_id,
				equipment_type, transport_mode, created_at, created_by, updated_at, updated_by`
		row := tx.QueryRow(ctx, query, in.TenantID, in.RateCardVersionID, in.OriginLocationID, in.DestinationLocationID,
			equipment, mode, in.Actor.ActorUserID)
		if err := scanRateLine(row, &created); err != nil {
			return err
		}
		return r.audit.InsertTx(ctx, tx, auditFromActor(domain.AuditEntityRateLine, created.ID, domain.AuditActionRateLineCreated, in.Actor, correlationID, nil))
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (r *RateLineRepository) GetByIDAndTenant(ctx context.Context, tenantID, lineID uuid.UUID) (*domain.RateLine, error) {
	return r.getByID(ctx, r.pool, tenantID, lineID)
}

func (r *RateLineRepository) ListByVersion(ctx context.Context, tenantID, versionID uuid.UUID) ([]domain.RateLine, error) {
	const query = `
		SELECT id, tenant_id, rate_card_version_id, origin_location_id, destination_location_id,
			equipment_type, transport_mode, created_at, created_by, updated_at, updated_by
		FROM contract_rate.rate_line
		WHERE tenant_id = $1 AND rate_card_version_id = $2
		ORDER BY created_at ASC`
	rows, err := r.pool.Query(ctx, query, tenantID, versionID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	items := make([]domain.RateLine, 0)
	for rows.Next() {
		var item domain.RateLine
		if err := scanRateLine(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *RateLineRepository) Update(ctx context.Context, tenantID, lineID uuid.UUID, patch domain.UpdateRateLineInput, correlationID *string) (*domain.RateLine, error) {
	var updated domain.RateLine
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		current, err := r.getForUpdate(ctx, tx, tenantID, lineID)
		if err != nil {
			return err
		}
		version, err := r.rateCards.getVersionForUpdate(ctx, tx, tenantID, current.RateCardVersionID)
		if err != nil {
			return err
		}
		if err := domain.ValidateDraftVersionMutation(version); err != nil {
			return err
		}
		origin := current.OriginLocationID
		if patch.OriginLocationID != nil {
			origin = *patch.OriginLocationID
		}
		dest := current.DestinationLocationID
		if patch.DestinationLocationID != nil {
			dest = *patch.DestinationLocationID
		}
		equipment := current.EquipmentType
		if patch.EquipmentType != nil {
			equipment, err = domain.NormalizeEquipmentType(*patch.EquipmentType)
			if err != nil {
				return err
			}
		}
		mode := current.TransportMode
		if patch.TransportMode != nil {
			mode, err = domain.NormalizeTransportMode(*patch.TransportMode)
			if err != nil {
				return err
			}
		}
		if origin == dest {
			return apperrors.Validation("origin and destination must differ", map[string]any{"field": "destination_location_id"})
		}
		ok, err := r.locations.ExistsAllInTenant(ctx, tenantID, []uuid.UUID{origin, dest})
		if err != nil {
			return err
		}
		if !ok {
			return apperrors.NotFound("location not found")
		}
		const query = `
			UPDATE contract_rate.rate_line SET
				origin_location_id = $3, destination_location_id = $4,
				equipment_type = $5, transport_mode = $6,
				updated_by = $7, updated_at = now()
			WHERE tenant_id = $1 AND id = $2
			RETURNING id, tenant_id, rate_card_version_id, origin_location_id, destination_location_id,
				equipment_type, transport_mode, created_at, created_by, updated_at, updated_by`
		row := tx.QueryRow(ctx, query, tenantID, lineID, origin, dest, equipment, mode, patch.Actor.ActorUserID)
		if err := scanRateLine(row, &updated); err != nil {
			return err
		}
		return r.audit.InsertTx(ctx, tx, auditFromActor(domain.AuditEntityRateLine, updated.ID, domain.AuditActionRateLineUpdated, patch.Actor, correlationID, nil))
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *RateLineRepository) Delete(ctx context.Context, tenantID, lineID uuid.UUID, actor domain.ActorInput, correlationID *string) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		current, err := r.getForUpdate(ctx, tx, tenantID, lineID)
		if err != nil {
			return err
		}
		version, err := r.rateCards.getVersionForUpdate(ctx, tx, tenantID, current.RateCardVersionID)
		if err != nil {
			return err
		}
		if err := domain.ValidateDraftVersionMutation(version); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `DELETE FROM contract_rate.rate_line WHERE tenant_id = $1 AND id = $2`, tenantID, lineID)
		if err != nil {
			return mapDBError(err)
		}
		if tag.RowsAffected() == 0 {
			return apperrors.NotFound("rate line not found")
		}
		return r.audit.InsertTx(ctx, tx, auditFromActor(domain.AuditEntityRateLine, lineID, domain.AuditActionRateLineDeleted, actor, correlationID, nil))
	})
}

func (r *RateLineRepository) getByID(ctx context.Context, q queryRowProvider, tenantID, lineID uuid.UUID) (*domain.RateLine, error) {
	const query = `
		SELECT id, tenant_id, rate_card_version_id, origin_location_id, destination_location_id,
			equipment_type, transport_mode, created_at, created_by, updated_at, updated_by
		FROM contract_rate.rate_line WHERE tenant_id = $1 AND id = $2`
	var line domain.RateLine
	if err := scanRateLine(q.QueryRow(ctx, query, tenantID, lineID), &line); err != nil {
		return nil, err
	}
	return &line, nil
}

func (r *RateLineRepository) getForUpdate(ctx context.Context, tx pgx.Tx, tenantID, lineID uuid.UUID) (*domain.RateLine, error) {
	const query = `
		SELECT id, tenant_id, rate_card_version_id, origin_location_id, destination_location_id,
			equipment_type, transport_mode, created_at, created_by, updated_at, updated_by
		FROM contract_rate.rate_line WHERE tenant_id = $1 AND id = $2 FOR UPDATE`
	var line domain.RateLine
	if err := scanRateLine(tx.QueryRow(ctx, query, tenantID, lineID), &line); err != nil {
		return nil, err
	}
	return &line, nil
}

func (r *RateLineRepository) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
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

func scanRateLine(row scannable, out *domain.RateLine) error {
	if err := row.Scan(
		&out.ID, &out.TenantID, &out.RateCardVersionID, &out.OriginLocationID, &out.DestinationLocationID,
		&out.EquipmentType, &out.TransportMode, &out.CreatedAt, &out.CreatedBy, &out.UpdatedAt, &out.UpdatedBy,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NotFound("rate line not found")
		}
		return mapDBError(err)
	}
	return nil
}

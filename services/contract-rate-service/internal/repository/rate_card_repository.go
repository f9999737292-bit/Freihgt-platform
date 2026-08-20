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

type RateCardRepository struct {
	pool       *pgxpool.Pool
	contracts  *ContractRepository
	audit      *AuditRepository
}

func NewRateCardRepository(pool *pgxpool.Pool, contracts *ContractRepository, audit *AuditRepository) *RateCardRepository {
	return &RateCardRepository{pool: pool, contracts: contracts, audit: audit}
}

func (r *RateCardRepository) Create(ctx context.Context, in domain.CreateRateCardInput, correlationID *string) (*domain.RateCard, error) {
	if err := domain.ValidateCreateRateCardInput(in); err != nil {
		return nil, err
	}
	var created domain.RateCard
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		contract, err := r.contracts.getByIDAndTenantForUpdate(ctx, tx, in.TenantID, in.ContractID)
		if err != nil {
			return err
		}
		if err := domain.ValidateRateCardParentContract(contract); err != nil {
			return err
		}
		const query = `
			INSERT INTO contract_rate.rate_card (
				tenant_id, contract_id, name, description, created_by, updated_by
			) VALUES ($1,$2,$3,$4,$5,$5)
			RETURNING id, tenant_id, contract_id, name, description,
				created_at, created_by, updated_at, updated_by, version`
		row := tx.QueryRow(ctx, query, in.TenantID, in.ContractID, in.Name, optionalString(in.Description), in.Actor.ActorUserID)
		if err := scanRateCard(row, &created); err != nil {
			return err
		}
		return r.audit.InsertTx(ctx, tx, auditFromActor(domain.AuditEntityRateCard, created.ID, domain.AuditActionRateCardCreated, in.Actor, correlationID, map[string]any{
			"contract_id": created.ContractID.String(),
		}))
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (r *RateCardRepository) GetByIDAndTenant(ctx context.Context, tenantID, rateCardID uuid.UUID) (*domain.RateCard, error) {
	return r.getRateCardByID(ctx, r.pool, tenantID, rateCardID)
}

func (r *RateCardRepository) ListByContract(ctx context.Context, tenantID, contractID uuid.UUID) ([]domain.RateCard, error) {
	const query = `
		SELECT id, tenant_id, contract_id, name, description,
			created_at, created_by, updated_at, updated_by, version
		FROM contract_rate.rate_card
		WHERE tenant_id = $1 AND contract_id = $2
		ORDER BY created_at ASC`
	rows, err := r.pool.Query(ctx, query, tenantID, contractID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	items := make([]domain.RateCard, 0)
	for rows.Next() {
		var item domain.RateCard
		if err := scanRateCard(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *RateCardRepository) Update(ctx context.Context, tenantID, rateCardID uuid.UUID, patch domain.UpdateRateCardInput, correlationID *string) (*domain.RateCard, error) {
	var updated domain.RateCard
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		current, err := r.getRateCardForUpdate(ctx, tx, tenantID, rateCardID)
		if err != nil {
			return err
		}
		contract, err := r.contracts.getByIDAndTenantForUpdate(ctx, tx, tenantID, current.ContractID)
		if err != nil {
			return err
		}
		if err := domain.ValidateRateCardParentContract(contract); err != nil {
			return err
		}
		name := current.Name
		if patch.Name != nil {
			name = *patch.Name
		}
		desc := current.Description
		if patch.Description != nil {
			desc = patch.Description
		}
		const query = `
			UPDATE contract_rate.rate_card SET
				name = $3, description = $4, updated_by = $5, updated_at = now(), version = version + 1
			WHERE tenant_id = $1 AND id = $2
			RETURNING id, tenant_id, contract_id, name, description,
				created_at, created_by, updated_at, updated_by, version`
		row := tx.QueryRow(ctx, query, tenantID, rateCardID, name, optionalString(desc), patch.Actor.ActorUserID)
		if err := scanRateCard(row, &updated); err != nil {
			return err
		}
		return r.audit.InsertTx(ctx, tx, auditFromActor(domain.AuditEntityRateCard, updated.ID, domain.AuditActionRateCardUpdated, patch.Actor, correlationID, nil))
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *RateCardRepository) CreateDraftVersion(ctx context.Context, in domain.CreateRateVersionInput, correlationID *string) (*domain.RateCardVersion, error) {
	if err := domain.ValidateCreateRateVersionInput(in); err != nil {
		return nil, err
	}
	var created domain.RateCardVersion
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		card, err := r.getRateCardForUpdate(ctx, tx, in.TenantID, in.RateCardID)
		if err != nil {
			return err
		}
		contract, err := r.contracts.getByIDAndTenantForUpdate(ctx, tx, in.TenantID, card.ContractID)
		if err != nil {
			return err
		}
		if err := domain.ValidateRateCardParentContract(contract); err != nil {
			return err
		}
		var nextVersion int
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(MAX(version_number), 0) + 1
			FROM contract_rate.rate_card_version
			WHERE tenant_id = $1 AND rate_card_id = $2`, in.TenantID, in.RateCardID).Scan(&nextVersion); err != nil {
			return mapDBError(err)
		}
		const query = `
			INSERT INTO contract_rate.rate_card_version (
				tenant_id, rate_card_id, version_number, valid_from, valid_to, status, created_by
			) VALUES ($1,$2,$3,$4,$5,'DRAFT',$6)
			RETURNING id, tenant_id, rate_card_id, version_number, valid_from, valid_to, status,
				supersedes_version_id, created_at, created_by, activated_at, activated_by, version`
		row := tx.QueryRow(ctx, query, in.TenantID, in.RateCardID, nextVersion, dateOnly(in.ValidFrom), optionalDate(in.ValidTo), in.Actor.ActorUserID)
		if err := scanRateVersion(row, &created); err != nil {
			return err
		}
		return r.audit.InsertTx(ctx, tx, auditFromActor(domain.AuditEntityRateVersion, created.ID, domain.AuditActionRateVersionCreated, in.Actor, correlationID, map[string]any{
			"version_number": created.VersionNumber,
		}))
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (r *RateCardRepository) GetVersionByIDAndTenant(ctx context.Context, tenantID, versionID uuid.UUID) (*domain.RateCardVersion, error) {
	return r.getVersionByID(ctx, r.pool, tenantID, versionID)
}

func (r *RateCardRepository) ListVersions(ctx context.Context, tenantID, rateCardID uuid.UUID) ([]domain.RateCardVersion, error) {
	const query = `
		SELECT id, tenant_id, rate_card_id, version_number, valid_from, valid_to, status,
			supersedes_version_id, created_at, created_by, activated_at, activated_by, version
		FROM contract_rate.rate_card_version
		WHERE tenant_id = $1 AND rate_card_id = $2
		ORDER BY version_number ASC`
	rows, err := r.pool.Query(ctx, query, tenantID, rateCardID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	items := make([]domain.RateCardVersion, 0)
	for rows.Next() {
		var item domain.RateCardVersion
		if err := scanRateVersion(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *RateCardRepository) UpdateDraftVersion(ctx context.Context, tenantID, versionID uuid.UUID, patch domain.UpdateRateVersionInput, correlationID *string) (*domain.RateCardVersion, error) {
	var updated domain.RateCardVersion
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		current, err := r.getVersionForUpdate(ctx, tx, tenantID, versionID)
		if err != nil {
			return err
		}
		if err := domain.ValidateUpdateRateVersionInput(current, patch); err != nil {
			return err
		}
		validFrom := current.ValidFrom
		if patch.ValidFrom != nil {
			validFrom = *patch.ValidFrom
		}
		validTo := current.ValidTo
		if patch.ValidTo != nil {
			validTo = patch.ValidTo
		}
		const query = `
			UPDATE contract_rate.rate_card_version SET
				valid_from = $3, valid_to = $4, version = version + 1
			WHERE tenant_id = $1 AND id = $2 AND status = 'DRAFT'
			RETURNING id, tenant_id, rate_card_id, version_number, valid_from, valid_to, status,
				supersedes_version_id, created_at, created_by, activated_at, activated_by, version`
		row := tx.QueryRow(ctx, query, tenantID, versionID, dateOnly(validFrom), optionalDate(validTo))
		if err := scanRateVersion(row, &updated); err != nil {
			return err
		}
		return r.audit.InsertTx(ctx, tx, auditFromActor(domain.AuditEntityRateVersion, updated.ID, domain.AuditActionRateVersionUpdated, patch.Actor, correlationID, nil))
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *RateCardRepository) DiscardDraftVersion(ctx context.Context, tenantID, versionID uuid.UUID, actor domain.ActorInput, correlationID *string) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		current, err := r.getVersionForUpdate(ctx, tx, tenantID, versionID)
		if err != nil {
			return err
		}
		if err := domain.ValidateDiscardRateVersion(current); err != nil {
			return err
		}
		const query = `DELETE FROM contract_rate.rate_card_version WHERE tenant_id = $1 AND id = $2 AND status = 'DRAFT'`
		tag, err := tx.Exec(ctx, query, tenantID, versionID)
		if err != nil {
			return mapDBError(err)
		}
		if tag.RowsAffected() == 0 {
			return apperrors.NotFound("rate version not found")
		}
		return r.audit.InsertTx(ctx, tx, auditFromActor(domain.AuditEntityRateVersion, versionID, domain.AuditActionRateVersionDiscarded, actor, correlationID, nil))
	})
}

func (r *RateCardRepository) getRateCardByID(ctx context.Context, q queryRowProvider, tenantID, rateCardID uuid.UUID) (*domain.RateCard, error) {
	const query = `
		SELECT id, tenant_id, contract_id, name, description,
			created_at, created_by, updated_at, updated_by, version
		FROM contract_rate.rate_card WHERE tenant_id = $1 AND id = $2`
	var card domain.RateCard
	if err := scanRateCard(q.QueryRow(ctx, query, tenantID, rateCardID), &card); err != nil {
		return nil, err
	}
	return &card, nil
}

func (r *RateCardRepository) getRateCardForUpdate(ctx context.Context, tx pgx.Tx, tenantID, rateCardID uuid.UUID) (*domain.RateCard, error) {
	const query = `
		SELECT id, tenant_id, contract_id, name, description,
			created_at, created_by, updated_at, updated_by, version
		FROM contract_rate.rate_card WHERE tenant_id = $1 AND id = $2 FOR UPDATE`
	var card domain.RateCard
	if err := scanRateCard(tx.QueryRow(ctx, query, tenantID, rateCardID), &card); err != nil {
		return nil, err
	}
	return &card, nil
}

func (r *RateCardRepository) getVersionByID(ctx context.Context, q queryRowProvider, tenantID, versionID uuid.UUID) (*domain.RateCardVersion, error) {
	const query = `
		SELECT id, tenant_id, rate_card_id, version_number, valid_from, valid_to, status,
			supersedes_version_id, created_at, created_by, activated_at, activated_by, version
		FROM contract_rate.rate_card_version WHERE tenant_id = $1 AND id = $2`
	var version domain.RateCardVersion
	if err := scanRateVersion(q.QueryRow(ctx, query, tenantID, versionID), &version); err != nil {
		return nil, err
	}
	return &version, nil
}

func (r *RateCardRepository) getVersionForUpdate(ctx context.Context, tx pgx.Tx, tenantID, versionID uuid.UUID) (*domain.RateCardVersion, error) {
	const query = `
		SELECT id, tenant_id, rate_card_id, version_number, valid_from, valid_to, status,
			supersedes_version_id, created_at, created_by, activated_at, activated_by, version
		FROM contract_rate.rate_card_version WHERE tenant_id = $1 AND id = $2 FOR UPDATE`
	var version domain.RateCardVersion
	if err := scanRateVersion(tx.QueryRow(ctx, query, tenantID, versionID), &version); err != nil {
		return nil, err
	}
	return &version, nil
}

func (r *RateCardRepository) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
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

func scanRateCard(row scannable, out *domain.RateCard) error {
	if err := row.Scan(
		&out.ID, &out.TenantID, &out.ContractID, &out.Name, &out.Description,
		&out.CreatedAt, &out.CreatedBy, &out.UpdatedAt, &out.UpdatedBy, &out.Version,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NotFound("rate card not found")
		}
		return mapDBError(err)
	}
	return nil
}

func scanRateVersion(row scannable, out *domain.RateCardVersion) error {
	if err := row.Scan(
		&out.ID, &out.TenantID, &out.RateCardID, &out.VersionNumber, &out.ValidFrom, &out.ValidTo, &out.Status,
		&out.SupersedesVersionID, &out.CreatedAt, &out.CreatedBy, &out.ActivatedAt, &out.ActivatedBy, &out.Version,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NotFound("rate version not found")
		}
		return mapDBError(err)
	}
	out.ValidFrom = dateOnly(out.ValidFrom)
	if out.ValidTo != nil {
		v := dateOnly(*out.ValidTo)
		out.ValidTo = &v
	}
	return nil
}

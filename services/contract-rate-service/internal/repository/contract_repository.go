package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
)

type ContractRepository struct {
	pool                 *pgxpool.Pool
	audit                *AuditRepository
	simulateAuditFailure bool
}

func NewContractRepository(pool *pgxpool.Pool, audit *AuditRepository) *ContractRepository {
	return &ContractRepository{pool: pool, audit: audit}
}

func (r *ContractRepository) Create(ctx context.Context, in domain.CreateContractInput, correlationID *string) (*domain.TransportContract, error) {
	if err := domain.ValidateCreateContractInput(in); err != nil {
		return nil, err
	}
	var created domain.TransportContract
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		const query = `
			INSERT INTO contract_rate.transport_contract (
				tenant_id, buyer_company_id, carrier_company_id, contract_number,
				external_reference, name, description, status, valid_from, valid_to,
				currency_code, created_by, updated_by
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$12
			)
			RETURNING id, tenant_id, buyer_company_id, carrier_company_id, contract_number,
				external_reference, name, description, status, valid_from, valid_to, currency_code,
				created_at, created_by, updated_at, updated_by, activated_at, activated_by,
				terminated_at, terminated_by, termination_reason, version`
		row := tx.QueryRow(ctx, query,
			in.TenantID, in.BuyerCompanyID, in.CarrierCompanyID,
			domain.NormalizeContractNumber(in.ContractNumber),
			optionalString(in.ExternalReference), in.Name, optionalString(in.Description),
			domain.ContractStatusDraft, dateOnly(in.ValidFrom), optionalDate(in.ValidTo),
			domain.NormalizeCurrencyCode(in.CurrencyCode), in.Actor.ActorUserID,
		)
		if err := scanContract(row, &created); err != nil {
			return err
		}
		if r.simulateAuditFailure {
			return errors.New("simulated contract audit failure")
		}
		return r.audit.InsertTx(ctx, tx, auditFromActor(domain.AuditEntityContract, created.ID, domain.AuditActionContractCreated, in.Actor, correlationID, map[string]any{
			"contract_number": created.ContractNumber,
			"status":          created.Status,
		}))
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (r *ContractRepository) GetByIDAndTenant(ctx context.Context, tenantID, contractID uuid.UUID) (*domain.TransportContract, error) {
	contract, err := r.getByIDAndTenantTx(ctx, r.pool, tenantID, contractID)
	if err != nil {
		return nil, err
	}
	expired, err := r.maybeExpire(ctx, contract)
	if err != nil {
		return nil, err
	}
	if expired {
		return r.getByIDAndTenantTx(ctx, r.pool, tenantID, contractID)
	}
	return contract, nil
}

func (r *ContractRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, buyerCompanyID *uuid.UUID) ([]domain.TransportContract, error) {
	query := `
		SELECT id, tenant_id, buyer_company_id, carrier_company_id, contract_number,
			external_reference, name, description, status, valid_from, valid_to, currency_code,
			created_at, created_by, updated_at, updated_by, activated_at, activated_by,
			terminated_at, terminated_by, termination_reason, version
		FROM contract_rate.transport_contract
		WHERE tenant_id = $1`
	args := []any{tenantID}
	if buyerCompanyID != nil {
		query += ` AND buyer_company_id = $2`
		args = append(args, *buyerCompanyID)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	items := make([]domain.TransportContract, 0)
	for rows.Next() {
		var item domain.TransportContract
		if err := scanContract(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *ContractRepository) UpdateDraft(ctx context.Context, tenantID, contractID uuid.UUID, patch domain.UpdateContractInput, correlationID *string) (*domain.TransportContract, error) {
	var updated domain.TransportContract
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		current, err := r.getByIDAndTenantForUpdate(ctx, tx, tenantID, contractID)
		if err != nil {
			return err
		}
		if err := domain.ValidateDraftContractUpdate(current, patch); err != nil {
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
		ext := current.ExternalReference
		if patch.ExternalReference != nil {
			ext = patch.ExternalReference
		}
		validTo := current.ValidTo
		if patch.ValidTo.Present {
			validTo = domain.ApplyNullableDatePatch(current.ValidTo, patch.ValidTo)
		}
		const query = `
			UPDATE contract_rate.transport_contract SET
				name = $3, description = $4, external_reference = $5, valid_to = $6,
				updated_by = $7, updated_at = now(), version = version + 1
			WHERE tenant_id = $1 AND id = $2 AND status = 'DRAFT'
			RETURNING id, tenant_id, buyer_company_id, carrier_company_id, contract_number,
				external_reference, name, description, status, valid_from, valid_to, currency_code,
				created_at, created_by, updated_at, updated_by, activated_at, activated_by,
				terminated_at, terminated_by, termination_reason, version`
		row := tx.QueryRow(ctx, query, tenantID, contractID, name, optionalString(desc), optionalString(ext), optionalDate(validTo), patch.Actor.ActorUserID)
		if err := scanContract(row, &updated); err != nil {
			return err
		}
		return r.audit.InsertTx(ctx, tx, auditFromActor(domain.AuditEntityContract, updated.ID, domain.AuditActionContractUpdated, patch.Actor, correlationID, map[string]any{"status": updated.Status}))
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *ContractRepository) PatchMetadata(ctx context.Context, tenantID, contractID uuid.UUID, patch domain.PatchContractMetadataInput, correlationID *string) (*domain.TransportContract, error) {
	var updated domain.TransportContract
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		current, err := r.getByIDAndTenantForUpdate(ctx, tx, tenantID, contractID)
		if err != nil {
			return err
		}
		if expired, err := r.maybeExpireTx(ctx, tx, current, &patch.Actor); err != nil {
			return err
		} else if expired {
			current, err = r.getByIDAndTenantForUpdate(ctx, tx, tenantID, contractID)
			if err != nil {
				return err
			}
		}
		if err := domain.ValidateMetadataPatch(current, patch); err != nil {
			return err
		}
		desc := current.Description
		if patch.Description != nil {
			desc = patch.Description
		}
		ext := current.ExternalReference
		if patch.ExternalReference != nil {
			ext = patch.ExternalReference
		}
		const query = `
			UPDATE contract_rate.transport_contract SET
				description = $3, external_reference = $4,
				updated_by = $5, updated_at = now(), version = version + 1
			WHERE tenant_id = $1 AND id = $2 AND status IN ('ACTIVE','SUSPENDED')
			RETURNING id, tenant_id, buyer_company_id, carrier_company_id, contract_number,
				external_reference, name, description, status, valid_from, valid_to, currency_code,
				created_at, created_by, updated_at, updated_by, activated_at, activated_by,
				terminated_at, terminated_by, termination_reason, version`
		row := tx.QueryRow(ctx, query, tenantID, contractID, optionalString(desc), optionalString(ext), patch.Actor.ActorUserID)
		if err := scanContract(row, &updated); err != nil {
			return err
		}
		return r.audit.InsertTx(ctx, tx, auditFromActor(domain.AuditEntityContract, updated.ID, domain.AuditActionContractUpdated, patch.Actor, correlationID, map[string]any{"status": updated.Status}))
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *ContractRepository) Activate(ctx context.Context, tenantID, contractID uuid.UUID, actor domain.ActorInput, correlationID *string) (*domain.TransportContract, error) {
	return r.transition(ctx, tenantID, contractID, actor, correlationID, func(current *domain.TransportContract, now time.Time) (string, error) {
		if err := domain.ValidateActivateContract(current, dateOnly(now)); err != nil {
			return "", err
		}
		if current.Status == domain.ContractStatusActive {
			return "", nil
		}
		domain.TransitionActivate(current, now, actor.ActorUserID)
		return domain.AuditActionContractActivated, nil
	})
}

func (r *ContractRepository) Suspend(ctx context.Context, tenantID, contractID uuid.UUID, actor domain.ActorInput, correlationID *string) (*domain.TransportContract, error) {
	return r.transition(ctx, tenantID, contractID, actor, correlationID, func(current *domain.TransportContract, _ time.Time) (string, error) {
		if err := domain.ValidateSuspendContract(current); err != nil {
			return "", err
		}
		if current.Status == domain.ContractStatusSuspended {
			return "", nil
		}
		domain.TransitionSuspend(current)
		return domain.AuditActionContractSuspended, nil
	})
}

func (r *ContractRepository) Reactivate(ctx context.Context, tenantID, contractID uuid.UUID, actor domain.ActorInput, correlationID *string) (*domain.TransportContract, error) {
	return r.transition(ctx, tenantID, contractID, actor, correlationID, func(current *domain.TransportContract, now time.Time) (string, error) {
		if err := domain.ValidateReactivateContract(current, dateOnly(now)); err != nil {
			return "", err
		}
		if current.Status == domain.ContractStatusActive {
			return "", nil
		}
		domain.TransitionReactivate(current)
		return domain.AuditActionContractReactivated, nil
	})
}

func (r *ContractRepository) Terminate(ctx context.Context, tenantID, contractID uuid.UUID, actor domain.ActorInput, reason *string, correlationID *string) (*domain.TransportContract, error) {
	return r.transition(ctx, tenantID, contractID, actor, correlationID, func(current *domain.TransportContract, now time.Time) (string, error) {
		if err := domain.ValidateTerminateContract(current); err != nil {
			return "", err
		}
		if current.Status == domain.ContractStatusTerminated {
			return "", nil
		}
		domain.TransitionTerminate(current, now, actor.ActorUserID, reason)
		return domain.AuditActionContractTerminated, nil
	})
}

func (r *ContractRepository) Cancel(ctx context.Context, tenantID, contractID uuid.UUID, actor domain.ActorInput, correlationID *string) (*domain.TransportContract, error) {
	return r.transition(ctx, tenantID, contractID, actor, correlationID, func(current *domain.TransportContract, _ time.Time) (string, error) {
		if err := domain.ValidateCancelContract(current); err != nil {
			return "", err
		}
		if current.Status == domain.ContractStatusCancelled {
			return "", nil
		}
		domain.TransitionCancel(current)
		return domain.AuditActionContractCancelled, nil
	})
}

type transitionFn func(current *domain.TransportContract, now time.Time) (auditAction string, err error)

func (r *ContractRepository) transition(ctx context.Context, tenantID, contractID uuid.UUID, actor domain.ActorInput, correlationID *string, fn transitionFn) (*domain.TransportContract, error) {
	var updated domain.TransportContract
	now := time.Now().UTC()
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		current, err := r.getByIDAndTenantForUpdate(ctx, tx, tenantID, contractID)
		if err != nil {
			return err
		}
		if expired, err := r.maybeExpireTx(ctx, tx, current, &actor); err != nil {
			return err
		} else if expired {
			current, err = r.getByIDAndTenantForUpdate(ctx, tx, tenantID, contractID)
			if err != nil {
				return err
			}
		}
		action, err := fn(current, now)
		if err != nil {
			return err
		}
		if action == "" {
			updated = *current
			return nil
		}
		const query = `
			UPDATE contract_rate.transport_contract SET
				status = $3, activated_at = $4, activated_by = $5,
				terminated_at = $6, terminated_by = $7, termination_reason = $8,
				updated_by = $9, updated_at = now(), version = version + 1
			WHERE tenant_id = $1 AND id = $2
			RETURNING id, tenant_id, buyer_company_id, carrier_company_id, contract_number,
				external_reference, name, description, status, valid_from, valid_to, currency_code,
				created_at, created_by, updated_at, updated_by, activated_at, activated_by,
				terminated_at, terminated_by, termination_reason, version`
		row := tx.QueryRow(ctx, query,
			tenantID, contractID, current.Status,
			optionalDate(current.ActivatedAt), optionalUUID(current.ActivatedBy),
			optionalDate(current.TerminatedAt), optionalUUID(current.TerminatedBy),
			optionalString(current.TerminationReason), actor.ActorUserID,
		)
		if err := scanContract(row, &updated); err != nil {
			return err
		}
		if r.simulateAuditFailure {
			return errors.New("simulated contract audit failure")
		}
		return r.audit.InsertTx(ctx, tx, auditFromActor(domain.AuditEntityContract, updated.ID, action, actor, correlationID, map[string]any{"status": updated.Status}))
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

// SimulateActivateAuditFailureForTest forces rollback when audit insert would succeed after lifecycle mutation.
func (r *ContractRepository) SimulateActivateAuditFailureForTest(ctx context.Context, tenantID, contractID uuid.UUID, actor domain.ActorInput) error {
	r.simulateAuditFailure = true
	defer func() { r.simulateAuditFailure = false }()
	_, err := r.Activate(ctx, tenantID, contractID, actor, nil)
	return err
}

func (r *ContractRepository) maybeExpire(ctx context.Context, contract *domain.TransportContract) (bool, error) {
	var expired bool
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		var err error
		expired, err = r.maybeExpireTx(ctx, tx, contract, &domain.ActorInput{TenantID: contract.TenantID})
		return err
	})
	return expired, err
}

func (r *ContractRepository) maybeExpireTx(ctx context.Context, tx pgx.Tx, contract *domain.TransportContract, actor *domain.ActorInput) (bool, error) {
	changed, err := domain.ApplyLazyExpiration(contract, todayUTC())
	if err != nil || !changed {
		return false, err
	}
	const query = `
		UPDATE contract_rate.transport_contract SET
			status = $3, updated_at = now(), version = version + 1
		WHERE tenant_id = $1 AND id = $2`
	if _, err := tx.Exec(ctx, query, contract.TenantID, contract.ID, contract.Status); err != nil {
		return false, mapDBError(err)
	}
	auditActor := domain.ActorInput{TenantID: contract.TenantID}
	if actor != nil {
		auditActor = *actor
	}
	return true, r.audit.InsertTx(ctx, tx, auditFromActor(domain.AuditEntityContract, contract.ID, domain.AuditActionContractExpired, auditActor, nil, map[string]any{"status": contract.Status}))
}

func (r *ContractRepository) getByIDAndTenantTx(ctx context.Context, q queryRowProvider, tenantID, contractID uuid.UUID) (*domain.TransportContract, error) {
	const query = `
		SELECT id, tenant_id, buyer_company_id, carrier_company_id, contract_number,
			external_reference, name, description, status, valid_from, valid_to, currency_code,
			created_at, created_by, updated_at, updated_by, activated_at, activated_by,
			terminated_at, terminated_by, termination_reason, version
		FROM contract_rate.transport_contract
		WHERE tenant_id = $1 AND id = $2`
	var contract domain.TransportContract
	if err := scanContract(q.QueryRow(ctx, query, tenantID, contractID), &contract); err != nil {
		return nil, err
	}
	return &contract, nil
}

func (r *ContractRepository) getByIDAndTenantForUpdate(ctx context.Context, tx pgx.Tx, tenantID, contractID uuid.UUID) (*domain.TransportContract, error) {
	const query = `
		SELECT id, tenant_id, buyer_company_id, carrier_company_id, contract_number,
			external_reference, name, description, status, valid_from, valid_to, currency_code,
			created_at, created_by, updated_at, updated_by, activated_at, activated_by,
			terminated_at, terminated_by, termination_reason, version
		FROM contract_rate.transport_contract
		WHERE tenant_id = $1 AND id = $2
		FOR UPDATE`
	var contract domain.TransportContract
	if err := scanContract(tx.QueryRow(ctx, query, tenantID, contractID), &contract); err != nil {
		return nil, err
	}
	return &contract, nil
}

func (r *ContractRepository) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
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

type scannable interface {
	Scan(dest ...any) error
}

type queryRowProvider interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func scanContract(row scannable, out *domain.TransportContract) error {
	if err := row.Scan(
		&out.ID, &out.TenantID, &out.BuyerCompanyID, &out.CarrierCompanyID, &out.ContractNumber,
		&out.ExternalReference, &out.Name, &out.Description, &out.Status, &out.ValidFrom, &out.ValidTo, &out.CurrencyCode,
		&out.CreatedAt, &out.CreatedBy, &out.UpdatedAt, &out.UpdatedBy, &out.ActivatedAt, &out.ActivatedBy,
		&out.TerminatedAt, &out.TerminatedBy, &out.TerminationReason, &out.Version,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NotFound("contract not found")
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

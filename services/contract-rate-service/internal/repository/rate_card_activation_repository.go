package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
)

type activeLaneRow struct {
	RateCardID            uuid.UUID
	RateCardVersionID     uuid.UUID
	OriginLocationID      uuid.UUID
	DestinationLocationID uuid.UUID
	EquipmentType         string
	TransportMode         string
	ValidFrom             time.Time
	ValidTo               *time.Time
}

func (r *RateCardRepository) ActivateVersion(ctx context.Context, tenantID, versionID uuid.UUID, actor domain.ActorInput, correlationID *string) (*domain.RateCardVersion, error) {
	var activated domain.RateCardVersion
	err := r.withSerializableTx(ctx, func(tx pgx.Tx) error {
		target, err := r.getVersionForUpdate(ctx, tx, tenantID, versionID)
		if err != nil {
			return err
		}
		if target.Status == domain.RateVersionStatusActive {
			activated = *target
			return nil
		}
		if target.Status != domain.RateVersionStatusDraft {
			return apperrors.Validation("only DRAFT versions can be activated", map[string]any{"code": domain.ReasonInvalidRateVersion})
		}
		card, err := r.getRateCardForUpdate(ctx, tx, tenantID, target.RateCardID)
		if err != nil {
			return err
		}
		contract, err := r.contracts.getByIDAndTenantForUpdate(ctx, tx, tenantID, card.ContractID)
		if err != nil {
			return err
		}
		if err := domain.ValidateRateCardParentContract(contract); err != nil {
			return err
		}
		if contract.Status != domain.ContractStatusActive {
			return apperrors.Validation("contract must be ACTIVE to activate rate versions", map[string]any{
				"code": domain.ReasonInvalidRateVersion, "status": contract.Status,
			})
		}
		lines, err := r.listLinesTx(ctx, tx, tenantID, versionID)
		if err != nil {
			return err
		}
		if len(lines) == 0 {
			return apperrors.Validation("at least one rate line is required for activation", map[string]any{"code": domain.ReasonInvalidRateVersion})
		}
		componentsByLine, err := r.listComponentsByVersionTx(ctx, tx, tenantID, versionID)
		if err != nil {
			return err
		}
		for _, line := range lines {
			components := componentsByLine[line.ID]
			if err := domain.ValidateActivatableLineComponents(components); err != nil {
				return err
			}
		}
		if err := r.validateCrossCardLaneConflicts(ctx, tx, tenantID, card.ContractID, card.ID, target, lines); err != nil {
			return err
		}
		var previousActiveID *uuid.UUID
		var prevVersion domain.RateCardVersion
		if err := tx.QueryRow(ctx, `
			SELECT id, tenant_id, rate_card_id, version_number, valid_from, valid_to, status,
				supersedes_version_id, created_at, created_by, activated_at, activated_by, version
			FROM contract_rate.rate_card_version
			WHERE tenant_id = $1 AND rate_card_id = $2 AND status = 'ACTIVE'
			FOR UPDATE`, tenantID, card.ID).Scan(
			&prevVersion.ID, &prevVersion.TenantID, &prevVersion.RateCardID, &prevVersion.VersionNumber,
			&prevVersion.ValidFrom, &prevVersion.ValidTo, &prevVersion.Status, &prevVersion.SupersedesVersionID,
			&prevVersion.CreatedAt, &prevVersion.CreatedBy, &prevVersion.ActivatedAt, &prevVersion.ActivatedBy, &prevVersion.Version,
		); err == nil {
			previousActiveID = &prevVersion.ID
			if _, err := tx.Exec(ctx, `
				UPDATE contract_rate.rate_card_version
				SET status = 'SUPERSEDED', version = version + 1
				WHERE tenant_id = $1 AND id = $2`, tenantID, prevVersion.ID); err != nil {
				return mapDBError(err)
			}
			if err := r.audit.InsertTx(ctx, tx, auditFromActor(domain.AuditEntityRateVersion, prevVersion.ID, domain.AuditActionRateVersionSuperseded, actor, correlationID, map[string]any{
				"superseded_by_version_id": versionID.String(),
			})); err != nil {
				return err
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return mapDBError(err)
		}
		now := time.Now().UTC()
		supersedes := previousActiveID
		row := tx.QueryRow(ctx, `
			UPDATE contract_rate.rate_card_version SET
				status = 'ACTIVE', activated_at = $3, activated_by = $4,
				supersedes_version_id = $5, version = version + 1
			WHERE tenant_id = $1 AND id = $2
			RETURNING id, tenant_id, rate_card_id, version_number, valid_from, valid_to, status,
				supersedes_version_id, created_at, created_by, activated_at, activated_by, version`,
			tenantID, versionID, now, actor.ActorUserID, supersedes)
		if err := scanRateVersion(row, &activated); err != nil {
			return err
		}
		if r.simulateAuditFailure {
			return errors.New("simulated rate version audit failure")
		}
		return r.audit.InsertTx(ctx, tx, auditFromActor(domain.AuditEntityRateVersion, activated.ID, domain.AuditActionRateVersionActivated, actor, correlationID, map[string]any{
			"version_number": activated.VersionNumber,
		}))
	})
	if err != nil {
		return nil, err
	}
	return &activated, nil
}

func (r *RateCardRepository) validateCrossCardLaneConflicts(ctx context.Context, tx pgx.Tx, tenantID, contractID, activatingCardID uuid.UUID, target *domain.RateCardVersion, draftLines []domain.RateLine) error {
	activeLanes, err := r.loadActiveLanesForContractTx(ctx, tx, tenantID, contractID, activatingCardID)
	if err != nil {
		return err
	}
	for _, draftLine := range draftLines {
		for _, active := range activeLanes {
			if active.OriginLocationID != draftLine.OriginLocationID ||
				active.DestinationLocationID != draftLine.DestinationLocationID ||
				active.EquipmentType != draftLine.EquipmentType ||
				active.TransportMode != draftLine.TransportMode {
				continue
			}
			if domain.IntervalsOverlap(target.ValidFrom, target.ValidTo, active.ValidFrom, active.ValidTo) {
				return apperrors.Conflict("overlapping active lane exists on another rate card", map[string]any{"code": domain.ReasonRateLaneConflict})
			}
		}
	}
	return nil
}

func (r *RateCardRepository) loadActiveLanesForContractTx(ctx context.Context, tx pgx.Tx, tenantID, contractID, excludeCardID uuid.UUID) ([]activeLaneRow, error) {
	const query = `
		SELECT rc.id, v.id, l.origin_location_id, l.destination_location_id, l.equipment_type, l.transport_mode,
			v.valid_from, v.valid_to
		FROM contract_rate.rate_card rc
		JOIN contract_rate.rate_card_version v ON v.tenant_id = rc.tenant_id AND v.rate_card_id = rc.id
		JOIN contract_rate.rate_line l ON l.tenant_id = v.tenant_id AND l.rate_card_version_id = v.id
		WHERE rc.tenant_id = $1 AND rc.contract_id = $2 AND rc.id <> $3 AND v.status = 'ACTIVE'`
	rows, err := tx.Query(ctx, query, tenantID, contractID, excludeCardID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	result := make([]activeLaneRow, 0)
	for rows.Next() {
		var row activeLaneRow
		if err := rows.Scan(&row.RateCardID, &row.RateCardVersionID, &row.OriginLocationID, &row.DestinationLocationID,
			&row.EquipmentType, &row.TransportMode, &row.ValidFrom, &row.ValidTo); err != nil {
			return nil, mapDBError(err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *RateCardRepository) listLinesTx(ctx context.Context, tx pgx.Tx, tenantID, versionID uuid.UUID) ([]domain.RateLine, error) {
	const query = `
		SELECT id, tenant_id, rate_card_version_id, origin_location_id, destination_location_id,
			equipment_type, transport_mode, created_at, created_by, updated_at, updated_by
		FROM contract_rate.rate_line
		WHERE tenant_id = $1 AND rate_card_version_id = $2`
	rows, err := tx.Query(ctx, query, tenantID, versionID)
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

func (r *RateCardRepository) listComponentsByVersionTx(ctx context.Context, tx pgx.Tx, tenantID, versionID uuid.UUID) (map[uuid.UUID][]domain.RateComponent, error) {
	const query = `
		SELECT c.id, c.tenant_id, c.rate_line_id, c.component_type, c.calculation_method,
			c.amount, c.percent_value, c.unit_code, c.created_at, c.created_by, c.updated_at, c.updated_by
		FROM contract_rate.rate_component c
		JOIN contract_rate.rate_line l ON l.tenant_id = c.tenant_id AND l.id = c.rate_line_id
		WHERE c.tenant_id = $1 AND l.rate_card_version_id = $2`
	rows, err := tx.Query(ctx, query, tenantID, versionID)
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

func (r *RateCardRepository) withSerializableTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return mapDBError(err)
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "40001" {
			return apperrors.Conflict("activation conflict, retry required", map[string]any{"code": domain.ReasonRateLaneConflict})
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "40001" {
			return apperrors.Conflict("activation conflict, retry required", map[string]any{"code": domain.ReasonRateLaneConflict})
		}
		return mapDBError(err)
	}
	return nil
}

// SimulateActivateVersionAuditFailureForTest forces rollback after version activation mutation.
func (r *RateCardRepository) SimulateActivateVersionAuditFailureForTest(ctx context.Context, tenantID, versionID uuid.UUID, actor domain.ActorInput) error {
	r.simulateAuditFailure = true
	defer func() { r.simulateAuditFailure = false }()
	_, err := r.ActivateVersion(ctx, tenantID, versionID, actor, nil)
	return err
}

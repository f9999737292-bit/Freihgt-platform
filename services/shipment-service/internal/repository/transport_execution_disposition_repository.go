package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type dispositionCase struct {
	ID                  uuid.UUID
	OperatingTenantID   uuid.UUID
	ExecutionID         uuid.UUID
	SourceRevisionID    uuid.UUID
	SourceStopID        uuid.UUID
	SourceActionID      uuid.UUID
	ShipmentID          uuid.UUID
	ShipmentTenantID    uuid.UUID
	CargoID             uuid.UUID
	Accepted            int
	Rejected            int
	Pending             int
	ReturnReserved      int
	RedirectReserved    int
	HoldReserved        int
	Resolved            int
	UOM                 string
	ReasonCode          string
	DispositionType     string
	Status              string
	ReturnTarget        *uuid.UUID
	RedirectTarget      *uuid.UUID
	SuccessorRevisionID *uuid.UUID
	Version             int
}

// RecordDeliveryDisposition stores an acceptance fact and, when quantity is rejected, a disposition case.
// Onboard quantity is read from cargo.pallet_count. The client cannot supply it.
func (r *TransportExecutionRepository) RecordDeliveryDisposition(ctx context.Context, cmd domain.RecordDeliveryDispositionCommand) (domain.DispositionResult, error) {
	if err := cmd.Validate(); err != nil {
		return domain.DispositionResult{}, err
	}
	normalizeRecord(&cmd)
	digest, err := domain.RecordDispositionDigest(cmd)
	if err != nil {
		return domain.DispositionResult{}, apperrors.Internal("disposition digest failed", err)
	}
	tx, err := r.beginTx(ctx)
	if err != nil {
		return domain.DispositionResult{}, err
	}
	defer tx.Rollback(ctx)

	operating, currentRevision, err := lockExecution(ctx, tx, cmd.ExecutionID)
	if err != nil {
		return domain.DispositionResult{}, err
	}
	if cmd.OperatingTenantID != operating {
		return domain.DispositionResult{}, domain.ExecutionCommandError(domain.ReasonTenantDenied, true)
	}
	commandID := uuid.New()
	replay, err := claimDispositionCommand(ctx, tx, commandID, operating, cmd.ExecutionID, cmd.IdempotencyKey, domain.CommandRecordDeliveryDisposition, digest)
	if err != nil || replay != nil {
		if err != nil {
			return domain.DispositionResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.DispositionResult{}, mapDBError(err)
		}
		return *replay, nil
	}
	if currentRevision != cmd.ExpectedRevisionID {
		return domain.DispositionResult{}, domain.ExecutionCommandError(domain.ReasonRevisionConflict, false)
	}
	actionID, shipmentTenant, err := validateDeliveryAttempt(ctx, tx, cmd, currentRevision)
	if err != nil {
		return domain.DispositionResult{}, err
	}
	onboard, err := onboardBeforeStop(ctx, tx, cmd.CargoID, shipmentTenant, cmd.ExecutionID)
	if err != nil {
		return domain.DispositionResult{}, err
	}
	attempted := cmd.AcceptedQuantity + cmd.RejectedQuantity
	if attempted > onboard {
		return domain.DispositionResult{}, domain.ExecutionCommandError(domain.ReasonQuantityExceeded, false)
	}
	now := time.Now().UTC()
	var caseID *uuid.UUID
	status := "ACCEPTED"
	if cmd.RejectedQuantity > 0 {
		id := uuid.New()
		caseID = &id
		status = domain.DispositionStatusPending
		if _, err := tx.Exec(ctx, `
			INSERT INTO transport.delivery_disposition_cases (
				id, operating_tenant_id, execution_id, source_revision_id, source_execution_stop_id, source_action_id,
				shipment_id, shipment_tenant_id, cargo_id, handling_unit_id, pallet_id,
				attempted_quantity, accepted_quantity, rejected_quantity,
				pending_quantity, return_reserved_quantity, redirect_reserved_quantity, hold_reserved_quantity, resolved_quantity,
				uom, reason_code, reason_comment, disposition_type, status,
				created_by_actor_kind, created_by_actor_id, created_at, updated_at, version
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,NULL,NULL,
				$10,$11,$12,$12,0,0,0,0,
				$13,$14,$15,NULL,$16,$17,$18,$19,$19,1
			)
		`, id, operating, cmd.ExecutionID, currentRevision, cmd.SourceStopID, actionID,
			cmd.ShipmentID, shipmentTenant, cmd.CargoID,
			attempted, cmd.AcceptedQuantity, cmd.RejectedQuantity,
			domain.UOMPallet, cmd.ReasonCode, nullIfEmpty(cmd.ReasonComment), status,
			cmd.ActorKind, nullUUID(cmd.ActorID), now); err != nil {
			return domain.DispositionResult{}, mapDBError(err)
		}
		if err := insertDispositionEvidence(ctx, tx, id, operating, cmd.Evidence, now); err != nil {
			return domain.DispositionResult{}, err
		}
		if err := insertDispositionAudit(ctx, tx, id, commandID, operating, cmd.ExecutionID, cmd.ActorKind, cmd.ActorID, cmd.ReasonCode, "", status, cmd.OccurredAt); err != nil {
			return domain.DispositionResult{}, err
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO transport.delivery_attempt_facts (
			id, operating_tenant_id, execution_id, source_revision_id, source_execution_stop_id, source_action_id,
			shipment_id, cargo_id, attempted_quantity, accepted_quantity, rejected_quantity, uom, disposition_case_id
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`, uuid.New(), operating, cmd.ExecutionID, currentRevision, cmd.SourceStopID, actionID,
		cmd.ShipmentID, cmd.CargoID, attempted, cmd.AcceptedQuantity, cmd.RejectedQuantity, domain.UOMPallet, caseID); err != nil {
		return domain.DispositionResult{}, mapDBError(err)
	}
	result := domain.DispositionResult{
		CommandID: commandID, ExecutionID: cmd.ExecutionID, CaseID: caseID, Status: status,
		AcceptedQuantity: cmd.AcceptedQuantity, RejectedQuantity: cmd.RejectedQuantity, RevisionID: currentRevision,
	}
	if caseID != nil {
		row, err := loadDispositionCase(ctx, tx, *caseID, operating)
		if err != nil {
			return domain.DispositionResult{}, err
		}
		rejectionEvent := domain.EventDeliveryRejected
		if cmd.AcceptedQuantity > 0 {
			rejectionEvent = domain.EventDeliveryPartiallyRejected
		}
		if err := emitDisposition(ctx, tx, rejectionEvent, row, 1, nil, cmd.OccurredAt); err != nil {
			return domain.DispositionResult{}, err
		}
		if err := emitDisposition(ctx, tx, domain.EventCargoDispositionPending, row, 2, nil, cmd.OccurredAt); err != nil {
			return domain.DispositionResult{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE transport.delivery_disposition_cases SET version=2, updated_at=$2 WHERE id=$1`, *caseID, now); err != nil {
			return domain.DispositionResult{}, mapDBError(err)
		}
	}
	if err := storeDispositionResult(ctx, tx, commandID, result); err != nil {
		return domain.DispositionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DispositionResult{}, mapDBError(err)
	}
	return result, nil
}

func (r *TransportExecutionRepository) AuthorizeReturn(ctx context.Context, cmd domain.AuthorizeDispositionCommand) (domain.DispositionResult, error) {
	return r.authorizeRoute(ctx, cmd, domain.DispositionReturnToOrigin, domain.CommandAuthorizeReturn)
}

func (r *TransportExecutionRepository) AuthorizeRedirect(ctx context.Context, cmd domain.AuthorizeDispositionCommand) (domain.DispositionResult, error) {
	return r.authorizeRoute(ctx, cmd, domain.DispositionRedirect, domain.CommandAuthorizeRedirect)
}

func (r *TransportExecutionRepository) authorizeRoute(ctx context.Context, cmd domain.AuthorizeDispositionCommand, kind, commandName string) (domain.DispositionResult, error) {
	if err := cmd.Validate(); err != nil {
		return domain.DispositionResult{}, err
	}
	cmd.IdempotencyKey = strings.TrimSpace(cmd.IdempotencyKey)
	cmd.Instruction = strings.TrimSpace(cmd.Instruction)
	cmd.OccurredAt = cmd.OccurredAt.UTC()
	digest, err := domain.AuthorizeDispositionDigest(cmd, kind)
	if err != nil {
		return domain.DispositionResult{}, apperrors.Internal("disposition digest failed", err)
	}
	tx, err := r.beginTx(ctx)
	if err != nil {
		return domain.DispositionResult{}, err
	}
	defer tx.Rollback(ctx)
	operating, currentRevision, err := lockExecution(ctx, tx, cmd.ExecutionID)
	if err != nil {
		return domain.DispositionResult{}, err
	}
	if cmd.OperatingTenantID != operating {
		return domain.DispositionResult{}, domain.ExecutionCommandError(domain.ReasonTenantDenied, true)
	}
	commandID := uuid.New()
	replay, err := claimDispositionCommand(ctx, tx, commandID, operating, cmd.ExecutionID, cmd.IdempotencyKey, commandName, digest)
	if err != nil || replay != nil {
		if err != nil {
			return domain.DispositionResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.DispositionResult{}, mapDBError(err)
		}
		return *replay, nil
	}
	if currentRevision != cmd.ExpectedRevisionID {
		return domain.DispositionResult{}, domain.ExecutionCommandError(domain.ReasonRevisionConflict, false)
	}
	row, err := lockDispositionCase(ctx, tx, cmd.CaseID, operating, cmd.ExecutionID)
	if err != nil {
		return domain.DispositionResult{}, err
	}
	if row.Status != domain.DispositionStatusPending {
		return domain.DispositionResult{}, domain.ExecutionCommandError(domain.ReasonDispositionConflict, false)
	}
	if err := validateDispositionTarget(ctx, tx, cmd.TargetLocationID, operating, row, kind == domain.DispositionReturnToOrigin); err != nil {
		return domain.DispositionResult{}, err
	}
	successorCmd, err := buildDispositionSuccessor(ctx, tx, cmd, row, kind, digest)
	if err != nil {
		return domain.DispositionResult{}, err
	}
	successorDigest, err := domain.SuccessorDigest(successorCmd)
	if err != nil {
		return domain.DispositionResult{}, apperrors.Internal("successor digest failed", err)
	}
	successor, err := createSuccessorRevisionInTx(ctx, tx, successorCmd, successorDigest)
	if err != nil {
		return domain.DispositionResult{}, err
	}
	status := domain.DispositionStatusInReturnTransit
	eventType := domain.EventCargoReturnAuthorized
	returnTarget := &cmd.TargetLocationID
	var redirectTarget *uuid.UUID
	returnQty, redirectQty := row.Rejected, 0
	if kind == domain.DispositionRedirect {
		status = domain.DispositionStatusInRedirectTransit
		eventType = domain.EventCargoRedirectAuthorized
		returnTarget = nil
		redirectTarget = &cmd.TargetLocationID
		returnQty, redirectQty = 0, row.Rejected
	}
	now := time.Now().UTC()
	version := row.Version + 1
	if _, err := tx.Exec(ctx, `
		UPDATE transport.delivery_disposition_cases
		SET disposition_type=$2, status=$3, pending_quantity=0, return_reserved_quantity=$4,
		    redirect_reserved_quantity=$5, hold_reserved_quantity=0,
		    return_target_location_id=$6, redirect_target_location_id=$7, successor_revision_id=$8,
		    version=$9, updated_at=$10
		WHERE id=$1 AND status=$11
	`, row.ID, kind, status, returnQty, redirectQty, returnTarget, redirectTarget, successor.RevisionID, version, now, domain.DispositionStatusPending); err != nil {
		return domain.DispositionResult{}, mapDBError(err)
	}
	if err := insertDispositionAudit(ctx, tx, row.ID, commandID, operating, row.ExecutionID, cmd.ActorKind, cmd.ActorID, kind, row.Status, status, cmd.OccurredAt); err != nil {
		return domain.DispositionResult{}, err
	}
	updated, err := loadDispositionCase(ctx, tx, row.ID, operating)
	if err != nil {
		return domain.DispositionResult{}, err
	}
	if err := emitDisposition(ctx, tx, eventType, updated, version, &cmd.TargetLocationID, cmd.OccurredAt); err != nil {
		return domain.DispositionResult{}, err
	}
	result := dispositionResult(commandID, updated, successor.RevisionID)
	if err := storeDispositionResult(ctx, tx, commandID, result); err != nil {
		return domain.DispositionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DispositionResult{}, mapDBError(err)
	}
	return result, nil
}

func (r *TransportExecutionRepository) HoldDisposition(ctx context.Context, cmd domain.HoldDispositionCommand) (domain.DispositionResult, error) {
	if err := cmd.Validate(); err != nil {
		return domain.DispositionResult{}, err
	}
	cmd.IdempotencyKey = strings.TrimSpace(cmd.IdempotencyKey)
	cmd.OccurredAt = cmd.OccurredAt.UTC()
	digest, err := domain.HoldDispositionDigest(cmd)
	if err != nil {
		return domain.DispositionResult{}, apperrors.Internal("disposition digest failed", err)
	}
	tx, err := r.beginTx(ctx)
	if err != nil {
		return domain.DispositionResult{}, err
	}
	defer tx.Rollback(ctx)
	operating, currentRevision, err := lockExecution(ctx, tx, cmd.ExecutionID)
	if err != nil {
		return domain.DispositionResult{}, err
	}
	if cmd.OperatingTenantID != operating {
		return domain.DispositionResult{}, domain.ExecutionCommandError(domain.ReasonTenantDenied, true)
	}
	commandID := uuid.New()
	replay, err := claimDispositionCommand(ctx, tx, commandID, operating, cmd.ExecutionID, cmd.IdempotencyKey, domain.CommandHoldDisposition, digest)
	if err != nil || replay != nil {
		if err != nil {
			return domain.DispositionResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.DispositionResult{}, mapDBError(err)
		}
		return *replay, nil
	}
	if currentRevision != cmd.ExpectedRevisionID {
		return domain.DispositionResult{}, domain.ExecutionCommandError(domain.ReasonRevisionConflict, false)
	}
	before := revisionCount(ctx, tx, cmd.ExecutionID)
	row, err := lockDispositionCase(ctx, tx, cmd.CaseID, operating, cmd.ExecutionID)
	if err != nil {
		return domain.DispositionResult{}, err
	}
	if row.Status != domain.DispositionStatusPending {
		return domain.DispositionResult{}, domain.ExecutionCommandError(domain.ReasonDispositionConflict, false)
	}
	now := time.Now().UTC()
	version := row.Version + 1
	if _, err := tx.Exec(ctx, `
		UPDATE transport.delivery_disposition_cases
		SET disposition_type=$2, pending_quantity=0, hold_reserved_quantity=$3, version=$4, updated_at=$5
		WHERE id=$1 AND status=$6
	`, row.ID, domain.DispositionHold, row.Rejected, version, now, domain.DispositionStatusPending); err != nil {
		return domain.DispositionResult{}, mapDBError(err)
	}
	if err := insertDispositionAudit(ctx, tx, row.ID, commandID, operating, row.ExecutionID, cmd.ActorKind, cmd.ActorID, domain.DispositionHold, row.Status, domain.DispositionStatusPending, cmd.OccurredAt); err != nil {
		return domain.DispositionResult{}, err
	}
	updated, err := loadDispositionCase(ctx, tx, row.ID, operating)
	if err != nil {
		return domain.DispositionResult{}, err
	}
	if err := emitDisposition(ctx, tx, domain.EventCargoDispositionPending, updated, version, nil, cmd.OccurredAt); err != nil {
		return domain.DispositionResult{}, err
	}
	if revisionCount(ctx, tx, cmd.ExecutionID) != before {
		return domain.DispositionResult{}, apperrors.Internal("hold changed the execution revision", nil)
	}
	result := dispositionResult(commandID, updated, currentRevision)
	if err := storeDispositionResult(ctx, tx, commandID, result); err != nil {
		return domain.DispositionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DispositionResult{}, mapDBError(err)
	}
	return result, nil
}

func (r *TransportExecutionRepository) CompleteDisposition(ctx context.Context, cmd domain.CompleteDispositionCommand) (domain.DispositionResult, error) {
	if err := cmd.Validate(); err != nil {
		return domain.DispositionResult{}, err
	}
	cmd.IdempotencyKey = strings.TrimSpace(cmd.IdempotencyKey)
	cmd.OccurredAt = cmd.OccurredAt.UTC()
	digest, err := domain.CompleteDispositionDigest(cmd)
	if err != nil {
		return domain.DispositionResult{}, apperrors.Internal("disposition digest failed", err)
	}
	tx, err := r.beginTx(ctx)
	if err != nil {
		return domain.DispositionResult{}, err
	}
	defer tx.Rollback(ctx)
	operating, currentRevision, err := lockExecution(ctx, tx, cmd.ExecutionID)
	if err != nil {
		return domain.DispositionResult{}, err
	}
	if cmd.OperatingTenantID != operating {
		return domain.DispositionResult{}, domain.ExecutionCommandError(domain.ReasonTenantDenied, true)
	}
	commandID := uuid.New()
	replay, err := claimDispositionCommand(ctx, tx, commandID, operating, cmd.ExecutionID, cmd.IdempotencyKey, domain.CommandCompleteDisposition, digest)
	if err != nil || replay != nil {
		if err != nil {
			return domain.DispositionResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.DispositionResult{}, mapDBError(err)
		}
		return *replay, nil
	}
	if currentRevision != cmd.ExpectedRevisionID {
		return domain.DispositionResult{}, domain.ExecutionCommandError(domain.ReasonRevisionConflict, false)
	}
	row, err := lockDispositionCase(ctx, tx, cmd.CaseID, operating, cmd.ExecutionID)
	if err != nil {
		return domain.DispositionResult{}, err
	}
	eventType := ""
	switch row.Status {
	case domain.DispositionStatusInReturnTransit:
		eventType = domain.EventCargoReturnCompleted
	case domain.DispositionStatusInRedirectTransit:
		eventType = domain.EventCargoRedirectCompleted
	default:
		return domain.DispositionResult{}, domain.ExecutionCommandError(domain.ReasonCompletionEvidenceMissing, false)
	}
	if err := requireCompletionEvidence(ctx, tx, row, cmd.ActionID, currentRevision); err != nil {
		return domain.DispositionResult{}, err
	}
	now := time.Now().UTC()
	version := row.Version + 1
	if _, err := tx.Exec(ctx, `
		UPDATE transport.delivery_disposition_cases
		SET status=$2, pending_quantity=0, return_reserved_quantity=0, redirect_reserved_quantity=0,
		    hold_reserved_quantity=0, resolved_quantity=$3, version=$4, updated_at=$5
		WHERE id=$1
	`, row.ID, domain.DispositionStatusResolved, row.Rejected, version, now); err != nil {
		return domain.DispositionResult{}, mapDBError(err)
	}
	if err := insertDispositionAudit(ctx, tx, row.ID, commandID, operating, row.ExecutionID, cmd.ActorKind, cmd.ActorID, row.ReasonCode, row.Status, domain.DispositionStatusResolved, cmd.OccurredAt); err != nil {
		return domain.DispositionResult{}, err
	}
	updated, err := loadDispositionCase(ctx, tx, row.ID, operating)
	if err != nil {
		return domain.DispositionResult{}, err
	}
	target := updated.ReturnTarget
	if updated.DispositionType == domain.DispositionRedirect {
		target = updated.RedirectTarget
	}
	if err := emitDisposition(ctx, tx, eventType, updated, version, target, cmd.OccurredAt); err != nil {
		return domain.DispositionResult{}, err
	}
	result := dispositionResult(commandID, updated, currentRevision)
	if err := storeDispositionResult(ctx, tx, commandID, result); err != nil {
		return domain.DispositionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DispositionResult{}, mapDBError(err)
	}
	return result, nil
}

func (r *TransportExecutionRepository) ListDeliveryDispositions(ctx context.Context, operatingTenant, executionID uuid.UUID) ([]domain.DispositionView, error) {
	if r == nil || r.pool == nil {
		return nil, apperrors.NotFound("transport execution not found")
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, execution_id, source_execution_stop_id, shipment_id, cargo_id,
		       accepted_quantity::int, rejected_quantity::int, uom, reason_code,
		       COALESCE(disposition_type, ''), status, return_target_location_id, redirect_target_location_id, version
		FROM transport.delivery_disposition_cases
		WHERE operating_tenant_id=$1 AND execution_id=$2
		ORDER BY created_at, id
	`, operatingTenant, executionID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	var out []domain.DispositionView
	for rows.Next() {
		var view domain.DispositionView
		var returnTarget, redirectTarget *uuid.UUID
		if err := rows.Scan(&view.CaseID, &view.ExecutionID, &view.SourceStopID, &view.ShipmentID, &view.CargoID,
			&view.AcceptedQuantity, &view.RejectedQuantity, &view.UOM, &view.ReasonCode,
			&view.DispositionType, &view.Status, &returnTarget, &redirectTarget, &view.Version); err != nil {
			return nil, mapDBError(err)
		}
		view.TargetLocationID = returnTarget
		if view.DispositionType == domain.DispositionRedirect {
			view.TargetLocationID = redirectTarget
		}
		out = append(out, view)
	}
	return out, mapDBError(rows.Err())
}

func (r *TransportExecutionRepository) beginTx(ctx context.Context) (pgx.Tx, error) {
	if r == nil || r.pool == nil {
		return nil, apperrors.NotFound("transport execution not found")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, mapDBError(err)
	}
	return tx, nil
}

func lockExecution(ctx context.Context, tx pgx.Tx, executionID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	var operating, revision uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT operating_tenant_id, current_revision_id
		FROM transport.transport_executions
		WHERE id=$1
		FOR UPDATE
	`, executionID).Scan(&operating, &revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, apperrors.NotFound("transport execution not found")
	}
	if err != nil {
		return uuid.Nil, uuid.Nil, mapDBError(err)
	}
	return operating, revision, nil
}

func claimDispositionCommand(ctx context.Context, tx pgx.Tx, commandID, operating, executionID uuid.UUID, key, name, digest string) (*domain.DispositionResult, error) {
	tag, err := tx.Exec(ctx, `
		INSERT INTO transport.delivery_disposition_commands (
			id, operating_tenant_id, execution_id, idempotency_key, command_name, request_sha256, result_json
		) VALUES ($1,$2,$3,$4,$5,$6,'{}'::jsonb)
		ON CONFLICT (operating_tenant_id, idempotency_key) DO NOTHING
	`, commandID, operating, executionID, key, name, digest)
	if err != nil {
		return nil, mapDBError(err)
	}
	if tag.RowsAffected() == 1 {
		return nil, nil
	}
	var storedName, storedDigest string
	var raw []byte
	if err := tx.QueryRow(ctx, `
		SELECT command_name, request_sha256, result_json
		FROM transport.delivery_disposition_commands
		WHERE operating_tenant_id=$1 AND idempotency_key=$2
	`, operating, key).Scan(&storedName, &storedDigest, &raw); err != nil {
		return nil, mapDBError(err)
	}
	if storedName != name || storedDigest != digest {
		return nil, domain.ExecutionCommandError(domain.ReasonCommandBodyConflict, false)
	}
	var result domain.DispositionResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, apperrors.Internal("decode disposition result", err)
	}
	result.Replayed = true
	return &result, nil
}

func validateDeliveryAttempt(ctx context.Context, tx pgx.Tx, cmd domain.RecordDeliveryDispositionCommand, revisionID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	var stopStatus string
	err := tx.QueryRow(ctx, `
		SELECT s.status
		FROM transport.transport_execution_revision_stops rs
		JOIN transport.transport_execution_stops s ON s.id = rs.stop_id
		WHERE rs.revision_id=$1 AND rs.stop_id=$2 AND rs.membership <> 'SUPERSEDED' AND s.execution_id=$3
	`, revisionID, cmd.SourceStopID, cmd.ExecutionID).Scan(&stopStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, domain.ExecutionCommandError(domain.ReasonStopNotInRevision, false)
	}
	if err != nil {
		return uuid.Nil, uuid.Nil, mapDBError(err)
	}
	switch stopStatus {
	case domain.StopStatusArrived, domain.StopStatusServiceStarted, domain.StopStatusCompleted:
	default:
		return uuid.Nil, uuid.Nil, domain.ExecutionCommandError(domain.ReasonStopNotReady, false)
	}
	var shipmentTenant uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT shipment_tenant_id
		FROM transport.transport_execution_participants
		WHERE execution_id=$1 AND shipment_id=$2 AND cargo_id=$3
	`, cmd.ExecutionID, cmd.ShipmentID, cmd.CargoID).Scan(&shipmentTenant)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, domain.ExecutionCommandError(domain.ReasonNotParticipant, false)
	}
	if err != nil {
		return uuid.Nil, uuid.Nil, mapDBError(err)
	}
	var actionID uuid.UUID
	var actionStatus string
	err = tx.QueryRow(ctx, `
		SELECT id, status
		FROM transport.transport_execution_actions
		WHERE execution_stop_id=$1 AND shipment_id=$2 AND cargo_id=$3 AND action_type='DELIVERY'
		ORDER BY ordinal
		LIMIT 1
	`, cmd.SourceStopID, cmd.ShipmentID, cmd.CargoID).Scan(&actionID, &actionStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, domain.ExecutionCommandError(domain.ReasonActionNotFound, false)
	}
	if err != nil {
		return uuid.Nil, uuid.Nil, mapDBError(err)
	}
	if actionStatus == domain.ActionStatusFailed || actionStatus == domain.ActionStatusCancelled {
		return uuid.Nil, uuid.Nil, domain.ExecutionCommandError(domain.ReasonActionNotFound, false)
	}
	onboard, err := cargoOnboard(ctx, tx, cmd.ExecutionID, cmd.ShipmentID, cmd.CargoID)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if !onboard {
		return uuid.Nil, uuid.Nil, domain.ExecutionCommandError(domain.ReasonCargoNotOnboard, false)
	}
	return actionID, shipmentTenant, nil
}

func cargoOnboard(ctx context.Context, tx pgx.Tx, executionID, shipmentID, cargoID uuid.UUID) (bool, error) {
	var picked bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM transport.transport_execution_actions a
			JOIN transport.transport_execution_stops s ON s.id = a.execution_stop_id
			WHERE s.execution_id=$1 AND a.cargo_id=$2 AND a.action_type='PICKUP' AND a.status='COMPLETED'
		) OR EXISTS (
			SELECT 1 FROM (
				SELECT state
				FROM transport.shipment_cargo_execution_evidence
				WHERE shipment_id=$3 AND cargo_id=$2
				ORDER BY state_version DESC
				LIMIT 1
			) latest
			WHERE state IN ('PICKED_UP', 'CONFIRMED_ONBOARD')
		)
	`, executionID, cargoID, shipmentID).Scan(&picked); err != nil {
		return false, mapDBError(err)
	}
	return picked, nil
}

func onboardBeforeStop(ctx context.Context, tx pgx.Tx, cargoID, shipmentTenant, executionID uuid.UUID) (int, error) {
	var pallets *int
	err := tx.QueryRow(ctx, `
		SELECT pallet_count FROM transport.cargoes WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL
	`, cargoID, shipmentTenant).Scan(&pallets)
	if errors.Is(err, pgx.ErrNoRows) || pallets == nil || *pallets <= 0 {
		return 0, domain.ExecutionCommandError(domain.ReasonQuantityUnknown, false)
	}
	if err != nil {
		return 0, mapDBError(err)
	}
	var accepted float64
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(accepted_quantity), 0)::float8
		FROM transport.delivery_attempt_facts
		WHERE execution_id=$1 AND cargo_id=$2
	`, executionID, cargoID).Scan(&accepted); err != nil {
		return 0, mapDBError(err)
	}
	if accepted != float64(int(accepted)) {
		return 0, domain.ExecutionCommandError(domain.ReasonQuantityInvalid, false)
	}
	remaining := *pallets - int(accepted)
	if remaining < 0 {
		return 0, domain.ExecutionCommandError(domain.ReasonQuantityExceeded, false)
	}
	return remaining, nil
}

func validateDispositionTarget(ctx context.Context, tx pgx.Tx, locationID, operating uuid.UUID, row dispositionCase, returnToOrigin bool) error {
	var tenant uuid.UUID
	var locationType string
	var deleted bool
	var lat, lon *float64
	err := tx.QueryRow(ctx, `
		SELECT tenant_id, location_type, deleted_at IS NOT NULL, lat::float8, lon::float8
		FROM transport.locations WHERE id=$1
	`, locationID).Scan(&tenant, &locationType, &deleted, &lat, &lon)
	if errors.Is(err, pgx.ErrNoRows) || deleted {
		return domain.ExecutionCommandError(domain.ReasonLocationDenied, false)
	}
	if err != nil {
		return mapDBError(err)
	}
	var origin uuid.UUID
	var shipmentTenant uuid.UUID
	if err := tx.QueryRow(ctx, `
		SELECT origin_location_id, tenant_id FROM transport.shipments WHERE id=$1
	`, row.ShipmentID).Scan(&origin, &shipmentTenant); err != nil {
		return mapDBError(err)
	}
	if shipmentTenant != row.ShipmentTenantID {
		return domain.ExecutionCommandError(domain.ReasonNotParticipant, false)
	}
	owned := tenant == operating || tenant == shipmentTenant
	if returnToOrigin {
		warehouse := locationType == "WAREHOUSE" || locationType == "DISTRIBUTION_CENTER" || locationType == "FACTORY"
		if locationID != origin && !(owned && warehouse) {
			return domain.ExecutionCommandError(domain.ReasonLocationDenied, false)
		}
	} else if !owned {
		return domain.ExecutionCommandError(domain.ReasonLocationDenied, false)
	}
	if lat == nil || lon == nil {
		return domain.ExecutionCommandError(domain.ReasonLocationDenied, false)
	}
	return nil
}

func buildDispositionSuccessor(ctx context.Context, tx pgx.Tx, cmd domain.AuthorizeDispositionCommand, row dispositionCase, kind, digest string) (domain.SuccessorRevisionCommand, error) {
	open, err := openFutureStops(ctx, tx, cmd.ExpectedRevisionID)
	if err != nil {
		return domain.SuccessorRevisionCommand{}, err
	}
	if err := matchFutureStops(open, cmd.FutureStopIDs); err != nil {
		return domain.SuccessorRevisionCommand{}, err
	}
	insertAt := len(cmd.FutureStopIDs)
	if cmd.InsertAt != nil {
		insertAt = *cmd.InsertAt
	}
	if insertAt > len(cmd.FutureStopIDs) {
		return domain.SuccessorRevisionCommand{}, apperrors.Validation("insert_at is invalid", map[string]any{"field": "insert_at"})
	}
	stops := make([]domain.SuccessorStopInput, 0, len(cmd.FutureStopIDs)+1)
	actions := make([]domain.SuccessorActionInput, 0)
	for _, stopID := range cmd.FutureStopIDs {
		stop, stopActions, err := cloneOpenStop(ctx, tx, cmd.ExecutionID, stopID)
		if err != nil {
			return domain.SuccessorRevisionCommand{}, err
		}
		index := len(stops)
		stops = append(stops, stop)
		for _, action := range stopActions {
			action.StopIndex = index
			actions = append(actions, action)
		}
	}
	targetStop, err := newCanonicalStop(ctx, tx, cmd.TargetLocationID)
	if err != nil {
		return domain.SuccessorRevisionCommand{}, err
	}
	stops = append(stops, domain.SuccessorStopInput{})
	copy(stops[insertAt+1:], stops[insertAt:])
	stops[insertAt] = targetStop
	for i := range actions {
		if actions[i].StopIndex >= insertAt {
			actions[i].StopIndex++
		}
	}
	actions = append(actions, domain.SuccessorActionInput{
		StopIndex: insertAt, ActionType: domain.ActionTypeDelivery, ShipmentID: row.ShipmentID, CargoID: row.CargoID,
	})
	activation := uuid.NewSHA1(uuid.NameSpaceOID, []byte(digest))
	return domain.SuccessorRevisionCommand{
		ExecutionID: cmd.ExecutionID, ExpectedRevisionID: cmd.ExpectedRevisionID, OperatingTenantID: cmd.OperatingTenantID,
		IdempotencyKey: "succ-" + digest[:40], ReasonCode: kind, OccurredAt: cmd.OccurredAt,
		ActorKind: cmd.ActorKind, ActorID: cmd.ActorID,
		SourceRoutePlanID: row.ID, SourceRoutePlanVersion: 1,
		SourceActivationID: activation, SourceActivationVersion: 1,
		PlanningMode: domain.PlanningModeCurrentTrip, EvaluationFingerprint: domain.DispositionFingerprint,
		Stops: stops, Actions: actions,
	}, nil
}

func openFutureStops(ctx context.Context, tx pgx.Tx, revisionID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx, `
		SELECT s.id
		FROM transport.transport_execution_revision_stops rs
		JOIN transport.transport_execution_stops s ON s.id = rs.stop_id
		WHERE rs.revision_id=$1 AND rs.membership <> 'SUPERSEDED' AND s.status IN ('PLANNED', 'ARRIVED')
		ORDER BY s.ordinal
	`, revisionID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, mapDBError(err)
		}
		ids = append(ids, id)
	}
	return ids, mapDBError(rows.Err())
}

func matchFutureStops(open, requested []uuid.UUID) error {
	seen := map[uuid.UUID]int{}
	for _, id := range requested {
		seen[id]++
		if seen[id] > 1 {
			return domain.ExecutionCommandError(domain.ReasonFutureStopUnknown, false)
		}
	}
	openSet := map[uuid.UUID]struct{}{}
	for _, id := range open {
		openSet[id] = struct{}{}
		if seen[id] == 0 {
			return domain.ExecutionCommandError(domain.ReasonFutureStopOmitted, false)
		}
	}
	for id := range seen {
		if _, ok := openSet[id]; !ok {
			return domain.ExecutionCommandError(domain.ReasonFutureStopUnknown, false)
		}
	}
	return nil
}

func cloneOpenStop(ctx context.Context, tx pgx.Tx, executionID, stopID uuid.UUID) (domain.SuccessorStopInput, []domain.SuccessorActionInput, error) {
	var stop domain.SuccessorStopInput
	err := tx.QueryRow(ctx, `
		SELECT stop_role, point_kind, location_id, latitude, longitude, planned_arrival, planned_departure, service_duration_seconds
		FROM transport.transport_execution_stops
		WHERE id=$1 AND execution_id=$2
	`, stopID, executionID).Scan(&stop.StopRole, &stop.PointKind, &stop.LocationID, &stop.Latitude, &stop.Longitude, &stop.PlannedArrival, &stop.PlannedDeparture, &stop.ServiceDurationSeconds)
	if err != nil {
		return domain.SuccessorStopInput{}, nil, mapDBError(err)
	}
	rows, err := tx.Query(ctx, `
		SELECT shipment_id, cargo_id, action_type
		FROM transport.transport_execution_actions
		WHERE execution_stop_id=$1 AND status='PENDING'
		ORDER BY ordinal
	`, stopID)
	if err != nil {
		return domain.SuccessorStopInput{}, nil, mapDBError(err)
	}
	defer rows.Close()
	var actions []domain.SuccessorActionInput
	for rows.Next() {
		var action domain.SuccessorActionInput
		if err := rows.Scan(&action.ShipmentID, &action.CargoID, &action.ActionType); err != nil {
			return domain.SuccessorStopInput{}, nil, mapDBError(err)
		}
		actions = append(actions, action)
	}
	return stop, actions, mapDBError(rows.Err())
}

func newCanonicalStop(ctx context.Context, tx pgx.Tx, locationID uuid.UUID) (domain.SuccessorStopInput, error) {
	var lat, lon float64
	if err := tx.QueryRow(ctx, `
		SELECT lat::float8, lon::float8 FROM transport.locations WHERE id=$1 AND lat IS NOT NULL AND lon IS NOT NULL
	`, locationID).Scan(&lat, &lon); err != nil {
		return domain.SuccessorStopInput{}, domain.ExecutionCommandError(domain.ReasonLocationDenied, false)
	}
	return domain.SuccessorStopInput{
		StopRole: domain.StopRoleCargo, PointKind: domain.PointKindCanonicalLocation, LocationID: &locationID,
		Latitude: lat, Longitude: lon,
	}, nil
}

func requireCompletionEvidence(ctx context.Context, tx pgx.Tx, row dispositionCase, actionID, revisionID uuid.UUID) error {
	if row.SuccessorRevisionID == nil {
		return domain.ExecutionCommandError(domain.ReasonCompletionEvidenceMissing, false)
	}
	target := row.ReturnTarget
	if row.DispositionType == domain.DispositionRedirect {
		target = row.RedirectTarget
	}
	if target == nil {
		return domain.ExecutionCommandError(domain.ReasonCompletionEvidenceMissing, false)
	}
	var ok bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM transport.transport_execution_actions a
			JOIN transport.transport_execution_stops s ON s.id = a.execution_stop_id
			JOIN transport.transport_execution_revision_actions ra ON ra.action_id = a.id
			WHERE a.id=$1 AND a.status='COMPLETED' AND a.cargo_id=$2 AND a.shipment_id=$3
			  AND a.execution_stop_id <> $4
			  AND s.location_id=$5 AND s.execution_id=$6
			  AND ra.revision_id=$7 AND ra.membership='INTRODUCED'
		)
	`, actionID, row.CargoID, row.ShipmentID, row.SourceStopID, *target, row.ExecutionID, revisionID).Scan(&ok)
	if err != nil {
		return mapDBError(err)
	}
	if !ok {
		return domain.ExecutionCommandError(domain.ReasonCompletionEvidenceMissing, false)
	}
	return nil
}

func lockDispositionCase(ctx context.Context, tx pgx.Tx, caseID, operating, executionID uuid.UUID) (dispositionCase, error) {
	var locked uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id FROM transport.delivery_disposition_cases
		WHERE id=$1 AND operating_tenant_id=$2 AND execution_id=$3
		FOR UPDATE
	`, caseID, operating, executionID).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return dispositionCase{}, apperrors.NotFound("delivery disposition not found")
	}
	if err != nil {
		return dispositionCase{}, mapDBError(err)
	}
	row, err := loadDispositionCase(ctx, tx, caseID, operating)
	if errors.Is(err, pgx.ErrNoRows) {
		return dispositionCase{}, apperrors.NotFound("delivery disposition not found")
	}
	if err != nil {
		return dispositionCase{}, err
	}
	if row.ExecutionID != executionID {
		return dispositionCase{}, apperrors.NotFound("delivery disposition not found")
	}
	return row, nil
}

func loadDispositionCase(ctx context.Context, tx pgx.Tx, caseID, operating uuid.UUID) (dispositionCase, error) {
	var row dispositionCase
	err := tx.QueryRow(ctx, `
		SELECT id, operating_tenant_id, execution_id, source_revision_id, source_execution_stop_id, source_action_id,
		       shipment_id, shipment_tenant_id, cargo_id,
		       accepted_quantity::int, rejected_quantity::int, pending_quantity::int,
		       return_reserved_quantity::int, redirect_reserved_quantity::int, hold_reserved_quantity::int, resolved_quantity::int,
		       uom, reason_code, COALESCE(disposition_type, ''), status,
		       return_target_location_id, redirect_target_location_id, successor_revision_id, version
		FROM transport.delivery_disposition_cases
		WHERE id=$1 AND operating_tenant_id=$2
	`, caseID, operating).Scan(
		&row.ID, &row.OperatingTenantID, &row.ExecutionID, &row.SourceRevisionID, &row.SourceStopID, &row.SourceActionID,
		&row.ShipmentID, &row.ShipmentTenantID, &row.CargoID,
		&row.Accepted, &row.Rejected, &row.Pending, &row.ReturnReserved, &row.RedirectReserved, &row.HoldReserved, &row.Resolved,
		&row.UOM, &row.ReasonCode, &row.DispositionType, &row.Status,
		&row.ReturnTarget, &row.RedirectTarget, &row.SuccessorRevisionID, &row.Version,
	)
	if err != nil {
		return dispositionCase{}, mapDBError(err)
	}
	return row, nil
}

func insertDispositionEvidence(ctx context.Context, tx pgx.Tx, caseID, operating uuid.UUID, items []domain.DeliveryEvidenceRef, now time.Time) error {
	for _, item := range items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO transport.delivery_disposition_evidence (
				id, case_id, operating_tenant_id, evidence_type, source, reference_id, created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7)
		`, uuid.New(), caseID, operating, strings.ToUpper(strings.TrimSpace(item.EvidenceType)), strings.TrimSpace(item.Source), strings.TrimSpace(item.ReferenceID), now); err != nil {
			return mapDBError(err)
		}
	}
	return nil
}

func insertDispositionAudit(ctx context.Context, tx pgx.Tx, caseID, commandID, operating, executionID uuid.UUID, actorKind string, actorID uuid.UUID, reason, previous, next string, occurred time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO transport.delivery_disposition_audit (
			id, case_id, command_id, operating_tenant_id, execution_id, actor_kind, actor_id,
			reason_code, previous_status, new_status, occurred_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, uuid.New(), caseID, commandID, operating, executionID, actorKind, nullUUID(actorID), nullIfEmpty(reason), nullIfEmpty(previous), next, occurred.UTC())
	return mapDBError(err)
}

func emitDisposition(ctx context.Context, tx pgx.Tx, eventType string, row dispositionCase, sequence int, target *uuid.UUID, occurred time.Time) error {
	body := map[string]any{
		"event_id":                 uuid.NewString(),
		"event_type":               eventType,
		"operating_tenant_id":      row.OperatingTenantID.String(),
		"execution_id":             row.ExecutionID.String(),
		"source_execution_stop_id": row.SourceStopID.String(),
		"shipment_id":              row.ShipmentID.String(),
		"cargo_id":                 row.CargoID.String(),
		"disposition_case_id":      row.ID.String(),
		"reason_code":              row.ReasonCode,
		"accepted_quantity":        row.Accepted,
		"rejected_quantity":        row.Rejected,
		"uom":                      row.UOM,
		"disposition_type":         row.DispositionType,
		"status":                   row.Status,
		"disposition_sequence":     sequence,
		"occurred_at":              occurred.UTC().Format(time.RFC3339Nano),
	}
	if target != nil {
		body["target_location_id"] = target.String()
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return apperrors.Internal("marshal disposition event", err)
	}
	headers, err := json.Marshal(map[string]string{"contentType": "application/json", "eventType": eventType})
	if err != nil {
		return apperrors.Internal("marshal disposition headers", err)
	}
	eventID := uuid.New()
	return executionOutboxInsert(ctx, tx, domain.ShipmentOutboxEvent{
		ID: eventID, TenantID: row.OperatingTenantID, AggregateType: "DELIVERY_DISPOSITION",
		AggregateID: row.ID, AggregateVersion: sequence, EventType: eventType, SchemaVersion: 1,
		SourceEventID: eventID, Payload: payload, Headers: headers,
		Status: domain.OutboxStatusPending, AvailableAt: time.Now().UTC(),
	})
}

func storeDispositionResult(ctx context.Context, tx pgx.Tx, commandID uuid.UUID, result domain.DispositionResult) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return apperrors.Internal("marshal disposition result", err)
	}
	_, err = tx.Exec(ctx, `UPDATE transport.delivery_disposition_commands SET result_json=$2 WHERE id=$1`, commandID, raw)
	return mapDBError(err)
}

func dispositionResult(commandID uuid.UUID, row dispositionCase, revisionID uuid.UUID) domain.DispositionResult {
	caseID := row.ID
	return domain.DispositionResult{
		CommandID: commandID, ExecutionID: row.ExecutionID, CaseID: &caseID, Status: row.Status,
		AcceptedQuantity: row.Accepted, RejectedQuantity: row.Rejected, RevisionID: revisionID,
	}
}

func revisionCount(ctx context.Context, tx pgx.Tx, executionID uuid.UUID) int {
	var n int
	_ = tx.QueryRow(ctx, `SELECT COUNT(*) FROM transport.transport_execution_revisions WHERE execution_id=$1`, executionID).Scan(&n)
	return n
}

func normalizeRecord(cmd *domain.RecordDeliveryDispositionCommand) {
	cmd.IdempotencyKey = strings.TrimSpace(cmd.IdempotencyKey)
	cmd.UOM = strings.ToUpper(strings.TrimSpace(cmd.UOM))
	cmd.ReasonCode = strings.ToUpper(strings.TrimSpace(cmd.ReasonCode))
	cmd.ReasonComment = strings.TrimSpace(cmd.ReasonComment)
	cmd.OccurredAt = cmd.OccurredAt.UTC()
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}

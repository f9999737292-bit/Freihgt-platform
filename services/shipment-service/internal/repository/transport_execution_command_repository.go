package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

// executionOutboxInsert writes one outbox row inside the caller transaction.
// Tests replace it to prove a failed outbox insert rolls the command back.
var executionOutboxInsert = insertOutboxRow

func SetExecutionOutboxInsertForTest(fn func(context.Context, pgx.Tx, domain.ShipmentOutboxEvent) error) func() {
	previous := executionOutboxInsert
	if fn == nil {
		executionOutboxInsert = insertOutboxRow
	} else {
		executionOutboxInsert = fn
	}
	return func() { executionOutboxInsert = previous }
}

type TransportExecutionCommandRepository struct {
	pool *pgxpool.Pool
}

func NewTransportExecutionCommandRepository(pool *pgxpool.Pool) *TransportExecutionCommandRepository {
	return &TransportExecutionCommandRepository{pool: pool}
}

type commandScope struct {
	cmd             domain.ExecutionCommand
	commandID       uuid.UUID
	operatingTenant uuid.UUID
	driverID        *uuid.UUID
	revisionID      uuid.UUID
	revisionVersion int
	skipped         []int
}

func (r *TransportExecutionCommandRepository) Execute(ctx context.Context, cmd domain.ExecutionCommand) (domain.ExecutionCommandResult, error) {
	if err := cmd.Validate(); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.IdempotencyKey = strings.TrimSpace(cmd.IdempotencyKey)
	cmd.ReasonCode = strings.TrimSpace(strings.ToUpper(cmd.ReasonCode))
	cmd.OccurredAt = cmd.OccurredAt.UTC()
	digest, err := domain.CommandDigest(cmd)
	if err != nil {
		return domain.ExecutionCommandResult{}, apperrors.Internal("command digest failed", err)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.ExecutionCommandResult{}, mapDBError(err)
	}
	defer tx.Rollback(ctx)

	var operating uuid.UUID
	var driverID *uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT operating_tenant_id, driver_id
		FROM transport.transport_executions
		WHERE id = $1
		FOR UPDATE
	`, cmd.ExecutionID).Scan(&operating, &driverID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ExecutionCommandResult{}, apperrors.NotFound("transport execution not found")
	}
	if err != nil {
		return domain.ExecutionCommandResult{}, mapDBError(err)
	}
	if err := authorizeCommand(cmd, operating, driverID); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	var revisionVersion int
	err = tx.QueryRow(ctx, `
		SELECT version
		FROM transport.transport_execution_revisions
		WHERE id = $1 AND execution_id = $2 AND operating_tenant_id = $3 AND status = 'ACTIVE'
	`, cmd.RevisionID, cmd.ExecutionID, operating).Scan(&revisionVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonStaleRevision, false)
	}
	if err != nil {
		return domain.ExecutionCommandResult{}, mapDBError(err)
	}

	commandID := uuid.New()
	tag, err := tx.Exec(ctx, `
		INSERT INTO transport.transport_execution_commands (
			id, operating_tenant_id, execution_id, idempotency_key, command_name, request_sha256, result_json
		) VALUES ($1,$2,$3,$4,$5,$6,'{}'::jsonb)
		ON CONFLICT (operating_tenant_id, idempotency_key) DO NOTHING
	`, commandID, operating, cmd.ExecutionID, cmd.IdempotencyKey, cmd.Name, digest)
	if err != nil {
		return domain.ExecutionCommandResult{}, mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return loadReplay(ctx, tx, operating, cmd.IdempotencyKey, digest)
	}

	scope := commandScope{
		cmd:             cmd,
		commandID:       commandID,
		operatingTenant: operating,
		driverID:        driverID,
		revisionID:      cmd.RevisionID,
		revisionVersion: revisionVersion,
	}
	before, err := currentStopID(ctx, tx, scope.revisionID, cmd.ExecutionID)
	if err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	result, err := applyCommand(ctx, tx, &scope)
	if err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	after, err := currentStopID(ctx, tx, scope.revisionID, cmd.ExecutionID)
	if err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if before != after && after != uuid.Nil && cmd.Name != domain.CommandArriveStop {
		if err := emitExecutionEvent(ctx, tx, scope, domain.EventRouteStopCurrent, after, uuid.Nil, nil); err != nil {
			return domain.ExecutionCommandResult{}, err
		}
	}
	result.CommandID = commandID
	result.ExecutionID = cmd.ExecutionID
	result.RevisionID = cmd.RevisionID
	result.Replayed = false
	raw, err := json.Marshal(result)
	if err != nil {
		return domain.ExecutionCommandResult{}, apperrors.Internal("marshal command result", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE transport.transport_execution_commands SET result_json = $2 WHERE id = $1
	`, commandID, raw); err != nil {
		return domain.ExecutionCommandResult{}, mapDBError(err)
	}
	if err := insertCommandAudit(ctx, tx, scope, result); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ExecutionCommandResult{}, mapDBError(err)
	}
	return result, nil
}

func authorizeCommand(cmd domain.ExecutionCommand, operating uuid.UUID, driverID *uuid.UUID) error {
	if cmd.OperatingTenantID != operating {
		return domain.ExecutionCommandError(domain.ReasonTenantDenied, true)
	}
	switch cmd.Name {
	case domain.CommandSkipStop, domain.CommandCancelInServiceStop:
		if cmd.ActorKind != domain.ActorKindOperator {
			return domain.ExecutionCommandError(domain.ReasonActorDenied, true)
		}
	case domain.CommandCancelRemainingStops:
		if cmd.ActorKind == domain.ActorKindDriver {
			return domain.ExecutionCommandError(domain.ReasonActorDenied, true)
		}
	case domain.CommandConfirmPickup, domain.CommandConfirmDelivery, domain.CommandFailAction, domain.CommandDepartedPickup:
		if cmd.ActorKind != domain.ActorKindDriver {
			return domain.ExecutionCommandError(domain.ReasonActorDenied, true)
		}
	default:
		if cmd.ActorKind == domain.ActorKindSystem {
			return domain.ExecutionCommandError(domain.ReasonActorDenied, true)
		}
	}
	if cmd.ActorKind == domain.ActorKindDriver {
		if driverID == nil || *driverID == uuid.Nil {
			return domain.ExecutionCommandError(domain.ReasonUnassignedDriver, true)
		}
		if *driverID != cmd.ActorID {
			return domain.ExecutionCommandError(domain.ReasonWrongDriver, true)
		}
	}
	return nil
}

func loadReplay(ctx context.Context, tx pgx.Tx, operating uuid.UUID, key, digest string) (domain.ExecutionCommandResult, error) {
	var storedDigest string
	var raw []byte
	err := tx.QueryRow(ctx, `
		SELECT request_sha256, result_json
		FROM transport.transport_execution_commands
		WHERE operating_tenant_id = $1 AND idempotency_key = $2
	`, operating, key).Scan(&storedDigest, &raw)
	if err != nil {
		return domain.ExecutionCommandResult{}, mapDBError(err)
	}
	if storedDigest != digest {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonCommandBodyConflict, false)
	}
	var result domain.ExecutionCommandResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return domain.ExecutionCommandResult{}, apperrors.Internal("decode command result", err)
	}
	result.Replayed = true
	return result, nil
}

func applyCommand(ctx context.Context, tx pgx.Tx, scope *commandScope) (domain.ExecutionCommandResult, error) {
	switch scope.cmd.Name {
	case domain.CommandArriveStop:
		return arriveStop(ctx, tx, scope)
	case domain.CommandStartStopService:
		return startStopService(ctx, tx, scope)
	case domain.CommandCompleteStop:
		return completeStop(ctx, tx, scope)
	case domain.CommandSkipStop:
		return skipStop(ctx, tx, scope)
	case domain.CommandCancelRemainingStops:
		return cancelRemaining(ctx, tx, scope)
	case domain.CommandCancelInServiceStop:
		return cancelInService(ctx, tx, scope)
	case domain.CommandConfirmPickup:
		return confirmAction(ctx, tx, scope, domain.ActionTypePickup, "PICKUP_COMPLETED")
	case domain.CommandConfirmDelivery:
		return confirmAction(ctx, tx, scope, domain.ActionTypeDelivery, "DELIVERY_COMPLETED")
	case domain.CommandFailAction:
		return failAction(ctx, tx, scope)
	case domain.CommandDepartedPickup:
		return departedPickup(ctx, tx, scope)
	default:
		return domain.ExecutionCommandResult{}, apperrors.Validation("command name is invalid", map[string]any{"field": "name"})
	}
}

func arriveStop(ctx context.Context, tx pgx.Tx, scope *commandScope) (domain.ExecutionCommandResult, error) {
	stop, err := loadStop(ctx, tx, scope, scope.cmd.StopID)
	if err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if stop.Status == domain.StopStatusArrived || stop.Status == domain.StopStatusServiceStarted {
		return stopResult(stop, ""), nil
	}
	if stop.Status != domain.StopStatusPlanned {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonTerminalStop, false)
	}
	current, err := currentStopID(ctx, tx, scope.revisionID, scope.cmd.ExecutionID)
	if err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if scope.cmd.ActorKind == domain.ActorKindDriver && current != stop.ID {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonStopNotCurrent, false)
	}
	if current != stop.ID {
		if err := requireKnownReason(scope.cmd); err != nil {
			return domain.ExecutionCommandResult{}, err
		}
		if err := assertPlannedPredecessors(ctx, tx, scope, stop.SourceOrdinal); err != nil {
			return domain.ExecutionCommandResult{}, err
		}
	}
	if stop.Version != scope.cmd.ExpectedStopVersion {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonVersionConflict, false)
	}
	if current != stop.ID {
		skipped, err := skipPlannedPredecessors(ctx, tx, scope, stop.SourceOrdinal)
		if err != nil {
			return domain.ExecutionCommandResult{}, err
		}
		scope.skipped = skipped
		if err := emitExecutionEvent(ctx, tx, *scope, domain.EventRouteStopSequenceOverridden, stop.ID, uuid.Nil, scope.skipped); err != nil {
			return domain.ExecutionCommandResult{}, err
		}
	}
	occurred := scope.cmd.OccurredAt
	if err := updateStop(ctx, tx, stop, domain.StopStatusArrived, nil, &occurred, nil, nil); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if err := alignArrive(ctx, tx, scope, stop.ID); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if err := emitExecutionEvent(ctx, tx, *scope, domain.EventRouteStopArrived, stop.ID, uuid.Nil, nil); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	stop.Status = domain.StopStatusArrived
	return stopResult(stop, ""), nil
}

func startStopService(ctx context.Context, tx pgx.Tx, scope *commandScope) (domain.ExecutionCommandResult, error) {
	stop, err := loadStop(ctx, tx, scope, scope.cmd.StopID)
	if err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if stop.Status == domain.StopStatusServiceStarted {
		return stopResult(stop, ""), nil
	}
	if stop.Status != domain.StopStatusArrived {
		if isTerminalStop(stop.Status) {
			return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonTerminalStop, false)
		}
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonStopTransitionDenied, false)
	}
	if err := requireCurrentOrOverride(ctx, tx, scope, stop.ID); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if stop.Version != scope.cmd.ExpectedStopVersion {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonVersionConflict, false)
	}
	occurred := scope.cmd.OccurredAt
	if err := updateStop(ctx, tx, stop, domain.StopStatusServiceStarted, nil, nil, &occurred, nil); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if err := alignStartService(ctx, tx, scope, stop.ID); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if err := emitExecutionEvent(ctx, tx, *scope, domain.EventRouteStopServiceStarted, stop.ID, uuid.Nil, nil); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	stop.Status = domain.StopStatusServiceStarted
	return stopResult(stop, ""), nil
}

func completeStop(ctx context.Context, tx pgx.Tx, scope *commandScope) (domain.ExecutionCommandResult, error) {
	stop, err := loadStop(ctx, tx, scope, scope.cmd.StopID)
	if err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if stop.Status == domain.StopStatusCompleted {
		return stopResult(stop, ""), nil
	}
	if stop.Status != domain.StopStatusServiceStarted {
		if isTerminalStop(stop.Status) {
			return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonTerminalStop, false)
		}
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonStopTransitionDenied, false)
	}
	if err := requireCurrentOrOverride(ctx, tx, scope, stop.ID); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	var pending, failed int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status = 'PENDING'), count(*) FILTER (WHERE status = 'FAILED')
		FROM transport.transport_execution_actions
		WHERE execution_stop_id = $1
	`, stop.ID).Scan(&pending, &failed); err != nil {
		return domain.ExecutionCommandResult{}, mapDBError(err)
	}
	if pending > 0 {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonActionPending, false)
	}
	if stop.Version != scope.cmd.ExpectedStopVersion {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonVersionConflict, false)
	}
	var reason *string
	if failed > 0 {
		value := domain.StopStatusReasonPartial
		reason = &value
	}
	occurred := scope.cmd.OccurredAt
	if err := updateStop(ctx, tx, stop, domain.StopStatusCompleted, reason, nil, nil, &occurred); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if err := emitExecutionEvent(ctx, tx, *scope, domain.EventRouteStopCompleted, stop.ID, uuid.Nil, nil); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	stop.Status = domain.StopStatusCompleted
	return stopResult(stop, ""), nil
}

func skipStop(ctx context.Context, tx pgx.Tx, scope *commandScope) (domain.ExecutionCommandResult, error) {
	if err := requireKnownReason(scope.cmd); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	stop, err := loadStop(ctx, tx, scope, scope.cmd.StopID)
	if err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if stop.Status == domain.StopStatusSkipped {
		return stopResult(stop, ""), nil
	}
	if stop.Status != domain.StopStatusPlanned {
		if isTerminalStop(stop.Status) {
			return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonTerminalStop, false)
		}
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonStopTransitionDenied, false)
	}
	if stop.Version != scope.cmd.ExpectedStopVersion {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonVersionConflict, false)
	}
	reason := scope.cmd.ReasonCode
	if err := updateStop(ctx, tx, stop, domain.StopStatusSkipped, &reason, nil, nil, nil); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if err := cancelPendingActions(ctx, tx, stop.ID); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	scope.skipped = []int{stop.SourceOrdinal}
	if err := emitExecutionEvent(ctx, tx, *scope, domain.EventRouteStopSequenceOverridden, stop.ID, uuid.Nil, scope.skipped); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	stop.Status = domain.StopStatusSkipped
	return stopResult(stop, ""), nil
}

func cancelRemaining(ctx context.Context, tx pgx.Tx, scope *commandScope) (domain.ExecutionCommandResult, error) {
	if err := requireKnownReason(scope.cmd); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	currentOrdinal, err := currentSourceOrdinal(ctx, tx, scope)
	if err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	rows, err := tx.Query(ctx, `
		SELECT stop.id
		FROM transport.transport_execution_stops AS stop
		JOIN transport.transport_execution_revision_stops AS link
		  ON link.stop_id = stop.id AND link.revision_id = $2
		WHERE stop.execution_id = $1
		  AND stop.status = 'PLANNED'
		  AND link.source_ordinal > $3
		ORDER BY link.source_ordinal
	`, scope.cmd.ExecutionID, scope.revisionID, currentOrdinal)
	if err != nil {
		return domain.ExecutionCommandResult{}, mapDBError(err)
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return domain.ExecutionCommandResult{}, mapDBError(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return domain.ExecutionCommandResult{}, mapDBError(err)
	}
	reason := scope.cmd.ReasonCode
	for _, id := range ids {
		stop, err := loadStop(ctx, tx, scope, id)
		if err != nil {
			return domain.ExecutionCommandResult{}, err
		}
		if stop.Status != domain.StopStatusPlanned {
			continue
		}
		if err := updateStop(ctx, tx, stop, domain.StopStatusCancelled, &reason, nil, nil, nil); err != nil {
			return domain.ExecutionCommandResult{}, err
		}
		if err := cancelPendingActions(ctx, tx, id); err != nil {
			return domain.ExecutionCommandResult{}, err
		}
	}
	return domain.ExecutionCommandResult{}, nil
}

func cancelInService(ctx context.Context, tx pgx.Tx, scope *commandScope) (domain.ExecutionCommandResult, error) {
	if err := requireKnownReason(scope.cmd); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	stop, err := loadStop(ctx, tx, scope, scope.cmd.StopID)
	if err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if stop.Status == domain.StopStatusCancelled {
		return stopResult(stop, ""), nil
	}
	if stop.Status != domain.StopStatusArrived && stop.Status != domain.StopStatusServiceStarted {
		if isTerminalStop(stop.Status) {
			return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonTerminalStop, false)
		}
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonStopTransitionDenied, false)
	}
	if stop.Version != scope.cmd.ExpectedStopVersion {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonVersionConflict, false)
	}
	reason := scope.cmd.ReasonCode
	if err := updateStop(ctx, tx, stop, domain.StopStatusCancelled, &reason, nil, nil, nil); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if err := cancelPendingActions(ctx, tx, stop.ID); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	stop.Status = domain.StopStatusCancelled
	return stopResult(stop, ""), nil
}

func confirmAction(ctx context.Context, tx pgx.Tx, scope *commandScope, actionType, evidenceEvent string) (domain.ExecutionCommandResult, error) {
	action, err := loadAction(ctx, tx, scope.cmd.ActionID)
	if err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if action.StopID != scope.cmd.StopID {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonActionTransitionDenied, false)
	}
	if action.ActionType != actionType {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonActionTransitionDenied, false)
	}
	stop, err := loadStop(ctx, tx, scope, action.StopID)
	if err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if action.Status == domain.ActionStatusCompleted {
		status, err := shipmentStatusOf(ctx, tx, action.TenantID, action.ShipmentID)
		if err != nil {
			return domain.ExecutionCommandResult{}, err
		}
		return actionResult(stop, action, status), nil
	}
	if action.Status != domain.ActionStatusPending {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonTerminalAction, false)
	}
	if stop.Status != domain.StopStatusServiceStarted {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonActionTransitionDenied, false)
	}
	if err := requireDriverCurrent(ctx, tx, scope, stop.ID); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if stop.Version != scope.cmd.ExpectedStopVersion {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonVersionConflict, false)
	}
	shipment, err := loadShipment(ctx, tx, action.TenantID, action.ShipmentID)
	if err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	shipment.CargoID = &action.CargoID
	if actionType == domain.ActionTypePickup && shipment.Status == domain.ShipmentStatusInPickup {
		shipment, err = transitionShipment(ctx, tx, shipment, domain.ShipmentStatusLoaded, scope.cmd.OccurredAt, scope.cmd)
		if err != nil {
			return domain.ExecutionCommandResult{}, err
		}
	}
	if actionType == domain.ActionTypeDelivery {
		finalAction, err := isFinalRequiredDelivery(ctx, tx, scope, action)
		if err != nil {
			return domain.ExecutionCommandResult{}, err
		}
		pending, failed, err := otherRequiredDeliveries(ctx, tx, scope, action)
		if err != nil {
			return domain.ExecutionCommandResult{}, err
		}
		if finalAction && pending == 0 && failed == 0 && shipment.Status == domain.ShipmentStatusUnloading {
			shipment, err = transitionShipment(ctx, tx, shipment, domain.ShipmentStatusDelivered, scope.cmd.OccurredAt, scope.cmd)
			if err != nil {
				return domain.ExecutionCommandResult{}, err
			}
		}
	}
	driverID := uuid.Nil
	if scope.driverID != nil {
		driverID = *scope.driverID
	}
	intent := domain.CargoEvidenceIntentForDriverEvent(evidenceEvent, scope.cmd.ActorID, driverID, scope.cmd.OccurredAt)
	evidenceID, err := insertExecutionCargoEvidence(ctx, tx, shipment, intent)
	if err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE transport.transport_execution_actions
		SET status = 'COMPLETED', evidence_id = $2, completed_at = $3, version = version + 1
		WHERE id = $1 AND status = 'PENDING'
	`, action.ID, evidenceID, scope.cmd.OccurredAt)
	if err != nil {
		return domain.ExecutionCommandResult{}, mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonActionTransitionDenied, false)
	}
	action.Status = domain.ActionStatusCompleted
	action.EvidenceID = &evidenceID
	return actionResult(stop, action, shipment.Status), nil
}

func failAction(ctx context.Context, tx pgx.Tx, scope *commandScope) (domain.ExecutionCommandResult, error) {
	if err := requireKnownReason(scope.cmd); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	action, err := loadAction(ctx, tx, scope.cmd.ActionID)
	if err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if action.StopID != scope.cmd.StopID {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonActionTransitionDenied, false)
	}
	stop, err := loadStop(ctx, tx, scope, action.StopID)
	if err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if action.Status == domain.ActionStatusFailed {
		status, err := shipmentStatusOf(ctx, tx, action.TenantID, action.ShipmentID)
		if err != nil {
			return domain.ExecutionCommandResult{}, err
		}
		return actionResult(stop, action, status), nil
	}
	if action.Status != domain.ActionStatusPending {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonTerminalAction, false)
	}
	if stop.Status != domain.StopStatusServiceStarted {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonActionTransitionDenied, false)
	}
	if err := requireDriverCurrent(ctx, tx, scope, stop.ID); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if stop.Version != scope.cmd.ExpectedStopVersion {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonVersionConflict, false)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE transport.transport_execution_actions
		SET status = 'FAILED', version = version + 1
		WHERE id = $1 AND status = 'PENDING'
	`, action.ID); err != nil {
		return domain.ExecutionCommandResult{}, mapDBError(err)
	}
	shipment, err := loadShipment(ctx, tx, action.TenantID, action.ShipmentID)
	if err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	eventID := uuid.New()
	sourceID := uuid.New()
	driverID := uuid.Nil
	if scope.driverID != nil {
		driverID = *scope.driverID
	}
	outbox, err := domain.BuildDriverEventOutbox(domain.BuildDriverEventParams{
		EventID:         eventID,
		EventType:       domain.DriverEventTypeProblemReported,
		TenantID:        action.TenantID,
		ShipmentID:      action.ShipmentID,
		ShipmentVersion: shipment.Version,
		DriverID:        driverID,
		ActorID:         &scope.cmd.ActorID,
		SourceEventID:   sourceID,
		OccurredAt:      scope.cmd.OccurredAt,
		ReasonCode:      scope.cmd.ReasonCode,
		Metadata: map[string]any{
			"execution_stop_id": stop.ID.String(),
			"action_id":         action.ID.String(),
		},
	})
	if err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if err := executionOutboxInsert(ctx, tx, outbox); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	action.Status = domain.ActionStatusFailed
	return actionResult(stop, action, shipment.Status), nil
}

func departedPickup(ctx context.Context, tx pgx.Tx, scope *commandScope) (domain.ExecutionCommandResult, error) {
	var owner uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT execution_id
		FROM transport.transport_execution_active_shipments
		WHERE shipment_tenant_id = $1 AND shipment_id = $2
	`, scope.cmd.ShipmentTenantID, scope.cmd.ShipmentID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && owner != scope.cmd.ExecutionID) {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonNotParticipant, false)
	}
	if err != nil {
		return domain.ExecutionCommandResult{}, mapDBError(err)
	}
	shipment, err := loadShipment(ctx, tx, scope.cmd.ShipmentTenantID, scope.cmd.ShipmentID)
	if err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	if shipment.Status == domain.ShipmentStatusInTransit {
		return domain.ExecutionCommandResult{ShipmentStatus: shipment.Status}, nil
	}
	if shipment.Status != domain.ShipmentStatusLoaded {
		return domain.ExecutionCommandResult{}, domain.ExecutionCommandError(domain.ReasonStopTransitionDenied, false)
	}
	if err := requirePickupDepartureReady(ctx, tx, scope); err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	updated, err := transitionShipment(ctx, tx, shipment, domain.ShipmentStatusInTransit, scope.cmd.OccurredAt, scope.cmd)
	if err != nil {
		return domain.ExecutionCommandResult{}, err
	}
	return domain.ExecutionCommandResult{ShipmentStatus: updated.Status}, nil
}

func requirePickupDepartureReady(ctx context.Context, tx pgx.Tx, scope *commandScope) error {
	var actionStatus, stopStatus, evidenceState string
	var pickupOrdinal int
	err := tx.QueryRow(ctx, `
		SELECT action.status, stop.status, link.source_ordinal, COALESCE(evidence.state, '')
		FROM transport.transport_execution_actions AS action
		JOIN transport.transport_execution_revision_actions AS action_link
		  ON action_link.action_id = action.id
		 AND action_link.revision_id = $1
		 AND action_link.membership <> 'SUPERSEDED'
		JOIN transport.transport_execution_stops AS stop
		  ON stop.id = action.execution_stop_id
		JOIN transport.transport_execution_revision_stops AS link
		  ON link.stop_id = stop.id
		 AND link.revision_id = $1
		 AND link.membership <> 'SUPERSEDED'
		LEFT JOIN transport.shipment_cargo_execution_evidence AS evidence
		  ON evidence.id = action.evidence_id
		 AND evidence.shipment_id = action.shipment_id
		 AND evidence.tenant_id = action.shipment_tenant_id
		WHERE action.shipment_id = $2
		  AND action.shipment_tenant_id = $3
		  AND action.action_type = 'PICKUP'
		  AND action.status <> 'CANCELLED'
		ORDER BY link.source_ordinal, action.ordinal
		LIMIT 1
	`, scope.revisionID, scope.cmd.ShipmentID, scope.cmd.ShipmentTenantID).Scan(
		&actionStatus, &stopStatus, &pickupOrdinal, &evidenceState,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ExecutionCommandError(domain.ReasonStopTransitionDenied, false)
	}
	if err != nil {
		return mapDBError(err)
	}
	if actionStatus != domain.ActionStatusCompleted || stopStatus != domain.StopStatusCompleted || evidenceState != domain.CargoEvidenceConfirmedOnboard {
		return domain.ExecutionCommandError(domain.ReasonStopTransitionDenied, false)
	}
	var blocking int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM transport.transport_execution_stops AS stop
		JOIN transport.transport_execution_revision_stops AS link
		  ON link.stop_id = stop.id
		 AND link.revision_id = $1
		 AND link.membership <> 'SUPERSEDED'
		WHERE stop.execution_id = $2
		  AND stop.status IN ('PLANNED', 'ARRIVED', 'SERVICE_STARTED')
		  AND link.source_ordinal <= $3
	`, scope.revisionID, scope.cmd.ExecutionID, pickupOrdinal).Scan(&blocking)
	if err != nil {
		return mapDBError(err)
	}
	if blocking > 0 {
		return domain.ExecutionCommandError(domain.ReasonStopTransitionDenied, false)
	}
	return nil
}

func alignArrive(ctx context.Context, tx pgx.Tx, scope *commandScope, stopID uuid.UUID) error {
	actions, err := actionsOnStop(ctx, tx, stopID)
	if err != nil {
		return err
	}
	seen := map[string]struct{}{}
	for _, action := range actions {
		key := action.ActionType + ":" + action.TenantID.String() + ":" + action.ShipmentID.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		shipment, err := loadShipment(ctx, tx, action.TenantID, action.ShipmentID)
		if err != nil {
			return err
		}
		if action.ActionType == domain.ActionTypePickup {
			first, err := isFirstPickupStop(ctx, tx, scope, action.ShipmentID, action.TenantID, stopID)
			if err != nil {
				return err
			}
			if first && shipment.Status == domain.ShipmentStatusPickupSlotBooked {
				if _, err := transitionShipment(ctx, tx, shipment, domain.ShipmentStatusInPickup, scope.cmd.OccurredAt, scope.cmd); err != nil {
					return err
				}
			}
		}
		if action.ActionType == domain.ActionTypeDelivery {
			final, err := isFinalDeliveryStop(ctx, tx, scope, action.ShipmentID, action.TenantID, stopID)
			if err != nil {
				return err
			}
			if final && shipment.Status == domain.ShipmentStatusInTransit {
				if _, err := transitionShipment(ctx, tx, shipment, domain.ShipmentStatusArrivedAtConsignee, scope.cmd.OccurredAt, scope.cmd); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func alignStartService(ctx context.Context, tx pgx.Tx, scope *commandScope, stopID uuid.UUID) error {
	actions, err := actionsOnStop(ctx, tx, stopID)
	if err != nil {
		return err
	}
	seen := map[string]struct{}{}
	for _, action := range actions {
		if action.ActionType != domain.ActionTypeDelivery {
			continue
		}
		key := action.ActionType + ":" + action.TenantID.String() + ":" + action.ShipmentID.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		final, err := isFinalDeliveryStop(ctx, tx, scope, action.ShipmentID, action.TenantID, stopID)
		if err != nil {
			return err
		}
		if !final {
			continue
		}
		shipment, err := loadShipment(ctx, tx, action.TenantID, action.ShipmentID)
		if err != nil {
			return err
		}
		if shipment.Status == domain.ShipmentStatusArrivedAtConsignee {
			if _, err := transitionShipment(ctx, tx, shipment, domain.ShipmentStatusUnloading, scope.cmd.OccurredAt, scope.cmd); err != nil {
				return err
			}
		}
	}
	return nil
}

type stopRow struct {
	ID            uuid.UUID
	Status        string
	Version       int
	SourceOrdinal int
}

type actionRow struct {
	ID         uuid.UUID
	StopID     uuid.UUID
	ShipmentID uuid.UUID
	TenantID   uuid.UUID
	CargoID    uuid.UUID
	ActionType string
	Status     string
	EvidenceID *uuid.UUID
}

func loadStop(ctx context.Context, tx pgx.Tx, scope *commandScope, stopID uuid.UUID) (stopRow, error) {
	var stop stopRow
	err := tx.QueryRow(ctx, `
		SELECT stop.id, stop.status, stop.version, link.source_ordinal
		FROM transport.transport_execution_stops AS stop
		JOIN transport.transport_execution_revision_stops AS link
		  ON link.stop_id = stop.id AND link.revision_id = $2
		WHERE stop.id = $1 AND stop.execution_id = $3
		FOR UPDATE OF stop
	`, stopID, scope.revisionID, scope.cmd.ExecutionID).Scan(&stop.ID, &stop.Status, &stop.Version, &stop.SourceOrdinal)
	if errors.Is(err, pgx.ErrNoRows) {
		return stopRow{}, apperrors.NotFound("execution stop not found")
	}
	if err != nil {
		return stopRow{}, mapDBError(err)
	}
	return stop, nil
}

func loadAction(ctx context.Context, tx pgx.Tx, actionID uuid.UUID) (actionRow, error) {
	var action actionRow
	err := tx.QueryRow(ctx, `
		SELECT id, execution_stop_id, shipment_id, shipment_tenant_id, cargo_id, action_type, status, evidence_id
		FROM transport.transport_execution_actions
		WHERE id = $1
		FOR UPDATE
	`, actionID).Scan(
		&action.ID, &action.StopID, &action.ShipmentID, &action.TenantID, &action.CargoID,
		&action.ActionType, &action.Status, &action.EvidenceID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return actionRow{}, apperrors.NotFound("execution action not found")
	}
	if err != nil {
		return actionRow{}, mapDBError(err)
	}
	return action, nil
}

func actionsOnStop(ctx context.Context, tx pgx.Tx, stopID uuid.UUID) ([]actionRow, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, execution_stop_id, shipment_id, shipment_tenant_id, cargo_id, action_type, status, evidence_id
		FROM transport.transport_execution_actions
		WHERE execution_stop_id = $1
		ORDER BY ordinal
	`, stopID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	var out []actionRow
	for rows.Next() {
		var action actionRow
		if err := rows.Scan(
			&action.ID, &action.StopID, &action.ShipmentID, &action.TenantID, &action.CargoID,
			&action.ActionType, &action.Status, &action.EvidenceID,
		); err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, action)
	}
	return out, mapDBError(rows.Err())
}

func updateStop(ctx context.Context, tx pgx.Tx, stop stopRow, status string, reason *string, arrived, started, completed *time.Time) error {
	var version int
	err := tx.QueryRow(ctx, `
		UPDATE transport.transport_execution_stops
		SET status = $1,
			status_reason = CASE WHEN $2::text IS NULL THEN status_reason ELSE $2 END,
			arrived_at = COALESCE(arrived_at, $3),
			service_started_at = COALESCE(service_started_at, $4),
			completed_at = COALESCE(completed_at, $5),
			version = version + 1,
			updated_at = now()
		WHERE id = $6 AND status = $7 AND version = $8
		RETURNING version
	`, status, reason, optionalTime(arrived), optionalTime(started), optionalTime(completed), stop.ID, stop.Status, stop.Version).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ExecutionCommandError(domain.ReasonVersionConflict, false)
	}
	if err != nil {
		return mapDBError(err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE transport.driver_stop_tasks
		SET status = $2, version = $3, updated_at = now()
		WHERE execution_stop_id = $1
	`, stop.ID, status, version); err != nil {
		return mapDBError(err)
	}
	return nil
}

func cancelPendingActions(ctx context.Context, tx pgx.Tx, stopID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		UPDATE transport.transport_execution_actions
		SET status = 'CANCELLED', version = version + 1
		WHERE execution_stop_id = $1 AND status = 'PENDING'
	`, stopID)
	return mapDBError(err)
}

func currentStopID(ctx context.Context, tx pgx.Tx, revisionID, executionID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT stop.id
		FROM transport.transport_execution_stops AS stop
		JOIN transport.transport_execution_revision_stops AS link
		  ON link.stop_id = stop.id AND link.revision_id = $1
		WHERE stop.execution_id = $2
		  AND stop.status IN ('PLANNED', 'ARRIVED', 'SERVICE_STARTED')
		ORDER BY link.source_ordinal
		LIMIT 1
	`, revisionID, executionID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, nil
	}
	if err != nil {
		return uuid.Nil, mapDBError(err)
	}
	return id, nil
}

func currentSourceOrdinal(ctx context.Context, tx pgx.Tx, scope *commandScope) (int, error) {
	var ordinal int
	err := tx.QueryRow(ctx, `
		SELECT link.source_ordinal
		FROM transport.transport_execution_stops AS stop
		JOIN transport.transport_execution_revision_stops AS link
		  ON link.stop_id = stop.id AND link.revision_id = $1
		WHERE stop.execution_id = $2
		  AND stop.status IN ('PLANNED', 'ARRIVED', 'SERVICE_STARTED')
		ORDER BY link.source_ordinal
		LIMIT 1
	`, scope.revisionID, scope.cmd.ExecutionID).Scan(&ordinal)
	if errors.Is(err, pgx.ErrNoRows) {
		return -1, nil
	}
	if err != nil {
		return 0, mapDBError(err)
	}
	return ordinal, nil
}

func isFinalRequiredDelivery(ctx context.Context, tx pgx.Tx, scope *commandScope, action actionRow) (bool, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT action.id
		FROM transport.transport_execution_actions AS action
		JOIN transport.transport_execution_stops AS stop ON stop.id = action.execution_stop_id
		JOIN transport.transport_execution_revision_stops AS link
		  ON link.stop_id = stop.id AND link.revision_id = $1 AND link.membership <> 'SUPERSEDED'
		JOIN transport.transport_execution_revision_actions AS alink
		  ON alink.action_id = action.id AND alink.revision_id = $1 AND alink.membership <> 'SUPERSEDED'
		WHERE action.shipment_id = $2 AND action.shipment_tenant_id = $3
		  AND action.action_type = 'DELIVERY' AND action.status <> 'CANCELLED'
		ORDER BY link.source_ordinal DESC, alink.source_action_ordinal DESC
		LIMIT 1
	`, scope.revisionID, action.ShipmentID, action.TenantID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, mapDBError(err)
	}
	return id == action.ID, nil
}

func isFirstPickupStop(ctx context.Context, tx pgx.Tx, scope *commandScope, shipmentID, tenantID, stopID uuid.UUID) (bool, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT stop.id
		FROM transport.transport_execution_actions AS action
		JOIN transport.transport_execution_stops AS stop ON stop.id = action.execution_stop_id
		JOIN transport.transport_execution_revision_stops AS link
		  ON link.stop_id = stop.id AND link.revision_id = $1 AND link.membership <> 'SUPERSEDED'
		JOIN transport.transport_execution_revision_actions AS alink
		  ON alink.action_id = action.id AND alink.revision_id = $1 AND alink.membership <> 'SUPERSEDED'
		WHERE action.shipment_id = $2 AND action.shipment_tenant_id = $3
		  AND action.action_type = 'PICKUP' AND action.status <> 'CANCELLED'
		ORDER BY link.source_ordinal, alink.source_action_ordinal
		LIMIT 1
	`, scope.revisionID, shipmentID, tenantID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, mapDBError(err)
	}
	return id == stopID, nil
}

func isFinalDeliveryStop(ctx context.Context, tx pgx.Tx, scope *commandScope, shipmentID, tenantID, stopID uuid.UUID) (bool, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT stop.id
		FROM transport.transport_execution_actions AS action
		JOIN transport.transport_execution_stops AS stop ON stop.id = action.execution_stop_id
		JOIN transport.transport_execution_revision_stops AS link
		  ON link.stop_id = stop.id AND link.revision_id = $1 AND link.membership <> 'SUPERSEDED'
		JOIN transport.transport_execution_revision_actions AS alink
		  ON alink.action_id = action.id AND alink.revision_id = $1 AND alink.membership <> 'SUPERSEDED'
		WHERE action.shipment_id = $2 AND action.shipment_tenant_id = $3
		  AND action.action_type = 'DELIVERY' AND action.status <> 'CANCELLED'
		ORDER BY link.source_ordinal DESC, alink.source_action_ordinal DESC
		LIMIT 1
	`, scope.revisionID, shipmentID, tenantID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, mapDBError(err)
	}
	return id == stopID, nil
}

func otherRequiredDeliveries(ctx context.Context, tx pgx.Tx, scope *commandScope, action actionRow) (int, int, error) {
	var pending, failed int
	err := tx.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE action.status = 'PENDING'),
			count(*) FILTER (WHERE action.status = 'FAILED')
		FROM transport.transport_execution_actions AS action
		JOIN transport.transport_execution_revision_actions AS link
		  ON link.action_id = action.id AND link.revision_id = $1 AND link.membership <> 'SUPERSEDED'
		WHERE action.shipment_id = $2 AND action.shipment_tenant_id = $3
		  AND action.action_type = 'DELIVERY' AND action.status <> 'CANCELLED' AND action.id <> $4
	`, scope.revisionID, action.ShipmentID, action.TenantID, action.ID).Scan(&pending, &failed)
	return pending, failed, mapDBError(err)
}

func assertPlannedPredecessors(ctx context.Context, tx pgx.Tx, scope *commandScope, ordinal int) error {
	rows, err := openPredecessors(ctx, tx, scope, ordinal)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.Status != domain.StopStatusPlanned {
			return domain.ExecutionCommandError(domain.ReasonStopTransitionDenied, false)
		}
	}
	return nil
}

func skipPlannedPredecessors(ctx context.Context, tx pgx.Tx, scope *commandScope, ordinal int) ([]int, error) {
	rows, err := openPredecessors(ctx, tx, scope, ordinal)
	if err != nil {
		return nil, err
	}
	reason := scope.cmd.ReasonCode
	ordinals := make([]int, 0, len(rows))
	for _, row := range rows {
		if row.Status != domain.StopStatusPlanned {
			return nil, domain.ExecutionCommandError(domain.ReasonStopTransitionDenied, false)
		}
		if err := updateStop(ctx, tx, row, domain.StopStatusSkipped, &reason, nil, nil, nil); err != nil {
			return nil, err
		}
		if err := cancelPendingActions(ctx, tx, row.ID); err != nil {
			return nil, err
		}
		ordinals = append(ordinals, row.SourceOrdinal)
	}
	return ordinals, nil
}

func openPredecessors(ctx context.Context, tx pgx.Tx, scope *commandScope, ordinal int) ([]stopRow, error) {
	rows, err := tx.Query(ctx, `
		SELECT stop.id, stop.status, stop.version, link.source_ordinal
		FROM transport.transport_execution_stops AS stop
		JOIN transport.transport_execution_revision_stops AS link
		  ON link.stop_id = stop.id AND link.revision_id = $2 AND link.membership <> 'SUPERSEDED'
		WHERE stop.execution_id = $1
		  AND link.source_ordinal < $3
		  AND stop.status IN ('PLANNED', 'ARRIVED', 'SERVICE_STARTED')
		ORDER BY link.source_ordinal
		FOR UPDATE OF stop
	`, scope.cmd.ExecutionID, scope.revisionID, ordinal)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	var out []stopRow
	for rows.Next() {
		var row stopRow
		if err := rows.Scan(&row.ID, &row.Status, &row.Version, &row.SourceOrdinal); err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, row)
	}
	return out, mapDBError(rows.Err())
}

func loadShipment(ctx context.Context, tx pgx.Tx, tenantID, shipmentID uuid.UUID) (*domain.Shipment, error) {
	const query = `
		SELECT id, tenant_id, shipment_number, transport_order_id,
			shipper_company_id, consignee_company_id, carrier_company_id, forwarder_company_id,
			driver_id, vehicle_id, origin_location_id, destination_location_id, cargo_id,
			transport_mode, status, planned_pickup_at, planned_delivery_at,
			actual_pickup_at, actual_delivery_at, created_at, updated_at, version
		FROM transport.shipments
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		FOR UPDATE
	`
	return scanShipment(tx.QueryRow(ctx, query, shipmentID, tenantID))
}

func shipmentStatusOf(ctx context.Context, tx pgx.Tx, tenantID, shipmentID uuid.UUID) (string, error) {
	var status string
	err := tx.QueryRow(ctx, `
		SELECT status FROM transport.shipments WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`, shipmentID, tenantID).Scan(&status)
	return status, mapDBError(err)
}

func transitionShipment(ctx context.Context, tx pgx.Tx, shipment *domain.Shipment, to string, occurred time.Time, cmd domain.ExecutionCommand) (*domain.Shipment, error) {
	if shipment.Status == to {
		return shipment, nil
	}
	if err := domain.ValidateStatusTransition(shipment.Status, to); err != nil {
		return nil, err
	}
	var pickupAt, deliveryAt *time.Time
	if to == domain.ShipmentStatusLoaded {
		value := occurred.UTC()
		pickupAt = &value
	}
	if to == domain.ShipmentStatusDelivered {
		value := occurred.UTC()
		deliveryAt = &value
	}
	const query = `
		UPDATE transport.shipments
		SET status = $1,
			actual_pickup_at = COALESCE($2, actual_pickup_at),
			actual_delivery_at = COALESCE($3, actual_delivery_at),
			version = version + 1,
			updated_at = now()
		WHERE id = $4 AND tenant_id = $5 AND deleted_at IS NULL AND version = $6
		RETURNING id, tenant_id, shipment_number, transport_order_id,
			shipper_company_id, consignee_company_id, carrier_company_id, forwarder_company_id,
			driver_id, vehicle_id, origin_location_id, destination_location_id, cargo_id,
			transport_mode, status, planned_pickup_at, planned_delivery_at,
			actual_pickup_at, actual_delivery_at, created_at, updated_at, version
	`
	updated, err := scanShipmentUpdate(tx.QueryRow(ctx, query,
		to, optionalTime(pickupAt), optionalTime(deliveryAt), shipment.ID, shipment.TenantID, shipment.Version,
	))
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
			return nil, domain.ExecutionCommandError(domain.ReasonVersionConflict, false)
		}
		return nil, err
	}
	actorType := domain.ActorTypeUser
	var actorID *uuid.UUID
	if cmd.ActorKind == domain.ActorKindSystem {
		actorType = domain.ActorTypeSystem
	} else if cmd.ActorID != uuid.Nil {
		id := cmd.ActorID
		actorID = &id
	}
	reason := cmd.Name
	write := statusHistoryWriteFromShipmentTransition(updated, stringPtr(shipment.Status), to, domain.StatusTransitionContext{
		ActorType:  actorType,
		ActorID:    actorID,
		Source:     domain.StatusHistorySourceShipmentService,
		OccurredAt: occurred.UTC(),
		ReasonCode: &reason,
	})
	if err := insertStatusHistoryAndOutbox(ctx, tx, write); err != nil {
		return nil, err
	}
	return updated, nil
}

func emitExecutionEvent(ctx context.Context, tx pgx.Tx, scope commandScope, eventType string, stopID, actionID uuid.UUID, skipped []int) error {
	var seq int64
	if err := tx.QueryRow(ctx, `
		UPDATE transport.transport_executions
		SET event_seq = event_seq + 1, updated_at = now()
		WHERE id = $1
		RETURNING event_seq
	`, scope.cmd.ExecutionID).Scan(&seq); err != nil {
		return mapDBError(err)
	}
	payload, err := json.Marshal(map[string]any{
		"event_id":         uuid.NewString(),
		"event_type":       eventType,
		"execution_id":     scope.cmd.ExecutionID.String(),
		"revision_id":      scope.revisionID.String(),
		"revision_version": scope.revisionVersion,
		"event_sequence":   seq,
		"stop_id":          uuidString(stopID),
		"action_id":        uuidString(actionID),
		"occurred_at":      scope.cmd.OccurredAt.UTC().Format(time.RFC3339Nano),
		"skipped_ordinals": skipped,
	})
	if err != nil {
		return apperrors.Internal("marshal execution event", err)
	}
	headers, err := json.Marshal(map[string]string{
		"contentType": "application/json",
		"eventType":   eventType,
	})
	if err != nil {
		return apperrors.Internal("marshal execution event headers", err)
	}
	eventID := uuid.New()
	return executionOutboxInsert(ctx, tx, domain.ShipmentOutboxEvent{
		ID:               eventID,
		TenantID:         scope.operatingTenant,
		AggregateType:    domain.OutboxAggregateTypeTransportExecution,
		AggregateID:      scope.cmd.ExecutionID,
		AggregateVersion: scope.revisionVersion,
		EventType:        eventType,
		SchemaVersion:    1,
		SourceEventID:    eventID,
		Payload:          payload,
		Headers:          headers,
		Status:           domain.OutboxStatusPending,
		AvailableAt:      time.Now().UTC(),
	})
}

func insertCommandAudit(ctx context.Context, tx pgx.Tx, scope commandScope, result domain.ExecutionCommandResult) error {
	ordinals := make([]int32, len(scope.skipped))
	for i, value := range scope.skipped {
		ordinals[i] = int32(value)
	}
	var actorID *uuid.UUID
	if scope.cmd.ActorID != uuid.Nil {
		id := scope.cmd.ActorID
		actorID = &id
	}
	var reason *string
	if scope.cmd.ReasonCode != "" {
		value := scope.cmd.ReasonCode
		reason = &value
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO transport.transport_execution_command_audit (
			id, command_id, operating_tenant_id, execution_id, revision_id, stop_id, action_id,
			actor_kind, actor_id, reason_code, skipped_ordinals, occurred_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`, uuid.New(), scope.commandID, scope.operatingTenant, scope.cmd.ExecutionID, scope.revisionID,
		result.StopID, result.ActionID, scope.cmd.ActorKind, actorID, reason, ordinals, scope.cmd.OccurredAt)
	return mapDBError(err)
}

func requireKnownReason(cmd domain.ExecutionCommand) error {
	if strings.TrimSpace(cmd.ReasonCode) == "" {
		return domain.ExecutionCommandError(domain.ReasonReasonRequired, false)
	}
	if !domain.KnownExecutionReason(cmd.ReasonCode) {
		return domain.ExecutionCommandError(domain.ReasonReasonUnknown, false)
	}
	return nil
}

func requireDriverCurrent(ctx context.Context, tx pgx.Tx, scope *commandScope, stopID uuid.UUID) error {
	current, err := currentStopID(ctx, tx, scope.revisionID, scope.cmd.ExecutionID)
	if err != nil {
		return err
	}
	if current != stopID {
		return domain.ExecutionCommandError(domain.ReasonStopNotCurrent, false)
	}
	return nil
}

func requireCurrentOrOverride(ctx context.Context, tx pgx.Tx, scope *commandScope, stopID uuid.UUID) error {
	current, err := currentStopID(ctx, tx, scope.revisionID, scope.cmd.ExecutionID)
	if err != nil {
		return err
	}
	if scope.cmd.ActorKind == domain.ActorKindDriver && current != stopID {
		return domain.ExecutionCommandError(domain.ReasonStopNotCurrent, false)
	}
	if current != stopID {
		return requireKnownReason(scope.cmd)
	}
	return nil
}

func isTerminalStop(status string) bool {
	switch status {
	case domain.StopStatusCompleted, domain.StopStatusSkipped, domain.StopStatusCancelled:
		return true
	default:
		return false
	}
}

func stopResult(stop stopRow, shipmentStatus string) domain.ExecutionCommandResult {
	id := stop.ID
	return domain.ExecutionCommandResult{StopID: &id, StopStatus: stop.Status, ShipmentStatus: shipmentStatus}
}

func actionResult(stop stopRow, action actionRow, shipmentStatus string) domain.ExecutionCommandResult {
	stopID := stop.ID
	actionID := action.ID
	return domain.ExecutionCommandResult{
		StopID:         &stopID,
		ActionID:       &actionID,
		StopStatus:     stop.Status,
		ActionStatus:   action.Status,
		EvidenceID:     action.EvidenceID,
		ShipmentStatus: shipmentStatus,
	}
}

func uuidString(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}

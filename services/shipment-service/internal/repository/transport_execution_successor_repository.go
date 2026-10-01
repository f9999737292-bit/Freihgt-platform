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

type successorStopLink struct {
	StopID        uuid.UUID
	Status        string
	SourceStopID  uuid.UUID
	SourceOrdinal int
	ArrivedAt     *time.Time
	ServiceStart  *time.Time
	CompletedAt   *time.Time
	Ordinal       int
	UpdatedAt     time.Time
}

type successorActionLink struct {
	ActionID       uuid.UUID
	StopID         uuid.UUID
	Status         string
	SourceActionID uuid.UUID
	SourceOrdinal  int
	CompletedAt    *time.Time
}

type successorParticipant struct {
	ShipmentTenantID uuid.UUID
	CargoVersion     int
	SubjectType      string
	SubjectID        uuid.UUID
}

// CreateSuccessorRevision appends one ACTIVE revision and supersedes the previous remaining route.
// Terminal stop and action rows stay on the historical revision only. The successor revision
// contains the newly introduced future route.
func (r *TransportExecutionRepository) CreateSuccessorRevision(ctx context.Context, cmd domain.SuccessorRevisionCommand) (domain.SuccessorRevisionResult, error) {
	if r == nil || r.pool == nil {
		return domain.SuccessorRevisionResult{}, apperrors.NotFound("transport execution not found")
	}
	if err := cmd.Validate(); err != nil {
		return domain.SuccessorRevisionResult{}, err
	}
	cmd.IdempotencyKey = strings.TrimSpace(cmd.IdempotencyKey)
	cmd.ReasonCode = strings.ToUpper(strings.TrimSpace(cmd.ReasonCode))
	cmd.OccurredAt = cmd.OccurredAt.UTC()
	cmd.EvaluationFingerprint = strings.TrimSpace(cmd.EvaluationFingerprint)
	digest, err := domain.SuccessorDigest(cmd)
	if err != nil {
		return domain.SuccessorRevisionResult{}, apperrors.Internal("successor digest failed", err)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.SuccessorRevisionResult{}, mapDBError(err)
	}
	defer tx.Rollback(ctx)
	result, err := createSuccessorRevisionInTx(ctx, tx, cmd, digest)
	if err != nil {
		return domain.SuccessorRevisionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.SuccessorRevisionResult{}, mapDBError(err)
	}
	return result, nil
}

// createSuccessorRevisionInTx is the 0.1F route writer. Callers that already hold
// the execution row lock must pass that same transaction so a failed disposition
// cannot leave a successor revision committed.
func createSuccessorRevisionInTx(ctx context.Context, tx pgx.Tx, cmd domain.SuccessorRevisionCommand, digest string) (domain.SuccessorRevisionResult, error) {
	var operating uuid.UUID
	var currentRevision uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT operating_tenant_id, current_revision_id
		FROM transport.transport_executions
		WHERE id = $1
		FOR UPDATE
	`, cmd.ExecutionID).Scan(&operating, &currentRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SuccessorRevisionResult{}, apperrors.NotFound("transport execution not found")
	}
	if err != nil {
		return domain.SuccessorRevisionResult{}, mapDBError(err)
	}
	if cmd.OperatingTenantID != operating {
		return domain.SuccessorRevisionResult{}, domain.ExecutionCommandError(domain.ReasonTenantDenied, true)
	}

	commandID := uuid.New()
	tag, err := tx.Exec(ctx, `
		INSERT INTO transport.transport_execution_commands (
			id, operating_tenant_id, execution_id, idempotency_key, command_name, request_sha256, result_json
		) VALUES ($1,$2,$3,$4,$5,$6,'{}'::jsonb)
		ON CONFLICT (operating_tenant_id, idempotency_key) DO NOTHING
	`, commandID, operating, cmd.ExecutionID, cmd.IdempotencyKey, domain.CommandCreateSuccessorRevision, digest)
	if err != nil {
		return domain.SuccessorRevisionResult{}, mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return loadSuccessorReplay(ctx, tx, operating, cmd.IdempotencyKey, digest)
	}

	if currentRevision != cmd.ExpectedRevisionID {
		return domain.SuccessorRevisionResult{}, domain.ExecutionCommandError(domain.ReasonRevisionConflict, false)
	}
	var revisionStatus string
	err = tx.QueryRow(ctx, `
		SELECT status
		FROM transport.transport_execution_revisions
		WHERE id = $1 AND execution_id = $2 AND operating_tenant_id = $3
	`, currentRevision, cmd.ExecutionID, operating).Scan(&revisionStatus)
	if errors.Is(err, pgx.ErrNoRows) || revisionStatus != domain.RevisionStatusActive {
		return domain.SuccessorRevisionResult{}, domain.ExecutionCommandError(domain.ReasonRevisionConflict, false)
	}
	if err != nil {
		return domain.SuccessorRevisionResult{}, mapDBError(err)
	}

	stops, err := loadSuccessorStops(ctx, tx, currentRevision)
	if err != nil {
		return domain.SuccessorRevisionResult{}, err
	}
	actions, err := loadSuccessorActions(ctx, tx, currentRevision)
	if err != nil {
		return domain.SuccessorRevisionResult{}, err
	}
	for _, stop := range stops {
		if stop.Status == domain.StopStatusServiceStarted {
			return domain.SuccessorRevisionResult{}, domain.ExecutionCommandError(domain.ReasonExecutionStopInService, false)
		}
	}
	participants, err := loadSuccessorParticipants(ctx, tx, cmd)
	if err != nil {
		return domain.SuccessorRevisionResult{}, err
	}

	now := time.Now().UTC()
	nextRevision := uuid.New()
	if _, err := tx.Exec(ctx, `
		UPDATE transport.transport_execution_revisions
		SET status = $2, updated_at = $3
		WHERE id = $1 AND status = 'ACTIVE'
	`, currentRevision, domain.RevisionStatusSuperseded, now); err != nil {
		return domain.SuccessorRevisionResult{}, mapDBError(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO transport.transport_execution_revisions (
			id, execution_id, operating_tenant_id, source_route_plan_id, source_route_plan_version,
			source_activation_id, source_activation_version, evaluation_fingerprint, planning_mode,
			supersedes_revision_id, status, version, contract_sha256, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,1,$12,$13,$13)
	`, nextRevision, cmd.ExecutionID, operating, cmd.SourceRoutePlanID, cmd.SourceRoutePlanVersion,
		cmd.SourceActivationID, cmd.SourceActivationVersion, cmd.EvaluationFingerprint, cmd.PlanningMode,
		currentRevision, domain.RevisionStatusActive, digest, now); err != nil {
		return domain.SuccessorRevisionResult{}, mapDBError(err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE transport.transport_executions
		SET current_revision_id = $2, updated_at = $3
		WHERE id = $1
	`, cmd.ExecutionID, nextRevision, now); err != nil {
		return domain.SuccessorRevisionResult{}, mapDBError(err)
	}

	for _, stop := range stops {
		if terminalStopStatus(stop.Status) {
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE transport.transport_execution_revision_stops
			SET membership = $3
			WHERE revision_id = $1 AND stop_id = $2
		`, currentRevision, stop.StopID, domain.MembershipSuperseded); err != nil {
			return domain.SuccessorRevisionResult{}, mapDBError(err)
		}
	}
	for _, action := range actions {
		if terminalActionStatus(action.Status) {
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE transport.transport_execution_revision_actions
			SET membership = $3
			WHERE revision_id = $1 AND action_id = $2
		`, currentRevision, action.ActionID, domain.MembershipSuperseded); err != nil {
			return domain.SuccessorRevisionResult{}, mapDBError(err)
		}
	}

	var maxOrdinal int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(ordinal), -1)
		FROM transport.transport_execution_stops
		WHERE execution_id = $1
	`, cmd.ExecutionID).Scan(&maxOrdinal); err != nil {
		return domain.SuccessorRevisionResult{}, mapDBError(err)
	}
	stopIDs := make([]uuid.UUID, len(cmd.Stops))
	for i, stop := range cmd.Stops {
		stopID := uuid.New()
		stopIDs[i] = stopID
		ordinal := maxOrdinal + 1 + i
		if _, err := tx.Exec(ctx, `
			INSERT INTO transport.transport_execution_stops (
				id, execution_id, operating_tenant_id, ordinal, stop_role, point_kind, location_id,
				latitude, longitude, planned_arrival, planned_departure, service_duration_seconds,
				status, status_reason, version, arrived_at, service_started_at, completed_at,
				created_at, updated_at
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NULL,1,NULL,NULL,NULL,$14,$14
			)
		`, stopID, cmd.ExecutionID, operating, ordinal, stop.StopRole, stop.PointKind, stop.LocationID,
			stop.Latitude, stop.Longitude, stop.PlannedArrival, stop.PlannedDeparture, stop.ServiceDurationSeconds,
			domain.StopStatusPlanned, now); err != nil {
			return domain.SuccessorRevisionResult{}, mapDBError(err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO transport.transport_execution_revision_stops (
				revision_id, stop_id, source_route_plan_stop_id, source_ordinal, membership
			) VALUES ($1,$2,$3,$4,$5)
		`, nextRevision, stopID, uuid.New(), i, domain.MembershipIntroduced); err != nil {
			return domain.SuccessorRevisionResult{}, mapDBError(err)
		}
	}
	actionOrdinal := map[uuid.UUID]int{}
	for _, action := range cmd.Actions {
		participant := participants[action.ShipmentID.String()+":"+action.CargoID.String()]
		stopID := stopIDs[action.StopIndex]
		ordinal := actionOrdinal[stopID]
		actionOrdinal[stopID] = ordinal + 1
		actionID := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO transport.transport_execution_actions (
				id, execution_stop_id, shipment_id, shipment_tenant_id, cargo_id, cargo_version,
				route_subject_type, route_subject_id, action_type, ordinal, status,
				evidence_id, completed_at, version
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NULL,NULL,1)
		`, actionID, stopID, action.ShipmentID, participant.ShipmentTenantID, action.CargoID, participant.CargoVersion,
			participant.SubjectType, participant.SubjectID, action.ActionType, ordinal, domain.ActionStatusPending); err != nil {
			return domain.SuccessorRevisionResult{}, mapDBError(err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO transport.transport_execution_revision_actions (
				revision_id, action_id, source_route_plan_action_id, source_action_ordinal, membership
			) VALUES ($1,$2,$3,$4,$5)
		`, nextRevision, actionID, uuid.New(), ordinal, domain.MembershipIntroduced); err != nil {
			return domain.SuccessorRevisionResult{}, mapDBError(err)
		}
	}

	var currentStop uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT s.id
		FROM transport.transport_execution_stops s
		JOIN transport.transport_execution_revision_stops rs ON rs.stop_id = s.id
		WHERE rs.revision_id = $1
		  AND s.execution_id = $2
		  AND rs.membership = 'INTRODUCED'
		  AND s.status = 'PLANNED'
		ORDER BY rs.source_ordinal
		LIMIT 1
	`, nextRevision, cmd.ExecutionID).Scan(&currentStop)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SuccessorRevisionResult{}, domain.ExecutionCommandError(domain.ReasonNoRemainingRoute, false)
	}
	if err != nil {
		return domain.SuccessorRevisionResult{}, mapDBError(err)
	}

	if err := emitExecutionPlanSuperseded(ctx, tx, operating, cmd.ExecutionID, currentRevision, nextRevision, cmd.ReasonCode, cmd.OccurredAt); err != nil {
		return domain.SuccessorRevisionResult{}, err
	}
	if err := emitExecutionPlanCreated(ctx, tx, operating, cmd.ExecutionID, nextRevision, cmd.OccurredAt, domain.MembershipIntroduced); err != nil {
		return domain.SuccessorRevisionResult{}, err
	}
	if err := emitInitialCurrentStop(ctx, tx, operating, cmd.ExecutionID, nextRevision, cmd.OccurredAt); err != nil {
		return domain.SuccessorRevisionResult{}, err
	}
	if err := refreshSuccessorDriverTasks(ctx, tx, cmd.ExecutionID, currentRevision, nextRevision, now); err != nil {
		return domain.SuccessorRevisionResult{}, err
	}

	result := domain.SuccessorRevisionResult{
		CommandID:          commandID,
		ExecutionID:        cmd.ExecutionID,
		PreviousRevisionID: currentRevision,
		RevisionID:         nextRevision,
		CurrentStopID:      currentStop,
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return domain.SuccessorRevisionResult{}, apperrors.Internal("marshal successor result", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE transport.transport_execution_commands SET result_json = $2 WHERE id = $1
	`, commandID, raw); err != nil {
		return domain.SuccessorRevisionResult{}, mapDBError(err)
	}
	stopID := currentStop
	if err := insertCommandAudit(ctx, tx, commandScope{
		cmd: domain.ExecutionCommand{
			ExecutionID: cmd.ExecutionID, ActorKind: cmd.ActorKind, ActorID: cmd.ActorID,
			ReasonCode: cmd.ReasonCode, OccurredAt: cmd.OccurredAt,
		},
		commandID: commandID, operatingTenant: operating, revisionID: nextRevision,
	}, domain.ExecutionCommandResult{StopID: &stopID}); err != nil {
		return domain.SuccessorRevisionResult{}, err
	}
	return result, nil
}

func loadSuccessorReplay(ctx context.Context, tx pgx.Tx, operating uuid.UUID, key, digest string) (domain.SuccessorRevisionResult, error) {
	var storedDigest string
	var raw []byte
	err := tx.QueryRow(ctx, `
		SELECT request_sha256, result_json
		FROM transport.transport_execution_commands
		WHERE operating_tenant_id = $1 AND idempotency_key = $2
	`, operating, key).Scan(&storedDigest, &raw)
	if err != nil {
		return domain.SuccessorRevisionResult{}, mapDBError(err)
	}
	if storedDigest != digest {
		return domain.SuccessorRevisionResult{}, domain.ExecutionCommandError(domain.ReasonCommandBodyConflict, false)
	}
	var result domain.SuccessorRevisionResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return domain.SuccessorRevisionResult{}, apperrors.Internal("decode successor result", err)
	}
	result.Replayed = true
	return result, nil
}

func loadSuccessorStops(ctx context.Context, tx pgx.Tx, revisionID uuid.UUID) ([]successorStopLink, error) {
	rows, err := tx.Query(ctx, `
		SELECT s.id, s.status, rs.source_route_plan_stop_id, rs.source_ordinal,
		       s.arrived_at, s.service_started_at, s.completed_at, s.ordinal, s.updated_at
		FROM transport.transport_execution_revision_stops rs
		JOIN transport.transport_execution_stops s ON s.id = rs.stop_id
		WHERE rs.revision_id = $1 AND rs.membership <> 'SUPERSEDED'
	`, revisionID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	var out []successorStopLink
	for rows.Next() {
		var row successorStopLink
		if err := rows.Scan(&row.StopID, &row.Status, &row.SourceStopID, &row.SourceOrdinal, &row.ArrivedAt, &row.ServiceStart, &row.CompletedAt, &row.Ordinal, &row.UpdatedAt); err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, row)
	}
	return out, mapDBError(rows.Err())
}

func loadSuccessorActions(ctx context.Context, tx pgx.Tx, revisionID uuid.UUID) ([]successorActionLink, error) {
	rows, err := tx.Query(ctx, `
		SELECT a.id, a.execution_stop_id, a.status, ra.source_route_plan_action_id, ra.source_action_ordinal, a.completed_at
		FROM transport.transport_execution_revision_actions ra
		JOIN transport.transport_execution_actions a ON a.id = ra.action_id
		WHERE ra.revision_id = $1 AND ra.membership <> 'SUPERSEDED'
	`, revisionID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	var out []successorActionLink
	for rows.Next() {
		var row successorActionLink
		if err := rows.Scan(&row.ActionID, &row.StopID, &row.Status, &row.SourceActionID, &row.SourceOrdinal, &row.CompletedAt); err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, row)
	}
	return out, mapDBError(rows.Err())
}

func loadSuccessorParticipants(ctx context.Context, tx pgx.Tx, cmd domain.SuccessorRevisionCommand) (map[string]successorParticipant, error) {
	out := map[string]successorParticipant{}
	for _, action := range cmd.Actions {
		key := action.ShipmentID.String() + ":" + action.CargoID.String()
		if _, ok := out[key]; ok {
			continue
		}
		rows, err := tx.Query(ctx, `
			SELECT shipment_tenant_id, cargo_version, route_subject_type, route_subject_id
			FROM transport.transport_execution_participants
			WHERE execution_id = $1 AND shipment_id = $2 AND cargo_id = $3
		`, cmd.ExecutionID, action.ShipmentID, action.CargoID)
		if err != nil {
			return nil, mapDBError(err)
		}
		var found []successorParticipant
		for rows.Next() {
			var row successorParticipant
			if err := rows.Scan(&row.ShipmentTenantID, &row.CargoVersion, &row.SubjectType, &row.SubjectID); err != nil {
				rows.Close()
				return nil, mapDBError(err)
			}
			found = append(found, row)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, mapDBError(err)
		}
		rows.Close()
		if len(found) != 1 {
			return nil, domain.ExecutionCommandError(domain.ReasonNotParticipant, false)
		}
		out[key] = found[0]
	}
	return out, nil
}

func terminalStopStatus(status string) bool {
	switch status {
	case domain.StopStatusCompleted, domain.StopStatusSkipped, domain.StopStatusCancelled:
		return true
	default:
		return false
	}
}

func terminalActionStatus(status string) bool {
	switch status {
	case domain.ActionStatusCompleted, domain.ActionStatusFailed, domain.ActionStatusCancelled:
		return true
	default:
		return false
	}
}

func emitExecutionPlanSuperseded(ctx context.Context, tx pgx.Tx, operatingTenant, executionID, oldRevision, newRevision uuid.UUID, reason string, occurred time.Time) error {
	var version int
	if err := tx.QueryRow(ctx, `
		SELECT version FROM transport.transport_execution_revisions WHERE id = $1
	`, oldRevision).Scan(&version); err != nil {
		return mapDBError(err)
	}
	var seq int64
	if err := tx.QueryRow(ctx, `
		UPDATE transport.transport_executions
		SET event_seq = event_seq + 1, updated_at = now()
		WHERE id = $1
		RETURNING event_seq
	`, executionID).Scan(&seq); err != nil {
		return mapDBError(err)
	}
	payload, err := json.Marshal(map[string]any{
		"event_id":            uuid.NewString(),
		"event_type":          domain.EventExecutionPlanSuperseded,
		"operating_tenant_id": operatingTenant.String(),
		"execution_id":        executionID.String(),
		"revision_id":         oldRevision.String(),
		"old_revision_id":     oldRevision.String(),
		"new_revision_id":     newRevision.String(),
		"revision_version":    version,
		"event_sequence":      seq,
		"occurred_at":         occurred.UTC().Format(time.RFC3339Nano),
		"reason_code":         reason,
	})
	if err != nil {
		return apperrors.Internal("marshal execution plan superseded", err)
	}
	headers, err := json.Marshal(map[string]string{
		"contentType": "application/json",
		"eventType":   domain.EventExecutionPlanSuperseded,
	})
	if err != nil {
		return apperrors.Internal("marshal execution plan superseded headers", err)
	}
	eventID := uuid.New()
	return executionOutboxInsert(ctx, tx, domain.ShipmentOutboxEvent{
		ID: eventID, TenantID: operatingTenant, AggregateType: domain.OutboxAggregateTypeTransportExecution,
		AggregateID: executionID, AggregateVersion: version, EventType: domain.EventExecutionPlanSuperseded,
		SchemaVersion: 1, SourceEventID: eventID, Payload: payload, Headers: headers,
		Status: domain.OutboxStatusPending, AvailableAt: time.Now().UTC(),
	})
}

func refreshSuccessorDriverTasks(ctx context.Context, tx pgx.Tx, executionID, oldRevision, newRevision uuid.UUID, now time.Time) error {
	if _, err := tx.Exec(ctx, `
		UPDATE transport.driver_stop_tasks AS task
		SET status = 'CANCELLED', updated_at = $2
		WHERE task.execution_id = $1
		  AND task.status IN ('PLANNED', 'ARRIVED')
		  AND EXISTS (
		      SELECT 1
		      FROM transport.transport_execution_revision_stops AS link
		      WHERE link.stop_id = task.execution_stop_id
		        AND link.revision_id = $3
		        AND link.membership = 'SUPERSEDED'
		  )
	`, executionID, now, oldRevision); err != nil {
		return mapDBError(err)
	}

	var tenant uuid.UUID
	var driverID, vehicleID *uuid.UUID
	if err := tx.QueryRow(ctx, `
		SELECT operating_tenant_id, driver_id, vehicle_id
		FROM transport.transport_executions
		WHERE id = $1
	`, executionID).Scan(&tenant, &driverID, &vehicleID); err != nil {
		return mapDBError(err)
	}
	rows, err := tx.Query(ctx, `
		SELECT s.id, s.ordinal, s.stop_role, s.point_kind, s.location_id, s.planned_arrival, s.status, s.version
		FROM transport.transport_execution_revision_stops rs
		JOIN transport.transport_execution_stops s ON s.id = rs.stop_id
		WHERE rs.revision_id = $1 AND rs.membership = 'INTRODUCED'
		ORDER BY rs.source_ordinal
	`, newRevision)
	if err != nil {
		return mapDBError(err)
	}
	defer rows.Close()
	var stops []stopTaskSource
	for rows.Next() {
		var stop stopTaskSource
		if err := rows.Scan(&stop.ID, &stop.Ordinal, &stop.Role, &stop.PointKind, &stop.LocationID, &stop.PlannedArrival, &stop.Status, &stop.Version); err != nil {
			return mapDBError(err)
		}
		stops = append(stops, stop)
	}
	if err := rows.Err(); err != nil {
		return mapDBError(err)
	}
	var lastCargo *uuid.UUID
	lastOrdinal := -1
	for _, stop := range stops {
		if stop.Role == domain.StopRoleCargo && stop.Ordinal >= lastOrdinal {
			lastOrdinal = stop.Ordinal
			lastCargo = stop.LocationID
		}
	}
	for _, stop := range stops {
		if !driverVisibleStop(stop, lastCargo) || stop.LocationID == nil {
			continue
		}
		actionRows, err := tx.Query(ctx, `
			SELECT a.id, a.execution_stop_id, a.shipment_id, a.cargo_id, a.action_type, a.ordinal
			FROM transport.transport_execution_revision_actions ra
			JOIN transport.transport_execution_actions a ON a.id = ra.action_id
			WHERE ra.revision_id = $1 AND ra.membership = 'INTRODUCED' AND a.execution_stop_id = $2
			ORDER BY a.ordinal
		`, newRevision, stop.ID)
		if err != nil {
			return mapDBError(err)
		}
		var actions []actionTaskSource
		for actionRows.Next() {
			var action actionTaskSource
			if err := actionRows.Scan(&action.ID, &action.StopID, &action.ShipmentID, &action.CargoID, &action.ActionType, &action.Ordinal); err != nil {
				actionRows.Close()
				return mapDBError(err)
			}
			actions = append(actions, action)
		}
		if err := actionRows.Err(); err != nil {
			actionRows.Close()
			return mapDBError(err)
		}
		actionRows.Close()
		summary, shipmentID := summarizeStopActions(actions)
		raw, err := json.Marshal(summary)
		if err != nil {
			return apperrors.Internal("marshal successor driver stop summary", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO transport.driver_stop_tasks (
				id, operating_tenant_id, execution_id, execution_stop_id, driver_id, vehicle_id,
				shipment_id, ordinal, location_id, planned_arrival, action_summary, status, version,
				created_at, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$14)
			ON CONFLICT (execution_stop_id) DO NOTHING
		`, uuid.New(), tenant, executionID, stop.ID, driverID, vehicleID, shipmentID, stop.Ordinal,
			*stop.LocationID, stop.PlannedArrival, raw, stop.Status, stop.Version, now); err != nil {
			return mapDBError(err)
		}
	}
	return nil
}

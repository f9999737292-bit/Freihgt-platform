package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

var materializeDriverStopTasksFn = materializeDriverStopTasks

func SetDriverStopTaskMaterializeForTest(fn func(context.Context, pgx.Tx, uuid.UUID, time.Time) error) func() {
	previous := materializeDriverStopTasksFn
	if fn == nil {
		materializeDriverStopTasksFn = materializeDriverStopTasks
	} else {
		materializeDriverStopTasksFn = fn
	}
	return func() { materializeDriverStopTasksFn = previous }
}

type stopTaskSource struct {
	ID             uuid.UUID
	Ordinal        int
	Role           string
	PointKind      string
	LocationID     *uuid.UUID
	PlannedArrival *time.Time
	Status         string
	Version        int
}

type actionTaskSource struct {
	ID         uuid.UUID
	StopID     uuid.UUID
	ShipmentID uuid.UUID
	CargoID    uuid.UUID
	ActionType string
	Ordinal    int
}

func materializeDriverStopTasks(ctx context.Context, tx pgx.Tx, executionID uuid.UUID, now time.Time) error {
	var tenant uuid.UUID
	var driverID, vehicleID *uuid.UUID
	if err := tx.QueryRow(ctx, `
		SELECT operating_tenant_id, driver_id, vehicle_id
		FROM transport.transport_executions
		WHERE id = $1
	`, executionID).Scan(&tenant, &driverID, &vehicleID); err != nil {
		return mapDBError(err)
	}
	stops, err := loadStopTaskSources(ctx, tx, executionID)
	if err != nil {
		return err
	}
	actions, err := loadActionTaskSources(ctx, tx, executionID)
	if err != nil {
		return err
	}
	var lastCargo *uuid.UUID
	lastOrdinal := -1
	for _, stop := range stops {
		if stop.Role == domain.StopRoleCargo && stop.Ordinal >= lastOrdinal {
			lastOrdinal = stop.Ordinal
			lastCargo = stop.LocationID
		}
	}
	byStop := map[uuid.UUID][]actionTaskSource{}
	for _, action := range actions {
		byStop[action.StopID] = append(byStop[action.StopID], action)
	}
	for _, stop := range stops {
		if !driverVisibleStop(stop, lastCargo) || stop.LocationID == nil {
			continue
		}
		summary, shipmentID := summarizeStopActions(byStop[stop.ID])
		raw, err := json.Marshal(summary)
		if err != nil {
			return apperrors.Internal("marshal driver stop action summary", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO transport.driver_stop_tasks (
				id, operating_tenant_id, execution_id, execution_stop_id, driver_id, vehicle_id,
				shipment_id, ordinal, location_id, planned_arrival, action_summary, status, version,
				created_at, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$14)
		`, uuid.New(), tenant, executionID, stop.ID, driverID, vehicleID, shipmentID, stop.Ordinal,
			*stop.LocationID, stop.PlannedArrival, raw, stop.Status, stop.Version, now); err != nil {
			return mapDBError(err)
		}
	}
	return nil
}

func driverVisibleStop(stop stopTaskSource, lastCargo *uuid.UUID) bool {
	if stop.PointKind == domain.PointKindPositionAnchor || stop.LocationID == nil {
		return false
	}
	if stop.Role == domain.StopRoleCargo {
		return true
	}
	if stop.Role != domain.StopRoleEnd {
		return false
	}
	if lastCargo == nil {
		return true
	}
	return *stop.LocationID != *lastCargo
}

func summarizeStopActions(actions []actionTaskSource) (domain.DriverStopActionSummary, *uuid.UUID) {
	summary := domain.DriverStopActionSummary{
		Actions: make([]domain.DriverStopActionFact, 0, len(actions)),
		Counts:  map[string]int{},
	}
	var shipmentID *uuid.UUID
	same := true
	for _, action := range actions {
		summary.Actions = append(summary.Actions, domain.DriverStopActionFact{
			ActionID: action.ID, ActionType: action.ActionType, CargoID: action.CargoID, Ordinal: action.Ordinal,
		})
		summary.Counts[action.ActionType]++
		if shipmentID == nil {
			id := action.ShipmentID
			shipmentID = &id
			continue
		}
		if *shipmentID != action.ShipmentID {
			same = false
		}
	}
	if !same || len(actions) == 0 {
		return summary, nil
	}
	return summary, shipmentID
}

func loadStopTaskSources(ctx context.Context, tx pgx.Tx, executionID uuid.UUID) ([]stopTaskSource, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, ordinal, stop_role, point_kind, location_id, planned_arrival, status, version
		FROM transport.transport_execution_stops
		WHERE execution_id = $1
		ORDER BY ordinal
	`, executionID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	var out []stopTaskSource
	for rows.Next() {
		var stop stopTaskSource
		if err := rows.Scan(&stop.ID, &stop.Ordinal, &stop.Role, &stop.PointKind, &stop.LocationID, &stop.PlannedArrival, &stop.Status, &stop.Version); err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, stop)
	}
	return out, mapDBError(rows.Err())
}

func loadActionTaskSources(ctx context.Context, tx pgx.Tx, executionID uuid.UUID) ([]actionTaskSource, error) {
	rows, err := tx.Query(ctx, `
		SELECT action.id, action.execution_stop_id, action.shipment_id, action.cargo_id, action.action_type, action.ordinal
		FROM transport.transport_execution_actions AS action
		JOIN transport.transport_execution_stops AS stop ON stop.id = action.execution_stop_id
		WHERE stop.execution_id = $1
		ORDER BY action.ordinal
	`, executionID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	var out []actionTaskSource
	for rows.Next() {
		var action actionTaskSource
		if err := rows.Scan(&action.ID, &action.StopID, &action.ShipmentID, &action.CargoID, &action.ActionType, &action.Ordinal); err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, action)
	}
	return out, mapDBError(rows.Err())
}

func (r *TransportExecutionCommandRepository) ListCurrentNext(ctx context.Context, tenantID, driverID uuid.UUID) (domain.DriverCurrentNextStops, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return domain.DriverCurrentNextStops{}, mapDBError(err)
	}
	defer tx.Rollback(ctx)

	eligible, err := tx.Query(ctx, `
		SELECT DISTINCT execution.id
		FROM transport.driver_stop_tasks AS task
		JOIN transport.transport_execution_stops AS stop ON stop.id = task.execution_stop_id
		JOIN transport.transport_executions AS execution ON execution.id = task.execution_id
		JOIN transport.transport_execution_revision_stops AS link
		  ON link.stop_id = stop.id AND link.revision_id = execution.current_revision_id
		 AND link.membership <> 'SUPERSEDED'
		WHERE task.operating_tenant_id = $1
		  AND execution.driver_id = $2
		  AND stop.status IN ('PLANNED', 'ARRIVED', 'SERVICE_STARTED')
	`, tenantID, driverID)
	if err != nil {
		return domain.DriverCurrentNextStops{}, mapDBError(err)
	}
	var executionIDs []uuid.UUID
	for eligible.Next() {
		var id uuid.UUID
		if err := eligible.Scan(&id); err != nil {
			eligible.Close()
			return domain.DriverCurrentNextStops{}, mapDBError(err)
		}
		executionIDs = append(executionIDs, id)
	}
	if err := eligible.Err(); err != nil {
		eligible.Close()
		return domain.DriverCurrentNextStops{}, mapDBError(err)
	}
	eligible.Close()
	if len(executionIDs) == 0 {
		return domain.DriverCurrentNextStops{}, nil
	}
	if len(executionIDs) > 1 {
		return domain.DriverCurrentNextStops{}, domain.ExecutionCommandError(domain.ReasonDriverExecutionAmbiguous, false)
	}

	rows, err := tx.Query(ctx, `
		SELECT task.id, task.execution_id, task.execution_stop_id, task.shipment_id, task.ordinal,
			task.location_id, task.planned_arrival, task.status, task.version, task.action_summary
		FROM transport.driver_stop_tasks AS task
		JOIN transport.transport_execution_stops AS stop ON stop.id = task.execution_stop_id
		JOIN transport.transport_executions AS execution ON execution.id = task.execution_id
		JOIN transport.transport_execution_revision_stops AS link
		  ON link.stop_id = stop.id AND link.revision_id = execution.current_revision_id
		 AND link.membership <> 'SUPERSEDED'
		WHERE task.operating_tenant_id = $1
		  AND execution.driver_id = $2
		  AND task.execution_id = $3
		  AND stop.status IN ('PLANNED', 'ARRIVED', 'SERVICE_STARTED')
		ORDER BY link.source_ordinal
		LIMIT 2
	`, tenantID, driverID, executionIDs[0])
	if err != nil {
		return domain.DriverCurrentNextStops{}, mapDBError(err)
	}
	defer rows.Close()
	var views []domain.DriverStopTaskView
	for rows.Next() {
		view, err := scanDriverStopTask(rows)
		if err != nil {
			return domain.DriverCurrentNextStops{}, err
		}
		views = append(views, view)
	}
	if err := rows.Err(); err != nil {
		return domain.DriverCurrentNextStops{}, mapDBError(err)
	}
	var result domain.DriverCurrentNextStops
	if len(views) > 0 {
		views[0].Position = domain.DriverStopPositionCurrent
		result.Current = &views[0]
	}
	if len(views) > 1 {
		views[1].Position = domain.DriverStopPositionNext
		result.Next = &views[1]
	}
	return result, nil
}

type driverStopRow struct {
	Task           domain.DriverStopTaskView
	Operating      uuid.UUID
	DriverID       *uuid.UUID
	RevisionID     uuid.UUID
	ActionID       uuid.UUID
	ActionType     string
	ShipmentTenant uuid.UUID
}

func (r *TransportExecutionCommandRepository) LoadDriverStop(ctx context.Context, stopID uuid.UUID) (domain.DriverStopTaskView, uuid.UUID, *uuid.UUID, uuid.UUID, error) {
	var view domain.DriverStopTaskView
	var operating uuid.UUID
	var driverID *uuid.UUID
	var revisionID uuid.UUID
	var raw []byte
	err := r.pool.QueryRow(ctx, `
		SELECT task.id, task.execution_id, task.execution_stop_id, task.shipment_id, task.ordinal,
			task.location_id, task.planned_arrival, task.status, task.version, task.action_summary,
			task.operating_tenant_id, execution.driver_id, execution.current_revision_id
		FROM transport.driver_stop_tasks AS task
		JOIN transport.transport_executions AS execution ON execution.id = task.execution_id
		JOIN transport.transport_execution_revision_stops AS link
		  ON link.stop_id = task.execution_stop_id AND link.revision_id = execution.current_revision_id
		 AND link.membership <> 'SUPERSEDED'
		WHERE task.execution_stop_id = $1
	`, stopID).Scan(
		&view.TaskID, &view.ExecutionID, &view.ExecutionStopID, &view.ShipmentID, &view.Ordinal,
		&view.LocationID, &view.PlannedArrival, &view.Status, &view.Version, &raw,
		&operating, &driverID, &revisionID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DriverStopTaskView{}, uuid.Nil, nil, uuid.Nil, apperrors.NotFound("driver stop task not found")
	}
	if err != nil {
		return domain.DriverStopTaskView{}, uuid.Nil, nil, uuid.Nil, mapDBError(err)
	}
	if err := json.Unmarshal(raw, &view.ActionSummary); err != nil {
		return domain.DriverStopTaskView{}, uuid.Nil, nil, uuid.Nil, apperrors.Internal("read driver stop action summary", err)
	}
	return view, operating, driverID, revisionID, nil
}

// LoadDeliveryAction returns the shipment and cargo of a delivery action on this stop.
// An action that belongs to another stop is not found.
func (r *TransportExecutionCommandRepository) LoadDeliveryAction(ctx context.Context, stopID, actionID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	var actionType string
	var owner, shipmentID, cargoID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT action_type, execution_stop_id, shipment_id, cargo_id
		FROM transport.transport_execution_actions
		WHERE id = $1
	`, actionID).Scan(&actionType, &owner, &shipmentID, &cargoID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && owner != stopID) {
		return uuid.Nil, uuid.Nil, apperrors.NotFound("execution action not found")
	}
	if err != nil {
		return uuid.Nil, uuid.Nil, mapDBError(err)
	}
	if actionType != domain.ActionTypeDelivery {
		return uuid.Nil, uuid.Nil, domain.ExecutionCommandError(domain.ReasonActionNotFound, false)
	}
	return shipmentID, cargoID, nil
}

func (r *TransportExecutionCommandRepository) LoadStopAction(ctx context.Context, stopID, actionID uuid.UUID) (string, error) {
	var actionType string
	var owner uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT action.action_type, action.execution_stop_id
		FROM transport.transport_execution_actions AS action
		WHERE action.id = $1
	`, actionID).Scan(&actionType, &owner)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && owner != stopID) {
		return "", apperrors.NotFound("execution action not found")
	}
	if err != nil {
		return "", mapDBError(err)
	}
	return actionType, nil
}

func (r *TransportExecutionCommandRepository) FindAssignedParticipant(ctx context.Context, operatingTenant, driverID, shipmentID uuid.UUID) (uuid.UUID, uuid.UUID, uuid.UUID, bool, error) {
	var executionID, revisionID, shipmentTenant uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT execution.id, execution.current_revision_id, participant.shipment_tenant_id
		FROM transport.transport_execution_active_shipments AS participant
		JOIN transport.transport_executions AS execution ON execution.id = participant.execution_id
		WHERE participant.shipment_id = $1
		  AND execution.operating_tenant_id = $2
		  AND execution.driver_id = $3
		  AND execution.current_revision_id IS NOT NULL
	`, shipmentID, operatingTenant, driverID).Scan(&executionID, &revisionID, &shipmentTenant)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, false, mapDBError(err)
	}
	return executionID, revisionID, shipmentTenant, true, nil
}

func scanDriverStopTask(rows pgx.Rows) (domain.DriverStopTaskView, error) {
	var view domain.DriverStopTaskView
	var raw []byte
	if err := rows.Scan(
		&view.TaskID, &view.ExecutionID, &view.ExecutionStopID, &view.ShipmentID, &view.Ordinal,
		&view.LocationID, &view.PlannedArrival, &view.Status, &view.Version, &raw,
	); err != nil {
		return domain.DriverStopTaskView{}, mapDBError(err)
	}
	if err := json.Unmarshal(raw, &view.ActionSummary); err != nil {
		return domain.DriverStopTaskView{}, apperrors.Internal("read driver stop action summary", err)
	}
	return view, nil
}

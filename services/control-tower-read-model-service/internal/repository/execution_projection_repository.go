package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
)

type ExecutionProjectionRepository struct {
	pool *pgxpool.Pool
}

func NewExecutionProjectionRepository(pool *pgxpool.Pool) *ExecutionProjectionRepository {
	return &ExecutionProjectionRepository{pool: pool}
}

var executionApplyFaultForTest func() error

func SetExecutionApplyFaultForTest(fn func() error) func() {
	previous := executionApplyFaultForTest
	executionApplyFaultForTest = fn
	return func() { executionApplyFaultForTest = previous }
}

type ExecutionView struct {
	OperatingTenantID uuid.UUID
	ExecutionID       uuid.UUID
	ActiveRevisionID  uuid.UUID
	CarrierCompanyID  *uuid.UUID
	DriverID          *uuid.UUID
	VehicleID         *uuid.UUID
	CurrentStopID     *uuid.UUID
	LastEventSequence int64
	LastEventType     string
	GapDetected       bool
	GapFromSequence   *int64
	GapToSequence     *int64
	Stops             []ExecutionStopView
	Actions           []ExecutionActionView
	Progress          []ExecutionProgressView
}

type ExecutionStopView struct {
	ExecutionStopID        uuid.UUID
	RevisionID             uuid.UUID
	Ordinal                int
	StopRole               string
	PointKind              string
	LocationID             *uuid.UUID
	Status                 string
	ArrivedAt              *time.Time
	ServiceStartedAt       *time.Time
	CompletedAt            *time.Time
	ApproachingAt          *time.Time
	ApproachDistanceMeters *float64
	PlannedArrival         *time.Time
	PlannedDeparture       *time.Time
}

type ExecutionActionView struct {
	ActionID        uuid.UUID
	ExecutionStopID uuid.UUID
	RevisionID      uuid.UUID
	ActionType      string
	ShipmentID      uuid.UUID
	CargoID         uuid.UUID
	Status          string
}

type ExecutionProgressView struct {
	EventID         uuid.UUID
	EventType       string
	ExecutionStopID *uuid.UUID
	ActionID        *uuid.UUID
	ShipmentID      *uuid.UUID
	ReasonCode      *string
	Severity        *string
	OccurredAt      time.Time
}

func (r *ExecutionProjectionRepository) ApplyRecord(ctx context.Context, payload []byte, meta domain.KafkaRecordMeta, receivedAt time.Time) error {
	if domain.PayloadHasForbiddenExecutionField(payload) {
		return r.rejectPrivacy(ctx, payload, meta, receivedAt)
	}
	kind, _ := domain.ClassifyEventPayload(payload)
	switch kind {
	case domain.RouteApproach:
		return r.applyApproach(ctx, payload, meta, receivedAt)
	case domain.RouteDisposition:
		return r.applyDisposition(ctx, payload, meta, receivedAt)
	case domain.RouteExecution:
		return r.applyExecution(ctx, payload, meta, receivedAt)
	default:
		return errors.New("unsupported execution record")
	}
}

func (r *ExecutionProjectionRepository) rejectPrivacy(ctx context.Context, payload []byte, meta domain.KafkaRecordMeta, receivedAt time.Time) error {
	eventID := uuid.New()
	var probe struct {
		EventID string `json:"event_id"`
		Camel   string `json:"eventId"`
	}
	_ = json.Unmarshal(payload, &probe)
	if parsed, err := uuid.Parse(firstNonEmpty(probe.EventID, probe.Camel)); err == nil {
		eventID = parsed
	}
	_, err := r.pool.Exec(ctx, `
INSERT INTO control_tower.execution_event_inbox (
  event_id, operating_tenant_id, execution_id, event_type, topic, partition_id, message_offset,
  payload_sha256, processing_outcome, occurred_at, received_at, processed_at
) VALUES ($1,$2,$3,'rejected',$4,$5,$6,$7,'rejected_privacy',$8,$8,$8)
ON CONFLICT (event_id) DO NOTHING
`, eventID, uuid.Nil, uuid.Nil, meta.Topic, meta.Partition, meta.Offset, sha256Hex(payload), receivedAt)
	return err
}

func (r *ExecutionProjectionRepository) applyExecution(ctx context.Context, payload []byte, meta domain.KafkaRecordMeta, receivedAt time.Time) error {
	event, err := parseExecutionEvent(payload)
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	inserted, err := insertExecutionInbox(ctx, tx, event, meta, receivedAt, payload, "pending")
	if err != nil {
		return err
	}
	if !inserted {
		return tx.Commit(ctx)
	}
	if executionApplyFaultForTest != nil {
		if err := executionApplyFaultForTest(); err != nil {
			return err
		}
	}
	if err := applyExecutionTx(ctx, tx, event, receivedAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
UPDATE control_tower.execution_event_inbox
SET processing_outcome='applied', processed_at=now()
WHERE event_id=$1 AND processing_outcome='pending'
`, event.EventID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func applyExecutionTx(ctx context.Context, tx pgx.Tx, event executionEvent, receivedAt time.Time) error {
	if event.EventType == domain.EventExecutionPlanCreated {
		return applyPlanCreated(ctx, tx, event, receivedAt)
	}
	return applyStopEvent(ctx, tx, event, receivedAt)
}

func applyPlanCreated(ctx context.Context, tx pgx.Tx, event executionEvent, receivedAt time.Time) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM control_tower.execution_projection WHERE execution_id=$1)`, event.ExecutionID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		proceed, err := claimSequence(ctx, tx, event, receivedAt)
		if err != nil || !proceed {
			return err
		}
		if err := projectSuccessorPlan(ctx, tx, event, receivedAt); err != nil {
			return err
		}
		if err := noteSequence(ctx, tx, event, receivedAt); err != nil {
			return err
		}
		return drainHeld(ctx, tx, event.ExecutionID, receivedAt)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO control_tower.execution_projection (
  execution_id, operating_tenant_id, active_revision_id, carrier_company_id, driver_id, vehicle_id,
  current_stop_id, last_event_sequence, last_event_id, last_event_type, last_occurred_at, last_consumed_at,
  gap_detected, created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,NULL,$7,$8,$9,$10,$11,false,$11,$11)
`, event.ExecutionID, event.OperatingTenantID, event.RevisionID, event.CarrierCompanyID, event.DriverID, event.VehicleID,
		event.Sequence, event.EventID, event.EventType, event.OccurredAt, receivedAt); err != nil {
		return err
	}
	for _, stop := range event.Stops {
		if _, err := tx.Exec(ctx, `
INSERT INTO control_tower.execution_stop_projection (
  operating_tenant_id, execution_id, revision_id, execution_stop_id, ordinal, stop_role, point_kind, location_id,
  planned_arrival, planned_departure, status, last_event_sequence, last_event_id, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
`, event.OperatingTenantID, event.ExecutionID, event.RevisionID, stop.StopID, stop.Ordinal, stop.StopRole, stop.PointKind,
			stop.LocationID, stop.PlannedArrival, stop.PlannedDeparture, stop.Status, event.Sequence, event.EventID, receivedAt); err != nil {
			return err
		}
	}
	for _, action := range event.Actions {
		if _, err := tx.Exec(ctx, `
INSERT INTO control_tower.execution_action_projection (
  operating_tenant_id, execution_id, revision_id, execution_stop_id, action_id, action_type, shipment_id, cargo_id,
  status, last_event_sequence, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
`, event.OperatingTenantID, event.ExecutionID, event.RevisionID, action.StopID, action.ActionID, action.ActionType,
			action.ShipmentID, action.CargoID, action.Status, event.Sequence, receivedAt); err != nil {
			return err
		}
	}
	if err := insertProgress(ctx, tx, event, nil, nil, nil); err != nil {
		return err
	}
	return drainHeld(ctx, tx, event.ExecutionID, receivedAt)
}

func applyStopEvent(ctx context.Context, tx pgx.Tx, event executionEvent, receivedAt time.Time) error {
	proceed, err := claimSequence(ctx, tx, event, receivedAt)
	if err != nil || !proceed {
		return err
	}
	if event.EventType != domain.EventExecutionPlanSuperseded {
		var active uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT active_revision_id FROM control_tower.execution_projection WHERE execution_id=$1`, event.ExecutionID).Scan(&active); err != nil {
			return err
		}
		if event.RevisionID != active {
			if err := noteSequence(ctx, tx, event, receivedAt); err != nil {
				return err
			}
			if err := markInbox(ctx, tx, event.EventID, "ignored_stale_revision"); err != nil {
				return err
			}
			return drainHeld(ctx, tx, event.ExecutionID, receivedAt)
		}
	}
	if err := mutateStop(ctx, tx, event, receivedAt); err != nil {
		return err
	}
	if err := noteSequence(ctx, tx, event, receivedAt); err != nil {
		return err
	}
	return drainHeld(ctx, tx, event.ExecutionID, receivedAt)
}

func claimSequence(ctx context.Context, tx pgx.Tx, event executionEvent, receivedAt time.Time) (bool, error) {
	var last int64
	var gap bool
	err := tx.QueryRow(ctx, `SELECT last_event_sequence, gap_detected FROM control_tower.execution_projection WHERE execution_id=$1 FOR UPDATE`, event.ExecutionID).Scan(&last, &gap)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, holdGap(ctx, tx, event, 1, event.Sequence, receivedAt)
	}
	if err != nil {
		return false, err
	}
	if event.Sequence <= last {
		return false, markInbox(ctx, tx, event.EventID, "ignored_old")
	}
	if event.Sequence > last+1 {
		return false, holdGap(ctx, tx, event, last+1, event.Sequence-1, receivedAt)
	}
	return true, nil
}

func projectSuccessorPlan(ctx context.Context, tx pgx.Tx, event executionEvent, receivedAt time.Time) error {
	var active uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT active_revision_id FROM control_tower.execution_projection WHERE execution_id=$1`, event.ExecutionID).Scan(&active); err != nil {
		return err
	}
	if event.RevisionID == active || event.RevisionID == uuid.Nil {
		return nil
	}
	var current *uuid.UUID
	best := int(^uint(0) >> 1)
	for _, stop := range event.Stops {
		if stop.Status != "PLANNED" || stop.Ordinal >= best {
			continue
		}
		id := stop.StopID
		current = &id
		best = stop.Ordinal
	}
	if _, err := tx.Exec(ctx, `
UPDATE control_tower.execution_projection
SET active_revision_id=$2,
    current_stop_id=COALESCE($3, current_stop_id),
    updated_at=$4
WHERE execution_id=$1
`, event.ExecutionID, event.RevisionID, current, receivedAt); err != nil {
		return err
	}
	for _, stop := range event.Stops {
		if stop.Status == "COMPLETED" || stop.Status == "SKIPPED" || stop.Status == "CANCELLED" {
			continue
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO control_tower.execution_stop_projection (
  operating_tenant_id, execution_id, revision_id, execution_stop_id, ordinal, stop_role, point_kind, location_id,
  planned_arrival, planned_departure, status, last_event_sequence, last_event_id, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
ON CONFLICT (execution_id, revision_id, execution_stop_id) DO NOTHING
`, event.OperatingTenantID, event.ExecutionID, event.RevisionID, stop.StopID, stop.Ordinal, stop.StopRole, stop.PointKind,
			stop.LocationID, stop.PlannedArrival, stop.PlannedDeparture, stop.Status, event.Sequence, event.EventID, receivedAt); err != nil {
			return err
		}
	}
	for _, action := range event.Actions {
		if action.Status == "COMPLETED" || action.Status == "FAILED" || action.Status == "CANCELLED" {
			continue
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO control_tower.execution_action_projection (
  operating_tenant_id, execution_id, revision_id, execution_stop_id, action_id, action_type, shipment_id, cargo_id,
  status, last_event_sequence, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
ON CONFLICT (execution_id, revision_id, action_id) DO NOTHING
`, event.OperatingTenantID, event.ExecutionID, event.RevisionID, action.StopID, action.ActionID, action.ActionType,
			action.ShipmentID, action.CargoID, action.Status, event.Sequence, receivedAt); err != nil {
			return err
		}
	}
	return nil
}

func mutateStop(ctx context.Context, tx pgx.Tx, event executionEvent, receivedAt time.Time) error {
	switch event.EventType {
	case domain.EventRouteStopCurrent:
		_, err := tx.Exec(ctx, `
UPDATE control_tower.execution_projection AS execution
SET current_stop_id=$2, updated_at=$3
WHERE execution.execution_id=$1
  AND EXISTS (
    SELECT 1 FROM control_tower.execution_stop_projection AS stop
    WHERE stop.execution_id=execution.execution_id
      AND stop.revision_id=execution.active_revision_id
      AND stop.execution_stop_id=$2
      AND stop.status IN ('PLANNED','ARRIVED','SERVICE_STARTED')
  )
`, event.ExecutionID, event.StopID, receivedAt)
		return err
	case domain.EventRouteStopArrived:
		_, err := tx.Exec(ctx, `
UPDATE control_tower.execution_stop_projection AS stop
SET status='ARRIVED', arrived_at=COALESCE(stop.arrived_at,$3), last_event_sequence=$4, last_event_id=$5, updated_at=$6
FROM control_tower.execution_projection AS execution
WHERE stop.execution_id=execution.execution_id
  AND stop.revision_id=execution.active_revision_id
  AND stop.execution_id=$1 AND stop.execution_stop_id=$2
  AND stop.status NOT IN ('COMPLETED','SUPERSEDED','CANCELLED','SKIPPED')
`, event.ExecutionID, event.StopID, event.OccurredAt, event.Sequence, event.EventID, receivedAt)
		return err
	case domain.EventExecutionPlanSuperseded:
		_, err := tx.Exec(ctx, `
UPDATE control_tower.execution_stop_projection
SET status='SUPERSEDED', updated_at=$3
WHERE execution_id=$1 AND revision_id=$2 AND status IN ('PLANNED','ARRIVED','SERVICE_STARTED')
`, event.ExecutionID, event.RevisionID, receivedAt)
		return err
	case domain.EventRouteStopServiceStarted:
		_, err := tx.Exec(ctx, `
UPDATE control_tower.execution_stop_projection AS stop
SET status='SERVICE_STARTED', service_started_at=COALESCE(stop.service_started_at,$3), last_event_sequence=$4, last_event_id=$5, updated_at=$6
FROM control_tower.execution_projection AS execution
WHERE stop.execution_id=execution.execution_id
  AND stop.revision_id=execution.active_revision_id
  AND stop.execution_id=$1 AND stop.execution_stop_id=$2
  AND stop.status NOT IN ('COMPLETED','SUPERSEDED','CANCELLED','SKIPPED')
`, event.ExecutionID, event.StopID, event.OccurredAt, event.Sequence, event.EventID, receivedAt)
		return err
	case domain.EventRouteStopCompleted:
		if _, err := tx.Exec(ctx, `
UPDATE control_tower.execution_stop_projection AS stop
SET status='COMPLETED', completed_at=COALESCE(stop.completed_at,$3), last_event_sequence=$4, last_event_id=$5, updated_at=$6
FROM control_tower.execution_projection AS execution
WHERE stop.execution_id=execution.execution_id
  AND stop.revision_id=execution.active_revision_id
  AND stop.execution_id=$1 AND stop.execution_stop_id=$2
  AND stop.status NOT IN ('COMPLETED','SUPERSEDED','CANCELLED','SKIPPED')
`, event.ExecutionID, event.StopID, event.OccurredAt, event.Sequence, event.EventID, receivedAt); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
UPDATE control_tower.execution_projection
SET current_stop_id=NULL, updated_at=$3
WHERE execution_id=$1 AND current_stop_id=$2 AND active_revision_id=$4
`, event.ExecutionID, event.StopID, receivedAt, event.RevisionID)
		return err
	case domain.EventRouteStopOverridden:
		return insertProgress(ctx, tx, event, event.StopID, nil, nil)
	default:
		return nil
	}
}

func noteSequence(ctx context.Context, tx pgx.Tx, event executionEvent, receivedAt time.Time) error {
	if _, err := tx.Exec(ctx, `
UPDATE control_tower.execution_projection
SET last_event_sequence=$2, last_event_id=$3, last_event_type=$4, last_occurred_at=$5, last_consumed_at=$6, updated_at=$6,
    gap_detected=false, gap_from_sequence=NULL, gap_to_sequence=NULL
WHERE execution_id=$1 AND last_event_sequence < $2
`, event.ExecutionID, event.Sequence, event.EventID, event.EventType, event.OccurredAt, receivedAt); err != nil {
		return err
	}
	if event.EventType != domain.EventRouteStopOverridden {
		return insertProgress(ctx, tx, event, event.StopID, event.ActionID, nil)
	}
	return nil
}

func holdGap(ctx context.Context, tx pgx.Tx, event executionEvent, from, to int64, receivedAt time.Time) error {
	if from > to {
		to = from
	}
	if _, err := tx.Exec(ctx, `
UPDATE control_tower.execution_projection
SET gap_detected=true, gap_from_sequence=$2, gap_to_sequence=$3, updated_at=$4
WHERE execution_id=$1
`, event.ExecutionID, from, to, receivedAt); err != nil {
		return err
	}
	raw, _ := json.Marshal(event)
	_, err := tx.Exec(ctx, `
UPDATE control_tower.execution_event_inbox
SET processing_outcome='gap_held', held_payload=$2::jsonb, processed_at=$3
WHERE event_id=$1
`, event.EventID, string(raw), receivedAt)
	return err
}

func applyHeldExecution(ctx context.Context, tx pgx.Tx, event executionEvent, receivedAt time.Time) error {
	if event.EventType == domain.EventExecutionPlanCreated {
		return projectSuccessorPlan(ctx, tx, event, receivedAt)
	}
	if event.EventType != domain.EventExecutionPlanSuperseded {
		var active uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT active_revision_id FROM control_tower.execution_projection WHERE execution_id=$1`, event.ExecutionID).Scan(&active); err != nil {
			return err
		}
		if event.RevisionID != active {
			return nil
		}
	}
	return mutateStop(ctx, tx, event, receivedAt)
}

func drainHeld(ctx context.Context, tx pgx.Tx, executionID uuid.UUID, receivedAt time.Time) error {
	for {
		var last int64
		if err := tx.QueryRow(ctx, `SELECT last_event_sequence FROM control_tower.execution_projection WHERE execution_id=$1`, executionID).Scan(&last); err != nil {
			return err
		}
		var raw []byte
		err := tx.QueryRow(ctx, `
SELECT held_payload FROM control_tower.execution_event_inbox
WHERE execution_id=$1 AND processing_outcome='gap_held' AND event_sequence=$2
ORDER BY event_sequence
LIMIT 1
`, executionID, last+1).Scan(&raw)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		var event executionEvent
		if err := json.Unmarshal(raw, &event); err != nil {
			return err
		}
		if err := applyHeldExecution(ctx, tx, event, receivedAt); err != nil {
			return err
		}
		if err := noteSequence(ctx, tx, event, receivedAt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE control_tower.execution_event_inbox SET processing_outcome='applied', held_payload=NULL WHERE event_id=$1`, event.EventID); err != nil {
			return err
		}
	}
}

func (r *ExecutionProjectionRepository) applyApproach(ctx context.Context, payload []byte, meta domain.KafkaRecordMeta, receivedAt time.Time) error {
	event, err := parseApproach(payload)
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var operating uuid.UUID
	err = tx.QueryRow(ctx, `SELECT operating_tenant_id FROM control_tower.execution_projection WHERE execution_id=$1 FOR UPDATE`, event.ExecutionID).Scan(&operating)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && operating != event.OperatingTenantID) {
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	inserted, err := insertApproachInbox(ctx, tx, event, meta, receivedAt, payload)
	if err != nil || !inserted {
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	tag, err := tx.Exec(ctx, `
UPDATE control_tower.execution_stop_projection AS stop
SET approaching_at=COALESCE(stop.approaching_at,$4),
    approach_distance_meters=COALESCE(stop.approach_distance_meters,$5),
    updated_at=$4
FROM control_tower.execution_projection AS execution
WHERE stop.execution_id=execution.execution_id
  AND stop.revision_id=execution.active_revision_id
  AND stop.execution_id=$1 AND stop.execution_stop_id=$2 AND stop.operating_tenant_id=$3
  AND stop.status NOT IN ('COMPLETED','SUPERSEDED','CANCELLED','SKIPPED')
`, event.ExecutionID, event.StopID, event.OperatingTenantID, event.OccurredAt, event.DistanceMeters)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		_, err = tx.Exec(ctx, `UPDATE control_tower.execution_event_inbox SET processing_outcome='rejected_unknown_stop' WHERE event_id=$1`, event.EventID)
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if err := insertProgress(ctx, tx, executionEvent{
		EventID: event.EventID, EventType: domain.EventStopApproaching, OperatingTenantID: event.OperatingTenantID,
		ExecutionID: event.ExecutionID, RevisionID: event.RevisionID, OccurredAt: event.OccurredAt,
	}, &event.StopID, nil, nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *ExecutionProjectionRepository) LinkDriverContext(ctx context.Context, tenantID, eventID, shipmentID uuid.UUID, eventType string, stopID, actionID *uuid.UUID, occurred time.Time, reason, severity string) error {
	if stopID == nil {
		return nil
	}
	var executionID, operating uuid.UUID
	err := r.pool.QueryRow(ctx, `
SELECT execution_id, operating_tenant_id
FROM control_tower.execution_action_projection
WHERE execution_stop_id=$1 AND shipment_id=$2
LIMIT 1
`, *stopID, shipmentID).Scan(&executionID, &operating)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if actionID != nil {
		var owned bool
		if err := r.pool.QueryRow(ctx, `
SELECT EXISTS(
  SELECT 1 FROM control_tower.execution_action_projection
  WHERE action_id=$1 AND execution_stop_id=$2 AND shipment_id=$3
)`, *actionID, *stopID, shipmentID).Scan(&owned); err != nil {
			return err
		}
		if !owned {
			return nil
		}
	}
	_, err = r.pool.Exec(ctx, `
INSERT INTO control_tower.execution_progress_event (
  operating_tenant_id, event_id, execution_id, execution_stop_id, action_id, event_type, shipment_id, reason_code, severity, occurred_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
ON CONFLICT (operating_tenant_id, event_id) DO NOTHING
`, operating, eventID, executionID, stopID, actionID, eventType, shipmentID, emptyToNil(reason), emptyToNil(severity), occurred)
	return err
}

func (r *ExecutionProjectionRepository) List(ctx context.Context, tenantID uuid.UUID, driverID, vehicleID *uuid.UUID, limit, offset int) ([]ExecutionView, error) {
	rows, err := r.pool.Query(ctx, `
SELECT execution_id FROM control_tower.execution_projection
WHERE operating_tenant_id=$1
  AND ($2::uuid IS NULL OR driver_id=$2)
  AND ($3::uuid IS NULL OR vehicle_id=$3)
ORDER BY updated_at DESC
LIMIT $4 OFFSET $5
`, tenantID, driverID, vehicleID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExecutionView
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		view, err := r.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if view != nil {
			out = append(out, *view)
		}
	}
	return out, rows.Err()
}

func (r *ExecutionProjectionRepository) Get(ctx context.Context, executionID uuid.UUID) (*ExecutionView, error) {
	var view ExecutionView
	err := r.pool.QueryRow(ctx, `
SELECT operating_tenant_id, execution_id, active_revision_id, carrier_company_id, driver_id, vehicle_id, current_stop_id,
       last_event_sequence, last_event_type, gap_detected, gap_from_sequence, gap_to_sequence
FROM control_tower.execution_projection WHERE execution_id=$1
`, executionID).Scan(&view.OperatingTenantID, &view.ExecutionID, &view.ActiveRevisionID, &view.CarrierCompanyID, &view.DriverID, &view.VehicleID, &view.CurrentStopID, &view.LastEventSequence, &view.LastEventType, &view.GapDetected, &view.GapFromSequence, &view.GapToSequence)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	stops, err := r.pool.Query(ctx, `
SELECT execution_stop_id, revision_id, ordinal, stop_role, point_kind, location_id, status, arrived_at, service_started_at, completed_at, approaching_at, approach_distance_meters, planned_arrival, planned_departure
FROM control_tower.execution_stop_projection WHERE execution_id=$1 ORDER BY ordinal
`, executionID)
	if err != nil {
		return nil, err
	}
	defer stops.Close()
	for stops.Next() {
		var stop ExecutionStopView
		if err := stops.Scan(&stop.ExecutionStopID, &stop.RevisionID, &stop.Ordinal, &stop.StopRole, &stop.PointKind, &stop.LocationID, &stop.Status, &stop.ArrivedAt, &stop.ServiceStartedAt, &stop.CompletedAt, &stop.ApproachingAt, &stop.ApproachDistanceMeters, &stop.PlannedArrival, &stop.PlannedDeparture); err != nil {
			return nil, err
		}
		view.Stops = append(view.Stops, stop)
	}
	actions, err := r.pool.Query(ctx, `
SELECT action_id, execution_stop_id, revision_id, action_type, shipment_id, cargo_id, status
FROM control_tower.execution_action_projection WHERE execution_id=$1 ORDER BY action_type
`, executionID)
	if err != nil {
		return nil, err
	}
	defer actions.Close()
	for actions.Next() {
		var action ExecutionActionView
		if err := actions.Scan(&action.ActionID, &action.ExecutionStopID, &action.RevisionID, &action.ActionType, &action.ShipmentID, &action.CargoID, &action.Status); err != nil {
			return nil, err
		}
		view.Actions = append(view.Actions, action)
	}
	progress, err := r.pool.Query(ctx, `
SELECT event_id, event_type, execution_stop_id, action_id, shipment_id, reason_code, severity, occurred_at
FROM control_tower.execution_progress_event WHERE execution_id=$1 ORDER BY occurred_at
`, executionID)
	if err != nil {
		return nil, err
	}
	defer progress.Close()
	for progress.Next() {
		var row ExecutionProgressView
		if err := progress.Scan(&row.EventID, &row.EventType, &row.ExecutionStopID, &row.ActionID, &row.ShipmentID, &row.ReasonCode, &row.Severity, &row.OccurredAt); err != nil {
			return nil, err
		}
		view.Progress = append(view.Progress, row)
	}
	return &view, nil
}

type executionEvent struct {
	EventID           uuid.UUID    `json:"event_id"`
	EventType         string       `json:"event_type"`
	OperatingTenantID uuid.UUID    `json:"operating_tenant_id"`
	ExecutionID       uuid.UUID    `json:"execution_id"`
	RevisionID        uuid.UUID    `json:"revision_id"`
	Sequence          int64        `json:"event_sequence"`
	StopID            *uuid.UUID   `json:"stop_id"`
	ActionID          *uuid.UUID   `json:"action_id"`
	OccurredAt        time.Time    `json:"occurred_at"`
	CarrierCompanyID  *uuid.UUID   `json:"carrier_company_id"`
	DriverID          *uuid.UUID   `json:"driver_id"`
	VehicleID         *uuid.UUID   `json:"vehicle_id"`
	Stops             []planStop   `json:"stops"`
	Actions           []planAction `json:"actions"`
}

type planStop struct {
	StopID           uuid.UUID  `json:"stop_id"`
	Ordinal          int        `json:"ordinal"`
	StopRole         string     `json:"stop_role"`
	PointKind        string     `json:"point_kind"`
	LocationID       *uuid.UUID `json:"location_id"`
	PlannedArrival   *time.Time `json:"planned_arrival"`
	PlannedDeparture *time.Time `json:"planned_departure"`
	Status           string     `json:"status"`
}

type planAction struct {
	ActionID   uuid.UUID `json:"action_id"`
	StopID     uuid.UUID `json:"stop_id"`
	ActionType string    `json:"action_type"`
	ShipmentID uuid.UUID `json:"shipment_id"`
	CargoID    uuid.UUID `json:"cargo_id"`
	Status     string    `json:"status"`
}

func parseExecutionEvent(payload []byte) (executionEvent, error) {
	normalized := bytes.ReplaceAll(payload, []byte(`"stop_id":""`), []byte(`"stop_id":null`))
	normalized = bytes.ReplaceAll(normalized, []byte(`"action_id":""`), []byte(`"action_id":null`))
	var event executionEvent
	if err := json.Unmarshal(normalized, &event); err != nil {
		return executionEvent{}, err
	}
	if event.EventID == uuid.Nil || event.ExecutionID == uuid.Nil || event.OperatingTenantID == uuid.Nil || event.Sequence <= 0 {
		return executionEvent{}, errors.New("execution event identity is incomplete")
	}
	return event, nil
}

type approachEvent struct {
	EventID           uuid.UUID
	OperatingTenantID uuid.UUID
	ExecutionID       uuid.UUID
	RevisionID        uuid.UUID
	StopID            uuid.UUID
	OccurredAt        time.Time
	DistanceMeters    *float64
}

func parseApproach(payload []byte) (approachEvent, error) {
	var raw struct {
		EventID           string   `json:"eventId"`
		EventIDSnake      string   `json:"event_id"`
		OperatingTenantID string   `json:"operatingTenantId"`
		OperatingSnake    string   `json:"operating_tenant_id"`
		ExecutionID       string   `json:"executionId"`
		ExecutionSnake    string   `json:"execution_id"`
		RevisionID        string   `json:"revisionId"`
		RevisionSnake     string   `json:"revision_id"`
		StopID            string   `json:"executionStopId"`
		StopSnake         string   `json:"execution_stop_id"`
		OccurredAt        string   `json:"occurredAt"`
		OccurredSnake     string   `json:"occurred_at"`
		Distance          *float64 `json:"distanceMeters"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return approachEvent{}, err
	}
	eventID, err := uuid.Parse(firstNonEmpty(raw.EventID, raw.EventIDSnake))
	if err != nil {
		return approachEvent{}, err
	}
	operating, err := uuid.Parse(firstNonEmpty(raw.OperatingTenantID, raw.OperatingSnake))
	if err != nil {
		return approachEvent{}, err
	}
	executionID, err := uuid.Parse(firstNonEmpty(raw.ExecutionID, raw.ExecutionSnake))
	if err != nil {
		return approachEvent{}, err
	}
	revisionID, err := uuid.Parse(firstNonEmpty(raw.RevisionID, raw.RevisionSnake))
	if err != nil {
		return approachEvent{}, err
	}
	stopID, err := uuid.Parse(firstNonEmpty(raw.StopID, raw.StopSnake))
	if err != nil {
		return approachEvent{}, err
	}
	occurred, err := time.Parse(time.RFC3339Nano, firstNonEmpty(raw.OccurredAt, raw.OccurredSnake))
	if err != nil {
		occurred, err = time.Parse(time.RFC3339, firstNonEmpty(raw.OccurredAt, raw.OccurredSnake))
		if err != nil {
			return approachEvent{}, err
		}
	}
	return approachEvent{EventID: eventID, OperatingTenantID: operating, ExecutionID: executionID, RevisionID: revisionID, StopID: stopID, OccurredAt: occurred, DistanceMeters: raw.Distance}, nil
}

func insertExecutionInbox(ctx context.Context, tx pgx.Tx, event executionEvent, meta domain.KafkaRecordMeta, receivedAt time.Time, payload []byte, outcome string) (bool, error) {
	tag, err := tx.Exec(ctx, `
INSERT INTO control_tower.execution_event_inbox (
  event_id, operating_tenant_id, execution_id, revision_id, event_sequence, event_type, topic, partition_id, message_offset,
  payload_sha256, processing_outcome, occurred_at, received_at, processed_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13)
ON CONFLICT (event_id) DO NOTHING
`, event.EventID, event.OperatingTenantID, event.ExecutionID, event.RevisionID, event.Sequence, event.EventType, meta.Topic, meta.Partition, meta.Offset, sha256Hex(payload), outcome, event.OccurredAt, receivedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func insertApproachInbox(ctx context.Context, tx pgx.Tx, event approachEvent, meta domain.KafkaRecordMeta, receivedAt time.Time, payload []byte) (bool, error) {
	tag, err := tx.Exec(ctx, `
INSERT INTO control_tower.execution_event_inbox (
  event_id, operating_tenant_id, execution_id, revision_id, event_type, topic, partition_id, message_offset,
  payload_sha256, processing_outcome, occurred_at, received_at, processed_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'applied',$10,$11,$11)
ON CONFLICT (event_id) DO NOTHING
`, event.EventID, event.OperatingTenantID, event.ExecutionID, event.RevisionID, domain.EventStopApproaching, meta.Topic, meta.Partition, meta.Offset, sha256Hex(payload), event.OccurredAt, receivedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func insertProgress(ctx context.Context, tx pgx.Tx, event executionEvent, stopID, actionID, shipmentID *uuid.UUID) error {
	_, err := tx.Exec(ctx, `
INSERT INTO control_tower.execution_progress_event (
  operating_tenant_id, event_id, execution_id, revision_id, execution_stop_id, action_id, event_type, shipment_id, occurred_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT (operating_tenant_id, event_id) DO NOTHING
`, event.OperatingTenantID, event.EventID, event.ExecutionID, event.RevisionID, stopID, actionID, event.EventType, shipmentID, event.OccurredAt)
	return err
}

func markInbox(ctx context.Context, tx pgx.Tx, eventID uuid.UUID, outcome string) error {
	_, err := tx.Exec(ctx, `UPDATE control_tower.execution_event_inbox SET processing_outcome=$2, processed_at=now() WHERE event_id=$1`, eventID, outcome)
	return err
}

func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func emptyToNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

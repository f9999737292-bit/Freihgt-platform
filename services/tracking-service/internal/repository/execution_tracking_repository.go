package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ExecutionTrackingState struct {
	OperatingTenantID          uuid.UUID
	ExecutionID                uuid.UUID
	RevisionID                 uuid.UUID
	CurrentStopID              *uuid.UUID
	CurrentStopOrdinal         *int
	PlannedArrival             *time.Time
	LocationID                 *uuid.UUID
	TargetLatitude             *float64
	TargetLongitude            *float64
	LiveETAStopID              *uuid.UUID
	LiveETAOrdinal             *int
	DriverID                   *uuid.UUID
	VehicleID                  *uuid.UUID
	LastExecutionEventSequence int64
	ApproachEmittedStopID      *uuid.UUID
	ApproachEmittedAt          *time.Time
	UpdatedAt                  time.Time
}

type ExecutionStopETAState struct {
	OperatingTenantID  uuid.UUID
	ExecutionID        uuid.UUID
	ExecutionStopID    uuid.UUID
	Status             string
	EstimatedArrivalAt *time.Time
	SourceType         *string
	ProviderCode       *string
	SourceObservedAt   *time.Time
	ReceivedAt         *time.Time
	FreshnessStatus    string
	QualityStatus      string
	QualityReasons     []string
	AgeSeconds         *int64
	PlannedArrival     *time.Time
	Version            int64
	UpdatedAt          time.Time
}

type ExecutionEventRow struct {
	EventID           uuid.UUID
	OperatingTenantID uuid.UUID
	ExecutionID       uuid.UUID
	EventSequence     int64
	EventType         string
}

type ExecutionTrackingRepository struct {
	pool *pgxpool.Pool
}

func NewExecutionTrackingRepository(pool *pgxpool.Pool) *ExecutionTrackingRepository {
	return &ExecutionTrackingRepository{pool: pool}
}

func (r *ExecutionTrackingRepository) Begin(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

func InsertExecutionInbox(ctx context.Context, tx pgx.Tx, row ExecutionEventRow) (bool, error) {
	tag, err := tx.Exec(ctx, `
		INSERT INTO tracking.execution_event_inbox (
			event_id, operating_tenant_id, execution_id, event_sequence, event_type
		) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (event_id) DO NOTHING
	`, row.EventID, row.OperatingTenantID, row.ExecutionID, row.EventSequence, row.EventType)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func LockExecutionTrackingState(ctx context.Context, tx pgx.Tx, executionID uuid.UUID) (*ExecutionTrackingState, error) {
	const q = `
SELECT operating_tenant_id, execution_id, revision_id, current_stop_id, current_stop_ordinal,
       planned_arrival, location_id, target_latitude, target_longitude, live_eta_stop_id, live_eta_ordinal,
       driver_id, vehicle_id, last_execution_event_sequence, approach_emitted_stop_id, approach_emitted_at, updated_at
FROM tracking.execution_tracking_state
WHERE execution_id = $1
FOR UPDATE`
	row := tx.QueryRow(ctx, q, executionID)
	state, err := scanExecutionTrackingState(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return state, err
}

func UpsertExecutionTrackingState(ctx context.Context, tx pgx.Tx, state ExecutionTrackingState) error {
	_, err := tx.Exec(ctx, `
INSERT INTO tracking.execution_tracking_state (
  operating_tenant_id, execution_id, revision_id, current_stop_id, current_stop_ordinal,
  planned_arrival, location_id, target_latitude, target_longitude, live_eta_stop_id, live_eta_ordinal,
  driver_id, vehicle_id, last_execution_event_sequence, approach_emitted_stop_id, approach_emitted_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,NULL,NULL,now())
ON CONFLICT (execution_id) DO UPDATE SET
  operating_tenant_id = EXCLUDED.operating_tenant_id,
  revision_id = EXCLUDED.revision_id,
  current_stop_id = EXCLUDED.current_stop_id,
  current_stop_ordinal = EXCLUDED.current_stop_ordinal,
  planned_arrival = EXCLUDED.planned_arrival,
  location_id = EXCLUDED.location_id,
  target_latitude = EXCLUDED.target_latitude,
  target_longitude = EXCLUDED.target_longitude,
  live_eta_stop_id = EXCLUDED.live_eta_stop_id,
  live_eta_ordinal = EXCLUDED.live_eta_ordinal,
  driver_id = EXCLUDED.driver_id,
  vehicle_id = EXCLUDED.vehicle_id,
  last_execution_event_sequence = EXCLUDED.last_execution_event_sequence,
  approach_emitted_stop_id = CASE
    WHEN tracking.execution_tracking_state.live_eta_stop_id IS DISTINCT FROM EXCLUDED.live_eta_stop_id THEN NULL
    ELSE tracking.execution_tracking_state.approach_emitted_stop_id
  END,
  approach_emitted_at = CASE
    WHEN tracking.execution_tracking_state.live_eta_stop_id IS DISTINCT FROM EXCLUDED.live_eta_stop_id THEN NULL
    ELSE tracking.execution_tracking_state.approach_emitted_at
  END,
  updated_at = now()
WHERE tracking.execution_tracking_state.operating_tenant_id = EXCLUDED.operating_tenant_id
  AND tracking.execution_tracking_state.last_execution_event_sequence < EXCLUDED.last_execution_event_sequence
`, state.OperatingTenantID, state.ExecutionID, state.RevisionID, state.CurrentStopID, state.CurrentStopOrdinal,
		state.PlannedArrival, state.LocationID, state.TargetLatitude, state.TargetLongitude, state.LiveETAStopID, state.LiveETAOrdinal,
		state.DriverID, state.VehicleID, state.LastExecutionEventSequence)
	return err
}

func ClearExecutionTarget(ctx context.Context, tx pgx.Tx, operatingTenantID, executionID, stopID uuid.UUID, sequence int64) error {
	_, err := tx.Exec(ctx, `
UPDATE tracking.execution_tracking_state
SET current_stop_id = CASE WHEN current_stop_id = $3 OR live_eta_stop_id = $3 THEN NULL ELSE current_stop_id END,
    current_stop_ordinal = CASE WHEN current_stop_id = $3 OR live_eta_stop_id = $3 THEN NULL ELSE current_stop_ordinal END,
    live_eta_stop_id = CASE WHEN current_stop_id = $3 OR live_eta_stop_id = $3 THEN NULL ELSE live_eta_stop_id END,
    live_eta_ordinal = CASE WHEN current_stop_id = $3 OR live_eta_stop_id = $3 THEN NULL ELSE live_eta_ordinal END,
    planned_arrival = CASE WHEN current_stop_id = $3 OR live_eta_stop_id = $3 THEN NULL ELSE planned_arrival END,
    location_id = CASE WHEN current_stop_id = $3 OR live_eta_stop_id = $3 THEN NULL ELSE location_id END,
    target_latitude = CASE WHEN current_stop_id = $3 OR live_eta_stop_id = $3 THEN NULL ELSE target_latitude END,
    target_longitude = CASE WHEN current_stop_id = $3 OR live_eta_stop_id = $3 THEN NULL ELSE target_longitude END,
    approach_emitted_stop_id = CASE WHEN current_stop_id = $3 OR live_eta_stop_id = $3 THEN NULL ELSE approach_emitted_stop_id END,
    approach_emitted_at = CASE WHEN current_stop_id = $3 OR live_eta_stop_id = $3 THEN NULL ELSE approach_emitted_at END,
    last_execution_event_sequence = $4,
    updated_at = now()
WHERE execution_id = $2
  AND operating_tenant_id = $1
  AND last_execution_event_sequence < $4
`, operatingTenantID, executionID, stopID, sequence)
	return err
}

func TouchExecutionSequence(ctx context.Context, tx pgx.Tx, operatingTenantID, executionID uuid.UUID, sequence int64) error {
	_, err := tx.Exec(ctx, `
UPDATE tracking.execution_tracking_state
SET last_execution_event_sequence = $3, updated_at = now()
WHERE execution_id = $2 AND operating_tenant_id = $1 AND last_execution_event_sequence < $3
`, operatingTenantID, executionID, sequence)
	return err
}

func (r *ExecutionTrackingRepository) GetState(ctx context.Context, executionID uuid.UUID) (*ExecutionTrackingState, error) {
	const q = `
SELECT operating_tenant_id, execution_id, revision_id, current_stop_id, current_stop_ordinal,
       planned_arrival, location_id, target_latitude, target_longitude, live_eta_stop_id, live_eta_ordinal,
       driver_id, vehicle_id, last_execution_event_sequence, approach_emitted_stop_id, approach_emitted_at, updated_at
FROM tracking.execution_tracking_state
WHERE execution_id = $1`
	state, err := scanExecutionTrackingState(r.pool.QueryRow(ctx, q, executionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return state, err
}

func (r *ExecutionTrackingRepository) ListActiveTargetsForActors(ctx context.Context, driverIDs, vehicleIDs []uuid.UUID) ([]ExecutionTrackingState, error) {
	if len(driverIDs) == 0 && len(vehicleIDs) == 0 {
		return nil, nil
	}
	if driverIDs == nil {
		driverIDs = []uuid.UUID{}
	}
	if vehicleIDs == nil {
		vehicleIDs = []uuid.UUID{}
	}
	rows, err := r.pool.Query(ctx, `
SELECT operating_tenant_id, execution_id, revision_id, current_stop_id, current_stop_ordinal,
       planned_arrival, location_id, target_latitude, target_longitude, live_eta_stop_id, live_eta_ordinal,
       driver_id, vehicle_id, last_execution_event_sequence, approach_emitted_stop_id, approach_emitted_at, updated_at
FROM tracking.execution_tracking_state
WHERE live_eta_stop_id IS NOT NULL
  AND (
    (cardinality($1::uuid[]) > 0 AND driver_id IS NOT NULL AND driver_id = ANY($1::uuid[]))
    OR (cardinality($2::uuid[]) > 0 AND vehicle_id IS NOT NULL AND vehicle_id = ANY($2::uuid[]))
  )`, driverIDs, vehicleIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExecutionTrackingState
	for rows.Next() {
		state, err := scanExecutionTrackingState(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *state)
	}
	return out, rows.Err()
}

var executionStopETAUpsertForTest func(context.Context, pgx.Tx, ExecutionStopETAState) error

func SetExecutionStopETAUpsertForTest(fn func(context.Context, pgx.Tx, ExecutionStopETAState) error) func() {
	previous := executionStopETAUpsertForTest
	executionStopETAUpsertForTest = fn
	return func() { executionStopETAUpsertForTest = previous }
}

func (r *ExecutionTrackingRepository) UpsertExecutionStopETA(ctx context.Context, state ExecutionStopETAState) error {
	return upsertExecutionStopETA(ctx, r.pool, state)
}

func (r *ExecutionTrackingRepository) UpsertExecutionStopETATx(ctx context.Context, tx pgx.Tx, state ExecutionStopETAState) error {
	if executionStopETAUpsertForTest != nil {
		return executionStopETAUpsertForTest(ctx, tx, state)
	}
	return upsertExecutionStopETA(ctx, tx, state)
}

func upsertExecutionStopETA(ctx context.Context, q sqlExec, state ExecutionStopETAState) error {
	reasons, _ := json.Marshal(state.QualityReasons)
	_, err := q.Exec(ctx, `
INSERT INTO tracking.execution_stop_eta_state (
  operating_tenant_id, execution_id, execution_stop_id, status, estimated_arrival_at, source_type, provider_code,
  source_observed_at, received_at, freshness_status, quality_status, quality_reasons, age_seconds, planned_arrival, version, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,1,now())
ON CONFLICT (operating_tenant_id, execution_stop_id) DO UPDATE SET
  execution_id = EXCLUDED.execution_id,
  status = EXCLUDED.status,
  estimated_arrival_at = EXCLUDED.estimated_arrival_at,
  source_type = EXCLUDED.source_type,
  provider_code = EXCLUDED.provider_code,
  source_observed_at = EXCLUDED.source_observed_at,
  received_at = EXCLUDED.received_at,
  freshness_status = EXCLUDED.freshness_status,
  quality_status = EXCLUDED.quality_status,
  quality_reasons = EXCLUDED.quality_reasons,
  age_seconds = EXCLUDED.age_seconds,
  planned_arrival = tracking.execution_stop_eta_state.planned_arrival,
  version = tracking.execution_stop_eta_state.version + 1,
  updated_at = now()
`, state.OperatingTenantID, state.ExecutionID, state.ExecutionStopID, state.Status, state.EstimatedArrivalAt,
		state.SourceType, state.ProviderCode, state.SourceObservedAt, state.ReceivedAt, state.FreshnessStatus,
		state.QualityStatus, reasons, state.AgeSeconds, state.PlannedArrival)
	return err
}

func (r *ExecutionTrackingRepository) GetExecutionStopETA(ctx context.Context, operatingTenantID, executionStopID uuid.UUID) (*ExecutionStopETAState, error) {
	const q = `
SELECT operating_tenant_id, execution_id, execution_stop_id, status, estimated_arrival_at, source_type, provider_code,
       source_observed_at, received_at, freshness_status, quality_status, quality_reasons, age_seconds, planned_arrival, version, updated_at
FROM tracking.execution_stop_eta_state
WHERE operating_tenant_id = $1 AND execution_stop_id = $2`
	row := r.pool.QueryRow(ctx, q, operatingTenantID, executionStopID)
	var state ExecutionStopETAState
	var reasons []byte
	err := row.Scan(
		&state.OperatingTenantID, &state.ExecutionID, &state.ExecutionStopID, &state.Status, &state.EstimatedArrivalAt,
		&state.SourceType, &state.ProviderCode, &state.SourceObservedAt, &state.ReceivedAt, &state.FreshnessStatus,
		&state.QualityStatus, &reasons, &state.AgeSeconds, &state.PlannedArrival, &state.Version, &state.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(reasons, &state.QualityReasons)
	return &state, nil
}

func MarkApproachEmitted(ctx context.Context, tx pgx.Tx, executionID, stopID uuid.UUID, at time.Time) (bool, error) {
	tag, err := tx.Exec(ctx, `
UPDATE tracking.execution_tracking_state
SET approach_emitted_stop_id = $2, approach_emitted_at = $3, updated_at = now()
WHERE execution_id = $1
  AND live_eta_stop_id = $2
  AND target_latitude IS NOT NULL
  AND target_longitude IS NOT NULL
  AND (approach_emitted_stop_id IS NULL OR approach_emitted_stop_id <> $2)
`, executionID, stopID, at)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

type TrackingOutboxEvent struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	AggregateType    string
	AggregateID      uuid.UUID
	AggregateVersion int
	EventType        string
	SchemaVersion    int
	SourceEventID    uuid.UUID
	Payload          []byte
	Headers          []byte
	AvailableAt      time.Time
}

var trackingOutboxInsert = insertTrackingOutbox

func SetTrackingOutboxInsertForTest(fn func(context.Context, pgx.Tx, TrackingOutboxEvent) error) func() {
	previous := trackingOutboxInsert
	if fn == nil {
		trackingOutboxInsert = insertTrackingOutbox
	} else {
		trackingOutboxInsert = fn
	}
	return func() { trackingOutboxInsert = previous }
}

func InsertTrackingOutbox(ctx context.Context, tx pgx.Tx, event TrackingOutboxEvent) error {
	return trackingOutboxInsert(ctx, tx, event)
}

func insertTrackingOutbox(ctx context.Context, tx pgx.Tx, event TrackingOutboxEvent) error {
	if event.Headers == nil {
		event.Headers = []byte(`{}`)
	}
	_, err := tx.Exec(ctx, `
INSERT INTO tracking.event_outbox (
  id, tenant_id, aggregate_type, aggregate_id, aggregate_version, event_type, schema_version,
  source_event_id, payload, headers, status, attempts, available_at, created_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10::jsonb,'PENDING',0,$11,now())
`, event.ID, event.TenantID, event.AggregateType, event.AggregateID, event.AggregateVersion, event.EventType,
		event.SchemaVersion, event.SourceEventID, string(event.Payload), string(event.Headers), event.AvailableAt)
	return err
}

func (r *TrackingRepository) ListActiveBindingsByDevice(ctx context.Context, providerCode, deviceID string) ([]DeviceBinding, error) {
	rows, err := r.pool.Query(ctx, `
SELECT tenant_id, shipment_id, vehicle_id, driver_id
FROM tracking.shipment_tracking_binding
WHERE provider_code = $1 AND provider_device_id = $2 AND status = 'active'
`, providerCode, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DeviceBinding
	for rows.Next() {
		var row DeviceBinding
		if err := rows.Scan(&row.TenantID, &row.ShipmentID, &row.VehicleID, &row.DriverID); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

type DeviceBinding struct {
	TenantID   uuid.UUID
	ShipmentID uuid.UUID
	VehicleID  *uuid.UUID
	DriverID   *uuid.UUID
}

func scanExecutionTrackingState(row pgx.Row) (*ExecutionTrackingState, error) {
	var state ExecutionTrackingState
	err := row.Scan(
		&state.OperatingTenantID, &state.ExecutionID, &state.RevisionID, &state.CurrentStopID, &state.CurrentStopOrdinal,
		&state.PlannedArrival, &state.LocationID, &state.TargetLatitude, &state.TargetLongitude, &state.LiveETAStopID, &state.LiveETAOrdinal,
		&state.DriverID, &state.VehicleID, &state.LastExecutionEventSequence, &state.ApproachEmittedStopID, &state.ApproachEmittedAt, &state.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &state, nil
}

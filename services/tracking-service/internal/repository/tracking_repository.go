package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/tracking-service/internal/domain"
)

type TrackingRepository struct {
	pool *pgxpool.Pool
}

func NewTrackingRepository(pool *pgxpool.Pool) *TrackingRepository {
	return &TrackingRepository{pool: pool}
}

func BuildDedupKey(providerCode, deviceID string, recordedAt time.Time, lat, lon float64) string {
	raw := fmt.Sprintf("%s|%s|%s|%.7f|%.7f", providerCode, deviceID, recordedAt.UTC().Format(time.RFC3339Nano), lat, lon)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (r *TrackingRepository) FindActiveBindingByDevice(ctx context.Context, tenantID uuid.UUID, providerCode, deviceID string) (*domain.ShipmentTrackingBinding, error) {
	const q = `
SELECT id, tenant_id, shipment_id, vehicle_id, driver_id, provider_code, provider_device_id,
       status, active_from, active_to, created_at, updated_at
FROM tracking.shipment_tracking_binding
WHERE tenant_id = $1 AND provider_code = $2 AND provider_device_id = $3 AND status = 'active'
ORDER BY active_from DESC
LIMIT 1`
	row := r.pool.QueryRow(ctx, q, tenantID, providerCode, deviceID)
	return scanBinding(row)
}

func (r *TrackingRepository) FindActiveBindingByDeviceAnyTenant(ctx context.Context, providerCode, deviceID string) (*domain.ShipmentTrackingBinding, error) {
	const q = `
SELECT id, tenant_id, shipment_id, vehicle_id, driver_id, provider_code, provider_device_id,
       status, active_from, active_to, created_at, updated_at
FROM tracking.shipment_tracking_binding
WHERE provider_code = $1 AND provider_device_id = $2 AND status = 'active'
ORDER BY active_from DESC
LIMIT 1`
	row := r.pool.QueryRow(ctx, q, providerCode, deviceID)
	return scanBinding(row)
}

func (r *TrackingRepository) HasActiveBinding(ctx context.Context, tenantID, shipmentID uuid.UUID) (bool, error) {
	const q = `
SELECT EXISTS(
  SELECT 1 FROM tracking.shipment_tracking_binding
  WHERE tenant_id = $1 AND shipment_id = $2 AND status = 'active'
)`
	var exists bool
	if err := r.pool.QueryRow(ctx, q, tenantID, shipmentID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (r *TrackingRepository) InsertLocationEvent(ctx context.Context, event domain.LocationEvent) (inserted bool, err error) {
	const q = `
INSERT INTO tracking.location_event (
  id, tenant_id, shipment_id, vehicle_id, driver_id, provider_code, provider_device_id,
  provider_event_id, dedup_key, latitude, longitude, recorded_at, received_at,
  speed_kph, heading_degrees, accuracy_meters, altitude_meters, source_type,
  quality_status, quality_reason
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20
)
ON CONFLICT DO NOTHING
RETURNING id`
	var returned uuid.UUID
	err = r.pool.QueryRow(ctx, q,
		event.ID, event.TenantID, event.ShipmentID, event.VehicleID, event.DriverID,
		event.ProviderCode, event.ProviderDeviceID, event.ProviderEventID, event.DedupKey,
		event.Latitude, event.Longitude, event.RecordedAt, event.ReceivedAt,
		event.SpeedKph, event.HeadingDegrees, event.AccuracyMeters, event.AltitudeMeters,
		event.SourceType, event.QualityStatus, event.QualityReason,
	).Scan(&returned)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *TrackingRepository) GetPreviousLocationEvent(ctx context.Context, tenantID, shipmentID uuid.UUID, before time.Time) (*domain.LocationEvent, error) {
	const q = `
SELECT id, tenant_id, shipment_id, vehicle_id, driver_id, provider_code, provider_device_id,
       provider_event_id, dedup_key, latitude, longitude, recorded_at, received_at,
       speed_kph, heading_degrees, accuracy_meters, altitude_meters, source_type,
       quality_status, quality_reason, created_at
FROM tracking.location_event
WHERE tenant_id = $1 AND shipment_id = $2 AND recorded_at < $3
ORDER BY recorded_at DESC, received_at DESC
LIMIT 1`
	row := r.pool.QueryRow(ctx, q, tenantID, shipmentID, before)
	return scanLocationEvent(row)
}

func (r *TrackingRepository) GetTrackingState(ctx context.Context, tenantID, shipmentID uuid.UUID) (*domain.ShipmentTrackingState, error) {
	const q = `
SELECT tenant_id, shipment_id, tracking_status, provider_code, last_latitude, last_longitude,
       last_recorded_at, last_received_at, last_speed_kph, last_heading_degrees,
       freshness_status, quality_status, age_seconds, delivery_delay_seconds, updated_at
FROM tracking.shipment_tracking_state
WHERE tenant_id = $1 AND shipment_id = $2`
	row := r.pool.QueryRow(ctx, q, tenantID, shipmentID)
	return scanTrackingState(row)
}

func (r *TrackingRepository) UpsertTrackingStateIfNewer(ctx context.Context, state domain.ShipmentTrackingState) error {
	const q = `
INSERT INTO tracking.shipment_tracking_state (
  tenant_id, shipment_id, tracking_status, provider_code, last_latitude, last_longitude,
  last_recorded_at, last_received_at, last_speed_kph, last_heading_degrees,
  freshness_status, quality_status, age_seconds, delivery_delay_seconds, updated_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15
)
ON CONFLICT (tenant_id, shipment_id) DO UPDATE SET
  tracking_status = CASE
    WHEN EXCLUDED.last_recorded_at IS NULL THEN tracking.shipment_tracking_state.tracking_status
    WHEN tracking.shipment_tracking_state.last_recorded_at IS NULL THEN EXCLUDED.tracking_status
    WHEN EXCLUDED.last_recorded_at > tracking.shipment_tracking_state.last_recorded_at THEN EXCLUDED.tracking_status
    WHEN EXCLUDED.last_recorded_at = tracking.shipment_tracking_state.last_recorded_at
         AND EXCLUDED.last_received_at >= tracking.shipment_tracking_state.last_received_at THEN EXCLUDED.tracking_status
    ELSE tracking.shipment_tracking_state.tracking_status
  END,
  provider_code = CASE
    WHEN EXCLUDED.last_recorded_at IS NULL THEN tracking.shipment_tracking_state.provider_code
    WHEN tracking.shipment_tracking_state.last_recorded_at IS NULL THEN EXCLUDED.provider_code
    WHEN EXCLUDED.last_recorded_at > tracking.shipment_tracking_state.last_recorded_at THEN EXCLUDED.provider_code
    WHEN EXCLUDED.last_recorded_at = tracking.shipment_tracking_state.last_recorded_at
         AND EXCLUDED.last_received_at >= tracking.shipment_tracking_state.last_received_at THEN EXCLUDED.provider_code
    ELSE tracking.shipment_tracking_state.provider_code
  END,
  last_latitude = CASE
    WHEN EXCLUDED.last_recorded_at IS NULL THEN tracking.shipment_tracking_state.last_latitude
    WHEN tracking.shipment_tracking_state.last_recorded_at IS NULL THEN EXCLUDED.last_latitude
    WHEN EXCLUDED.last_recorded_at > tracking.shipment_tracking_state.last_recorded_at THEN EXCLUDED.last_latitude
    WHEN EXCLUDED.last_recorded_at = tracking.shipment_tracking_state.last_recorded_at
         AND EXCLUDED.last_received_at >= tracking.shipment_tracking_state.last_received_at THEN EXCLUDED.last_latitude
    ELSE tracking.shipment_tracking_state.last_latitude
  END,
  last_longitude = CASE
    WHEN EXCLUDED.last_recorded_at IS NULL THEN tracking.shipment_tracking_state.last_longitude
    WHEN tracking.shipment_tracking_state.last_recorded_at IS NULL THEN EXCLUDED.last_longitude
    WHEN EXCLUDED.last_recorded_at > tracking.shipment_tracking_state.last_recorded_at THEN EXCLUDED.last_longitude
    WHEN EXCLUDED.last_recorded_at = tracking.shipment_tracking_state.last_recorded_at
         AND EXCLUDED.last_received_at >= tracking.shipment_tracking_state.last_received_at THEN EXCLUDED.last_longitude
    ELSE tracking.shipment_tracking_state.last_longitude
  END,
  last_recorded_at = CASE
    WHEN EXCLUDED.last_recorded_at IS NULL THEN tracking.shipment_tracking_state.last_recorded_at
    WHEN tracking.shipment_tracking_state.last_recorded_at IS NULL THEN EXCLUDED.last_recorded_at
    WHEN EXCLUDED.last_recorded_at > tracking.shipment_tracking_state.last_recorded_at THEN EXCLUDED.last_recorded_at
    WHEN EXCLUDED.last_recorded_at = tracking.shipment_tracking_state.last_recorded_at
         AND EXCLUDED.last_received_at >= tracking.shipment_tracking_state.last_received_at THEN EXCLUDED.last_recorded_at
    ELSE tracking.shipment_tracking_state.last_recorded_at
  END,
  last_received_at = CASE
    WHEN EXCLUDED.last_recorded_at IS NULL THEN tracking.shipment_tracking_state.last_received_at
    WHEN tracking.shipment_tracking_state.last_recorded_at IS NULL THEN EXCLUDED.last_received_at
    WHEN EXCLUDED.last_recorded_at > tracking.shipment_tracking_state.last_recorded_at THEN EXCLUDED.last_received_at
    WHEN EXCLUDED.last_recorded_at = tracking.shipment_tracking_state.last_recorded_at
         AND EXCLUDED.last_received_at >= tracking.shipment_tracking_state.last_received_at THEN EXCLUDED.last_received_at
    ELSE tracking.shipment_tracking_state.last_received_at
  END,
  last_speed_kph = CASE
    WHEN EXCLUDED.last_recorded_at IS NULL THEN tracking.shipment_tracking_state.last_speed_kph
    WHEN tracking.shipment_tracking_state.last_recorded_at IS NULL THEN EXCLUDED.last_speed_kph
    WHEN EXCLUDED.last_recorded_at > tracking.shipment_tracking_state.last_recorded_at THEN EXCLUDED.last_speed_kph
    WHEN EXCLUDED.last_recorded_at = tracking.shipment_tracking_state.last_recorded_at
         AND EXCLUDED.last_received_at >= tracking.shipment_tracking_state.last_received_at THEN EXCLUDED.last_speed_kph
    ELSE tracking.shipment_tracking_state.last_speed_kph
  END,
  last_heading_degrees = CASE
    WHEN EXCLUDED.last_recorded_at IS NULL THEN tracking.shipment_tracking_state.last_heading_degrees
    WHEN tracking.shipment_tracking_state.last_recorded_at IS NULL THEN EXCLUDED.last_heading_degrees
    WHEN EXCLUDED.last_recorded_at > tracking.shipment_tracking_state.last_recorded_at THEN EXCLUDED.last_heading_degrees
    WHEN EXCLUDED.last_recorded_at = tracking.shipment_tracking_state.last_recorded_at
         AND EXCLUDED.last_received_at >= tracking.shipment_tracking_state.last_received_at THEN EXCLUDED.last_heading_degrees
    ELSE tracking.shipment_tracking_state.last_heading_degrees
  END,
  freshness_status = EXCLUDED.freshness_status,
  quality_status = EXCLUDED.quality_status,
  age_seconds = EXCLUDED.age_seconds,
  delivery_delay_seconds = EXCLUDED.delivery_delay_seconds,
  updated_at = EXCLUDED.updated_at`
	_, err := r.pool.Exec(ctx, q,
		state.TenantID, state.ShipmentID, state.TrackingStatus, state.ProviderCode,
		state.LastLatitude, state.LastLongitude, state.LastRecordedAt, state.LastReceivedAt,
		state.LastSpeedKph, state.LastHeadingDegrees, state.FreshnessStatus, state.QualityStatus,
		state.AgeSeconds, state.DeliveryDelaySeconds, state.UpdatedAt,
	)
	return err
}

func (r *TrackingRepository) RefreshTrackingStateComputed(ctx context.Context, tenantID, shipmentID uuid.UUID, state domain.ShipmentTrackingState) error {
	const q = `
INSERT INTO tracking.shipment_tracking_state (
  tenant_id, shipment_id, tracking_status, provider_code, last_latitude, last_longitude,
  last_recorded_at, last_received_at, last_speed_kph, last_heading_degrees,
  freshness_status, quality_status, age_seconds, delivery_delay_seconds, updated_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15
)
ON CONFLICT (tenant_id, shipment_id) DO UPDATE SET
  tracking_status = EXCLUDED.tracking_status,
  freshness_status = EXCLUDED.freshness_status,
  quality_status = EXCLUDED.quality_status,
  age_seconds = EXCLUDED.age_seconds,
  delivery_delay_seconds = EXCLUDED.delivery_delay_seconds,
  updated_at = EXCLUDED.updated_at`
	_, err := r.pool.Exec(ctx, q,
		state.TenantID, state.ShipmentID, state.TrackingStatus, state.ProviderCode,
		state.LastLatitude, state.LastLongitude, state.LastRecordedAt, state.LastReceivedAt,
		state.LastSpeedKph, state.LastHeadingDegrees, state.FreshnessStatus, state.QualityStatus,
		state.AgeSeconds, state.DeliveryDelaySeconds, state.UpdatedAt,
	)
	return err
}

func (r *TrackingRepository) ListLocationHistory(ctx context.Context, tenantID, shipmentID uuid.UUID, from, to *time.Time, limit, offset int) ([]domain.LocationEvent, int, error) {
	args := []any{tenantID, shipmentID}
	where := "tenant_id = $1 AND shipment_id = $2"
	idx := 3
	if from != nil {
		where += fmt.Sprintf(" AND recorded_at >= $%d", idx)
		args = append(args, *from)
		idx++
	}
	if to != nil {
		where += fmt.Sprintf(" AND recorded_at <= $%d", idx)
		args = append(args, *to)
		idx++
	}
	countQ := "SELECT COUNT(*) FROM tracking.location_event WHERE " + where
	var total int
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	listQ := fmt.Sprintf(`
SELECT id, tenant_id, shipment_id, vehicle_id, driver_id, provider_code, provider_device_id,
       provider_event_id, dedup_key, latitude, longitude, recorded_at, received_at,
       speed_kph, heading_degrees, accuracy_meters, altitude_meters, source_type,
       quality_status, quality_reason, created_at
FROM tracking.location_event
WHERE %s
ORDER BY recorded_at DESC, received_at DESC
LIMIT $%d OFFSET $%d`, where, idx, idx+1)
	args = append(args, limit, offset)
	rows, err := r.pool.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]domain.LocationEvent, 0)
	for rows.Next() {
		item, err := scanLocationEvent(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}

func (r *TrackingRepository) BatchActiveBindings(ctx context.Context, tenantID uuid.UUID, shipmentIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	out := make(map[uuid.UUID]bool, len(shipmentIDs))
	if len(shipmentIDs) == 0 {
		return out, nil
	}
	const q = `
SELECT shipment_id
FROM tracking.shipment_tracking_binding
WHERE tenant_id = $1 AND shipment_id = ANY($2) AND status = 'active'`
	rows, err := r.pool.Query(ctx, q, tenantID, shipmentIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var shipmentID uuid.UUID
		if err := rows.Scan(&shipmentID); err != nil {
			return nil, err
		}
		out[shipmentID] = true
	}
	return out, rows.Err()
}

func (r *TrackingRepository) LookupTrackingStates(ctx context.Context, tenantID uuid.UUID, shipmentIDs []uuid.UUID) (map[uuid.UUID]domain.ShipmentTrackingState, error) {
	out := make(map[uuid.UUID]domain.ShipmentTrackingState)
	if len(shipmentIDs) == 0 {
		return out, nil
	}
	const q = `
SELECT tenant_id, shipment_id, tracking_status, provider_code, last_latitude, last_longitude,
       last_recorded_at, last_received_at, last_speed_kph, last_heading_degrees,
       freshness_status, quality_status, age_seconds, delivery_delay_seconds, updated_at
FROM tracking.shipment_tracking_state
WHERE tenant_id = $1 AND shipment_id = ANY($2)`
	rows, err := r.pool.Query(ctx, q, tenantID, shipmentIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		item, err := scanTrackingState(rows)
		if err != nil {
			return nil, err
		}
		out[item.ShipmentID] = *item
	}
	return out, rows.Err()
}

func (r *TrackingRepository) InsertStateTransition(ctx context.Context, transition domain.StateTransition) error {
	const q = `
INSERT INTO tracking.tracking_state_transition (
  id, tenant_id, shipment_id, transition_type, from_status, to_status, metadata, occurred_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	meta := transition.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	_, err := r.pool.Exec(ctx, q,
		transition.ID, transition.TenantID, transition.ShipmentID,
		transition.TransitionType, transition.FromStatus, transition.ToStatus, meta, transition.OccurredAt,
	)
	return err
}

func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

type scannable interface {
	Scan(dest ...any) error
}

func scanBinding(row scannable) (*domain.ShipmentTrackingBinding, error) {
	var b domain.ShipmentTrackingBinding
	err := row.Scan(
		&b.ID, &b.TenantID, &b.ShipmentID, &b.VehicleID, &b.DriverID,
		&b.ProviderCode, &b.ProviderDeviceID, &b.Status, &b.ActiveFrom, &b.ActiveTo,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrorsNotFound()
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func scanLocationEvent(row scannable) (*domain.LocationEvent, error) {
	var e domain.LocationEvent
	err := row.Scan(
		&e.ID, &e.TenantID, &e.ShipmentID, &e.VehicleID, &e.DriverID,
		&e.ProviderCode, &e.ProviderDeviceID, &e.ProviderEventID, &e.DedupKey,
		&e.Latitude, &e.Longitude, &e.RecordedAt, &e.ReceivedAt,
		&e.SpeedKph, &e.HeadingDegrees, &e.AccuracyMeters, &e.AltitudeMeters,
		&e.SourceType, &e.QualityStatus, &e.QualityReason, &e.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrorsNotFound()
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func scanTrackingState(row scannable) (*domain.ShipmentTrackingState, error) {
	var s domain.ShipmentTrackingState
	err := row.Scan(
		&s.TenantID, &s.ShipmentID, &s.TrackingStatus, &s.ProviderCode,
		&s.LastLatitude, &s.LastLongitude, &s.LastRecordedAt, &s.LastReceivedAt,
		&s.LastSpeedKph, &s.LastHeadingDegrees, &s.FreshnessStatus, &s.QualityStatus,
		&s.AgeSeconds, &s.DeliveryDelaySeconds, &s.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrorsNotFound()
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func apperrorsNotFound() error {
	return pgx.ErrNoRows
}

func ParseUUID(raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.Nil, fmt.Errorf("empty uuid")
	}
	return uuid.Parse(raw)
}

func (r *TrackingRepository) EnsureActiveDriverMobileBinding(
	ctx context.Context,
	tenantID, shipmentID, driverID uuid.UUID,
	vehicleID *uuid.UUID,
) error {
	deviceID := driverID.String()
	existing, err := r.FindActiveBindingByDevice(ctx, tenantID, "driver_mobile", deviceID)
	if err == nil && existing != nil {
		if existing.ShipmentID == shipmentID {
			return nil
		}
		return fmt.Errorf("device bound to another shipment")
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	const q = `
INSERT INTO tracking.shipment_tracking_binding (
	tenant_id, shipment_id, vehicle_id, driver_id, provider_code, provider_device_id, status
) VALUES ($1,$2,$3,$4,'driver_mobile',$5,'active')
ON CONFLICT DO NOTHING`
	_, err = r.pool.Exec(ctx, q, tenantID, shipmentID, vehicleID, driverID, deviceID)
	return err
}

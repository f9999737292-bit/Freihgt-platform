package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	TrackingAutomationOK   = "TRACKING_OK"
	TrackingAutomationLost = "TRACKING_LOST"
)

type TrackingAutomationCandidate struct {
	TenantID              uuid.UUID
	ShipmentID            uuid.UUID
	DriverID              *uuid.UUID
	ShipmentVersion       int
	LastLocationRecorded  *time.Time
	AutomationState       string
	TrackingStatus        string
}

func (r *TrackingRepository) ListTrackingAutomationCandidates(ctx context.Context, limit int) ([]TrackingAutomationCandidate, error) {
	if limit <= 0 {
		limit = 100
	}
	const q = `
SELECT s.tenant_id, s.shipment_id, b.driver_id, sh.version,
       s.last_recorded_at, COALESCE(a.automation_state, 'TRACKING_OK'), s.tracking_status
FROM tracking.shipment_tracking_state s
JOIN tracking.shipment_tracking_binding b
  ON b.tenant_id = s.tenant_id AND b.shipment_id = s.shipment_id AND b.status = 'active'
JOIN transport.shipments sh
  ON sh.id = s.shipment_id AND sh.tenant_id = s.tenant_id
LEFT JOIN tracking.driver_tracking_automation_state a
  ON a.tenant_id = s.tenant_id AND a.shipment_id = s.shipment_id
WHERE s.tracking_status NOT IN ('ended')
ORDER BY s.updated_at ASC
LIMIT $1`
	rows, err := r.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]TrackingAutomationCandidate, 0)
	for rows.Next() {
		var item TrackingAutomationCandidate
		if err := rows.Scan(
			&item.TenantID, &item.ShipmentID, &item.DriverID, &item.ShipmentVersion,
			&item.LastLocationRecorded, &item.AutomationState, &item.TrackingStatus,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *TrackingRepository) UpsertTrackingAutomationState(
	ctx context.Context,
	tenantID, shipmentID uuid.UUID,
	state string,
	lastLocationRecorded *time.Time,
) error {
	const q = `
INSERT INTO tracking.driver_tracking_automation_state (
	tenant_id, shipment_id, automation_state, last_location_recorded_at, last_transition_at, updated_at
) VALUES ($1,$2,$3,$4,NOW(),NOW())
ON CONFLICT (tenant_id, shipment_id) DO UPDATE SET
	automation_state = EXCLUDED.automation_state,
	last_location_recorded_at = COALESCE(EXCLUDED.last_location_recorded_at, tracking.driver_tracking_automation_state.last_location_recorded_at),
	last_transition_at = CASE
		WHEN tracking.driver_tracking_automation_state.automation_state = EXCLUDED.automation_state
		THEN tracking.driver_tracking_automation_state.last_transition_at
		ELSE NOW()
	END,
	updated_at = NOW()`
	_, err := r.pool.Exec(ctx, q, tenantID, shipmentID, state, lastLocationRecorded)
	return err
}

func (r *TrackingRepository) GetTrackingAutomationState(ctx context.Context, tenantID, shipmentID uuid.UUID) (string, error) {
	const q = `SELECT automation_state FROM tracking.driver_tracking_automation_state WHERE tenant_id=$1 AND shipment_id=$2`
	var state string
	err := r.pool.QueryRow(ctx, q, tenantID, shipmentID).Scan(&state)
	if err == pgx.ErrNoRows {
		return TrackingAutomationOK, nil
	}
	return state, err
}

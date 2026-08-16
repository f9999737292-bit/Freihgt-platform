package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/tracking-service/internal/domain"
)

type SlotRepository struct {
	pool *pgxpool.Pool
}

func NewSlotRepository(pool *pgxpool.Pool) *SlotRepository {
	return &SlotRepository{pool: pool}
}

func BuildSlotDedupKey(providerCode, slotType string, shipmentID uuid.UUID, windowStart, windowEnd, observedAt time.Time, providerSlotID, providerVersion *string) string {
	slotPart := ""
	if providerSlotID != nil {
		slotPart = *providerSlotID
	}
	versionPart := ""
	if providerVersion != nil {
		versionPart = *providerVersion
	}
	raw := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s",
		providerCode, slotType, shipmentID.String(),
		windowStart.UTC().Format(time.RFC3339Nano),
		windowEnd.UTC().Format(time.RFC3339Nano),
		observedAt.UTC().Format(time.RFC3339Nano),
		slotPart, versionPart,
	)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (r *SlotRepository) InsertSlotRevision(ctx context.Context, rev domain.SlotRevision) (inserted bool, err error) {
	reasonsJSON, _ := json.Marshal(rev.QualityReasons)
	const q = `
INSERT INTO tracking.shipment_slot_revision (
  id, tenant_id, shipment_id, slot_type, facility_id, location_id,
  window_start, window_end, timezone, slot_status, source_type, provider_code,
  provider_slot_id, provider_version, dedup_key, source_observed_at, received_at,
  quality_status, quality_reasons, booked_at, confirmed_at, cancelled_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)
ON CONFLICT DO NOTHING
RETURNING id`
	var returned uuid.UUID
	err = r.pool.QueryRow(ctx, q,
		rev.ID, rev.TenantID, rev.ShipmentID, rev.SlotType, rev.FacilityID, rev.LocationID,
		rev.WindowStart, rev.WindowEnd, rev.Timezone, rev.SlotStatus, rev.SourceType, rev.ProviderCode,
		rev.ProviderSlotID, rev.ProviderVersion, rev.DedupKey, rev.SourceObservedAt, rev.ReceivedAt,
		rev.QualityStatus, reasonsJSON, rev.BookedAt, rev.ConfirmedAt, rev.CancelledAt,
	).Scan(&returned)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *SlotRepository) GetSlotState(ctx context.Context, tenantID, shipmentID uuid.UUID, slotType string) (*domain.ShipmentSlotState, error) {
	const q = `
SELECT tenant_id, shipment_id, slot_type, window_status, slot_status, window_start, window_end, timezone,
       facility_id, location_id, source_type, provider_code, provider_slot_id, source_observed_at, received_at,
       quality_status, booked_at, confirmed_at, version, updated_at
FROM tracking.shipment_slot_state
WHERE tenant_id = $1 AND shipment_id = $2 AND slot_type = $3`
	row := r.pool.QueryRow(ctx, q, tenantID, shipmentID, slotType)
	return scanSlotState(row)
}

func (r *SlotRepository) UpsertSlotStateIfNewer(ctx context.Context, state domain.ShipmentSlotState, replace bool) error {
	if !replace {
		return nil
	}
	const q = `
INSERT INTO tracking.shipment_slot_state (
  tenant_id, shipment_id, slot_type, window_status, slot_status, window_start, window_end, timezone,
  facility_id, location_id, source_type, provider_code, provider_slot_id, source_observed_at, received_at,
  quality_status, booked_at, confirmed_at, version, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,1,$19)
ON CONFLICT (tenant_id, shipment_id, slot_type) DO UPDATE SET
  window_status = EXCLUDED.window_status,
  slot_status = EXCLUDED.slot_status,
  window_start = EXCLUDED.window_start,
  window_end = EXCLUDED.window_end,
  timezone = EXCLUDED.timezone,
  facility_id = EXCLUDED.facility_id,
  location_id = EXCLUDED.location_id,
  source_type = EXCLUDED.source_type,
  provider_code = EXCLUDED.provider_code,
  provider_slot_id = EXCLUDED.provider_slot_id,
  source_observed_at = EXCLUDED.source_observed_at,
  received_at = EXCLUDED.received_at,
  quality_status = EXCLUDED.quality_status,
  booked_at = EXCLUDED.booked_at,
  confirmed_at = EXCLUDED.confirmed_at,
  version = tracking.shipment_slot_state.version + 1,
  updated_at = EXCLUDED.updated_at`
	_, err := r.pool.Exec(ctx, q,
		state.TenantID, state.ShipmentID, state.SlotType, state.WindowStatus, state.SlotStatus,
		state.WindowStart, state.WindowEnd, state.Timezone, state.FacilityID, state.LocationID,
		state.SourceType, state.ProviderCode, state.ProviderSlotID, state.SourceObservedAt, state.ReceivedAt,
		state.QualityStatus, state.BookedAt, state.ConfirmedAt, state.UpdatedAt,
	)
	return err
}

func (r *SlotRepository) LookupSlotStates(ctx context.Context, tenantID uuid.UUID, shipmentIDs []uuid.UUID, slotType string) (map[uuid.UUID]domain.ShipmentSlotState, error) {
	out := make(map[uuid.UUID]domain.ShipmentSlotState)
	if len(shipmentIDs) == 0 {
		return out, nil
	}
	const q = `
SELECT tenant_id, shipment_id, slot_type, window_status, slot_status, window_start, window_end, timezone,
       facility_id, location_id, source_type, provider_code, provider_slot_id, source_observed_at, received_at,
       quality_status, booked_at, confirmed_at, version, updated_at
FROM tracking.shipment_slot_state
WHERE tenant_id = $1 AND shipment_id = ANY($2) AND slot_type = $3`
	rows, err := r.pool.Query(ctx, q, tenantID, shipmentIDs, slotType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		item, err := scanSlotState(rows)
		if err != nil {
			return nil, err
		}
		out[item.ShipmentID] = *item
	}
	return out, nil
}

func (r *SlotRepository) ListSlotHistory(ctx context.Context, tenantID, shipmentID uuid.UUID, slotType string, from, to *time.Time, limit, offset int) ([]domain.SlotRevision, int, error) {
	args := []any{tenantID, shipmentID, slotType}
	where := `tenant_id = $1 AND shipment_id = $2 AND slot_type = $3`
	argN := 4
	if from != nil {
		where += fmt.Sprintf(" AND source_observed_at >= $%d", argN)
		args = append(args, *from)
		argN++
	}
	if to != nil {
		where += fmt.Sprintf(" AND source_observed_at <= $%d", argN)
		args = append(args, *to)
		argN++
	}
	countQ := `SELECT COUNT(*) FROM tracking.shipment_slot_revision WHERE ` + where
	var total int
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	listQ := fmt.Sprintf(`
SELECT id, tenant_id, shipment_id, slot_type, facility_id, location_id,
       window_start, window_end, timezone, slot_status, source_type, provider_code,
       provider_slot_id, provider_version, dedup_key, source_observed_at, received_at,
       quality_status, quality_reasons, booked_at, confirmed_at, cancelled_at, created_at
FROM tracking.shipment_slot_revision
WHERE %s
ORDER BY source_observed_at DESC
LIMIT $%d OFFSET $%d`, where, argN, argN+1)
	args = append(args, limit, offset)
	rows, err := r.pool.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]domain.SlotRevision, 0)
	for rows.Next() {
		item, err := scanSlotRevision(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, nil
}

func (r *SlotRepository) InsertSlotTransition(ctx context.Context, t domain.SlotStateTransition) error {
	metaJSON, _ := json.Marshal(t.Metadata)
	const q = `
INSERT INTO tracking.slot_state_transition (
  id, tenant_id, shipment_id, slot_type, transition_type, from_status, to_status, metadata, occurred_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	_, err := r.pool.Exec(ctx, q,
		t.ID, t.TenantID, t.ShipmentID, t.SlotType, t.TransitionType,
		t.FromStatus, t.ToStatus, metaJSON, t.OccurredAt,
	)
	return err
}

func scanSlotState(row scannable) (*domain.ShipmentSlotState, error) {
	var s domain.ShipmentSlotState
	err := row.Scan(
		&s.TenantID, &s.ShipmentID, &s.SlotType, &s.WindowStatus, &s.SlotStatus,
		&s.WindowStart, &s.WindowEnd, &s.Timezone, &s.FacilityID, &s.LocationID,
		&s.SourceType, &s.ProviderCode, &s.ProviderSlotID, &s.SourceObservedAt, &s.ReceivedAt,
		&s.QualityStatus, &s.BookedAt, &s.ConfirmedAt, &s.Version, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func scanSlotRevision(row scannable) (*domain.SlotRevision, error) {
	var rev domain.SlotRevision
	var reasonsJSON []byte
	err := row.Scan(
		&rev.ID, &rev.TenantID, &rev.ShipmentID, &rev.SlotType, &rev.FacilityID, &rev.LocationID,
		&rev.WindowStart, &rev.WindowEnd, &rev.Timezone, &rev.SlotStatus, &rev.SourceType, &rev.ProviderCode,
		&rev.ProviderSlotID, &rev.ProviderVersion, &rev.DedupKey, &rev.SourceObservedAt, &rev.ReceivedAt,
		&rev.QualityStatus, &reasonsJSON, &rev.BookedAt, &rev.ConfirmedAt, &rev.CancelledAt, &rev.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(reasonsJSON) > 0 {
		_ = json.Unmarshal(reasonsJSON, &rev.QualityReasons)
	}
	return &rev, nil
}

func ParseSlotType(raw string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case domain.SlotTypePickup, domain.SlotTypeDelivery:
		return strings.TrimSpace(strings.ToLower(raw)), nil
	default:
		return "", fmt.Errorf("invalid slot type")
	}
}

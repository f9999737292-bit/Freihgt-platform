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

type ETARepository struct {
	pool *pgxpool.Pool
}

func NewETARepository(pool *pgxpool.Pool) *ETARepository {
	return &ETARepository{pool: pool}
}

func BuildETADedupKey(providerCode, targetType string, shipmentID uuid.UUID, estimatedAt, observedAt time.Time, providerEventID *string) string {
	eventPart := ""
	if providerEventID != nil {
		eventPart = *providerEventID
	}
	raw := fmt.Sprintf("%s|%s|%s|%s|%s|%s", providerCode, targetType, shipmentID.String(), estimatedAt.UTC().Format(time.RFC3339Nano), observedAt.UTC().Format(time.RFC3339Nano), eventPart)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (r *ETARepository) InsertETAObservation(ctx context.Context, obs domain.ETAObservation) (inserted bool, err error) {
	reasonsJSON, _ := json.Marshal(obs.QualityReasons)
	const q = `
INSERT INTO tracking.eta_observation (
  id, tenant_id, shipment_id, target_type, target_reference, estimated_arrival_at,
  source_type, provider_code, provider_event_id, dedup_key, source_observed_at, received_at,
  quality_status, quality_reasons, provider_confidence
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
ON CONFLICT DO NOTHING
RETURNING id`
	var returned uuid.UUID
	err = r.pool.QueryRow(ctx, q,
		obs.ID, obs.TenantID, obs.ShipmentID, obs.TargetType, obs.TargetReference,
		obs.EstimatedArrivalAt, obs.SourceType, obs.ProviderCode, obs.ProviderEventID, obs.DedupKey,
		obs.SourceObservedAt, obs.ReceivedAt, obs.QualityStatus, reasonsJSON, obs.ProviderConfidence,
	).Scan(&returned)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *ETARepository) GetETAState(ctx context.Context, tenantID, shipmentID uuid.UUID, targetType string) (*domain.ShipmentETAState, error) {
	const q = `
SELECT tenant_id, shipment_id, target_type, status, estimated_arrival_at, source_type, provider_code,
       source_observed_at, received_at, freshness_status, quality_status, age_seconds, delivery_lag_seconds,
       version, updated_at
FROM tracking.shipment_eta_state
WHERE tenant_id = $1 AND shipment_id = $2 AND target_type = $3`
	row := r.pool.QueryRow(ctx, q, tenantID, shipmentID, targetType)
	return scanETAState(row)
}

func (r *ETARepository) UpsertETAStateIfNewer(ctx context.Context, state domain.ShipmentETAState, replace bool) error {
	if !replace {
		return nil
	}
	const q = `
INSERT INTO tracking.shipment_eta_state (
  tenant_id, shipment_id, target_type, status, estimated_arrival_at, source_type, provider_code,
  source_observed_at, received_at, freshness_status, quality_status, age_seconds, delivery_lag_seconds,
  version, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,1,$14)
ON CONFLICT (tenant_id, shipment_id, target_type) DO UPDATE SET
  status = EXCLUDED.status,
  estimated_arrival_at = EXCLUDED.estimated_arrival_at,
  source_type = EXCLUDED.source_type,
  provider_code = EXCLUDED.provider_code,
  source_observed_at = EXCLUDED.source_observed_at,
  received_at = EXCLUDED.received_at,
  freshness_status = EXCLUDED.freshness_status,
  quality_status = EXCLUDED.quality_status,
  age_seconds = EXCLUDED.age_seconds,
  delivery_lag_seconds = EXCLUDED.delivery_lag_seconds,
  version = tracking.shipment_eta_state.version + 1,
  updated_at = EXCLUDED.updated_at`
	_, err := r.pool.Exec(ctx, q,
		state.TenantID, state.ShipmentID, state.TargetType, state.Status,
		state.EstimatedArrivalAt, state.SourceType, state.ProviderCode,
		state.SourceObservedAt, state.ReceivedAt, state.FreshnessStatus, state.QualityStatus,
		state.AgeSeconds, state.DeliveryLagSeconds, state.UpdatedAt,
	)
	return err
}

func (r *ETARepository) RefreshETAStateComputed(ctx context.Context, state domain.ShipmentETAState) error {
	const q = `
INSERT INTO tracking.shipment_eta_state (
  tenant_id, shipment_id, target_type, status, estimated_arrival_at, source_type, provider_code,
  source_observed_at, received_at, freshness_status, quality_status, age_seconds, delivery_lag_seconds,
  version, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,1,$14)
ON CONFLICT (tenant_id, shipment_id, target_type) DO UPDATE SET
  status = EXCLUDED.status,
  freshness_status = EXCLUDED.freshness_status,
  quality_status = EXCLUDED.quality_status,
  age_seconds = EXCLUDED.age_seconds,
  delivery_lag_seconds = EXCLUDED.delivery_lag_seconds,
  updated_at = EXCLUDED.updated_at`
	_, err := r.pool.Exec(ctx, q,
		state.TenantID, state.ShipmentID, state.TargetType, state.Status,
		state.EstimatedArrivalAt, state.SourceType, state.ProviderCode,
		state.SourceObservedAt, state.ReceivedAt, state.FreshnessStatus, state.QualityStatus,
		state.AgeSeconds, state.DeliveryLagSeconds, state.UpdatedAt,
	)
	return err
}

func (r *ETARepository) LookupETAStates(ctx context.Context, tenantID uuid.UUID, shipmentIDs []uuid.UUID, targetType string) (map[uuid.UUID]domain.ShipmentETAState, error) {
	out := make(map[uuid.UUID]domain.ShipmentETAState)
	if len(shipmentIDs) == 0 {
		return out, nil
	}
	const q = `
SELECT tenant_id, shipment_id, target_type, status, estimated_arrival_at, source_type, provider_code,
       source_observed_at, received_at, freshness_status, quality_status, age_seconds, delivery_lag_seconds,
       version, updated_at
FROM tracking.shipment_eta_state
WHERE tenant_id = $1 AND shipment_id = ANY($2) AND target_type = $3`
	rows, err := r.pool.Query(ctx, q, tenantID, shipmentIDs, targetType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		item, err := scanETAState(rows)
		if err != nil {
			return nil, err
		}
		out[item.ShipmentID] = *item
	}
	return out, rows.Err()
}

func (r *ETARepository) ListETAHistory(ctx context.Context, tenantID, shipmentID uuid.UUID, targetType string, from, to *time.Time, limit, offset int) ([]domain.ETAObservation, int, error) {
	args := []any{tenantID, shipmentID, targetType}
	where := "tenant_id = $1 AND shipment_id = $2 AND target_type = $3"
	idx := 4
	if from != nil {
		where += fmt.Sprintf(" AND source_observed_at >= $%d", idx)
		args = append(args, *from)
		idx++
	}
	if to != nil {
		where += fmt.Sprintf(" AND source_observed_at <= $%d", idx)
		args = append(args, *to)
		idx++
	}
	countQ := "SELECT COUNT(*) FROM tracking.eta_observation WHERE " + where
	var total int
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	listQ := fmt.Sprintf(`
SELECT id, tenant_id, shipment_id, target_type, target_reference, estimated_arrival_at,
       source_type, provider_code, provider_event_id, dedup_key, source_observed_at, received_at,
       quality_status, quality_reasons, provider_confidence, created_at
FROM tracking.eta_observation
WHERE %s
ORDER BY source_observed_at DESC, received_at DESC
LIMIT $%d OFFSET $%d`, where, idx, idx+1)
	args = append(args, limit, offset)
	rows, err := r.pool.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]domain.ETAObservation, 0)
	for rows.Next() {
		item, err := scanETAObservation(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}

func (r *ETARepository) InsertETATransition(ctx context.Context, t domain.ETAStateTransition) error {
	const q = `
INSERT INTO tracking.eta_state_transition (
  id, tenant_id, shipment_id, target_type, transition_type, from_status, to_status, metadata, occurred_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	meta := t.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	_, err := r.pool.Exec(ctx, q, t.ID, t.TenantID, t.ShipmentID, t.TargetType, t.TransitionType, t.FromStatus, t.ToStatus, meta, t.OccurredAt)
	return err
}

type etaScannable interface {
	Scan(dest ...any) error
}

func scanETAState(row etaScannable) (*domain.ShipmentETAState, error) {
	var s domain.ShipmentETAState
	err := row.Scan(
		&s.TenantID, &s.ShipmentID, &s.TargetType, &s.Status, &s.EstimatedArrivalAt,
		&s.SourceType, &s.ProviderCode, &s.SourceObservedAt, &s.ReceivedAt,
		&s.FreshnessStatus, &s.QualityStatus, &s.AgeSeconds, &s.DeliveryLagSeconds,
		&s.Version, &s.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, pgx.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func scanETAObservation(row etaScannable) (*domain.ETAObservation, error) {
	var o domain.ETAObservation
	var reasonsJSON []byte
	err := row.Scan(
		&o.ID, &o.TenantID, &o.ShipmentID, &o.TargetType, &o.TargetReference, &o.EstimatedArrivalAt,
		&o.SourceType, &o.ProviderCode, &o.ProviderEventID, &o.DedupKey,
		&o.SourceObservedAt, &o.ReceivedAt, &o.QualityStatus, &reasonsJSON, &o.ProviderConfidence, &o.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(reasonsJSON) > 0 {
		_ = json.Unmarshal(reasonsJSON, &o.QualityReasons)
	}
	return &o, nil
}

func ShouldReplaceETAObservation(currentSource, incomingSource string, currentObserved, incomingObserved, currentReceived, incomingReceived time.Time) bool {
	inPriority := domain.SourcePriority(incomingSource)
	curPriority := domain.SourcePriority(currentSource)
	if inPriority > curPriority {
		return true
	}
	if inPriority < curPriority {
		return false
	}
	if incomingObserved.After(currentObserved) {
		return true
	}
	if incomingObserved.Equal(currentObserved) && !incomingReceived.Before(currentReceived) {
		return true
	}
	return false
}

func ParseTargetType(raw string) (string, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	switch raw {
	case domain.TargetPickup, domain.TargetDelivery:
		return raw, nil
	default:
		return "", fmt.Errorf("invalid target type")
	}
}

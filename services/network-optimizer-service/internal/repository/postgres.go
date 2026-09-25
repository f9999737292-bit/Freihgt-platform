package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
)

const loadColumns = `
id, owner_tenant_id, source_type, source_id,
pickup_location_id, pickup_label, pickup_latitude, pickup_longitude,
pickup_window_start, pickup_window_end,
delivery_location_id, delivery_label, delivery_latitude, delivery_longitude,
delivery_window_start, delivery_window_end,
weight_kg, volume_m3, body_type, equipment, cargo,
commercial_mode, commercial_amount, commercial_currency,
visibility_scope, invited_carrier_company_ids,
status, version, created_at, updated_at,
pickup_country_code, pickup_region, pickup_city,
delivery_country_code, delivery_region, delivery_city`

const capacityColumns = `
id, owner_tenant_id, carrier_company_id, vehicle_id,
location_label, latitude, longitude,
available_from, available_until, source, body_type, equipment,
payload_remaining_kg, volume_remaining_m3,
visibility_scope, audience_tenant_ids,
status, version, created_at, updated_at,
location_id, country_code, region, city`

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

func (p *Postgres) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

func (p *Postgres) Within(ctx context.Context, fn func(Tx) error) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(&pgTx{tx: tx}); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (p *Postgres) ListOutbox(ctx context.Context) ([]OutboxEvent, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, event_name, schema_version, tenant_id, aggregate_id, aggregate_version, occurred_at, payload
		FROM network_optimizer.outbox ORDER BY occurred_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OutboxEvent
	for rows.Next() {
		var event OutboxEvent
		if err := rows.Scan(&event.ID, &event.EventName, &event.SchemaVersion, &event.TenantID, &event.AggregateID, &event.AggregateVersion, &event.OccurredAt, &event.Payload); err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, rows.Err()
}

func (p *Postgres) ListAudit(ctx context.Context) ([]AuditEvent, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, actor_user_id, tenant_id, aggregate_type, aggregate_id, action, from_status, to_status,
		       visibility_scope, source_type, source_id, occurred_at
		FROM network_optimizer.audit_events ORDER BY occurred_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEvent
	for rows.Next() {
		var event AuditEvent
		if err := rows.Scan(&event.ID, &event.ActorUserID, &event.TenantID, &event.AggregateType, &event.AggregateID, &event.Action, &event.FromStatus, &event.ToStatus, &event.VisibilityScope, &event.SourceType, &event.SourceID, &event.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, rows.Err()
}

type pgTx struct{ tx pgx.Tx }

func (t *pgTx) InsertLoad(ctx context.Context, load domain.LoadOpportunity) error {
	cargo, err := json.Marshal(load.Cargo)
	if err != nil {
		return err
	}
	_, err = t.tx.Exec(ctx, `
		INSERT INTO network_optimizer.load_opportunities (`+loadColumns+`)
		VALUES (
			$1,$2,$3,$4,
			$5,$6,$7,$8,$9,$10,
			$11,$12,$13,$14,$15,$16,
			$17,$18,$19,$20,$21,
			$22,$23,$24,
			$25,$26,
			$27,$28,$29,$30,
			$31,$32,$33,$34,$35,$36
		)`,
		load.ID, load.OwnerTenantID, load.SourceType, load.SourceID,
		load.Pickup.LocationID, load.Pickup.Label, load.Pickup.Latitude, load.Pickup.Longitude, load.PickupWindow.Start, load.PickupWindow.End,
		load.Delivery.LocationID, load.Delivery.Label, load.Delivery.Latitude, load.Delivery.Longitude, load.DeliveryWindow.Start, load.DeliveryWindow.End,
		load.WeightKg, load.VolumeM3, load.BodyType, emptyStrings(load.Equipment), cargo,
		load.Commercial.Mode, load.Commercial.Amount, load.Commercial.Currency,
		load.VisibilityScope, emptyUUIDs(load.InvitedCarrierCompanyIDs),
		load.Status, load.Version, load.CreatedAt, load.UpdatedAt,
		nullString(load.Pickup.CountryCode), nullString(load.Pickup.Region), nullString(load.Pickup.City),
		nullString(load.Delivery.CountryCode), nullString(load.Delivery.Region), nullString(load.Delivery.City),
	)
	if isConstraint(err, "load_opportunities_active_source_uidx") {
		return ErrDuplicateSource
	}
	return err
}

func (t *pgTx) UpdateLoad(ctx context.Context, load domain.LoadOpportunity, expected int) error {
	cargo, err := json.Marshal(load.Cargo)
	if err != nil {
		return err
	}
	tag, err := t.tx.Exec(ctx, `
		UPDATE network_optimizer.load_opportunities SET
			pickup_location_id=$3, pickup_label=$4, pickup_latitude=$5, pickup_longitude=$6,
			pickup_window_start=$7, pickup_window_end=$8,
			delivery_location_id=$9, delivery_label=$10, delivery_latitude=$11, delivery_longitude=$12,
			delivery_window_start=$13, delivery_window_end=$14,
			weight_kg=$15, volume_m3=$16, body_type=$17, equipment=$18, cargo=$19,
			commercial_mode=$20, commercial_amount=$21, commercial_currency=$22,
			visibility_scope=$23, invited_carrier_company_ids=$24,
			status=$25, version=$26, updated_at=$27,
			pickup_country_code=$28, pickup_region=$29, pickup_city=$30,
			delivery_country_code=$31, delivery_region=$32, delivery_city=$33
		WHERE id=$1 AND version=$2`,
		load.ID, expected,
		load.Pickup.LocationID, load.Pickup.Label, load.Pickup.Latitude, load.Pickup.Longitude, load.PickupWindow.Start, load.PickupWindow.End,
		load.Delivery.LocationID, load.Delivery.Label, load.Delivery.Latitude, load.Delivery.Longitude, load.DeliveryWindow.Start, load.DeliveryWindow.End,
		load.WeightKg, load.VolumeM3, load.BodyType, emptyStrings(load.Equipment), cargo,
		load.Commercial.Mode, load.Commercial.Amount, load.Commercial.Currency,
		load.VisibilityScope, emptyUUIDs(load.InvitedCarrierCompanyIDs),
		load.Status, load.Version, load.UpdatedAt,
		nullString(load.Pickup.CountryCode), nullString(load.Pickup.Region), nullString(load.Pickup.City),
		nullString(load.Delivery.CountryCode), nullString(load.Delivery.Region), nullString(load.Delivery.City),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return versionMiss(ctx, t.tx, "network_optimizer.load_opportunities", load.ID)
	}
	return nil
}

func (t *pgTx) GetLoad(ctx context.Context, id uuid.UUID) (domain.LoadOpportunity, error) {
	row := t.tx.QueryRow(ctx, `SELECT `+loadColumns+` FROM network_optimizer.load_opportunities WHERE id=$1`, id)
	return scanLoad(row)
}

func (t *pgTx) ListOwnLoads(ctx context.Context, tenant uuid.UUID, limit, offset int) ([]domain.LoadOpportunity, error) {
	rows, err := t.tx.Query(ctx, `SELECT `+loadColumns+` FROM network_optimizer.load_opportunities WHERE owner_tenant_id=$1 ORDER BY created_at DESC, id LIMIT $2 OFFSET $3`, tenant, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLoads(rows)
}

func (t *pgTx) ListMarketplaceLoads(ctx context.Context, viewer uuid.UUID, company *uuid.UUID, limit, offset int) ([]domain.LoadOpportunity, error) {
	rows, err := t.tx.Query(ctx, `
		SELECT `+loadColumns+` FROM network_optimizer.load_opportunities
		WHERE status='PUBLISHED' AND owner_tenant_id <> $1 AND (
			visibility_scope IN ('MARKETPLACE', 'ANONYMIZED_MARKETPLACE')
			OR (visibility_scope='INVITED_CARRIERS' AND $2::uuid IS NOT NULL AND $2 = ANY(invited_carrier_company_ids))
		)
		ORDER BY created_at DESC, id LIMIT $3 OFFSET $4`, viewer, company, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLoads(rows)
}

func (t *pgTx) ActiveLoadBySource(ctx context.Context, tenant uuid.UUID, sourceType string, sourceID uuid.UUID) (domain.LoadOpportunity, error) {
	row := t.tx.QueryRow(ctx, `SELECT `+loadColumns+` FROM network_optimizer.load_opportunities WHERE owner_tenant_id=$1 AND source_type=$2 AND source_id=$3 AND status IN ('DRAFT','PUBLISHED')`, tenant, sourceType, sourceID)
	return scanLoad(row)
}

func (t *pgTx) InsertCapacity(ctx context.Context, cap domain.Capacity) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO network_optimizer.capacities (`+capacityColumns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24)`,
		cap.ID, cap.OwnerTenantID, cap.CarrierCompanyID, cap.VehicleID,
		cap.LocationLabel, cap.Latitude, cap.Longitude,
		cap.AvailableFrom, cap.AvailableUntil, cap.Source, cap.BodyType, emptyStrings(cap.Equipment),
		cap.PayloadRemainingKg, cap.VolumeRemainingM3,
		cap.VisibilityScope, emptyUUIDs(cap.AudienceTenantIDs),
		cap.Status, cap.Version, cap.CreatedAt, cap.UpdatedAt,
		cap.LocationID, nullString(cap.CountryCode), nullString(cap.Region), nullString(cap.City),
	)
	return err
}

func (t *pgTx) UpdateCapacity(ctx context.Context, cap domain.Capacity, expected int) error {
	tag, err := t.tx.Exec(ctx, `
		UPDATE network_optimizer.capacities SET
			carrier_company_id=$3, vehicle_id=$4, location_label=$5, latitude=$6, longitude=$7,
			available_from=$8, available_until=$9, body_type=$10, equipment=$11,
			payload_remaining_kg=$12, volume_remaining_m3=$13,
			visibility_scope=$14, audience_tenant_ids=$15,
			status=$16, version=$17, updated_at=$18,
			location_id=$19, country_code=$20, region=$21, city=$22
		WHERE id=$1 AND version=$2`,
		cap.ID, expected,
		cap.CarrierCompanyID, cap.VehicleID, cap.LocationLabel, cap.Latitude, cap.Longitude,
		cap.AvailableFrom, cap.AvailableUntil, cap.BodyType, emptyStrings(cap.Equipment),
		cap.PayloadRemainingKg, cap.VolumeRemainingM3,
		cap.VisibilityScope, emptyUUIDs(cap.AudienceTenantIDs),
		cap.Status, cap.Version, cap.UpdatedAt,
		cap.LocationID, nullString(cap.CountryCode), nullString(cap.Region), nullString(cap.City),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return versionMiss(ctx, t.tx, "network_optimizer.capacities", cap.ID)
	}
	return nil
}

func (t *pgTx) GetCapacity(ctx context.Context, id uuid.UUID) (domain.Capacity, error) {
	row := t.tx.QueryRow(ctx, `SELECT `+capacityColumns+` FROM network_optimizer.capacities WHERE id=$1`, id)
	return scanCapacity(row)
}

func (t *pgTx) ListOwnCapacities(ctx context.Context, tenant uuid.UUID, limit, offset int) ([]domain.Capacity, error) {
	rows, err := t.tx.Query(ctx, `SELECT `+capacityColumns+` FROM network_optimizer.capacities WHERE owner_tenant_id=$1 ORDER BY created_at DESC, id LIMIT $2 OFFSET $3`, tenant, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCapacities(rows)
}

func (t *pgTx) ListMarketplaceCapacities(ctx context.Context, viewer uuid.UUID, limit, offset int) ([]domain.Capacity, error) {
	rows, err := t.tx.Query(ctx, `
		SELECT `+capacityColumns+` FROM network_optimizer.capacities
		WHERE status='AVAILABLE' AND owner_tenant_id <> $1 AND (
			visibility_scope IN ('MARKETPLACE', 'ANONYMIZED')
			OR (visibility_scope='SHIPPER_NETWORK' AND $1 = ANY(audience_tenant_ids))
		)
		ORDER BY created_at DESC, id LIMIT $2 OFFSET $3`, viewer, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCapacities(rows)
}

func (t *pgTx) GetIdempotency(ctx context.Context, tenant uuid.UUID, key string) (IdempotencyRecord, error) {
	var rec IdempotencyRecord
	err := t.tx.QueryRow(ctx, `
		SELECT idempotency_key, request_hash, response_status, response_body
		FROM network_optimizer.idempotency_keys WHERE tenant_id=$1 AND idempotency_key=$2`, tenant, key).
		Scan(&rec.Key, &rec.Hash, &rec.Status, &rec.Body)
	if errors.Is(err, pgx.ErrNoRows) {
		return IdempotencyRecord{}, ErrNotFound
	}
	return rec, err
}

func (t *pgTx) PutIdempotency(ctx context.Context, tenant uuid.UUID, rec IdempotencyRecord) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO network_optimizer.idempotency_keys
		(tenant_id, idempotency_key, request_hash, response_status, response_body, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)`, tenant, rec.Key, rec.Hash, rec.Status, rec.Body, time.Now().UTC())
	if isConstraint(err, "idempotency_keys_pkey") {
		return ErrIdempotencyRace
	}
	return err
}

func (t *pgTx) InsertAudit(ctx context.Context, event AuditEvent) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO network_optimizer.audit_events
		(id, actor_user_id, tenant_id, aggregate_type, aggregate_id, action, from_status, to_status, visibility_scope, source_type, source_id, occurred_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		event.ID, event.ActorUserID, event.TenantID, event.AggregateType, event.AggregateID, event.Action,
		event.FromStatus, event.ToStatus, event.VisibilityScope, event.SourceType, event.SourceID, event.OccurredAt)
	return err
}

func (t *pgTx) InsertOutbox(ctx context.Context, event OutboxEvent) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO network_optimizer.outbox
		(id, event_name, schema_version, tenant_id, aggregate_id, aggregate_version, occurred_at, payload)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		event.ID, event.EventName, event.SchemaVersion, event.TenantID, event.AggregateID, event.AggregateVersion, event.OccurredAt, event.Payload)
	return err
}

func scanLoads(rows pgx.Rows) ([]domain.LoadOpportunity, error) {
	var out []domain.LoadOpportunity
	for rows.Next() {
		load, err := scanLoad(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, load)
	}
	if out == nil {
		out = []domain.LoadOpportunity{}
	}
	return out, rows.Err()
}

func scanLoad(row pgx.Row) (domain.LoadOpportunity, error) {
	var load domain.LoadOpportunity
	var cargo []byte
	var pickupCountry, pickupRegion, pickupCity, deliveryCountry, deliveryRegion, deliveryCity *string
	err := row.Scan(
		&load.ID, &load.OwnerTenantID, &load.SourceType, &load.SourceID,
		&load.Pickup.LocationID, &load.Pickup.Label, &load.Pickup.Latitude, &load.Pickup.Longitude, &load.PickupWindow.Start, &load.PickupWindow.End,
		&load.Delivery.LocationID, &load.Delivery.Label, &load.Delivery.Latitude, &load.Delivery.Longitude, &load.DeliveryWindow.Start, &load.DeliveryWindow.End,
		&load.WeightKg, &load.VolumeM3, &load.BodyType, &load.Equipment, &cargo,
		&load.Commercial.Mode, &load.Commercial.Amount, &load.Commercial.Currency,
		&load.VisibilityScope, &load.InvitedCarrierCompanyIDs,
		&load.Status, &load.Version, &load.CreatedAt, &load.UpdatedAt,
		&pickupCountry, &pickupRegion, &pickupCity, &deliveryCountry, &deliveryRegion, &deliveryCity,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.LoadOpportunity{}, ErrNotFound
	}
	if err != nil {
		return domain.LoadOpportunity{}, err
	}
	if len(cargo) > 0 {
		if err := json.Unmarshal(cargo, &load.Cargo); err != nil {
			return domain.LoadOpportunity{}, err
		}
	}
	load.Pickup.CountryCode = stringValue(pickupCountry)
	load.Pickup.Region = stringValue(pickupRegion)
	load.Pickup.City = stringValue(pickupCity)
	load.Delivery.CountryCode = stringValue(deliveryCountry)
	load.Delivery.Region = stringValue(deliveryRegion)
	load.Delivery.City = stringValue(deliveryCity)
	return load, nil
}

func scanCapacities(rows pgx.Rows) ([]domain.Capacity, error) {
	var out []domain.Capacity
	for rows.Next() {
		cap, err := scanCapacity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, cap)
	}
	if out == nil {
		out = []domain.Capacity{}
	}
	return out, rows.Err()
}

func scanCapacity(row pgx.Row) (domain.Capacity, error) {
	var cap domain.Capacity
	var country, region, city *string
	err := row.Scan(
		&cap.ID, &cap.OwnerTenantID, &cap.CarrierCompanyID, &cap.VehicleID,
		&cap.LocationLabel, &cap.Latitude, &cap.Longitude,
		&cap.AvailableFrom, &cap.AvailableUntil, &cap.Source, &cap.BodyType, &cap.Equipment,
		&cap.PayloadRemainingKg, &cap.VolumeRemainingM3,
		&cap.VisibilityScope, &cap.AudienceTenantIDs,
		&cap.Status, &cap.Version, &cap.CreatedAt, &cap.UpdatedAt,
		&cap.LocationID, &country, &region, &city,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Capacity{}, ErrNotFound
	}
	if err != nil {
		return domain.Capacity{}, err
	}
	cap.CountryCode = stringValue(country)
	cap.Region = stringValue(region)
	cap.City = stringValue(city)
	return cap, nil
}

func nullString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func versionMiss(ctx context.Context, tx pgx.Tx, table string, id uuid.UUID) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM `+table+` WHERE id=$1)`, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return ErrConflict
}

func isConstraint(err error, name string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.ConstraintName == name
}

func emptyStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func emptyUUIDs(v []uuid.UUID) []uuid.UUID {
	if v == nil {
		return []uuid.UUID{}
	}
	return v
}

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditRepository struct {
	pool *pgxpool.Pool
	exec dbExecutor
	// injectRecordFailure is set only by integration tests to verify transactional rollback.
	injectRecordFailure bool
}

func NewAuditRepository(pool *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{pool: pool}
}

// SetInjectRecordFailure enables a one-shot audit insert failure for integration tests.
func (r *AuditRepository) SetInjectRecordFailure(enabled bool) {
	r.injectRecordFailure = enabled
}

type AuditRecord struct {
	TenantID       uuid.UUID
	EntityType     string
	EntityID       uuid.UUID
	Action         string
	ActorUserID    *uuid.UUID
	ActorCompanyID *uuid.UUID
	Metadata       map[string]any
}

type AuditEvent struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	EntityType     string
	EntityID       uuid.UUID
	Action         string
	ActorUserID    *uuid.UUID
	ActorCompanyID *uuid.UUID
	Metadata       map[string]any
	CreatedAt      time.Time
}

func (r *AuditRepository) ListByEntity(ctx context.Context, tenantID uuid.UUID, entityType string, entityID uuid.UUID, limit int) ([]AuditEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	const query = `
		SELECT id, tenant_id, entity_type, entity_id, action, actor_user_id, actor_company_id, metadata, occurred_at
		FROM rfx.audit_events
		WHERE tenant_id = $1 AND entity_type = $2 AND entity_id = $3
		ORDER BY occurred_at DESC
		LIMIT $4
	`
	rows, err := r.db().Query(ctx, query, tenantID, entityType, entityID, limit)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	events := make([]AuditEvent, 0)
	for rows.Next() {
		var event AuditEvent
		var meta []byte
		if err := rows.Scan(
			&event.ID,
			&event.TenantID,
			&event.EntityType,
			&event.EntityID,
			&event.Action,
			&event.ActorUserID,
			&event.ActorCompanyID,
			&meta,
			&event.CreatedAt,
		); err != nil {
			return nil, mapDBError(err)
		}
		event.Metadata = decodeAuditMetadata(meta)
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *AuditRepository) Record(ctx context.Context, rec AuditRecord) error {
	if r.injectRecordFailure {
		return mapDBError(fmt.Errorf("injected audit record failure"))
	}
	meta := rec.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	payload, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	const query = `
		INSERT INTO rfx.audit_events (
			tenant_id, entity_type, entity_id, action, actor_user_id, actor_company_id, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
	`
	_, err = r.db().Exec(ctx, query,
		rec.TenantID,
		rec.EntityType,
		rec.EntityID,
		rec.Action,
		optionalUUID(rec.ActorUserID),
		optionalUUID(rec.ActorCompanyID),
		string(payload),
	)
	return mapDBError(err)
}

func decodeAuditMetadata(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var meta map[string]any
	if err := json.Unmarshal(raw, &meta); err != nil {
		return map[string]any{}
	}
	return meta
}

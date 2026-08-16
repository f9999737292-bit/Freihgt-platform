package repository

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditRepository struct {
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{pool: pool}
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

func (r *AuditRepository) Record(ctx context.Context, rec AuditRecord) error {
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
	_, err = r.pool.Exec(ctx, query,
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

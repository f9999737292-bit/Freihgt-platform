package repository

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/contract-rate-service/internal/domain"
)

type AuditRepository struct{}

func NewAuditRepository() *AuditRepository { return &AuditRepository{} }

type AuditInsert struct {
	TenantID       uuid.UUID
	EntityType     string
	EntityID       uuid.UUID
	Action         string
	ActorUserID    *uuid.UUID
	ActorCompanyID *uuid.UUID
	CorrelationID  *string
	Metadata       map[string]any
}

func (r *AuditRepository) InsertTx(ctx context.Context, tx pgx.Tx, in AuditInsert) error {
	meta := in.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	payload, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	const query = `
		INSERT INTO contract_rate.audit_event (
			tenant_id, entity_type, entity_id, action,
			actor_user_id, actor_company_id, correlation_id, metadata
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb)`
	_, err = tx.Exec(ctx, query,
		in.TenantID, in.EntityType, in.EntityID, in.Action,
		optionalUUID(in.ActorUserID), optionalUUID(in.ActorCompanyID),
		optionalString(in.CorrelationID), string(payload),
	)
	return mapDBError(err)
}

func auditFromActor(entityType string, entityID uuid.UUID, action string, actor domain.ActorInput, correlationID *string, metadata map[string]any) AuditInsert {
	userID := actor.ActorUserID
	companyID := actor.ActorCompanyID
	return AuditInsert{
		TenantID:       actor.TenantID,
		EntityType:     entityType,
		EntityID:       entityID,
		Action:         action,
		ActorUserID:    &userID,
		ActorCompanyID: &companyID,
		CorrelationID:  correlationID,
		Metadata:       metadata,
	}
}

package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/repository"
)

type AuditRecorder interface {
	Record(ctx context.Context, rec repository.AuditRecord) error
}

type ActorResolver interface {
	ResolveActorKind(ctx context.Context, actor domain.ActorContext) (domain.ActorKind, []uuid.UUID, error)
}

func auditUser(actor domain.ActorContext) *uuid.UUID {
	if actor.UserID == uuid.Nil {
		return nil
	}
	id := actor.UserID
	return &id
}

func recordAudit(
	ctx context.Context,
	audit AuditRecorder,
	actor domain.ActorContext,
	verifiedCompanyID uuid.UUID,
	entityType string,
	entityID uuid.UUID,
	action string,
	metadata map[string]any,
) error {
	if audit == nil {
		return nil
	}
	return audit.Record(ctx, repository.AuditRecord{
		TenantID:       actor.TenantID,
		EntityType:     entityType,
		EntityID:       entityID,
		Action:         action,
		ActorUserID:    auditUser(actor),
		ActorCompanyID: verifiedActorCompany(actor, verifiedCompanyID),
		Metadata:       metadata,
	})
}

func recordSystemAudit(
	ctx context.Context,
	audit AuditRecorder,
	tenantID uuid.UUID,
	entityType string,
	entityID uuid.UUID,
	action string,
	metadata map[string]any,
) error {
	if audit == nil {
		return nil
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["actor_type"] = domain.AuditActorTypeSystem
	metadata["source"] = "deadline_worker"
	return audit.Record(ctx, repository.AuditRecord{
		TenantID:   tenantID,
		EntityType: entityType,
		EntityID:   entityID,
		Action:     action,
		Metadata:   metadata,
	})
}

func nowUTC() time.Time {
	return time.Now().UTC()
}

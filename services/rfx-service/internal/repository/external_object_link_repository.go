package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type ExternalObjectLinkRepository struct {
	pool *pgxpool.Pool
	exec dbExecutor
}

func NewExternalObjectLinkRepository(pool *pgxpool.Pool) *ExternalObjectLinkRepository {
	return &ExternalObjectLinkRepository{pool: pool}
}

func (r *ExternalObjectLinkRepository) WithTx(tx pgx.Tx) *ExternalObjectLinkRepository {
	return &ExternalObjectLinkRepository{pool: r.pool, exec: tx}
}

func (r *ExternalObjectLinkRepository) db() dbExecutor {
	if r.exec != nil {
		return r.exec
	}
	return r.pool
}

func (r *ExternalObjectLinkRepository) InsertLink(ctx context.Context, link domain.ExternalObjectLink) (*domain.ExternalObjectLink, error) {
	if err := validateExternalObjectLink(link); err != nil {
		return nil, err
	}
	revision := strings.TrimSpace(link.ExternalRevision)
	if revision == "" {
		revision = strings.TrimSpace(link.ExternalVersion)
	}
	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_external_object_links (
			tenant_id, integration_principal_id, external_system, external_object_type,
			external_object_id, external_version, external_revision, payload_hash, rfx_event_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, tenant_id, integration_principal_id, external_system, external_object_type,
		          external_object_id, external_version, external_revision, payload_hash, rfx_event_id, created_at, updated_at
	`, link.TenantID, link.IntegrationPrincipalID, link.ExternalSystem, link.ExternalObjectType,
		link.ExternalObjectID, link.ExternalVersion, revision, link.PayloadHash, link.RfxEventID)
	created, err := scanExternalObjectLink(row)
	if err != nil {
		return nil, mapStableIdentityConflict(err)
	}
	return created, nil
}

func mapStableIdentityConflict(err error) error {
	mapped := mapDBError(err)
	var appErr *apperrors.AppError
	if errors.As(mapped, &appErr) && appErr.Code == apperrors.CodeConflict {
		detail, _ := appErr.Details["detail"].(string)
		if strings.Contains(detail, "uq_rfx_external_object_stable_identity") {
			return apperrors.Conflict("stable external identity already exists", map[string]any{
				"machine_code": domain.MachineCodeExternalIDConflict,
			})
		}
	}
	return mapped
}

func (r *ExternalObjectLinkRepository) UpsertLink(ctx context.Context, link domain.ExternalObjectLink) (*domain.ExternalObjectLink, error) {
	if err := validateExternalObjectLink(link); err != nil {
		return nil, err
	}
	revision := strings.TrimSpace(link.ExternalRevision)
	if revision == "" {
		revision = strings.TrimSpace(link.ExternalVersion)
	}
	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_external_object_links (
			tenant_id, integration_principal_id, external_system, external_object_type,
			external_object_id, external_version, external_revision, payload_hash, rfx_event_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (
			tenant_id, integration_principal_id, external_system,
			external_object_type, external_object_id
		) DO UPDATE SET
			payload_hash = EXCLUDED.payload_hash,
			external_version = EXCLUDED.external_version,
			external_revision = EXCLUDED.external_revision,
			rfx_event_id = EXCLUDED.rfx_event_id,
			updated_at = now()
		RETURNING id, tenant_id, integration_principal_id, external_system, external_object_type,
		          external_object_id, external_version, external_revision, payload_hash, rfx_event_id, created_at, updated_at
	`, link.TenantID, link.IntegrationPrincipalID, link.ExternalSystem, link.ExternalObjectType,
		link.ExternalObjectID, link.ExternalVersion, revision, link.PayloadHash, link.RfxEventID)
	return scanExternalObjectLink(row)
}

func (r *ExternalObjectLinkRepository) GetByStableIdentity(
	ctx context.Context,
	tenantID, integrationPrincipalID uuid.UUID,
	externalSystem, externalObjectType, externalObjectID string,
) (*domain.ExternalObjectLink, error) {
	row := r.db().QueryRow(ctx, `
		SELECT id, tenant_id, integration_principal_id, external_system, external_object_type,
		       external_object_id, external_version, external_revision, payload_hash, rfx_event_id, created_at, updated_at
		FROM rfx.rfx_external_object_links
		WHERE tenant_id = $1
		  AND integration_principal_id = $2
		  AND external_system = $3
		  AND external_object_type = $4
		  AND external_object_id = $5
	`, tenantID, integrationPrincipalID, externalSystem, externalObjectType, externalObjectID)
	link, err := scanExternalObjectLink(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("external object link not found")
		}
		return nil, mapDBError(err)
	}
	return link, nil
}

func (r *ExternalObjectLinkRepository) RecordRevision(ctx context.Context, revision domain.ExternalObjectLinkRevision) (*domain.ExternalObjectLinkRevision, error) {
	if revision.TenantID == uuid.Nil || revision.LinkID == uuid.Nil {
		return nil, apperrors.Validation("tenant_id and link_id are required", map[string]any{"field": "revision"})
	}
	if strings.TrimSpace(revision.ExternalRevision) == "" {
		return nil, apperrors.Validation("external_revision is required", map[string]any{"field": "external_revision"})
	}
	if len(strings.TrimSpace(revision.PayloadHash)) != 64 {
		return nil, apperrors.Validation("payload_hash must be sha256 hex", map[string]any{"field": "payload_hash"})
	}
	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_external_object_link_revisions (
			tenant_id, link_id, external_revision, payload_hash
		) VALUES ($1, $2, $3, $4)
		RETURNING id, tenant_id, link_id, external_revision, payload_hash, recorded_at
	`, revision.TenantID, revision.LinkID, strings.TrimSpace(revision.ExternalRevision), revision.PayloadHash)
	var out domain.ExternalObjectLinkRevision
	if err := row.Scan(&out.ID, &out.TenantID, &out.LinkID, &out.ExternalRevision, &out.PayloadHash, &out.RecordedAt); err != nil {
		return nil, mapDBError(err)
	}
	return &out, nil
}

func (r *ExternalObjectLinkRepository) GetByExternalIdentity(
	ctx context.Context,
	tenantID, integrationPrincipalID uuid.UUID,
	externalSystem, externalObjectType, externalObjectID, externalVersion string,
) (*domain.ExternalObjectLink, error) {
	return r.GetByStableIdentity(ctx, tenantID, integrationPrincipalID, externalSystem, externalObjectType, externalObjectID)
}

func validateExternalObjectLink(link domain.ExternalObjectLink) error {
	if link.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if link.IntegrationPrincipalID == uuid.Nil {
		return apperrors.Validation("integration_principal_id is required", map[string]any{"field": "integration_principal_id"})
	}
	if strings.TrimSpace(link.ExternalSystem) == "" {
		return apperrors.Validation("external_system is required", map[string]any{"field": "external_system"})
	}
	if strings.TrimSpace(link.ExternalObjectType) == "" {
		return apperrors.Validation("external_object_type is required", map[string]any{"field": "external_object_type"})
	}
	if strings.TrimSpace(link.ExternalObjectID) == "" {
		return apperrors.Validation("external_object_id is required", map[string]any{"field": "external_object_id"})
	}
	if strings.TrimSpace(link.ExternalVersion) == "" && strings.TrimSpace(link.ExternalRevision) == "" {
		return apperrors.Validation("external_revision is required", map[string]any{"field": "external_revision"})
	}
	if len(strings.TrimSpace(link.PayloadHash)) != 64 {
		return apperrors.Validation("payload_hash must be sha256 hex", map[string]any{"field": "payload_hash"})
	}
	if link.RfxEventID == uuid.Nil {
		return apperrors.Validation("rfx_event_id is required", map[string]any{"field": "rfx_event_id"})
	}
	return nil
}

func scanExternalObjectLink(row pgx.Row) (*domain.ExternalObjectLink, error) {
	var out domain.ExternalObjectLink
	var externalRevision *string
	err := row.Scan(
		&out.ID, &out.TenantID, &out.IntegrationPrincipalID, &out.ExternalSystem, &out.ExternalObjectType,
		&out.ExternalObjectID, &out.ExternalVersion, &externalRevision, &out.PayloadHash, &out.RfxEventID, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if externalRevision != nil {
		out.ExternalRevision = *externalRevision
	}
	return &out, nil
}

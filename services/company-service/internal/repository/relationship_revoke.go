package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
)

// RevokeBeforeCommitHook runs after the version, head, audit, outbox, and idempotency
// writes and before commit. A non-nil error rolls the transition back. Production leaves it nil.
var RevokeBeforeCommitHook func(context.Context) error

type relationshipRoot struct {
	ShipperTenantID    uuid.UUID
	ShipperCompanyID   uuid.UUID
	OperatingTenantID  uuid.UUID
	OperatingCompanyID uuid.UUID
}

func (r *RelationshipRepository) RevokeRelationship(ctx context.Context, in domain.PersistRevoke) (domain.PersistRevokeResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.PersistRevokeResult{}, revokeStoreError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var root relationshipRoot
	err = tx.QueryRow(ctx, `
		SELECT shipper_tenant_id, shipper_carrier_company_id, operating_tenant_id, operating_carrier_company_id
		FROM company.carrier_relationships
		WHERE relationship_id = $1
	`, in.RelationshipID).Scan(&root.ShipperTenantID, &root.ShipperCompanyID, &root.OperatingTenantID, &root.OperatingCompanyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PersistRevokeResult{}, revokeNotFound()
	}
	if err != nil {
		return domain.PersistRevokeResult{}, revokeStoreError(err)
	}
	if in.ActorTenantID != root.ShipperTenantID && in.ActorTenantID != root.OperatingTenantID {
		return domain.PersistRevokeResult{}, revokeNotFound()
	}

	lockKey := "relationship:" + root.ShipperTenantID.String() + ":" + root.ShipperCompanyID.String() + ":" + root.OperatingTenantID.String()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, lockKey); err != nil {
		return domain.PersistRevokeResult{}, revokeStoreError(err)
	}

	var currentVersion int
	var currentVersionID uuid.UUID
	var headStatus string
	err = tx.QueryRow(ctx, `
		SELECT current_version, current_version_id, status
		FROM company.carrier_relationship_heads
		WHERE relationship_id = $1
		FOR UPDATE
	`, in.RelationshipID).Scan(&currentVersion, &currentVersionID, &headStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PersistRevokeResult{}, revokeNotFound()
	}
	if err != nil {
		return domain.PersistRevokeResult{}, revokeStoreError(err)
	}

	actor, err := loadRevokeActor(ctx, tx, in.ActorTenantID, in.ActorUserID, in.SelectedCompanyID)
	if err != nil {
		return domain.PersistRevokeResult{}, err
	}
	side, sourceTypes, ok, err := revokeSide(ctx, tx, root, in)
	if err != nil {
		return domain.PersistRevokeResult{}, err
	}
	if !ok || !actor.Active || !actor.ActiveMembership {
		return domain.PersistRevokeResult{}, revokeNotFound()
	}
	if !revokeRoleAllows(side, actor, sourceTypes) {
		return domain.PersistRevokeResult{}, revokeForbidden()
	}

	var storedVersion int
	err = tx.QueryRow(ctx, `
		SELECT relationship_version
		FROM company.carrier_relationship_revoke_idempotency
		WHERE relationship_id = $1 AND actor_tenant_id = $2 AND selected_company_id = $3 AND idempotency_key = $4
	`, in.RelationshipID, in.ActorTenantID, in.SelectedCompanyID, in.IdempotencyKey).Scan(&storedVersion)
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return domain.PersistRevokeResult{}, revokeStoreError(err)
		}
		return domain.PersistRevokeResult{
			RelationshipID: in.RelationshipID, RelationshipVersion: storedVersion, Status: domain.RelationshipStatusRevoked,
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.PersistRevokeResult{}, revokeStoreError(err)
	}

	if headStatus == domain.RelationshipStatusRevoked {
		if err := insertRevokeIdempotency(ctx, tx, in, currentVersion); err != nil {
			return domain.PersistRevokeResult{}, err
		}
		if err := commitRevoke(ctx, tx); err != nil {
			return domain.PersistRevokeResult{}, err
		}
		return domain.PersistRevokeResult{
			RelationshipID: in.RelationshipID, RelationshipVersion: currentVersion, Status: domain.RelationshipStatusRevoked,
		}, nil
	}
	if headStatus != domain.RelationshipStatusActive {
		return domain.PersistRevokeResult{}, revokeStoreError(errors.New("unknown relationship status"))
	}
	if in.ExpectedRelationshipVersion != nil && *in.ExpectedRelationshipVersion != currentVersion {
		return domain.PersistRevokeResult{}, apperrors.New(apperrors.CodeRelationshipVersionConflict, "relationship version conflict")
	}

	var sourceRequestID uuid.UUID
	var sourceType string
	var sourceID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT v.source_relationship_request_id, r.source_type, r.source_id
		FROM company.carrier_relationship_versions v
		JOIN company.carrier_relationship_requests r ON r.relationship_request_id = v.source_relationship_request_id
		WHERE v.relationship_version_id = $1
	`, currentVersionID).Scan(&sourceRequestID, &sourceType, &sourceID)
	if err != nil {
		return domain.PersistRevokeResult{}, revokeStoreError(err)
	}

	next := currentVersion + 1
	nextID := uuid.New()
	var reason *string
	if in.Reason != "" {
		reason = &in.Reason
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO company.carrier_relationship_versions (
			relationship_version_id, relationship_id, relationship_version, status,
			supersedes_relationship_version_id, source_relationship_request_id, created_by, revoke_reason
		) VALUES ($1, $2, $3, 'REVOKED', $4, $5, $6, $7)
	`, nextID, in.RelationshipID, next, currentVersionID, sourceRequestID, in.ActorUserID, reason); err != nil {
		return domain.PersistRevokeResult{}, revokeStoreError(err)
	}
	tag, err := tx.Exec(ctx, `
		UPDATE company.carrier_relationship_heads
		SET current_version = $2, current_version_id = $3, status = 'REVOKED'
		WHERE relationship_id = $1 AND status = 'ACTIVE' AND current_version = $4
	`, in.RelationshipID, next, nextID, currentVersion)
	if err != nil {
		return domain.PersistRevokeResult{}, revokeStoreError(err)
	}
	if tag.RowsAffected() != 1 {
		return domain.PersistRevokeResult{}, revokeStoreError(errors.New("relationship head was not moved"))
	}
	if err := insertAudit(ctx, tx, auditRow{
		ActorUserID: in.ActorUserID, ActorTenantID: in.ActorTenantID, SelectedCompanyID: in.SelectedCompanyID,
		RequestID: &sourceRequestID, RelationshipID: &in.RelationshipID, RelationshipVersion: &next,
		SourceType: sourceType, SourceID: &sourceID, IdempotencyKey: in.IdempotencyKey,
		CorrelationID: in.CorrelationID, Reason: in.Reason, Action: "relationship_revoked",
	}); err != nil {
		return domain.PersistRevokeResult{}, err
	}
	if err := insertOutbox(ctx, tx, "carrier_relationship.revoked", in.RelationshipID, map[string]any{
		"relationship_id":                    in.RelationshipID.String(),
		"relationship_version":               next,
		"supersedes_relationship_version_id": currentVersionID.String(),
		"status":                             domain.RelationshipStatusRevoked,
	}); err != nil {
		return domain.PersistRevokeResult{}, err
	}
	if err := insertRevokeIdempotency(ctx, tx, in, next); err != nil {
		return domain.PersistRevokeResult{}, err
	}
	if err := commitRevoke(ctx, tx); err != nil {
		return domain.PersistRevokeResult{}, err
	}
	return domain.PersistRevokeResult{
		RelationshipID: in.RelationshipID, RelationshipVersion: next, Status: domain.RelationshipStatusRevoked,
	}, nil
}

func revokeSide(ctx context.Context, tx pgx.Tx, root relationshipRoot, in domain.PersistRevoke) (string, []string, bool, error) {
	if in.ActorTenantID == root.OperatingTenantID && in.SelectedCompanyID == root.OperatingCompanyID {
		return "operating", nil, true, nil
	}
	if in.ActorTenantID != root.ShipperTenantID {
		return "", nil, false, nil
	}
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT r.source_type
		FROM company.carrier_relationship_request_events e
		JOIN company.carrier_relationship_requests r ON r.relationship_request_id = e.relationship_request_id
		WHERE e.event_type = 'ACCEPTED'
		  AND e.relationship_id = $1
		  AND r.requesting_company_id = $2
		  AND r.source_tenant_id = $3
	`, in.RelationshipID, in.SelectedCompanyID, in.ActorTenantID)
	if err != nil {
		return "", nil, false, revokeStoreError(err)
	}
	defer rows.Close()
	var sourceTypes []string
	for rows.Next() {
		var sourceType string
		if err := rows.Scan(&sourceType); err != nil {
			return "", nil, false, revokeStoreError(err)
		}
		sourceTypes = append(sourceTypes, sourceType)
	}
	if err := rows.Err(); err != nil {
		return "", nil, false, revokeStoreError(err)
	}
	if len(sourceTypes) == 0 {
		return "", nil, false, nil
	}
	return "shipper", sourceTypes, true, nil
}

func revokeRoleAllows(side string, actor domain.ActorAuthority, sourceTypes []string) bool {
	switch side {
	case "operating":
		return domain.CanRevokeOperating(actor)
	case "shipper":
		for _, sourceType := range sourceTypes {
			if domain.CanRevokeShipper(actor, sourceType) {
				return true
			}
		}
	}
	return false
}

func loadRevokeActor(ctx context.Context, tx pgx.Tx, tenantID, userID, companyID uuid.UUID) (domain.ActorAuthority, error) {
	var actor domain.ActorAuthority
	err := tx.QueryRow(ctx, `
		SELECT
			EXISTS (
				SELECT 1
				FROM core.company_memberships m
				JOIN core.companies c ON c.id = m.company_id
				WHERE m.tenant_id = u.tenant_id
				  AND m.user_id = u.id
				  AND m.company_id = $3
				  AND m.status = 'ACTIVE'
				  AND m.deleted_at IS NULL
				  AND c.tenant_id = u.tenant_id
				  AND c.deleted_at IS NULL
				  AND c.status = 'ACTIVE'
			),
			COALESCE((
				SELECT array_agg(r.code ORDER BY r.code)
				FROM core.user_roles ur
				JOIN core.roles r ON r.id = ur.role_id AND r.deleted_at IS NULL
				WHERE ur.tenant_id = u.tenant_id
				  AND ur.user_id = u.id
				  AND ur.company_id = $3
			), ARRAY[]::varchar[]),
			EXISTS (
				SELECT 1
				FROM core.user_roles ur
				JOIN core.roles r ON r.id = ur.role_id AND r.deleted_at IS NULL
				WHERE ur.tenant_id = u.tenant_id
				  AND ur.user_id = u.id
				  AND ur.company_id IS NULL
				  AND r.code = 'PLATFORM_ADMIN'
			)
		FROM core.users u
		WHERE u.id = $1
		  AND u.tenant_id = $2
		  AND u.deleted_at IS NULL
		  AND u.status = 'ACTIVE'
	`, userID, tenantID, companyID).Scan(&actor.ActiveMembership, &actor.CompanyRoles, &actor.TenantGlobalPlatformAdmin)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ActorAuthority{}, nil
	}
	if err != nil {
		return domain.ActorAuthority{}, revokeStoreError(err)
	}
	actor.Active = true
	if actor.CompanyRoles == nil {
		actor.CompanyRoles = []string{}
	}
	return actor, nil
}

func insertRevokeIdempotency(ctx context.Context, tx pgx.Tx, in domain.PersistRevoke, version int) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO company.carrier_relationship_revoke_idempotency (
			relationship_id, actor_tenant_id, selected_company_id, idempotency_key, body_hash, relationship_version
		) VALUES ($1, $2, $3, $4, $5, $6)
	`, in.RelationshipID, in.ActorTenantID, in.SelectedCompanyID, in.IdempotencyKey, domain.RevokeBodyHash(in.RelationshipID), version)
	if err != nil {
		return revokeStoreError(err)
	}
	return nil
}

func commitRevoke(ctx context.Context, tx pgx.Tx) error {
	if RevokeBeforeCommitHook != nil {
		if err := RevokeBeforeCommitHook(ctx); err != nil {
			return revokeStoreError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return revokeStoreError(err)
	}
	return nil
}

func revokeNotFound() error {
	return apperrors.New(apperrors.CodeRelationshipNotFound, "relationship not found")
}

func revokeForbidden() error {
	return apperrors.New(apperrors.CodeRelationshipRevokeForbidden, "relationship command refused")
}

func revokeStoreError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505", "23514", "55000":
			return apperrors.New(apperrors.CodeRelationshipStoreUnavailable, "relationship store unavailable")
		}
	}
	return apperrors.New(apperrors.CodeRelationshipStoreUnavailable, "relationship store unavailable")
}

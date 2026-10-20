package repository

import (
	"context"
	"crypto/hmac"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
)

type RelationshipRepository struct {
	pool *pgxpool.Pool
}

func NewRelationshipRepository(pool *pgxpool.Pool) *RelationshipRepository {
	return &RelationshipRepository{pool: pool}
}

func (r *RelationshipRepository) LoadRelationshipActor(ctx context.Context, tenantID, userID, companyID uuid.UUID) (domain.ActorAuthority, error) {
	const query = `
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
	`
	var actor domain.ActorAuthority
	err := r.pool.QueryRow(ctx, query, userID, tenantID, companyID).Scan(
		&actor.ActiveMembership,
		&actor.CompanyRoles,
		&actor.TenantGlobalPlatformAdmin,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ActorAuthority{}, nil
	}
	if err != nil {
		return domain.ActorAuthority{}, relationshipStoreError(err)
	}
	actor.Active = true
	if actor.CompanyRoles == nil {
		actor.CompanyRoles = []string{}
	}
	return actor, nil
}

func (r *RelationshipRepository) ActiveUserInTenant(ctx context.Context, tenantID, userID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM core.users
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL AND status = 'ACTIVE'
		)
	`
	var active bool
	if err := r.pool.QueryRow(ctx, query, userID, tenantID).Scan(&active); err != nil {
		return false, relationshipStoreError(err)
	}
	return active, nil
}

func (r *RelationshipRepository) ActiveCompanyInTenant(ctx context.Context, tenantID, companyID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM core.companies
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL AND status = 'ACTIVE'
		)
	`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, companyID, tenantID).Scan(&exists); err != nil {
		return false, relationshipStoreError(err)
	}
	return exists, nil
}

func (r *RelationshipRepository) CreateRelationshipRequest(ctx context.Context, in domain.PersistCreate) (domain.PersistCreateResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.PersistCreateResult{}, relationshipStoreError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, createScope(in)); err != nil {
		return domain.PersistCreateResult{}, relationshipStoreError(err)
	}

	var existingHash []byte
	var existingID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT request_hash, relationship_request_id
		FROM company.carrier_relationship_request_idempotency
		WHERE caller_service = $1 AND source_tenant_id = $2 AND requesting_company_id = $3 AND idempotency_key = $4
	`, in.CallerService, in.SourceTenantID, in.RequestingCompanyID, in.IdempotencyKey).Scan(&existingHash, &existingID)
	if err == nil {
		if !hmac.Equal(existingHash, in.RequestHash) {
			return domain.PersistCreateResult{}, apperrors.New(apperrors.CodeRelationshipIdempotencyConflict, "idempotency conflict")
		}
		status, statusErr := requestStatus(ctx, tx, existingID)
		if statusErr != nil {
			return domain.PersistCreateResult{}, statusErr
		}
		return domain.PersistCreateResult{RequestID: existingID, Status: status, Created: false}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.PersistCreateResult{}, relationshipStoreError(err)
	}

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, pendingScope(in)); err != nil {
		return domain.PersistCreateResult{}, relationshipStoreError(err)
	}
	var pendingID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT relationship_request_id
		FROM company.carrier_relationship_request_heads
		WHERE source_type = $1 AND source_id = $2 AND shipper_carrier_company_id = $3 AND request_status = 'PENDING'
		FOR UPDATE
	`, in.SourceType, in.SourceID, in.ShipperCarrierCompanyID).Scan(&pendingID)
	if err == nil {
		return domain.PersistCreateResult{}, apperrors.New(apperrors.CodeRelationshipRequestPendingConflict, "pending relationship request already exists")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.PersistCreateResult{}, relationshipStoreError(err)
	}

	requestID := uuid.New()
	eventID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO company.carrier_relationship_requests (
			relationship_request_id, source_type, source_id, source_tenant_id, source_version_or_fingerprint,
			shipper_carrier_company_id, requesting_company_id, requesting_user_id, acceptance_token_hash,
			token_created_at, token_expires_at, created_at, created_by
		)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, ts.t, ts.t + interval '72 hours', ts.t, $8
		FROM (SELECT clock_timestamp() AS t) ts
	`, requestID, in.SourceType, in.SourceID, in.SourceTenantID, in.Fingerprint, in.ShipperCarrierCompanyID,
		in.RequestingCompanyID, in.RequestingUserID, in.TokenHash)
	if err != nil {
		return domain.PersistCreateResult{}, relationshipStoreError(err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO company.carrier_relationship_request_events (
			event_id, relationship_request_id, request_version, event_type, request_status, actor_user_id
		) VALUES ($1, $2, 1, 'CREATED', 'PENDING', $3)
	`, eventID, requestID, in.RequestingUserID)
	if err != nil {
		return domain.PersistCreateResult{}, relationshipStoreError(err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO company.carrier_relationship_request_heads (
			relationship_request_id, source_type, source_id, shipper_carrier_company_id,
			current_event_id, current_version, request_status
		) VALUES ($1, $2, $3, $4, $5, 1, 'PENDING')
	`, requestID, in.SourceType, in.SourceID, in.ShipperCarrierCompanyID, eventID)
	if err != nil {
		return domain.PersistCreateResult{}, relationshipStoreError(err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO company.carrier_relationship_request_idempotency (
			caller_service, source_tenant_id, requesting_company_id, idempotency_key, request_hash, relationship_request_id
		) VALUES ($1, $2, $3, $4, $5, $6)
	`, in.CallerService, in.SourceTenantID, in.RequestingCompanyID, in.IdempotencyKey, in.RequestHash, requestID)
	if err != nil {
		return domain.PersistCreateResult{}, relationshipStoreError(err)
	}
	if err := insertAudit(ctx, tx, auditRow{
		ActorUserID: in.RequestingUserID, ActorTenantID: in.SourceTenantID, SelectedCompanyID: in.RequestingCompanyID,
		RequestID: &requestID, SourceType: in.SourceType, SourceID: &in.SourceID, IdempotencyKey: in.IdempotencyKey,
		CorrelationID: in.CorrelationID, Action: "relationship_request_created",
	}); err != nil {
		return domain.PersistCreateResult{}, err
	}
	if err := insertOutbox(ctx, tx, "carrier_relationship.requested", requestID, map[string]any{
		"relationship_request_id": requestID.String(),
		"source_type":             in.SourceType,
		"source_id":               in.SourceID.String(),
		"source_tenant_id":        in.SourceTenantID.String(),
		"request_status":          domain.RequestStatusPending,
	}); err != nil {
		return domain.PersistCreateResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.PersistCreateResult{}, relationshipStoreError(err)
	}
	return domain.PersistCreateResult{RequestID: requestID, Status: domain.RequestStatusPending, Created: true}, nil
}

func (r *RelationshipRepository) AcceptRelationshipRequest(ctx context.Context, in domain.PersistAccept, source domain.SourceRevalidator) (domain.PersistAcceptResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.PersistAcceptResult{}, relationshipStoreError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, acceptScope(in)); err != nil {
		return domain.PersistAcceptResult{}, relationshipStoreError(err)
	}

	var storedCompany uuid.UUID
	var storedRequest uuid.UUID
	var storedHash []byte
	var storedRelationship uuid.UUID
	var storedVersion int
	err = tx.QueryRow(ctx, `
		SELECT operating_company_id, relationship_request_id, body_hash, relationship_id, relationship_version
		FROM company.carrier_relationship_acceptance_idempotency
		WHERE actor_tenant_id = $1 AND idempotency_key = $2
	`, in.ActorTenantID, in.IdempotencyKey).Scan(&storedCompany, &storedRequest, &storedHash, &storedRelationship, &storedVersion)
	if err == nil {
		if storedCompany != in.OperatingCompanyID || (in.RequestID != nil && *in.RequestID != storedRequest) || !hmac.Equal(storedHash, in.BodyHash) {
			return domain.PersistAcceptResult{}, apperrors.New(apperrors.CodeRelationshipIdempotencyConflict, "idempotency conflict")
		}
		return domain.PersistAcceptResult{
			RelationshipID: storedRelationship, RelationshipVersion: storedVersion,
			RequestStatus: domain.RequestStatusAccepted, Created: false,
		}, commitAccept(ctx, tx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.PersistAcceptResult{}, relationshipStoreError(err)
	}
	if !in.TokenPresent {
		return domain.PersistAcceptResult{}, relationshipNotFound()
	}

	var request requestRow
	err = tx.QueryRow(ctx, `
		SELECT relationship_request_id, acceptance_token_hash, source_type, source_id, source_tenant_id,
		       source_version_or_fingerprint, shipper_carrier_company_id, token_expires_at <= clock_timestamp()
		FROM company.carrier_relationship_requests
		WHERE acceptance_token_hash = $1
	`, in.TokenHash).Scan(
		&request.ID, &request.TokenHash, &request.SourceType, &request.SourceID, &request.SourceTenantID,
		&request.Fingerprint, &request.CarrierCompanyID, &request.Expired,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		var dummy [32]byte
		hmac.Equal(in.TokenHash, dummy[:])
		return domain.PersistAcceptResult{}, relationshipNotFound()
	}
	if err != nil {
		return domain.PersistAcceptResult{}, relationshipStoreError(err)
	}
	if !hmac.Equal(request.TokenHash, in.TokenHash) || (in.RequestID != nil && *in.RequestID != request.ID) {
		return domain.PersistAcceptResult{}, relationshipNotFound()
	}

	var status string
	var version int
	err = tx.QueryRow(ctx, `
		SELECT request_status, current_version
		FROM company.carrier_relationship_request_heads
		WHERE relationship_request_id = $1
		FOR UPDATE
	`, request.ID).Scan(&status, &version)
	if err != nil {
		return domain.PersistAcceptResult{}, relationshipStoreError(err)
	}
	if request.Expired || status == domain.RequestStatusExpired {
		if status == domain.RequestStatusPending {
			if err := appendRequestEvent(ctx, tx, request.ID, version+1, "EXPIRED", domain.RequestStatusExpired, in.ActorUserID, in.ActorTenantID, in.OperatingCompanyID, nil); err != nil {
				return domain.PersistAcceptResult{}, err
			}
			if err := commitAccept(ctx, tx); err != nil {
				return domain.PersistAcceptResult{}, err
			}
		}
		return domain.PersistAcceptResult{}, apperrors.New(apperrors.CodeRelationshipRequestExpired, "relationship request expired")
	}
	switch status {
	case domain.RequestStatusAccepted:
		return domain.PersistAcceptResult{}, apperrors.New(apperrors.CodeRelationshipRequestAlreadyAccepted, "relationship request already accepted")
	case domain.RequestStatusCancelled:
		return domain.PersistAcceptResult{}, apperrors.New(apperrors.CodeRelationshipRequestCancelled, "relationship request cancelled")
	case domain.RequestStatusPending:
	default:
		return domain.PersistAcceptResult{}, relationshipStoreError(errors.New("unknown request status"))
	}

	if err := lockRelationship(ctx, tx, request, in.ActorTenantID); err != nil {
		return domain.PersistAcceptResult{}, err
	}
	mismatch, err := operatingEndpointMismatch(ctx, tx, request, in)
	if err != nil {
		return domain.PersistAcceptResult{}, err
	}
	if mismatch {
		return domain.PersistAcceptResult{}, apperrors.New(apperrors.CodeRelationshipAcceptanceForbidden, "relationship command refused")
	}
	if source == nil {
		return domain.PersistAcceptResult{}, apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
	}
	decision, err := source.Revalidate(ctx, domain.SourceSnapshot{
		SourceType: request.SourceType, SourceID: request.SourceID, SourceTenantID: request.SourceTenantID,
		Fingerprint: request.Fingerprint, ShipperCarrierCompanyID: request.CarrierCompanyID,
	})
	if err != nil || decision == domain.SourceUnavailable {
		return domain.PersistAcceptResult{}, apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
	}
	if decision != domain.SourceMatch {
		return domain.PersistAcceptResult{}, apperrors.New(apperrors.CodeRelationshipRequestStaleSource, "relationship source is stale")
	}
	relationshipID, relationshipVersion, err := ensureActiveRelationship(ctx, tx, request, in)
	if err != nil {
		return domain.PersistAcceptResult{}, err
	}
	accepted := acceptedEvent{
		OperatingTenantID:   in.ActorTenantID,
		OperatingCompanyID:  in.OperatingCompanyID,
		RelationshipID:      relationshipID,
		RelationshipVersion: relationshipVersion,
	}
	if err := appendRequestEvent(ctx, tx, request.ID, version+1, "ACCEPTED", domain.RequestStatusAccepted, in.ActorUserID, in.ActorTenantID, in.OperatingCompanyID, &accepted); err != nil {
		return domain.PersistAcceptResult{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO company.carrier_relationship_acceptance_idempotency (
			actor_tenant_id, idempotency_key, operating_company_id, relationship_request_id,
			body_hash, relationship_id, relationship_version
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, in.ActorTenantID, in.IdempotencyKey, in.OperatingCompanyID, request.ID, in.BodyHash, relationshipID, relationshipVersion)
	if err != nil {
		return domain.PersistAcceptResult{}, relationshipStoreError(err)
	}
	if err := insertAudit(ctx, tx, auditRow{
		ActorUserID: in.ActorUserID, ActorTenantID: in.ActorTenantID, SelectedCompanyID: in.OperatingCompanyID,
		RequestID: &request.ID, RelationshipID: &relationshipID, RelationshipVersion: &relationshipVersion,
		SourceType: request.SourceType, SourceID: &request.SourceID, IdempotencyKey: in.IdempotencyKey,
		CorrelationID: in.CorrelationID, Action: "relationship_request_accepted",
	}); err != nil {
		return domain.PersistAcceptResult{}, err
	}
	if err := insertOutbox(ctx, tx, "carrier_relationship.accepted", relationshipID, map[string]any{
		"relationship_request_id": request.ID.String(),
		"relationship_id":         relationshipID.String(),
		"relationship_version":    relationshipVersion,
		"source_type":             request.SourceType,
		"source_id":               request.SourceID.String(),
		"source_tenant_id":        request.SourceTenantID.String(),
		"request_status":          domain.RequestStatusAccepted,
	}); err != nil {
		return domain.PersistAcceptResult{}, err
	}
	if err := commitAccept(ctx, tx); err != nil {
		return domain.PersistAcceptResult{}, err
	}
	return domain.PersistAcceptResult{
		RelationshipID: relationshipID, RelationshipVersion: relationshipVersion,
		RequestStatus: domain.RequestStatusAccepted, Created: true,
	}, nil
}

func (r *RelationshipRepository) GetVisibleRelationshipRequest(ctx context.Context, tenantID, companyID, requestID uuid.UUID) (domain.VisibleRequest, error) {
	const query = `
		SELECT r.relationship_request_id, h.request_status
		FROM company.carrier_relationship_requests r
		JOIN company.carrier_relationship_request_heads h ON h.relationship_request_id = r.relationship_request_id
		WHERE r.relationship_request_id = $1
		  AND (
			(r.source_tenant_id = $2 AND r.requesting_company_id = $3)
			OR EXISTS (
				SELECT 1 FROM company.carrier_relationship_request_events e
				WHERE e.relationship_request_id = r.relationship_request_id
				  AND e.event_type = 'ACCEPTED'
				  AND e.accepted_operating_tenant_id = $2
				  AND e.accepted_operating_carrier_company_id = $3
			)
		  )
	`
	var view domain.VisibleRequest
	err := r.pool.QueryRow(ctx, query, requestID, tenantID, companyID).Scan(&view.RequestID, &view.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.VisibleRequest{}, relationshipNotFound()
	}
	if err != nil {
		return domain.VisibleRequest{}, relationshipStoreError(err)
	}
	return view, nil
}

type requestRow struct {
	ID               uuid.UUID
	TokenHash        []byte
	SourceType       string
	SourceID         uuid.UUID
	SourceTenantID   uuid.UUID
	Fingerprint      string
	CarrierCompanyID uuid.UUID
	Expired          bool
}

type acceptedEvent struct {
	OperatingTenantID   uuid.UUID
	OperatingCompanyID  uuid.UUID
	RelationshipID      uuid.UUID
	RelationshipVersion int
}

type auditRow struct {
	ActorUserID         uuid.UUID
	ActorTenantID       uuid.UUID
	SelectedCompanyID   uuid.UUID
	RequestID           *uuid.UUID
	RelationshipID      *uuid.UUID
	RelationshipVersion *int
	SourceType          string
	SourceID            *uuid.UUID
	IdempotencyKey      string
	CorrelationID       string
	Reason              string
	Action              string
}

func lockRelationship(ctx context.Context, tx pgx.Tx, request requestRow, actorTenant uuid.UUID) error {
	relLock := "relationship:" + request.SourceTenantID.String() + ":" + request.CarrierCompanyID.String() + ":" + actorTenant.String()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, relLock); err != nil {
		return relationshipStoreError(err)
	}
	return nil
}

func operatingEndpointMismatch(ctx context.Context, tx pgx.Tx, request requestRow, in domain.PersistAccept) (bool, error) {
	var stored uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT operating_carrier_company_id
		FROM company.carrier_relationships
		WHERE shipper_tenant_id = $1 AND shipper_carrier_company_id = $2 AND operating_tenant_id = $3
	`, request.SourceTenantID, request.CarrierCompanyID, in.ActorTenantID).Scan(&stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, relationshipStoreError(err)
	}
	return stored != in.OperatingCompanyID, nil
}

func ensureActiveRelationship(ctx context.Context, tx pgx.Tx, request requestRow, in domain.PersistAccept) (uuid.UUID, int, error) {
	var relationshipID uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT relationship_id
		FROM company.carrier_relationships
		WHERE shipper_tenant_id = $1 AND shipper_carrier_company_id = $2 AND operating_tenant_id = $3
	`, request.SourceTenantID, request.CarrierCompanyID, in.ActorTenantID).Scan(&relationshipID)
	if errors.Is(err, pgx.ErrNoRows) {
		relationshipID = uuid.New()
		versionID := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO company.carrier_relationships (
				relationship_id, shipper_tenant_id, shipper_carrier_company_id, operating_tenant_id, operating_carrier_company_id
			) VALUES ($1, $2, $3, $4, $5)
		`, relationshipID, request.SourceTenantID, request.CarrierCompanyID, in.ActorTenantID, in.OperatingCompanyID); err != nil {
			return uuid.Nil, 0, relationshipStoreError(err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO company.carrier_relationship_versions (
				relationship_version_id, relationship_id, relationship_version, status, source_relationship_request_id, created_by
			) VALUES ($1, $2, 1, 'ACTIVE', $3, $4)
		`, versionID, relationshipID, request.ID, in.ActorUserID); err != nil {
			return uuid.Nil, 0, relationshipStoreError(err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO company.carrier_relationship_heads (relationship_id, current_version, current_version_id, status)
			VALUES ($1, 1, $2, 'ACTIVE')
		`, relationshipID, versionID); err != nil {
			return uuid.Nil, 0, relationshipStoreError(err)
		}
		return relationshipID, 1, nil
	}
	if err != nil {
		return uuid.Nil, 0, relationshipStoreError(err)
	}
	var currentVersion int
	var currentVersionID uuid.UUID
	var headStatus string
	err = tx.QueryRow(ctx, `
		SELECT current_version, current_version_id, status
		FROM company.carrier_relationship_heads
		WHERE relationship_id = $1
		FOR UPDATE
	`, relationshipID).Scan(&currentVersion, &currentVersionID, &headStatus)
	if err != nil {
		return uuid.Nil, 0, relationshipStoreError(err)
	}
	if headStatus == domain.RelationshipStatusActive {
		return relationshipID, currentVersion, nil
	}
	if headStatus != domain.RelationshipStatusRevoked {
		return uuid.Nil, 0, relationshipStoreError(errors.New("unknown relationship status"))
	}
	next := currentVersion + 1
	versionID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO company.carrier_relationship_versions (
			relationship_version_id, relationship_id, relationship_version, status,
			supersedes_relationship_version_id, source_relationship_request_id, created_by
		) VALUES ($1, $2, $3, 'ACTIVE', $4, $5, $6)
	`, versionID, relationshipID, next, currentVersionID, request.ID, in.ActorUserID); err != nil {
		return uuid.Nil, 0, relationshipStoreError(err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE company.carrier_relationship_heads
		SET current_version = $2, current_version_id = $3, status = 'ACTIVE'
		WHERE relationship_id = $1
	`, relationshipID, next, versionID); err != nil {
		return uuid.Nil, 0, relationshipStoreError(err)
	}
	return relationshipID, next, nil
}

func appendRequestEvent(ctx context.Context, tx pgx.Tx, requestID uuid.UUID, version int, eventType, status string, actor, actorTenant, selectedCompany uuid.UUID, accepted *acceptedEvent) error {
	eventID := uuid.New()
	if accepted == nil {
		if _, err := tx.Exec(ctx, `
			INSERT INTO company.carrier_relationship_request_events (
				event_id, relationship_request_id, request_version, event_type, request_status, actor_user_id
			) VALUES ($1, $2, $3, $4, $5, $6)
		`, eventID, requestID, version, eventType, status, actor); err != nil {
			return relationshipStoreError(err)
		}
	} else {
		if _, err := tx.Exec(ctx, `
			INSERT INTO company.carrier_relationship_request_events (
				event_id, relationship_request_id, request_version, event_type, request_status, actor_user_id,
				accepted_at, accepted_by, accepted_operating_tenant_id, accepted_operating_carrier_company_id,
				relationship_id, relationship_version
			)
			SELECT $1, $2, $3, $4, $5, $6, ts.t, $6, $7, $8, $9, $10
			FROM (SELECT clock_timestamp() AS t) ts
		`, eventID, requestID, version, eventType, status, actor, accepted.OperatingTenantID, accepted.OperatingCompanyID,
			accepted.RelationshipID, accepted.RelationshipVersion); err != nil {
			return relationshipStoreError(err)
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE company.carrier_relationship_request_heads
		SET current_event_id = $2, current_version = $3, request_status = $4
		WHERE relationship_request_id = $1
	`, requestID, eventID, version, status); err != nil {
		return relationshipStoreError(err)
	}
	if eventType == "EXPIRED" {
		if err := insertAudit(ctx, tx, auditRow{
			ActorUserID: actor, ActorTenantID: actorTenant, SelectedCompanyID: selectedCompany,
			RequestID: &requestID, Action: "relationship_request_expired",
		}); err != nil {
			return err
		}
		return insertOutbox(ctx, tx, "carrier_relationship.expired", requestID, map[string]any{
			"relationship_request_id": requestID.String(),
			"request_status":          domain.RequestStatusExpired,
		})
	}
	return nil
}

func insertAudit(ctx context.Context, tx pgx.Tx, row auditRow) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO company.carrier_relationship_audit (
			audit_id, actor_user_id, actor_tenant_id, selected_company_id, relationship_request_id,
			relationship_id, relationship_version, source_type, source_id, idempotency_key, correlation_id, reason, action
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9, NULLIF($10, ''), NULLIF($11, ''), NULLIF($12, ''), $13)
	`, uuid.New(), row.ActorUserID, row.ActorTenantID, row.SelectedCompanyID, row.RequestID, row.RelationshipID,
		row.RelationshipVersion, row.SourceType, row.SourceID, row.IdempotencyKey, row.CorrelationID, row.Reason, row.Action)
	if err != nil {
		return relationshipStoreError(err)
	}
	return nil
}

func insertOutbox(ctx context.Context, tx pgx.Tx, eventType string, aggregateID uuid.UUID, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return relationshipStoreError(err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO company.carrier_relationship_outbox (outbox_id, event_type, aggregate_id, payload)
		VALUES ($1, $2, $3, $4::jsonb)
	`, uuid.New(), eventType, aggregateID, string(raw))
	if err != nil {
		return relationshipStoreError(err)
	}
	return nil
}

func requestStatus(ctx context.Context, tx pgx.Tx, requestID uuid.UUID) (string, error) {
	var status string
	err := tx.QueryRow(ctx, `
		SELECT request_status FROM company.carrier_relationship_request_heads WHERE relationship_request_id = $1
	`, requestID).Scan(&status)
	if err != nil {
		return "", relationshipStoreError(err)
	}
	return status, nil
}

func commitAccept(ctx context.Context, tx pgx.Tx) error {
	if err := tx.Commit(ctx); err != nil {
		return relationshipStoreError(err)
	}
	return nil
}

func createScope(in domain.PersistCreate) string {
	return "create:" + in.CallerService + ":" + in.SourceTenantID.String() + ":" + in.RequestingCompanyID.String() + ":" + in.IdempotencyKey
}

func pendingScope(in domain.PersistCreate) string {
	return "pending:" + in.SourceType + ":" + in.SourceID.String() + ":" + in.ShipperCarrierCompanyID.String()
}

func acceptScope(in domain.PersistAccept) string {
	return "accept:" + in.ActorTenantID.String() + ":" + in.IdempotencyKey
}

func relationshipNotFound() error {
	return apperrors.New(apperrors.CodeRelationshipRequestNotFound, "relationship request not found")
}

func relationshipStoreError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			if pgErr.ConstraintName == "uq_crr_one_pending_source_carrier" {
				return apperrors.New(apperrors.CodeRelationshipRequestPendingConflict, "pending relationship request already exists")
			}
			if pgErr.ConstraintName == "uq_crr_one_terminal_event" || pgErr.ConstraintName == "uq_crr_event_version" {
				return apperrors.New(apperrors.CodeRelationshipStoreUnavailable, "relationship store unavailable")
			}
			return apperrors.New(apperrors.CodeRelationshipIdempotencyConflict, "idempotency conflict")
		case "23514", "55000":
			return apperrors.New(apperrors.CodeRelationshipRequestForbidden, "relationship command refused")
		}
	}
	return apperrors.New(apperrors.CodeRelationshipStoreUnavailable, "relationship store unavailable")
}

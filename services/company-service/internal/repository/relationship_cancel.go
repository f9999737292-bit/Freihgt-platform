package repository

import (
	"context"
	"crypto/hmac"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
)

// CancelTerminalInsertHook runs after the request head is locked and before the
// terminal event insert. CancelTerminalConflictHook runs after that transaction
// rolls back and before the single locked reread. Production leaves both nil.
var (
	CancelTerminalInsertHook   func(context.Context)
	CancelTerminalConflictHook func(context.Context)
)

var errTerminalEventConflict = errors.New("terminal relationship event conflict")

func (r *RelationshipRepository) ListSourceRelationshipRequests(ctx context.Context, sourceType string, sourceID, sourceTenantID uuid.UUID) ([]domain.SourceRequestHead, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT r.relationship_request_id, h.request_status, r.shipper_carrier_company_id,
		       r.requesting_company_id, r.token_expires_at
		FROM company.carrier_relationship_requests r
		JOIN company.carrier_relationship_request_heads h
		  ON h.relationship_request_id = r.relationship_request_id
		WHERE r.source_type = $1 AND r.source_id = $2 AND r.source_tenant_id = $3
		ORDER BY r.created_at, r.relationship_request_id
	`, sourceType, sourceID, sourceTenantID)
	if err != nil {
		return nil, relationshipStoreError(err)
	}
	defer rows.Close()
	var out []domain.SourceRequestHead
	for rows.Next() {
		var row domain.SourceRequestHead
		if err := rows.Scan(&row.RequestID, &row.Status, &row.CarrierCompanyID, &row.RequestingCompanyID, &row.TokenExpiresAt); err != nil {
			return nil, relationshipStoreError(err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, relationshipStoreError(err)
	}
	if out == nil {
		out = []domain.SourceRequestHead{}
	}
	return out, nil
}

func (r *RelationshipRepository) CancelRelationshipRequest(ctx context.Context, in domain.PersistCancel) (domain.PersistCancelResult, error) {
	result, err := r.cancelOnce(ctx, in)
	if errors.Is(err, errTerminalEventConflict) {
		if CancelTerminalConflictHook != nil {
			CancelTerminalConflictHook(ctx)
		}
		return r.rereadCancel(ctx, in)
	}
	return result, err
}

func (r *RelationshipRepository) cancelOnce(ctx context.Context, in domain.PersistCancel) (domain.PersistCancelResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.PersistCancelResult{}, relationshipStoreError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	root, err := loadCancelRoot(ctx, tx, in.RequestID)
	if err != nil {
		return domain.PersistCancelResult{}, err
	}
	if !cancelRootMatches(root, in) {
		return domain.PersistCancelResult{}, relationshipNotFound()
	}
	if !domain.CallerAllowsSource(in.CallerService, root.SourceType) {
		return domain.PersistCancelResult{}, apperrors.New(apperrors.CodeRelationshipRequestForbidden, "relationship command refused")
	}
	if in.AssertedCompanyID != nil && *in.AssertedCompanyID != root.RequestingCompanyID {
		return domain.PersistCancelResult{}, relationshipNotFound()
	}
	if err := authorizeCancelActor(ctx, tx, in.SourceTenantID, in.ActorUserID, root.RequestingCompanyID, root.SourceType); err != nil {
		return domain.PersistCancelResult{}, err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, cancelScope(in, root.RequestingCompanyID)); err != nil {
		return domain.PersistCancelResult{}, relationshipStoreError(err)
	}
	if replay, ok, err := readCancelIdempotency(ctx, tx, in, root.RequestingCompanyID); err != nil || ok {
		if err != nil {
			return domain.PersistCancelResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.PersistCancelResult{}, relationshipStoreError(err)
		}
		return replay, nil
	}

	var status string
	var version int
	var expired bool
	err = tx.QueryRow(ctx, `
		SELECT h.request_status, h.current_version, r.token_expires_at <= clock_timestamp()
		FROM company.carrier_relationship_request_heads h
		JOIN company.carrier_relationship_requests r ON r.relationship_request_id = h.relationship_request_id
		WHERE h.relationship_request_id = $1
		FOR UPDATE OF h
	`, root.RequestID).Scan(&status, &version, &expired)
	if err != nil {
		return domain.PersistCancelResult{}, relationshipStoreError(err)
	}
	switch status {
	case domain.RequestStatusAccepted:
		return domain.PersistCancelResult{}, apperrors.New(apperrors.CodeRelationshipRequestAlreadyAccepted, "relationship request already accepted")
	case domain.RequestStatusCancelled, domain.RequestStatusExpired:
		if err := insertCancelIdempotency(ctx, tx, in, root.RequestingCompanyID, status); err != nil {
			return domain.PersistCancelResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.PersistCancelResult{}, relationshipStoreError(err)
		}
		return domain.PersistCancelResult{RequestID: root.RequestID, Status: status}, nil
	case domain.RequestStatusPending:
	default:
		return domain.PersistCancelResult{}, relationshipStoreError(errors.New("unknown request status"))
	}

	next := domain.RequestStatusCancelled
	if expired {
		next = domain.RequestStatusExpired
	}
	if CancelTerminalInsertHook != nil {
		CancelTerminalInsertHook(ctx)
	}
	if err := writeCancelTerminal(ctx, tx, root, version+1, next, in); err != nil {
		if errors.Is(err, errTerminalEventConflict) {
			_ = tx.Rollback(ctx)
			return domain.PersistCancelResult{}, errTerminalEventConflict
		}
		return domain.PersistCancelResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.PersistCancelResult{}, relationshipStoreError(err)
	}
	return domain.PersistCancelResult{RequestID: root.RequestID, Status: next}, nil
}

func (r *RelationshipRepository) rereadCancel(ctx context.Context, in domain.PersistCancel) (domain.PersistCancelResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.PersistCancelResult{}, relationshipStoreError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	root, err := loadCancelRoot(ctx, tx, in.RequestID)
	if err != nil {
		return domain.PersistCancelResult{}, err
	}
	if !cancelRootMatches(root, in) {
		return domain.PersistCancelResult{}, relationshipNotFound()
	}
	if !domain.CallerAllowsSource(in.CallerService, root.SourceType) {
		return domain.PersistCancelResult{}, apperrors.New(apperrors.CodeRelationshipRequestForbidden, "relationship command refused")
	}
	if in.AssertedCompanyID != nil && *in.AssertedCompanyID != root.RequestingCompanyID {
		return domain.PersistCancelResult{}, relationshipNotFound()
	}
	if err := authorizeCancelActor(ctx, tx, in.SourceTenantID, in.ActorUserID, root.RequestingCompanyID, root.SourceType); err != nil {
		return domain.PersistCancelResult{}, err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, cancelScope(in, root.RequestingCompanyID)); err != nil {
		return domain.PersistCancelResult{}, relationshipStoreError(err)
	}
	if replay, ok, err := readCancelIdempotency(ctx, tx, in, root.RequestingCompanyID); err != nil || ok {
		if err != nil {
			return domain.PersistCancelResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.PersistCancelResult{}, relationshipStoreError(err)
		}
		return replay, nil
	}
	var status string
	err = tx.QueryRow(ctx, `
		SELECT request_status
		FROM company.carrier_relationship_request_heads
		WHERE relationship_request_id = $1
		FOR UPDATE
	`, root.RequestID).Scan(&status)
	if err != nil {
		return domain.PersistCancelResult{}, relationshipStoreError(err)
	}
	switch status {
	case domain.RequestStatusCancelled, domain.RequestStatusExpired:
		if err := insertCancelIdempotency(ctx, tx, in, root.RequestingCompanyID, status); err != nil {
			return domain.PersistCancelResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.PersistCancelResult{}, relationshipStoreError(err)
		}
		return domain.PersistCancelResult{RequestID: root.RequestID, Status: status}, nil
	case domain.RequestStatusAccepted:
		return domain.PersistCancelResult{}, apperrors.New(apperrors.CodeRelationshipRequestAlreadyAccepted, "relationship request already accepted")
	default:
		return domain.PersistCancelResult{}, apperrors.New(apperrors.CodeRelationshipStoreUnavailable, "relationship store unavailable")
	}
}

type cancelRoot struct {
	RequestID           uuid.UUID
	SourceType          string
	SourceID            uuid.UUID
	SourceTenantID      uuid.UUID
	RequestingCompanyID uuid.UUID
	CarrierCompanyID    uuid.UUID
}

func loadCancelRoot(ctx context.Context, tx pgx.Tx, requestID uuid.UUID) (cancelRoot, error) {
	var root cancelRoot
	err := tx.QueryRow(ctx, `
		SELECT relationship_request_id, source_type, source_id, source_tenant_id,
		       requesting_company_id, shipper_carrier_company_id
		FROM company.carrier_relationship_requests
		WHERE relationship_request_id = $1
	`, requestID).Scan(&root.RequestID, &root.SourceType, &root.SourceID, &root.SourceTenantID, &root.RequestingCompanyID, &root.CarrierCompanyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return cancelRoot{}, relationshipNotFound()
	}
	if err != nil {
		return cancelRoot{}, relationshipStoreError(err)
	}
	return root, nil
}

func cancelRootMatches(root cancelRoot, in domain.PersistCancel) bool {
	return root.SourceType == in.SourceType && root.SourceID == in.SourceID && root.SourceTenantID == in.SourceTenantID
}

func authorizeCancelActor(ctx context.Context, tx pgx.Tx, tenantID, userID, companyID uuid.UUID, sourceType string) error {
	actor, err := loadActorTx(ctx, tx, tenantID, userID, companyID)
	if err != nil {
		return err
	}
	if !actor.Active {
		return apperrors.New(apperrors.CodeRelationshipRequestForbidden, "relationship command refused")
	}
	if !actor.ActiveMembership {
		return relationshipNotFound()
	}
	if !domain.CanCreateRelationshipRequest(actor, sourceType) {
		return apperrors.New(apperrors.CodeRelationshipRequestForbidden, "relationship command refused")
	}
	return nil
}

func loadActorTx(ctx context.Context, tx pgx.Tx, tenantID, userID, companyID uuid.UUID) (domain.ActorAuthority, error) {
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
	err := tx.QueryRow(ctx, query, userID, tenantID, companyID).Scan(&actor.ActiveMembership, &actor.CompanyRoles, &actor.TenantGlobalPlatformAdmin)
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

func readCancelIdempotency(ctx context.Context, tx pgx.Tx, in domain.PersistCancel, companyID uuid.UUID) (domain.PersistCancelResult, bool, error) {
	var storedRequest uuid.UUID
	var storedHash []byte
	var storedStatus string
	err := tx.QueryRow(ctx, `
		SELECT relationship_request_id, body_hash, request_status
		FROM company.carrier_relationship_cancel_idempotency
		WHERE caller_service = $1 AND source_tenant_id = $2 AND requesting_company_id = $3 AND idempotency_key = $4
	`, in.CallerService, in.SourceTenantID, companyID, in.IdempotencyKey).Scan(&storedRequest, &storedHash, &storedStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PersistCancelResult{}, false, nil
	}
	if err != nil {
		return domain.PersistCancelResult{}, false, relationshipStoreError(err)
	}
	if storedRequest != in.RequestID || !hmac.Equal(storedHash, domain.CancelBodyHash(in.RequestID)) {
		return domain.PersistCancelResult{}, false, apperrors.New(apperrors.CodeRelationshipIdempotencyConflict, "idempotency conflict")
	}
	return domain.PersistCancelResult{RequestID: storedRequest, Status: storedStatus}, true, nil
}

func insertCancelIdempotency(ctx context.Context, tx pgx.Tx, in domain.PersistCancel, companyID uuid.UUID, status string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO company.carrier_relationship_cancel_idempotency (
			caller_service, source_tenant_id, requesting_company_id, idempotency_key,
			relationship_request_id, body_hash, request_status
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, in.CallerService, in.SourceTenantID, companyID, in.IdempotencyKey, in.RequestID, domain.CancelBodyHash(in.RequestID), status)
	if err != nil {
		return relationshipStoreError(err)
	}
	return nil
}

func writeCancelTerminal(ctx context.Context, tx pgx.Tx, root cancelRoot, version int, status string, in domain.PersistCancel) error {
	eventID := uuid.New()
	var err error
	if status == domain.RequestStatusCancelled {
		_, err = tx.Exec(ctx, `
			INSERT INTO company.carrier_relationship_request_events (
				event_id, relationship_request_id, request_version, event_type, request_status, actor_user_id,
				cancelled_at, cancelled_by, cancel_reason
			)
			SELECT $1, $2, $3, 'CANCELLED', 'CANCELLED', $4, ts.t, $4, NULLIF($5, '')
			FROM (SELECT clock_timestamp() AS t) ts
		`, eventID, root.RequestID, version, in.ActorUserID, in.Reason)
	} else {
		_, err = tx.Exec(ctx, `
			INSERT INTO company.carrier_relationship_request_events (
				event_id, relationship_request_id, request_version, event_type, request_status, actor_user_id
			) VALUES ($1, $2, $3, 'EXPIRED', 'EXPIRED', $4)
		`, eventID, root.RequestID, version, in.ActorUserID)
	}
	if isTerminalEventConflict(err) {
		return errTerminalEventConflict
	}
	if err != nil {
		return relationshipStoreError(err)
	}
	_, err = tx.Exec(ctx, `
		UPDATE company.carrier_relationship_request_heads
		SET current_event_id = $2, current_version = $3, request_status = $4
		WHERE relationship_request_id = $1
	`, root.RequestID, eventID, version, status)
	if err != nil {
		return relationshipStoreError(err)
	}
	action := "relationship_request_cancelled"
	outboxType := "carrier_relationship.cancelled"
	if status == domain.RequestStatusExpired {
		action = "relationship_request_expired"
		outboxType = "carrier_relationship.expired"
	}
	requestID := root.RequestID
	sourceID := root.SourceID
	if err := insertAudit(ctx, tx, auditRow{
		ActorUserID: in.ActorUserID, ActorTenantID: in.SourceTenantID, SelectedCompanyID: root.RequestingCompanyID,
		RequestID: &requestID, SourceType: root.SourceType, SourceID: &sourceID,
		IdempotencyKey: in.IdempotencyKey, CorrelationID: in.CorrelationID, Reason: in.Reason, Action: action,
	}); err != nil {
		return err
	}
	if err := insertOutbox(ctx, tx, outboxType, root.RequestID, map[string]any{
		"relationship_request_id": root.RequestID.String(),
		"source_type":             root.SourceType,
		"source_id":               root.SourceID.String(),
		"source_tenant_id":        root.SourceTenantID.String(),
		"request_status":          status,
	}); err != nil {
		return err
	}
	return insertCancelIdempotency(ctx, tx, in, root.RequestingCompanyID, status)
}

func cancelScope(in domain.PersistCancel, companyID uuid.UUID) string {
	return "cancel:" + in.CallerService + ":" + in.SourceTenantID.String() + ":" + companyID.String() + ":" + in.IdempotencyKey
}

func isTerminalEventConflict(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "uq_crr_one_terminal_event", "uq_crr_event_version":
			return true
		}
	}
	return false
}

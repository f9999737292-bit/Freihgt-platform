package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

var ErrIdempotencyRecordActive = errors.New("idempotency record not expired")

type IdempotencyRepository struct {
	pool   *pgxpool.Pool
	exec   dbExecutor
	inject *testInjectState
}

type IdempotencyScope struct {
	TenantID               uuid.UUID
	ActorID                uuid.UUID
	IntegrationPrincipalID uuid.UUID
	OwnerKind              string
	Operation              string
	AggregateScope         uuid.UUID
}

type IdempotencyRecord struct {
	ID                     uuid.UUID
	TenantID               uuid.UUID
	ActorID                uuid.UUID
	IntegrationPrincipalID uuid.UUID
	OwnerKind              string
	Operation              string
	AggregateScope         uuid.UUID
	IdempotencyKey         string
	RequestBodyHash        string
	ResponseStatus         int
	ResponseBody           json.RawMessage
	CreatedAt              time.Time
	ExpiresAt              time.Time
}

func NewIdempotencyRepository(pool *pgxpool.Pool) *IdempotencyRepository {
	return &IdempotencyRepository{pool: pool, inject: &testInjectState{}}
}

// SetInjectStoreFailure arms a one-shot idempotency insert failure for integration tests.
func (r *IdempotencyRepository) SetInjectStoreFailure(enabled bool) {
	r.inject.set(enabled)
}

func (r *IdempotencyRepository) Get(ctx context.Context, scope IdempotencyScope, key string) (*IdempotencyRecord, error) {
	if err := validateIdempotencyScope(scope); err != nil {
		return nil, err
	}
	switch resolvedOwnerKind(scope) {
	case domain.OwnerKindIntegrationPrincipal:
		row := r.db().QueryRow(ctx, `
			SELECT id, tenant_id, actor_id, integration_principal_id, operation, aggregate_scope, idempotency_key,
				request_body_hash, response_status, response_body_json, created_at, expires_at
			FROM rfx.rfx_idempotency_records
			WHERE tenant_id = $1 AND integration_principal_id = $2 AND operation = $3 AND aggregate_scope = $4
				AND idempotency_key = $5 AND expires_at > now()
		`, scope.TenantID, scope.IntegrationPrincipalID, scope.Operation, scope.AggregateScope, key)
		return scanIdempotencyRecord(row)
	default:
		row := r.db().QueryRow(ctx, `
			SELECT id, tenant_id, actor_id, integration_principal_id, operation, aggregate_scope, idempotency_key,
				request_body_hash, response_status, response_body_json, created_at, expires_at
			FROM rfx.rfx_idempotency_records
			WHERE tenant_id = $1 AND actor_id = $2 AND operation = $3 AND aggregate_scope = $4
				AND idempotency_key = $5 AND expires_at > now()
		`, scope.TenantID, scope.ActorID, scope.Operation, scope.AggregateScope, key)
		return scanIdempotencyRecord(row)
	}
}

func (r *IdempotencyRepository) Store(ctx context.Context, record IdempotencyRecord) error {
	if err := validateIdempotencyRecord(record); err != nil {
		return err
	}
	if r.inject.consume() {
		return mapDBError(fmt.Errorf("injected idempotency store failure"))
	}
	if len(record.ResponseBody) == 0 {
		record.ResponseBody = json.RawMessage(`{}`)
	}
	switch resolvedOwnerKind(idempotencyScopeFromRecord(record)) {
	case domain.OwnerKindIntegrationPrincipal:
		tag, err := r.db().Exec(ctx, `
			INSERT INTO rfx.rfx_idempotency_records (
				tenant_id, integration_principal_id, operation, aggregate_scope, idempotency_key,
				request_body_hash, response_status, response_body_json, expires_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9)
			ON CONFLICT (tenant_id, integration_principal_id, operation, aggregate_scope, idempotency_key)
			WHERE integration_principal_id IS NOT NULL
			DO UPDATE SET
				request_body_hash = EXCLUDED.request_body_hash,
				response_status = EXCLUDED.response_status,
				response_body_json = EXCLUDED.response_body_json,
				created_at = now(),
				expires_at = EXCLUDED.expires_at
			WHERE rfx.rfx_idempotency_records.expires_at <= now()
		`,
			record.TenantID,
			record.IntegrationPrincipalID,
			record.Operation,
			record.AggregateScope,
			record.IdempotencyKey,
			record.RequestBodyHash,
			record.ResponseStatus,
			string(record.ResponseBody),
			record.ExpiresAt,
		)
		if err != nil {
			return mapDBError(err)
		}
		if tag.RowsAffected() == 0 {
			return ErrIdempotencyRecordActive
		}
		return nil
	default:
		tag, err := r.db().Exec(ctx, `
			INSERT INTO rfx.rfx_idempotency_records (
				tenant_id, actor_id, operation, aggregate_scope, idempotency_key,
				request_body_hash, response_status, response_body_json, expires_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9)
			ON CONFLICT (tenant_id, actor_id, operation, aggregate_scope, idempotency_key)
			WHERE actor_id IS NOT NULL
			DO UPDATE SET
				request_body_hash = EXCLUDED.request_body_hash,
				response_status = EXCLUDED.response_status,
				response_body_json = EXCLUDED.response_body_json,
				created_at = now(),
				expires_at = EXCLUDED.expires_at
			WHERE rfx.rfx_idempotency_records.expires_at <= now()
		`,
			record.TenantID,
			record.ActorID,
			record.Operation,
			record.AggregateScope,
			record.IdempotencyKey,
			record.RequestBodyHash,
			record.ResponseStatus,
			string(record.ResponseBody),
			record.ExpiresAt,
		)
		if err != nil {
			return mapDBError(err)
		}
		if tag.RowsAffected() == 0 {
			return ErrIdempotencyRecordActive
		}
		return nil
	}
}

func validateIdempotencyScope(scope IdempotencyScope) error {
	if scope.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if scope.Operation == "" {
		return apperrors.Validation("operation is required", map[string]any{"field": "operation"})
	}
	switch resolvedOwnerKind(scope) {
	case domain.OwnerKindIntegrationPrincipal:
		if scope.IntegrationPrincipalID == uuid.Nil {
			return apperrors.Validation("integration_principal_id is required", map[string]any{"field": "integration_principal_id"})
		}
		if scope.ActorID != uuid.Nil {
			return apperrors.Validation("actor_id must be empty for integration principal idempotency", map[string]any{"field": "actor_id"})
		}
	default:
		if scope.ActorID == uuid.Nil {
			return apperrors.Validation("actor_id is required", map[string]any{"field": "actor_id"})
		}
		if scope.IntegrationPrincipalID != uuid.Nil {
			return apperrors.Validation("integration_principal_id must be empty for human idempotency", map[string]any{"field": "integration_principal_id"})
		}
	}
	return nil
}

func validateIdempotencyRecord(record IdempotencyRecord) error {
	return validateIdempotencyScope(idempotencyScopeFromRecord(record))
}

func resolvedOwnerKind(scope IdempotencyScope) string {
	if scope.OwnerKind != "" {
		return scope.OwnerKind
	}
	if scope.IntegrationPrincipalID != uuid.Nil {
		return domain.OwnerKindIntegrationPrincipal
	}
	return domain.OwnerKindHuman
}

func idempotencyScopeFromRecord(record IdempotencyRecord) IdempotencyScope {
	return IdempotencyScope{
		TenantID:               record.TenantID,
		ActorID:                record.ActorID,
		IntegrationPrincipalID: record.IntegrationPrincipalID,
		OwnerKind:              record.OwnerKind,
		Operation:              record.Operation,
		AggregateScope:         record.AggregateScope,
	}
}

func scanIdempotencyRecord(row interface{ Scan(dest ...any) error }) (*IdempotencyRecord, error) {
	var record IdempotencyRecord
	var actorID *uuid.UUID
	var integrationPrincipalID *uuid.UUID
	if err := row.Scan(
		&record.ID,
		&record.TenantID,
		&actorID,
		&integrationPrincipalID,
		&record.Operation,
		&record.AggregateScope,
		&record.IdempotencyKey,
		&record.RequestBodyHash,
		&record.ResponseStatus,
		&record.ResponseBody,
		&record.CreatedAt,
		&record.ExpiresAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapDBError(err)
	}
	if actorID != nil {
		record.ActorID = *actorID
	}
	if integrationPrincipalID != nil {
		record.IntegrationPrincipalID = *integrationPrincipalID
	}
	if record.IntegrationPrincipalID != uuid.Nil {
		record.OwnerKind = domain.OwnerKindIntegrationPrincipal
	} else {
		record.OwnerKind = domain.OwnerKindHuman
	}
	if len(record.ResponseBody) == 0 {
		record.ResponseBody = json.RawMessage(`{}`)
	}
	return &record, nil
}

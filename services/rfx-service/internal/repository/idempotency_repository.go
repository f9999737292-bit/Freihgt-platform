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
)

var ErrIdempotencyRecordActive = errors.New("idempotency record not expired")

type IdempotencyRepository struct {
	pool *pgxpool.Pool
	exec dbExecutor
	// injectStoreFailure is set only by integration tests to verify transactional rollback.
	injectStoreFailure bool
}

type IdempotencyScope struct {
	TenantID       uuid.UUID
	ActorID        uuid.UUID
	Operation      string
	AggregateScope uuid.UUID
}

type IdempotencyRecord struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	ActorID         uuid.UUID
	Operation       string
	AggregateScope  uuid.UUID
	IdempotencyKey  string
	RequestBodyHash string
	ResponseStatus  int
	ResponseBody    json.RawMessage
	CreatedAt       time.Time
	ExpiresAt       time.Time
}

func NewIdempotencyRepository(pool *pgxpool.Pool) *IdempotencyRepository {
	return &IdempotencyRepository{pool: pool}
}

// SetInjectStoreFailure enables a one-shot idempotency insert failure for integration tests.
func (r *IdempotencyRepository) SetInjectStoreFailure(enabled bool) {
	r.injectStoreFailure = enabled
}

func (r *IdempotencyRepository) Get(ctx context.Context, scope IdempotencyScope, key string) (*IdempotencyRecord, error) {
	row := r.db().QueryRow(ctx, `
		SELECT id, tenant_id, actor_id, operation, aggregate_scope, idempotency_key,
			request_body_hash, response_status, response_body_json, created_at, expires_at
		FROM rfx.rfx_idempotency_records
		WHERE tenant_id = $1 AND actor_id = $2 AND operation = $3 AND aggregate_scope = $4
			AND idempotency_key = $5 AND expires_at > now()
	`, scope.TenantID, scope.ActorID, scope.Operation, scope.AggregateScope, key)
	record, err := scanIdempotencyRecord(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapDBError(err)
	}
	return record, nil
}

func (r *IdempotencyRepository) Store(ctx context.Context, record IdempotencyRecord) error {
	if r.injectStoreFailure {
		return mapDBError(fmt.Errorf("injected idempotency store failure"))
	}
	if len(record.ResponseBody) == 0 {
		record.ResponseBody = json.RawMessage(`{}`)
	}
	tag, err := r.db().Exec(ctx, `
		INSERT INTO rfx.rfx_idempotency_records (
			tenant_id, actor_id, operation, aggregate_scope, idempotency_key,
			request_body_hash, response_status, response_body_json, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9)
		ON CONFLICT (tenant_id, actor_id, operation, aggregate_scope, idempotency_key)
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

func scanIdempotencyRecord(row interface{ Scan(dest ...any) error }) (*IdempotencyRecord, error) {
	var record IdempotencyRecord
	if err := row.Scan(
		&record.ID,
		&record.TenantID,
		&record.ActorID,
		&record.Operation,
		&record.AggregateScope,
		&record.IdempotencyKey,
		&record.RequestBodyHash,
		&record.ResponseStatus,
		&record.ResponseBody,
		&record.CreatedAt,
		&record.ExpiresAt,
	); err != nil {
		return nil, err
	}
	if len(record.ResponseBody) == 0 {
		record.ResponseBody = json.RawMessage(`{}`)
	}
	return &record, nil
}

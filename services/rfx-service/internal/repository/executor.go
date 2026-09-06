package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type dbExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *RfxRepository) db() dbExecutor {
	if r.exec != nil {
		return r.exec
	}
	return r.pool
}

func (r *AuditRepository) db() dbExecutor {
	if r.exec != nil {
		return r.exec
	}
	return r.pool
}

func (r *BidRepository) db() dbExecutor {
	if r.exec != nil {
		return r.exec
	}
	return r.pool
}

func (r *FreightRequestRepository) db() dbExecutor {
	if r.exec != nil {
		return r.exec
	}
	return r.pool
}

// WithTx returns a repository bound to an open transaction.
func (r *RfxRepository) WithTx(tx pgx.Tx) *RfxRepository {
	return &RfxRepository{pool: r.pool, exec: tx}
}

func (r *AuditRepository) WithTx(tx pgx.Tx) *AuditRepository {
	return &AuditRepository{pool: r.pool, exec: tx, injectRecordFailure: r.injectRecordFailure}
}

func (r *BidRepository) WithTx(tx pgx.Tx) *BidRepository {
	return &BidRepository{pool: r.pool, exec: tx}
}

func (r *FreightRequestRepository) WithTx(tx pgx.Tx) *FreightRequestRepository {
	return &FreightRequestRepository{pool: r.pool, exec: tx}
}

func (r *QuestionnaireRepository) WithTx(tx pgx.Tx) *QuestionnaireRepository {
	return &QuestionnaireRepository{pool: r.pool, exec: tx}
}

func (r *AnswerRepository) WithTx(tx pgx.Tx) *AnswerRepository {
	return &AnswerRepository{pool: r.pool, exec: tx}
}

// TransactionRunner executes callbacks inside a database transaction.
type TransactionRunner struct {
	pool *pgxpool.Pool
}

func NewTransactionRunner(pool *pgxpool.Pool) *TransactionRunner {
	return &TransactionRunner{pool: pool}
}

func (t *TransactionRunner) Run(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error {
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return mapDBError(err)
	}
	defer tx.Rollback(ctx)
	if err := fn(ctx, tx); err != nil {
		return err
	}
	return mapDBError(tx.Commit(ctx))
}

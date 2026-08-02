package rebuild

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	ProjectionRebuildAdvisoryLockKey int64 = 0x4354505350524F4A // "CTPSPROJ" documented constant
)

const advisoryLockSharedSQL = `SELECT pg_advisory_xact_lock_shared($1)`
const advisoryLockExclusiveSQL = `SELECT pg_advisory_xact_lock($1)`

func AcquireProjectionSharedLock(ctx context.Context, tx pgx.Tx) error {
	return acquireProjectionLock(ctx, tx, advisoryLockSharedSQL)
}

func AcquireProjectionExclusiveLock(ctx context.Context, tx pgx.Tx) error {
	return acquireProjectionLock(ctx, tx, advisoryLockExclusiveSQL)
}

func acquireProjectionLock(ctx context.Context, tx pgx.Tx, sql string) error {
	if err := setLockTimeoutFromContext(ctx, tx); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, sql, ProjectionRebuildAdvisoryLockKey)
	if err != nil {
		return mapLockError(ctx, err)
	}
	return nil
}

func setLockTimeoutFromContext(ctx context.Context, tx pgx.Tx) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		return nil
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return context.DeadlineExceeded
	}
	ms := remaining.Milliseconds()
	if ms < 1 {
		ms = 1
	}
	if ms > 2147483647 {
		ms = 2147483647
	}
	_, err := tx.Exec(ctx, `SELECT set_config('lock_timeout', $1, true)`, formatLockTimeoutMS(ms))
	return err
}

func formatLockTimeoutMS(ms int64) string {
	// Bounded integer milliseconds only — safe for set_config.
	return time.Duration(ms * int64(time.Millisecond)).String()
}

func mapLockError(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		return ctx.Err()
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return newActivationError(CodeProjectionLockTimeout, ctx.Err())
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
		return newActivationError(CodeProjectionLockTimeout, err)
	}
	return err
}

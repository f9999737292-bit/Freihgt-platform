package rebuild

import (
	"context"

	"github.com/jackc/pgx/v5"
)

const (
	ProjectionRebuildAdvisoryLockKey int64 = 0x4354505350524F4A // "CTPSPROJ" documented constant
)

const advisoryLockSharedSQL = `SELECT pg_advisory_xact_lock_shared($1)`
const advisoryLockExclusiveSQL = `SELECT pg_advisory_xact_lock($1)`

func AcquireProjectionSharedLock(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, advisoryLockSharedSQL, ProjectionRebuildAdvisoryLockKey)
	return err
}

func AcquireProjectionExclusiveLock(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, advisoryLockExclusiveSQL, ProjectionRebuildAdvisoryLockKey)
	return err
}

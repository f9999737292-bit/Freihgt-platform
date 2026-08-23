package repository

import (
	"context"
	"hash/fnv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const analyticsProjectionLockNamespace int64 = 0x4643415050524F4A // FCAPPROJ

func tenantAnalyticsLockKey(tenantID uuid.UUID) int64 {
	h := fnv.New64a()
	_, _ = h.Write(tenantID[:])
	return analyticsProjectionLockNamespace ^ int64(h.Sum64())
}

func AcquireTenantAnalyticsExclusiveLock(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, tenantAnalyticsLockKey(tenantID))
	return err
}

//go:build integration

package rebuild

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	apprebuild "github.com/freight-platform/control-tower-read-model-service/internal/rebuild"
)

func TestActivationTenant(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	tenantA, tenantB := uuid.New(), uuid.New()
	shipA1, shipA2 := uuid.New(), uuid.New()
	shipB1 := uuid.New()

	insertLegacyProjection(t, pool, tenantA, shipA1, 3, "CARRIER_ASSIGNED")
	insertLegacyProjection(t, pool, tenantA, shipA2, 4, "LOADED")
	insertLegacyProjection(t, pool, tenantB, shipB1, 2, "IN_TRANSIT")

	beforeB := snapshotAllProjections(t, pool, tenantB)

	actRepo := apprebuild.NewActivationRepository(pool)
	snapshotID := importSnapshot(t, pool, buildTenantScopedStream(t, tenantA, 2))
	result, err := actRepo.Activate(ctx, snapshotID)
	require.NoError(t, err)
	require.Equal(t, int64(2), result.ActivatedRows)
	require.Equal(t, int64(2), result.BackupRows)

	var backupTenantA, backupTenantB int64
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM control_tower.shipment_status_projection_rebuild_backup
WHERE snapshot_id=$1 AND tenant_id=$2`, snapshotID, tenantA).Scan(&backupTenantA))
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM control_tower.shipment_status_projection_rebuild_backup
WHERE snapshot_id=$1 AND tenant_id=$2`, snapshotID, tenantB).Scan(&backupTenantB))
	require.Equal(t, int64(2), backupTenantA)
	require.Equal(t, int64(0), backupTenantB)

	afterB := snapshotAllProjections(t, pool, tenantB)
	requireProjectionsEqual(t, beforeB, afterB)

	var activeA int64
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM control_tower.shipment_status_projection
WHERE tenant_id=$1 AND snapshot_id=$2`, tenantA, snapshotID).Scan(&activeA))
	require.Equal(t, int64(2), activeA)
}

func TestRollbackTenant(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	tenantA, tenantB := uuid.New(), uuid.New()
	shipA := uuid.New()
	shipB := uuid.New()

	insertLegacyProjection(t, pool, tenantA, shipA, 5, "IN_TRANSIT")
	insertLegacyProjection(t, pool, tenantB, shipB, 1, "CARRIER_ASSIGNED")
	beforeA := snapshotAllProjections(t, pool, tenantA)
	beforeB := snapshotAllProjections(t, pool, tenantB)

	actRepo := apprebuild.NewActivationRepository(pool)
	snapshotID := importSnapshot(t, pool, buildTenantScopedStream(t, tenantA, 1))
	_, err := actRepo.Activate(ctx, snapshotID)
	require.NoError(t, err)

	_, err = actRepo.Rollback(ctx, snapshotID)
	require.NoError(t, err)

	afterA := snapshotAllProjections(t, pool, tenantA)
	afterB := snapshotAllProjections(t, pool, tenantB)
	requireProjectionsEqual(t, beforeA, afterA)
	requireProjectionsEqual(t, beforeB, afterB)
}

func TestActivationEmptyTenant(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	tenantID := uuid.New()
	actRepo := apprebuild.NewActivationRepository(pool)
	snapshotID := importSnapshot(t, pool, buildTenantScopedStream(t, tenantID, 3))

	result, err := actRepo.Activate(ctx, snapshotID)
	require.NoError(t, err)
	require.Equal(t, int64(3), result.ActivatedRows)
	require.Equal(t, int64(0), result.BackupRows)
	require.Equal(t, int64(0), countBackupRows(t, pool, snapshotID))

	rollback, err := actRepo.Rollback(ctx, snapshotID)
	require.NoError(t, err)
	require.Equal(t, int64(0), rollback.RestoredRows)

	var tenantRows int64
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM control_tower.shipment_status_projection WHERE tenant_id=$1`, tenantID).Scan(&tenantRows))
	require.Equal(t, int64(0), tenantRows)
}

func TestActivationReplaceTenantWithEmpty(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	tenantID := uuid.New()
	ship1, ship2 := uuid.New(), uuid.New()
	insertLegacyProjection(t, pool, tenantID, ship1, 2, "CARRIER_ASSIGNED")
	insertLegacyProjection(t, pool, tenantID, ship2, 3, "LOADED")
	before := snapshotAllProjections(t, pool, tenantID)

	actRepo := apprebuild.NewActivationRepository(pool)
	snapshotID := importSnapshot(t, pool, buildEmptyTenantStream(t, tenantID))
	result, err := actRepo.Activate(ctx, snapshotID)
	require.NoError(t, err)
	require.Equal(t, int64(0), result.ActivatedRows)
	require.Equal(t, int64(2), result.BackupRows)

	var active int64
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM control_tower.shipment_status_projection WHERE tenant_id=$1`, tenantID).Scan(&active))
	require.Equal(t, int64(0), active)

	_, err = actRepo.Rollback(ctx, snapshotID)
	require.NoError(t, err)
	after := snapshotAllProjections(t, pool, tenantID)
	requireProjectionsEqual(t, before, after)
}

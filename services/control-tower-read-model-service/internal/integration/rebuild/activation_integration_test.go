//go:build integration

package rebuild

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	apprebuild "github.com/freight-platform/control-tower-read-model-service/internal/rebuild"
)

func TestMigration000017ActivationColumns(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	var colExists bool
	require.NoError(t, pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1 FROM information_schema.columns
  WHERE table_schema='control_tower' AND table_name='shipment_status_projection_rebuild_job'
    AND column_name='rollback_eligible'
)`).Scan(&colExists))
	require.True(t, colExists)
}

func TestActivationALLScope(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	repo := apprebuild.NewRepository(pool)
	actRepo := apprebuild.NewActivationRepository(pool)
	stream := buildIntegrationStream(t, 3)
	require.NoError(t, apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(stream), 100))
	id := extractSnapshotID(t, stream)

	result, err := actRepo.Activate(ctx, id)
	require.NoError(t, err)
	require.Equal(t, apprebuild.StateActive, result.State)
	require.Equal(t, int64(3), result.ActivatedRows)

	var activeCount int64
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM control_tower.shipment_status_projection WHERE snapshot_id=$1`, id).Scan(&activeCount))
	require.Equal(t, int64(3), activeCount)
}

func TestActivationEmptyALLScope(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	repo := apprebuild.NewRepository(pool)
	actRepo := apprebuild.NewActivationRepository(pool)
	stream := buildEmptyAllStream(t)
	require.NoError(t, apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(stream), 100))
	id := extractSnapshotID(t, stream)

	result, err := actRepo.Activate(ctx, id)
	require.NoError(t, err)
	require.Equal(t, int64(0), result.ActivatedRows)
	require.Equal(t, int64(0), result.BackupRows)
}

func TestActivationInvalidStateRejected(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	actRepo := apprebuild.NewActivationRepository(pool)
	_, err := actRepo.Activate(ctx, uuid.New())
	require.Error(t, err)
}

func TestActivationIdempotentSecondCall(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	repo := apprebuild.NewRepository(pool)
	actRepo := apprebuild.NewActivationRepository(pool)
	stream := buildIntegrationStream(t, 1)
	require.NoError(t, apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(stream), 100))
	id := extractSnapshotID(t, stream)
	_, err := actRepo.Activate(ctx, id)
	require.NoError(t, err)
	_, err = actRepo.Activate(ctx, id)
	require.Error(t, err)
	require.Equal(t, apprebuild.CodeSnapshotAlreadyActive, apprebuild.ActivationErrorCode(err))
}

func TestRollbackALLScope(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	tenantID, shipmentID := uuid.New(), uuid.New()
	eventID, sourceID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO control_tower.shipment_status_projection (
    tenant_id, shipment_id, shipment_version, current_status, last_event_id, last_source_event_id,
    last_event_type, last_occurred_at, last_consumed_at, complete, gap_detected, created_at, updated_at,
    projection_source
) VALUES ($1,$2,1,'CARRIER_ASSIGNED',$3,$4,'shipment.created',NOW(),NOW(),TRUE,FALSE,NOW(),NOW(),'LIVE_EVENT')`,
		tenantID, shipmentID, eventID, sourceID)
	require.NoError(t, err)

	repo := apprebuild.NewRepository(pool)
	actRepo := apprebuild.NewActivationRepository(pool)
	stream := buildIntegrationStreamForTenant(t, tenantID, 2)
	require.NoError(t, apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(stream), 100))
	id := extractSnapshotID(t, stream)
	_, err = actRepo.Activate(ctx, id)
	require.NoError(t, err)
	result, err := actRepo.Rollback(ctx, id)
	require.NoError(t, err)
	require.Equal(t, apprebuild.StateRolledBack, result.State)
	require.Equal(t, int64(1), result.RestoredRows)

	var restored int64
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM control_tower.shipment_status_projection WHERE tenant_id=$1 AND shipment_id=$2`,
		tenantID, shipmentID).Scan(&restored))
	require.Equal(t, int64(1), restored)
}

func TestRollbackAfterLiveUpdateRejected(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	repo := apprebuild.NewRepository(pool)
	actRepo := apprebuild.NewActivationRepository(pool)
	stream := buildIntegrationStream(t, 1)
	require.NoError(t, apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(stream), 100))
	id := extractSnapshotID(t, stream)
	_, err := actRepo.Activate(ctx, id)
	require.NoError(t, err)

	var tenantID, shipmentID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `
SELECT tenant_id, shipment_id FROM control_tower.shipment_status_projection WHERE snapshot_id=$1 LIMIT 1`,
		id).Scan(&tenantID, &shipmentID))

	_, err = pool.Exec(ctx, `
UPDATE control_tower.shipment_status_projection
SET projection_source='LIVE_EVENT', snapshot_id=NULL, shipment_version=shipment_version+1
WHERE tenant_id=$1 AND shipment_id=$2`, tenantID, shipmentID)
	require.NoError(t, err)

	_, err = actRepo.Rollback(ctx, id)
	require.Error(t, err)
	require.Equal(t, apprebuild.CodeRollbackWindowClosed, apprebuild.ActivationErrorCode(err))
}

func TestAdvisoryLockSharedBlocksExclusive(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	txA, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = txA.Rollback(ctx) })
	require.NoError(t, apprebuild.AcquireProjectionSharedLock(ctx, txA))

	blocker := make(chan struct{})
	acquired := make(chan error, 1)
	go func() {
		<-blocker
		txB, err := pool.Begin(ctx)
		if err != nil {
			acquired <- err
			return
		}
		defer txB.Rollback(ctx)
		err = apprebuild.AcquireProjectionExclusiveLock(ctx, txB)
		acquired <- err
		if err == nil {
			_ = txB.Commit(ctx)
		}
	}()

	lockCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	txBlocked, err := pool.Begin(lockCtx)
	require.NoError(t, err)
	err = apprebuild.AcquireProjectionExclusiveLock(lockCtx, txBlocked)
	require.Error(t, err)
	require.Equal(t, apprebuild.CodeProjectionLockTimeout, apprebuild.ActivationErrorCode(err))
	_ = txBlocked.Rollback(context.Background())

	require.NoError(t, txA.Commit(ctx))
	close(blocker)

	select {
	case err := <-acquired:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("exclusive lock should acquire after shared release")
	}
}

func TestActivationFailurePreservesOldProjection(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	tenantID := uuid.New()
	oldShip := uuid.New()
	eventID, sourceID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO control_tower.shipment_status_projection (
    tenant_id, shipment_id, shipment_version, current_status, last_event_id, last_source_event_id,
    last_event_type, last_occurred_at, last_consumed_at, complete, gap_detected,
    gap_from_version, gap_to_version, created_at, updated_at
) VALUES ($1,$2,5,'IN_TRANSIT',$3,$4,'shipment.status.changed',NOW(),NOW(),FALSE,TRUE,1,4,NOW(),NOW())`,
		tenantID, oldShip, eventID, sourceID)
	require.NoError(t, err)

	apprebuild.SetActivationFailureHookForTest(apprebuild.FailPointAfterDelete)
	t.Cleanup(func() { apprebuild.SetActivationFailureHookForTest("") })

	repo := apprebuild.NewRepository(pool)
	actRepo := apprebuild.NewActivationRepository(pool)
	stream := buildIntegrationStreamForTenant(t, tenantID, 1)
	require.NoError(t, apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(stream), 100))
	id := extractSnapshotID(t, stream)

	_, err = actRepo.Activate(ctx, id)
	require.Error(t, err)

	var version int
	require.NoError(t, pool.QueryRow(ctx, `
SELECT shipment_version FROM control_tower.shipment_status_projection WHERE tenant_id=$1 AND shipment_id=$2`,
		tenantID, oldShip).Scan(&version))
	require.Equal(t, 5, version)

	job, err := repo.GetJobStatus(ctx, id)
	require.NoError(t, err)
	require.Equal(t, apprebuild.StateValidated, job.State)
}

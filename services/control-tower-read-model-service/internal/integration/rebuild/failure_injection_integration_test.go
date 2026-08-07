//go:build integration

package rebuild

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	apprebuild "github.com/freight-platform/control-tower-read-model-service/internal/rebuild"
)

func TestActivationFailurePointsPreserveProjection(t *testing.T) {
	points := []string{
		apprebuild.FailPointAfterJobLock,
		apprebuild.FailPointAfterBackup,
		apprebuild.FailPointAfterDelete,
		apprebuild.FailPointAfterInsert,
		apprebuild.FailPointBeforePostValidate,
		apprebuild.FailPointBeforeActive,
	}
	for _, point := range points {
		t.Run(point, func(t *testing.T) {
			ctx := context.Background()
			pool := setupMigrationDB(t)
			t.Cleanup(pool.Close)

			tenantID := uuid.New()
			oldShip := uuid.New()
			insertLegacyProjection(t, pool, tenantID, oldShip, 5, "IN_TRANSIT")

			before := snapshotAllProjections(t, pool, tenantID)
			inboxBefore := countInbox(t, pool)
			dlBefore := countDeadLetter(t, pool)

			apprebuild.SetActivationFailureHookForTest(point)
			t.Cleanup(func() { apprebuild.SetActivationFailureHookForTest("") })

			snapshotID := importSnapshot(t, pool, buildIntegrationStreamForTenant(t, tenantID, 1))
			actRepo := apprebuild.NewActivationRepository(pool)
			_, err := actRepo.Activate(ctx, snapshotID)
			require.Error(t, err)

			job, err := apprebuild.NewRepository(pool).GetJobStatus(ctx, snapshotID)
			require.NoError(t, err)
			require.Equal(t, apprebuild.StateValidated, job.State)
			require.Equal(t, int64(0), countBackupRows(t, pool, snapshotID))

			after := snapshotAllProjections(t, pool, tenantID)
			requireProjectionsEqual(t, before, after)
			require.Equal(t, inboxBefore, countInbox(t, pool))
			require.Equal(t, dlBefore, countDeadLetter(t, pool))
		})
	}
}

func TestRollbackFailurePointsKeepActive(t *testing.T) {
	points := []string{
		apprebuild.FailPointRollbackAfterState,
		apprebuild.FailPointRollbackAfterDelete,
		apprebuild.FailPointRollbackAfterInsert,
		apprebuild.FailPointRollbackBeforeValidate,
		apprebuild.FailPointRollbackBeforeRolledBack,
	}
	for _, point := range points {
		t.Run(point, func(t *testing.T) {
			ctx := context.Background()
			pool := setupMigrationDB(t)
			t.Cleanup(pool.Close)

			tenantID := uuid.New()
			insertLegacyProjection(t, pool, tenantID, uuid.New(), 1, "CARRIER_ASSIGNED")
			snapshotID := importSnapshot(t, pool, buildTenantScopedStream(t, tenantID, 1))
			actRepo := apprebuild.NewActivationRepository(pool)
			_, err := actRepo.Activate(ctx, snapshotID)
			require.NoError(t, err)

			apprebuild.SetActivationFailureHookForTest(point)
			t.Cleanup(func() { apprebuild.SetActivationFailureHookForTest("") })

			_, err = actRepo.Rollback(ctx, snapshotID)
			require.Error(t, err)

			job, err := apprebuild.NewRepository(pool).GetJobStatus(ctx, snapshotID)
			require.NoError(t, err)
			require.Equal(t, apprebuild.StateActive, job.State)
		})
	}
}

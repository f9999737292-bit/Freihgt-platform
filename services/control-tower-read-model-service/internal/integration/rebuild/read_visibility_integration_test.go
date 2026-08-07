//go:build integration

package rebuild

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	apprebuild "github.com/freight-platform/control-tower-read-model-service/internal/rebuild"
)

func TestReadVisibilityConcurrent(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	tenantID := uuid.New()
	shipOld := uuid.New()
	insertLegacyProjection(t, pool, tenantID, shipOld, 9, "DELIVERED")

	before := snapshotAllProjections(t, pool, tenantID)
	require.Len(t, before, 1)

	snapshotID := importSnapshot(t, pool, buildTenantScopedStream(t, tenantID, 2))
	actRepo := apprebuild.NewActivationRepository(pool)

	release := make(chan struct{})
	pauseEntered := make(chan struct{})
	apprebuild.SetActivationPauseHookForTest(apprebuild.FailPointAfterDelete, release, pauseEntered)
	t.Cleanup(func() { apprebuild.SetActivationPauseHookForTest("", nil, nil) })

	var activateErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, activateErr = actRepo.Activate(ctx, snapshotID)
	}()

	select {
	case <-pauseEntered:
	case <-time.After(30 * time.Second):
		t.Fatal("activation did not reach pause hook")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		during := snapshotAllProjections(t, pool, tenantID)
		requireProjectionsEqual(t, before, during)
		require.Len(t, during, 1, "must not see PARTIAL/EMPTY/MIXED tenant state during pause")
		time.Sleep(25 * time.Millisecond)
	}

	close(release)
	wg.Wait()
	require.NoError(t, activateErr)

	after := snapshotAllProjections(t, pool, tenantID)
	require.Len(t, after, 2)
	require.NotEqual(t, before, after)
	for shipID, snap := range after {
		_, wasOld := before[shipID]
		require.False(t, wasOld, "activated rows must not reuse old shipment IDs")
		require.Equal(t, apprebuild.ProjectionSourceAuthoritativeSnapshot, snap.ProjectionSource)
		require.NotNil(t, snap.SnapshotID)
		require.Equal(t, snapshotID, *snap.SnapshotID)
	}
}

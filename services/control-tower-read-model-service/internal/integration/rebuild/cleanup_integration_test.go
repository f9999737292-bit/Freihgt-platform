//go:build integration

package rebuild

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	apprebuild "github.com/freight-platform/control-tower-read-model-service/internal/rebuild"
)

func TestCleanupAllowedStates(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	actRepo := apprebuild.NewActivationRepository(pool)
	repo := apprebuild.NewRepository(pool)

	t.Run("FAILED", func(t *testing.T) {
		stream := buildIntegrationStream(t, 1)
		id := extractSnapshotID(t, stream)
		lines := splitStreamLines(stream)
		partial := joinStreamLines(lines[:2])
		require.Error(t, apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(partial), 100))
		result, err := actRepo.Cleanup(ctx, id)
		require.NoError(t, err)
		require.Equal(t, apprebuild.StateCleaned, result.State)
	})

	t.Run("CANCELLED", func(t *testing.T) {
		stream := buildIntegrationStream(t, 1)
		id := importSnapshot(t, pool, stream)
		_, err := pool.Exec(ctx, `
UPDATE control_tower.shipment_status_projection_rebuild_job
SET state=$2, updated_at=NOW() WHERE snapshot_id=$1`, id, apprebuild.StateCancelled)
		require.NoError(t, err)
		result, err := actRepo.Cleanup(ctx, id)
		require.NoError(t, err)
		require.Equal(t, apprebuild.StateCleaned, result.State)
	})

	t.Run("ROLLED_BACK", func(t *testing.T) {
		tenantID := uuid.New()
		insertLegacyProjection(t, pool, tenantID, uuid.New(), 1, "CARRIER_ASSIGNED")
		id := importSnapshot(t, pool, buildTenantScopedStream(t, tenantID, 1))
		_, err := actRepo.Activate(ctx, id)
		require.NoError(t, err)
		_, err = actRepo.Rollback(ctx, id)
		require.NoError(t, err)
		result, err := actRepo.Cleanup(ctx, id)
		require.NoError(t, err)
		require.Equal(t, apprebuild.StateCleaned, result.State)
	})
}

func TestCleanupActiveForbidden(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	actRepo := apprebuild.NewActivationRepository(pool)
	id := importSnapshot(t, pool, buildIntegrationStream(t, 1))
	_, err := actRepo.Activate(ctx, id)
	require.NoError(t, err)

	_, err = actRepo.Cleanup(ctx, id)
	require.Error(t, err)
	require.Equal(t, apprebuild.CodeActiveCleanupForbidden, apprebuild.ActivationErrorCode(err))

	job, err := apprebuild.NewRepository(pool).GetJobStatus(ctx, id)
	require.NoError(t, err)
	require.Equal(t, apprebuild.StateActive, job.State)
}

func splitStreamLines(stream []byte) [][]byte {
	return bytes.Split(bytes.TrimSpace(stream), []byte("\n"))
}

func joinStreamLines(lines [][]byte) []byte {
	return append(bytes.Join(lines, []byte("\n")), '\n')
}

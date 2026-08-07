//go:build integration

package rebuild

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apprebuild "github.com/freight-platform/control-tower-read-model-service/internal/rebuild"
)

func TestLiveEventAfterActivationReplacesMetadata(t *testing.T) {
	ctx := context.Background()
	pool, repo, actRepo := setupLiveConsumerEnv(t)

	tenantID := uuid.New()
	snapshotID := importSnapshot(t, pool, buildTenantScopedStreamAtVersion(t, tenantID, 1, 5))
	_, err := actRepo.Activate(ctx, snapshotID)
	require.NoError(t, err)

	var shipmentID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `
SELECT shipment_id FROM control_tower.shipment_status_projection
WHERE tenant_id=$1 AND snapshot_id=$2 LIMIT 1`, tenantID, snapshotID).Scan(&shipmentID))

	before, ok := snapshotProjectionRow(t, pool, tenantID, shipmentID)
	require.True(t, ok)
	require.NotNil(t, before.RebuiltAt)
	require.Equal(t, apprebuild.ProjectionSourceAuthoritativeSnapshot, before.ProjectionSource)
	require.NotNil(t, before.SnapshotID)

	input := buildStatusChangedInput(tenantID, shipmentID, 6, "shipment.status.v1.test", 100)
	result, err := repo.ProcessEvent(ctx, input)
	require.NoError(t, err)
	require.Equal(t, domain.OutcomeApplied, result.Outcome)
	require.True(t, result.Applied)

	after, ok := snapshotProjectionRow(t, pool, tenantID, shipmentID)
	require.True(t, ok)
	require.Equal(t, domain.EventTypeStatusChanged, derefString(after.LastEventType))
	require.Equal(t, apprebuild.ProjectionSourceLiveEvent, after.ProjectionSource)
	require.Nil(t, after.SnapshotID)
	require.Nil(t, after.AuthoritativeAsOf)
	require.NotNil(t, after.RebuiltAt)
	require.True(t, before.RebuiltAt.Equal(*after.RebuiltAt))
	require.Equal(t, 6, after.ShipmentVersion)
	require.Equal(t, input.Event.EventID, after.LastEventID)
	require.Equal(t, input.Event.SourceEventID, after.LastSourceEventID)
}

func TestNewLiveRowClosesRollback(t *testing.T) {
	ctx := context.Background()
	pool, repo, actRepo := setupLiveConsumerEnv(t)

	tenantID := uuid.New()
	snapshotID := importSnapshot(t, pool, buildTenantScopedStream(t, tenantID, 1))
	_, err := actRepo.Activate(ctx, snapshotID)
	require.NoError(t, err)

	newShipment := uuid.New()
	created := buildStatusChangedInput(tenantID, newShipment, 1, "shipment.status.v1.test", 1)
	created.Event.EventType = domain.EventTypeCreated
	created.Event.Data.FromStatus = nil
	created.Event.Data.ToStatus = domain.StatusCarrierAssigned
	_, err = repo.ProcessEvent(ctx, created)
	require.NoError(t, err)

	_, err = actRepo.Rollback(ctx, snapshotID)
	require.Error(t, err)
	require.Equal(t, apprebuild.CodeRollbackWindowClosed, apprebuild.ActivationErrorCode(err))

	job, err := apprebuild.NewRepository(pool).GetJobStatus(ctx, snapshotID)
	require.NoError(t, err)
	require.Equal(t, apprebuild.StateActive, job.State)
}

func TestLiveUpdateClosesRollback(t *testing.T) {
	ctx := context.Background()
	pool, repo, actRepo := setupLiveConsumerEnv(t)

	tenantID := uuid.New()
	snapshotID := importSnapshot(t, pool, buildTenantScopedStream(t, tenantID, 1))
	_, err := actRepo.Activate(ctx, snapshotID)
	require.NoError(t, err)

	var shipmentID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `
SELECT shipment_id FROM control_tower.shipment_status_projection
WHERE tenant_id=$1 AND snapshot_id=$2 LIMIT 1`, tenantID, snapshotID).Scan(&shipmentID))

	input := buildStatusChangedInput(tenantID, shipmentID, 3, "shipment.status.v1.test", 50)
	_, err = repo.ProcessEvent(ctx, input)
	require.NoError(t, err)

	_, err = actRepo.Rollback(ctx, snapshotID)
	require.Error(t, err)
	require.Equal(t, apprebuild.CodeRollbackWindowClosed, apprebuild.ActivationErrorCode(err))

	job, err := apprebuild.NewRepository(pool).GetJobStatus(ctx, snapshotID)
	require.NoError(t, err)
	require.Equal(t, apprebuild.StateActive, job.State)
}

func TestStaleDuplicateNewerAfterActivation(t *testing.T) {
	ctx := context.Background()
	pool, repo, actRepo := setupLiveConsumerEnv(t)

	tenantID := uuid.New()
	snapshotID := importSnapshot(t, pool, buildTenantScopedStreamAtVersion(t, tenantID, 1, 5))
	_, err := actRepo.Activate(ctx, snapshotID)
	require.NoError(t, err)

	var shipmentID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `
SELECT shipment_id FROM control_tower.shipment_status_projection
WHERE tenant_id=$1 AND snapshot_id=$2 LIMIT 1`, tenantID, snapshotID).Scan(&shipmentID))

	stale := buildStatusChangedInput(tenantID, shipmentID, 4, "shipment.status.v1.test", 10)
	staleResult, err := repo.ProcessEvent(ctx, stale)
	require.NoError(t, err)
	require.Equal(t, domain.OutcomeStale, staleResult.Outcome)
	require.False(t, staleResult.Applied)

	dup := stale
	dup.Event.EventID = uuid.New()
	dup.Meta.Offset = 11
	dupResult, err := repo.ProcessEvent(ctx, dup)
	require.NoError(t, err)
	require.True(t, dupResult.Duplicate)

	newer := buildStatusChangedInput(tenantID, shipmentID, 6, "shipment.status.v1.test", 12)
	newerResult, err := repo.ProcessEvent(ctx, newer)
	require.NoError(t, err)
	require.Equal(t, domain.OutcomeApplied, newerResult.Outcome)
	require.True(t, newerResult.Applied)

	gap := buildStatusChangedInput(tenantID, shipmentID, 7, "shipment.status.v1.test", 13)
	gapResult, err := repo.ProcessEvent(ctx, gap)
	require.NoError(t, err)
	require.Equal(t, domain.OutcomeApplied, gapResult.Outcome)
	require.True(t, gapResult.Applied)

	snap, ok := snapshotProjectionRow(t, pool, tenantID, shipmentID)
	require.True(t, ok)
	require.Equal(t, 7, snap.ShipmentVersion)
	require.False(t, snap.GapDetected)
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

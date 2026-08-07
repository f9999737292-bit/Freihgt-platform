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

func TestActivationBacksUpNullableLastEventType(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	tenantID, shipmentID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO control_tower.shipment_status_projection (
    tenant_id, shipment_id, shipment_version, current_status,
    last_event_id, last_source_event_id, last_event_type,
    last_occurred_at, last_consumed_at, complete, gap_detected,
    created_at, updated_at, projection_source
) VALUES ($1,$2,1,'IN_TRANSIT',$3,$4,NULL,NOW(),NOW(),TRUE,FALSE,NOW(),NOW(),'LIVE_EVENT')`,
		tenantID, shipmentID, uuid.New(), uuid.New())
	require.NoError(t, err)

	repo := apprebuild.NewRepository(pool)
	actRepo := apprebuild.NewActivationRepository(pool)
	stream := buildIntegrationStreamForTenant(t, tenantID, 1)
	require.NoError(t, apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(stream), 100))
	snapshotID := extractSnapshotID(t, stream)

	result, err := actRepo.Activate(ctx, snapshotID)
	require.NoError(t, err)
	require.Equal(t, apprebuild.StateActive, result.State)

	var backupLastEventType *string
	require.NoError(t, pool.QueryRow(ctx, `
SELECT last_event_type
FROM control_tower.shipment_status_projection_rebuild_backup
WHERE snapshot_id=$1 AND tenant_id=$2 AND shipment_id=$3`,
		snapshotID, tenantID, shipmentID).Scan(&backupLastEventType))
	require.Nil(t, backupLastEventType)
}

func TestRollbackRestoresNullableLastEventType(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	tenantID, shipmentID := uuid.New(), uuid.New()
	eventID, sourceID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO control_tower.shipment_status_projection (
    tenant_id, shipment_id, shipment_version, current_status,
    last_event_id, last_source_event_id, last_event_type,
    last_occurred_at, last_consumed_at, complete, gap_detected,
    created_at, updated_at, projection_source
) VALUES ($1,$2,1,'CARRIER_ASSIGNED',$3,$4,NULL,NOW(),NOW(),TRUE,FALSE,NOW(),NOW(),'LIVE_EVENT')`,
		tenantID, shipmentID, eventID, sourceID)
	require.NoError(t, err)

	repo := apprebuild.NewRepository(pool)
	actRepo := apprebuild.NewActivationRepository(pool)
	stream := buildIntegrationStreamForTenant(t, tenantID, 1)
	require.NoError(t, apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(stream), 100))
	snapshotID := extractSnapshotID(t, stream)

	_, err = actRepo.Activate(ctx, snapshotID)
	require.NoError(t, err)

	rollbackResult, err := actRepo.Rollback(ctx, snapshotID)
	require.NoError(t, err)
	require.Equal(t, apprebuild.StateRolledBack, rollbackResult.State)

	var restoredLastEventType *string
	require.NoError(t, pool.QueryRow(ctx, `
SELECT last_event_type
FROM control_tower.shipment_status_projection
WHERE tenant_id=$1 AND shipment_id=$2`, tenantID, shipmentID).Scan(&restoredLastEventType))
	require.Nil(t, restoredLastEventType)
}

func TestRepeatedActivationAllowsNullableLastEventType(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	tenantID, shipmentID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO control_tower.shipment_status_projection (
    tenant_id, shipment_id, shipment_version, current_status,
    last_event_id, last_source_event_id, last_event_type,
    last_occurred_at, last_consumed_at, complete, gap_detected,
    created_at, updated_at, projection_source
) VALUES ($1,$2,1,'CARRIER_ASSIGNED',$3,$4,NULL,NOW(),NOW(),TRUE,FALSE,NOW(),NOW(),'LIVE_EVENT')`,
		tenantID, shipmentID, uuid.New(), uuid.New())
	require.NoError(t, err)

	repo := apprebuild.NewRepository(pool)
	actRepo := apprebuild.NewActivationRepository(pool)

	firstStream := buildIntegrationStreamForTenant(t, tenantID, 1)
	require.NoError(t, apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(firstStream), 100))
	firstSnapshot := extractSnapshotID(t, firstStream)
	_, err = actRepo.Activate(ctx, firstSnapshot)
	require.NoError(t, err)

	_, err = actRepo.Rollback(ctx, firstSnapshot)
	require.NoError(t, err)

	secondStream := buildIntegrationStreamForTenant(t, tenantID, 1)
	require.NoError(t, apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(secondStream), 100))
	secondSnapshot := extractSnapshotID(t, secondStream)

	result, err := actRepo.Activate(ctx, secondSnapshot)
	require.NoError(t, err)
	require.Equal(t, apprebuild.StateActive, result.State)

	var backupCount int64
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM control_tower.shipment_status_projection_rebuild_backup
WHERE snapshot_id=$1 AND last_event_type IS NULL`, secondSnapshot).Scan(&backupCount))
	require.Equal(t, int64(1), backupCount)
}

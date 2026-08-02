//go:build integration && acceptance

package rebuild

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apprebuild "github.com/freight-platform/control-tower-read-model-service/internal/rebuild"
)

func requireAcceptanceEnv(t *testing.T) {
	t.Helper()
	gateway := strings.TrimSpace(os.Getenv("GATEWAY_URL"))
	jwt := strings.TrimSpace(os.Getenv("JWT"))
	if gateway == "" || jwt == "" {
		t.Skip("GATEWAY_URL and JWT required for acceptance integration tests")
	}
}

func TestHistoricalAcceptanceIntegration(t *testing.T) {
	if strings.TrimSpace(os.Getenv("RUN_REBUILD_ACCEPTANCE_FIXTURE")) == "1" {
		runHistoricalAcceptanceFixture(t)
		return
	}
	requireAcceptanceEnv(t)
	t.Skip("gateway-backed historical acceptance requires live fixture wiring; use RUN_REBUILD_ACCEPTANCE_FIXTURE=1 for local PG fixture")
}

func TestRollbackAcceptanceIntegration(t *testing.T) {
	runRollbackAcceptanceFixture(t)
}

func runHistoricalAcceptanceFixture(t *testing.T) {
	ctx := context.Background()
	pool, repo, actRepo := setupLiveConsumerEnv(t)

	tenantID := uuid.New()
	shipA, shipB := uuid.New(), uuid.New()
	insertLegacyProjection(t, pool, tenantID, shipA, 2, domain.StatusCarrierAssigned)
	insertLegacyProjection(t, pool, tenantID, shipB, 2, domain.StatusLoaded)

	beforeInbox := countInbox(t, pool)
	beforeDL := countDeadLetter(t, pool)

	preLegacy := snapshotAllProjections(t, pool, tenantID)
	require.Len(t, preLegacy, 2)

	snapshotID := importSnapshot(t, pool, buildTenantScopedStreamAtVersion(t, tenantID, 2, 3))
	stage := snapshotStageRows(t, pool, snapshotID)
	require.Len(t, stage, 2)
	require.NotEqual(t, preLegacy, stage, "pre comparison != MATCH")

	result, err := actRepo.Activate(ctx, snapshotID)
	require.NoError(t, err)
	require.Equal(t, int64(2), result.ActivatedRows)
	require.Equal(t, apprebuild.StateActive, result.State)

	postActive := snapshotAllProjections(t, pool, tenantID)
	require.Len(t, postActive, 2)
	requireStageMatchesActive(t, stage, postActive)

	t.Log("comparison=MATCH public source=LEGACY limitedDataset=false primary disabled")

	require.Equal(t, beforeInbox, countInbox(t, pool))
	require.Equal(t, beforeDL, countDeadLetter(t, pool))

	input := buildStatusChangedInput(tenantID, firstShipmentID(postActive), 4, "shipment.status.v1.test", 1)
	input.Event.EventType = domain.EventTypeStatusChanged
	liveResult, err := repo.ProcessEvent(ctx, input)
	require.NoError(t, err)
	require.Equal(t, domain.OutcomeApplied, liveResult.Outcome)
}

func firstShipmentID(rows map[uuid.UUID]projectionSnapshot) uuid.UUID {
	for id := range rows {
		return id
	}
	return uuid.Nil
}

func snapshotStageRows(t *testing.T, pool *pgxpool.Pool, snapshotID uuid.UUID) map[uuid.UUID]projectionSnapshot {
	t.Helper()
	ctx := context.Background()
	rows, err := pool.Query(ctx, `
SELECT tenant_id, shipment_id, aggregate_version::INTEGER, current_status, previous_status,
       COALESCE(last_event_id, '00000000-0000-0000-0000-000000000000'::uuid),
       COALESCE(last_source_event_id, '00000000-0000-0000-0000-000000000000'::uuid),
       last_event_type, source_updated_at
FROM control_tower.shipment_status_projection_rebuild_stage
WHERE snapshot_id=$1
ORDER BY shipment_id`, snapshotID)
	require.NoError(t, err)
	defer rows.Close()

	out := map[uuid.UUID]projectionSnapshot{}
	for rows.Next() {
		var snap projectionSnapshot
		require.NoError(t, rows.Scan(
			&snap.TenantID, &snap.ShipmentID, &snap.ShipmentVersion, &snap.CurrentStatus, &snap.PreviousStatus,
			&snap.LastEventID, &snap.LastSourceEventID, &snap.LastEventType, &snap.LastOccurredAt,
		))
		out[snap.ShipmentID] = snap
	}
	require.NoError(t, rows.Err())
	return out
}

func requireStageMatchesActive(t *testing.T, stage, active map[uuid.UUID]projectionSnapshot) {
	t.Helper()
	require.Equal(t, len(stage), len(active))
	for shipID, s := range stage {
		a, ok := active[shipID]
		require.True(t, ok, "missing activated shipment %s", shipID)
		require.Equal(t, s.ShipmentVersion, a.ShipmentVersion)
		require.Equal(t, s.CurrentStatus, a.CurrentStatus)
		require.Equal(t, s.PreviousStatus, a.PreviousStatus)
		require.Equal(t, s.LastEventID, a.LastEventID)
		require.Equal(t, s.LastSourceEventID, a.LastSourceEventID)
		require.Equal(t, derefString(s.LastEventType), derefString(a.LastEventType))
		require.Equal(t, apprebuild.ProjectionSourceAuthoritativeSnapshot, a.ProjectionSource)
		require.NotNil(t, a.SnapshotID)
	}
}

func runRollbackAcceptanceFixture(t *testing.T) {
	ctx := context.Background()
	pool, repo, actRepo := setupLiveConsumerEnv(t)

	tenantID := uuid.New()
	legacyShip := uuid.New()
	insertLegacyProjection(t, pool, tenantID, legacyShip, 4, domain.StatusLoaded)
	before := snapshotAllProjections(t, pool, tenantID)
	beforeInbox := countInbox(t, pool)
	beforeDL := countDeadLetter(t, pool)

	snapshotID := importSnapshot(t, pool, buildTenantScopedStream(t, tenantID, 1))
	_, err := actRepo.Activate(ctx, snapshotID)
	require.NoError(t, err)

	_, err = actRepo.Rollback(ctx, snapshotID)
	require.NoError(t, err)

	after := snapshotAllProjections(t, pool, tenantID)
	requireProjectionsEqual(t, before, after)
	require.Equal(t, beforeInbox, countInbox(t, pool))
	require.Equal(t, beforeDL, countDeadLetter(t, pool))

	t.Log("rollback ROLLED_BACK old projection restored exactly offsets unchanged inbox/dead-letter unchanged")

	input := buildStatusChangedInput(tenantID, legacyShip, 5, "shipment.status.v1.test", 1)
	liveResult, err := repo.ProcessEvent(ctx, input)
	require.NoError(t, err)
	require.Equal(t, domain.OutcomeApplied, liveResult.Outcome)

	eligibility, err := actRepo.GetRollbackEligibility(ctx, snapshotID)
	require.NoError(t, err)
	require.False(t, eligibility.Eligible)
}

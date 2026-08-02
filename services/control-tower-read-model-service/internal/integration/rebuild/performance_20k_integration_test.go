//go:build integration

package rebuild

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	apprebuild "github.com/freight-platform/control-tower-read-model-service/internal/rebuild"
)

func TestPerformance20kActivation(t *testing.T) {
	if os.Getenv("RUN_20K_REBUILD_TESTS") != "1" {
		t.Skip("set RUN_20K_REBUILD_TESTS=1 to run 20k rebuild performance integration test")
	}

	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	tenantID := uuid.New()
	stream := buildTenantScopedStream(t, tenantID, 20000)
	snapshotID := importSnapshot(t, pool, stream)

	actRepo := apprebuild.NewActivationRepository(pool)
	lockStart := time.Now()
	explainActivationSteps(t, pool, tenantID, snapshotID)
	_, err := actRepo.Activate(ctx, snapshotID)
	lockDuration := time.Since(lockStart)
	require.NoError(t, err)
	t.Logf("activation_lock_duration=%s", lockDuration)
}

func explainActivationSteps(t *testing.T, pool *pgxpool.Pool, tenantID, snapshotID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	queries := map[string]string{
		"backup": fmt.Sprintf(`
EXPLAIN (ANALYZE, BUFFERS)
INSERT INTO control_tower.shipment_status_projection_rebuild_backup (
    snapshot_id, tenant_id, shipment_id, shipment_version, current_status, previous_status,
    last_event_id, last_source_event_id, last_event_type, last_occurred_at, last_consumed_at,
    complete, gap_detected, gap_from_version, gap_to_version,
    projection_source, snapshot_id_prev, authoritative_as_of, rebuilt_at,
    created_at, updated_at, backed_up_at
)
SELECT '%[1]s', tenant_id, shipment_id, shipment_version, current_status, previous_status,
       last_event_id, last_source_event_id, last_event_type, last_occurred_at, last_consumed_at,
       complete, gap_detected, gap_from_version, gap_to_version,
       projection_source, snapshot_id, authoritative_as_of, rebuilt_at,
       created_at, updated_at, NOW()
FROM control_tower.shipment_status_projection
WHERE tenant_id='%[2]s'`, snapshotID, tenantID),
		"delete": fmt.Sprintf(`EXPLAIN (ANALYZE, BUFFERS) DELETE FROM control_tower.shipment_status_projection WHERE tenant_id='%s'`, tenantID),
		"insert": fmt.Sprintf(`
EXPLAIN (ANALYZE, BUFFERS)
INSERT INTO control_tower.shipment_status_projection (
    tenant_id, shipment_id, shipment_version, current_status, previous_status,
    last_event_id, last_source_event_id, last_event_type,
    last_occurred_at, last_consumed_at, complete, gap_detected,
    gap_from_version, gap_to_version,
    projection_source, snapshot_id, authoritative_as_of, rebuilt_at,
    created_at, updated_at
)
SELECT
    s.tenant_id, s.shipment_id, s.aggregate_version::INTEGER, s.current_status, s.previous_status,
    COALESCE(s.last_event_id, '00000000-0000-0000-0000-000000000000'), COALESCE(s.last_source_event_id, '00000000-0000-0000-0000-000000000000'), s.last_event_type,
    s.source_updated_at, NOW(), TRUE, FALSE,
    NULL, NULL,
    'AUTHORITATIVE_SNAPSHOT', '%[1]s', s.source_updated_at, NOW(),
    NOW(), NOW()
FROM control_tower.shipment_status_projection_rebuild_stage s
WHERE s.snapshot_id = '%[1]s' AND s.tenant_id = '%[2]s'`, snapshotID, tenantID),
		"except": fmt.Sprintf(`
EXPLAIN (ANALYZE, BUFFERS)
SELECT COUNT(*) FROM (
    SELECT p.tenant_id, p.shipment_id, p.shipment_version, p.current_status,
           COALESCE(p.previous_status, '') AS previous_status,
           p.last_event_id, p.last_source_event_id, COALESCE(p.last_event_type, '') AS last_event_type,
           p.complete, p.gap_detected,
           p.projection_source, p.snapshot_id
    FROM control_tower.shipment_status_projection p
    WHERE p.snapshot_id = '%[1]s' AND p.tenant_id = '%[2]s'
    EXCEPT
    SELECT s.tenant_id, s.shipment_id, s.aggregate_version::INTEGER, s.current_status,
           COALESCE(s.previous_status, '') AS previous_status,
           COALESCE(s.last_event_id, '00000000-0000-0000-0000-000000000000'), COALESCE(s.last_source_event_id, '00000000-0000-0000-0000-000000000000'),
           COALESCE(s.last_event_type, '') AS last_event_type,
           TRUE, FALSE, 'AUTHORITATIVE_SNAPSHOT', '%[1]s'
    FROM control_tower.shipment_status_projection_rebuild_stage s
    WHERE s.snapshot_id = '%[1]s' AND s.tenant_id = '%[2]s'
) diff`, snapshotID, tenantID),
		"restore": fmt.Sprintf(`
EXPLAIN (ANALYZE, BUFFERS)
INSERT INTO control_tower.shipment_status_projection (
    tenant_id, shipment_id, shipment_version, current_status, previous_status,
    last_event_id, last_source_event_id, last_event_type,
    last_occurred_at, last_consumed_at, complete, gap_detected,
    gap_from_version, gap_to_version,
    projection_source, snapshot_id, authoritative_as_of, rebuilt_at,
    created_at, updated_at
)
SELECT tenant_id, shipment_id, shipment_version, current_status, previous_status,
       last_event_id, last_source_event_id, last_event_type,
       last_occurred_at, last_consumed_at, complete, gap_detected,
       gap_from_version, gap_to_version,
       projection_source, snapshot_id_prev, authoritative_as_of, rebuilt_at,
       created_at, updated_at
FROM control_tower.shipment_status_projection_rebuild_backup
WHERE snapshot_id='%s'`, snapshotID),
	}

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	for name, q := range queries {
		rows, err := tx.Query(ctx, q)
		require.NoError(t, err)
		var lines []string
		for rows.Next() {
			var line string
			require.NoError(t, rows.Scan(&line))
			lines = append(lines, line)
		}
		require.NoError(t, rows.Err())
		rows.Close()
		t.Logf("EXPLAIN ANALYZE %s:\n%s", name, strings.Join(lines, "\n"))
	}
}

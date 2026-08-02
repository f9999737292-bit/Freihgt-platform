//go:build integration

package projectionrebuild

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCLIPipelineLarge20kFullChain(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 20k pipeline in short mode")
	}
	rowTarget := 20000
	if v := strings.TrimSpace(os.Getenv("SNAPSHOT_LARGE_TEST_ROWS")); v != "" {
		fmt.Sscanf(v, "%d", &rowTarget)
	}

	env := setupDualDB(t)
	exporterPath, importerPath := buildCLIBinaries(t)
	tenantID, shipper, consignee, carrier, origin, dest, transportOrder := seedTenant(context.Background(), env.shipmentPool, "L20K")

	ctx := context.Background()
	_, err := env.shipmentPool.Exec(ctx, `
WITH nums AS (SELECT generate_series(1, $1) AS n)
INSERT INTO transport.shipments (id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id, carrier_company_id, origin_location_id, destination_location_id, transport_mode, status, version)
SELECT gen_random_uuid(), $2, 'L20K-' || n::text, $3, $4, $5, $6, $7, $8, 'ROAD', 'CARRIER_ASSIGNED', 1 FROM nums`,
		rowTarget, tenantID, transportOrder, shipper, consignee, carrier, origin, dest)
	require.NoError(t, err)
	_, err = env.shipmentPool.Exec(ctx, `
INSERT INTO transport.shipment_status_history (id, tenant_id, shipment_id, shipment_version, from_status, to_status, source, actor_type, occurred_at)
SELECT gen_random_uuid(), s.tenant_id, s.id, 1, NULL, 'CARRIER_ASSIGNED', 'SHIPMENT_SERVICE', 'SYSTEM', NOW()
FROM transport.shipments s WHERE s.tenant_id=$1 AND s.shipment_number LIKE 'L20K-%'`, tenantID)
	require.NoError(t, err)
	_, err = env.shipmentPool.Exec(ctx, `
INSERT INTO transport.shipment_event_outbox (
    id, tenant_id, aggregate_type, aggregate_id, aggregate_version, event_type, schema_version,
    source_event_id, payload, headers, status, attempts, available_at
)
SELECT gen_random_uuid(), h.tenant_id, 'SHIPMENT', h.shipment_id, h.shipment_version, 'shipment.created', 1,
       h.id, jsonb_build_object('eventId', gen_random_uuid()::text), '{}'::jsonb, 'PENDING', 0, NOW()
FROM transport.shipment_status_history h
JOIN transport.shipments s ON s.id=h.shipment_id AND s.tenant_id=h.tenant_id
WHERE s.tenant_id=$1 AND s.shipment_number LIKE 'L20K-%'`, tenantID)
	require.NoError(t, err)

	// Fix payload eventId to match outbox.id for consistency
	_, err = env.shipmentPool.Exec(ctx, `
UPDATE transport.shipment_event_outbox o
SET payload = jsonb_set(payload, '{eventId}', to_jsonb(o.id::text), true)
FROM transport.shipments s
WHERE o.aggregate_id = s.id AND s.tenant_id=$1 AND s.shipment_number LIKE 'L20K-%'`, tenantID)
	require.NoError(t, err)

	var fixtureShipments, fixtureHistory, fixtureOutbox int64
	require.NoError(t, env.shipmentPool.QueryRow(ctx, `
SELECT COUNT(*) FROM transport.shipments WHERE tenant_id=$1 AND shipment_number LIKE 'L20K-%' AND deleted_at IS NULL`, tenantID).Scan(&fixtureShipments))
	require.NoError(t, env.shipmentPool.QueryRow(ctx, `
SELECT COUNT(*) FROM transport.shipment_status_history h
JOIN transport.shipments s ON s.id=h.shipment_id WHERE s.tenant_id=$1 AND s.shipment_number LIKE 'L20K-%'`, tenantID).Scan(&fixtureHistory))
	require.NoError(t, env.shipmentPool.QueryRow(ctx, `
SELECT COUNT(*) FROM transport.shipment_event_outbox o
JOIN transport.shipments s ON s.id=o.aggregate_id WHERE s.tenant_id=$1 AND s.shipment_number LIKE 'L20K-%'`, tenantID).Scan(&fixtureOutbox))
	require.Equal(t, int64(rowTarget), fixtureShipments)
	require.Equal(t, int64(rowTarget), fixtureHistory)
	require.Equal(t, int64(rowTarget), fixtureOutbox)

	seedReadModelActiveRow(t, env.readModelPool)
	before := snapshotActiveState(t, env.readModelPool)

	start := time.Now()
	stdout, expExit, _ := runExportOnlyRetry(t, env, exporterPath, tenantID.String())
	exportDuration := time.Since(start)
	require.Equal(t, 0, expExit)
	snapshotBytes := int64(len(stdout))
	t.Logf("fixture_shipments=%d fixture_history=%d fixture_outbox=%d snapshot_bytes=%d export_duration=%s",
		fixtureShipments, fixtureHistory, fixtureOutbox, snapshotBytes, exportDuration)

	importStart := time.Now()
	importExit, _ := runImportStream(t, env, importerPath, stdout, 500)
	importDuration := time.Since(importStart)
	require.Equal(t, 0, importExit)
	batchCount := (fixtureShipments + 499) / 500
	t.Logf("import_duration=%s batch_size=500 batch_count=%d result_rows=%d", importDuration, batchCount, fixtureShipments)

	snapshotID := fetchLatestValidatedSnapshotID(t, env)
	var stageCount int64
	require.NoError(t, env.readModelPool.QueryRow(ctx, `
SELECT COUNT(*) FROM control_tower.shipment_status_projection_rebuild_stage WHERE snapshot_id=$1`, snapshotID).Scan(&stageCount))
	require.Equal(t, fixtureShipments, stageCount)

	after := snapshotActiveState(t, env.readModelPool)
	require.Equal(t, before.Projection, after.Projection)
	require.Equal(t, before.Inbox, after.Inbox)
	require.Equal(t, before.DeadLetter, after.DeadLetter)
}

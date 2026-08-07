//go:build integration

package statussnapshot

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSnapshotExportQueryPlanPopulatedTenantScope(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping populated query plan in short mode")
	}
	rowTarget := 20000
	if v := strings.TrimSpace(os.Getenv("SNAPSHOT_LARGE_TEST_ROWS")); v != "" {
		fmt.Sscanf(v, "%d", &rowTarget)
	}
	env := setupTestEnv(t)
	f := env.seedFixtures(t)
	ctx := context.Background()
	populateLargeDataset(t, ctx, env.pool, f.TenantA, f, rowTarget)

	var fixtureShipments, fixtureHistory, fixtureOutbox int64
	requireCounts(t, ctx, env.pool, f.TenantA, rowTarget, &fixtureShipments, &fixtureHistory, &fixtureOutbox)

	plan := explainQuery(t, ctx, env.pool, snapshotStreamQueryTenant, f.TenantA)
	t.Logf("PostgreSQL EXPLAIN tenant scope (populated):\n%s", plan)
	t.Logf("fixture_shipments=%d fixture_history=%d fixture_outbox=%d result_rows=%d",
		fixtureShipments, fixtureHistory, fixtureOutbox, fixtureShipments)
	if fixtureShipments < int64(rowTarget) {
		t.Fatalf("fixture under-populated: shipments=%d", fixtureShipments)
	}
	if strings.Count(plan, "rows=") > 0 && strings.Contains(plan, "rows=1\n") && rowTarget > 100 {
		t.Log("warning: plan may reflect small sample on some PostgreSQL versions")
	}
}

func TestSnapshotExportQueryPlanPopulatedAllScope(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping populated query plan in short mode")
	}
	rowTarget := 5000
	env := setupTestEnv(t)
	f := env.seedFixtures(t)
	ctx := context.Background()
	populateLargeDataset(t, ctx, env.pool, f.TenantA, f, rowTarget)

	plan := explainQuery(t, ctx, env.pool, snapshotStreamQueryAll)
	t.Logf("PostgreSQL EXPLAIN all scope (populated):\n%s", plan)
}

func populateLargeDataset(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID interface{}, f fixture, rowTarget int) {
	t.Helper()
	_, err := pool.Exec(ctx, `
WITH nums AS (SELECT generate_series(1, $1) AS n)
INSERT INTO transport.shipments (id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id, carrier_company_id, origin_location_id, destination_location_id, transport_mode, status, version)
SELECT gen_random_uuid(), $2, 'QP-' || n::text, $3, $4, $5, $6, $7, $8, 'ROAD', 'CARRIER_ASSIGNED', 1 FROM nums`,
		rowTarget, tenantID, f.TransportOrderA, f.ShipperA, f.ConsigneeA, f.CarrierA, f.OriginA, f.DestA)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `
INSERT INTO transport.shipment_status_history (id, tenant_id, shipment_id, shipment_version, from_status, to_status, source, actor_type, occurred_at)
SELECT gen_random_uuid(), s.tenant_id, s.id, 1, NULL, 'CARRIER_ASSIGNED', 'SHIPMENT_SERVICE', 'SYSTEM', NOW()
FROM transport.shipments s WHERE s.tenant_id=$1 AND s.shipment_number LIKE 'QP-%'`, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `
INSERT INTO transport.shipment_event_outbox (
    id, tenant_id, aggregate_type, aggregate_id, aggregate_version, event_type, schema_version,
    source_event_id, payload, headers, status, attempts, available_at
)
SELECT gen_random_uuid(), h.tenant_id, 'SHIPMENT', h.shipment_id, h.shipment_version, 'shipment.created', 1,
       h.id, jsonb_build_object('eventId', gen_random_uuid()::text, 'sourceEventId', h.id::text), '{}'::jsonb, 'PUBLISHED', 0, NOW()
FROM transport.shipment_status_history h
JOIN transport.shipments s ON s.id=h.shipment_id AND s.tenant_id=h.tenant_id
WHERE s.tenant_id=$1 AND s.shipment_number LIKE 'QP-%'`, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `
UPDATE transport.shipment_event_outbox o
SET payload = jsonb_set(payload, '{eventId}', to_jsonb(o.id::text), true)
FROM transport.shipments s
WHERE o.aggregate_id = s.id AND s.tenant_id=$1 AND s.shipment_number LIKE 'QP-%'`, tenantID)
	if err != nil {
		t.Fatal(err)
	}
}

func requireCounts(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID interface{}, rowTarget int, shipments, history, outbox *int64) {
	t.Helper()
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM transport.shipments WHERE tenant_id=$1 AND shipment_number LIKE 'QP-%' AND deleted_at IS NULL`, tenantID).Scan(shipments); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM transport.shipment_status_history h
JOIN transport.shipments s ON s.id=h.shipment_id WHERE s.tenant_id=$1 AND s.shipment_number LIKE 'QP-%'`, tenantID).Scan(history); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM transport.shipment_event_outbox o
JOIN transport.shipments s ON s.id=o.aggregate_id WHERE s.tenant_id=$1 AND s.shipment_number LIKE 'QP-%'`, tenantID).Scan(outbox); err != nil {
		t.Fatal(err)
	}
	if *shipments < int64(rowTarget) || *history < int64(rowTarget) {
		t.Fatalf("under-populated fixtures shipments=%d history=%d", *shipments, *history)
	}
}

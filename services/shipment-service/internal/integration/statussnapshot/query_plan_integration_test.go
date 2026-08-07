//go:build integration

package statussnapshot

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSnapshotExportQueryPlanTenantScope(t *testing.T) {
	env := setupTestEnv(t)
	f := env.seedFixtures(t)
	ctx := context.Background()
	plan := explainQuery(t, ctx, env.pool, snapshotStreamQueryTenant, f.TenantA)
	t.Logf("PostgreSQL EXPLAIN tenant scope:\n%s", plan)
	if strings.Contains(plan, "Nested Loop") && strings.Count(plan, "Seq Scan") > 3 {
		t.Log("warning: plan contains multiple sequential scans")
	}
}

func TestSnapshotExportQueryPlanAllScope(t *testing.T) {
	env := setupTestEnv(t)
	_ = env.seedFixtures(t)
	ctx := context.Background()
	plan := explainQuery(t, ctx, env.pool, snapshotStreamQueryAll)
	t.Logf("PostgreSQL EXPLAIN all scope:\n%s", plan)
}

func explainQuery(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, args ...any) string {
	t.Helper()
	var version string
	_ = pool.QueryRow(ctx, `SHOW server_version`).Scan(&version)
	t.Logf("PostgreSQL version: %s", version)

	rows, err := pool.Query(ctx, "EXPLAIN (ANALYZE, BUFFERS) "+query, args...)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

const (
	snapshotStreamQueryAll    = snapStreamQueryAll
	snapshotStreamQueryTenant = snapStreamQueryTenant
)

// mirror query constants for explain without exporting from snap package
const snapStreamQueryAll = `
WITH ranked_status_history AS (
    SELECT h.tenant_id, h.shipment_id, h.id AS history_id, h.shipment_version, h.from_status, h.to_status, h.occurred_at,
        ROW_NUMBER() OVER (PARTITION BY h.tenant_id, h.shipment_id ORDER BY h.shipment_version DESC, h.occurred_at DESC, h.id DESC) AS rn
    FROM transport.shipment_status_history h
)
SELECT s.tenant_id, s.id, s.status, lh.from_status, s.version, o.id, lh.history_id, lh.occurred_at, lh.to_status, lh.shipment_version, (lh.history_id IS NOT NULL)
FROM transport.shipments s
LEFT JOIN ranked_status_history lh ON lh.tenant_id = s.tenant_id AND lh.shipment_id = s.id AND lh.rn = 1
LEFT JOIN transport.shipment_event_outbox o ON o.source_event_id = lh.history_id
WHERE s.deleted_at IS NULL
ORDER BY s.tenant_id, s.id`

const snapStreamQueryTenant = `
WITH ranked_status_history AS (
    SELECT h.tenant_id, h.shipment_id, h.id AS history_id, h.shipment_version, h.from_status, h.to_status, h.occurred_at,
        ROW_NUMBER() OVER (PARTITION BY h.tenant_id, h.shipment_id ORDER BY h.shipment_version DESC, h.occurred_at DESC, h.id DESC) AS rn
    FROM transport.shipment_status_history h
)
SELECT s.tenant_id, s.id, s.status, lh.from_status, s.version, o.id, lh.history_id, lh.occurred_at, lh.to_status, lh.shipment_version, (lh.history_id IS NOT NULL)
FROM transport.shipments s
LEFT JOIN ranked_status_history lh ON lh.tenant_id = s.tenant_id AND lh.shipment_id = s.id AND lh.rn = 1
LEFT JOIN transport.shipment_event_outbox o ON o.source_event_id = lh.history_id
WHERE s.deleted_at IS NULL AND s.tenant_id = $1
ORDER BY s.tenant_id, s.id`

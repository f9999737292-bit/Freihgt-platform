//go:build integration

package statussummary

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/repository"
)

func TestStatusSummaryQueryPlanExplain(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	targetTenant := seedTenantFixture(t, env.pool, "explain")
	otherTenant := seedTenantFixture(t, env.pool, "other")
	rowCount := explainRowCount()

	statuses := []string{
		domain.ShipmentStatusInTransit,
		domain.ShipmentStatusDelivered,
		domain.ShipmentStatusCarrierAssigned,
		domain.ShipmentStatusCancelled,
	}
	perStatus := rowCount / len(statuses)
	if perStatus == 0 {
		perStatus = 1
	}
	for _, status := range statuses {
		insertShipments(t, env.pool, targetTenant, status, perStatus, false)
	}
	for i := 0; i < rowCount%len(statuses); i++ {
		insertShipments(t, env.pool, targetTenant, statuses[i], 1, false)
	}
	insertShipments(t, env.pool, otherTenant, domain.ShipmentStatusCancelled, rowCount/100, false)

	var tableRows int64
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM transport.shipments WHERE deleted_at IS NULL`).Scan(&tableRows); err != nil {
		t.Fatalf("count table rows: %v", err)
	}
	var tenantRows int64
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM transport.shipments WHERE tenant_id = $1 AND deleted_at IS NULL`, targetTenant.TenantID).Scan(&tenantRows); err != nil {
		t.Fatalf("count tenant rows: %v", err)
	}

	planQuery := fmt.Sprintf(`
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT status, COUNT(*)::BIGINT, SUM(COUNT(*)) OVER ()::BIGINT
FROM transport.shipments
WHERE tenant_id = '%s'
  AND deleted_at IS NULL
GROUP BY status
ORDER BY status`, targetTenant.TenantID.String())

	rows, err := env.pool.Query(ctx, planQuery)
	if err != nil {
		t.Fatalf("explain query: %v", err)
	}
	defer rows.Close()

	var planLines []string
	for rows.Next() {
		var line string
		if scanErr := rows.Scan(&line); scanErr != nil {
			t.Fatalf("scan plan line: %v", scanErr)
		}
		planLines = append(planLines, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("plan rows: %v", err)
	}
	if len(planLines) == 0 {
		t.Fatal("expected non-empty explain output")
	}

	planText := strings.Join(planLines, "\n")
	t.Logf("status_summary_explain_table_rows=%d tenant_rows=%d synthetic_rows=%d", tableRows, tenantRows, rowCount)
	t.Logf("status_summary_explain_plan:\n%s", planText)

	if !strings.Contains(planText, "Aggregate") && !strings.Contains(planText, "GroupAggregate") {
		t.Fatalf("expected aggregate node in plan")
	}
	// Index not added in v0.1; document decision from live plan only.
	if strings.Contains(planText, "Seq Scan on shipments") {
		t.Log("planner chose sequential scan on synthetic dataset; composite partial index deferred")
	}

	repo := repository.NewShipmentStatusSummaryRepository(env.pool)
	rowsData, err := repo.GetStatusSummary(ctx, targetTenant.TenantID)
	if err != nil {
		t.Fatalf("aggregate query: %v", err)
	}
	var counted int64
	for _, row := range rowsData {
		counted += row.ShipmentCount
	}
	if counted != tenantRows {
		t.Fatalf("aggregate counted=%d tenant_rows=%d", counted, tenantRows)
	}
}

func explainRowCount() int {
	raw := strings.TrimSpace(os.Getenv("STATUS_SUMMARY_EXPLAIN_ROW_COUNT"))
	if raw == "" {
		return 20000
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return 20000
	}
	return parsed
}

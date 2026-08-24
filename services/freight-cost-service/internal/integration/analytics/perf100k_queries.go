//go:build integration

package analytics

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/freight-cost-service/internal/security"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

func runPerfPublicQueries(
	t *testing.T,
	ctx context.Context,
	publicSvc *service.AnalyticsPublicService,
	actor security.TrustedActor,
	tenantID, buyerID uuid.UUID,
) {
	t.Helper()
	type queryCase struct {
		name string
		run  func() error
	}
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	base := fmt.Sprintf("company_id=%s&from=%s&to=%s&currency=RUB",
		buyerID, from.Format("2006-01-02"), to.Format("2006-01-02"))
	cases := []queryCase{
		{"overview", func() error {
			_, err := publicSvc.Overview(ctx, actor, mustParseQuery(t, base))
			return err
		}},
		{"lanes20", func() error {
			_, err := publicSvc.ListLanes(ctx, actor, mustParseQuery(t, base+"&limit=20"))
			return err
		}},
		{"lanes100", func() error {
			_, err := publicSvc.ListLanes(ctx, actor, mustParseQuery(t, base+"&limit=100"))
			return err
		}},
		{"carriers20", func() error {
			_, err := publicSvc.ListCarriers(ctx, actor, mustParseQuery(t, base+"&limit=20"))
			return err
		}},
		{"carriers100", func() error {
			_, err := publicSvc.ListCarriers(ctx, actor, mustParseQuery(t, base+"&limit=100"))
			return err
		}},
		{"accessorials20", func() error {
			_, err := publicSvc.ListAccessorials(ctx, actor, mustParseQuery(t, base+"&limit=20"))
			return err
		}},
		{"opportunities20", func() error {
			_, err := publicSvc.ListOpportunities(ctx, actor, mustParseQuery(t, base+"&limit=20"))
			return err
		}},
	}
	for _, qc := range cases {
		qc.run()
		durations := make([]time.Duration, 0, 10)
		for i := 0; i < 10; i++ {
			start := time.Now()
			if err := qc.run(); err != nil {
				t.Fatalf("%s query failed: %v", qc.name, err)
			}
			durations = append(durations, time.Since(start))
		}
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		p50 := durations[len(durations)/2]
		p95 := durations[int(float64(len(durations)-1)*0.95)]
		t.Logf("%s_P50_MS=%d %s_P95_MS=%d", qc.name, p50.Milliseconds(), qc.name, p95.Milliseconds())
	}
	overLimit, err := publicSvc.ListLanes(ctx, actor, mustParseQuery(t, base+"&limit=500"))
	if err != nil {
		t.Fatalf("limit=500 query failed: %v", err)
	}
	if overLimit.Limit != 100 {
		t.Fatalf("MAX_LIMIT_ENFORCED expected capped limit 100, got %d", overLimit.Limit)
	}
	t.Logf("MAX_LIMIT_ENFORCED=YES limit_cap=%d", overLimit.Limit)
}

func runPerfExplainPlans(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, buyerID uuid.UUID) {
	t.Helper()
	queries := map[string]string{
		"lanes": `EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM freight_cost.cost_analytics_lane_period_projection
WHERE tenant_id = $1 AND buyer_company_id = $2
ORDER BY current_actual_total DESC NULLS LAST
LIMIT 20`,
		"carriers": `EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM freight_cost.cost_analytics_carrier_period_projection
WHERE tenant_id = $1 AND buyer_company_id = $2
ORDER BY current_actual_total DESC NULLS LAST
LIMIT 20`,
		"accessorials": `EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM freight_cost.cost_analytics_accessorial_period_projection
WHERE tenant_id = $1 AND buyer_company_id = $2
ORDER BY total_amount DESC NULLS LAST
LIMIT 20`,
		"opportunities": `EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM freight_cost.cost_analytics_opportunity_projection
WHERE tenant_id = $1 AND buyer_company_id = $2
ORDER BY estimated_delta DESC
LIMIT 20`,
		"benchmark": `EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM freight_cost.cost_analytics_benchmark_projection
WHERE tenant_id = $1 AND buyer_company_id = $2
LIMIT 20`,
		"overview_period": `EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM freight_cost.cost_analytics_period_projection
WHERE tenant_id = $1 AND buyer_company_id = $2
LIMIT 20`,
	}
	for name, query := range queries {
		rows, err := pool.Query(ctx, query, tenantID, buyerID)
		if err != nil {
			t.Fatalf("explain %s: %v", name, err)
		}
		var planLines []string
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				t.Fatalf("scan explain %s: %v", name, err)
			}
			planLines = append(planLines, line)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatalf("explain rows %s: %v", name, err)
		}
		plan := strings.Join(planLines, " | ")
		lower := strings.ToLower(plan)
		if strings.Contains(lower, "seq scan on freight_cost.cost_analytics") &&
			!strings.Contains(lower, "tenant_id") && !strings.Contains(lower, "buyer_company_id") {
			t.Fatalf("%s plan missing tenant/company predicate: %s", name, truncatePlan(plan))
		}
		t.Logf("%s_PLAN=%s", name, truncatePlan(plan))
	}
}

func truncatePlan(plan string) string {
	if len(plan) <= 500 {
		return plan
	}
	return plan[:500] + "..."
}

func mustParseQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	values, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("parse query %q: %v", raw, err)
	}
	return values
}

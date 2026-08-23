//go:build integration

package analytics

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/repository"
	"github.com/freight-platform/freight-cost-service/internal/security"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

const perf100kOrderCount = 100000

func TestFC22G1_PERF001_100kAnalyticsRebuild(t *testing.T) {
	if os.Getenv("PERF_100K") != "1" {
		t.Skip("set PERF_100K=1 to run controlled 100k verification")
	}
	env := setupAnalyticsEnv(t)
	ctx := context.Background()
	tenantID, buyerID, _ := seedPerf100kCanonical(ctx, t, env)

	start := time.Now()
	if err := env.analytics.RebuildTenant(ctx, tenantID); err != nil {
		t.Fatalf("100k rebuild: %v", err)
	}
	rebuildDuration := time.Since(start)
	t.Logf("REBUILD_DURATION_MS=%d", rebuildDuration.Milliseconds())

	orderFacts := countTable(ctx, env.pool, tenantID, "freight_cost.cost_analytics_order_fact")
	periods := countTable(ctx, env.pool, tenantID, "freight_cost.cost_analytics_period_projection")
	lanes := countTable(ctx, env.pool, tenantID, "freight_cost.cost_analytics_lane_period_projection")
	carriers := countTable(ctx, env.pool, tenantID, "freight_cost.cost_analytics_carrier_period_projection")
	accessorials := countTable(ctx, env.pool, tenantID, "freight_cost.cost_analytics_accessorial_fact")
	benchmarks := countTable(ctx, env.pool, tenantID, "freight_cost.cost_analytics_benchmark_projection")
	opportunities := countTable(ctx, env.pool, tenantID, "freight_cost.cost_analytics_opportunity_projection")

	t.Logf("ORDER_FACT_ROWS=%d PERIOD_ROWS=%d LANE_ROWS=%d CARRIER_ROWS=%d ACCESSORIAL_ROWS=%d BENCHMARK_ROWS=%d OPPORTUNITY_ROWS=%d",
		orderFacts, periods, lanes, carriers, accessorials, benchmarks, opportunities)
	if orderFacts != perf100kOrderCount {
		t.Fatalf("expected %d order facts, got %d", perf100kOrderCount, orderFacts)
	}

	publicSvc := newPerfPublicService(env)
	actor := security.TrustedActor{
		TenantID: tenantID, UserID: uuid.New(), CompanyID: buyerID, ActorKind: security.ActorKindBuyer,
	}
	runPerfPublicQueries(t, ctx, publicSvc, actor, tenantID, buyerID)
	runPerfExplainPlans(t, ctx, env.pool, tenantID, buyerID)
}

func seedPerf100kCanonical(ctx context.Context, t *testing.T, env *analyticsEnv) (tenantID, buyerID, carrierID uuid.UUID) {
	t.Helper()
	rng := rand.New(rand.NewSource(220001))
	tenantID = uuid.MustParse("11111111-1111-4111-8111-111111110001")
	buyerID = uuid.MustParse("22222222-2222-4222-8222-222222220001")
	carrierID = uuid.MustParse("33333333-3333-4333-8333-333333330001")
	period := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)

	if _, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, 'perf100k', 'Perf 100k Tenant') ON CONFLICT DO NOTHING`, tenantID); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
		VALUES ($1, $2, 'SHIPPER', 'Perf Buyer', 'ACTIVE') ON CONFLICT DO NOTHING`, buyerID, tenantID); err != nil {
		t.Fatalf("seed buyer: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
		VALUES ($1, $2, 'CARRIER', 'Perf Carrier', 'ACTIVE') ON CONFLICT DO NOTHING`, carrierID, tenantID); err != nil {
		t.Fatalf("seed carrier: %v", err)
	}

	const batchSize = 2000
	for offset := 0; offset < perf100kOrderCount; offset += batchSize {
		end := offset + batchSize
		if end > perf100kOrderCount {
			end = perf100kOrderCount
		}
		var builder strings.Builder
		builder.WriteString(`INSERT INTO freight_cost.cost_summary_projection (
			tenant_id, transport_order_id, buyer_company_id, carrier_company_id, currency_code,
			planned_amount, current_actual_amount, data_stage, financial_finality,
			billing_reconciliation_status, sources_available, projection_revision, updated_at
		) VALUES `)
		args := make([]any, 0, (end-offset)*13)
		argIdx := 1
		for i := offset; i < end; i++ {
			if i > offset {
				builder.WriteString(",")
			}
			orderID := perf100kOrderID(i)
			currency := "RUB"
			if i%50 == 0 {
				currency = "EUR"
			}
			carrier := carrierID
			if i%3 == 0 {
				carrier = uuid.MustParse("44444444-4444-4444-8444-444444440001")
			}
			amount := decimal.NewFromInt(int64(1000 + rng.Intn(5000))).StringFixed(2)
			builder.WriteString(fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,'CURRENT_ACTUAL_AVAILABLE','CURRENT_ACTUAL','UNLINKED',ARRAY['FREIGHT_SETTLEMENT'],1,$%d)",
				argIdx, argIdx+1, argIdx+2, argIdx+3, argIdx+4, argIdx+5, argIdx+6, argIdx+7))
			args = append(args, tenantID, orderID, buyerID, carrier, currency, amount, amount, period)
			argIdx += 8
		}
		builder.WriteString(` ON CONFLICT (tenant_id, transport_order_id) DO UPDATE SET
			current_actual_amount = EXCLUDED.current_actual_amount,
			updated_at = EXCLUDED.updated_at`)
		if _, err := env.pool.Exec(ctx, builder.String(), args...); err != nil {
			t.Fatalf("bulk seed batch offset=%d: %v", offset, err)
		}
	}
	return tenantID, buyerID, carrierID
}

func perf100kOrderID(index int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("a0000000-0000-4000-8000-%012d", index+1))
}

func newPerfPublicService(env *analyticsEnv) *service.AnalyticsPublicService {
	orderFacts := repository.NewAnalyticsOrderFactRepository(env.pool)
	state := repository.NewAnalyticsProjectionStateRepository(env.pool)
	return service.NewAnalyticsPublicService(env.analytics, orderFacts, state, true)
}

func runPerfPublicQueries(
	t *testing.T,
	ctx context.Context,
	publicSvc *service.AnalyticsPublicService,
	actor security.TrustedActor,
	tenantID, buyerID uuid.UUID,
) {
	t.Helper()
	type queryCase struct {
		name   string
		run    func() error
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
		qc.run() // warm-up
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
		p95 := durations[int(float64(len(durations))*0.95)]
		max := durations[len(durations)-1]
		t.Logf("%s_P50_MS=%d %s_P95_MS=%d %s_MAX_MS=%d", qc.name, p50.Milliseconds(), qc.name, p95.Milliseconds(), qc.name, max.Milliseconds())
	}
	_, err := publicSvc.ListLanes(ctx, actor, mustParseQuery(t, base+"&limit=500"))
	if err == nil {
		t.Fatal("expected limit=500 to be rejected")
	}
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
		if !strings.Contains(plan, "tenant_id") && !strings.Contains(strings.ToLower(plan), "index") {
			t.Logf("%s plan (no explicit tenant filter in text): %s", name, truncatePlan(plan))
		} else {
			t.Logf("%s_PLAN=%s", name, truncatePlan(plan))
		}
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

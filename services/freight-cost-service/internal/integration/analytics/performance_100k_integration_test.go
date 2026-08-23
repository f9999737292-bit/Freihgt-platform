//go:build integration

package analytics

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/repository"
	"github.com/freight-platform/freight-cost-service/internal/security"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

func TestFC22G1_PERF001_100kAnalyticsRebuild(t *testing.T) {
	if os.Getenv("PERF_100K") != "1" {
		t.Skip("set PERF_100K=1 to run controlled 100k verification")
	}
	if strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")) == "" {
		t.Fatal("TEST_DATABASE_URL is required when PERF_100K=1")
	}

	env := setupPerf100kEnv(t)
	ctx := context.Background()
	tenantID, buyerID := seedPerf100kCanonical(ctx, t, env)
	t.Logf("ORDER_COUNT=%d BUYER_COMPANIES=1 LANES=%d CARRIERS=%d CURRENCIES=RUB,EUR ACCESSORIAL_LINES=%d",
		perf100kOrderCount, perf100kLaneCount, perf100kCarrierCount, perf100kOrderCount/perf100kAccessorialEvery)

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
	assertPositiveCount(t, "lane", lanes)
	assertPositiveCount(t, "carrier", carriers)
	assertPositiveCount(t, "accessorial_fact", accessorials)
	assertPositiveCount(t, "benchmark", benchmarks)
	assertPositiveCount(t, "opportunity", opportunities)

	publicSvc := newPerfPublicService(env.analyticsEnv)
	actor := security.TrustedActor{
		TenantID: tenantID, UserID: uuid.New(), CompanyID: buyerID, ActorKind: security.ActorKindBuyer,
	}
	runPerfPublicQueries(t, ctx, publicSvc, actor, tenantID, buyerID)
	runPerfExplainPlans(t, ctx, env.pool, tenantID, buyerID)
}

func newPerfPublicService(env *analyticsEnv) *service.AnalyticsPublicService {
	orderFacts := repository.NewAnalyticsOrderFactRepository(env.pool)
	state := repository.NewAnalyticsProjectionStateRepository(env.pool)
	return service.NewAnalyticsPublicService(env.analytics, orderFacts, state, true)
}

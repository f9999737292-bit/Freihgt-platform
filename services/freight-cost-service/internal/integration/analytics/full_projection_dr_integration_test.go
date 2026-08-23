//go:build integration

package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/repository"
)

type fullProjectionEnv struct {
	*benchmarkEnv
	accessorialFacts   *repository.AnalyticsAccessorialFactRepository
	accessorialPeriods *repository.AnalyticsAccessorialPeriodProjectionRepository
	mappings           *repository.ChargeCodeMappingRepository
	state              *repository.AnalyticsProjectionStateRepository
}

func setupFullProjectionEnv(t *testing.T) *fullProjectionEnv {
	t.Helper()
	base := setupBenchmarkEnv(t)
	accessorialFacts := repository.NewAnalyticsAccessorialFactRepository(base.pool)
	accessorialPeriods := repository.NewAnalyticsAccessorialPeriodProjectionRepository(base.pool)
	mappings := repository.NewChargeCodeMappingRepository(base.pool)
	dimensions := newDBTransportDimensionReader(base.pool)
	companies := newDBCompanyDisplayReader(base.pool)
	settlements := newDBSettlementAccessorialReader(base.pool)
	base.analytics = newAnalyticsService(
		base.pool, base.summaries, base.periods, base.lanes, base.carriers,
		accessorialFacts, accessorialPeriods, base.coverage, mappings,
		dimensions, companies, settlements,
	)
	return &fullProjectionEnv{
		benchmarkEnv:       base,
		accessorialFacts:   accessorialFacts,
		accessorialPeriods: accessorialPeriods,
		mappings:           mappings,
		state:              repository.NewAnalyticsProjectionStateRepository(base.pool),
	}
}

type fullProjectionFixture struct {
	tenantID   uuid.UUID
	buyerID    uuid.UUID
	carrierA   uuid.UUID
	carrierB   uuid.UUID
	period     time.Time
	orderCount int
}

func seedFullProjectionFixture(t *testing.T, env *fullProjectionEnv) fullProjectionFixture {
	t.Helper()
	ctx := context.Background()
	fix := fullProjectionFixture{
		tenantID: uuid.New(),
		buyerID:  uuid.New(),
		carrierA: uuid.New(),
		carrierB: uuid.New(),
		period:   time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
	}
	equipment := "TENT"
	lcEnv := env.laneCarrierEnv

	if _, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		fix.tenantID, "t-"+fix.tenantID.String()[:8], "Full Projection Tenant"); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
		VALUES ($1, $2, 'SHIPPER', 'Buyer Co', 'ACTIVE') ON CONFLICT DO NOTHING`, fix.buyerID, fix.tenantID); err != nil {
		t.Fatalf("seed buyer: %v", err)
	}

	seedCarrierCompany(t, &accessorialEnv{analyticsEnv: env.analyticsEnv, mappings: env.mappings}, fix.tenantID, fix.carrierA, "Carrier Alpha LLC", "AlphaCarrier")
	seedCarrierCompany(t, &accessorialEnv{analyticsEnv: env.analyticsEnv, mappings: env.mappings}, fix.tenantID, fix.carrierB, "Carrier Beta LLC", "BetaCarrier")

	pinTime := fix.period
	seedChargeMapping(t, &accessorialEnv{analyticsEnv: env.analyticsEnv, mappings: env.mappings}, fix.tenantID, "DETENTION", "DETENTION", 10, pinTime.Add(-72*time.Hour))
	seedChargeMapping(t, &accessorialEnv{analyticsEnv: env.analyticsEnv, mappings: env.mappings}, fix.tenantID, "DETENTION", "OTHER", 20, pinTime.Add(24*time.Hour))

	laneBenchmarkAmounts := []string{"38000.00", "38000.00", "38000.00", "38000.00", "45000.00"}
	for i, amount := range laneBenchmarkAmounts {
		orderID := uuid.New()
		carrierID := fix.carrierA
		if i%2 == 1 {
			carrierID = fix.carrierB
		}
		seedTransportOrderWithLocations(t, lcEnv, fix.tenantID, fix.buyerID, orderID, "Moscow", "SPB", "ROAD", &equipment)
		upsertSummary(t, env.analyticsEnv, fix.tenantID, fix.buyerID, carrierID, orderID, "RUB", amount, amount, fix.period)
		pinSummaryAttribution(t, &accessorialEnv{analyticsEnv: env.analyticsEnv}, fix.tenantID, orderID, 10, pinTime)
		fix.orderCount++

		if i == 0 {
			settlementID := uuid.New()
			var originID, destID uuid.UUID
			if err := env.pool.QueryRow(ctx, `
				SELECT origin_location_id, destination_location_id
				FROM transport.transport_orders WHERE id = $1`, orderID).Scan(&originID, &destID); err != nil {
				t.Fatalf("load locations: %v", err)
			}
			seedSettlementWithAccessorials(t, &accessorialEnv{analyticsEnv: env.analyticsEnv, mappings: env.mappings},
				fix.tenantID, fix.buyerID, carrierID, orderID, settlementID, originID, destID, []settlementAccessorialSeed{
					{chargeCode: "DETENTION", amount: decimal.RequireFromString("150.00"), status: domain.AccessorialStatusApproved},
				})
		}
	}

	eurOrderID := uuid.New()
	seedTransportOrderWithLocations(t, lcEnv, fix.tenantID, fix.buyerID, eurOrderID, "Moscow", "Kazan", "ROAD", &equipment)
	upsertSummary(t, env.analyticsEnv, fix.tenantID, fix.buyerID, fix.carrierB, eurOrderID, "EUR", "1200.00", "1250.00", fix.period)
	fix.orderCount++

	if err := env.analytics.RebuildTenant(ctx, fix.tenantID); err != nil {
		t.Fatalf("initial rebuild: %v", err)
	}
	return fix
}

func assertFullProjectionLayersPopulated(t *testing.T, ctx context.Context, env *fullProjectionEnv, tenantID uuid.UUID) {
	t.Helper()
	assertPositiveCount(t, "lane", countTable(ctx, env.pool, tenantID, "freight_cost.cost_analytics_lane_period_projection"))
	assertPositiveCount(t, "carrier", countTable(ctx, env.pool, tenantID, "freight_cost.cost_analytics_carrier_period_projection"))
	assertPositiveCount(t, "accessorial_fact", countTable(ctx, env.pool, tenantID, "freight_cost.cost_analytics_accessorial_fact"))
	assertPositiveCount(t, "benchmark", countTable(ctx, env.pool, tenantID, "freight_cost.cost_analytics_benchmark_projection"))
	assertPositiveCount(t, "opportunity", countTable(ctx, env.pool, tenantID, "freight_cost.cost_analytics_opportunity_projection"))
}

func countTable(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, table string) int {
	var count int
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+table+" WHERE tenant_id = $1", tenantID).Scan(&count)
	return count
}

func loadFirstAccessorialFact(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) (category string, mappingVersion int64, err error) {
	err = pool.QueryRow(ctx, `
		SELECT normalized_category, mapping_version
		FROM freight_cost.cost_analytics_accessorial_fact
		WHERE tenant_id = $1
		ORDER BY accessorial_id
		LIMIT 1`, tenantID).Scan(&category, &mappingVersion)
	return category, mappingVersion, err
}

func assertPositiveCount(t *testing.T, label string, count int) {
	t.Helper()
	if count <= 0 {
		t.Fatalf("expected %s rows > 0, got %d", label, count)
	}
}

func TestFC22G1_FullProjectionLossAndRebuildRestoresBusinessState(t *testing.T) {
	env := setupFullProjectionEnv(t)
	ctx := context.Background()
	fix := seedFullProjectionFixture(t, env)
	assertFullProjectionLayersPopulated(t, ctx, env, fix.tenantID)

	preCanonical, err := SnapshotCanonicalSourceCounts(ctx, env.pool, fix.tenantID)
	if err != nil {
		t.Fatalf("canonical snapshot: %v", err)
	}
	preLoss, err := ComputeAnalyticsBusinessChecksum(ctx, env.pool, fix.tenantID)
	if err != nil {
		t.Fatalf("pre-loss checksum: %v", err)
	}
	preOppIDs, err := ListOpportunityIDs(ctx, env.pool, fix.tenantID)
	if err != nil || len(preOppIDs) == 0 {
		t.Fatalf("pre-loss opportunities: err=%v len=%d", err, len(preOppIDs))
	}

	category, mappingVersion, err := loadFirstAccessorialFact(ctx, env.pool, fix.tenantID)
	if err != nil {
		t.Fatalf("accessorial facts before loss: %v", err)
	}
	if mappingVersion != 10 || category != "DETENTION" {
		t.Fatalf("expected pinned DETENTION v10 before loss, got category=%s version=%d", category, mappingVersion)
	}

	if err := DeleteDerivedAnalyticsForTenant(ctx, env.pool, fix.tenantID); err != nil {
		t.Fatalf("simulate loss: %v", err)
	}
	derivedAfterLoss, err := CountDerivedAnalyticsRows(ctx, env.pool, fix.tenantID)
	if err != nil {
		t.Fatalf("count derived after loss: %v", err)
	}
	if derivedAfterLoss != 0 {
		t.Fatalf("expected zero derived rows after loss, got %d", derivedAfterLoss)
	}
	postLossCanonical, err := SnapshotCanonicalSourceCounts(ctx, env.pool, fix.tenantID)
	if err != nil {
		t.Fatalf("canonical after loss: %v", err)
	}
	if !CanonicalSourceCountsEqual(preCanonical, postLossCanonical) {
		t.Fatalf("canonical source changed after derived loss: before=%+v after=%+v", preCanonical, postLossCanonical)
	}

	if err := env.analytics.RebuildTenant(ctx, fix.tenantID); err != nil {
		t.Fatalf("rebuild after loss: %v", err)
	}
	postRebuild, err := ComputeAnalyticsBusinessChecksum(ctx, env.pool, fix.tenantID)
	if err != nil {
		t.Fatalf("post-rebuild checksum: %v", err)
	}
	if preLoss.Combined != postRebuild.Combined {
		t.Fatalf("combined checksum mismatch pre=%s post=%s", preLoss.Combined, postRebuild.Combined)
	}
	for layer, pre := range preLoss.Layers {
		post := postRebuild.Layers[layer]
		if pre != post {
			t.Fatalf("layer %s checksum mismatch pre=%s post=%s", layer, pre, post)
		}
	}

	postOppIDs, err := ListOpportunityIDs(ctx, env.pool, fix.tenantID)
	if err != nil {
		t.Fatalf("post opportunity ids: %v", err)
	}
	if len(preOppIDs) != len(postOppIDs) {
		t.Fatalf("opportunity count changed pre=%d post=%d", len(preOppIDs), len(postOppIDs))
	}
	preSet := map[uuid.UUID]struct{}{}
	for _, id := range preOppIDs {
		preSet[id] = struct{}{}
	}
	for _, id := range postOppIDs {
		if _, ok := preSet[id]; !ok {
			t.Fatalf("opportunity id changed after rebuild: %s", id)
		}
	}

	postCategory, postMappingVersion, err := loadFirstAccessorialFact(ctx, env.pool, fix.tenantID)
	if err != nil {
		t.Fatalf("accessorial facts after rebuild: %v", err)
	}
	if postMappingVersion != 10 || postCategory != "DETENTION" {
		t.Fatalf("pinned mapping not restored: category=%s version=%d", postCategory, postMappingVersion)
	}
}

func TestFC22G1_FailedRebuildDoesNotPublishPartialFreshState(t *testing.T) {
	env := setupFullProjectionEnv(t)
	ctx := context.Background()
	fix := seedFullProjectionFixture(t, env)

	preLoss, err := ComputeAnalyticsBusinessChecksum(ctx, env.pool, fix.tenantID)
	if err != nil {
		t.Fatalf("pre-failure checksum: %v", err)
	}
	preState, err := env.state.Get(ctx, nil, domain.AnalyticsProjectionNamePeriod, fix.tenantID)
	if err != nil {
		t.Fatalf("pre-failure state: %v", err)
	}

	failingSettlements := newFailingSettlementReader(newDBSettlementAccessorialReader(env.pool))
	failingSettlements.setFailNext(true)
	dimensions := newDBTransportDimensionReader(env.pool)
	companies := newDBCompanyDisplayReader(env.pool)
	env.analytics = newAnalyticsService(
		env.pool, env.summaries, env.periods, env.lanes, env.carriers,
		env.accessorialFacts, env.accessorialPeriods, env.coverage, env.mappings,
		dimensions, companies, failingSettlements,
	)

	if err := env.analytics.RebuildTenant(ctx, fix.tenantID); err == nil {
		t.Fatal("expected injected rebuild failure")
	}

	postFailure, err := ComputeAnalyticsBusinessChecksum(ctx, env.pool, fix.tenantID)
	if err != nil {
		t.Fatalf("post-failure checksum: %v", err)
	}
	if preLoss.Combined != postFailure.Combined {
		t.Fatalf("failed rebuild mutated business state pre=%s post=%s", preLoss.Combined, postFailure.Combined)
	}

	currentState, err := env.state.Get(ctx, nil, domain.AnalyticsProjectionNamePeriod, fix.tenantID)
	if err != nil {
		t.Fatalf("post-failure state read: %v", err)
	}
	if preState == nil || currentState == nil {
		t.Fatal("expected projection state before and after failure")
	}
	if currentState.Status == domain.AnalyticsProjectionStatusReady && preState.LastSuccessfulRunAt != nil &&
		currentState.LastSuccessfulRunAt != nil && currentState.LastSuccessfulRunAt.After(*preState.LastSuccessfulRunAt) {
		t.Fatalf("state falsely marked fresh after failed rebuild: %+v", currentState)
	}
}

func TestFC22G1_RetryAfterFailureRestoresBusinessState(t *testing.T) {
	env := setupFullProjectionEnv(t)
	ctx := context.Background()
	fix := seedFullProjectionFixture(t, env)

	expected, err := ComputeAnalyticsBusinessChecksum(ctx, env.pool, fix.tenantID)
	if err != nil {
		t.Fatalf("expected checksum: %v", err)
	}

	failingSettlements := newFailingSettlementReader(newDBSettlementAccessorialReader(env.pool))
	failingSettlements.setFailNext(true)
	dimensions := newDBTransportDimensionReader(env.pool)
	companies := newDBCompanyDisplayReader(env.pool)
	env.analytics = newAnalyticsService(
		env.pool, env.summaries, env.periods, env.lanes, env.carriers,
		env.accessorialFacts, env.accessorialPeriods, env.coverage, env.mappings,
		dimensions, companies, failingSettlements,
	)
	if err := env.analytics.RebuildTenant(ctx, fix.tenantID); err == nil {
		t.Fatal("expected first rebuild failure")
	}

	env.analytics = newAnalyticsService(
		env.pool, env.summaries, env.periods, env.lanes, env.carriers,
		env.accessorialFacts, env.accessorialPeriods, env.coverage, env.mappings,
		dimensions, companies, newDBSettlementAccessorialReader(env.pool),
	)
	if err := env.analytics.RebuildTenant(ctx, fix.tenantID); err != nil {
		t.Fatalf("retry rebuild: %v", err)
	}

	afterRetry, err := ComputeAnalyticsBusinessChecksum(ctx, env.pool, fix.tenantID)
	if err != nil {
		t.Fatalf("after retry checksum: %v", err)
	}
	if expected.Combined != afterRetry.Combined {
		t.Fatalf("retry checksum mismatch expected=%s got=%s", expected.Combined, afterRetry.Combined)
	}
}

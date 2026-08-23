//go:build integration

package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	fcmetrics "github.com/freight-platform/freight-cost-service/internal/platform/metrics"
	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/provider"
	"github.com/freight-platform/freight-cost-service/internal/repository"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

type laneCarrierEnv struct {
	*analyticsEnv
	lanes    *repository.AnalyticsLanePeriodProjectionRepository
	carriers *repository.AnalyticsCarrierPeriodProjectionRepository
	coverage *repository.AnalyticsProjectionCoverageRepository
}

func setupLaneCarrierEnv(t *testing.T) *laneCarrierEnv {
	t.Helper()
	base := setupAnalyticsEnv(t)
	lanes := repository.NewAnalyticsLanePeriodProjectionRepository(base.pool)
	carriers := repository.NewAnalyticsCarrierPeriodProjectionRepository(base.pool)
	coverage := repository.NewAnalyticsProjectionCoverageRepository(base.pool)
	dimensions := newDBTransportDimensionReader(base.pool)
	mappings := repository.NewChargeCodeMappingRepository(base.pool)
	accessorialFacts := repository.NewAnalyticsAccessorialFactRepository(base.pool)
	accessorialPeriods := repository.NewAnalyticsAccessorialPeriodProjectionRepository(base.pool)
	base.analytics = newAnalyticsService(
		base.pool, base.summaries, base.periods, lanes, carriers,
		accessorialFacts, accessorialPeriods, coverage, mappings,
		dimensions, nil, nil,
	)
	return &laneCarrierEnv{analyticsEnv: base, lanes: lanes, carriers: carriers, coverage: coverage}
}

func newAnalyticsService(
	pool *pgxpool.Pool,
	summaries *repository.CostSummaryProjectionRepository,
	periods *repository.AnalyticsPeriodProjectionRepository,
	lanes *repository.AnalyticsLanePeriodProjectionRepository,
	carriers *repository.AnalyticsCarrierPeriodProjectionRepository,
	accessorialFacts *repository.AnalyticsAccessorialFactRepository,
	accessorialPeriods *repository.AnalyticsAccessorialPeriodProjectionRepository,
	coverage *repository.AnalyticsProjectionCoverageRepository,
	mappings *repository.ChargeCodeMappingRepository,
	dimensions provider.TransportDimensionReader,
	companies provider.CompanyDisplayReader,
	settlements provider.SettlementAccessorialReader,
) *service.AnalyticsProjectionService {
	orderFacts := repository.NewAnalyticsOrderFactRepository(pool)
	state := repository.NewAnalyticsProjectionStateRepository(pool)
	dirty := repository.NewAnalyticsDirtyQueueRepository(pool)
	benchmarks := repository.NewAnalyticsBenchmarkProjectionRepository(pool)
	opportunities := repository.NewAnalyticsOpportunityProjectionRepository(pool)
	attributions := repository.NewVarianceAttributionRepository()
	metrics := fcmetrics.New()
	benchmarkConfig := domain.AnalyticsBenchmarkConfig{
		MinBenchmarkSample:             domain.DefaultMinBenchmarkSample,
		RepeatedVarianceMinOccurrences: domain.DefaultRepeatedVarianceMinOccurrences,
	}
	return service.NewAnalyticsProjectionService(
		pool, summaries, orderFacts, periods, lanes, carriers,
		accessorialFacts, accessorialPeriods, benchmarks, opportunities, attributions,
		coverage, state, dirty, mappings, dimensions, companies, settlements,
		benchmarkConfig, metrics,
	)
}

func seedTransportOrderWithLocations(
	t *testing.T,
	env *laneCarrierEnv,
	tenantID, buyerID uuid.UUID,
	orderID uuid.UUID,
	originCity, destCity string,
	transportMode string,
	equipment *string,
) {
	t.Helper()
	ctx := context.Background()
	originID := uuid.New()
	destID := uuid.New()
	cargoID := uuid.New()
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, tenantID, "t-"+tenantID.String()[:8], "Analytics Tenant"); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
		VALUES ($1, $2, 'SHIPPER', 'Buyer Co', 'ACTIVE')
		ON CONFLICT DO NOTHING`, buyerID, tenantID); err != nil {
		t.Fatalf("seed company: %v", err)
	}
	_, err := env.pool.Exec(ctx, `
		INSERT INTO transport.locations (
			id, tenant_id, company_id, location_type, name, country_code, city, timezone, status, version
		) VALUES ($1, $2, $3, 'WAREHOUSE', 'Origin', 'RU', $4, 'Europe/Moscow', 'ACTIVE', 1),
		         ($5, $2, $3, 'WAREHOUSE', 'Destination', 'RU', $6, 'Europe/Moscow', 'ACTIVE', 1)`,
		originID, tenantID, buyerID, originCity, destID, destCity)
	if err != nil {
		t.Fatalf("seed locations: %v", err)
	}
	_, err = env.pool.Exec(ctx, `
		INSERT INTO transport.cargoes (id, tenant_id, cargo_type, description, gross_weight)
		VALUES ($1, $2, 'GENERAL', 'test cargo', 1000)`, cargoID, tenantID)
	if err != nil {
		t.Fatalf("seed cargo: %v", err)
	}
	_, err = env.pool.Exec(ctx, `
		INSERT INTO transport.transport_orders (
			id, tenant_id, order_number, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id,
			transport_mode, equipment_type, status, pricing_model_version
		) VALUES ($1, $2, $3, $4, $4, $5, $6, $7, $8, $9, 'CONVERTED_TO_SHIPMENT', 'SNAPSHOT_V1')
		ON CONFLICT (id) DO UPDATE SET
			origin_location_id = EXCLUDED.origin_location_id,
			destination_location_id = EXCLUDED.destination_location_id,
			transport_mode = EXCLUDED.transport_mode,
			equipment_type = EXCLUDED.equipment_type`,
		orderID, tenantID, "TO-"+orderID.String()[:8], buyerID,
		originID, destID, cargoID, transportMode, equipment)
	if err != nil {
		t.Fatalf("seed transport order: %v", err)
	}
}

func TestFC22CLP001OneLaneOneCurrency(t *testing.T) {
	env := setupLaneCarrierEnv(t)
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	orderID := uuid.New()
	period := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	equipment := "TENT"
	seedTransportOrderWithLocations(t, env, tenantID, buyerID, orderID, "Moscow", "SPB", "ROAD", &equipment)
	upsertSummary(t, env.analyticsEnv, tenantID, buyerID, carrierID, orderID, "RUB", "1000.00", "1100.00", period)

	if err := env.analytics.RebuildTenant(context.Background(), tenantID); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	laneKey := domain.BuildLaneKey(domain.LaneKeyInput{
		OriginCountry: "RU", OriginCity: "Moscow",
		DestinationCountry: "RU", DestinationCity: "SPB",
		TransportMode: "ROAD", EquipmentType: "TENT",
	}).LaneKey
	items, err := env.lanes.List(context.Background(), tenantID, repository.AnalyticsLaneListFilter{
		BuyerCompanyID: &buyerID,
		CurrencyCode:   "RUB",
		LaneKey:        laneKey,
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("list lanes: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 lane row, got %d", len(items))
	}
	if items[0].OrderCount != 1 || !decimal.RequireFromString("1100.00").Equal(*items[0].CurrentActualTotal) {
		t.Fatalf("unexpected lane projection: %+v", items[0])
	}
}

func TestFC22CLP004MultipleCurrenciesSeparate(t *testing.T) {
	env := setupLaneCarrierEnv(t)
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	orderRUB := uuid.New()
	orderEUR := uuid.New()
	period := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	equipment := "TENT"
	seedTransportOrderWithLocations(t, env, tenantID, buyerID, orderRUB, "Moscow", "SPB", "ROAD", &equipment)
	seedTransportOrderWithLocations(t, env, tenantID, buyerID, orderEUR, "Moscow", "SPB", "ROAD", &equipment)
	upsertSummary(t, env.analyticsEnv, tenantID, buyerID, carrierID, orderRUB, "RUB", "100.00", "110.00", period)
	upsertSummary(t, env.analyticsEnv, tenantID, buyerID, carrierID, orderEUR, "EUR", "200.00", "250.00", period)

	if err := env.analytics.RebuildTenant(context.Background(), tenantID); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	rubRows, _ := env.lanes.List(context.Background(), tenantID, repository.AnalyticsLaneListFilter{CurrencyCode: "RUB", Limit: 10})
	eurRows, _ := env.lanes.List(context.Background(), tenantID, repository.AnalyticsLaneListFilter{CurrencyCode: "EUR", Limit: 10})
	if len(rubRows) != 1 || len(eurRows) != 1 {
		t.Fatalf("expected separate currency rows rub=%d eur=%d", len(rubRows), len(eurRows))
	}
	if rubRows[0].CurrentActualTotal.Equal(*eurRows[0].CurrentActualTotal) {
		t.Fatal("RUB and EUR must not share aggregated totals")
	}
}

func TestFC22CEqv001RebuildMatchesIncremental(t *testing.T) {
	env := setupLaneCarrierEnv(t)
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	orderID := uuid.New()
	period := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	equipment := "TENT"
	seedTransportOrderWithLocations(t, env, tenantID, buyerID, orderID, "Moscow", "SPB", "ROAD", &equipment)
	upsertSummary(t, env.analyticsEnv, tenantID, buyerID, carrierID, orderID, "RUB", "300.00", "330.00", period)

	if err := env.analytics.RebuildTenant(context.Background(), tenantID); err != nil {
		t.Fatalf("full rebuild: %v", err)
	}
	fullLanes, err := env.lanes.List(context.Background(), tenantID, repository.AnalyticsLaneListFilter{Limit: 20})
	if err != nil || len(fullLanes) != 1 {
		t.Fatalf("full lane rows: %v len=%d", err, len(fullLanes))
	}
	fullCarriers, err := env.carriers.List(context.Background(), tenantID, repository.AnalyticsCarrierListFilter{Limit: 20})
	if err != nil || len(fullCarriers) != 1 {
		t.Fatalf("full carrier rows: %v len=%d", err, len(fullCarriers))
	}

	ctx := context.Background()
	_, _ = env.pool.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_lane_period_projection WHERE tenant_id = $1`, tenantID)
	_, _ = env.pool.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_carrier_period_projection WHERE tenant_id = $1`, tenantID)
	_, _ = env.pool.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_order_fact WHERE tenant_id = $1`, tenantID)
	_, _ = env.pool.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_period_projection WHERE tenant_id = $1`, tenantID)
	_, _ = env.pool.Exec(ctx, `DELETE FROM freight_cost.analytics_projection_state WHERE tenant_id = $1`, tenantID)
	_, _ = env.pool.Exec(ctx, `DELETE FROM freight_cost.analytics_projection_coverage WHERE tenant_id = $1`, tenantID)

	tx, err := env.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := env.analytics.MarkCostSummaryChanged(ctx, tx, serviceAnalyticsChange(tenantID, buyerID, orderID, "RUB", period)); err != nil {
		t.Fatalf("mark dirty: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := env.analytics.ProcessDirtyBatch(ctx, 10); err != nil {
		t.Fatalf("incremental: %v", err)
	}

	incrLanes, err := env.lanes.List(ctx, tenantID, repository.AnalyticsLaneListFilter{Limit: 20})
	if err != nil || len(incrLanes) != 1 {
		t.Fatalf("incr lane rows: %v len=%d", err, len(incrLanes))
	}
	incrCarriers, err := env.carriers.List(ctx, tenantID, repository.AnalyticsCarrierListFilter{Limit: 20})
	if err != nil || len(incrCarriers) != 1 {
		t.Fatalf("incr carrier rows: %v len=%d", err, len(incrCarriers))
	}
	if fullLanes[0].OrderCount != incrLanes[0].OrderCount ||
		!fullLanes[0].CurrentActualTotal.Equal(*incrLanes[0].CurrentActualTotal) {
		t.Fatalf("lane equivalence failed full=%+v incr=%+v", fullLanes[0], incrLanes[0])
	}
	if fullCarriers[0].OrderCount != incrCarriers[0].OrderCount ||
		!fullCarriers[0].CurrentActualTotal.Equal(*incrCarriers[0].CurrentActualTotal) {
		t.Fatalf("carrier equivalence failed full=%+v incr=%+v", fullCarriers[0], incrCarriers[0])
	}
}

func TestFC22CSEC001TenantIsolation(t *testing.T) {
	env := setupLaneCarrierEnv(t)
	tenantA := uuid.New()
	tenantB := uuid.New()
	buyerA := uuid.New()
	buyerB := uuid.New()
	carrier := uuid.New()
	orderA := uuid.New()
	orderB := uuid.New()
	period := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	equipment := "TENT"
	seedTransportOrderWithLocations(t, env, tenantA, buyerA, orderA, "Moscow", "SPB", "ROAD", &equipment)
	seedTransportOrderWithLocations(t, env, tenantB, buyerB, orderB, "Moscow", "Kazan", "ROAD", &equipment)
	upsertSummary(t, env.analyticsEnv, tenantA, buyerA, carrier, orderA, "RUB", "100.00", "120.00", period)
	upsertSummary(t, env.analyticsEnv, tenantB, buyerB, carrier, orderB, "RUB", "200.00", "240.00", period)
	if err := env.analytics.RebuildTenant(context.Background(), tenantA); err != nil {
		t.Fatalf("rebuild A: %v", err)
	}
	rowsB, err := env.lanes.List(context.Background(), tenantB, repository.AnalyticsLaneListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list B: %v", err)
	}
	if len(rowsB) != 0 {
		t.Fatalf("tenant B must not appear when querying with tenant B scope before rebuild")
	}
}

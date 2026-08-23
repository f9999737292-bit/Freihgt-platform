//go:build integration

package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/repository"
)

const nPlusOneOrderCount = 120

func TestFC22G_NPlusOne001EnrichmentUsesBatchNotPerOrder(t *testing.T) {
	env := setupAccessorialEnv(t)
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	period := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	equipment := "TENT"
	lcEnv := &laneCarrierEnv{analyticsEnv: env.analyticsEnv}

	ctx := context.Background()
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		tenantID, "t-"+tenantID.String()[:8], "N+1 Tenant"); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
		VALUES ($1, $2, 'SHIPPER', 'Buyer Co', 'ACTIVE') ON CONFLICT DO NOTHING`, buyerID, tenantID); err != nil {
		t.Fatalf("seed buyer company: %v", err)
	}

	seedCarrierCompany(t, env, tenantID, carrierID, "Carrier Batch Co", "BatchCo")
	seedChargeMapping(t, env, tenantID, "DETENTION", "DETENTION", 10, period.Add(-24*time.Hour))

	companies := wrapCountingCompany(newDBCompanyDisplayReader(env.pool))
	dimensions := wrapCountingDimensions(newDBTransportDimensionReader(env.pool))
	settlements := wrapCountingSettlements(newDBSettlementAccessorialReader(env.pool))
	lanes := repository.NewAnalyticsLanePeriodProjectionRepository(env.pool)
	carriers := repository.NewAnalyticsCarrierPeriodProjectionRepository(env.pool)
	coverage := repository.NewAnalyticsProjectionCoverageRepository(env.pool)
	env.analytics = newAnalyticsService(
		env.pool, env.summaries, env.periods, lanes, carriers,
		env.accessorialFacts, env.accessorialPeriods, coverage, env.mappings,
		dimensions, companies, settlements,
	)

	for i := 0; i < nPlusOneOrderCount; i++ {
		orderID := uuid.New()
		settlementID := uuid.New()
		seedTransportOrderWithLocations(t, lcEnv, tenantID, buyerID, orderID, "Moscow", "SPB", "ROAD", &equipment)
		upsertSummary(t, env.analyticsEnv, tenantID, buyerID, carrierID, orderID, "RUB", "1000.00", "950.00", period)
		pinSummaryAttribution(t, env, tenantID, orderID, 10, period)
		var originID, destID uuid.UUID
		if err := env.pool.QueryRow(ctx, `
			SELECT origin_location_id, destination_location_id
			FROM transport.transport_orders WHERE id = $1`, orderID).Scan(&originID, &destID); err != nil {
			t.Fatalf("load locations: %v", err)
		}
		seedSettlementWithAccessorials(t, env, tenantID, buyerID, carrierID, orderID, settlementID, originID, destID, []settlementAccessorialSeed{
			{chargeCode: "DETENTION", amount: decimal.RequireFromString("50.00"), status: domain.AccessorialStatusApproved},
		})
	}

	if err := env.analytics.RebuildTenant(ctx, tenantID); err != nil {
		t.Fatalf("rebuild tenant: %v", err)
	}

	companyCalls, companyIDs := companies.snapshot()
	dimensionCalls, dimensionIDs := dimensions.snapshot()
	settlementCalls, settlementIDs := settlements.snapshot()

	if companyCalls == 0 || dimensionCalls == 0 || settlementCalls == 0 {
		t.Fatalf("expected batch enrichment calls company=%d dimension=%d settlement=%d", companyCalls, dimensionCalls, settlementCalls)
	}
	if companyCalls >= nPlusOneOrderCount || dimensionCalls >= nPlusOneOrderCount || settlementCalls >= nPlusOneOrderCount {
		t.Fatalf("N+1 detected: company=%d dimension=%d settlement=%d for %d orders", companyCalls, dimensionCalls, settlementCalls, nPlusOneOrderCount)
	}
	if companyIDs < 1 || dimensionIDs < nPlusOneOrderCount || settlementIDs < nPlusOneOrderCount {
		t.Fatalf("batch id counts company=%d dimension=%d settlement=%d want >=1, >=%d, >=%d", companyIDs, dimensionIDs, settlementIDs, nPlusOneOrderCount, nPlusOneOrderCount)
	}
	t.Logf("N+1 gate: orders=%d company_calls=%d dimension_calls=%d settlement_calls=%d", nPlusOneOrderCount, companyCalls, dimensionCalls, settlementCalls)
}

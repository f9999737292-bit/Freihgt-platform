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

type accessorialEnv struct {
	*analyticsEnv
	accessorialFacts   *repository.AnalyticsAccessorialFactRepository
	accessorialPeriods *repository.AnalyticsAccessorialPeriodProjectionRepository
	orderFacts         *repository.AnalyticsOrderFactRepository
	mappings           *repository.ChargeCodeMappingRepository
}

func setupAccessorialEnv(t *testing.T) *accessorialEnv {
	t.Helper()
	base := setupAnalyticsEnv(t)
	accessorialFacts := repository.NewAnalyticsAccessorialFactRepository(base.pool)
	accessorialPeriods := repository.NewAnalyticsAccessorialPeriodProjectionRepository(base.pool)
	orderFacts := repository.NewAnalyticsOrderFactRepository(base.pool)
	coverage := repository.NewAnalyticsProjectionCoverageRepository(base.pool)
	lanes := repository.NewAnalyticsLanePeriodProjectionRepository(base.pool)
	carriers := repository.NewAnalyticsCarrierPeriodProjectionRepository(base.pool)
	mappings := repository.NewChargeCodeMappingRepository(base.pool)
	dimensions := newDBTransportDimensionReader(base.pool)
	companies := newDBCompanyDisplayReader(base.pool)
	settlements := newDBSettlementAccessorialReader(base.pool)
	base.analytics = newAnalyticsService(
		base.pool, base.summaries, base.periods, lanes, carriers,
		accessorialFacts, accessorialPeriods, coverage, mappings,
		dimensions, companies, settlements,
	)
	return &accessorialEnv{
		analyticsEnv:       base,
		accessorialFacts:   accessorialFacts,
		accessorialPeriods: accessorialPeriods,
		orderFacts:         orderFacts,
		mappings:           mappings,
	}
}

func seedCarrierCompany(t *testing.T, env *accessorialEnv, tenantID, carrierID uuid.UUID, legalName, shortName string) {
	t.Helper()
	ctx := context.Background()
	_, err := env.pool.Exec(ctx, `
		INSERT INTO core.companies (id, tenant_id, company_type, legal_name, short_name, status)
		VALUES ($1, $2, 'CARRIER', $3, $4, 'ACTIVE')
		ON CONFLICT (id) DO UPDATE SET legal_name = EXCLUDED.legal_name, short_name = EXCLUDED.short_name`,
		carrierID, tenantID, legalName, shortName)
	if err != nil {
		t.Fatalf("seed carrier company: %v", err)
	}
}

func seedChargeMapping(
	t *testing.T,
	env *accessorialEnv,
	tenantID uuid.UUID,
	sourceCode, category string,
	version int64,
	effectiveFrom time.Time,
) {
	t.Helper()
	ctx := context.Background()
	normalized, err := domain.NormalizeChargeCode(sourceCode)
	if err != nil {
		t.Fatalf("normalize charge code: %v", err)
	}
	categoryNorm, err := domain.NormalizeMappingCategory(category)
	if err != nil {
		t.Fatalf("normalize category: %v", err)
	}
	_, err = env.pool.Exec(ctx, `
		INSERT INTO freight_cost.charge_code_mapping (
			mapping_scope, tenant_id, source_charge_code_normalized, normalized_category,
			mapping_version, effective_from
		) VALUES ('TENANT', $1, $2, $3, $4, $5)`,
		tenantID, normalized, categoryNorm, version, effectiveFrom.UTC())
	if err != nil {
		t.Fatalf("seed mapping: %v", err)
	}
}

func pinSummaryAttribution(t *testing.T, env *accessorialEnv, tenantID, orderID uuid.UUID, version int64, evaluatedAt time.Time) {
	t.Helper()
	_, err := env.pool.Exec(context.Background(), `
		UPDATE freight_cost.cost_summary_projection
		SET attribution_mapping_version = $3, attribution_mapping_evaluated_at = $4
		WHERE tenant_id = $1 AND transport_order_id = $2`,
		tenantID, orderID, version, evaluatedAt)
	if err != nil {
		t.Fatalf("pin attribution: %v", err)
	}
}

func seedSettlementWithAccessorials(
	t *testing.T,
	env *accessorialEnv,
	tenantID, buyerID, carrierID, orderID uuid.UUID,
	settlementID uuid.UUID,
	originID, destID uuid.UUID,
	lines []settlementAccessorialSeed,
) {
	t.Helper()
	ctx := context.Background()
	shipmentID := uuid.New()
	_, err := env.pool.Exec(ctx, `
		INSERT INTO transport.shipments (
			id, tenant_id, shipment_number, transport_order_id,
			shipper_company_id, consignee_company_id, carrier_company_id,
			origin_location_id, destination_location_id, transport_mode, status
		) VALUES ($1, $2, $3, $4, $5, $5, $6, $7, $8, 'ROAD', 'READY_FOR_BILLING')`,
		shipmentID, tenantID, "SH-"+shipmentID.String()[:8], orderID, buyerID, carrierID, originID, destID)
	if err != nil {
		t.Fatalf("seed shipment: %v", err)
	}
	var approvedTotal decimal.Decimal
	for _, line := range lines {
		if line.status == domain.AccessorialStatusApproved {
			approvedTotal = approvedTotal.Add(line.amount)
		}
	}
	_, err = env.pool.Exec(ctx, `
		INSERT INTO billing.freight_settlements (
			id, tenant_id, shipment_id, transport_order_id, buyer_company_id, carrier_company_id,
			settlement_number, base_freight_amount, currency_code,
			approved_accessorial_total, total_without_vat, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 1000.00, 'RUB', $8, 1000.00 + $8, 'APPROVED')`,
		settlementID, tenantID, shipmentID, orderID, buyerID, carrierID,
		"FS-"+settlementID.String()[:8], approvedTotal.StringFixed(domain.MoneyScale))
	if err != nil {
		t.Fatalf("seed settlement: %v", err)
	}
	for _, line := range lines {
		accessorialID := line.id
		if accessorialID == uuid.Nil {
			accessorialID = uuid.New()
		}
		_, err = env.pool.Exec(ctx, `
			INSERT INTO billing.settlement_accessorials (
				id, tenant_id, settlement_id, charge_code, amount, currency_code, status,
				submitted_by, submitted_by_company_id
			) VALUES ($1, $2, $3, $4, $5, 'RUB', $6, $7, $8)`,
			accessorialID, tenantID, settlementID, line.chargeCode, line.amount.StringFixed(domain.MoneyScale),
			line.status, buyerID, buyerID)
		if err != nil {
			t.Fatalf("seed accessorial line: %v", err)
		}
	}
}

type settlementAccessorialSeed struct {
	id         uuid.UUID
	chargeCode string
	amount     decimal.Decimal
	status     string
}

func TestFC22DACC001ApprovedAccessorialAggregation(t *testing.T) {
	env := setupAccessorialEnv(t)
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	orderID := uuid.New()
	settlementID := uuid.New()
	period := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	equipment := "TENT"
	lcEnv := &laneCarrierEnv{analyticsEnv: env.analyticsEnv}
	seedTransportOrderWithLocations(t, lcEnv, tenantID, buyerID, orderID, "Moscow", "SPB", "ROAD", &equipment)
	seedCarrierCompany(t, env, tenantID, carrierID, "Carrier Legal LLC", "FastCarrier")
	seedChargeMapping(t, env, tenantID, "DETENTION", "DETENTION", 10, period.Add(-24*time.Hour))
	upsertSummary(t, env.analyticsEnv, tenantID, buyerID, carrierID, orderID, "RUB", "1000.00", "1200.00", period)
	pinSummaryAttribution(t, env, tenantID, orderID, 10, period)

	ctx := context.Background()
	var originID, destID uuid.UUID
	if err := env.pool.QueryRow(ctx, `
		SELECT origin_location_id, destination_location_id
		FROM transport.transport_orders WHERE id = $1`, orderID).Scan(&originID, &destID); err != nil {
		t.Fatalf("load locations: %v", err)
	}
	seedSettlementWithAccessorials(t, env, tenantID, buyerID, carrierID, orderID, settlementID, originID, destID, []settlementAccessorialSeed{
		{chargeCode: "DETENTION", amount: decimal.RequireFromString("150.00"), status: domain.AccessorialStatusApproved},
		{chargeCode: "DETENTION", amount: decimal.RequireFromString("50.00"), status: domain.AccessorialStatusProposed},
	})

	if err := env.analytics.RebuildTenant(ctx, tenantID); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	rows, err := env.accessorialPeriods.List(ctx, tenantID, repository.AnalyticsAccessorialListFilter{
		BuyerCompanyID:     &buyerID,
		CurrencyCode:       "RUB",
		NormalizedCategory: "DETENTION",
		Limit:              10,
	})
	if err != nil {
		t.Fatalf("list accessorial projections: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 accessorial period row, got %d", len(rows))
	}
	if rows[0].LineCount != 1 || rows[0].OrderCount != 1 {
		t.Fatalf("expected approved-only aggregation line=1 order=1, got line=%d order=%d", rows[0].LineCount, rows[0].OrderCount)
	}
	if !decimal.RequireFromString("150.00").Equal(*rows[0].TotalAmount) {
		t.Fatalf("unexpected total: %v", rows[0].TotalAmount)
	}
}

func TestFC22DREC001ReconciliationWithSettlementApprovedTotal(t *testing.T) {
	env := setupAccessorialEnv(t)
	lcEnv := &laneCarrierEnv{analyticsEnv: env.analyticsEnv}
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	orderID := uuid.New()
	settlementID := uuid.New()
	period := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	equipment := "TENT"
	seedTransportOrderWithLocations(t, lcEnv, tenantID, buyerID, orderID, "Moscow", "Kazan", "ROAD", &equipment)
	seedChargeMapping(t, env, tenantID, "FUEL", "FUEL", 11, period.Add(-24*time.Hour))
	upsertSummary(t, env.analyticsEnv, tenantID, buyerID, carrierID, orderID, "RUB", "800.00", "980.00", period)
	pinSummaryAttribution(t, env, tenantID, orderID, 11, period)

	ctx := context.Background()
	var originID, destID uuid.UUID
	_ = env.pool.QueryRow(ctx, `SELECT origin_location_id, destination_location_id FROM transport.transport_orders WHERE id = $1`, orderID).Scan(&originID, &destID)
	seedSettlementWithAccessorials(t, env, tenantID, buyerID, carrierID, orderID, settlementID, originID, destID, []settlementAccessorialSeed{
		{chargeCode: "FUEL", amount: decimal.RequireFromString("80.00"), status: domain.AccessorialStatusApproved},
		{chargeCode: "FUEL", amount: decimal.RequireFromString("20.00"), status: domain.AccessorialStatusApproved},
	})

	if err := env.analytics.RebuildTenant(ctx, tenantID); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	settlements := newDBSettlementAccessorialReader(env.pool)
	batch, err := settlements.BatchGetSettlementsByTransportOrder(ctx, tenantID, []uuid.UUID{orderID})
	if err != nil {
		t.Fatalf("load settlement batch: %v", err)
	}
	item, ok := batch[orderID]
	if !ok {
		t.Fatal("settlement batch item missing")
	}
	rows, err := env.accessorialPeriods.List(ctx, tenantID, repository.AnalyticsAccessorialListFilter{
		NormalizedCategory: "FUEL",
		Limit:              10,
	})
	if err != nil || len(rows) != 1 {
		t.Fatalf("accessorial projection: err=%v len=%d", err, len(rows))
	}
	if !item.ApprovedAccessorialTotal.Equal(*rows[0].TotalAmount) {
		t.Fatalf("projection total %v must match settlement approved total %v", rows[0].TotalAmount, item.ApprovedAccessorialTotal)
	}
}

func TestFC22DENRICH002OrderReference(t *testing.T) {
	env := setupAccessorialEnv(t)
	lcEnv := &laneCarrierEnv{analyticsEnv: env.analyticsEnv}
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	orderID := uuid.New()
	period := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	orderNumber := "TO-ENRICH-002"
	equipment := "TENT"
	seedTransportOrderWithLocations(t, lcEnv, tenantID, buyerID, orderID, "Moscow", "SPB", "ROAD", &equipment)
	_, err := env.pool.Exec(context.Background(), `
		UPDATE transport.transport_orders SET order_number = $2 WHERE id = $1`, orderID, orderNumber)
	if err != nil {
		t.Fatalf("set order_number: %v", err)
	}
	seedCarrierCompany(t, env, tenantID, carrierID, "Carrier Legal", "CarrierShort")
	upsertSummary(t, env.analyticsEnv, tenantID, buyerID, carrierID, orderID, "RUB", "500.00", "550.00", period)

	if err := env.analytics.RebuildTenant(context.Background(), tenantID); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	fact, err := env.orderFacts.GetByTransportOrder(context.Background(), tenantID, orderID, "RUB")
	if err != nil {
		t.Fatalf("get order fact: %v", err)
	}
	if fact.OrderReference == nil || *fact.OrderReference != orderNumber {
		t.Fatalf("expected order_reference %q, got %v", orderNumber, fact.OrderReference)
	}
	if fact.CarrierDisplayName == nil || *fact.CarrierDisplayName != "CarrierShort" {
		t.Fatalf("expected carrier_display_name CarrierShort, got %v", fact.CarrierDisplayName)
	}
}

func TestFC22DEQV001RebuildMatchesIncremental(t *testing.T) {
	env := setupAccessorialEnv(t)
	lcEnv := &laneCarrierEnv{analyticsEnv: env.analyticsEnv}
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	orderID := uuid.New()
	settlementID := uuid.New()
	period := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	equipment := "TENT"
	seedTransportOrderWithLocations(t, lcEnv, tenantID, buyerID, orderID, "Moscow", "SPB", "ROAD", &equipment)
	seedChargeMapping(t, env, tenantID, "WAITING", "WAITING", 12, period.Add(-24*time.Hour))
	upsertSummary(t, env.analyticsEnv, tenantID, buyerID, carrierID, orderID, "RUB", "300.00", "360.00", period)
	pinSummaryAttribution(t, env, tenantID, orderID, 12, period)

	ctx := context.Background()
	var originID, destID uuid.UUID
	_ = env.pool.QueryRow(ctx, `SELECT origin_location_id, destination_location_id FROM transport.transport_orders WHERE id = $1`, orderID).Scan(&originID, &destID)
	seedSettlementWithAccessorials(t, env, tenantID, buyerID, carrierID, orderID, settlementID, originID, destID, []settlementAccessorialSeed{
		{chargeCode: "WAITING", amount: decimal.RequireFromString("60.00"), status: domain.AccessorialStatusApproved},
	})

	if err := env.analytics.RebuildTenant(ctx, tenantID); err != nil {
		t.Fatalf("full rebuild: %v", err)
	}
	fullRows, err := env.accessorialPeriods.List(ctx, tenantID, repository.AnalyticsAccessorialListFilter{Limit: 20})
	if err != nil || len(fullRows) != 1 {
		t.Fatalf("full accessorial rows: err=%v len=%d", err, len(fullRows))
	}

	_, _ = env.pool.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_accessorial_period_projection WHERE tenant_id = $1`, tenantID)
	_, _ = env.pool.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_accessorial_fact WHERE tenant_id = $1`, tenantID)
	_, _ = env.pool.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_order_fact WHERE tenant_id = $1`, tenantID)
	_, _ = env.pool.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_period_projection WHERE tenant_id = $1`, tenantID)
	_, _ = env.pool.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_lane_period_projection WHERE tenant_id = $1`, tenantID)
	_, _ = env.pool.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_carrier_period_projection WHERE tenant_id = $1`, tenantID)
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

	incrRows, err := env.accessorialPeriods.List(ctx, tenantID, repository.AnalyticsAccessorialListFilter{Limit: 20})
	if err != nil || len(incrRows) != 1 {
		t.Fatalf("incr accessorial rows: err=%v len=%d", err, len(incrRows))
	}
	if fullRows[0].LineCount != incrRows[0].LineCount ||
		!fullRows[0].TotalAmount.Equal(*incrRows[0].TotalAmount) {
		t.Fatalf("accessorial equivalence failed full=%+v incr=%+v", fullRows[0], incrRows[0])
	}
}

//go:build integration

package analytics

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	layerOrderFact         = "order_fact"
	layerPeriod            = "period"
	layerLane              = "lane"
	layerCarrier           = "carrier"
	layerAccessorialFact   = "accessorial_fact"
	layerAccessorialPeriod = "accessorial_period"
	layerBenchmark         = "benchmark"
	layerOpportunity       = "opportunity"
)

var derivedAnalyticsTables = []string{
	"freight_cost.cost_analytics_order_fact",
	"freight_cost.cost_analytics_period_projection",
	"freight_cost.cost_analytics_lane_period_projection",
	"freight_cost.cost_analytics_carrier_period_projection",
	"freight_cost.cost_analytics_accessorial_fact",
	"freight_cost.cost_analytics_accessorial_period_projection",
	"freight_cost.cost_analytics_benchmark_projection",
	"freight_cost.cost_analytics_opportunity_projection",
	"freight_cost.analytics_projection_coverage",
}

type AnalyticsBusinessChecksum struct {
	Combined string
	Layers   map[string]string
}

type CanonicalSourceCounts struct {
	CostSummaryProjection int
	CostEntry             int
	TransportOrders       int
	FreightSettlements    int
	SettlementAccessorial int
	Companies             int
}

func ComputeAnalyticsBusinessChecksum(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) (AnalyticsBusinessChecksum, error) {
	layers := map[string]string{
		layerOrderFact:         hashQuery(ctx, pool, orderFactChecksumSQL, tenantID),
		layerPeriod:            hashQuery(ctx, pool, periodChecksumSQL, tenantID),
		layerLane:              hashQuery(ctx, pool, laneChecksumSQL, tenantID),
		layerCarrier:           hashQuery(ctx, pool, carrierChecksumSQL, tenantID),
		layerAccessorialFact:   hashQuery(ctx, pool, accessorialFactChecksumSQL, tenantID),
		layerAccessorialPeriod: hashQuery(ctx, pool, accessorialPeriodChecksumSQL, tenantID),
		layerBenchmark:         hashQuery(ctx, pool, benchmarkChecksumSQL, tenantID),
		layerOpportunity:       hashQuery(ctx, pool, opportunityChecksumSQL, tenantID),
	}
	keys := make([]string, 0, len(layers))
	for key := range layers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	combined := sha256.New()
	for _, key := range keys {
		combined.Write([]byte(key + ":" + layers[key] + "\n"))
	}
	return AnalyticsBusinessChecksum{
		Combined: hex.EncodeToString(combined.Sum(nil)),
		Layers:   layers,
	}, nil
}

func CountDerivedAnalyticsRows(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) (int, error) {
	total := 0
	for _, table := range derivedAnalyticsTables {
		var count int
		query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE tenant_id = $1`, table)
		if err := pool.QueryRow(ctx, query, tenantID).Scan(&count); err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

func DeleteDerivedAnalyticsForTenant(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) error {
	for _, table := range derivedAnalyticsTables {
		query := fmt.Sprintf(`DELETE FROM %s WHERE tenant_id = $1`, table)
		if _, err := pool.Exec(ctx, query, tenantID); err != nil {
			return err
		}
	}
	_, err := pool.Exec(ctx, `DELETE FROM freight_cost.analytics_projection_state WHERE tenant_id = $1`, tenantID)
	return err
}

func SnapshotCanonicalSourceCounts(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) (CanonicalSourceCounts, error) {
	var counts CanonicalSourceCounts
	queries := []struct {
		dest *int
		sql  string
	}{
		{&counts.CostSummaryProjection, `SELECT COUNT(*) FROM freight_cost.cost_summary_projection WHERE tenant_id = $1`},
		{&counts.CostEntry, `SELECT COUNT(*) FROM freight_cost.cost_entry WHERE tenant_id = $1`},
		{&counts.TransportOrders, `SELECT COUNT(*) FROM transport.transport_orders WHERE tenant_id = $1`},
		{&counts.FreightSettlements, `SELECT COUNT(*) FROM billing.freight_settlements WHERE tenant_id = $1`},
		{&counts.SettlementAccessorial, `SELECT COUNT(*) FROM billing.settlement_accessorials WHERE tenant_id = $1`},
		{&counts.Companies, `SELECT COUNT(*) FROM core.companies WHERE tenant_id = $1`},
	}
	for _, q := range queries {
		if err := pool.QueryRow(ctx, q.sql, tenantID).Scan(q.dest); err != nil {
			return CanonicalSourceCounts{}, err
		}
	}
	return counts, nil
}

func CanonicalSourceCountsEqual(before, after CanonicalSourceCounts) bool {
	return before == after
}

func ListOpportunityIDs(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := pool.Query(ctx, `
		SELECT opportunity_id
		FROM freight_cost.cost_analytics_opportunity_projection
		WHERE tenant_id = $1
		ORDER BY opportunity_id`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func hashQuery(ctx context.Context, pool *pgxpool.Pool, query string, tenantID uuid.UUID) string {
	rows, err := pool.Query(ctx, query, tenantID)
	if err != nil {
		return "query_error:" + err.Error()
	}
	defer rows.Close()
	h := sha256.New()
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return "scan_error:" + err.Error()
		}
		parts := make([]string, len(values))
		for i, value := range values {
			parts[i] = fmt.Sprint(value)
		}
		h.Write([]byte(strings.Join(parts, "|") + "\n"))
	}
	if err := rows.Err(); err != nil {
		return "rows_error:" + err.Error()
	}
	return hex.EncodeToString(h.Sum(nil))
}

const orderFactChecksumSQL = `
SELECT transport_order_id, buyer_company_id, carrier_company_id, currency_code,
       COALESCE(planned_amount::text, ''), COALESCE(accrued_amount::text, ''),
       COALESCE(current_actual_amount::text, ''), COALESCE(final_actual_amount::text, ''),
       data_stage, financial_finality, COALESCE(lane_key, ''), COALESCE(order_reference, ''),
       COALESCE(carrier_display_name, ''), COALESCE(lane_label, ''), lane_eligible
FROM freight_cost.cost_analytics_order_fact
WHERE tenant_id = $1
ORDER BY transport_order_id, currency_code`

const periodChecksumSQL = `
SELECT buyer_company_id, period_start, period_grain, currency_code, order_count,
       COALESCE(planned_total::text, ''), COALESCE(accrued_total::text, ''),
       COALESCE(current_actual_total::text, ''), COALESCE(final_actual_total::text, ''),
       reconciliation_open_count, projection_version
FROM freight_cost.cost_analytics_period_projection
WHERE tenant_id = $1
ORDER BY buyer_company_id, period_start, currency_code`

const laneChecksumSQL = `
SELECT buyer_company_id, lane_key, transport_mode, equipment_type, period_start, currency_code,
       order_count, carrier_count,
       COALESCE(planned_total::text, ''), COALESCE(current_actual_total::text, ''),
       projection_version
FROM freight_cost.cost_analytics_lane_period_projection
WHERE tenant_id = $1
ORDER BY buyer_company_id, lane_key, transport_mode, equipment_type, period_start, currency_code`

const carrierChecksumSQL = `
SELECT buyer_company_id, carrier_company_id, period_start, currency_code, order_count,
       COALESCE(planned_total::text, ''), COALESCE(current_actual_total::text, ''),
       projection_version
FROM freight_cost.cost_analytics_carrier_period_projection
WHERE tenant_id = $1
ORDER BY buyer_company_id, carrier_company_id, period_start, currency_code`

const accessorialFactChecksumSQL = `
SELECT accessorial_id, currency_code, transport_order_id, buyer_company_id, settlement_id,
       charge_code, normalized_category, amount::text, status, mapping_version, eligible
FROM freight_cost.cost_analytics_accessorial_fact
WHERE tenant_id = $1
ORDER BY accessorial_id, currency_code`

const accessorialPeriodChecksumSQL = `
SELECT buyer_company_id, normalized_category, period_start, currency_code,
       line_count, order_count, COALESCE(total_amount::text, ''), projection_version
FROM freight_cost.cost_analytics_accessorial_period_projection
WHERE tenant_id = $1
ORDER BY buyer_company_id, normalized_category, period_start, currency_code`

const benchmarkChecksumSQL = `
SELECT buyer_company_id, cohort_type, lane_key, transport_mode, equipment_type,
       period_start, currency_code, sample_count,
       COALESCE(mean_amount::text, ''), COALESCE(median_amount::text, ''),
       COALESCE(p25_amount::text, ''), COALESCE(p75_amount::text, ''), COALESCE(p90_amount::text, ''),
       data_quality, rule_version
FROM freight_cost.cost_analytics_benchmark_projection
WHERE tenant_id = $1
ORDER BY buyer_company_id, lane_key, currency_code`

const opportunityChecksumSQL = `
SELECT opportunity_id, buyer_company_id, opportunity_type, scope, entity_key, currency_code,
       COALESCE(transport_order_id::text, ''), COALESCE(carrier_company_id::text, ''),
       COALESCE(lane_key, ''), period_start,
       observed_amount::text, baseline_amount::text, estimated_delta::text,
       sample_size, data_quality, rule_version
FROM freight_cost.cost_analytics_opportunity_projection
WHERE tenant_id = $1
ORDER BY opportunity_id`

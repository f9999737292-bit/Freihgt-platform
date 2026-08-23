//go:build integration

package analytics

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
)

const (
	perf100kOrderCount       = 100000
	perf100kLaneCount        = 10
	perf100kCarrierCount     = 10
	perf100kAccessorialEvery = 100
)

func perf100kOrderID(index int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("a0000000-0000-4000-8000-%012d", index+1))
}

func setupPerf100kEnv(t *testing.T) *fullProjectionEnv {
	t.Helper()
	return setupFullProjectionEnv(t)
}

func seedPerf100kCanonical(ctx context.Context, t *testing.T, env *fullProjectionEnv) (tenantID, buyerID uuid.UUID) {
	t.Helper()
	tenantID = uuid.MustParse("11111111-1111-4111-8111-111111110001")
	buyerID = uuid.MustParse("22222222-2222-4222-8222-222222220001")
	period := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)

	if _, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, 'perf100k', 'Perf 100k Tenant') ON CONFLICT DO NOTHING`, tenantID); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
		VALUES ($1, $2, 'SHIPPER', 'Perf Buyer', 'ACTIVE') ON CONFLICT DO NOTHING`, buyerID, tenantID); err != nil {
		t.Fatalf("seed buyer: %v", err)
	}

	carrierIDs := make([]uuid.UUID, perf100kCarrierCount)
	for c := 0; c < perf100kCarrierCount; c++ {
		carrierIDs[c] = uuid.MustParse(fmt.Sprintf("c0000000-0000-4000-8000-%012d", c+1))
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, company_type, legal_name, short_name, status)
			VALUES ($1, $2, 'CARRIER', $3, $4, 'ACTIVE') ON CONFLICT DO NOTHING`,
			carrierIDs[c], tenantID, fmt.Sprintf("Perf Carrier %d LLC", c+1), fmt.Sprintf("Carrier%d", c+1)); err != nil {
			t.Fatalf("seed carrier %d: %v", c, err)
		}
	}

	seedChargeMapping(t, &accessorialEnv{analyticsEnv: env.analyticsEnv, mappings: env.mappings},
		tenantID, "DETENTION", "DETENTION", 10, period.Add(-72*time.Hour))

	cities := [][2]string{
		{"Moscow", "SPB"}, {"Moscow", "Kazan"}, {"Moscow", "Novosibirsk"},
		{"SPB", "Moscow"}, {"Kazan", "Moscow"}, {"Novosibirsk", "Moscow"},
		{"Moscow", "Yekaterinburg"}, {"SPB", "Kazan"}, {"Kazan", "Novosibirsk"}, {"Novosibirsk", "SPB"},
	}
	originIDs := make([]uuid.UUID, perf100kLaneCount)
	destIDs := make([]uuid.UUID, perf100kLaneCount)
	for lane := 0; lane < perf100kLaneCount; lane++ {
		originIDs[lane] = uuid.MustParse(fmt.Sprintf("b1000000-0000-4000-8000-%012d", lane*2+1))
		destIDs[lane] = uuid.MustParse(fmt.Sprintf("b1000000-0000-4000-8000-%012d", lane*2+2))
		if _, err := env.pool.Exec(ctx, `
			INSERT INTO transport.locations (
				id, tenant_id, company_id, location_type, name, country_code, city, timezone, status, version
			) VALUES ($1, $2, $3, 'WAREHOUSE', $4, 'RU', $5, 'Europe/Moscow', 'ACTIVE', 1),
			         ($6, $2, $3, 'WAREHOUSE', $7, 'RU', $8, 'Europe/Moscow', 'ACTIVE', 1)
			ON CONFLICT DO NOTHING`,
			originIDs[lane], tenantID, buyerID, "Origin "+cities[lane][0], cities[lane][0],
			destIDs[lane], "Dest "+cities[lane][1], cities[lane][1]); err != nil {
			t.Fatalf("seed locations lane %d: %v", lane, err)
		}
	}

	if _, err := env.pool.Exec(ctx, `
		INSERT INTO transport.cargoes (id, tenant_id, cargo_type, description, gross_weight)
		SELECT ('c0000000-0000-4000-8000-' || lpad(gs::text, 12, '0'))::uuid, $1, 'GENERAL', 'perf cargo', 1000
		FROM generate_series(1, $2) gs
		ON CONFLICT DO NOTHING`, tenantID, perf100kOrderCount); err != nil {
		t.Fatalf("bulk seed cargoes: %v", err)
	}

	if _, err := env.pool.Exec(ctx, `
		INSERT INTO transport.transport_orders (
			id, tenant_id, order_number, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id,
			transport_mode, equipment_type, status, pricing_model_version
		)
		SELECT
			('a0000000-0000-4000-8000-' || lpad(gs::text, 12, '0'))::uuid,
			$1,
			'TO-P100K-' || lpad(gs::text, 6, '0'),
			$2, $2,
			CASE (gs % $3)
				WHEN 0 THEN $4::uuid WHEN 1 THEN $5::uuid WHEN 2 THEN $6::uuid WHEN 3 THEN $7::uuid WHEN 4 THEN $8::uuid
				WHEN 5 THEN $9::uuid WHEN 6 THEN $10::uuid WHEN 7 THEN $11::uuid WHEN 8 THEN $12::uuid ELSE $13::uuid
			END,
			CASE (gs % $3)
				WHEN 0 THEN $14::uuid WHEN 1 THEN $15::uuid WHEN 2 THEN $16::uuid WHEN 3 THEN $17::uuid WHEN 4 THEN $18::uuid
				WHEN 5 THEN $19::uuid WHEN 6 THEN $20::uuid WHEN 7 THEN $21::uuid WHEN 8 THEN $22::uuid ELSE $23::uuid
			END,
			('c0000000-0000-4000-8000-' || lpad(gs::text, 12, '0'))::uuid,
			'ROAD', 'TENT', 'CONVERTED_TO_SHIPMENT', 'SNAPSHOT_V1'
		FROM generate_series(1, $24) gs
		ON CONFLICT (id) DO NOTHING`,
		tenantID, buyerID, perf100kLaneCount,
		originIDs[0], originIDs[1], originIDs[2], originIDs[3], originIDs[4],
		originIDs[5], originIDs[6], originIDs[7], originIDs[8], originIDs[9],
		destIDs[0], destIDs[1], destIDs[2], destIDs[3], destIDs[4],
		destIDs[5], destIDs[6], destIDs[7], destIDs[8], destIDs[9],
		perf100kOrderCount,
	); err != nil {
		t.Fatalf("bulk seed transport orders: %v", err)
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
		args := make([]any, 0, (end-offset)*8)
		argIdx := 1
		for i := offset; i < end; i++ {
			if i > offset {
				builder.WriteString(",")
			}
			orderID := perf100kOrderID(i)
			idx := i + 1
			currency := "RUB"
			if idx%50 == 0 {
				currency = "EUR"
			}
			carrier := carrierIDs[idx%perf100kCarrierCount]
			amount := perf100kAmount(idx)
			builder.WriteString(fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,'CURRENT_ACTUAL_AVAILABLE','CURRENT_ACTUAL','UNLINKED',ARRAY['FREIGHT_SETTLEMENT'],1,$%d)",
				argIdx, argIdx+1, argIdx+2, argIdx+3, argIdx+4, argIdx+5, argIdx+6, argIdx+7))
			args = append(args, tenantID, orderID, buyerID, carrier, currency, amount, amount, period)
			argIdx += 8
		}
		builder.WriteString(` ON CONFLICT (tenant_id, transport_order_id) DO UPDATE SET
			current_actual_amount = EXCLUDED.current_actual_amount,
			updated_at = EXCLUDED.updated_at`)
		if _, err := env.pool.Exec(ctx, builder.String(), args...); err != nil {
			t.Fatalf("bulk seed summaries offset=%d: %v", offset, err)
		}
	}

	lcEnv := &laneCarrierEnv{analyticsEnv: env.analyticsEnv}
	accEnv := &accessorialEnv{analyticsEnv: env.analyticsEnv, mappings: env.mappings}
	for i := 1; i <= perf100kOrderCount; i += perf100kAccessorialEvery {
		orderID := perf100kOrderID(i - 1)
		carrier := carrierIDs[i%perf100kCarrierCount]
		lane := (i - 1) % perf100kLaneCount
		pinSummaryAttribution(t, accEnv, tenantID, orderID, 10, period)
		settlementID := uuid.MustParse(fmt.Sprintf("d0000000-0000-4000-8000-%012d", i/perf100kAccessorialEvery))
		seedSettlementWithAccessorials(t, accEnv, tenantID, buyerID, carrier, orderID, settlementID, originIDs[lane], destIDs[lane], []settlementAccessorialSeed{
			{chargeCode: "DETENTION", amount: decimal.RequireFromString("75.00"), status: domain.AccessorialStatusApproved},
		})
		_ = lcEnv
	}

	return tenantID, buyerID
}

func perf100kAmount(orderIndex int) string {
	laneBucket := (orderIndex - 1) % perf100kLaneCount
	if laneBucket == 0 {
		if orderIndex%500 == 0 {
			return "45000.00"
		}
		return "38000.00"
	}
	base := 1000 + (orderIndex % 5000)
	return decimal.NewFromInt(int64(base)).StringFixed(2)
}

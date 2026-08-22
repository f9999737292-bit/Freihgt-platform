//go:build integration

package ledger

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
)

func TestFC_B_MON_001_DecimalStringRoundTrip(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	amount := decimal.RequireFromString("1234.56")
	ingest(t, env, baseIngestInput(fix, ingestOpts{sourceRevision: 1, amount: &amount}))
	var stored string
	err := env.pool.QueryRow(context.Background(), `
		SELECT amount::text FROM freight_cost.cost_entry
		WHERE tenant_id = $1 AND transport_order_id = $2 LIMIT 1`,
		fix.TenantID, fix.OrderID).Scan(&stored)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if stored != "1234.56" {
		t.Fatalf("stored amount = %q", stored)
	}
}

func TestFC_B_MON_002_NoFloatCorruptionAtBoundary(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	// Value that float64 mishandles near large integers.
	amount := decimal.RequireFromString("999999999999.99")
	ingest(t, env, baseIngestInput(fix, ingestOpts{sourceRevision: 1, amount: &amount}))
	projection := getProjection(t, env, fix)
	if projection.AccruedAmount == nil || !projection.AccruedAmount.Equal(amount) {
		t.Fatalf("projection corrupted: %v", projection.AccruedAmount)
	}
}

func TestFC_B_MON_003_MoneyScaleTwoEnforced(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	amount := decimal.RequireFromString("1000.005")
	ingest(t, env, baseIngestInput(fix, ingestOpts{sourceRevision: 1, amount: &amount}))
	projection := getProjection(t, env, fix)
	if projection.AccruedAmount == nil || !projection.AccruedAmount.Equal(decimal.RequireFromString("1000.01")) {
		t.Fatalf("expected bankers rounding to 2dp, got %v", projection.AccruedAmount)
	}
}

func TestFC_B_MON_004_ZeroSerializedAsDecimalString(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{sourceRevision: 1, amount: decimalAmount("0.00")}))
	var stored *string
	err := env.pool.QueryRow(context.Background(), `
		SELECT amount::text FROM freight_cost.cost_entry
		WHERE tenant_id = $1 AND transport_order_id = $2 LIMIT 1`,
		fix.TenantID, fix.OrderID).Scan(&stored)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if stored == nil || *stored != "0.00" {
		t.Fatalf("zero amount = %v", stored)
	}
}

func TestFC_B_MON_005_TaxBasisStoredPerEntryKind(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindAccrualCostSnapshot, sourceRevision: 1,
		taxBasis: domain.TaxBasisExVAT, amount: decimalAmount("1000.00"),
	}))
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindPaidAmountSnapshot, sourceType: domain.SourceTypePaymentObligation,
		sourceID: uuid.New(), sourceRevision: 1, taxBasis: domain.TaxBasisWithVAT, amount: decimalAmount("1200.00"),
	}))
	var exVAT, withVAT int
	err := env.pool.QueryRow(context.Background(), `
		SELECT
			COUNT(*) FILTER (WHERE tax_basis = 'EX_VAT'),
			COUNT(*) FILTER (WHERE tax_basis = 'WITH_VAT')
		FROM freight_cost.cost_entry WHERE tenant_id = $1 AND transport_order_id = $2`,
		fix.TenantID, fix.OrderID).Scan(&exVAT, &withVAT)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if exVAT != 1 || withVAT != 1 {
		t.Fatalf("tax basis counts ex=%d with=%d", exVAT, withVAT)
	}
}

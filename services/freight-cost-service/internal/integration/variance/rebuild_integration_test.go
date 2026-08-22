//go:build integration

package variance

import (
	"context"
	"testing"
	"time"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/repository"
)

func TestFC_C_RBL_002_RepeatedRebuildZeroNewAttributionRows(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingestPlannedAndActual(t, env, fix)
	before := countAttributions(t, env, fix)
	if _, err := env.rebuild.RebuildTransportOrder(context.Background(), fix.TenantID, fix.OrderID); err != nil {
		t.Fatalf("first rebuild: %v", err)
	}
	mid := countAttributions(t, env, fix)
	if _, err := env.rebuild.RebuildTransportOrder(context.Background(), fix.TenantID, fix.OrderID); err != nil {
		t.Fatalf("second rebuild: %v", err)
	}
	after := countAttributions(t, env, fix)
	if before == 0 && mid == 0 {
		t.Fatal("expected attribution rows after variance ingest")
	}
	if after != mid {
		t.Fatalf("repeated rebuild added attribution rows: mid=%d after=%d", mid, after)
	}
}

func TestFC_C_RBL_005_MappingVersionChangeVarianceUnchanged(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingestPlannedAndActual(t, env, fix)
	projection := getProjection(t, env, fix)
	before := projection.CurrentVarianceAmount

	_, err := env.mappings.UpsertMapping(context.Background(), repository.UpsertChargeCodeMappingInput{
		MappingScope:   domain.MappingScopeTenant,
		TenantID:       &fix.TenantID,
		SourceCode:     "NEW_CODE",
		TargetCategory: "FUEL",
		EffectiveFrom:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("upsert mapping: %v", err)
	}
	if _, err := env.rebuild.RebuildTransportOrder(context.Background(), fix.TenantID, fix.OrderID); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	after := getProjection(t, env, fix)
	if before == nil || after.CurrentVarianceAmount == nil || !before.Equal(*after.CurrentVarianceAmount) {
		t.Fatalf("mapping version change must not alter variance: before=%v after=%v", before, after.CurrentVarianceAmount)
	}
}

func TestFC_C_RBL_006_StandardRebuildUsesPinnedMappingVersion(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingestPlannedAndActual(t, env, fix)
	projection := getProjection(t, env, fix)
	if projection.AttributionMappingVersion == nil {
		t.Fatal("expected pinned attribution_mapping_version after first compute")
	}
	pinned := *projection.AttributionMappingVersion

	if _, err := env.mappings.UpsertMapping(context.Background(), repository.UpsertChargeCodeMappingInput{
		MappingScope:   domain.MappingScopeTenant,
		TenantID:       &fix.TenantID,
		SourceCode:     "PINNED_TEST",
		TargetCategory: "DETENTION",
		EffectiveFrom:  time.Now().UTC(),
	}); err != nil {
		t.Fatalf("upsert mapping: %v", err)
	}
	if _, err := env.rebuild.RebuildTransportOrder(context.Background(), fix.TenantID, fix.OrderID); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	after := getProjection(t, env, fix)
	if after.AttributionMappingVersion == nil || *after.AttributionMappingVersion != pinned {
		t.Fatalf("standard rebuild must reuse pinned version %d, got %v", pinned, after.AttributionMappingVersion)
	}
}

func TestFC_C_RBL_007_HistoricalAttributionPreserved(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingestPlannedAndActual(t, env, fix)
	firstCount := countAttributions(t, env, fix)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind:      domain.EntryKindCurrentActualCostSnapshot,
		sourceRevision: 2,
		amount:         decimalAmount("1200.00"),
	}))
	secondCount := countAttributions(t, env, fix)
	if secondCount <= firstCount {
		t.Fatalf("expected more attribution rows after state change: first=%d second=%d", firstCount, secondCount)
	}
	var total int
	err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM freight_cost.variance_attribution
		WHERE tenant_id = $1 AND transport_order_id = $2`, fix.TenantID, fix.OrderID).Scan(&total)
	if err != nil {
		t.Fatalf("count all: %v", err)
	}
	if total < secondCount {
		t.Fatal("historical attribution rows must be preserved append-only")
	}
}

func TestFC_C_RBL_008_ReclassifyUsesCurrentMappingVersion(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingestPlannedAndActual(t, env, fix)
	currentVersion, err := env.mappings.CurrentPlatformMappingVersion(context.Background())
	if err != nil {
		t.Fatalf("platform version: %v", err)
	}
	inserted, err := env.derived.ReclassifyAttribution(context.Background(), fix.TenantID, fix.OrderID)
	if err != nil {
		t.Fatalf("reclassify: %v", err)
	}
	if inserted == 0 {
		t.Fatal("expected reclassified driver rows")
	}
	var mappingVersion int64
	err = env.pool.QueryRow(context.Background(), `
		SELECT mapping_version FROM freight_cost.variance_attribution
		WHERE tenant_id = $1 AND transport_order_id = $2 AND semantic_class = 'VARIANCE_DRIVER' AND is_current = TRUE
		ORDER BY recorded_at DESC LIMIT 1`, fix.TenantID, fix.OrderID).Scan(&mappingVersion)
	if err != nil {
		t.Fatalf("query mapping version: %v", err)
	}
	if mappingVersion < currentVersion {
		t.Fatalf("reclassify must use current mapping version >= %d, got %d", currentVersion, mappingVersion)
	}
}

//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
)

func TestMigration000077UpDownUp(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000076(t, ctx, pool)
	if _, err := pool.Exec(ctx, mustRead(t, "000077_bno_cargo_equipment_compatibility_v0_1b2.up.sql")); err != nil {
		t.Fatalf("up: %v", err)
	}
	assertBNO077Present(t, ctx, pool, true)
	if _, err := pool.Exec(ctx, mustRead(t, "000077_bno_cargo_equipment_compatibility_v0_1b2.down.sql")); err != nil {
		t.Fatalf("down: %v", err)
	}
	assertBNO077Present(t, ctx, pool, false)
	var capacities bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='network_optimizer' AND table_name='capacities')`).Scan(&capacities); err != nil || !capacities {
		t.Fatalf("000076 baseline missing after down: %v %v", capacities, err)
	}
	if _, err := pool.Exec(ctx, mustRead(t, "000077_bno_cargo_equipment_compatibility_v0_1b2.up.sql")); err != nil {
		t.Fatalf("up again: %v", err)
	}
	assertBNO077Present(t, ctx, pool, true)
}

func TestPostgresRuntimeCatalog(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000076(t, ctx, pool)
	if _, err := pool.Exec(ctx, mustRead(t, "000077_bno_cargo_equipment_compatibility_v0_1b2.up.sql")); err != nil {
		t.Fatal(err)
	}
	store := NewPostgres(pool)
	tenant := uuid.New()

	t.Run("BNO141_CARGO_PARENT_DERIVED_FROM_ACTIVE_CATALOG", func(t *testing.T) {
		insertRule(t, ctx, pool, "FOOD_PARENT", "PLATFORM", "PARENT", "FOOD", "ANY", "", "DENY", "FOOD_PARENT_DENY", "")
		got := evaluatePair(t, ctx, store, tenant, "DAIRY", "DAIRY")
		if !hasCode(got, "FOOD_PARENT_DENY") {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("BNO143_CALLER_CANNOT_FORGE_CATALOG_TAG", func(t *testing.T) {
		insertRule(t, ctx, pool, "FORGED", "PLATFORM", "TAG", "FORGED", "ANY", "", "DENY", "FORGED_TAG", "")
		evalCtx, err := store.Evaluation(ctx, tenant)
		if err != nil {
			t.Fatal(err)
		}
		got := compat.EvaluateGroupage(compat.Equipment{}, []compat.Cargo{
			{ID: "a", CargoTypeCode: str("DAIRY"), Tags: []string{"FORGED"}},
			{ID: "b", CargoTypeCode: str("DAIRY"), Tags: []string{"FORGED"}},
		}, compat.AccessNeed{}, evalCtx)
		if hasCode(got, "FORGED_TAG") {
			t.Fatal("forged tag matched")
		}
	})
	t.Run("BNO144_CATALOG_TAG_DERIVED_SERVER_SIDE", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `UPDATE network_optimizer.cargo_type_catalog SET tags = ARRAY['CHILLED'] WHERE code = 'DAIRY'`); err != nil {
			t.Fatal(err)
		}
		insertRule(t, ctx, pool, "CHILLED", "PLATFORM", "TAG", "CHILLED", "ANY", "", "DENY", "CATALOG_TAG", "")
		got := evaluatePair(t, ctx, store, tenant, "DAIRY", "DAIRY")
		if !hasCode(got, "CATALOG_TAG") {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("BNO145_UNKNOWN_CARGO_TYPE_FAILS_SAFE", func(t *testing.T) {
		got := evaluatePair(t, ctx, store, tenant, "NOT_A_TYPE", "DAIRY")
		if !hasCode(got, "REFERENCE_DATA_UNAVAILABLE") || got.Status == compat.StatusCompatible {
			t.Fatalf("%s %+v", got.Status, got.IndeterminateReasons)
		}
	})
	t.Run("BNO146_EQUIPMENT_PROFILE_DERIVED_FROM_CATALOG", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			UPDATE network_optimizer.equipment_type_catalog
			SET nominal_payload_kg = 22000, body_type = 'REEFER'
			WHERE code = 'SEMITRAILER_REEFER'`); err != nil {
			t.Fatal(err)
		}
		evalCtx, err := store.Evaluation(ctx, tenant)
		if err != nil {
			t.Fatal(err)
		}
		equipment, _, reasons := compat.ResolveProfiles(compat.Equipment{EquipmentTypeCode: str("SEMITRAILER_REEFER")}, nil, evalCtx)
		if len(reasons.IndeterminateReasons) != 0 || equipment.PayloadKg == nil || *equipment.PayloadKg != 22000 || equipment.Provenance["payload_kg"] != compat.ProvenanceReferenceDefault {
			t.Fatalf("%v %v %+v", equipment.PayloadKg, equipment.Provenance, reasons.IndeterminateReasons)
		}
	})
	t.Run("BNO149_POSTGRES_PALLET_EQUIVALENCE_LOADED", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.pallet_equivalences
			(id, version_id, from_code, basis_code, positions_each, source_reference)
			VALUES ($1, '00000000-0000-4000-8000-000000000087', 'EUR2', 'EUR', 2, 'TEST')`, uuid.New()); err != nil {
			t.Fatal(err)
		}
		evalCtx, err := store.Evaluation(ctx, tenant)
		if err != nil {
			t.Fatal(err)
		}
		if evalCtx.PalletCatalogVersion != 1 || len(evalCtx.Equivalences) != 1 || evalCtx.Equivalences[0].PositionsEach != 2 {
			t.Fatalf("%d %#v", evalCtx.PalletCatalogVersion, evalCtx.Equivalences)
		}
		got := compat.EvaluateGroupage(compat.Equipment{PalletPositions: intPtr(2), PalletBasisCode: str("EUR")}, []compat.Cargo{
			{ID: "a", PalletCount: intPtr(1), PalletTypeCode: str("EUR2")},
		}, compat.AccessNeed{}, evalCtx)
		if hasCode(got, "PALLET_EQUIVALENCE_UNKNOWN") || got.Status != compat.StatusCompatible {
			t.Fatalf("%s %+v", got.Status, got)
		}
	})
	t.Run("BNO150_RETIRED_PALLET_EQUIVALENCE_NOT_USED", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.reference_catalog_versions
			(id, catalog_kind, scope, version, status, source_reference, created_by)
			VALUES ($1, 'PALLET_TYPE', 'SYSTEM', 2, 'RETIRED', 'TEST', 'TEST')`, uuid.MustParse("00000000-0000-4000-8000-000000000187")); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.pallet_equivalences
			(id, version_id, from_code, basis_code, positions_each, source_reference)
			VALUES ($1, '00000000-0000-4000-8000-000000000187', 'EUR3', 'EUR', 3, 'RETIRED')`, uuid.New()); err != nil {
			t.Fatal(err)
		}
		evalCtx, err := store.Evaluation(ctx, tenant)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range evalCtx.Equivalences {
			if item.FromCode == "EUR3" {
				t.Fatal("retired equivalence loaded")
			}
		}
		got := compat.EvaluateGroupage(compat.Equipment{PalletPositions: intPtr(9), PalletBasisCode: str("EUR")}, []compat.Cargo{
			{ID: "a", PalletCount: intPtr(1), PalletTypeCode: str("EUR3")},
		}, compat.AccessNeed{}, evalCtx)
		if !hasCode(got, "PALLET_EQUIVALENCE_UNKNOWN") {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("BNO151_UNRELATED_ADR_RULE_DOES_NOT_COUNT_AS_COVERAGE", func(t *testing.T) {
		insertRule(t, ctx, pool, "ADR_3_51", "REGULATORY", "HAZARD_CLASS", "3", "HAZARD_CLASS", "5.1", "DENY", "ADR_INCOMPATIBLE", "ADR-2025-7.5.2")
		evalCtx, err := store.Evaluation(ctx, tenant)
		if err != nil {
			t.Fatal(err)
		}
		got := compat.EvaluateCargoEquipment(compat.Cargo{ID: "a", DangerousGoods: boolPtr(true), HazardClasses: []string{"8"}}, compat.Equipment{ADRCapability: boolPtr(true)}, compat.AccessNeed{}, evalCtx)
		if !hasCode(got, "ADR_COMPATIBILITY_RULE_UNAVAILABLE") || got.Status != compat.StatusIndeterminate {
			t.Fatalf("%s %+v", got.Status, got)
		}
	})
	t.Run("BNO156_RUNTIME_ALIAS_EXACT_ONLY", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.catalog_aliases (id, version_id, alias_code, canonical_code)
			VALUES ($1, '00000000-0000-4000-8000-000000000083', 'MILK', 'DAIRY')`, uuid.New()); err != nil {
			t.Fatal(err)
		}
		insertRule(t, ctx, pool, "DAIRY_TYPE", "PLATFORM", "CARGO_TYPE", "DAIRY", "ANY", "", "DENY", "DAIRY_TYPE", "")
		got := evaluatePair(t, ctx, store, tenant, "MILK", "DAIRY")
		if !hasCode(got, "DAIRY_TYPE") {
			t.Fatalf("%+v", got)
		}
		fuzzy := evaluatePair(t, ctx, store, tenant, "молоко", "DAIRY")
		if !hasCode(fuzzy, "REFERENCE_DATA_UNAVAILABLE") {
			t.Fatal("fuzzy alias resolved")
		}
	})
}

func applyThrough000076(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for _, name := range []string{
		"000001_create_schemas.up.sql",
		"000003_create_transport_tables.up.sql",
		"000075_bno_capacity_marketplace_foundation_v0_1a.up.sql",
		"000076_bno_predictive_capacity_v0_1b.up.sql",
	} {
		if _, err := pool.Exec(ctx, mustRead(t, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func assertBNO077Present(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want bool) {
	t.Helper()
	checks := []struct {
		query string
	}{
		{`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='transport' AND table_name='cargoes' AND column_name='pallet_count')`},
		{`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='transport' AND table_name='vehicles' AND column_name='adr_capability')`},
		{`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='network_optimizer' AND table_name='cargo_type_catalog')`},
		{`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='network_optimizer' AND table_name='compatibility_rule_sets')`},
		{`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='network_optimizer' AND table_name='pallet_equivalences')`},
	}
	for _, check := range checks {
		var exists bool
		if err := pool.QueryRow(ctx, check.query).Scan(&exists); err != nil || exists != want {
			t.Fatalf("%s exists=%v want %v err=%v", check.query, exists, want, err)
		}
	}
	if !want {
		return
	}
	var parent *string
	if err := pool.QueryRow(ctx, `SELECT parent_code FROM network_optimizer.cargo_type_catalog WHERE code='DAIRY'`).Scan(&parent); err != nil || parent == nil || *parent != "FOOD" {
		t.Fatalf("seed DAIRY parent=%v err=%v", parent, err)
	}
}

func insertRule(t *testing.T, ctx context.Context, pool *pgxpool.Pool, code, layer, leftKind, leftValue, rightKind, rightValue, decision, reason, source string) {
	t.Helper()
	var sourceArg any
	if source != "" {
		sourceArg = source
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.compatibility_rules (
			id, rule_set_id, rule_code, rule_kind, layer, left_selector_type, left_selector_value,
			right_selector_type, right_selector_value, decision, reason_code, source_reference
		) VALUES ($1, '00000000-0000-4000-8000-000000000091', $2, 'CARGO_CARGO', $3, $4, $5, $6, $7, $8, $9, $10)`,
		uuid.New(), code, layer, leftKind, leftValue, rightKind, rightValue, decision, reason, sourceArg); err != nil {
		t.Fatal(err)
	}
}

func evaluatePair(t *testing.T, ctx context.Context, store *Postgres, tenant uuid.UUID, left, right string) compat.Result {
	t.Helper()
	evalCtx, err := store.Evaluation(ctx, tenant)
	if err != nil {
		t.Fatal(err)
	}
	return compat.EvaluateGroupage(compat.Equipment{}, []compat.Cargo{
		{ID: "a", CargoTypeCode: str(left)},
		{ID: "b", CargoTypeCode: str(right)},
	}, compat.AccessNeed{}, evalCtx)
}

func str(v string) *string { return &v }
func intPtr(v int) *int    { return &v }
func boolPtr(v bool) *bool { return &v }

func hasCode(got compat.Result, code string) bool {
	for _, reason := range append(append([]compat.Reason{}, got.HardRejects...), got.IndeterminateReasons...) {
		if reason.ReasonCode == code {
			return true
		}
	}
	for _, pair := range got.CargoPairs {
		for _, reason := range pair.Reasons {
			if reason.ReasonCode == code {
				return true
			}
		}
	}
	return false
}

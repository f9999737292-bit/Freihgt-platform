//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/reference"
)

func TestPostgresRuleAndCatalogTrace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000076(t, ctx, pool)
	if _, err := pool.Exec(ctx, mustRead(t, "000077_bno_cargo_equipment_compatibility_v0_1b2.up.sql")); err != nil {
		t.Fatal(err)
	}
	store := NewPostgres(pool)
	tenant := uuid.New()
	actor := uuid.New()
	equipmentVersion := "00000000-0000-4000-8000-000000000086"
	cargoVersion := "00000000-0000-4000-8000-000000000083"

	t.Run("BNO167_INVALID_BODY_TYPE_REFERENCE_REJECTED", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.equipment_type_catalog (id, version_id, code, unit_kind, body_type)
			VALUES ($1, $2, 'BAD_BODY', 'SEMITRAILER', 'REEFER')`, uuid.New(), equipmentVersion)
		if err == nil {
			t.Fatal("REEFER body type accepted")
		}
	})
	t.Run("BNO168_INVALID_LOADING_ACCESS_REFERENCE_REJECTED", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.equipment_type_catalog (id, version_id, code, unit_kind, body_type, loading_access)
			VALUES ($1, $2, 'BAD_ACCESS', 'SEMITRAILER', 'BOX', ARRAY['FRONT'])`, uuid.New(), equipmentVersion)
		if err == nil {
			t.Fatal("FRONT loading access accepted")
		}
	})
	t.Run("BNO169_INVALID_TEMPERATURE_RANGE_REFERENCE_REJECTED", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.equipment_type_catalog
			(id, version_id, code, unit_kind, body_type, temperature_min_c, temperature_max_c)
			VALUES ($1, $2, 'BAD_TEMP', 'SEMITRAILER', 'REFRIGERATOR', 8, 2)`, uuid.New(), equipmentVersion)
		if err == nil {
			t.Fatal("inverted temperature range accepted")
		}
	})

	t.Run("BNO157_REQUIRE_SEPARATION_PERSISTED", func(t *testing.T) {
		set, err := store.CreateRuleSet(ctx, actor, tenant)
		if err != nil {
			t.Fatal(err)
		}
		separation := "PHYSICAL_PARTITION"
		if err := store.AddRule(ctx, actor, tenant, set.ID, reference.Rule{
			RuleCode: "SEP", RuleKind: "CARGO_CARGO", Layer: "TENANT", Decision: "REQUIRE_SEPARATION",
			ReasonCode: "KEEP_APART", Severity: "HARD", RequiredSeparation: &separation,
			LeftSelectorType: "ANY", RightSelectorType: "ANY",
		}); err != nil {
			t.Fatal(err)
		}
		if err := store.ActivateRuleSet(ctx, actor, tenant, set.ID); err != nil {
			t.Fatal(err)
		}
		var stored *string
		if err := pool.QueryRow(ctx, `SELECT required_separation FROM network_optimizer.compatibility_rules WHERE rule_code = 'SEP'`).Scan(&stored); err != nil || stored == nil || *stored != separation {
			t.Fatalf("stored %v err %v", stored, err)
		}
		evalCtx, err := store.Evaluation(ctx, tenant)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, rule := range evalCtx.Rules {
			if rule.RuleCode == "SEP" && rule.RequiredSeparation != nil && *rule.RequiredSeparation == separation {
				found = true
			}
		}
		if !found {
			t.Fatal("evaluation did not load required_separation")
		}
	})
	t.Run("BNO158_REQUIRE_SEPARATION_IN_RESULT", func(t *testing.T) {
		evalCtx, err := store.Evaluation(ctx, tenant)
		if err != nil {
			t.Fatal(err)
		}
		got := compat.EvaluateGroupage(compat.Equipment{}, []compat.Cargo{{ID: "a"}, {ID: "b"}}, compat.AccessNeed{}, evalCtx)
		if got.Status != compat.StatusIndeterminate || len(got.Conditions) == 0 || got.Conditions[0].RequiredSeparation == nil || *got.Conditions[0].RequiredSeparation != "PHYSICAL_PARTITION" {
			t.Fatalf("%s %#v", got.Status, got.Conditions)
		}
	})
	t.Run("BNO159_REQUIRE_SEPARATION_MISSING_REJECTED", func(t *testing.T) {
		set, err := store.CreateRuleSet(ctx, actor, tenant)
		if err != nil {
			t.Fatal(err)
		}
		err = store.AddRule(ctx, actor, tenant, set.ID, reference.Rule{
			RuleCode: "BARE", RuleKind: "CARGO_CARGO", Layer: "TENANT", Decision: "REQUIRE_SEPARATION",
			ReasonCode: "KEEP_APART", LeftSelectorType: "ANY", RightSelectorType: "ANY",
		})
		if err == nil {
			t.Fatal("missing separation accepted")
		}
	})
	t.Run("BNO162_SYSTEM_AND_TENANT_RULESET_V1_DISTINGUISHABLE", func(t *testing.T) {
		evalCtx, err := store.Evaluation(ctx, tenant)
		if err != nil {
			t.Fatal(err)
		}
		got := compat.EvaluateCargoEquipment(compat.Cargo{ID: "a"}, compat.Equipment{}, compat.AccessNeed{}, evalCtx)
		systemSeen, tenantSeen := false, false
		for _, ref := range got.RuleSetsUsed {
			if ref.Version != 1 {
				continue
			}
			if ref.Scope == "SYSTEM" {
				systemSeen = true
			}
			if ref.Scope == "TENANT" && ref.TenantID != nil && *ref.TenantID == tenant.String() && ref.ID != "00000000-0000-4000-8000-000000000091" {
				tenantSeen = true
			}
		}
		if !systemSeen || !tenantSeen {
			t.Fatalf("%#v", got.RuleSetsUsed)
		}
	})
	var tenantCatalog uuid.UUID
	t.Run("BNO163_SYSTEM_AND_TENANT_CATALOG_V1_DISTINGUISHABLE", func(t *testing.T) {
		tenantCatalog = uuid.New()
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.reference_catalog_versions
			(id, catalog_kind, scope, tenant_id, version, status, source_reference, created_by, activated_at)
			VALUES ($1, 'CARGO_TYPE', 'TENANT', $2, 1, 'ACTIVE', 'TEST', 'TEST', now())`, tenantCatalog, tenant); err != nil {
			t.Fatal(err)
		}
		evalCtx, err := store.Evaluation(ctx, tenant)
		if err != nil {
			t.Fatal(err)
		}
		got := compat.EvaluateCargoEquipment(compat.Cargo{ID: "a"}, compat.Equipment{}, compat.AccessNeed{}, evalCtx)
		systemSeen, tenantSeen := false, false
		for _, ref := range got.CatalogVersionsUsed {
			if ref.CatalogKind != "CARGO_TYPE" || ref.Version != 1 {
				continue
			}
			if ref.Scope == "SYSTEM" && ref.ID == cargoVersion {
				systemSeen = true
			}
			if ref.Scope == "TENANT" && ref.ID == tenantCatalog.String() {
				tenantSeen = true
			}
		}
		if !systemSeen || !tenantSeen {
			t.Fatalf("%#v", got.CatalogVersionsUsed)
		}
	})
	t.Run("BNO164_TENANT_CATALOG_CHANGE_CHANGES_FINGERPRINT", func(t *testing.T) {
		before, err := store.Evaluation(ctx, tenant)
		if err != nil {
			t.Fatal(err)
		}
		first := compat.EvaluateCargoEquipment(compat.Cargo{ID: "a"}, compat.Equipment{}, compat.AccessNeed{}, before).Fingerprint
		if _, err := pool.Exec(ctx, `UPDATE network_optimizer.reference_catalog_versions SET version = 2 WHERE id = $1`, tenantCatalog); err != nil {
			t.Fatal(err)
		}
		after, err := store.Evaluation(ctx, tenant)
		if err != nil {
			t.Fatal(err)
		}
		second := compat.EvaluateCargoEquipment(compat.Cargo{ID: "a"}, compat.Equipment{}, compat.AccessNeed{}, after).Fingerprint
		if first == second {
			t.Fatal("fingerprint ignored tenant catalog version")
		}
	})
	t.Run("BNO165_ALIAS_NAMESPACE_ISOLATED", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.catalog_aliases (id, version_id, alias_code, canonical_code)
			VALUES ($1, $2, 'REF', 'FOOD'), ($3, $4, 'REF', 'SEMITRAILER_REEFER')`,
			uuid.New(), cargoVersion, uuid.New(), equipmentVersion); err != nil {
			t.Fatal(err)
		}
		evalCtx, err := store.Evaluation(ctx, tenant)
		if err != nil {
			t.Fatal(err)
		}
		equipment, cargoes, reasons := compat.ResolveProfiles(compat.Equipment{EquipmentTypeCode: str("REF")}, []compat.Cargo{{ID: "a", CargoTypeCode: str("REF")}}, evalCtx)
		if len(reasons.IndeterminateReasons) != 0 || cargoes[0].CargoTypeCode == nil || *cargoes[0].CargoTypeCode != "FOOD" || equipment.EquipmentTypeCode == nil || *equipment.EquipmentTypeCode != "SEMITRAILER_REEFER" {
			t.Fatalf("cargo %v equipment %v %+v", cargoes[0].CargoTypeCode, equipment.EquipmentTypeCode, reasons.IndeterminateReasons)
		}
	})
	t.Run("BNO166_TENANT_ALIAS_OVERRIDES_SAME_KIND_SYSTEM_ALIAS", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.catalog_aliases (id, version_id, alias_code, canonical_code)
			VALUES ($1, $2, 'MILK', 'GENERAL_CARGO')`, uuid.New(), cargoVersion); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.catalog_aliases (id, version_id, alias_code, canonical_code)
			VALUES ($1, $2, 'MILK', 'DAIRY')`, uuid.New(), tenantCatalog); err != nil {
			t.Fatal(err)
		}
		evalCtx, err := store.Evaluation(ctx, tenant)
		if err != nil {
			t.Fatal(err)
		}
		if evalCtx.CargoAliases["MILK"] != "DAIRY" || evalCtx.EquipmentAliases["REF"] != "SEMITRAILER_REEFER" {
			t.Fatalf("cargo %v equipment %v", evalCtx.CargoAliases, evalCtx.EquipmentAliases)
		}
	})
}

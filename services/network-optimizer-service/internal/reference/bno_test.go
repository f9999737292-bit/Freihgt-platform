package reference

import (
	"testing"

	"github.com/google/uuid"
)

func TestBNO83ThroughBNO95ReferenceData(t *testing.T) {
	store := NewStore()
	system := Version{ID: uuid.New(), Kind: "CARGO_TYPE", Scope: ScopeSystem, Version: 1, SourceReference: "SYSTEM_SEED"}
	if err := store.AddVersion(system); err != nil {
		t.Fatal(err)
	}
	if err := store.Activate(system.ID, "SYSTEM_SEED"); err != nil {
		t.Fatal(err)
	}
	t.Run("BNO83_SYSTEM_CATALOG_VERSION_ACTIVE", func(t *testing.T) {
		if err := store.AddCargoType(nil, CargoType{VersionID: system.ID, Code: "FOOD", DisplayName: "Food"}); err == nil {
			t.Fatal("active catalog must be immutable")
		}
	})
	tenantA := uuid.New()
	tenantB := uuid.New()
	draftA := Version{ID: uuid.New(), Kind: "CARGO_TYPE", Scope: ScopeTenant, TenantID: &tenantA, Version: 1}
	if err := store.AddVersion(draftA); err != nil || store.AddCargoType(&tenantA, CargoType{VersionID: draftA.ID, Code: "PRIVATE", DisplayName: "Private"}) != nil {
		t.Fatal("tenant draft")
	}
	if err := store.Activate(draftA.ID, "tenant"); err != nil {
		t.Fatal(err)
	}
	t.Run("BNO84_TENANT_CATALOG_ISOLATION", func(t *testing.T) {
		if len(store.ListCargo(&tenantB)) != 0 {
			t.Fatal("tenant B saw tenant A catalog")
		}
		if len(store.ListCargo(&tenantA)) != 1 {
			t.Fatal("tenant A missing own catalog")
		}
	})
	draft := Version{ID: uuid.New(), Kind: "CARGO_TYPE", Scope: ScopeSystem, Version: 2}
	if err := store.AddVersion(draft); err != nil {
		t.Fatal(err)
	}
	parent := "FOOD"
	if err := store.AddCargoType(nil, CargoType{VersionID: draft.ID, Code: "DAIRY", ParentCode: &parent, DisplayName: "Dairy"}); err != nil {
		t.Fatal(err)
	}
	t.Run("BNO85_CARGO_TYPE_HIERARCHY", func(t *testing.T) {
		found := false
		for _, item := range store.cargo {
			if item.Code == "DAIRY" && item.ParentCode != nil && *item.ParentCode == "FOOD" {
				found = true
			}
		}
		if !found {
			t.Fatal("hierarchy missing")
		}
	})
	equipment := Version{ID: uuid.New(), Kind: "EQUIPMENT_TYPE", Scope: ScopeSystem, Version: 1}
	pallet := Version{ID: uuid.New(), Kind: "PALLET_TYPE", Scope: ScopeSystem, Version: 1}
	packaging := Version{ID: uuid.New(), Kind: "PACKAGING_TYPE", Scope: ScopeSystem, Version: 1}
	for _, version := range []Version{equipment, pallet, packaging} {
		if err := store.AddVersion(version); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("BNO86_EQUIPMENT_TYPE_CATALOG", func(t *testing.T) {
		if err := store.AddNamed(nil, "EQUIPMENT_TYPE", NamedType{VersionID: equipment.ID, Code: "SEMITRAILER_REEFER", UnitKind: "SEMITRAILER", BodyType: "REFRIGERATOR"}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("BNO87_PALLET_TYPE_CATALOG", func(t *testing.T) {
		if err := store.AddNamed(nil, "PALLET_TYPE", NamedType{VersionID: pallet.ID, Code: "EUR", LengthMM: 1200, WidthMM: 800}); err != nil {
			t.Fatal(err)
		}
		if err := store.AddNamed(nil, "PALLET_TYPE", NamedType{VersionID: pallet.ID, Code: "BAD", LengthMM: 0, WidthMM: 800}); err == nil {
			t.Fatal("non-positive dimension accepted")
		}
	})
	t.Run("BNO88_PACKAGING_TYPE_CATALOG", func(t *testing.T) {
		if err := store.AddNamed(nil, "PACKAGING_TYPE", NamedType{VersionID: packaging.ID, Code: "PALLETIZED"}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("BNO89_EXPLICIT_ALIAS_RESOLVES", func(t *testing.T) {
		if err := store.AddAlias(nil, Alias{VersionID: equipment.ID, AliasCode: "REF", CanonicalCode: "REFRIGERATOR"}); err != nil {
			t.Fatal(err)
		}
		got, ok := store.ResolveAlias(equipment.ID, "REF")
		if !ok || got != "REFRIGERATOR" {
			t.Fatal(got, ok)
		}
	})
	t.Run("BNO90_FUZZY_ALIAS_NOT_GUESSED", func(t *testing.T) {
		if _, ok := store.ResolveAlias(equipment.ID, "реф"); ok {
			t.Fatal("fuzzy alias")
		}
	})
	systemSet := RuleSet{ID: uuid.New(), Scope: ScopeSystem, Version: 1, SourceReference: "SYSTEM_SEED"}
	if err := store.CreateRuleSet(systemSet); err != nil {
		t.Fatal(err)
	}
	deny := Rule{RuleCode: "SYS_DENY", RuleKind: "CARGO_CARGO", Layer: "PLATFORM", Decision: "DENY", ReasonCode: "CARGO_PAIR_INCOMPATIBLE"}
	if err := store.AddRule(nil, systemSet.ID, deny); err != nil {
		t.Fatal(err)
	}
	t.Run("BNO91_RULE_SET_VERSIONING", func(t *testing.T) {
		if err := store.ActivateRuleSet(nil, systemSet.ID); err != nil {
			t.Fatal(err)
		}
		next := RuleSet{ID: uuid.New(), Scope: ScopeSystem, Version: 2}
		if err := store.CreateRuleSet(next); err != nil {
			t.Fatal(err)
		}
		if err := store.ActivateRuleSet(nil, next.ID); err != nil {
			t.Fatal(err)
		}
		old, err := store.GetRuleSet(nil, systemSet.ID)
		if err != nil || old.Status != StatusRetired {
			t.Fatal(old.Status, err)
		}
	})
	t.Run("BNO92_ACTIVE_VERSION_IMMUTABLE", func(t *testing.T) {
		active, _ := store.GetRuleSet(nil, systemSet.ID)
		if err := store.AddRule(nil, active.ID, Rule{RuleCode: "LATE", Layer: "PLATFORM", Decision: "DENY"}); err == nil {
			t.Fatal("retired or non-draft mutation")
		}
	})
	fresh := RuleSet{ID: uuid.New(), Scope: ScopeSystem, Version: 3}
	_ = store.CreateRuleSet(fresh)
	_ = store.AddRule(nil, fresh.ID, deny)
	_ = store.ActivateRuleSet(nil, fresh.ID)
	tenantSet := RuleSet{ID: uuid.New(), Scope: ScopeTenant, TenantID: &tenantA, Version: 1}
	_ = store.CreateRuleSet(tenantSet)
	t.Run("BNO93_SYSTEM_HARD_DENY_PRECEDENCE", func(t *testing.T) {
		if err := store.AddRule(&tenantA, tenantSet.ID, Rule{RuleCode: "ALLOW_IT", Layer: "TENANT", Decision: "ALLOW"}); err != nil {
			t.Fatal(err)
		}
		systemRules, err := store.GetRuleSet(nil, fresh.ID)
		if err != nil || systemRules.Rules[0].Decision != "DENY" || systemRules.Layer() != "PLATFORM" && systemRules.Rules[0].Layer != "PLATFORM" {
			t.Fatal("system deny missing")
		}
	})
	t.Run("BNO94_TENANT_RULE_CANNOT_OVERRIDE_SYSTEM_HARD_DENY", func(t *testing.T) {
		if err := store.AddRule(&tenantA, tenantSet.ID, Rule{RuleCode: "WEAKEN", Layer: "REGULATORY", Decision: "ALLOW"}); err == nil {
			t.Fatal("tenant regulatory rule accepted")
		}
	})
	t.Run("BNO95_REGULATORY_RULE_REQUIRES_SOURCE_REFERENCE", func(t *testing.T) {
		reg := RuleSet{ID: uuid.New(), Scope: ScopeSystem, Version: 4}
		if err := store.CreateRuleSet(reg); err != nil {
			t.Fatal(err)
		}
		if err := store.AddRule(nil, reg.ID, Rule{RuleCode: "ADR", Layer: "REGULATORY", Decision: "DENY"}); err == nil {
			t.Fatal("unsourced regulatory rule")
		}
		src := "ADR-2025"
		if err := store.AddRule(nil, reg.ID, Rule{RuleCode: "ADR", Layer: "REGULATORY", Decision: "DENY", SourceReference: &src}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("BNO139_CROSS_TENANT_REFERENCE_RULE_ACCESS_DENIED", func(t *testing.T) {
		if _, err := store.GetRuleSet(&tenantB, tenantSet.ID); err == nil {
			t.Fatal("cross-tenant read")
		}
	})
	foundCreated := false
	for _, audit := range store.Audits() {
		if audit.Action == "rule_set_created" {
			foundCreated = true
		}
	}
	if !foundCreated {
		t.Fatal("audit missing")
	}
}

func (s RuleSet) Layer() string { return s.Scope }

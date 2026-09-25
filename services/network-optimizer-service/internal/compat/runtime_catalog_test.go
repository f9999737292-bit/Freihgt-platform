package compat

import "testing"

func dairyCatalog() Context {
	food := "FOOD"
	root := "ROOT"
	parent := "PARENT"
	return Context{
		CargoCatalogVersion: 1,
		CargoClasses: []CargoClass{
			{Code: "ROOT", Scope: "SYSTEM"},
			{Code: "FOOD", Parent: &root, Scope: "SYSTEM"},
			{Code: "DAIRY", Parent: &food, Tags: []string{"CHILLED"}, Scope: "SYSTEM"},
			{Code: "CHILD", Parent: &parent, Scope: "SYSTEM"},
			{Code: "PARENT", Parent: &root, Scope: "SYSTEM"},
		},
	}
}

func TestBNO141CargoParentDerivedFromActiveCatalog(t *testing.T) {
	ctx := dairyCatalog()
	ctx.Rules = []Rule{parentFoodDeny()}
	got := EvaluateGroupage(Equipment{}, []Cargo{{ID: "a", CargoTypeCode: s("DAIRY")}, {ID: "b", CargoTypeCode: s("DAIRY")}}, AccessNeed{}, ctx)
	requireCode(t, got, "FOOD_PARENT_DENY")
}

func TestBNO142CallerCannotHideCatalogParent(t *testing.T) {
	ctx := dairyCatalog()
	ctx.Rules = []Rule{parentFoodDeny()}
	got := EvaluateGroupage(Equipment{}, []Cargo{
		{ID: "a", CargoTypeCode: s("DAIRY"), ParentCodes: []string{}},
		{ID: "b", CargoTypeCode: s("DAIRY"), ParentCodes: []string{}},
	}, AccessNeed{}, ctx)
	requireCode(t, got, "FOOD_PARENT_DENY")
}

func TestBNO143CallerCannotForgeCatalogTag(t *testing.T) {
	ctx := dairyCatalog()
	ctx.Rules = []Rule{{
		RuleCode: "FORGED", RuleKind: KindCargoCargo, Layer: LayerPlatform,
		LeftSelectorType: "TAG", LeftSelectorValue: "FORGED", RightSelectorType: "ANY",
		Decision: DecisionDeny, ReasonCode: "FORGED_TAG",
	}}
	got := EvaluateGroupage(Equipment{}, []Cargo{
		{ID: "a", CargoTypeCode: s("DAIRY"), Tags: []string{"FORGED"}},
		{ID: "b", CargoTypeCode: s("DAIRY"), Tags: []string{"FORGED"}},
	}, AccessNeed{}, ctx)
	if hasReason(got.HardRejects, "FORGED_TAG") {
		t.Fatal("caller tag was authoritative")
	}
}

func TestBNO144CatalogTagDerivedServerSide(t *testing.T) {
	ctx := dairyCatalog()
	ctx.Rules = []Rule{{
		RuleCode: "CHILLED", RuleKind: KindCargoCargo, Layer: LayerPlatform,
		LeftSelectorType: "TAG", LeftSelectorValue: "CHILLED", RightSelectorType: "ANY",
		Decision: DecisionDeny, ReasonCode: "CATALOG_TAG",
	}}
	got := EvaluateGroupage(Equipment{}, []Cargo{{ID: "a", CargoTypeCode: s("DAIRY")}, {ID: "b", CargoTypeCode: s("DAIRY")}}, AccessNeed{}, ctx)
	requireCode(t, got, "CATALOG_TAG")
}

func TestBNO145UnknownCargoTypeFailsSafe(t *testing.T) {
	got := EvaluateCargoEquipment(Cargo{ID: "a", CargoTypeCode: s("NOT_A_TYPE")}, Equipment{}, AccessNeed{}, dairyCatalog())
	requireIndeterminate(t, got, "REFERENCE_DATA_UNAVAILABLE")
}

func TestBNO146EquipmentProfileDerivedFromCatalog(t *testing.T) {
	ctx := Context{EquipmentCatalogVersion: 1, EquipmentClasses: []EquipmentClass{{
		Code: "SEMITRAILER_REEFER", Scope: "SYSTEM", UnitKind: s("SEMITRAILER"), BodyType: s("REEFER"), PayloadKg: f64(22000),
	}}}
	equipment, _, reasons := ResolveProfiles(Equipment{EquipmentTypeCode: s("SEMITRAILER_REEFER")}, nil, ctx)
	if len(reasons.IndeterminateReasons) != 0 || equipment.PayloadKg == nil || *equipment.PayloadKg != 22000 || equipment.BodyType == nil || *equipment.BodyType != "REEFER" {
		t.Fatalf("%#v %#v", equipment, reasons)
	}
}

func TestBNO147AssetFactOverridesReferenceDefault(t *testing.T) {
	ctx := Context{EquipmentCatalogVersion: 1, EquipmentClasses: []EquipmentClass{{Code: "SEMITRAILER_REEFER", Scope: "SYSTEM", PayloadKg: f64(22000)}}}
	equipment, _, _ := ResolveProfiles(Equipment{EquipmentTypeCode: s("SEMITRAILER_REEFER"), PayloadKg: f64(20500)}, nil, ctx)
	if equipment.PayloadKg == nil || *equipment.PayloadKg != 20500 || equipment.Provenance["payload_kg"] != ProvenanceAssetConfirmed {
		t.Fatalf("%v %v", equipment.PayloadKg, equipment.Provenance)
	}
}

func TestBNO148ReferenceDefaultProvenance(t *testing.T) {
	ctx := Context{EquipmentCatalogVersion: 1, EquipmentClasses: []EquipmentClass{{Code: "SEMITRAILER_REEFER", Scope: "SYSTEM", PayloadKg: f64(22000)}}}
	equipment, _, _ := ResolveProfiles(Equipment{EquipmentTypeCode: s("SEMITRAILER_REEFER")}, nil, ctx)
	if equipment.PayloadKg == nil || *equipment.PayloadKg != 22000 || equipment.Provenance["payload_kg"] != ProvenanceReferenceDefault {
		t.Fatalf("%v %v", equipment.PayloadKg, equipment.Provenance)
	}
}

func TestBNO151UnrelatedADRRuleDoesNotCountAsCoverage(t *testing.T) {
	got := EvaluateCargoEquipment(
		Cargo{ID: "a", DangerousGoods: b(true), HazardClasses: []string{"8"}},
		Equipment{ADRCapability: b(true)},
		AccessNeed{},
		Context{Rules: []Rule{adrPair("3", "5.1")}},
	)
	requireIndeterminate(t, got, "ADR_COMPATIBILITY_RULE_UNAVAILABLE")
}

func TestBNO152ADRCapabilityUnknownIndeterminate(t *testing.T) {
	got := EvaluateCargoEquipment(
		Cargo{ID: "a", DangerousGoods: b(true), HazardClasses: []string{"8"}},
		Equipment{},
		AccessNeed{},
		Context{},
	)
	requireIndeterminate(t, got, "ADR_CAPABILITY_UNKNOWN")
}

func TestBNO153ApplicableSourcedADRRuleUsed(t *testing.T) {
	got := EvaluateGroupage(
		Equipment{ADRCapability: b(true)},
		[]Cargo{
			{ID: "a", DangerousGoods: b(true), HazardClasses: []string{"8"}},
			{ID: "b", DangerousGoods: b(true), HazardClasses: []string{"8"}},
		},
		AccessNeed{},
		Context{Rules: []Rule{adrPair("3", "5.1"), adrPair("8", "8")}},
	)
	requireCode(t, got, "ADR_INCOMPATIBLE")
	if hasReason(got.IndeterminateReasons, "ADR_COMPATIBILITY_RULE_UNAVAILABLE") {
		t.Fatal("applicable sourced rule was treated as missing")
	}
}

func TestBNO155TenantAllowCannotOverridePlatformDenyAtEvaluator(t *testing.T) {
	ctx := dairyCatalog()
	ctx.Rules = []Rule{
		parentFoodDeny(),
		{RuleCode: "TENANT_ALLOW", RuleKind: KindCargoCargo, Layer: LayerTenant, LeftSelectorType: "ANY", RightSelectorType: "ANY", Decision: DecisionAllow, ReasonCode: "TENANT_ALLOW"},
	}
	got := EvaluateGroupage(Equipment{}, []Cargo{{ID: "a", CargoTypeCode: s("DAIRY")}, {ID: "b", CargoTypeCode: s("DAIRY")}}, AccessNeed{}, ctx)
	requireCode(t, got, "FOOD_PARENT_DENY")
	if got.Status != StatusIncompatible {
		t.Fatal(got.Status)
	}
}

func TestCatalogHierarchyFailsClosed(t *testing.T) {
	self := "LOOP"
	ctx := Context{CargoCatalogVersion: 1, CargoClasses: []CargoClass{{Code: "LOOP", Parent: &self, Scope: "SYSTEM"}}}
	got := EvaluateCargoEquipment(Cargo{ID: "a", CargoTypeCode: s("LOOP")}, Equipment{}, AccessNeed{}, ctx)
	requireIndeterminate(t, got, "REFERENCE_DATA_UNAVAILABLE")
	missing := "MISSING"
	ctx = Context{CargoCatalogVersion: 1, CargoClasses: []CargoClass{{Code: "ORPHAN", Parent: &missing, Scope: "SYSTEM"}}}
	got = EvaluateCargoEquipment(Cargo{ID: "a", CargoTypeCode: s("ORPHAN")}, Equipment{}, AccessNeed{}, ctx)
	requireIndeterminate(t, got, "REFERENCE_DATA_UNAVAILABLE")
	ctx = dairyCatalog()
	ctx.Rules = []Rule{{
		RuleCode: "ROOT", RuleKind: KindCargoCargo, Layer: LayerPlatform,
		LeftSelectorType: "PARENT", LeftSelectorValue: "ROOT", RightSelectorType: "ANY",
		Decision: DecisionDeny, ReasonCode: "ROOT_PARENT",
	}}
	got = EvaluateGroupage(Equipment{}, []Cargo{{ID: "a", CargoTypeCode: s("CHILD")}, {ID: "b", CargoTypeCode: s("CHILD")}}, AccessNeed{}, ctx)
	requireCode(t, got, "ROOT_PARENT")
}

func TestBNO158RequireSeparationInResult(t *testing.T) {
	separation := "PHYSICAL_PARTITION"
	setID := "00000000-0000-4000-8000-000000000099"
	got := EvaluateGroupage(Equipment{}, []Cargo{{ID: "a"}, {ID: "b"}}, AccessNeed{}, Context{Rules: []Rule{{
		RuleCode: "SEP", RuleKind: KindCargoCargo, Layer: LayerPlatform, LeftSelectorType: "ANY", RightSelectorType: "ANY",
		Decision: DecisionRequireSeparation, ReasonCode: "KEEP_APART", Severity: SeverityHard,
		RequiredSeparation: &separation, RuleSetID: setID, RuleSetScope: "SYSTEM", RuleSetVersion: 1,
	}}})
	if got.Status != StatusIndeterminate {
		t.Fatal(got.Status)
	}
	if len(got.Conditions) != 1 || got.Conditions[0].RequiredSeparation == nil || *got.Conditions[0].RequiredSeparation != separation {
		t.Fatalf("%#v", got.Conditions)
	}
	if got.Conditions[0].RuleCode == nil || *got.Conditions[0].RuleCode != "SEP" || got.Conditions[0].RuleSetID == nil || *got.Conditions[0].RuleSetID != setID || got.Conditions[0].RuleSetScope == nil || *got.Conditions[0].RuleSetScope != "SYSTEM" {
		t.Fatalf("%#v", got.Conditions[0])
	}
}

func TestBNO160HardDenyIncompatible(t *testing.T) {
	got := EvaluateGroupage(Equipment{}, []Cargo{{ID: "a"}, {ID: "b"}}, AccessNeed{}, Context{Rules: []Rule{{
		RuleCode: "HARD", RuleKind: KindCargoCargo, Layer: LayerPlatform, LeftSelectorType: "ANY", RightSelectorType: "ANY",
		Decision: DecisionDeny, ReasonCode: "HARD_DENY", Severity: SeverityHard,
	}}})
	requireCode(t, got, "HARD_DENY")
}

func TestBNO161SoftDenyWarning(t *testing.T) {
	got := EvaluateGroupage(Equipment{}, []Cargo{{ID: "a"}, {ID: "b"}}, AccessNeed{}, Context{Rules: []Rule{{
		RuleCode: "SOFT", RuleKind: KindCargoCargo, Layer: LayerPlatform, LeftSelectorType: "ANY", RightSelectorType: "ANY",
		Decision: DecisionDeny, ReasonCode: "SOFT_DENY", Severity: SeveritySoft,
	}}})
	if got.Status != StatusCompatible || len(got.Warnings) != 1 || got.Warnings[0].ReasonCode != "SOFT_DENY" || len(got.HardRejects) != 0 {
		t.Fatalf("%s %#v %#v", got.Status, got.Warnings, got.HardRejects)
	}
}

func TestBNO164TenantCatalogChangeChangesFingerprint(t *testing.T) {
	base := Context{CatalogRefs: []CatalogVersionRef{{ID: "system", CatalogKind: "CARGO_TYPE", Scope: "SYSTEM", Version: 1}, {ID: "tenant-a", CatalogKind: "CARGO_TYPE", Scope: "TENANT", Version: 1}}}
	changed := base
	changed.CatalogRefs = []CatalogVersionRef{{ID: "system", CatalogKind: "CARGO_TYPE", Scope: "SYSTEM", Version: 1}, {ID: "tenant-a", CatalogKind: "CARGO_TYPE", Scope: "TENANT", Version: 2}}
	otherSet := Context{RuleSets: []RuleSetRef{{ID: "system-set", Scope: "SYSTEM", Version: 1}}}
	sameNumber := Context{RuleSets: []RuleSetRef{{ID: "tenant-set", Scope: "TENANT", Version: 1}}}
	cargoes := []Cargo{{ID: "a"}}
	if fingerprint(Equipment{}, cargoes, base) == fingerprint(Equipment{}, cargoes, changed) {
		t.Fatal("tenant catalog version did not change fingerprint")
	}
	if fingerprint(Equipment{}, cargoes, otherSet) == fingerprint(Equipment{}, cargoes, sameNumber) {
		t.Fatal("same version number collapsed different rule sets")
	}
}

func TestAliasNamespacesStaySeparate(t *testing.T) {
	ctx := Context{
		CargoCatalogVersion: 1, EquipmentCatalogVersion: 1,
		CargoClasses:     []CargoClass{{Code: "FOOD", Scope: "SYSTEM"}},
		EquipmentClasses: []EquipmentClass{{Code: "SEMITRAILER_REEFER", Scope: "SYSTEM", BodyType: s("REFRIGERATOR")}},
		CargoAliases:     map[string]string{"REF": "FOOD"},
		EquipmentAliases: map[string]string{"REF": "SEMITRAILER_REEFER"},
	}
	equipment, cargoes, reasons := ResolveProfiles(Equipment{EquipmentTypeCode: s("REF")}, []Cargo{{ID: "a", CargoTypeCode: s("REF")}}, ctx)
	if len(reasons.IndeterminateReasons) != 0 || cargoes[0].CargoTypeCode == nil || *cargoes[0].CargoTypeCode != "FOOD" || equipment.EquipmentTypeCode == nil || *equipment.EquipmentTypeCode != "SEMITRAILER_REEFER" {
		t.Fatalf("%#v %#v %+v", cargoes, equipment.EquipmentTypeCode, reasons.IndeterminateReasons)
	}
}

func parentFoodDeny() Rule {
	return Rule{
		RuleCode: "FOOD_PARENT", RuleKind: KindCargoCargo, Layer: LayerPlatform,
		LeftSelectorType: "PARENT", LeftSelectorValue: "FOOD", RightSelectorType: "ANY",
		Decision: DecisionDeny, ReasonCode: "FOOD_PARENT_DENY",
	}
}

func adrPair(left, right string) Rule {
	return Rule{
		RuleCode: "ADR_" + left + "_" + right, RuleKind: KindCargoCargo, Layer: LayerRegulatory,
		LeftSelectorType: "HAZARD_CLASS", LeftSelectorValue: left,
		RightSelectorType: "HAZARD_CLASS", RightSelectorValue: right,
		Decision: DecisionDeny, ReasonCode: "ADR_INCOMPATIBLE", SourceReference: s("ADR-2025-7.5.2"),
	}
}

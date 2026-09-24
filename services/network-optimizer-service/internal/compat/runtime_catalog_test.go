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

package compat

import "testing"

func TestBNO96ThroughBNO140Compatibility(t *testing.T) {
	body := s("REFRIGERATOR")
	t.Run("BNO96_BODY_COMPATIBLE", func(t *testing.T) {
		got := EvaluateCargoEquipment(Cargo{ID: "a"}, Equipment{BodyType: body}, AccessNeed{RequiredBodyTypes: []string{"REFRIGERATOR"}}, Context{})
		if got.Status != StatusCompatible {
			t.Fatalf("status %s", got.Status)
		}
	})
	t.Run("BNO97_BODY_INCOMPATIBLE", func(t *testing.T) {
		got := EvaluateCargoEquipment(Cargo{ID: "a"}, Equipment{BodyType: s("TENT")}, AccessNeed{RequiredBodyTypes: []string{"REFRIGERATOR"}}, Context{})
		requireCode(t, got, "BODY_TYPE_INCOMPATIBLE")
	})
	t.Run("BNO98_LOADING_ACCESS_REQUIRED_ALL", func(t *testing.T) {
		got := EvaluateCargoEquipment(Cargo{ID: "a"}, Equipment{LoadingAccess: []string{"REAR"}}, AccessNeed{RequiredLoadingAccess: []string{"REAR", "SIDE"}}, Context{})
		requireCode(t, got, "LOADING_ACCESS_UNSUPPORTED")
	})
	t.Run("BNO99_LOADING_ACCESS_ALLOWED_ANY", func(t *testing.T) {
		got := EvaluateCargoEquipment(Cargo{ID: "a"}, Equipment{LoadingAccess: []string{"SIDE"}}, AccessNeed{AllowedLoadingAccess: []string{"SIDE", "REAR"}}, Context{})
		if got.Status != StatusCompatible {
			t.Fatalf("status %s reasons %#v", got.Status, got.HardRejects)
		}
	})
	t.Run("BNO100_UNLOADING_ACCESS", func(t *testing.T) {
		got := EvaluateCargoEquipment(Cargo{ID: "a"}, Equipment{UnloadingAccess: []string{"TOP"}}, AccessNeed{RequiredUnloadingAccess: []string{"REAR"}}, Context{})
		requireCode(t, got, "UNLOADING_ACCESS_UNSUPPORTED")
	})
	t.Run("BNO101_TEMPERATURE_SINGLE_CARGO", func(t *testing.T) {
		pass := EvaluateCargoEquipment(tempCargo("a", 2, 8), Equipment{TemperatureControlMode: s("ACTIVE"), TemperatureMinC: f64(-25), TemperatureMaxC: f64(20)}, AccessNeed{}, Context{})
		if pass.Status != StatusCompatible {
			t.Fatalf("pass %s %#v", pass.Status, pass.HardRejects)
		}
		fail := EvaluateCargoEquipment(tempCargo("a", 2, 8), Equipment{TemperatureControlMode: s("ACTIVE"), TemperatureMinC: f64(-25), TemperatureMaxC: f64(0)}, AccessNeed{}, Context{})
		requireCode(t, fail, "TEMPERATURE_RANGE_UNSUPPORTED")
	})
	t.Run("BNO102_PAYLOAD_EXCEEDED", func(t *testing.T) {
		got := EvaluateCargoEquipment(Cargo{ID: "a", WeightKg: f64(23000)}, Equipment{PayloadKg: f64(22000)}, AccessNeed{}, Context{})
		requireCode(t, got, "PAYLOAD_EXCEEDED")
	})
	t.Run("BNO103_PAYLOAD_UNKNOWN", func(t *testing.T) {
		got := EvaluateCargoEquipment(Cargo{ID: "a", WeightKg: f64(1000)}, Equipment{}, AccessNeed{}, Context{})
		requireIndeterminate(t, got, "PAYLOAD_UNKNOWN")
	})
	t.Run("BNO104_VOLUME_EXCEEDED", func(t *testing.T) {
		got := EvaluateCargoEquipment(Cargo{ID: "a", VolumeM3: f64(90)}, Equipment{VolumeM3: f64(82)}, AccessNeed{}, Context{})
		requireCode(t, got, "VOLUME_EXCEEDED")
	})
	t.Run("BNO105_VOLUME_UNKNOWN", func(t *testing.T) {
		got := EvaluateCargoEquipment(Cargo{ID: "a", VolumeM3: f64(10)}, Equipment{}, AccessNeed{}, Context{})
		requireIndeterminate(t, got, "VOLUME_UNKNOWN")
	})
	t.Run("BNO106_PALLET_POSITIONS_EXCEEDED", func(t *testing.T) {
		got := EvaluateCargoEquipment(pallet("a", 36, "EUR"), Equipment{PalletPositions: i(33), PalletBasisCode: s("EUR")}, AccessNeed{}, Context{})
		requireCode(t, got, "PALLET_POSITIONS_EXCEEDED")
	})
	t.Run("BNO107_PALLET_COUNT_UNKNOWN", func(t *testing.T) {
		got := EvaluateCargoEquipment(Cargo{ID: "a", PalletTypeCode: s("EUR"), PackagingTypeCode: s("PALLETIZED")}, Equipment{PalletPositions: i(33)}, AccessNeed{}, Context{})
		requireIndeterminate(t, got, "PALLET_COUNT_UNKNOWN")
	})
	t.Run("BNO108_PALLET_TYPE_UNKNOWN", func(t *testing.T) {
		got := EvaluateCargoEquipment(Cargo{ID: "a", PalletCount: i(10)}, Equipment{PalletPositions: i(33)}, AccessNeed{}, Context{})
		requireIndeterminate(t, got, "PALLET_TYPE_UNKNOWN")
	})
	t.Run("BNO109_MIXED_PALLET_TYPES_REQUIRE_EQUIVALENCE", func(t *testing.T) {
		got := EvaluateGroupage(Equipment{PalletPositions: i(33)}, []Cargo{pallet("a", 10, "EUR"), pallet("b", 5, "INDUSTRIAL")}, AccessNeed{}, Context{})
		requireIndeterminate(t, got, "PALLET_EQUIVALENCE_UNKNOWN")
	})
	t.Run("BNO110_LINEAR_METERS_EXCEEDED", func(t *testing.T) {
		got := EvaluateCargoEquipment(Cargo{ID: "a", LinearMeters: f64(14)}, Equipment{UsableLinearMeters: f64(13.6)}, AccessNeed{}, Context{})
		requireCode(t, got, "LINEAR_METERS_EXCEEDED")
	})
	t.Run("BNO111_LINEAR_METERS_UNKNOWN", func(t *testing.T) {
		got := EvaluateCargoEquipment(Cargo{ID: "a", LinearMeters: f64(4)}, Equipment{}, AccessNeed{}, Context{})
		requireIndeterminate(t, got, "LINEAR_METERS_UNKNOWN")
	})
	t.Run("BNO112_HEIGHT_EXCEEDED", func(t *testing.T) {
		got := EvaluateCargoEquipment(Cargo{ID: "a", MaxLoadedHeightMM: i(2800)}, Equipment{InternalHeightMM: i(2700)}, AccessNeed{}, Context{})
		requireCode(t, got, "HEIGHT_EXCEEDED")
	})
	t.Run("BNO113_STACKABLE_DOES_NOT_INVENT_EXTRA_SPACE", func(t *testing.T) {
		got := EvaluateGroupage(Equipment{PalletPositions: i(15), PalletBasisCode: s("EUR")}, []Cargo{
			stackablePallet("a", 10), stackablePallet("b", 10),
		}, AccessNeed{}, Context{})
		requireCode(t, got, "PALLET_POSITIONS_EXCEEDED")
	})
	t.Run("BNO114_FOOD_GRADE_REQUIRED", func(t *testing.T) {
		got := EvaluateCargoEquipment(Cargo{ID: "a", FoodGradeRequired: b(true)}, Equipment{FoodGradeCapability: b(false)}, AccessNeed{}, Context{})
		requireCode(t, got, "FOOD_GRADE_REQUIRED")
	})
	t.Run("BNO115_FOOD_GRADE_UNKNOWN", func(t *testing.T) {
		got := EvaluateCargoEquipment(Cargo{ID: "a", FoodGradeRequired: b(true)}, Equipment{}, AccessNeed{}, Context{})
		requireIndeterminate(t, got, "FOOD_GRADE_CAPABILITY_UNKNOWN")
	})
	t.Run("BNO116_PLUS5_MINUS20_SINGLE_ZONE_HARD_REJECT", func(t *testing.T) {
		got := EvaluateGroupage(Equipment{TemperatureZoneCount: i(1), TemperatureControlMode: s("ACTIVE"), TemperatureMinC: f64(-30), TemperatureMaxC: f64(20)}, []Cargo{
			tempCargo("a", 2, 8), tempCargo("b", -25, -18),
		}, AccessNeed{}, Context{})
		requireCode(t, got, "TEMPERATURE_RANGES_INCOMPATIBLE")
		if got.Status != StatusIncompatible {
			t.Fatal(got.Status)
		}
	})
	t.Run("BNO117_OVERLAPPING_TEMP_RANGES_PASS", func(t *testing.T) {
		got := EvaluateGroupage(Equipment{TemperatureZoneCount: i(1), TemperatureControlMode: s("ACTIVE"), TemperatureMinC: f64(-25), TemperatureMaxC: f64(20)}, []Cargo{
			tempCargo("a", 2, 8), tempCargo("b", 4, 6),
		}, AccessNeed{}, Context{})
		if got.Status != StatusCompatible || got.Temperature == nil || *got.Temperature.CommonMinC != 4 || *got.Temperature.CommonMaxC != 6 {
			t.Fatalf("%s %#v", got.Status, got.Temperature)
		}
	})
	food := Rule{RuleCode: "FOOD_CONTAM", RuleKind: KindCargoCargo, Layer: LayerPlatform, LeftSelectorType: "TAG", LeftSelectorValue: "FOOD", RightSelectorType: "TAG", RightSelectorValue: "CONTAMINATION_RISK_HIGH", Decision: DecisionDeny, ReasonCode: "CONTAMINATION_COMPATIBILITY_RULE", RuleSetVersion: 1}
	t.Run("BNO118_FOOD_RULE_FROM_ACTIVE_RULESET", func(t *testing.T) {
		got := EvaluateGroupage(Equipment{}, []Cargo{{ID: "a", Tags: []string{"FOOD"}}, {ID: "b", Tags: []string{"CONTAMINATION_RISK_HIGH"}}}, AccessNeed{}, Context{Rules: []Rule{food}})
		requireCode(t, got, "CONTAMINATION_COMPATIBILITY_RULE")
	})
	odor := Rule{RuleCode: "ODOR", RuleKind: KindCargoCargo, Layer: LayerPlatform, LeftSelectorType: "ODOR_SENSITIVE", LeftSelectorValue: "true", RightSelectorType: "ODOR_EMISSION_CLASS", RightSelectorValue: "HIGH", Decision: DecisionDeny, ReasonCode: "ODOR_COMPATIBILITY_RULE", RuleSetVersion: 2}
	t.Run("BNO119_ODOR_RULE_FROM_ACTIVE_RULESET", func(t *testing.T) {
		got := EvaluateGroupage(Equipment{}, []Cargo{{ID: "a", OdorSensitive: b(true)}, {ID: "b", OdorEmissionClass: s("HIGH")}}, AccessNeed{}, Context{Rules: []Rule{odor}})
		requireCode(t, got, "ODOR_COMPATIBILITY_RULE")
	})
	t.Run("BNO120_CONTAMINATION_RULE_FROM_ACTIVE_RULESET", func(t *testing.T) {
		got := EvaluateGroupage(Equipment{}, []Cargo{{ID: "a", Tags: []string{"FOOD"}}, {ID: "b", ContaminationClass: s("HIGH")}}, AccessNeed{}, Context{Rules: []Rule{{
			RuleCode: "CONTAM", RuleKind: KindCargoCargo, Layer: LayerPlatform, LeftSelectorType: "TAG", LeftSelectorValue: "FOOD", RightSelectorType: "CONTAMINATION_CLASS", RightSelectorValue: "HIGH", Decision: DecisionDeny, ReasonCode: "CONTAMINATION_COMPATIBILITY_RULE", RuleSetVersion: 3,
		}}})
		requireCode(t, got, "CONTAMINATION_COMPATIBILITY_RULE")
	})
	adr := Rule{RuleCode: "ADR3", RuleKind: KindCargoCargo, Layer: LayerRegulatory, LeftSelectorType: "HAZARD_CLASS", LeftSelectorValue: "3", RightSelectorType: "HAZARD_CLASS", RightSelectorValue: "5.1", Decision: DecisionDeny, ReasonCode: "ADR_INCOMPATIBLE", SourceReference: s("ADR-2025-7.5.2"), RuleSetVersion: 4}
	t.Run("BNO121_ADR_SOURCED_RULE_APPLIED", func(t *testing.T) {
		got := EvaluateGroupage(Equipment{ADRCapability: b(true)}, []Cargo{{ID: "a", DangerousGoods: b(true), HazardClasses: []string{"3"}}, {ID: "b", DangerousGoods: b(true), HazardClasses: []string{"5.1"}}}, AccessNeed{}, Context{Rules: []Rule{adr}})
		requireCode(t, got, "ADR_INCOMPATIBLE")
		if got.HardRejects[0].SourceReference == nil || got.HardRejects[0].RuleSetVersion == nil {
			t.Fatal("explanation missing source or version")
		}
	})
	t.Run("BNO122_ADR_RULE_MISSING_INDETERMINATE", func(t *testing.T) {
		got := EvaluateCargoEquipment(Cargo{ID: "a", DangerousGoods: b(true), HazardClasses: []string{"3"}}, Equipment{ADRCapability: b(true)}, AccessNeed{}, Context{})
		requireIndeterminate(t, got, "ADR_COMPATIBILITY_RULE_UNAVAILABLE")
	})
	t.Run("BNO123_PAIRWISE_MATRIX_GENERATED", func(t *testing.T) {
		got := EvaluateGroupage(Equipment{}, []Cargo{{ID: "a", Tags: []string{"FOOD"}}, {ID: "b"}, {ID: "c", Tags: []string{"CONTAMINATION_RISK_HIGH"}}}, AccessNeed{}, Context{Rules: []Rule{food}})
		if len(got.CargoPairs) != 3 {
			t.Fatalf("pairs %d", len(got.CargoPairs))
		}
	})
	t.Run("BNO124_ANY_HARD_REJECT_REJECTS_GROUPAGE", func(t *testing.T) {
		got := EvaluateGroupage(Equipment{}, []Cargo{{ID: "a", Tags: []string{"FOOD"}}, {ID: "b"}, {ID: "c", Tags: []string{"CONTAMINATION_RISK_HIGH"}}}, AccessNeed{}, Context{Rules: []Rule{food}})
		if got.Status != StatusIncompatible {
			t.Fatal(got.Status)
		}
	})
	t.Run("BNO125_UNKNOWN_PAIR_FACT_INDETERMINATE", func(t *testing.T) {
		got := EvaluateGroupage(Equipment{}, []Cargo{{ID: "a", Tags: []string{"FOOD"}, ContaminationClass: nil}, {ID: "b", Tags: []string{"FOOD"}}}, AccessNeed{}, Context{Rules: []Rule{{
			RuleCode: "NEED", RuleKind: KindCargoCargo, Layer: LayerPlatform, LeftSelectorType: "CONTAMINATION_CLASS", LeftSelectorValue: "HIGH", RightSelectorType: "TAG", RightSelectorValue: "FOOD", Decision: DecisionDeny, ReasonCode: "CONTAMINATION_COMPATIBILITY_RULE",
		}}})
		requireIndeterminate(t, got, "CONTAMINATION_COMPATIBILITY_RULE")
	})
	t.Run("BNO126_MULTI_ZONE_SINGLE_ZONE_REJECT", func(t *testing.T) {
		got := EvaluateGroupage(Equipment{TemperatureZoneCount: i(1)}, []Cargo{tempCargo("a", 2, 8), tempCargo("b", -25, -18)}, AccessNeed{}, Context{})
		requireCode(t, got, "MULTI_ZONE_REQUIRED")
	})
	t.Run("BNO127_MULTI_ZONE_WITHOUT_ALLOCATOR_INDETERMINATE", func(t *testing.T) {
		got := EvaluateGroupage(Equipment{TemperatureZoneCount: i(2), IndependentTemperatureControl: b(true), TemperatureControlMode: s("ACTIVE"), TemperatureMinC: f64(-30), TemperatureMaxC: f64(30)}, []Cargo{tempCargo("a", 2, 8), tempCargo("b", -25, -18)}, AccessNeed{}, Context{})
		requireIndeterminate(t, got, "MULTI_ZONE_ALLOCATION_REQUIRED")
		if got.Status == StatusCompatible {
			t.Fatal("allocator absence must not be compatible")
		}
	})
	t.Run("BNO128_GROUPAGE_WEIGHT_SUM", func(t *testing.T) {
		got := EvaluateGroupage(Equipment{PayloadKg: f64(22000)}, []Cargo{{ID: "a", WeightKg: f64(6000)}, {ID: "b", WeightKg: f64(4500)}, {ID: "c", WeightKg: f64(2000)}}, AccessNeed{}, Context{})
		if got.Status != StatusCompatible || got.CapacityUsage.WeightUsed == nil || *got.CapacityUsage.WeightUsed != 12500 {
			t.Fatalf("%s %#v", got.Status, got.CapacityUsage)
		}
	})
	t.Run("BNO129_GROUPAGE_WEIGHT_EXCEEDED", func(t *testing.T) {
		got := EvaluateGroupage(Equipment{PayloadKg: f64(10000)}, []Cargo{{ID: "a", WeightKg: f64(6000)}, {ID: "b", WeightKg: f64(4500)}}, AccessNeed{}, Context{})
		requireCode(t, got, "PAYLOAD_EXCEEDED")
	})
	t.Run("BNO130_GROUPAGE_VOLUME_SUM", func(t *testing.T) {
		got := EvaluateGroupage(Equipment{VolumeM3: f64(80)}, []Cargo{{ID: "a", VolumeM3: f64(20)}, {ID: "b", VolumeM3: f64(15)}}, AccessNeed{}, Context{})
		if got.CapacityUsage.VolumeUsed == nil || *got.CapacityUsage.VolumeUsed != 35 || got.Status != StatusCompatible {
			t.Fatal(got.Status)
		}
	})
	t.Run("BNO131_GROUPAGE_PALLET_SUM", func(t *testing.T) {
		got := EvaluateGroupage(Equipment{PalletPositions: i(33), PalletBasisCode: s("EUR")}, []Cargo{pallet("a", 10, "EUR"), pallet("b", 12, "EUR")}, AccessNeed{}, Context{})
		if got.CapacityUsage.PalletsUsed == nil || *got.CapacityUsage.PalletsUsed != 22 || got.Status != StatusCompatible {
			t.Fatalf("%s %#v", got.Status, got.CapacityUsage)
		}
	})
	t.Run("BNO132_GROUPAGE_LINEAR_METERS_SUM", func(t *testing.T) {
		got := EvaluateGroupage(Equipment{UsableLinearMeters: f64(13.6)}, []Cargo{{ID: "a", LinearMeters: f64(4)}, {ID: "b", LinearMeters: f64(5)}}, AccessNeed{}, Context{})
		if got.CapacityUsage.LinearMetersUsed == nil || *got.CapacityUsage.LinearMetersUsed != 9 || got.Status != StatusCompatible {
			t.Fatal(got.Status)
		}
	})
	t.Run("BNO133_MULTI_DIMENSION_ONE_FAILURE_REJECTS", func(t *testing.T) {
		got := EvaluateCargoEquipment(Cargo{ID: "a", WeightKg: f64(1000), VolumeM3: f64(2), PalletCount: i(40), PalletTypeCode: s("EUR")}, Equipment{PayloadKg: f64(22000), VolumeM3: f64(80), PalletPositions: i(33), PalletBasisCode: s("EUR")}, AccessNeed{}, Context{})
		if got.Status != StatusIncompatible {
			t.Fatal(got.Status)
		}
		requireCode(t, got, "PALLET_POSITIONS_EXCEEDED")
	})
	t.Run("BNO134_UNKNOWN_REQUIRED_DIMENSION_INDETERMINATE", func(t *testing.T) {
		got := EvaluateCargoEquipment(Cargo{ID: "a", WeightKg: f64(1000), VolumeM3: f64(2), PalletCount: i(10)}, Equipment{PayloadKg: f64(22000), VolumeM3: f64(80), PalletPositions: i(33)}, AccessNeed{}, Context{})
		requireIndeterminate(t, got, "PALLET_TYPE_UNKNOWN")
	})
	t.Run("BNO135_SPECIFIC_CARGO_FACT_OVERRIDES_CATALOG_DEFAULT", func(t *testing.T) {
		got := ApplyCatalogDefault(Cargo{TemperatureRequired: b(true), TemperatureMinC: f64(2), TemperatureMaxC: f64(6)}, Cargo{TemperatureRequired: b(false)})
		if got.TemperatureRequired == nil || !*got.TemperatureRequired || got.Provenance["temperature"] != ProvenanceCargoConfirmed {
			t.Fatalf("%#v", got)
		}
	})
	t.Run("BNO136_REFERENCE_DEFAULT_PROVENANCE_PRESERVED", func(t *testing.T) {
		got := ApplyEquipmentDefault(Equipment{}, Equipment{PayloadKg: f64(22000)})
		if got.Provenance["payload_kg"] != ProvenanceReferenceDefault || *got.PayloadKg != 22000 {
			t.Fatal(got.Provenance)
		}
	})
	t.Run("BNO140_ORDER_INDEPENDENT_FINGERPRINT", func(t *testing.T) {
		eq := Equipment{PayloadKg: f64(100)}
		left := EvaluateGroupage(eq, []Cargo{{ID: "a", WeightKg: f64(1)}, {ID: "c", WeightKg: f64(2)}, {ID: "b", WeightKg: f64(3)}}, AccessNeed{}, Context{CargoCatalogVersion: 1})
		right := EvaluateGroupage(eq, []Cargo{{ID: "c", WeightKg: f64(2)}, {ID: "a", WeightKg: f64(1)}, {ID: "b", WeightKg: f64(3)}}, AccessNeed{}, Context{CargoCatalogVersion: 1})
		if left.Fingerprint == "" || left.Fingerprint != right.Fingerprint || left.Status != right.Status {
			t.Fatalf("%s %s", left.Fingerprint, right.Fingerprint)
		}
	})
	t.Run("UNKNOWN_LOADING_IS_NOT_SUPPORTED", func(t *testing.T) {
		got := EvaluateCargoEquipment(Cargo{ID: "a"}, Equipment{}, AccessNeed{AllowedLoadingAccess: []string{"REAR"}}, Context{})
		requireIndeterminate(t, got, "LOADING_ACCESS_UNKNOWN")
	})
	alias, ok := ResolveAlias("реф", map[string]string{"REF": "REFRIGERATOR"})
	if ok || alias != "" {
		t.Fatal("fuzzy alias must not resolve")
	}
	alias, ok = ResolveAlias("REF", map[string]string{"REF": "REFRIGERATOR"})
	if !ok || alias != "REFRIGERATOR" {
		t.Fatal("explicit alias must resolve")
	}
}

func tempCargo(id string, min, max float64) Cargo {
	return Cargo{ID: id, TemperatureRequired: b(true), TemperatureMinC: f64(min), TemperatureMaxC: f64(max)}
}

func pallet(id string, count int, kind string) Cargo {
	return Cargo{ID: id, PalletCount: i(count), PalletTypeCode: s(kind), PackagingTypeCode: s("PALLETIZED")}
}

func stackablePallet(id string, count int) Cargo {
	cargo := pallet(id, count, "EUR")
	cargo.Stackable = b(true)
	return cargo
}

func requireCode(t *testing.T, got Result, code string) {
	t.Helper()
	if got.Status != StatusIncompatible || !hasReason(got.HardRejects, code) {
		t.Fatalf("want hard %s, got %s %#v %#v", code, got.Status, got.HardRejects, got.IndeterminateReasons)
	}
}

func requireIndeterminate(t *testing.T, got Result, code string) {
	t.Helper()
	if got.Status != StatusIndeterminate || !hasReason(got.IndeterminateReasons, code) {
		t.Fatalf("want indeterminate %s, got %s %#v %#v", code, got.Status, got.IndeterminateReasons, got.HardRejects)
	}
}

func hasReason(reasons []Reason, code string) bool {
	for _, reason := range reasons {
		if reason.ReasonCode == code {
			return true
		}
	}
	return false
}

func f64(v float64) *float64 { return &v }
func i(v int) *int           { return &v }
func b(v bool) *bool         { return &v }
func s(v string) *string     { return &v }

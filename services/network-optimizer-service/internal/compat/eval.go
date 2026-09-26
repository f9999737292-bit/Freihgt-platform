package compat

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

func EvaluateCargoEquipment(cargo Cargo, equipment Equipment, need AccessNeed, ctx Context) Result {
	return EvaluateGroupage(equipment, []Cargo{cargo}, need, ctx)
}

type GroupageItem struct {
	Cargo      Cargo
	AccessNeed AccessNeed
}

func EvaluateGroupage(equipment Equipment, cargoes []Cargo, need AccessNeed, ctx Context) Result {
	items := make([]GroupageItem, len(cargoes))
	for i, cargo := range cargoes {
		items[i] = GroupageItem{Cargo: cargo, AccessNeed: need}
	}
	return evaluateGroupageItems(equipment, items, ctx, false)
}

func EvaluateGroupageItems(equipment Equipment, items []GroupageItem, ctx Context) Result {
	return evaluateGroupageItems(equipment, items, ctx, true)
}

func evaluateGroupageItems(equipment Equipment, items []GroupageItem, ctx Context, fingerprintAccess bool) Result {
	items = append([]GroupageItem(nil), items...)
	sort.Slice(items, func(i, j int) bool { return items[i].Cargo.ID < items[j].Cargo.ID })
	cargoes := make([]Cargo, len(items))
	needs := make(map[string]AccessNeed, len(items))
	for i, item := range items {
		cargoes[i] = item.Cargo
		needs[item.Cargo.ID] = item.AccessNeed
	}
	var classResult Result
	equipment, cargoes, classResult = ResolveProfiles(equipment, cargoes, ctx)
	resolved := make([]GroupageItem, len(cargoes))
	for i, cargo := range cargoes {
		resolved[i] = GroupageItem{Cargo: cargo, AccessNeed: needs[cargo.ID]}
	}
	out := Result{
		HardRejects: []Reason{}, IndeterminateReasons: []Reason{}, Conditions: []Reason{}, Warnings: []Reason{},
		RuleSetVersions: ruleVersions(ctx.Rules),
		CatalogVersions: map[string]int{
			"cargo": ctx.CargoCatalogVersion, "equipment": ctx.EquipmentCatalogVersion,
			"pallet": ctx.PalletCatalogVersion, "packaging": ctx.PackagingCatalogVersion,
		},
		RuleSetsUsed:        sortedRuleSets(ctx.RuleSets),
		CatalogVersionsUsed: sortedCatalogRefs(ctx.CatalogRefs),
	}
	merge(&out, classResult)
	for _, item := range resolved {
		part := cargoEquipment(item.Cargo, equipment, item.AccessNeed, ctx)
		out.CargoEquipment = append(out.CargoEquipment, PairResult{Left: item.Cargo.ID, Right: "equipment", Status: part.Status, Reasons: append(append([]Reason{}, part.HardRejects...), part.IndeterminateReasons...)})
		merge(&out, part)
	}
	for i := 0; i < len(resolved); i++ {
		for j := i + 1; j < len(resolved); j++ {
			pair, part := cargoPair(resolved[i].Cargo, resolved[j].Cargo, ctx)
			out.CargoPairs = append(out.CargoPairs, pair)
			merge(&out, part)
		}
	}
	usage, physical := capacity(cargoes, equipment, ctx)
	out.CapacityUsage = &usage
	merge(&out, physical)
	temp := temperatures(cargoes, equipment, ctx)
	out.Temperature = temp.Temperature
	merge(&out, temp)
	adr := adrCheck(cargoes, equipment, ctx)
	merge(&out, adr)
	out.Status = resolve(out)
	if fingerprintAccess {
		out.Fingerprint = fingerprintItems(equipment, resolved, ctx)
	} else {
		out.Fingerprint = fingerprint(equipment, cargoes, ctx)
	}
	return out
}

func cargoEquipment(cargo Cargo, equipment Equipment, need AccessNeed, ctx Context) Result {
	var out Result
	if need.RequiredBodyTypes != nil && len(need.RequiredBodyTypes) > 0 || cargo.CargoTypeCode != nil {
		bodyNeed := need.RequiredBodyTypes
		if len(bodyNeed) == 0 {
			bodyNeed = nil
		}
		hard, reason := bodyResult(bodyNeed, equipment.BodyType)
		add(&out, hard, reason)
	} else {
		hard, reason := bodyResult(need.RequiredBodyTypes, equipment.BodyType)
		add(&out, hard, reason)
	}
	hard, reason := tokenSet(need.RequiredEquipmentTypes, equipment.EquipmentTypeCode, "EQUIPMENT_TYPE_UNKNOWN", "EQUIPMENT_TYPE_INCOMPATIBLE", "equipment_type")
	add(&out, hard, reason)
	hard, reason = accessResult(need.RequiredLoadingAccess, need.AllowedLoadingAccess, equipment.LoadingAccess, "LOADING_ACCESS_UNSUPPORTED", "LOADING_ACCESS_UNKNOWN")
	add(&out, hard, reason)
	hard, reason = accessResult(need.RequiredUnloadingAccess, need.AllowedUnloadingAccess, equipment.UnloadingAccess, "UNLOADING_ACCESS_UNSUPPORTED", "UNLOADING_ACCESS_UNKNOWN")
	add(&out, hard, reason)
	hard, reason = singleTemperature(cargo, equipment)
	add(&out, hard, reason)
	hard, reason = foodGrade(cargo, equipment)
	add(&out, hard, reason)
	for _, rule := range matchingEquipment(cargo, equipment, ctx.Rules) {
		applyRule(&out, rule, "cargo_equipment")
	}
	out.Status = resolve(out)
	return out
}

func cargoPair(a, b Cargo, ctx Context) (PairResult, Result) {
	var part Result
	for _, rule := range matchingPair(a, b, ctx.Rules) {
		applyRule(&part, rule, "cargo_cargo")
	}
	if a.ContaminationClass == nil && ruleNeeds(ctx.Rules, "CONTAMINATION_CLASS") && (hasTag(a, "FOOD") || hasTag(b, "FOOD")) {
		add(&part, false, Reason{ReasonCode: "CONTAMINATION_COMPATIBILITY_RULE", Dimension: "contamination"})
	}
	part.Status = resolve(part)
	reasons := append([]Reason{}, part.HardRejects...)
	reasons = append(reasons, part.IndeterminateReasons...)
	reasons = append(reasons, part.Warnings...)
	return PairResult{Left: a.ID, Right: b.ID, Status: part.Status, Reasons: reasons}, part
}

func capacity(cargoes []Cargo, equipment Equipment, ctx Context) (Usage, Result) {
	var out Result
	usage := Usage{WeightAvailable: equipment.PayloadKg, VolumeAvailable: equipment.VolumeM3, PalletsAvailable: equipment.PalletPositions, LinearMetersAvailable: equipment.UsableLinearMeters}
	if cargoHas(cargoes, func(c Cargo) bool { return c.WeightKg != nil }) || equipment.PayloadKg != nil {
		weight, weightOK := sumFloat(pluckFloat(cargoes, func(c Cargo) *float64 { return c.WeightKg }))
		if equipment.PayloadKg == nil || !weightOK {
			add(&out, false, Reason{ReasonCode: "PAYLOAD_UNKNOWN", Dimension: "payload"})
		} else {
			usage.WeightUsed = &weight
			if weight > *equipment.PayloadKg {
				add(&out, true, Reason{ReasonCode: "PAYLOAD_EXCEEDED", Dimension: "payload"})
			}
		}
	}
	if cargoHas(cargoes, func(c Cargo) bool { return c.VolumeM3 != nil }) || equipment.VolumeM3 != nil {
		volume, volumeOK := sumFloat(pluckFloat(cargoes, func(c Cargo) *float64 { return c.VolumeM3 }))
		if equipment.VolumeM3 == nil || !volumeOK {
			add(&out, false, Reason{ReasonCode: "VOLUME_UNKNOWN", Dimension: "volume"})
		} else {
			usage.VolumeUsed = &volume
			if volume > *equipment.VolumeM3 {
				add(&out, true, Reason{ReasonCode: "VOLUME_EXCEEDED", Dimension: "volume"})
			}
		}
	}
	addPallets(&out, &usage, cargoes, equipment, ctx)
	if relevantLinear(cargoes) {
		if sum, ok := sumFloat(pluckFloat(cargoes, func(c Cargo) *float64 { return c.LinearMeters })); equipment.UsableLinearMeters == nil || !ok {
			add(&out, false, Reason{ReasonCode: "LINEAR_METERS_UNKNOWN", Dimension: "linear_meters"})
		} else {
			usage.LinearMetersUsed = &sum
			if sum > *equipment.UsableLinearMeters {
				add(&out, true, Reason{ReasonCode: "LINEAR_METERS_EXCEEDED", Dimension: "linear_meters"})
			}
		}
	}
	if relevantHeight(cargoes) {
		if equipment.InternalHeightMM == nil || !allHeights(cargoes) {
			add(&out, false, Reason{ReasonCode: "HEIGHT_UNKNOWN", Dimension: "height"})
		} else {
			for _, cargo := range cargoes {
				if cargo.MaxLoadedHeightMM != nil && *cargo.MaxLoadedHeightMM > *equipment.InternalHeightMM {
					add(&out, true, Reason{ReasonCode: "HEIGHT_EXCEEDED", Dimension: "height"})
					break
				}
			}
		}
	}
	out.Status = resolve(out)
	return usage, out
}

func addPallets(out *Result, usage *Usage, cargoes []Cargo, equipment Equipment, ctx Context) {
	if !relevantPallets(cargoes) {
		return
	}
	var positions float64
	var basis string
	if equipment.PalletBasisCode != nil {
		basis = *equipment.PalletBasisCode
	}
	seen := map[string]struct{}{}
	for _, cargo := range cargoes {
		if cargo.PalletCount == nil {
			add(out, false, Reason{ReasonCode: "PALLET_COUNT_UNKNOWN", Dimension: "pallets"})
			return
		}
		if cargo.PalletTypeCode == nil {
			add(out, false, Reason{ReasonCode: "PALLET_TYPE_UNKNOWN", Dimension: "pallets"})
			return
		}
		seen[*cargo.PalletTypeCode] = struct{}{}
		each := 1.0
		if basis != "" && basis != *cargo.PalletTypeCode {
			factor, ok := equivalence(*cargo.PalletTypeCode, basis, ctx.Equivalences)
			if !ok {
				add(out, false, Reason{ReasonCode: "PALLET_EQUIVALENCE_UNKNOWN", Dimension: "pallets"})
				return
			}
			each = factor
		}
		if basis == "" && len(seen) > 1 {
			add(out, false, Reason{ReasonCode: "PALLET_EQUIVALENCE_UNKNOWN", Dimension: "pallets"})
			return
		}
		positions += float64(*cargo.PalletCount) * each
	}
	if len(seen) > 1 && basis == "" {
		add(out, false, Reason{ReasonCode: "PALLET_EQUIVALENCE_UNKNOWN", Dimension: "pallets"})
		return
	}
	usage.PalletsUsed = &positions
	if equipment.PalletPositions == nil {
		add(out, false, Reason{ReasonCode: "PALLET_POSITIONS_UNKNOWN", Dimension: "pallets"})
		return
	}
	if positions > float64(*equipment.PalletPositions) {
		add(out, true, Reason{ReasonCode: "PALLET_POSITIONS_EXCEEDED", Dimension: "pallets"})
	}
}

func temperatures(cargoes []Cargo, equipment Equipment, ctx Context) Result {
	var out Result
	if len(cargoes) == 0 {
		return out
	}
	low, high, state := commonInterval(cargoes)
	if state == "none" {
		out.Status = StatusCompatible
		return out
	}
	if state == "unknown" {
		add(&out, false, Reason{ReasonCode: "TEMPERATURE_CAPABILITY_UNKNOWN", Dimension: "temperature"})
		out.Status = resolve(out)
		return out
	}
	if state == "incompatible" {
		zones := 1
		if equipment.TemperatureZoneCount != nil {
			zones = *equipment.TemperatureZoneCount
		}
		if zones > 1 && equipment.IndependentTemperatureControl != nil && *equipment.IndependentTemperatureControl {
			add(&out, false, Reason{ReasonCode: "MULTI_ZONE_ALLOCATION_REQUIRED", Dimension: "temperature", CatalogVersion: &ctx.EquipmentCatalogVersion})
		} else {
			add(&out, true, Reason{ReasonCode: "TEMPERATURE_RANGES_INCOMPATIBLE", Dimension: "temperature"})
			if zones == 1 {
				add(&out, true, Reason{ReasonCode: "MULTI_ZONE_REQUIRED", Dimension: "temperature"})
			}
		}
		out.Status = resolve(out)
		return out
	}
	out.Temperature = &TemperatureOutcome{CommonMinC: &low, CommonMaxC: &high}
	mode := ""
	if equipment.TemperatureControlMode != nil {
		mode = *equipment.TemperatureControlMode
	}
	needsActive := false
	for _, cargo := range cargoes {
		if cargo.TemperatureRequired != nil && *cargo.TemperatureRequired {
			needsActive = true
		}
	}
	if needsActive && mode != "ACTIVE" {
		add(&out, true, Reason{ReasonCode: "TEMPERATURE_CONTROL_REQUIRED", Dimension: "temperature"})
	}
	if equipment.TemperatureMinC == nil || equipment.TemperatureMaxC == nil {
		if needsActive || anyTemp(cargoes) {
			add(&out, false, Reason{ReasonCode: "TEMPERATURE_CAPABILITY_UNKNOWN", Dimension: "temperature"})
		}
	} else if low < *equipment.TemperatureMinC || high > *equipment.TemperatureMaxC {
		add(&out, true, Reason{ReasonCode: "TEMPERATURE_RANGE_UNSUPPORTED", Dimension: "temperature"})
	}
	out.Status = resolve(out)
	return out
}

func singleTemperature(cargo Cargo, equipment Equipment) (bool, Reason) {
	part := temperatures([]Cargo{cargo}, equipment, Context{})
	if len(part.HardRejects) > 0 {
		return true, part.HardRejects[0]
	}
	if len(part.IndeterminateReasons) > 0 {
		return false, part.IndeterminateReasons[0]
	}
	return false, Reason{}
}

func adrCheck(cargoes []Cargo, equipment Equipment, ctx Context) Result {
	var out Result
	needs := false
	for _, cargo := range cargoes {
		if cargo.DangerousGoods != nil && *cargo.DangerousGoods || len(cargo.HazardClasses) > 0 {
			needs = true
		}
	}
	if !needs {
		return out
	}
	if equipment.ADRCapability == nil {
		add(&out, false, Reason{ReasonCode: "ADR_CAPABILITY_UNKNOWN", Dimension: "adr"})
	} else if !*equipment.ADRCapability {
		add(&out, true, Reason{ReasonCode: "ADR_INCOMPATIBLE", Dimension: "adr"})
	}
	applicable := false
	for _, rule := range ctx.Rules {
		if rule.Layer != LayerRegulatory {
			continue
		}
		if rule.SourceReference == nil || strings.TrimSpace(*rule.SourceReference) == "" {
			continue
		}
		if !regulatoryHit(rule, cargoes, equipment) {
			continue
		}
		applicable = true
		if rule.Decision == DecisionDeny {
			add(&out, true, reasonFrom(rule, "adr"))
		}
	}
	if !applicable {
		add(&out, false, Reason{ReasonCode: "ADR_COMPATIBILITY_RULE_UNAVAILABLE", Dimension: "adr"})
	}
	out.Status = resolve(out)
	return out
}

func foodGrade(cargo Cargo, equipment Equipment) (bool, Reason) {
	if cargo.FoodGradeRequired == nil || !*cargo.FoodGradeRequired {
		return false, Reason{}
	}
	if equipment.FoodGradeCapability == nil {
		return false, Reason{ReasonCode: "FOOD_GRADE_CAPABILITY_UNKNOWN", Dimension: "food_grade"}
	}
	if !*equipment.FoodGradeCapability {
		return true, Reason{ReasonCode: "FOOD_GRADE_REQUIRED", Dimension: "food_grade"}
	}
	return false, Reason{}
}

func tokenSet(required []string, actual *string, unknownCode, incompatibleCode, dimension string) (bool, Reason) {
	if len(required) == 0 {
		return false, Reason{}
	}
	if actual == nil || *actual == "" {
		return false, Reason{ReasonCode: unknownCode, Dimension: dimension}
	}
	for _, item := range required {
		if item == *actual {
			return false, Reason{}
		}
	}
	return true, Reason{ReasonCode: incompatibleCode, Dimension: dimension}
}

func bodyResult(required []string, body *string) (bool, Reason) {
	if len(required) == 0 {
		return false, Reason{}
	}
	if body == nil || *body == "" {
		return false, Reason{ReasonCode: "BODY_TYPE_UNKNOWN", Dimension: "body"}
	}
	for _, item := range required {
		if item == *body {
			return false, Reason{}
		}
	}
	return true, Reason{ReasonCode: "BODY_TYPE_INCOMPATIBLE", Dimension: "body"}
}

func accessResult(required, allowed, supported []string, unsupported, unknown string) (bool, Reason) {
	if len(required) == 0 && len(allowed) == 0 {
		return false, Reason{}
	}
	if supported == nil {
		return false, Reason{ReasonCode: unknown, Dimension: "access"}
	}
	have := map[string]struct{}{}
	for _, item := range supported {
		have[item] = struct{}{}
	}
	for _, item := range required {
		if _, ok := have[item]; !ok {
			return true, Reason{ReasonCode: unsupported, Dimension: "access"}
		}
	}
	if len(allowed) > 0 {
		ok := false
		for _, item := range allowed {
			if _, found := have[item]; found {
				ok = true
			}
		}
		if !ok {
			return true, Reason{ReasonCode: unsupported, Dimension: "access"}
		}
	}
	return false, Reason{}
}

func applyRule(out *Result, rule Rule, dimension string) {
	reason := reasonFrom(rule, dimension)
	switch rule.Decision {
	case DecisionDeny:
		if rule.Severity == SeveritySoft {
			out.Warnings = append(out.Warnings, reason)
			return
		}
		out.HardRejects = append(out.HardRejects, reason)
	case DecisionRequireCondition, DecisionRequireSeparation:
		out.Conditions = append(out.Conditions, reason)
		out.IndeterminateReasons = append(out.IndeterminateReasons, reason)
	}
}

func matchingEquipment(cargo Cargo, equipment Equipment, rules []Rule) []Rule {
	var matched []Rule
	for _, rule := range sortedRules(rules) {
		if rule.RuleKind != KindCargoEquipment || rule.Decision == DecisionAllow {
			continue
		}
		if matchCargo(rule.LeftSelectorType, rule.LeftSelectorValue, cargo) && matchEquipment(rule.RightSelectorType, rule.RightSelectorValue, equipment) {
			matched = append(matched, rule)
		}
	}
	return matched
}

func matchingPair(a, b Cargo, rules []Rule) []Rule {
	var matched []Rule
	for _, rule := range sortedRules(rules) {
		if rule.RuleKind != KindCargoCargo || rule.Decision == DecisionAllow {
			continue
		}
		direct := matchCargo(rule.LeftSelectorType, rule.LeftSelectorValue, a) && matchCargo(rule.RightSelectorType, rule.RightSelectorValue, b)
		swap := matchCargo(rule.LeftSelectorType, rule.LeftSelectorValue, b) && matchCargo(rule.RightSelectorType, rule.RightSelectorValue, a)
		if direct || swap {
			matched = append(matched, rule)
		}
	}
	return matched
}

func sortedRules(rules []Rule) []Rule {
	out := append([]Rule(nil), rules...)
	sort.Slice(out, func(i, j int) bool {
		if layerRank(out[i].Layer) != layerRank(out[j].Layer) {
			return layerRank(out[i].Layer) < layerRank(out[j].Layer)
		}
		return out[i].Priority > out[j].Priority
	})
	return out
}

func layerRank(layer string) int {
	switch layer {
	case LayerRegulatory:
		return 0
	case LayerPlatform:
		return 1
	default:
		return 2
	}
}

func matchCargo(kind, value string, cargo Cargo) bool {
	switch kind {
	case "ANY":
		return true
	case "CARGO_TYPE":
		return cargo.CargoTypeCode != nil && *cargo.CargoTypeCode == value
	case "PARENT":
		for _, parent := range cargo.ParentCodes {
			if parent == value {
				return true
			}
		}
		return cargo.CargoTypeCode != nil && *cargo.CargoTypeCode == value
	case "TAG":
		return hasTag(cargo, value)
	case "ODOR_EMISSION_CLASS":
		return cargo.OdorEmissionClass != nil && *cargo.OdorEmissionClass == value
	case "ODOR_SENSITIVE":
		return cargo.OdorSensitive != nil && ((*cargo.OdorSensitive && value == "true") || (!*cargo.OdorSensitive && value == "false"))
	case "CONTAMINATION_CLASS":
		return cargo.ContaminationClass != nil && *cargo.ContaminationClass == value
	case "HAZARD_CLASS":
		for _, class := range cargo.HazardClasses {
			if class == value {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func matchEquipment(kind, value string, equipment Equipment) bool {
	switch kind {
	case "ANY":
		return true
	case "BODY_TYPE":
		return equipment.BodyType != nil && *equipment.BodyType == value
	case "EQUIPMENT_TYPE":
		return equipment.EquipmentTypeCode != nil && *equipment.EquipmentTypeCode == value
	case "UNIT_KIND":
		return equipment.UnitKind != nil && *equipment.UnitKind == value
	default:
		return false
	}
}

func regulatoryHit(rule Rule, cargoes []Cargo, equipment Equipment) bool {
	if rule.RuleKind == KindCargoEquipment {
		for _, cargo := range cargoes {
			if matchCargo(rule.LeftSelectorType, rule.LeftSelectorValue, cargo) && matchEquipment(rule.RightSelectorType, rule.RightSelectorValue, equipment) {
				return true
			}
		}
		return false
	}
	for i := 0; i < len(cargoes); i++ {
		for j := 0; j < len(cargoes); j++ {
			if i == j {
				continue
			}
			if matchCargo(rule.LeftSelectorType, rule.LeftSelectorValue, cargoes[i]) && matchCargo(rule.RightSelectorType, rule.RightSelectorValue, cargoes[j]) {
				return true
			}
		}
	}
	return false
}

func commonInterval(cargoes []Cargo) (float64, float64, string) {
	var low, high float64
	set := false
	for _, cargo := range cargoes {
		if cargo.TemperatureMinC == nil && cargo.TemperatureMaxC == nil && (cargo.TemperatureRequired == nil || !*cargo.TemperatureRequired) {
			continue
		}
		if cargo.TemperatureMinC == nil || cargo.TemperatureMaxC == nil {
			return 0, 0, "unknown"
		}
		if !set {
			low, high, set = *cargo.TemperatureMinC, *cargo.TemperatureMaxC, true
			continue
		}
		if *cargo.TemperatureMinC > low {
			low = *cargo.TemperatureMinC
		}
		if *cargo.TemperatureMaxC < high {
			high = *cargo.TemperatureMaxC
		}
	}
	if !set {
		return 0, 0, "none"
	}
	if low > high {
		return 0, 0, "incompatible"
	}
	return low, high, "ok"
}

func resolve(out Result) string {
	if len(out.HardRejects) > 0 {
		return StatusIncompatible
	}
	if len(out.IndeterminateReasons) > 0 {
		return StatusIndeterminate
	}
	return StatusCompatible
}

func merge(dst *Result, src Result) {
	dst.HardRejects = append(dst.HardRejects, src.HardRejects...)
	dst.IndeterminateReasons = append(dst.IndeterminateReasons, src.IndeterminateReasons...)
	dst.Conditions = append(dst.Conditions, src.Conditions...)
	dst.Warnings = append(dst.Warnings, src.Warnings...)
}

func add(out *Result, hard bool, reason Reason) {
	if reason.ReasonCode == "" {
		return
	}
	if hard {
		out.HardRejects = append(out.HardRejects, reason)
		return
	}
	out.IndeterminateReasons = append(out.IndeterminateReasons, reason)
}

func reasonFrom(rule Rule, dimension string) Reason {
	code := rule.ReasonCode
	if code == "" {
		code = "CARGO_PAIR_INCOMPATIBLE"
	}
	version := rule.RuleSetVersion
	reason := Reason{
		ReasonCode: code, Dimension: dimension, RuleCode: &rule.RuleCode,
		RuleSetVersion: &version, RequiredSeparation: rule.RequiredSeparation, SourceReference: rule.SourceReference,
	}
	if rule.RuleSetID != "" {
		id := rule.RuleSetID
		reason.RuleSetID = &id
	}
	if rule.RuleSetScope != "" {
		scope := rule.RuleSetScope
		reason.RuleSetScope = &scope
	}
	return reason
}

func ruleVersions(rules []Rule) []int {
	seen := map[int]struct{}{}
	var out []int
	for _, rule := range rules {
		if _, ok := seen[rule.RuleSetVersion]; ok {
			continue
		}
		seen[rule.RuleSetVersion] = struct{}{}
		out = append(out, rule.RuleSetVersion)
	}
	sort.Ints(out)
	return out
}

func equivalence(from, basis string, rows []Equivalence) (float64, bool) {
	for _, row := range rows {
		if row.FromCode == from && row.BasisCode == basis && row.PositionsEach > 0 {
			return row.PositionsEach, true
		}
	}
	return 0, false
}

func relevantPallets(cargoes []Cargo) bool {
	for _, cargo := range cargoes {
		if cargo.PalletCount != nil || cargo.PalletTypeCode != nil || (cargo.PackagingTypeCode != nil && *cargo.PackagingTypeCode == "PALLETIZED") {
			return true
		}
	}
	return false
}

func relevantLinear(cargoes []Cargo) bool {
	for _, cargo := range cargoes {
		if cargo.LinearMeters != nil {
			return true
		}
	}
	return false
}

func relevantHeight(cargoes []Cargo) bool {
	for _, cargo := range cargoes {
		if cargo.MaxLoadedHeightMM != nil {
			return true
		}
	}
	return false
}

func allHeights(cargoes []Cargo) bool {
	for _, cargo := range cargoes {
		if cargo.MaxLoadedHeightMM == nil {
			return false
		}
	}
	return true
}

func anyTemp(cargoes []Cargo) bool {
	for _, cargo := range cargoes {
		if cargo.TemperatureMinC != nil || cargo.TemperatureMaxC != nil {
			return true
		}
	}
	return false
}

func hasTag(cargo Cargo, tag string) bool {
	for _, item := range cargo.Tags {
		if item == tag {
			return true
		}
	}
	return false
}

func ruleNeeds(rules []Rule, selector string) bool {
	for _, rule := range rules {
		if rule.LeftSelectorType == selector || rule.RightSelectorType == selector {
			return true
		}
	}
	return false
}

func cargoHas(cargoes []Cargo, pred func(Cargo) bool) bool {
	for _, cargo := range cargoes {
		if pred(cargo) {
			return true
		}
	}
	return false
}

func pluckFloat(cargoes []Cargo, pick func(Cargo) *float64) []*float64 {
	out := make([]*float64, 0, len(cargoes))
	for _, cargo := range cargoes {
		out = append(out, pick(cargo))
	}
	return out
}

func sumFloat(values []*float64) (float64, bool) {
	var sum float64
	for _, value := range values {
		if value == nil {
			return 0, false
		}
		sum += *value
	}
	return sum, true
}

func fingerprintItems(equipment Equipment, items []GroupageItem, ctx Context) string {
	cargoes := make([]Cargo, len(items))
	needs := make([]AccessNeed, len(items))
	for i, item := range items {
		cargoes[i] = item.Cargo
		needs[i] = item.AccessNeed
	}
	body := struct {
		Base  string       `json:"base"`
		Needs []AccessNeed `json:"access_needs"`
	}{fingerprint(equipment, cargoes, ctx), needs}
	raw, _ := json.Marshal(body)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func fingerprint(equipment Equipment, cargoes []Cargo, ctx Context) string {
	ctx.RuleSets = sortedRuleSets(ctx.RuleSets)
	ctx.CatalogRefs = sortedCatalogRefs(ctx.CatalogRefs)
	body := struct {
		Equipment Equipment `json:"equipment"`
		Cargoes   []Cargo   `json:"cargoes"`
		Catalogs  Context   `json:"catalogs"`
	}{equipment, cargoes, ctx}
	raw, _ := json.Marshal(body)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func sortedRuleSets(items []RuleSetRef) []RuleSetRef {
	out := append([]RuleSetRef{}, items...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Scope != out[j].Scope {
			return out[i].Scope < out[j].Scope
		}
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Version < out[j].Version
	})
	if out == nil {
		return []RuleSetRef{}
	}
	return out
}

func sortedCatalogRefs(items []CatalogVersionRef) []CatalogVersionRef {
	out := append([]CatalogVersionRef{}, items...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].CatalogKind != out[j].CatalogKind {
			return out[i].CatalogKind < out[j].CatalogKind
		}
		if out[i].Scope != out[j].Scope {
			return out[i].Scope < out[j].Scope
		}
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Version < out[j].Version
	})
	if out == nil {
		return []CatalogVersionRef{}
	}
	return out
}

func ApplyCatalogDefault(cargo Cargo, defaults Cargo) Cargo {
	if cargo.TemperatureRequired == nil && defaults.TemperatureRequired != nil {
		cargo.TemperatureRequired = defaults.TemperatureRequired
		cargo.Provenance = putProv(cargo.Provenance, "temperature_required", ProvenanceReferenceDefault)
	}
	if cargo.TemperatureMinC == nil && cargo.TemperatureMaxC == nil && defaults.TemperatureMinC != nil {
		cargo.TemperatureMinC = defaults.TemperatureMinC
		cargo.TemperatureMaxC = defaults.TemperatureMaxC
		cargo.Provenance = putProv(cargo.Provenance, "temperature", ProvenanceReferenceDefault)
	}
	if cargo.TemperatureRequired != nil && *cargo.TemperatureRequired {
		cargo.Provenance = putProv(cargo.Provenance, "temperature", ProvenanceCargoConfirmed)
	}
	return cargo
}

func ApplyEquipmentDefault(asset, defaults Equipment) Equipment {
	if asset.PayloadKg == nil && defaults.PayloadKg != nil {
		asset.PayloadKg = defaults.PayloadKg
		asset.Provenance = putProv(asset.Provenance, "payload_kg", ProvenanceReferenceDefault)
	} else if asset.PayloadKg != nil {
		asset.Provenance = putProv(asset.Provenance, "payload_kg", ProvenanceAssetConfirmed)
	}
	return asset
}

func putProv(in map[string]string, key, value string) map[string]string {
	if in == nil {
		in = map[string]string{}
	}
	if _, ok := in[key]; !ok || value == ProvenanceCargoConfirmed || value == ProvenanceAssetConfirmed {
		in[key] = value
	}
	return in
}

func ResolveAlias(alias string, aliases map[string]string) (string, bool) {
	token := strings.TrimSpace(alias)
	if token == "" {
		return "", false
	}
	canonical, ok := aliases[token]
	return canonical, ok
}

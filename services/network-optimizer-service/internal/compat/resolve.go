package compat

import "strings"

func ResolveProfiles(equipment Equipment, cargoes []Cargo, ctx Context) (Equipment, []Cargo, Result) {
	var out Result
	cargoIndex, cargoOK := indexCargo(ctx.CargoClasses)
	equipmentIndex, equipmentOK := indexEquipment(ctx.EquipmentClasses)
	if ctx.CatalogInvalid || !cargoOK || !equipmentOK {
		add(&out, false, Reason{ReasonCode: "REFERENCE_DATA_UNAVAILABLE", Dimension: "catalog"})
		if cargoAuthoritative(ctx) {
			for i := range cargoes {
				cargoes[i].ParentCodes = nil
				cargoes[i].Tags = nil
			}
		}
		out.Status = resolve(out)
		return equipment, cargoes, out
	}
	if cargoAuthoritative(ctx) {
		for i := range cargoes {
			next, reason := resolveCargo(cargoes[i], cargoIndex, ctx.CargoAliases)
			cargoes[i] = next
			add(&out, false, reason)
		}
	}
	if equipmentAuthoritative(ctx) {
		next, reason := resolveEquipment(equipment, equipmentIndex, ctx.EquipmentAliases)
		equipment = next
		add(&out, false, reason)
	}
	for i := range cargoes {
		rewriteExactAlias(&cargoes[i].PalletTypeCode, ctx.PalletAliases)
		rewriteExactAlias(&cargoes[i].PackagingTypeCode, ctx.PackagingAliases)
	}
	rewriteExactAlias(&equipment.PalletBasisCode, ctx.PalletAliases)
	out.Status = resolve(out)
	return equipment, cargoes, out
}

func cargoAuthoritative(ctx Context) bool {
	return ctx.CargoCatalogVersion > 0 || len(ctx.CargoClasses) > 0
}

func equipmentAuthoritative(ctx Context) bool {
	return ctx.EquipmentCatalogVersion > 0 || len(ctx.EquipmentClasses) > 0
}

func resolveCargo(cargo Cargo, classes map[string]CargoClass, aliases map[string]string) (Cargo, Reason) {
	cargo.ParentCodes = nil
	cargo.Tags = nil
	if cargo.CargoTypeCode == nil || strings.TrimSpace(*cargo.CargoTypeCode) == "" {
		cargo.CargoTypeCode = nil
		return cargo, Reason{}
	}
	code, ok := canonicalCode(strings.TrimSpace(*cargo.CargoTypeCode), classes, aliases)
	if !ok {
		return cargo, Reason{ReasonCode: "REFERENCE_DATA_UNAVAILABLE", Dimension: "cargo_type"}
	}
	parents, ok := ancestors(code, classes)
	if !ok {
		return cargo, Reason{ReasonCode: "REFERENCE_DATA_UNAVAILABLE", Dimension: "cargo_type"}
	}
	cargo.CargoTypeCode = &code
	cargo.ParentCodes = parents
	cargo.Tags = append([]string(nil), classes[code].Tags...)
	return cargo, Reason{}
}

func canonicalCode(code string, classes map[string]CargoClass, aliases map[string]string) (string, bool) {
	if _, ok := classes[code]; ok {
		return code, true
	}
	mapped, ok := aliases[code]
	if !ok {
		return "", false
	}
	if _, ok := classes[mapped]; !ok {
		return "", false
	}
	return mapped, true
}

func ancestors(code string, classes map[string]CargoClass) ([]string, bool) {
	var out []string
	seen := map[string]bool{code: true}
	current := code
	for {
		item, ok := classes[current]
		if !ok {
			return nil, false
		}
		if item.Parent == nil || strings.TrimSpace(*item.Parent) == "" {
			return out, true
		}
		parent := strings.TrimSpace(*item.Parent)
		if seen[parent] {
			return nil, false
		}
		if _, ok := classes[parent]; !ok {
			return nil, false
		}
		seen[parent] = true
		out = append(out, parent)
		current = parent
		if len(out) > len(classes) {
			return nil, false
		}
	}
}

func indexCargo(items []CargoClass) (map[string]CargoClass, bool) {
	system := map[string]CargoClass{}
	tenant := map[string]CargoClass{}
	for _, item := range items {
		item.Code = strings.TrimSpace(item.Code)
		if item.Code == "" {
			return nil, false
		}
		dest := system
		if item.Scope == "TENANT" {
			dest = tenant
		}
		if _, exists := dest[item.Code]; exists {
			return nil, false
		}
		dest[item.Code] = item
	}
	for code, item := range tenant {
		system[code] = item
	}
	return system, true
}

func resolveEquipment(equipment Equipment, classes map[string]EquipmentClass, aliases map[string]string) (Equipment, Reason) {
	if equipment.EquipmentTypeCode == nil || strings.TrimSpace(*equipment.EquipmentTypeCode) == "" {
		return equipment, Reason{}
	}
	code := strings.TrimSpace(*equipment.EquipmentTypeCode)
	class, ok := classes[code]
	if !ok {
		mapped, aliased := aliases[code]
		if !aliased {
			return equipment, Reason{ReasonCode: "REFERENCE_DATA_UNAVAILABLE", Dimension: "equipment_type"}
		}
		class, ok = classes[mapped]
		if !ok {
			return equipment, Reason{ReasonCode: "REFERENCE_DATA_UNAVAILABLE", Dimension: "equipment_type"}
		}
		code = mapped
	}
	equipment.EquipmentTypeCode = &code
	if equipment.Provenance == nil {
		equipment.Provenance = map[string]string{}
	}
	applyString(&equipment.UnitKind, class.UnitKind, equipment.Provenance, "unit_kind")
	applyString(&equipment.CombinationType, class.CombinationType, equipment.Provenance, "combination_type")
	applyString(&equipment.BodyType, class.BodyType, equipment.Provenance, "body_type")
	applyFloat(&equipment.PayloadKg, class.PayloadKg, equipment.Provenance, "payload_kg")
	applyFloat(&equipment.VolumeM3, class.VolumeM3, equipment.Provenance, "volume_m3")
	applyInt(&equipment.PalletPositions, class.PalletPositions, equipment.Provenance, "pallet_positions")
	applyFloat(&equipment.UsableLinearMeters, class.UsableLinearMeters, equipment.Provenance, "usable_linear_meters")
	applyInt(&equipment.InternalLengthMM, class.InternalLengthMM, equipment.Provenance, "internal_length_mm")
	applyInt(&equipment.InternalWidthMM, class.InternalWidthMM, equipment.Provenance, "internal_width_mm")
	applyInt(&equipment.InternalHeightMM, class.InternalHeightMM, equipment.Provenance, "internal_height_mm")
	applyStrings(&equipment.LoadingAccess, class.LoadingAccess, equipment.Provenance, "loading_access")
	applyStrings(&equipment.UnloadingAccess, class.UnloadingAccess, equipment.Provenance, "unloading_access")
	applyString(&equipment.TemperatureControlMode, class.TemperatureControlMode, equipment.Provenance, "temperature_control_mode")
	applyFloat(&equipment.TemperatureMinC, class.TemperatureMinC, equipment.Provenance, "temperature_min_c")
	applyFloat(&equipment.TemperatureMaxC, class.TemperatureMaxC, equipment.Provenance, "temperature_max_c")
	applyInt(&equipment.TemperatureZoneCount, class.TemperatureZoneCount, equipment.Provenance, "temperature_zone_count")
	applyBool(&equipment.IndependentTemperatureControl, class.IndependentTemperatureControl, equipment.Provenance, "independent_temperature_control")
	applyBool(&equipment.FoodGradeCapability, class.FoodGradeCapability, equipment.Provenance, "food_grade_capability")
	applyBool(&equipment.ADRCapability, class.ADRCapability, equipment.Provenance, "adr_capability")
	return equipment, Reason{}
}

func indexEquipment(items []EquipmentClass) (map[string]EquipmentClass, bool) {
	system := map[string]EquipmentClass{}
	tenant := map[string]EquipmentClass{}
	for _, item := range items {
		item.Code = strings.TrimSpace(item.Code)
		if item.Code == "" {
			return nil, false
		}
		dest := system
		if item.Scope == "TENANT" {
			dest = tenant
		}
		if _, exists := dest[item.Code]; exists {
			return nil, false
		}
		dest[item.Code] = item
	}
	for code, item := range tenant {
		system[code] = item
	}
	return system, true
}

func applyString(dst **string, src *string, provenance map[string]string, name string) {
	if *dst != nil {
		provenance[name] = ProvenanceAssetConfirmed
		return
	}
	if src == nil {
		return
	}
	value := *src
	*dst = &value
	provenance[name] = ProvenanceReferenceDefault
}

func applyFloat(dst **float64, src *float64, provenance map[string]string, name string) {
	if *dst != nil {
		provenance[name] = ProvenanceAssetConfirmed
		return
	}
	if src == nil {
		return
	}
	value := *src
	*dst = &value
	provenance[name] = ProvenanceReferenceDefault
}

func applyInt(dst **int, src *int, provenance map[string]string, name string) {
	if *dst != nil {
		provenance[name] = ProvenanceAssetConfirmed
		return
	}
	if src == nil {
		return
	}
	value := *src
	*dst = &value
	provenance[name] = ProvenanceReferenceDefault
}

func applyBool(dst **bool, src *bool, provenance map[string]string, name string) {
	if *dst != nil {
		provenance[name] = ProvenanceAssetConfirmed
		return
	}
	if src == nil {
		return
	}
	value := *src
	*dst = &value
	provenance[name] = ProvenanceReferenceDefault
}

func rewriteExactAlias(code **string, aliases map[string]string) {
	if code == nil || *code == nil || len(aliases) == 0 {
		return
	}
	canonical, ok := aliases[strings.TrimSpace(**code)]
	if !ok {
		return
	}
	**code = canonical
}

type AliasBuilder struct {
	system  map[string]map[string]string
	tenant  map[string]map[string]string
	invalid bool
}

func NewAliasBuilder() *AliasBuilder {
	return &AliasBuilder{system: map[string]map[string]string{}, tenant: map[string]map[string]string{}}
}

func (b *AliasBuilder) Add(kind, scope, alias, canonical string) {
	kind = strings.TrimSpace(kind)
	alias = strings.TrimSpace(alias)
	canonical = strings.TrimSpace(canonical)
	switch kind {
	case "CARGO_TYPE", "EQUIPMENT_TYPE", "PALLET_TYPE", "PACKAGING_TYPE":
	default:
		b.invalid = true
		return
	}
	if alias == "" || canonical == "" {
		b.invalid = true
		return
	}
	dest := b.system
	if scope == "TENANT" {
		dest = b.tenant
	}
	bucket := dest[kind]
	if bucket == nil {
		bucket = map[string]string{}
		dest[kind] = bucket
	}
	if _, exists := bucket[alias]; exists {
		b.invalid = true
		return
	}
	bucket[alias] = canonical
}

func (b *AliasBuilder) Apply(ctx *Context) {
	if b.invalid {
		ctx.CatalogInvalid = true
		return
	}
	ctx.CargoAliases = mergeAliasKind(b.system["CARGO_TYPE"], b.tenant["CARGO_TYPE"])
	ctx.EquipmentAliases = mergeAliasKind(b.system["EQUIPMENT_TYPE"], b.tenant["EQUIPMENT_TYPE"])
	ctx.PalletAliases = mergeAliasKind(b.system["PALLET_TYPE"], b.tenant["PALLET_TYPE"])
	ctx.PackagingAliases = mergeAliasKind(b.system["PACKAGING_TYPE"], b.tenant["PACKAGING_TYPE"])
}

func mergeAliasKind(system, tenant map[string]string) map[string]string {
	if len(system) == 0 && len(tenant) == 0 {
		return nil
	}
	out := map[string]string{}
	for alias, canonical := range system {
		out[alias] = canonical
	}
	for alias, canonical := range tenant {
		out[alias] = canonical
	}
	return out
}

func applyStrings(dst *[]string, src []string, provenance map[string]string, name string) {
	if len(*dst) > 0 {
		provenance[name] = ProvenanceAssetConfirmed
		return
	}
	if len(src) == 0 {
		return
	}
	*dst = append([]string(nil), src...)
	provenance[name] = ProvenanceReferenceDefault
}

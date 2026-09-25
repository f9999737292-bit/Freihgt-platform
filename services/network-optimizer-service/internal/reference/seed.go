package reference

import (
	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
)

func SeedSystem(store *Store) error {
	cargo := Version{ID: uuid.MustParse("00000000-0000-4000-8000-000000000083"), Kind: "CARGO_TYPE", Scope: ScopeSystem, Version: 1, Status: StatusActive, SourceReference: "SYSTEM_SEED", CreatedBy: "SYSTEM_SEED"}
	equipment := Version{ID: uuid.MustParse("00000000-0000-4000-8000-000000000086"), Kind: "EQUIPMENT_TYPE", Scope: ScopeSystem, Version: 1, Status: StatusActive, SourceReference: "SYSTEM_SEED", CreatedBy: "SYSTEM_SEED"}
	pallet := Version{ID: uuid.MustParse("00000000-0000-4000-8000-000000000087"), Kind: "PALLET_TYPE", Scope: ScopeSystem, Version: 1, Status: StatusActive, SourceReference: "SYSTEM_SEED", CreatedBy: "SYSTEM_SEED"}
	packaging := Version{ID: uuid.MustParse("00000000-0000-4000-8000-000000000088"), Kind: "PACKAGING_TYPE", Scope: ScopeSystem, Version: 1, Status: StatusActive, SourceReference: "SYSTEM_SEED", CreatedBy: "SYSTEM_SEED"}
	for _, version := range []Version{cargo, equipment, pallet, packaging} {
		if err := store.AddVersion(version); err != nil {
			return err
		}
	}
	parent := "FOOD"
	for _, item := range []CargoType{
		{VersionID: cargo.ID, Code: "GENERAL_CARGO", DisplayName: "General cargo"},
		{VersionID: cargo.ID, Code: "FOOD", DisplayName: "Food"},
		{VersionID: cargo.ID, Code: "DAIRY", ParentCode: &parent, DisplayName: "Dairy"},
	} {
		store.cargo = append(store.cargo, item)
	}
	store.named = append(store.named,
		NamedType{VersionID: equipment.ID, Code: "SEMITRAILER_REEFER", DisplayName: "Semitrailer refrigerator", UnitKind: "SEMITRAILER", BodyType: "REFRIGERATOR"},
		NamedType{VersionID: pallet.ID, Code: "EUR", DisplayName: "EUR pallet", LengthMM: 1200, WidthMM: 800},
		NamedType{VersionID: packaging.ID, Code: "PALLETIZED", DisplayName: "Palletized"},
	)
	return store.CreateRuleSet(RuleSet{ID: uuid.MustParse("00000000-0000-4000-8000-000000000091"), Scope: ScopeSystem, Version: 1, Status: StatusActive, SourceReference: "SYSTEM_SEED"})
}

func (s *Store) ListNamed(kind string, viewer *uuid.UUID) []NamedType {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []NamedType
	for _, item := range s.named {
		versionKind := ""
		for _, version := range s.versions {
			if version.ID == item.VersionID {
				versionKind = version.Kind
			}
		}
		if versionKind == kind && s.visible(item.VersionID, viewer) {
			out = append(out, item)
		}
	}
	return out
}

func (s *Store) ListRuleSets(viewer *uuid.UUID) []RuleSet {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []RuleSet
	for _, set := range s.sets {
		if set.Scope == ScopeSystem || (viewer != nil && set.TenantID != nil && *set.TenantID == *viewer) {
			out = append(out, set)
		}
	}
	return out
}

func (s *Store) NextTenantVersion(tenant uuid.UUID) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := 1
	for _, set := range s.sets {
		if set.Scope == ScopeTenant && set.TenantID != nil && *set.TenantID == tenant && set.Version >= next {
			next = set.Version + 1
		}
	}
	return next
}

func (s *Store) EvaluationContext(viewer *uuid.UUID) compat.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := compat.Context{Rules: []compat.Rule{}}
	for _, version := range s.versions {
		if version.Status != StatusActive || version.Scope != ScopeSystem {
			continue
		}
		switch version.Kind {
		case "CARGO_TYPE":
			ctx.CargoCatalogVersion = version.Version
		case "EQUIPMENT_TYPE":
			ctx.EquipmentCatalogVersion = version.Version
		case "PALLET_TYPE":
			ctx.PalletCatalogVersion = version.Version
		case "PACKAGING_TYPE":
			ctx.PackagingCatalogVersion = version.Version
		}
	}
	for _, set := range s.sets {
		if set.Status != StatusActive {
			continue
		}
		if set.Scope != ScopeSystem && (viewer == nil || set.TenantID == nil || *set.TenantID != *viewer) {
			continue
		}
		ref := compat.RuleSetRef{ID: set.ID.String(), Scope: set.Scope, Version: set.Version}
		if set.TenantID != nil {
			tenantID := set.TenantID.String()
			ref.TenantID = &tenantID
		}
		ctx.RuleSets = append(ctx.RuleSets, ref)
		for _, rule := range set.Rules {
			item := compat.Rule{
				RuleCode: rule.RuleCode, RuleKind: rule.RuleKind, Layer: rule.Layer,
				LeftSelectorType: rule.LeftSelectorType, LeftSelectorValue: rule.LeftSelectorValue,
				RightSelectorType: rule.RightSelectorType, RightSelectorValue: rule.RightSelectorValue,
				Decision: rule.Decision, ReasonCode: rule.ReasonCode, Severity: rule.Severity,
				RequiredSeparation: rule.RequiredSeparation, SourceReference: rule.SourceReference,
				Priority: rule.Priority, RuleSetID: set.ID.String(), RuleSetScope: set.Scope, RuleSetVersion: set.Version,
			}
			if ref.TenantID != nil {
				item.RuleSetTenantID = ref.TenantID
			}
			if item.Severity == "" {
				item.Severity = compat.SeverityHard
			}
			ctx.Rules = append(ctx.Rules, item)
		}
	}
	for _, version := range s.versions {
		if !s.activeForViewer(version, viewer) {
			continue
		}
		ref := compat.CatalogVersionRef{ID: version.ID.String(), CatalogKind: version.Kind, Scope: version.Scope, Version: version.Version}
		if version.TenantID != nil {
			tenantID := version.TenantID.String()
			ref.TenantID = &tenantID
		}
		ctx.CatalogRefs = append(ctx.CatalogRefs, ref)
	}
	ctx.CargoClasses = s.activeCargoClasses(viewer)
	ctx.EquipmentClasses = s.activeEquipmentClasses(viewer)
	s.activeAliases(viewer).Apply(&ctx)
	return ctx
}

func (s *Store) activeCargoClasses(viewer *uuid.UUID) []compat.CargoClass {
	var out []compat.CargoClass
	for _, item := range s.cargo {
		version, ok := s.versionOf(item.VersionID)
		if !ok || !s.activeForViewer(version, viewer) {
			continue
		}
		out = append(out, compat.CargoClass{Code: item.Code, Parent: item.ParentCode, Tags: append([]string(nil), item.Tags...), Scope: version.Scope})
	}
	return out
}

func (s *Store) activeEquipmentClasses(viewer *uuid.UUID) []compat.EquipmentClass {
	var out []compat.EquipmentClass
	for _, item := range s.named {
		version, ok := s.versionOf(item.VersionID)
		if !ok || version.Kind != "EQUIPMENT_TYPE" || !s.activeForViewer(version, viewer) {
			continue
		}
		class := compat.EquipmentClass{Code: item.Code, Scope: version.Scope}
		if item.UnitKind != "" {
			value := item.UnitKind
			class.UnitKind = &value
		}
		if item.BodyType != "" {
			value := item.BodyType
			class.BodyType = &value
		}
		out = append(out, class)
	}
	return out
}

func (s *Store) activeAliases(viewer *uuid.UUID) *compat.AliasBuilder {
	builder := compat.NewAliasBuilder()
	for _, item := range s.aliases {
		version, ok := s.versionOf(item.VersionID)
		if !ok || !s.activeForViewer(version, viewer) {
			continue
		}
		builder.Add(version.Kind, version.Scope, item.AliasCode, item.CanonicalCode)
	}
	return builder
}

func (s *Store) versionOf(id uuid.UUID) (Version, bool) {
	for _, version := range s.versions {
		if version.ID == id {
			return version, true
		}
	}
	return Version{}, false
}

func (s *Store) activeForViewer(version Version, viewer *uuid.UUID) bool {
	if version.Status != StatusActive {
		return false
	}
	if version.Scope == ScopeSystem {
		return true
	}
	return viewer != nil && version.TenantID != nil && *version.TenantID == *viewer
}

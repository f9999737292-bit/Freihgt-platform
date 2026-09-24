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
		for _, rule := range set.Rules {
			ctx.Rules = append(ctx.Rules, compat.Rule{
				RuleCode: rule.RuleCode, RuleKind: rule.RuleKind, Layer: rule.Layer,
				LeftSelectorType: rule.LeftSelectorType, LeftSelectorValue: rule.LeftSelectorValue,
				RightSelectorType: rule.RightSelectorType, RightSelectorValue: rule.RightSelectorValue,
				Decision: rule.Decision, ReasonCode: rule.ReasonCode, SourceReference: rule.SourceReference,
				Priority: rule.Priority, RuleSetVersion: set.Version,
			})
		}
	}
	return ctx
}

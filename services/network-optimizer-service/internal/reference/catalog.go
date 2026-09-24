package reference

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
)

type Catalog interface {
	ListCargo(ctx context.Context, viewer uuid.UUID, limit, offset int) ([]CargoType, error)
	ListNamed(ctx context.Context, kind string, viewer uuid.UUID, limit, offset int) ([]NamedType, error)
	ListRuleSets(ctx context.Context, viewer uuid.UUID) ([]RuleSet, error)
	CreateRuleSet(ctx context.Context, actor, tenant uuid.UUID) (RuleSet, error)
	AddRule(ctx context.Context, actor, tenant, setID uuid.UUID, rule Rule) error
	RemoveRule(ctx context.Context, actor, tenant, setID uuid.UUID, ruleCode string) error
	ActivateRuleSet(ctx context.Context, actor, tenant, setID uuid.UUID) error
	RetireRuleSet(ctx context.Context, actor, tenant, setID uuid.UUID) error
	Evaluation(ctx context.Context, tenant uuid.UUID) (compat.Context, error)
}

type MemoryCatalog struct{ store *Store }

func NewMemoryCatalog() *MemoryCatalog {
	store := NewStore()
	_ = SeedSystem(store)
	return &MemoryCatalog{store: store}
}

func (m *MemoryCatalog) Store() *Store { return m.store }

func (m *MemoryCatalog) ListCargo(_ context.Context, viewer uuid.UUID, limit, offset int) ([]CargoType, error) {
	return page(m.store.ListCargo(&viewer), limit, offset), nil
}

func (m *MemoryCatalog) ListNamed(_ context.Context, kind string, viewer uuid.UUID, limit, offset int) ([]NamedType, error) {
	return page(m.store.ListNamed(kind, &viewer), limit, offset), nil
}

func (m *MemoryCatalog) ListRuleSets(_ context.Context, viewer uuid.UUID) ([]RuleSet, error) {
	return m.store.ListRuleSets(&viewer), nil
}

func (m *MemoryCatalog) CreateRuleSet(_ context.Context, actor, tenant uuid.UUID) (RuleSet, error) {
	set := RuleSet{ID: uuid.New(), Scope: ScopeTenant, TenantID: &tenant, Version: m.store.NextTenantVersion(tenant), Status: StatusDraft, SourceReference: "TENANT"}
	if err := m.store.CreateRuleSet(set); err != nil {
		return RuleSet{}, err
	}
	_ = actor
	return set, nil
}

func (m *MemoryCatalog) AddRule(_ context.Context, _, tenant, setID uuid.UUID, rule Rule) error {
	return m.store.AddRule(&tenant, setID, rule)
}

func (m *MemoryCatalog) RemoveRule(_ context.Context, _, tenant, setID uuid.UUID, ruleCode string) error {
	return m.store.RemoveDraftRule(&tenant, setID, ruleCode)
}

func (m *MemoryCatalog) ActivateRuleSet(_ context.Context, _, tenant, setID uuid.UUID) error {
	return m.store.ActivateRuleSet(&tenant, setID)
}

func (m *MemoryCatalog) RetireRuleSet(_ context.Context, _, tenant, setID uuid.UUID) error {
	return m.store.RetireRuleSet(&tenant, setID)
}

func (m *MemoryCatalog) Evaluation(_ context.Context, tenant uuid.UUID) (compat.Context, error) {
	return m.store.EvaluationContext(&tenant), nil
}

func page[T any](items []T, limit, offset int) []T {
	if offset > len(items) {
		return []T{}
	}
	items = items[offset:]
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	return items
}

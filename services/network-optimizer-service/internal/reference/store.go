package reference

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
)

const (
	StatusDraft   = "DRAFT"
	StatusActive  = "ACTIVE"
	StatusRetired = "RETIRED"
	ScopeSystem   = "SYSTEM"
	ScopeTenant   = "TENANT"
)

type Version struct {
	ID              uuid.UUID
	Kind            string
	Scope           string
	TenantID        *uuid.UUID
	Version         int
	Status          string
	SourceReference string
	CreatedBy       string
}

type CargoType struct {
	VersionID   uuid.UUID `json:"version_id"`
	Code        string    `json:"code"`
	ParentCode  *string   `json:"parent_code,omitempty"`
	DisplayName string    `json:"display_name"`
	Tags        []string  `json:"tags,omitempty"`
}

type NamedType struct {
	VersionID   uuid.UUID `json:"version_id"`
	Code        string    `json:"code"`
	DisplayName string    `json:"display_name"`
	LengthMM    int       `json:"length_mm,omitempty"`
	WidthMM     int       `json:"width_mm,omitempty"`
	UnitKind    string    `json:"unit_kind,omitempty"`
	BodyType    string    `json:"body_type,omitempty"`
}

type Alias struct {
	VersionID     uuid.UUID
	AliasCode     string
	CanonicalCode string
}

type RuleSet struct {
	ID              uuid.UUID  `json:"id"`
	Scope           string     `json:"scope"`
	TenantID        *uuid.UUID `json:"tenant_id,omitempty"`
	Version         int        `json:"version"`
	Status          string     `json:"status"`
	SourceReference string     `json:"source_reference"`
	Rules           []Rule     `json:"rules,omitempty"`
}

type Rule struct {
	RuleCode           string  `json:"rule_code"`
	RuleKind           string  `json:"rule_kind"`
	Layer              string  `json:"layer"`
	LeftSelectorType   string  `json:"left_selector_type"`
	LeftSelectorValue  string  `json:"left_selector_value"`
	RightSelectorType  string  `json:"right_selector_type"`
	RightSelectorValue string  `json:"right_selector_value"`
	Decision           string  `json:"decision"`
	ReasonCode         string  `json:"reason_code"`
	SourceReference    *string `json:"source_reference,omitempty"`
	Priority           int     `json:"priority"`
}

type Audit struct {
	Action   string
	Scope    string
	TenantID *uuid.UUID
}

type Store struct {
	mu       sync.Mutex
	versions []Version
	cargo    []CargoType
	named    []NamedType
	aliases  []Alias
	sets     []RuleSet
	audits   []Audit
}

func NewStore() *Store { return &Store{} }

func (s *Store) AddVersion(v Version) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v.Version < 1 || v.Kind == "" || (v.Scope != ScopeSystem && v.Scope != ScopeTenant) {
		return fmt.Errorf("invalid catalog version")
	}
	if v.Scope == ScopeTenant && (v.TenantID == nil || *v.TenantID == uuid.Nil) {
		return fmt.Errorf("tenant scope requires tenant_id")
	}
	if v.Scope == ScopeSystem {
		v.TenantID = nil
	}
	if v.Status == "" {
		v.Status = StatusDraft
	}
	s.versions = append(s.versions, v)
	return nil
}

func (s *Store) Activate(id uuid.UUID, actor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i := range s.versions {
		if s.versions[i].ID == id {
			idx = i
		}
	}
	if idx < 0 {
		return fmt.Errorf("catalog version not found")
	}
	next := s.versions[idx]
	for i := range s.versions {
		if s.versions[i].Status == StatusActive && sameStream(s.versions[i], next) {
			s.versions[i].Status = StatusRetired
		}
	}
	next.Status = StatusActive
	if next.CreatedBy == "" {
		next.CreatedBy = actor
	}
	s.versions[idx] = next
	return nil
}

func (s *Store) AddCargoType(actorTenant *uuid.UUID, item CargoType) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	version, err := s.draft(item.VersionID, actorTenant)
	if err != nil {
		return err
	}
	if version.Kind != "CARGO_TYPE" {
		return fmt.Errorf("wrong catalog kind")
	}
	s.cargo = append(s.cargo, item)
	return nil
}

func (s *Store) AddNamed(actorTenant *uuid.UUID, kind string, item NamedType) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	version, err := s.draft(item.VersionID, actorTenant)
	if err != nil {
		return err
	}
	if version.Kind != kind {
		return fmt.Errorf("wrong catalog kind")
	}
	if kind == "PALLET_TYPE" && (item.LengthMM <= 0 || item.WidthMM <= 0) {
		return fmt.Errorf("pallet dimensions must be positive")
	}
	s.named = append(s.named, item)
	return nil
}

func (s *Store) AddAlias(actorTenant *uuid.UUID, item Alias) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.draft(item.VersionID, actorTenant); err != nil {
		return err
	}
	s.aliases = append(s.aliases, item)
	return nil
}

func (s *Store) ResolveAlias(versionID uuid.UUID, alias string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.aliases {
		if item.VersionID == versionID && item.AliasCode == alias {
			return item.CanonicalCode, true
		}
	}
	return "", false
}

func (s *Store) ListCargo(viewer *uuid.UUID) []CargoType {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []CargoType
	for _, item := range s.cargo {
		if s.visible(item.VersionID, viewer) {
			out = append(out, item)
		}
	}
	return out
}

func (s *Store) CreateRuleSet(set RuleSet) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if set.Version < 1 || (set.Scope != ScopeSystem && set.Scope != ScopeTenant) {
		return fmt.Errorf("invalid rule set")
	}
	if set.Scope == ScopeTenant && (set.TenantID == nil || *set.TenantID == uuid.Nil) {
		return fmt.Errorf("tenant scope requires tenant_id")
	}
	if set.Status == "" {
		set.Status = StatusDraft
	}
	s.sets = append(s.sets, set)
	s.audits = append(s.audits, Audit{Action: "rule_set_created", Scope: set.Scope, TenantID: set.TenantID})
	return nil
}

func (s *Store) AddRule(actorTenant *uuid.UUID, setID uuid.UUID, rule Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, err := s.draftSet(setID, actorTenant)
	if err != nil {
		return err
	}
	if err := ValidateRule(rule); err != nil {
		return err
	}
	if set.Scope == ScopeTenant && rule.Layer == "REGULATORY" {
		return fmt.Errorf("tenant rule cannot be regulatory")
	}
	for i := range s.sets {
		if s.sets[i].ID == setID {
			for _, existing := range s.sets[i].Rules {
				if existing.RuleCode == rule.RuleCode {
					return fmt.Errorf("duplicate rule_code")
				}
			}
			s.sets[i].Rules = append(s.sets[i].Rules, rule)
			s.audits = append(s.audits, Audit{Action: "rule_added", Scope: set.Scope, TenantID: set.TenantID})
		}
	}
	return nil
}

func (s *Store) RemoveDraftRule(actorTenant *uuid.UUID, setID uuid.UUID, ruleCode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, err := s.draftSet(setID, actorTenant)
	if err != nil {
		return err
	}
	for i := range s.sets {
		if s.sets[i].ID != setID {
			continue
		}
		next := s.sets[i].Rules[:0]
		found := false
		for _, rule := range s.sets[i].Rules {
			if rule.RuleCode == ruleCode {
				found = true
				continue
			}
			next = append(next, rule)
		}
		if !found {
			return fmt.Errorf("rule not found")
		}
		s.sets[i].Rules = next
		s.audits = append(s.audits, Audit{Action: "rule_removed", Scope: set.Scope, TenantID: set.TenantID})
	}
	return nil
}

func (s *Store) ActivateRuleSet(actorTenant *uuid.UUID, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i := range s.sets {
		if s.sets[i].ID == id {
			idx = i
		}
	}
	if idx < 0 || !owns(s.sets[idx].Scope, s.sets[idx].TenantID, actorTenant) {
		return fmt.Errorf("rule set not found")
	}
	for i := range s.sets {
		if s.sets[i].Status == StatusActive && s.sets[i].Scope == s.sets[idx].Scope && sameTenant(s.sets[i].TenantID, s.sets[idx].TenantID) {
			s.sets[i].Status = StatusRetired
		}
	}
	s.sets[idx].Status = StatusActive
	s.audits = append(s.audits, Audit{Action: "rule_set_activated", Scope: s.sets[idx].Scope, TenantID: s.sets[idx].TenantID})
	return nil
}

func (s *Store) RetireRuleSet(actorTenant *uuid.UUID, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.sets {
		if s.sets[i].ID != id || !owns(s.sets[i].Scope, s.sets[i].TenantID, actorTenant) {
			continue
		}
		if s.sets[i].Status != StatusActive {
			return fmt.Errorf("only an active rule set can be retired")
		}
		s.sets[i].Status = StatusRetired
		s.audits = append(s.audits, Audit{Action: "rule_set_retired", Scope: s.sets[i].Scope, TenantID: s.sets[i].TenantID})
		return nil
	}
	return fmt.Errorf("rule set not found")
}

func (s *Store) Audits() []Audit {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Audit(nil), s.audits...)
}

func (s *Store) GetRuleSet(actorTenant *uuid.UUID, id uuid.UUID) (RuleSet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, set := range s.sets {
		if set.ID == id {
			if set.Scope == ScopeSystem || owns(set.Scope, set.TenantID, actorTenant) {
				return set, nil
			}
			return RuleSet{}, fmt.Errorf("rule set not found")
		}
	}
	return RuleSet{}, fmt.Errorf("rule set not found")
}

func (s *Store) draft(id uuid.UUID, actor *uuid.UUID) (Version, error) {
	for _, version := range s.versions {
		if version.ID != id {
			continue
		}
		if version.Status != StatusDraft {
			return Version{}, fmt.Errorf("active catalog version is immutable")
		}
		if !owns(version.Scope, version.TenantID, actor) && version.Scope == ScopeTenant {
			return Version{}, fmt.Errorf("catalog version not found")
		}
		return version, nil
	}
	return Version{}, fmt.Errorf("catalog version not found")
}

func (s *Store) draftSet(id uuid.UUID, actor *uuid.UUID) (RuleSet, error) {
	for _, set := range s.sets {
		if set.ID != id {
			continue
		}
		if !owns(set.Scope, set.TenantID, actor) && set.Scope == ScopeTenant {
			return RuleSet{}, fmt.Errorf("rule set not found")
		}
		if set.Status != StatusDraft {
			return RuleSet{}, fmt.Errorf("active rule set is immutable")
		}
		return set, nil
	}
	return RuleSet{}, fmt.Errorf("rule set not found")
}

func (s *Store) visible(versionID uuid.UUID, viewer *uuid.UUID) bool {
	for _, version := range s.versions {
		if version.ID != versionID || version.Status != StatusActive {
			continue
		}
		if version.Scope == ScopeSystem {
			return true
		}
		return viewer != nil && version.TenantID != nil && *version.TenantID == *viewer
	}
	return false
}

func owns(scope string, owner, actor *uuid.UUID) bool {
	if scope == ScopeSystem {
		return actor == nil
	}
	return owner != nil && actor != nil && *owner == *actor
}

func sameStream(a, b Version) bool {
	if a.Kind != b.Kind || a.Scope != b.Scope {
		return false
	}
	return sameTenant(a.TenantID, b.TenantID)
}

func sameTenant(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

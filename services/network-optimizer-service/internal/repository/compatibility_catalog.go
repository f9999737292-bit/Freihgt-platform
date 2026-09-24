package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/reference"
)

func (p *Postgres) ListCargo(ctx context.Context, viewer uuid.UUID, limit, offset int) ([]reference.CargoType, error) {
	limit, offset = bounds(limit, offset)
	rows, err := p.pool.Query(ctx, `
		SELECT c.version_id, c.code, c.parent_code, c.display_name, c.tags
		FROM network_optimizer.cargo_type_catalog c
		JOIN network_optimizer.reference_catalog_versions v ON v.id = c.version_id
		WHERE v.status = 'ACTIVE' AND (v.scope = 'SYSTEM' OR v.tenant_id = $1)
		ORDER BY c.code
		LIMIT $2 OFFSET $3`, viewer, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []reference.CargoType
	for rows.Next() {
		var item reference.CargoType
		if err := rows.Scan(&item.VersionID, &item.Code, &item.ParentCode, &item.DisplayName, &item.Tags); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (p *Postgres) ListNamed(ctx context.Context, kind string, viewer uuid.UUID, limit, offset int) ([]reference.NamedType, error) {
	limit, offset = bounds(limit, offset)
	var query string
	switch kind {
	case "EQUIPMENT_TYPE":
		query = `
			SELECT item.version_id, item.code, item.code, 0, 0, item.unit_kind, item.body_type
			FROM network_optimizer.equipment_type_catalog item
			JOIN network_optimizer.reference_catalog_versions v ON v.id = item.version_id
			WHERE v.status = 'ACTIVE' AND (v.scope = 'SYSTEM' OR v.tenant_id = $1)
			ORDER BY item.code LIMIT $2 OFFSET $3`
	case "PALLET_TYPE":
		query = `
			SELECT item.version_id, item.code, item.display_name, item.length_mm, item.width_mm, '', ''
			FROM network_optimizer.pallet_type_catalog item
			JOIN network_optimizer.reference_catalog_versions v ON v.id = item.version_id
			WHERE v.status = 'ACTIVE' AND (v.scope = 'SYSTEM' OR v.tenant_id = $1)
			ORDER BY item.code LIMIT $2 OFFSET $3`
	case "PACKAGING_TYPE":
		query = `
			SELECT item.version_id, item.code, item.display_name, 0, 0, '', ''
			FROM network_optimizer.packaging_type_catalog item
			JOIN network_optimizer.reference_catalog_versions v ON v.id = item.version_id
			WHERE v.status = 'ACTIVE' AND (v.scope = 'SYSTEM' OR v.tenant_id = $1)
			ORDER BY item.code LIMIT $2 OFFSET $3`
	default:
		return nil, fmt.Errorf("unknown catalog kind")
	}
	rows, err := p.pool.Query(ctx, query, viewer, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []reference.NamedType
	for rows.Next() {
		var item reference.NamedType
		if err := rows.Scan(&item.VersionID, &item.Code, &item.DisplayName, &item.LengthMM, &item.WidthMM, &item.UnitKind, &item.BodyType); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (p *Postgres) ListRuleSets(ctx context.Context, viewer uuid.UUID) ([]reference.RuleSet, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, scope, tenant_id, version, status, source_reference
		FROM network_optimizer.compatibility_rule_sets
		WHERE scope = 'SYSTEM' OR tenant_id = $1
		ORDER BY scope, version`, viewer)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []reference.RuleSet
	for rows.Next() {
		var set reference.RuleSet
		if err := rows.Scan(&set.ID, &set.Scope, &set.TenantID, &set.Version, &set.Status, &set.SourceReference); err != nil {
			return nil, err
		}
		out = append(out, set)
	}
	return out, rows.Err()
}

func (p *Postgres) CreateRuleSet(ctx context.Context, actor, tenant uuid.UUID) (reference.RuleSet, error) {
	var version int
	if err := p.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM network_optimizer.compatibility_rule_sets
		WHERE scope = 'TENANT' AND tenant_id = $1`, tenant).Scan(&version); err != nil {
		return reference.RuleSet{}, err
	}
	set := reference.RuleSet{ID: uuid.New(), Scope: reference.ScopeTenant, TenantID: &tenant, Version: version, Status: reference.StatusDraft, SourceReference: "TENANT"}
	_, err := p.pool.Exec(ctx, `
		INSERT INTO network_optimizer.compatibility_rule_sets
		(id, scope, tenant_id, version, status, source_reference, created_by)
		VALUES ($1, 'TENANT', $2, $3, 'DRAFT', 'TENANT', $4)`, set.ID, tenant, version, actor)
	if err != nil {
		return reference.RuleSet{}, err
	}
	p.audit(ctx, actor, tenant, set.ID, "rule_set_created", "", reference.StatusDraft)
	return set, nil
}

func (p *Postgres) AddRule(ctx context.Context, actor, tenant, setID uuid.UUID, rule reference.Rule) error {
	if err := reference.ValidateRule(rule); err != nil {
		return err
	}
	if err := p.requireDraft(ctx, tenant, setID); err != nil {
		return err
	}
	if rule.Layer == "REGULATORY" {
		return fmt.Errorf("tenant rule cannot be regulatory")
	}
	_, err := p.pool.Exec(ctx, `
		INSERT INTO network_optimizer.compatibility_rules (
			id, rule_set_id, rule_code, rule_kind, layer, left_selector_type, left_selector_value,
			right_selector_type, right_selector_value, decision, reason_code, source_reference, priority
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		uuid.New(), setID, rule.RuleCode, rule.RuleKind, rule.Layer, rule.LeftSelectorType, rule.LeftSelectorValue,
		rule.RightSelectorType, rule.RightSelectorValue, rule.Decision, rule.ReasonCode, rule.SourceReference, rule.Priority)
	if err != nil {
		return err
	}
	p.audit(ctx, actor, tenant, setID, "rule_added", reference.StatusDraft, reference.StatusDraft)
	return nil
}

func (p *Postgres) RemoveRule(ctx context.Context, actor, tenant, setID uuid.UUID, ruleCode string) error {
	if err := p.requireDraft(ctx, tenant, setID); err != nil {
		return err
	}
	tag, err := p.pool.Exec(ctx, `
		DELETE FROM network_optimizer.compatibility_rules
		WHERE rule_set_id = $1 AND rule_code = $2`, setID, ruleCode)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("rule set not found")
	}
	p.audit(ctx, actor, tenant, setID, "rule_removed", reference.StatusDraft, reference.StatusDraft)
	return nil
}

func (p *Postgres) ActivateRuleSet(ctx context.Context, actor, tenant, setID uuid.UUID) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var scope string
	var owner *uuid.UUID
	var status string
	if err := tx.QueryRow(ctx, `SELECT scope, tenant_id, status FROM network_optimizer.compatibility_rule_sets WHERE id = $1`, setID).Scan(&scope, &owner, &status); err != nil {
		return fmt.Errorf("rule set not found")
	}
	if scope != reference.ScopeTenant || owner == nil || *owner != tenant || status != reference.StatusDraft {
		return fmt.Errorf("rule set not found")
	}
	if _, err := tx.Exec(ctx, `
		UPDATE network_optimizer.compatibility_rule_sets
		SET status = 'RETIRED'
		WHERE scope = 'TENANT' AND tenant_id = $1 AND status = 'ACTIVE'`, tenant); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE network_optimizer.compatibility_rule_sets
		SET status = 'ACTIVE', activated_at = $2, activated_by = $3
		WHERE id = $1`, setID, time.Now().UTC(), actor); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	p.audit(ctx, actor, tenant, setID, "rule_set_activated", reference.StatusDraft, reference.StatusActive)
	return nil
}

func (p *Postgres) RetireRuleSet(ctx context.Context, actor, tenant, setID uuid.UUID) error {
	tag, err := p.pool.Exec(ctx, `
		UPDATE network_optimizer.compatibility_rule_sets
		SET status = 'RETIRED'
		WHERE id = $1 AND scope = 'TENANT' AND tenant_id = $2 AND status = 'ACTIVE'`, setID, tenant)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("rule set not found")
	}
	p.audit(ctx, actor, tenant, setID, "rule_set_retired", reference.StatusActive, reference.StatusRetired)
	return nil
}

func (p *Postgres) Evaluation(ctx context.Context, tenant uuid.UUID) (compat.Context, error) {
	out := compat.Context{Rules: []compat.Rule{}}
	rows, err := p.pool.Query(ctx, `
		SELECT catalog_kind, version FROM network_optimizer.reference_catalog_versions
		WHERE status = 'ACTIVE' AND scope = 'SYSTEM'`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var kind string
		var version int
		if err := rows.Scan(&kind, &version); err != nil {
			rows.Close()
			return out, err
		}
		switch kind {
		case "CARGO_TYPE":
			out.CargoCatalogVersion = version
		case "EQUIPMENT_TYPE":
			out.EquipmentCatalogVersion = version
		case "PALLET_TYPE":
			out.PalletCatalogVersion = version
		case "PACKAGING_TYPE":
			out.PackagingCatalogVersion = version
		}
	}
	rows.Close()
	rules, err := p.pool.Query(ctx, `
		SELECT r.rule_code, r.rule_kind, r.layer, r.left_selector_type, r.left_selector_value,
			r.right_selector_type, r.right_selector_value, r.decision, r.reason_code, r.source_reference,
			r.priority, s.version
		FROM network_optimizer.compatibility_rules r
		JOIN network_optimizer.compatibility_rule_sets s ON s.id = r.rule_set_id
		WHERE s.status = 'ACTIVE' AND (s.scope = 'SYSTEM' OR s.tenant_id = $1)
		ORDER BY r.priority DESC, r.rule_code`, tenant)
	if err != nil {
		return out, err
	}
	defer rules.Close()
	for rules.Next() {
		var rule compat.Rule
		if err := rules.Scan(&rule.RuleCode, &rule.RuleKind, &rule.Layer, &rule.LeftSelectorType, &rule.LeftSelectorValue, &rule.RightSelectorType, &rule.RightSelectorValue, &rule.Decision, &rule.ReasonCode, &rule.SourceReference, &rule.Priority, &rule.RuleSetVersion); err != nil {
			return out, err
		}
		out.Rules = append(out.Rules, rule)
	}
	if err := rules.Err(); err != nil {
		return out, err
	}
	if err := p.loadCargoClasses(ctx, tenant, &out); err != nil {
		return out, err
	}
	if err := p.loadEquipmentClasses(ctx, tenant, &out); err != nil {
		return out, err
	}
	if err := p.loadAliases(ctx, tenant, &out); err != nil {
		return out, err
	}
	if err := p.loadEquivalences(ctx, tenant, &out); err != nil {
		return out, err
	}
	return out, nil
}

func (p *Postgres) loadCargoClasses(ctx context.Context, tenant uuid.UUID, out *compat.Context) error {
	rows, err := p.pool.Query(ctx, `
		SELECT c.code, c.parent_code, c.tags, v.scope
		FROM network_optimizer.cargo_type_catalog c
		JOIN network_optimizer.reference_catalog_versions v ON v.id = c.version_id
		WHERE c.active = true AND v.status = 'ACTIVE' AND v.catalog_kind = 'CARGO_TYPE'
			AND (v.scope = 'SYSTEM' OR v.tenant_id = $1)`, tenant)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item compat.CargoClass
		if err := rows.Scan(&item.Code, &item.Parent, &item.Tags, &item.Scope); err != nil {
			return err
		}
		out.CargoClasses = append(out.CargoClasses, item)
	}
	return rows.Err()
}

func (p *Postgres) loadEquipmentClasses(ctx context.Context, tenant uuid.UUID, out *compat.Context) error {
	rows, err := p.pool.Query(ctx, `
		SELECT code, v.scope, unit_kind, combination_type, body_type,
			nominal_payload_kg, nominal_volume_m3, pallet_positions, usable_linear_meters,
			internal_length_mm, internal_width_mm, internal_height_mm,
			loading_access, unloading_access, temperature_control_mode, temperature_min_c, temperature_max_c,
			temperature_zone_count, independent_temperature_control, food_grade_capability, adr_capability
		FROM network_optimizer.equipment_type_catalog item
		JOIN network_optimizer.reference_catalog_versions v ON v.id = item.version_id
		WHERE item.active = true AND v.status = 'ACTIVE' AND v.catalog_kind = 'EQUIPMENT_TYPE'
			AND (v.scope = 'SYSTEM' OR v.tenant_id = $1)`, tenant)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item compat.EquipmentClass
		var unit, body string
		if err := rows.Scan(
			&item.Code, &item.Scope, &unit, &item.CombinationType, &body,
			&item.PayloadKg, &item.VolumeM3, &item.PalletPositions, &item.UsableLinearMeters,
			&item.InternalLengthMM, &item.InternalWidthMM, &item.InternalHeightMM,
			&item.LoadingAccess, &item.UnloadingAccess, &item.TemperatureControlMode, &item.TemperatureMinC, &item.TemperatureMaxC,
			&item.TemperatureZoneCount, &item.IndependentTemperatureControl, &item.FoodGradeCapability, &item.ADRCapability,
		); err != nil {
			return err
		}
		item.UnitKind = &unit
		item.BodyType = &body
		out.EquipmentClasses = append(out.EquipmentClasses, item)
	}
	return rows.Err()
}

func (p *Postgres) loadAliases(ctx context.Context, tenant uuid.UUID, out *compat.Context) error {
	rows, err := p.pool.Query(ctx, `
		SELECT a.alias_code, a.canonical_code, v.scope
		FROM network_optimizer.catalog_aliases a
		JOIN network_optimizer.reference_catalog_versions v ON v.id = a.version_id
		WHERE v.status = 'ACTIVE' AND (v.scope = 'SYSTEM' OR v.tenant_id = $1)`, tenant)
	if err != nil {
		return err
	}
	defer rows.Close()
	system := map[string]string{}
	overlay := map[string]string{}
	seenSystem := map[string]int{}
	seenTenant := map[string]int{}
	for rows.Next() {
		var alias, canonical, scope string
		if err := rows.Scan(&alias, &canonical, &scope); err != nil {
			return err
		}
		if scope == reference.ScopeTenant {
			seenTenant[alias]++
			overlay[alias] = canonical
			continue
		}
		seenSystem[alias]++
		system[alias] = canonical
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for alias, count := range seenSystem {
		if count > 1 {
			out.CatalogInvalid = true
			return nil
		}
		_ = alias
	}
	for alias, count := range seenTenant {
		if count > 1 {
			out.CatalogInvalid = true
			return nil
		}
		_ = alias
	}
	for alias, canonical := range overlay {
		system[alias] = canonical
	}
	if len(system) > 0 {
		out.Aliases = system
	}
	return nil
}

func (p *Postgres) loadEquivalences(ctx context.Context, tenant uuid.UUID, out *compat.Context) error {
	rows, err := p.pool.Query(ctx, `
		SELECT e.from_code, e.basis_code, e.positions_each, v.scope
		FROM network_optimizer.pallet_equivalences e
		JOIN network_optimizer.reference_catalog_versions v ON v.id = e.version_id
		WHERE v.status = 'ACTIVE' AND v.catalog_kind = 'PALLET_TYPE'
			AND (v.scope = 'SYSTEM' OR v.tenant_id = $1)`, tenant)
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		item  compat.Equivalence
		scope string
	}
	var loaded []row
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.item.FromCode, &item.item.BasisCode, &item.item.PositionsEach, &item.scope); err != nil {
			return err
		}
		loaded = append(loaded, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	system := map[string]compat.Equivalence{}
	tenantRows := map[string]compat.Equivalence{}
	for _, item := range loaded {
		key := item.item.FromCode + "\x00" + item.item.BasisCode
		if item.scope == reference.ScopeTenant {
			if _, exists := tenantRows[key]; exists {
				out.Equivalences = nil
				return nil
			}
			tenantRows[key] = item.item
			continue
		}
		if _, exists := system[key]; exists {
			out.Equivalences = nil
			return nil
		}
		system[key] = item.item
	}
	for key, item := range tenantRows {
		system[key] = item
	}
	for _, item := range system {
		out.Equivalences = append(out.Equivalences, item)
	}
	return nil
}

func (p *Postgres) requireDraft(ctx context.Context, tenant, setID uuid.UUID) error {
	var status string
	err := p.pool.QueryRow(ctx, `
		SELECT status FROM network_optimizer.compatibility_rule_sets
		WHERE id = $1 AND scope = 'TENANT' AND tenant_id = $2`, setID, tenant).Scan(&status)
	if err != nil || status != reference.StatusDraft {
		return fmt.Errorf("rule set not found")
	}
	return nil
}

func (p *Postgres) audit(ctx context.Context, actor, tenant, aggregate uuid.UUID, action, from, to string) {
	_, _ = p.pool.Exec(ctx, `
		INSERT INTO network_optimizer.audit_events
		(id, actor_user_id, tenant_id, aggregate_type, aggregate_id, action, from_status, to_status, occurred_at)
		VALUES ($1,$2,$3,'compatibility_rule_set',$4,$5,$6,$7,$8)`,
		uuid.New(), actor, tenant, aggregate, action, from, to, time.Now().UTC())
}

func bounds(limit, offset int) (int, int) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

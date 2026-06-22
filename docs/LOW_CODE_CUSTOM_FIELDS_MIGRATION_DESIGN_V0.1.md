# Low-code Custom Fields Migration Design v0.1

Date: 2026-06-22  
Project: `D:\Projects\freight-platform`  
Status: **Migration design only — no SQL files created**  
Related:

- `docs/LOW_CODE_CUSTOM_FIELDS_TECHNICAL_DESIGN_V0.1.md`
- `docs/LOW_CODE_MVP_SCOPE_V0.1.md`
- `docs/LOW_CODE_DATA_MODEL_DESIGN_V0.1.md`

Git baseline: `b6e5fef`

---

## Summary

This document defines the **future PostgreSQL migration** for Low-code Custom Fields MVP v0.1.

Important:

- This is **SQL/migration design**, not a real migration.
- Migration files will be created **only after explicit approval** of this design.
- New schema: **`lowcode`**
- **Core domain tables are not altered** (`core`, `transport`, `rfx`, `documents`, `billing` unchanged).

MVP tables (10):

| Table | Purpose |
| ----- | ------- |
| `low_code_configurations` | Central config registry |
| `form_templates` | Entity form layouts |
| `form_sections` | Section grouping |
| `form_fields` | Field definitions |
| `custom_field_values` | Runtime values (EAV/JSONB) |
| `rule_sets` | Rule collections |
| `rules` | Individual declarative rules |
| `configuration_audit_log` | Append-only audit |
| `configuration_approvals` | Optional publish approval gate |

Service target: `low-code-service` (port 8088) — not created in this pack.

---

## Migration Principles

| Principle | Rationale |
| --------- | --------- |
| Separate schema `lowcode` | Isolation from Core TMS; clear ownership |
| **Do not ALTER core tables** | Custom fields live in `custom_field_values` only |
| `tenant_id` on every tenant-owned row | Tenant isolation |
| `created_at` / `updated_at` on major tables | Consistent with existing migrations |
| Published configs versioned | `version` column + immutable publish semantics (app layer) |
| Status lifecycle | `DRAFT` → `REVIEW` → `PUBLISHED` → `ARCHIVED` |
| JSONB for flexible payloads only | `value_json`, rule DSL, localization — not for relational keys |
| Indexes on `tenant_id` + lookup patterns | List/filter performance |
| **No FK on polymorphic `entity_id`** | Domain entities span multiple schemas |
| Audit log append-only | No UPDATE/DELETE from application |
| **No secrets** in MVP tables | Connectors deferred to v0.4 |
| TEXT + CHECK constraints | Prefer over PostgreSQL ENUM for early flexibility (matches existing project style) |

---

## Proposed Migration Files

Following existing `golang-migrate` convention (`000001` … `000010` already used):

| File | Purpose |
| ---- | ------- |
| `infrastructure/migrations/000011_create_lowcode_custom_fields_v0.1.up.sql` | Create schema + tables + indexes + triggers |
| `infrastructure/migrations/000011_create_lowcode_custom_fields_v0.1.down.sql` | Reverse drop (dev only) |

**Do not create these files until design approval.**

Alternative naming (if split): `000011_create_lowcode_schema.up.sql` + `000012_create_lowcode_tables.up.sql` — single migration preferred for MVP atomicity.

---

## SQL Draft

> **Design-only SQL below.** Copy into migration file after approval.  
> Assumes `pgcrypto` extension and `core.set_updated_at()` already exist (migration `000001`, `000007`).

### 1. Schema

```sql
-- 000011_create_lowcode_custom_fields_v0.1.up.sql (DRAFT)

CREATE SCHEMA IF NOT EXISTS lowcode;
```

### 2. Shared CHECK constraint helpers (inline per table)

Strategy: **TEXT columns + CHECK constraints** — no PostgreSQL ENUM types.

---

### 3. `lowcode.low_code_configurations`

```sql
CREATE TABLE lowcode.low_code_configurations (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    code                TEXT NOT NULL,
    name                TEXT NOT NULL,
    description         TEXT,
    config_type         TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'DRAFT',
    version             INT NOT NULL DEFAULT 1,
    owner_company_id    UUID,
    created_by_user_id  UUID,
    updated_by_user_id  UUID,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at        TIMESTAMPTZ,
    archived_at         TIMESTAMPTZ,

    CONSTRAINT uq_lowcode_configurations_tenant_code_version
        UNIQUE (tenant_id, code, version),

    CONSTRAINT chk_lowcode_configurations_status
        CHECK (status IN ('DRAFT', 'REVIEW', 'PUBLISHED', 'ARCHIVED')),

    CONSTRAINT chk_lowcode_configurations_config_type
        CHECK (config_type IN (
            'FORM_TEMPLATE',
            'RULE_SET',
            'DOCUMENT_TEMPLATE',
            'DASHBOARD_TEMPLATE',
            'PROCESS_TEMPLATE',
            'CONNECTOR'
        ))
);

CREATE INDEX idx_lowcode_configurations_tenant_id
    ON lowcode.low_code_configurations (tenant_id);

CREATE INDEX idx_lowcode_configurations_tenant_code
    ON lowcode.low_code_configurations (tenant_id, code);

CREATE INDEX idx_lowcode_configurations_tenant_config_type
    ON lowcode.low_code_configurations (tenant_id, config_type);

CREATE INDEX idx_lowcode_configurations_tenant_status
    ON lowcode.low_code_configurations (tenant_id, status);

CREATE INDEX idx_lowcode_configurations_tenant_type_status
    ON lowcode.low_code_configurations (tenant_id, config_type, status);
```

**Optional FK (deferred decision):** `tenant_id REFERENCES core.tenants(id)` — recommend **no FK in v0.1** to match polymorphic pattern and avoid cross-schema coupling; validate in service layer.

---

### 4. `lowcode.form_templates`

```sql
CREATE TABLE lowcode.form_templates (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    configuration_id    UUID NOT NULL REFERENCES lowcode.low_code_configurations(id),
    entity_type         TEXT NOT NULL,
    code                TEXT NOT NULL,
    name                TEXT NOT NULL,
    description         TEXT,
    status              TEXT NOT NULL DEFAULT 'DRAFT',
    version             INT NOT NULL DEFAULT 1,
    locale_default      TEXT NOT NULL DEFAULT 'ru-RU',
    created_by_user_id  UUID,
    updated_by_user_id  UUID,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at        TIMESTAMPTZ,

    CONSTRAINT uq_form_templates_tenant_entity_code_version
        UNIQUE (tenant_id, entity_type, code, version),

    CONSTRAINT chk_form_templates_status
        CHECK (status IN ('DRAFT', 'REVIEW', 'PUBLISHED', 'ARCHIVED')),

    CONSTRAINT chk_form_templates_entity_type
        CHECK (entity_type IN (
            'TRANSPORT_ORDER',
            'RFX',
            'FREIGHT_REQUEST',
            'BID',
            'SHIPMENT',
            'DOCUMENT',
            'BILLING_REGISTER',
            'COMPANY_PROFILE',
            'DRIVER_TASK'
        ))
);

CREATE INDEX idx_form_templates_tenant_id
    ON lowcode.form_templates (tenant_id);

CREATE INDEX idx_form_templates_tenant_entity_type
    ON lowcode.form_templates (tenant_id, entity_type);

CREATE INDEX idx_form_templates_tenant_status
    ON lowcode.form_templates (tenant_id, status);

CREATE INDEX idx_form_templates_tenant_entity_status
    ON lowcode.form_templates (tenant_id, entity_type, status);

CREATE INDEX idx_form_templates_configuration_id
    ON lowcode.form_templates (configuration_id);
```

---

### 5. `lowcode.form_sections`

```sql
CREATE TABLE lowcode.form_sections (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID NOT NULL,
    form_template_id        UUID NOT NULL REFERENCES lowcode.form_templates(id) ON DELETE CASCADE,
    code                    TEXT NOT NULL,
    title                   TEXT NOT NULL,
    description             TEXT,
    sort_order              INT NOT NULL DEFAULT 100,
    visibility_rule_json    JSONB,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_form_sections_tenant_template_code
        UNIQUE (tenant_id, form_template_id, code)
);

CREATE INDEX idx_form_sections_tenant_template
    ON lowcode.form_sections (tenant_id, form_template_id);

CREATE INDEX idx_form_sections_tenant_template_sort
    ON lowcode.form_sections (tenant_id, form_template_id, sort_order);
```

---

### 6. `lowcode.form_fields`

```sql
CREATE TABLE lowcode.form_fields (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID NOT NULL,
    form_template_id        UUID NOT NULL REFERENCES lowcode.form_templates(id) ON DELETE CASCADE,
    section_id              UUID REFERENCES lowcode.form_sections(id) ON DELETE SET NULL,
    code                    TEXT NOT NULL,
    label                   TEXT NOT NULL,
    field_type              TEXT NOT NULL,
    required                BOOLEAN NOT NULL DEFAULT false,
    read_only               BOOLEAN NOT NULL DEFAULT false,
    system_field            BOOLEAN NOT NULL DEFAULT false,
    default_value_json      JSONB,
    options_json            JSONB,
    validation_rule_json    JSONB,
    visibility_rule_json    JSONB,
    localization_json       JSONB,
    sort_order              INT NOT NULL DEFAULT 100,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_form_fields_tenant_template_code
        UNIQUE (tenant_id, form_template_id, code),

    CONSTRAINT chk_form_fields_field_type
        CHECK (field_type IN (
            'TEXT', 'NUMBER', 'DATE', 'DATETIME',
            'SELECT', 'MULTI_SELECT', 'CHECKBOX', 'FILE',
            'COMPANY_REFERENCE', 'ROUTE', 'ADDRESS', 'VEHICLE',
            'MONEY', 'CURRENCY', 'VAT_TAX', 'DOCUMENT_REFERENCE'
        ))
);

CREATE INDEX idx_form_fields_tenant_template
    ON lowcode.form_fields (tenant_id, form_template_id);

CREATE INDEX idx_form_fields_tenant_section
    ON lowcode.form_fields (tenant_id, section_id);

CREATE INDEX idx_form_fields_tenant_field_type
    ON lowcode.form_fields (tenant_id, field_type);

CREATE INDEX idx_form_fields_tenant_template_sort
    ON lowcode.form_fields (tenant_id, form_template_id, sort_order);
```

---

### 7. `lowcode.custom_field_values`

```sql
CREATE TABLE lowcode.custom_field_values (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    entity_type         TEXT NOT NULL,
    entity_id           UUID NOT NULL,
    form_template_id    UUID REFERENCES lowcode.form_templates(id),
    field_id            UUID NOT NULL REFERENCES lowcode.form_fields(id),
    field_code          TEXT NOT NULL,
    value_json          JSONB,
    created_by_user_id  UUID,
    updated_by_user_id  UUID,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_custom_field_values_tenant_entity_field
        UNIQUE (tenant_id, entity_type, entity_id, field_id),

    CONSTRAINT chk_custom_field_values_entity_type
        CHECK (entity_type IN (
            'TRANSPORT_ORDER',
            'RFX',
            'FREIGHT_REQUEST',
            'BID',
            'SHIPMENT',
            'DOCUMENT',
            'BILLING_REGISTER',
            'COMPANY_PROFILE',
            'DRIVER_TASK'
        ))
);

CREATE INDEX idx_custom_field_values_tenant_entity
    ON lowcode.custom_field_values (tenant_id, entity_type, entity_id);

CREATE INDEX idx_custom_field_values_tenant_field_id
    ON lowcode.custom_field_values (tenant_id, field_id);

CREATE INDEX idx_custom_field_values_tenant_field_code
    ON lowcode.custom_field_values (tenant_id, field_code);

-- Optional / deferred — enable only after performance validation:
-- CREATE INDEX idx_custom_field_values_value_json_gin
--     ON lowcode.custom_field_values USING GIN (value_json);
```

**Polymorphic reference notes:**

- `entity_id` has **no FK** to `transport.transport_orders`, `rfx.freight_requests`, etc.
- Application must verify: (1) entity exists in owning service, (2) entity belongs to same `tenant_id`.
- Orphan risk on hard delete → future cleanup job or domain delete events.

---

### 8. `lowcode.rule_sets`

```sql
CREATE TABLE lowcode.rule_sets (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    configuration_id    UUID REFERENCES lowcode.low_code_configurations(id),
    code                TEXT NOT NULL,
    name                TEXT NOT NULL,
    description         TEXT,
    rule_set_type       TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'DRAFT',
    version             INT NOT NULL DEFAULT 1,
    created_by_user_id  UUID,
    updated_by_user_id  UUID,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at        TIMESTAMPTZ,

    CONSTRAINT uq_rule_sets_tenant_code_version
        UNIQUE (tenant_id, code, version),

    CONSTRAINT chk_rule_sets_status
        CHECK (status IN ('DRAFT', 'REVIEW', 'PUBLISHED', 'ARCHIVED')),

    CONSTRAINT chk_rule_sets_rule_set_type
        CHECK (rule_set_type IN (
            'VALIDATION', 'VISIBILITY', 'READ_ONLY', 'NOTIFICATION', 'APPROVAL'
        ))
);

CREATE INDEX idx_rule_sets_tenant_id
    ON lowcode.rule_sets (tenant_id);

CREATE INDEX idx_rule_sets_tenant_type
    ON lowcode.rule_sets (tenant_id, rule_set_type);

CREATE INDEX idx_rule_sets_tenant_status
    ON lowcode.rule_sets (tenant_id, status);
```

---

### 9. `lowcode.rules`

```sql
CREATE TABLE lowcode.rules (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    rule_set_id         UUID NOT NULL REFERENCES lowcode.rule_sets(id) ON DELETE CASCADE,
    code                TEXT NOT NULL,
    name                TEXT NOT NULL,
    description         TEXT,
    priority            INT NOT NULL DEFAULT 100,
    condition_json      JSONB NOT NULL,
    action_json         JSONB NOT NULL,
    enabled             BOOLEAN NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_rules_tenant_set_code
        UNIQUE (tenant_id, rule_set_id, code)
);

CREATE INDEX idx_rules_tenant_rule_set
    ON lowcode.rules (tenant_id, rule_set_id);

CREATE INDEX idx_rules_tenant_set_enabled
    ON lowcode.rules (tenant_id, rule_set_id, enabled);

CREATE INDEX idx_rules_tenant_set_priority
    ON lowcode.rules (tenant_id, rule_set_id, priority);
```

---

### 10. `lowcode.configuration_audit_log`

```sql
CREATE TABLE lowcode.configuration_audit_log (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    configuration_id    UUID,
    entity_type         TEXT NOT NULL,
    entity_id           UUID,
    action              TEXT NOT NULL,
    old_value_json      JSONB,
    new_value_json      JSONB,
    changed_by_user_id  UUID,
    request_id          TEXT,
    ip_address          TEXT,
    user_agent          TEXT,
    changed_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_configuration_audit_log_action
        CHECK (action IN (
            'CREATE', 'UPDATE', 'DELETE',
            'PUBLISH', 'ARCHIVE', 'ROLLBACK',
            'TEST', 'SIMULATE'
        ))
);

CREATE INDEX idx_configuration_audit_log_tenant_config
    ON lowcode.configuration_audit_log (tenant_id, configuration_id);

CREATE INDEX idx_configuration_audit_log_tenant_entity
    ON lowcode.configuration_audit_log (tenant_id, entity_type, entity_id);

CREATE INDEX idx_configuration_audit_log_tenant_changed_at
    ON lowcode.configuration_audit_log (tenant_id, changed_at DESC);

CREATE INDEX idx_configuration_audit_log_tenant_action
    ON lowcode.configuration_audit_log (tenant_id, action);
```

**Append-only:** application must not UPDATE or DELETE rows. Optional DB rule: revoke UPDATE/DELETE grants from app role.

---

### 11. `lowcode.configuration_approvals`

```sql
CREATE TABLE lowcode.configuration_approvals (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID NOT NULL,
    configuration_id        UUID NOT NULL REFERENCES lowcode.low_code_configurations(id),
    requested_by_user_id    UUID,
    approved_by_user_id     UUID,
    status                  TEXT NOT NULL DEFAULT 'PENDING',
    comment                 TEXT,
    requested_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    approved_at             TIMESTAMPTZ,
    rejected_at             TIMESTAMPTZ,
    cancelled_at            TIMESTAMPTZ,

    CONSTRAINT chk_configuration_approvals_status
        CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED', 'CANCELLED'))
);

CREATE INDEX idx_configuration_approvals_tenant_config
    ON lowcode.configuration_approvals (tenant_id, configuration_id);

CREATE INDEX idx_configuration_approvals_tenant_status
    ON lowcode.configuration_approvals (tenant_id, status);

CREATE INDEX idx_configuration_approvals_tenant_requested_at
    ON lowcode.configuration_approvals (tenant_id, requested_at DESC);
```

**MVP note:** Approval workflow optional at runtime (can publish without approval in dev); table created for forward compatibility.

---

### 12. `updated_at` triggers (draft)

Reuse existing function from migration `000007`:

```sql
-- Reuse core.set_updated_at() — do NOT duplicate function definition

CREATE TRIGGER trg_lowcode_configurations_updated_at
    BEFORE UPDATE ON lowcode.low_code_configurations
    FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();

CREATE TRIGGER trg_form_templates_updated_at
    BEFORE UPDATE ON lowcode.form_templates
    FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();

CREATE TRIGGER trg_form_sections_updated_at
    BEFORE UPDATE ON lowcode.form_sections
    FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();

CREATE TRIGGER trg_form_fields_updated_at
    BEFORE UPDATE ON lowcode.form_fields
    FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();

CREATE TRIGGER trg_custom_field_values_updated_at
    BEFORE UPDATE ON lowcode.custom_field_values
    FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();

CREATE TRIGGER trg_rule_sets_updated_at
    BEFORE UPDATE ON lowcode.rule_sets
    FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();

CREATE TRIGGER trg_rules_updated_at
    BEFORE UPDATE ON lowcode.rules
    FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();
```

**Not triggered:** `configuration_audit_log` (append-only), `configuration_approvals` (timestamp columns set explicitly).

---

### 13. Down migration draft

```sql
-- 000011_create_lowcode_custom_fields_v0.1.down.sql (DRAFT — dev only)

DROP TRIGGER IF EXISTS trg_rules_updated_at ON lowcode.rules;
DROP TRIGGER IF EXISTS trg_rule_sets_updated_at ON lowcode.rule_sets;
DROP TRIGGER IF EXISTS trg_custom_field_values_updated_at ON lowcode.custom_field_values;
DROP TRIGGER IF EXISTS trg_form_fields_updated_at ON lowcode.form_fields;
DROP TRIGGER IF EXISTS trg_form_sections_updated_at ON lowcode.form_sections;
DROP TRIGGER IF EXISTS trg_form_templates_updated_at ON lowcode.form_templates;
DROP TRIGGER IF EXISTS trg_lowcode_configurations_updated_at ON lowcode.low_code_configurations;

DROP TABLE IF EXISTS lowcode.configuration_approvals;
DROP TABLE IF EXISTS lowcode.configuration_audit_log;
DROP TABLE IF EXISTS lowcode.custom_field_values;
DROP TABLE IF EXISTS lowcode.rules;
DROP TABLE IF EXISTS lowcode.rule_sets;
DROP TABLE IF EXISTS lowcode.form_fields;
DROP TABLE IF EXISTS lowcode.form_sections;
DROP TABLE IF EXISTS lowcode.form_templates;
DROP TABLE IF EXISTS lowcode.low_code_configurations;

-- Only drop schema if empty (future lowcode tables may exist)
-- DROP SCHEMA IF EXISTS lowcode;
```

---

## Updated_at Trigger Strategy

| Approach | Recommendation |
| -------- | -------------- |
| Reuse `core.set_updated_at()` | ✅ **Yes** — already defined in `000007_create_triggers.up.sql` |
| Create `lowcode.set_updated_at()` | ❌ Avoid duplication |
| Add triggers in same migration as tables | ✅ `000011` includes triggers |
| Extend `000007` retroactively | ❌ Do not modify applied migrations |

If `core.set_updated_at()` is unavailable (fresh DB without `000007`), migration `000011` should fail fast — migrations are sequential.

---

## RLS Strategy

| Option | v0.1 recommendation |
| ------ | ------------------- |
| Application-level `tenant_id` filtering | ✅ **Primary** — matches existing services |
| PostgreSQL RLS policies | ❌ **Defer** — no platform-wide RLS strategy yet |
| `current_setting('app.tenant_id')` session var | Future — requires gateway/service middleware standardization |

**Rationale:** Existing services (`company-service`, `transport-order-service`, etc.) use application-level tenant checks, not RLS. Introducing RLS only on `lowcode` schema would be inconsistent without defense-in-depth everywhere.

**Future RLS sketch (not in v0.1 migration):**

```sql
-- EXAMPLE ONLY — not part of MVP migration
ALTER TABLE lowcode.form_templates ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON lowcode.form_templates
    USING (tenant_id = current_setting('app.tenant_id')::uuid);
```

---

## Seed Templates Draft

Dev/demo seed templates — **not in migration SQL** unless explicitly required. Create via `scripts/dev/seed_lowcode_templates.sh` (future) or demo seed extension.

### 1. Transport Order Default Form

| Section | Fields |
| ------- | ------ |
| **Cargo** | `cargo_class` (SELECT), `internal_cost_center` (TEXT), `loading_window_note` (TEXT) |

- `entity_type`: `TRANSPORT_ORDER`
- `code`: `DEFAULT-TO-FORM-V1`

### 2. Shipment Default Form

| Section | Fields |
| ------- | ------ |
| **Operations** | `temperature_mode` (SELECT), `loading_contact_phone` (TEXT), `driver_comment` (TEXT) |

- `entity_type`: `SHIPMENT`
- `code`: `DEFAULT-SH-FORM-V1`

### 3. Billing Register Default Form

| Section | Fields |
| ------- | ------ |
| **Finance** | `cost_allocation_code` (TEXT), `approval_group` (TEXT), `payment_priority` (SELECT) |

- `entity_type`: `BILLING_REGISTER`
- `code`: `DEFAULT-BR-FORM-V1`

**Seed rules:**

- Templates are **tenant-specific** (`tenant_id = 74519f22-…` for dev)
- Status: `PUBLISHED` after seed
- No production seed in migration
- Extend `make seed-demo-data` later — optional pack

---

## Rollback Strategy

| Environment | Strategy |
| ----------- | -------- |
| **Local dev** | `make migrate-down` — runs `.down.sql` drop order |
| **Staging** | Prefer `ARCHIVED` status over drop |
| **Production** | **Never destructive drop** — forward-only migrations + archive |

**Reverse dependency drop order:**

1. `configuration_approvals`
2. `configuration_audit_log`
3. `custom_field_values`
4. `rules`
5. `rule_sets`
6. `form_fields`
7. `form_sections`
8. `form_templates`
9. `low_code_configurations`
10. Triggers
11. Schema `lowcode` — only if no other lowcode objects exist

---

## Performance Considerations

| Topic | Guidance |
| ----- | -------- |
| `custom_field_values` growth | Highest volume table — monitor row count per tenant |
| Required indexes | `(tenant_id, entity_type, entity_id)` — mandatory |
| GIN on `value_json` | **Defer** until query patterns proven |
| List pages | Do not JOIN all custom fields by default |
| Batch reads | Future API: values by multiple `entity_id`s |
| Audit log | Partition by month (`changed_at`) in v0.2+ |
| Published template cache | In-memory cache in `low-code-service` |
| Connection pool | Dedicated pool for `low-code-service` like other services |

---

## Migration Risk Assessment

| Risk | Severity | Mitigation |
| ---- | -------- | ---------- |
| Polymorphic `entity_id` without FK | Medium | Mandatory tenant + entity existence check on write |
| JSONB schema drift | Medium | Validate `value_json` envelope in service layer |
| Tenant mismatch bugs | High | Integration tests; gateway tenant header enforcement |
| Large audit log | Low (early) | Retention policy; partitioning later |
| Over-indexing | Low | MVP indexes only; add GIN when measured |
| Published immutability not DB-enforced | Medium | Application: new version on edit; optional trigger |
| Status lifecycle bypass | Medium | CHECK constraints + service validation |
| Migration breaks smoke test | High | Run `make migrate-up` + `make integration-smoke-test` before merge |
| Empty `lowcode` schema unused | Low | Feature flag `LOW_CODE_ENABLED=false` until service ready |

---

## Acceptance Criteria for Future Real Migration

Future migration `000011` is **ready to merge** when:

| # | Criterion |
| - | --------- |
| 1 | Creates schema `lowcode` |
| 2 | Creates all 10 MVP tables |
| 3 | Does **not** ALTER core/transport/rfx/documents/billing tables |
| 4 | `tenant_id NOT NULL` on all tenant-owned tables |
| 5 | Indexes for common tenant query patterns |
| 6 | Status and type CHECK constraints present |
| 7 | No secret/credential columns |
| 8 | `.down.sql` rollback script included |
| 9 | `make migrate-up` succeeds on clean DB |
| 10 | `make migrate-down` + `make migrate-up` round-trip succeeds locally |
| 11 | `make integration-smoke-test` passes (lowcode unused) |
| 12 | `make db-check` updated to list `lowcode` schema (optional docs PR) |
| 13 | Reviewed against this design document |

---

## Recommended Next Action

1. **Review and approve** this migration design (architecture + DBA review).
2. **Create real migration** `000011_create_lowcode_custom_fields_v0.1.up/down.sql` — only after approval.
3. Run `make migrate-up` locally and verify schema via `make db-check`.
4. **Create `low-code-service` skeleton** (health, metrics, no business routes yet).
5. **Repository layer** for read-only form template queries.
6. **Read-only API** `GET /v1/low-code/form-templates` — separate API contract draft pack.
7. Keep `make integration-smoke-test` green throughout.

---

## Related Documentation

| Document | Role |
| -------- | ---- |
| `docs/LOW_CODE_CUSTOM_FIELDS_TECHNICAL_DESIGN_V0.1.md` | Service boundary, validation flow |
| `infrastructure/migrations/000007_create_triggers.up.sql` | `core.set_updated_at()` reference |
| `infrastructure/migrations/000001_create_schemas.up.sql` | Existing schema pattern |
| `Makefile` | `migrate-up`, `migrate-down`, `db-check` |

---

## Implementation Notes

| Item | Value |
| ---- | ----- |
| Migration files | `infrastructure/migrations/000011_create_lowcode_custom_fields_v0.1.up.sql`, `infrastructure/migrations/000011_create_lowcode_custom_fields_v0.1.down.sql` |
| `core.set_updated_at()` reused | Yes — triggers on 7 tables with `updated_at` use `core.set_updated_at()` from migration `000007` |
| Migration applied | Yes — `make migrate-up` applied version `11/u create_lowcode_custom_fields_v0.1` |
| Schema / tables verified | Yes — `lowcode` schema with 9 tables, 48 indexes, CHECK/FK/UNIQUE constraints confirmed via `psql` |
| GIN on `value_json` | Not added (deferred per design) |
| RLS | Not enabled (deferred per design) |
| Core tables changed | No |
| Seed data in migration | No |
| `make health-check` | Passed |
| `make seed-dev-admin` | Passed (script via Git Bash on Windows) |
| `make seed-demo-data` | Passed (script via Git Bash on Windows) |
| `make integration-smoke-test` | Passed — `SMOKE TEST PASSED` |

---

## Document History

| Version | Date | Notes |
| ------- | ---- | ----- |
| v0.1 | 2026-06-22 | Initial migration design — docs only, no SQL files |
| v0.1 impl | 2026-06-22 | Real migration `000011` created and applied |

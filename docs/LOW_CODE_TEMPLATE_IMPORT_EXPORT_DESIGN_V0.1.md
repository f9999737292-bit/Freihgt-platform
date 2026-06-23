# Low-code Template Import/Export Design v0.1

## Summary

Design-only specification for **safe JSON import/export** of low-code form templates. Enables administrators to move template definitions between dev/staging/prod or between tenants **without direct database access**, while preserving the existing draft → publish lifecycle, active template policy, and permissions model.

**Baseline commit:** `c68412b` (`feat: add low-code permissions matrix and UI guardrails`).

**Implementation status:** design only — no backend, frontend, API, or migration changes in this pack.

## Why Import/Export Is Needed

| Problem | Import/export solution |
|---------|------------------------|
| Manual psql / seed scripts for templates | Portable JSON artifact |
| Copy-paste errors between environments | Validated schema + preview |
| No audit trail for cross-env moves | Dedicated audit events |
| Risk of publishing wrong version | Import always creates DRAFT; publish explicit |
| Custom field values tied to old template IDs | Separate migrate-to-active flow after publish |

## Scope

**In scope (design)**

- Export JSON format for DRAFT and PUBLISHED templates
- Import preview + execute API design
- Conflict strategies
- Field matching (`field_code` stable, IDs regenerated)
- Security guardrails (reuse existing draft validation rules)
- Permissions (`PLATFORM_ADMIN` when auth-on)
- Audit events
- Admin UI and CLI/script flows
- Verification plan for future implementation packs

**Out of scope (v0.1 design / implementation)**

- Custom field values export/import
- Audit event export
- Bulk multi-template ZIP archives
- Automatic publish on import
- Cross-tenant value migration
- i18n label maps (future extension; v0.1 uses single `label` string)
- BPMN, Rule Engine, Connectors

## Current Template Model

Source: `services/low-code-service`, migration `000011`, docs `LOW_CODE_FORM_TEMPLATE_*`.

### Identity & lifecycle

| Attribute | Storage | Notes |
|-----------|---------|-------|
| `tenant_id` | DB | Required on all operations; from `X-Tenant-ID` |
| `entity_type` | template row | Enum: TRANSPORT_ORDER, SHIPMENT, BILLING_REGISTER, … |
| `code` | template row | Lowercase snake_case; logical key per tenant + entity_type |
| `version` | template row | Server-assigned integer; increments on publish / clone |
| `status` | template row | `DRAFT` \| `PUBLISHED` \| `ARCHIVED` |
| `name`, `description` | template row | Human-readable |
| `published_at` | template row | Set on publish only |
| Active template | **derived** | Highest PUBLISHED version per `(tenant_id, entity_type, code)` |

### Structure

```
form_template
├── sections[] (code, title, sort_order)
│   └── fields[] (code, label, field_type, required, read_only, system_field,
│                 options_json, validation_rule_json, visibility_rule_json, sort_order)
```

### Field semantics

| Flag | Meaning |
|------|---------|
| `required` | Static required on save |
| `read_only` | PUT rejects client writes (`READ_ONLY_FIELD_PROTECTED`) |
| `system_field` | Protected; migration skips |
| `validation_rule_json` | Conditional required (`if` / `then`) |
| `visibility_rule_json` | Preview/runtime visibility rules |
| `options_json` | SELECT / MULTI_SELECT options |

**Labels (v0.1):** single `label` string per field (UI i18n via web-admin `i18n` files is separate from template storage). Future export may add optional `labels_i18n: { "en-US": "...", "ru-RU": "..." }` in extension block — not in v0.1 schema.

### Existing admin API (unchanged by this design)

| Operation | Endpoint |
|-----------|----------|
| Create DRAFT | `POST /admin/form-templates` |
| Update DRAFT | `PUT /admin/form-templates/{id}` |
| Publish | `POST /admin/form-templates/{id}/publish` |
| Clone PUBLISHED → DRAFT | `POST /admin/form-templates/{id}/clone-to-draft` |

### Existing audit (configuration_audit_log)

| Event | Kind |
|-------|------|
| Draft created | `FORM_TEMPLATE_DRAFT_CREATED` |
| Draft updated | `FORM_TEMPLATE_DRAFT_UPDATED` |
| Published | `FORM_TEMPLATE_DRAFT_PUBLISHED` |
| Cloned | `FORM_TEMPLATE_CLONED_TO_DRAFT` |

Runtime custom-field audit remains separate (`CUSTOM_FIELD_VALUES_*`).

## Export Format

### Recommended endpoint (v0.1 implementation)

```http
GET /api/v1/low-code/admin/form-templates/{id}/export
X-Tenant-ID: {tenant_uuid}
X-User-ID: {actor_uuid}   # when LOW_CODE_ADMIN_AUTH_ENABLED=true
```

**Alternative (batch export, future):** `POST /admin/form-templates/export` with `{ "template_ids": ["..."] }`.

### Response envelope

```json
{
  "schema_version": "lowcode.template.export.v1",
  "exported_at": "2026-06-24T12:00:00Z",
  "source": {
    "tenant_id": "74519f22-ff9b-4a8b-8fff-a958c689682f",
    "template_id": "b1111111-1111-4111-8111-111111111102",
    "environment": "dev",
    "template_code": "transport_order_default",
    "template_version": 1,
    "template_status": "PUBLISHED"
  },
  "template": {
    "entity_type": "TRANSPORT_ORDER",
    "code": "transport_order_default",
    "name": "Transport Order Default Form",
    "description": "Default published template",
    "version": 1,
    "status": "PUBLISHED",
    "sections": [
      {
        "code": "general",
        "title": "General",
        "sort_order": 100,
        "fields": [
          {
            "code": "cargo_class",
            "label": "Cargo class",
            "field_type": "SELECT",
            "required": false,
            "read_only": false,
            "system_field": false,
            "options_json": {
              "options": [
                { "value": "GENERAL", "label": "General" },
                { "value": "DANGEROUS", "label": "Dangerous" }
              ]
            },
            "validation_rule_json": null,
            "visibility_rule_json": null,
            "sort_order": 100
          }
        ]
      }
    ]
  },
  "metadata": {
    "exported_by": "8541a3a3-bde7-4fed-9501-37b9953bf904",
    "request_id": "req-uuid",
    "sections_count": 1,
    "fields_count": 3,
    "checksum_sha256": "optional-hex-digest-of-template-object"
  }
}
```

### Export rules

| Rule | Detail |
|------|--------|
| Allowed source status | `DRAFT` and `PUBLISHED` ( `ARCHIVED` optional in v0.1.1 ) |
| Permission | `PLATFORM_ADMIN` when auth-on; default-off dev unchanged |
| Exclude | Custom field values, audit rows, DB UUIDs for sections/fields |
| Include | `field_code`, field config, rules JSON, sort orders |
| Source IDs | Only in `source.*` metadata — **not** used as import keys |
| `tenant_id` in export | Informational in `source` only; import ignores it |
| `environment` | Optional client hint (`dev` / `staging` / `prod`) |

## Import Format

### Endpoints (proposed)

```http
POST /api/v1/low-code/admin/form-templates/import-preview
POST /api/v1/low-code/admin/form-templates/import
```

Both require `X-Tenant-ID`. Admin auth guard applies when `LOW_CODE_ADMIN_AUTH_ENABLED=true`.

### Request body

```json
{
  "schema_version": "lowcode.template.export.v1",
  "mode": "CREATE_DRAFT",
  "conflict_strategy": "NEW_VERSION",
  "target_code": null,
  "dry_run": true,
  "template": {
    "entity_type": "TRANSPORT_ORDER",
    "code": "transport_order_default",
    "name": "Transport Order Default Form",
    "description": "Imported from dev",
    "sections": []
  },
  "source_metadata": {
    "source_tenant_id": "74519f22-ff9b-4a8b-8fff-a958c689682f",
    "source_template_id": "b1111111-1111-4111-8111-111111111102",
    "source_version": 1,
    "source_status": "PUBLISHED",
    "exported_at": "2026-06-24T12:00:00Z"
  }
}
```

| Field | Required | Notes |
|-------|----------|-------|
| `schema_version` | yes | Must match supported version |
| `mode` | yes | See modes below |
| `conflict_strategy` | yes | See conflict section |
| `target_code` | optional | Override `template.code` on import |
| `dry_run` | preview only | `true` for import-preview; ignored on execute |
| `template` | yes | Same shape as export `template` (without DB ids) |
| `source_metadata` | optional | Traceability only; not trusted for auth |

### Import modes

| Mode | Behavior |
|------|----------|
| `CREATE_DRAFT` | Default. Create new DRAFT with `template.code` (or `target_code`) |
| `CREATE_NEW_CODE` | Require `target_code`; fail if code exists unless strategy allows |
| `REPLACE_EXISTING_DRAFT` | Replace sections/fields of existing DRAFT with same code (DRAFT only) |
| `NEW_VERSION_FROM_EXPORT` | Alias for `CREATE_DRAFT` + `NEW_VERSION` strategy when published versions exist |

### Import rules (hard)

1. Import **always** creates or updates **DRAFT** — never `PUBLISHED` directly.
2. Publish remains explicit via existing `POST .../publish`.
3. `tenant_id` from request context (`X-Tenant-ID`) — **never** from export file.
4. Reject unknown `schema_version`.
5. Reject duplicate `field_code` / `section.code` within template.
6. Reject `system_field: true` unless `allow_system_fields=true` admin flag (default **false**).
7. Validate via existing `ValidateDraftFormTemplateInput` rules.
8. Max payload size (recommended): **512 KB** request body.
9. Do not import `status`, `version`, `published_at` from file — server assigns.

## JSON Schema

Logical schema (implementation may use Go structs + manual validation mirroring draft API):

```yaml
schema_version: enum ["lowcode.template.export.v1"]
exported_at: ISO8601 datetime (export only)

source:  # export only
  tenant_id: uuid string
  template_id: uuid string
  environment: string optional
  template_code: string
  template_version: integer
  template_status: enum [DRAFT, PUBLISHED, ARCHIVED]

template:
  entity_type: enum [TRANSPORT_ORDER, SHIPMENT, ...]
  code: string pattern ^[a-z][a-z0-9_]*$
  name: string minLength 1 maxLength 256
  description: string maxLength 2000 optional
  version: integer optional (informational on export; ignored on import)
  status: enum optional (informational on export; ignored on import)
  sections:
    type: array
    minItems: 1
    maxItems: 50
    items:
      code: string pattern ^[a-z][a-z0-9_]*$
      title: string maxLength 256
      sort_order: integer
      fields:
        type: array
        maxItems: 200 total across template
        items:
          code: string
          label: string maxLength 256
          field_type: enum [TEXT, NUMBER, DATE, ...]
          required: boolean
          read_only: boolean
          system_field: boolean default false
          options_json: object optional
          validation_rule_json: object optional
          visibility_rule_json: object optional
          sort_order: integer

metadata:  # export only
  exported_by: uuid optional
  request_id: string optional
  sections_count: integer
  fields_count: integer
  checksum_sha256: string optional
```

**Unknown keys:** reject at import (strict mode v0.1) to prevent injection of future dangerous fields.

## Versioning Rules

| Topic | Rule |
|-------|------|
| Export schema | `lowcode.template.export.v1` |
| Breaking changes | New schema version (`v2`); importer supports `v1` + `v2` during transition |
| Template `version` | On import as new DRAFT: if code exists with PUBLISHED versions, next publish gets `max(version)+1` |
| Active template | Unchanged until new version published |
| Downgrade | Not supported — import as new DRAFT only |

## Tenant Handling

```
Export tenant A  →  Import into tenant B
```

| Step | Behavior |
|------|----------|
| Export | Records `source.tenant_id` for traceability |
| Import | Uses `X-Tenant-ID` from authenticated request |
| Trust | Import **must not** trust `source.tenant_id` or file `tenant_id` |
| Cross-tenant | Allowed for `PLATFORM_ADMIN`; audit records source + target |
| Same tenant | Normal version bump / new draft flow |

## Template Code Handling

| Scenario | Recommended strategy |
|----------|------------------------|
| Same code, no DRAFT, has PUBLISHED | `NEW_VERSION` → create DRAFT; publish becomes vN+1 |
| Same code, DRAFT exists | `REPLACE_EXISTING_DRAFT` or `FAIL_IF_EXISTS` |
| New code in target tenant | `CREATE_NEW_CODE` with `target_code` |
| Code collision undesired | `target_code: "transport_order_imported_v2"` |

Import normalizes `code` / `target_code` to lowercase snake_case.

## Draft/PUBLISHED Handling

| Source status | Export | Import result |
|---------------|--------|---------------|
| DRAFT | Allowed | New DRAFT (or replace existing DRAFT) |
| PUBLISHED | Allowed | New DRAFT copy; **never** auto-published |
| ARCHIVED | Future | Import as DRAFT with new code recommended |

**Never overwrite PUBLISHED** template rows via import.

## Field Matching Strategy

| Key | Import behavior |
|-----|-----------------|
| `field_code` | Stable logical identifier; preserved across environments |
| `field_id` (DB) | **Not exported**; regenerated on import |
| `section_id` (DB) | **Not exported**; regenerated |
| `form_template_id` | New UUID on import |
| Existing custom values | **Not moved** by import |
| After publish | Use existing `migration-preview` / `migrate-to-active` / batch migration |

Field-level diff (preview):

| Change | Migration impact |
|--------|------------------|
| Same code, same type | SAFE / compatible |
| Same code, type change | WARNING / BLOCKED per migration rules |
| Removed field | Legacy field on migration preview |
| New field | Missing required check after publish |

## Validation Rules

Import reuses `ValidateDraftFormTemplateInput` (`form_template_draft.go`):

| Check | Limit |
|-------|-------|
| Entity type | Allowed enum |
| Section count | ≤ 50 |
| Field count | ≤ 200 |
| Field types | Whitelist (16 types) |
| Rule JSON | Valid JSON; no SQL fragments |
| Options JSON | Required shape for SELECT types |
| Labels | Non-empty; max 256 chars (recommended) |

Conditional required / visibility rules: stored as JSON; validated for structure only (same as draft API).

## Security Guardrails

| Threat | Mitigation |
|--------|------------|
| Arbitrary code execution | No scripting in template JSON |
| SQL injection | `containsSQLFragment` on rule JSON |
| XSS via labels | Plain text labels only; no HTML; UI escapes output (no v-html) |
| Unknown field types | Whitelist `allowedFieldTypes` |
| Unknown top-level keys | Strict schema rejection |
| Oversized payload | 512 KB max body; 50 sections / 200 fields |
| Tenant spoofing | `tenant_id` from auth context only |
| Privilege escalation | `PLATFORM_ADMIN` for import/export when auth-on |
| Secret leakage | Export excludes credentials, tokens, custom values |
| Auto-publish attack | Import cannot set PUBLISHED |
| system_field abuse | Reject unless explicit admin override flag |

## Permissions

Per `docs/LOW_CODE_PERMISSIONS_MATRIX_V0.1.md`:

| Operation | Role (auth-on) | Default-off |
|-----------|----------------|-------------|
| Export template | `PLATFORM_ADMIN` | Open (dev) |
| Import preview | `PLATFORM_ADMIN` | Open (dev) |
| Import execute | `PLATFORM_ADMIN` | Open (dev) |
| Publish imported DRAFT | `PLATFORM_ADMIN` | Open (dev) |

Runtime template read remains tenant-scoped without role check.

## Audit Model

New configuration audit event kinds (proposed):

| Event | When |
|-------|------|
| `FORM_TEMPLATE_EXPORTED` | Successful export |
| `FORM_TEMPLATE_IMPORT_PREVIEWED` | Preview completed (dry-run) |
| `FORM_TEMPLATE_IMPORTED_AS_DRAFT` | DRAFT created/updated via import |

### Payload (new_value_json)

```json
{
  "event_kind": "FORM_TEMPLATE_IMPORTED_AS_DRAFT",
  "template_id": "new-draft-uuid",
  "code": "transport_order_default",
  "version": 2,
  "entity_type": "TRANSPORT_ORDER",
  "source_schema_version": "lowcode.template.export.v1",
  "conflict_strategy": "NEW_VERSION",
  "import_mode": "CREATE_DRAFT",
  "imported_sections_count": 2,
  "imported_fields_count": 8,
  "source_template_id": "b1111111-...",
  "source_tenant_id": "74519f22-...",
  "source_version": 1,
  "target_code": "transport_order_default",
  "dry_run": false
}
```

Actor: `X-User-ID`. Correlation: `X-Request-ID`.

## Import Conflict Strategy

| Strategy | Behavior | v0.1 default |
|----------|----------|--------------|
| `FAIL_IF_EXISTS` | Error if DRAFT or any template with same code exists | — |
| `NEW_VERSION` | Create DRAFT; on publish, version = max(existing)+1 | **Recommended default** |
| `NEW_CODE` | Require unique `target_code`; always new DRAFT | — |
| `REPLACE_EXISTING_DRAFT` | Upsert existing DRAFT only; fail if no DRAFT | — |

**Hard rules:**

- Never overwrite `PUBLISHED` or `ARCHIVED` rows
- Never auto-publish
- Preview returns chosen strategy outcome before execute

### Preview response (proposed)

```json
{
  "status": "READY",
  "conflict_strategy": "NEW_VERSION",
  "target_entity_type": "TRANSPORT_ORDER",
  "target_code": "transport_order_default",
  "existing_draft_id": null,
  "existing_published_versions": [1],
  "proposed_draft_version_on_publish": 2,
  "warnings": [],
  "validation_errors": [],
  "summary": {
    "sections_count": 2,
    "fields_count": 8,
    "new_field_codes": ["loading_window_note"],
    "removed_field_codes": [],
    "type_changes": []
  }
}
```

## Import Preview Flow

```
1. Client POST import-preview with full template JSON + strategy
2. Server validate schema_version + template shape
3. Server run ValidateDraftFormTemplateInput (dry)
4. Server resolve conflicts for tenant + entity_type + code
5. Server compare field_codes vs existing active/published template (optional warnings)
6. Return preview response (no DB writes except optional audit preview event)
7. Client shows summary in UI
```

## Import Execute Flow

```
1. Client POST import (dry_run=false) with same payload as approved preview
2. Server re-run validation + conflict resolution (idempotent)
3. Server BEGIN transaction
4. Create or replace DRAFT (reuse admin Create/Update repository paths)
5. Write FORM_TEMPLATE_IMPORTED_AS_DRAFT audit
6. COMMIT
7. Return { id, status: "DRAFT", version, code }
8. Client links to /low-code/admin/form-templates/{id}
9. Operator reviews in editor + publish when ready
10. Optional: run migration-preview for affected entities
```

## Admin UI Flow

### Export (future pack)

1. Admin template detail page → **Export JSON** button
2. Calls `GET .../export`
3. Browser download `transport_order_default.v1.export.json` or copy to clipboard
4. Toast success; no navigation

### Import (future pack)

1. `/low-code/admin/form-templates/import` or modal from admin list
2. Paste JSON or upload file
3. Select conflict strategy (default `NEW_VERSION`)
4. Optional `target_code` override
5. **Preview** → show validation errors, warnings, proposed DRAFT metadata
6. **Import as DRAFT** → execute
7. Redirect to draft editor
8. **Publish** remains separate action with existing diff/review UI

## API Design

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/v1/low-code/admin/form-templates/{id}/export` | Export single template |
| POST | `/v1/low-code/admin/form-templates/import-preview` | Dry-run validation + conflict summary |
| POST | `/v1/low-code/admin/form-templates/import` | Create/update DRAFT |

All under existing `adminGuard` (`RequireLowCodeAdmin`).

### Error codes (proposed)

| Code | HTTP | When |
|------|------|------|
| `VALIDATION_ERROR` | 400 | Schema / field validation failed |
| `UNSUPPORTED_SCHEMA_VERSION` | 400 | Unknown `schema_version` |
| `IMPORT_PAYLOAD_TOO_LARGE` | 413 | Body > 512 KB |
| `FORM_TEMPLATE_CONFLICT` | 409 | Strategy `FAIL_IF_EXISTS` |
| `FORM_TEMPLATE_NOT_DRAFT` | 409 | `REPLACE_EXISTING_DRAFT` but no DRAFT |
| `FORBIDDEN` | 403 | Auth-on without PLATFORM_ADMIN |
| `TENANT_REQUIRED` | 400 | Missing tenant header |

Successful import response mirrors create draft:

```json
{
  "id": "...",
  "status": "DRAFT",
  "version": 1,
  "code": "transport_order_default",
  "import_summary": {
    "sections_count": 2,
    "fields_count": 8,
    "conflict_strategy": "NEW_VERSION"
  }
}
```

## CLI/Script Design

Future dev script (not implemented):

```powershell
# Export
./scripts/dev/lowcode_template_export.sh --tenant $T --template-id $ID --out template.json

# Import preview
./scripts/dev/lowcode_template_import.sh --tenant $T --file template.json --preview

# Import execute
./scripts/dev/lowcode_template_import.sh --tenant $T --file template.json --strategy NEW_VERSION
```

Uses gateway URL + `X-Tenant-ID` + optional `X-User-ID`. Idempotent preview before execute.

Sample payload location (future): `scripts/dev/payloads/lowcode_template_export_sample.v1.json`

## Error Handling

| Failure | User-facing | System |
|---------|---------------|--------|
| Invalid JSON | 400 + parse error | Log request_id |
| Bad field_type | 400 + field path | No partial write |
| Conflict | 409 + preview hint | Suggest strategy change |
| Auth failure | 401/403 | No leak of template existence cross-tenant |
| DB failure | 500 | Transaction rollback |

Import execute must be **atomic** (single transaction).

## Rollback Strategy

| Situation | Action |
|-----------|--------|
| Bad import DRAFT | Delete DRAFT via admin (future) or leave unpublished |
| Published wrong version | Publish is separate; use clone-to-draft fix path |
| Values on old template | Entities keep old `form_template_id` until migration |
| Emergency | Do not DELETE PUBLISHED; archive policy future pack |

No automatic rollback of publish — operator uses audit + migration tools.

## Verification Plan

Future packs must verify:

| Check | Method |
|-------|--------|
| Export PUBLISHED template | GET export → valid JSON |
| Export excludes values/audit | Assert keys absent |
| Import preview validation errors | Invalid field_type → 400 |
| Import creates DRAFT only | status always DRAFT |
| NEW_VERSION strategy | Publish → version increment |
| FAIL_IF_EXISTS | 409 when DRAFT exists |
| REPLACE_EXISTING_DRAFT | Updates DRAFT sections |
| Tenant isolation | Cross-tenant import requires correct header |
| Auth-on guard | 401/403 without PLATFORM_ADMIN |
| Default-off smoke | Export/import without X-User-ID |
| Audit events | Three new kinds in configuration_audit_log |
| SQL fragment in rules | Rejected |
| Max sections/fields | 400 |

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo
# After Export API pack:
curl.exe -H "X-Tenant-ID: $T" http://localhost:8080/api/v1/low-code/admin/form-templates/{id}/export
```

## Next Implementation Packs

| # | Pack | Deliverable |
|---|------|-------------|
| 1 | **Low-code Template Export API Pack v0.1** | `GET .../export`, audit, tests |
| 2 | **Low-code Template Import Preview API Pack v0.1** | `POST .../import-preview`, validation |
| 3 | **Low-code Template Import Execute API Pack v0.1** | `POST .../import`, DRAFT create, audit |
| 4 | **Admin UI Template Import/Export Pack v0.1** | Export button, import wizard |
| 5 | **Template Import/Export Edge Cases Test Pack v0.1** | Conflict, tenant, auth, size limits |

**Recommended next:** **Low-code Template Export API Pack v0.1**

## Related Documentation

- `docs/LOW_CODE_FORM_TEMPLATE_DRAFT_API_V0.1.md`
- `docs/LOW_CODE_FORM_TEMPLATE_VERSION_ACTIVATION_POLICY_V0.1.md`
- `docs/LOW_CODE_PERMISSIONS_MATRIX_V0.1.md`
- `docs/LOW_CODE_RUNTIME_PILOT_READINESS_V0.1.md`
- `docs/LOW_CODE_MIGRATE_TO_ACTIVE_DESIGN_V0.1.md`
- `scripts/dev/payloads/lowcode_form_template_draft_transport_order.json` (shape reference)

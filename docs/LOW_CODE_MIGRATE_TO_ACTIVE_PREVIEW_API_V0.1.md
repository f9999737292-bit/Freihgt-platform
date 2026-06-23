# Low-code Migrate-to-Active Preview API v0.1

Date: 2026-06-23  
Project: `D:\Projects\freight-platform`  
Related:

- `docs/LOW_CODE_MIGRATE_TO_ACTIVE_DESIGN_V0.1.md`
- `docs/PLATFORM_RUNTIME_RECHECK_V0.1.md`

---

## Summary

Read-only admin API that previews how custom field values for one or more entities would remap onto the current active published form template. Matching is by stable `field_code`; no values are written and no core entity data is changed.

---

## Endpoint

```http
POST /v1/low-code/admin/custom-field-values/migration-preview
POST /api/v1/low-code/admin/custom-field-values/migration-preview
```

Headers:

- `X-Tenant-ID` — required
- `Content-Type: application/json`

---

## Request

```json
{
  "entity_type": "TRANSPORT_ORDER",
  "template_code": "transport_order_default",
  "entity_ids": [
    "2db04b49-665c-469f-bcb1-ffeb1274fedb"
  ],
  "target_template_id": "optional-uuid"
}
```

Rules:

| Field | Rule |
|-------|------|
| `entity_type` | Required; validated against allowed entity types |
| `entity_ids` | Required; 1..100 UUIDs |
| `template_code` | Required when multiple active published templates exist for `entity_type` |
| `target_template_id` | Optional; must belong to tenant and be `PUBLISHED` |

If `target_template_id` is omitted, the service resolves the active published template by `entity_type` + `template_code`.

---

## Response

```json
{
  "tenant_id": "74519f22-ff9b-4a8b-8fff-a958c689682f",
  "entity_type": "TRANSPORT_ORDER",
  "target_template": {
    "id": "...",
    "code": "transport_order_default",
    "version": 1
  },
  "summary": {
    "entities_checked": 1,
    "safe_to_migrate": 1,
    "warnings": 0,
    "blocked": 0
  },
  "items": [
    {
      "entity_id": "2db04b49-665c-469f-bcb1-ffeb1274fedb",
      "source_template_id": "...",
      "target_template_id": "...",
      "status": "SAFE",
      "copied_fields": ["cargo_class", "internal_cost_center"],
      "legacy_fields": [],
      "missing_required_fields": [],
      "incompatible_fields": [],
      "warnings": []
    }
  ]
}
```

Per-entity `status`:

| Status | Meaning |
|--------|---------|
| `SAFE` | Compatible copy; no legacy, missing required, or warnings |
| `WARNING` | Legacy fields, missing required fields, or type-conversion warnings |
| `BLOCKED` | Incompatible fields or values that fail target validation |

---

## Matching Strategy

1. Load existing values for each entity (tenant-scoped).
2. Infer `source_template_id` from the most frequent `form_template_id` on value rows.
3. Load source template field definitions when available (for type comparison).
4. Match values to target template fields by **`field_code` only** — never by `field_id`.
5. Simulate copy into target template field definitions without persisting.

Field buckets:

- `copied_fields` — compatible and passes target validation
- `legacy_fields` — value exists but field removed from target template
- `missing_required_fields` — target required field absent after simulated copy
- `incompatible_fields` — type/value cannot migrate
- `warnings` — protected-field skips, soft conversions

Protected fields (`system_field`, `read_only`) are skipped and listed in `warnings`.

---

## Compatibility Rules

Conservative v0.1 rules:

| Source | Target | Result |
|--------|--------|--------|
| Same type | Same type | Compatible |
| TEXT | CURRENCY | Compatible |
| CURRENCY | TEXT | Compatible |
| SELECT | TEXT | Compatible |
| TEXT | SELECT | Warning if value not in options |
| SELECT | MULTI_SELECT | Warning (wrap single value in array for validation) |
| NUMBER | MONEY | Blocked |
| MONEY | NUMBER | Blocked |
| CHECKBOX | non-CHECKBOX | Blocked |
| DATE | non-DATE | Blocked |
| DATETIME | non-DATETIME | Blocked |

Final copy decision uses `ValidateFieldValue` against the target field definition.

---

## Status Calculation

Summary counts:

- `safe_to_migrate` — items with status `SAFE`
- `warnings` — items with status `WARNING`
- `blocked` — items with status `BLOCKED`

Entity status priority:

1. `BLOCKED` if any `incompatible_fields`
2. `WARNING` if `legacy_fields`, `missing_required_fields`, or `warnings`
3. `SAFE` otherwise

---

## Tenant Isolation

- `X-Tenant-ID` required on every request.
- Target template lookup scoped to tenant.
- Value reads scoped to tenant.
- Cross-tenant template access returns `TENANT_MISMATCH` / not found.

---

## Side Effects

**None.** Preview is read-only:

- No updates to `lowcode.custom_field_values`
- No audit events written in v0.1 (keeps endpoint side-effect free)
- No core TMS entity mutations

---

## What Is Not Implemented Yet

| Item | Status |
|------|--------|
| Batch/entity execute migration API enhancement | Not in this pack |
| UI preview modal | Not implemented |
| Audit event `CUSTOM_FIELD_VALUES_MIGRATION_PREVIEWED` | Skipped in v0.1 |
| Preview token / session binding for execute | Not implemented |
| Automatic migration on publish | Intentionally excluded |

Existing entity-level execute API remains:

```http
POST /api/v1/low-code/admin/custom-field-values/migrate-to-active
```

---

## Verification Commands

```powershell
cd D:\Projects\freight-platform

make platform-build-service SERVICE=low-code-service
make platform-up-no-build
make health-check
make seed-lowcode-demo

curl.exe -X POST `
  -H "Content-Type: application/json" `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  --data-binary "@scripts/dev/payloads/lowcode_migration_preview_transport_order.json" `
  http://localhost:8080/api/v1/low-code/admin/custom-field-values/migration-preview

cd services/low-code-service
go test ./...

make integration-smoke-test
```

Expected curl result: HTTP 200, `summary.entities_checked = 1`, `target_template.code = transport_order_default`, per-entity status `SAFE` or `WARNING` depending on demo data.

---

## Next Action

**Low-code Migrate-to-Active Execute Enhancement Pack v0.1** — enhance entity-level migrate-to-active with preview response shape, dedicated audit events, and soft warnings; then admin UI preview modal.

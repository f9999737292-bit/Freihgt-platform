# Low-code Migrate-to-Active Execute API v0.1

Date: 2026-06-23  
Project: `D:\Projects\freight-platform`  
Related:

- `docs/LOW_CODE_MIGRATE_TO_ACTIVE_DESIGN_V0.1.md`
- `docs/LOW_CODE_MIGRATE_TO_ACTIVE_PREVIEW_API_V0.1.md`

---

## Summary

Enhanced entity-level execute API for migrating custom field values onto the active published form template. Execute reuses the preview matching engine, supports warning confirmation via `allow_warnings`, writes dedicated migration audit events, and preserves legacy values when schema constraints allow.

---

## Endpoint

```http
POST /v1/low-code/admin/custom-field-values/migrate-to-active
POST /api/v1/low-code/admin/custom-field-values/migrate-to-active
```

Headers:

- `X-Tenant-ID` — required
- `X-User-ID` — optional (audit actor)
- `X-Request-ID` — optional (audit correlation)
- `Content-Type: application/json`

---

## Request

```json
{
  "entity_type": "TRANSPORT_ORDER",
  "entity_id": "2db04b49-665c-469f-bcb1-ffeb1274fedb",
  "template_code": "transport_order_default",
  "target_template_id": "optional-uuid",
  "allow_warnings": true,
  "code": "transport_order_default"
}
```

| Field | Rule |
|-------|------|
| `entity_type` | Required |
| `entity_id` | Required UUID |
| `template_code` | Optional if `target_template_id` provided; required when multiple active templates exist |
| `target_template_id` | Optional published template UUID (tenant-scoped) |
| `allow_warnings` | Default `false`; must be `true` to execute when preview status is `WARNING` |
| `code` | Legacy alias for `template_code` (backward compatible) |

---

## Response

Success (`200`):

```json
{
  "status": "migrated",
  "tenant_id": "74519f22-ff9b-4a8b-8fff-a958c689682f",
  "entity_type": "TRANSPORT_ORDER",
  "entity_id": "2db04b49-665c-469f-bcb1-ffeb1274fedb",
  "active_template_id": "...",
  "target_template_id": "...",
  "source_template_id": "...",
  "migrated_count": 3,
  "skipped_count": 0,
  "skipped_fields": [],
  "copied_fields": ["cargo_class", "internal_cost_center"],
  "legacy_fields": [],
  "missing_required_fields": [],
  "incompatible_fields": [],
  "warnings": []
}
```

Status values:

| Status | Meaning |
|--------|---------|
| `migrated` | Preview status was `SAFE` |
| `migrated_with_warnings` | Preview status was `WARNING` and `allow_warnings=true` |

Backward-compatible fields retained: `active_template_id`, `migrated_count`, `skipped_count`, `skipped_fields`.

Warning blocked (`409`):

```json
{
  "error": {
    "code": "MIGRATION_WARNINGS_REQUIRE_CONFIRMATION",
    "message": "migration has warnings and requires allow_warnings=true",
    "details": {},
    "preview": { "...": "same shape as migration-preview response for entity" }
  }
}
```

Blocked (`409`):

```json
{
  "error": {
    "code": "MIGRATION_BLOCKED",
    "message": "migration is blocked by incompatible fields",
    "details": {},
    "preview": { "...": "..." }
  }
}
```

---

## Execution Rules

1. Resolve target published template (active or explicit `target_template_id`).
2. Build preview item using the same logic as `migration-preview`.
3. Apply status gate:
   - `SAFE` → migrate
   - `WARNING` + `allow_warnings=false` → 409, no writes
   - `WARNING` + `allow_warnings=true` → migrate compatible fields
   - `BLOCKED` → 409, no writes
4. For each `copied_fields` entry:
   - remap to target `field_id` + `form_template_id`
   - preserve `value_json` (with deterministic transforms when preview allows)
5. Legacy field rows (field codes removed from target template) are **not deleted**.
6. No core TMS entity columns are modified.

---

## Warning Handling

Warnings include:

- legacy fields present
- missing required target fields after simulated copy
- soft type-conversion warnings (e.g. TEXT → SELECT)

When warnings exist, execute requires explicit `allow_warnings: true`.

---

## Blocked Migration

Blocked when preview finds `incompatible_fields` or values fail target validation. No database writes and no audit event in v0.1.

---

## Idempotency

Repeated execute on already-migrated values is safe:

- Preview re-evaluates current rows
- Compatible fields are replaced via delete-by-`field_code` + insert with target IDs
- No duplicate active values for the same `field_code`

---

## Audit Event

On successful execute with writes:

| Field | Value |
|-------|-------|
| Event kind | `CUSTOM_FIELD_VALUES_MIGRATED_TO_ACTIVE` |
| Payload | `source_template_id`, `target_template_id`, `copied_fields`, `legacy_fields`, `missing_required_fields`, `incompatible_fields`, `warnings`, `allow_warnings`, `status` |
| Actor | `X-User-ID` when provided |
| Request ID | `X-Request-ID` when provided |

Audit is written in the same transaction as value migration writes.

Query example:

```powershell
curl.exe -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  "http://localhost:8080/api/v1/low-code/audit-events?entity_type=TRANSPORT_ORDER&entity_id=2db04b49-665c-469f-bcb1-ffeb1274fedb&limit=10"
```

---

## Tenant Isolation

All template lookups and value reads/writes are scoped by `X-Tenant-ID`. Cross-tenant template access returns `TENANT_MISMATCH` / not found.

---

## Schema Limitations

Unique constraint: `(tenant_id, entity_type, entity_id, field_id)`.

Execute behavior:

- Matched `field_code` rows are deleted by `field_code` then re-inserted with target `field_id` (same logical field, new version binding).
- Legacy/orphan rows (field codes not on target template) remain in place and stay readable via GET.
- No legacy marker column exists in v0.1 schema — legacy detection is preview/response metadata only.

---

## What Is Not Implemented Yet

| Item | Status |
|------|--------|
| Batch execute API | Not implemented |
| Admin UI preview modal | Not implemented |
| Preview token / session binding | Not implemented |
| Audit on blocked no-write path | Skipped in v0.1 |
| Automatic migration on publish | Intentionally excluded |

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
  --data-binary "@scripts/dev/payloads/lowcode_migrate_to_active_transport_order.json" `
  http://localhost:8080/api/v1/low-code/admin/custom-field-values/migrate-to-active

cd services/low-code-service
go test ./...

make integration-smoke-test
```

---

## Next Action

**Low-code Migrate-to-Active Admin UI Preview Modal Pack v0.1**

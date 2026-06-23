# Low-code Batch Migration Preview API v0.1

Date: 2026-06-23  
Project: `D:\Projects\freight-platform`  
Related:

- `docs/LOW_CODE_BATCH_MIGRATION_DESIGN_V0.1.md`
- `docs/LOW_CODE_MIGRATE_TO_ACTIVE_PREVIEW_API_V0.1.md`

---

## Summary

Read-only admin API that previews how custom field values for up to **100 entities** would remap onto the current active published form template. The endpoint reuses the existing single/bulk migration preview service logic (`PreviewMigrationToActive`) and returns a batch-oriented summary (`total`, `safe`, `warnings`, `blocked`) plus per-entity items.

**Batch preview only.** No execute, no writes, no audit events in v0.1.

---

## Endpoint

```http
POST /v1/low-code/admin/custom-field-values/batch-migration-preview
POST /api/v1/low-code/admin/custom-field-values/batch-migration-preview
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
  "target_template_id": null
}
```

| Field | Rule |
|-------|------|
| `entity_type` | Required; validated against allowed entity types |
| `entity_ids` | Required; 1..100 UUIDs |
| `template_code` | Required when multiple active published templates exist for `entity_type`; optional when `target_template_id` is provided or resolution is unambiguous |
| `target_template_id` | Optional; must belong to tenant and be `PUBLISHED` |

---

## Response

```json
{
  "tenant_id": "74519f22-ff9b-4a8b-8fff-a958c689682f",
  "entity_type": "TRANSPORT_ORDER",
  "template_code": "transport_order_default",
  "target_template": {
    "id": "...",
    "code": "transport_order_default",
    "version": 1
  },
  "summary": {
    "total": 3,
    "safe": 2,
    "warnings": 1,
    "blocked": 0
  },
  "items": [
    {
      "entity_id": "2db04b49-665c-469f-bcb1-ffeb1274fedb",
      "source_template_id": "...",
      "target_template_id": "...",
      "status": "SAFE",
      "copied_fields": [],
      "legacy_fields": [],
      "missing_required_fields": [],
      "incompatible_fields": [],
      "warnings": []
    }
  ]
}
```

Per-entity `items` use the same structure as `POST .../migration-preview`. Batch summary field names differ from the single preview endpoint (`entities_checked` / `safe_to_migrate` → `total` / `safe`).

---

## Validation

- `X-Tenant-ID` required
- `entity_type` required and valid
- `entity_ids` required; length 1..100
- Each `entity_id` must be a UUID
- `target_template_id` must belong to tenant and be `PUBLISHED`
- Tenant isolation enforced on template and value reads

---

## Template Resolution

Same rules as migration preview:

1. If `target_template_id` is set, load that published template for the tenant.
2. Otherwise resolve active published template by `entity_type` + `template_code`.
3. If multiple active templates exist and `template_code` is omitted, return validation error.

---

## Status Summary

| Summary field | Meaning |
|---------------|---------|
| `total` | Number of entities checked |
| `safe` | Items with status `SAFE` |
| `warnings` | Items with status `WARNING` |
| `blocked` | Items with status `BLOCKED` |

Per-entity status priority (unchanged from preview engine):

1. `BLOCKED` if any incompatible fields
2. `WARNING` if legacy fields, missing required fields, or warnings
3. `SAFE` otherwise

---

## Side Effects

**None.**

- No updates to `lowcode.custom_field_values`
- No audit events in v0.1 (preview is side-effect free)
- No core TMS entity mutations

---

## Tenant Isolation

- Every request requires `X-Tenant-ID`.
- Target template lookup is tenant-scoped.
- Value reads are tenant-scoped.
- Cross-tenant template access returns `TENANT_MISMATCH` / not found.

---

## Safety Limits

| Limit | Value |
|-------|-------|
| Max `entity_ids` per request | 100 (sync) |
| Writes | None |
| Audit | None in v0.1 |

Reuses existing preview matching engine — no duplicated field compatibility logic.

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
  --data-binary "@scripts/dev/payloads/lowcode_batch_migration_preview_transport_order.json" `
  http://localhost:8080/api/v1/low-code/admin/custom-field-values/batch-migration-preview

cd services\low-code-service
go test ./...

make integration-smoke-test
```

Expected curl result: HTTP 200, `summary.total = 1`, `items` length 1, `target_template` present, no writes.

Dev payload: `scripts/dev/payloads/lowcode_batch_migration_preview_transport_order.json`

---

## What Is Not Implemented Yet

| Item | Status |
|------|--------|
| Batch migration execute API | Future pack |
| Admin UI batch wizard | Future pack |
| Batch audit events / `batch_id` | Future pack |
| Background job for >100 entities | Future (v0.2+) |
| Audit event on preview | Skipped in v0.1 |

Existing endpoints unchanged:

- `POST .../migration-preview` — same contract (`entities_checked`, `safe_to_migrate`, …)
- `POST .../migrate-to-active` — single-entity execute

---

## Next Action

**Low-code Batch Migration Execute API Pack v0.1** — batch execute endpoint with partial success, `allow_warnings` / `skip_blocked`, and batch audit correlation.

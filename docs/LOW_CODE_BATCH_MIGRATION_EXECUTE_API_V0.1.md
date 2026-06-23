# Low-code Batch Migration Execute API v0.1

Date: 2026-06-23  
Project: `D:\Projects\freight-platform`  
Related:

- `docs/LOW_CODE_BATCH_MIGRATION_DESIGN_V0.1.md`
- `docs/LOW_CODE_BATCH_MIGRATION_PREVIEW_API_V0.1.md`
- `docs/LOW_CODE_MIGRATE_TO_ACTIVE_EXECUTE_V0.1.md`

---

## Summary

Admin API for synchronous batch migration of custom field values onto the active published form template for up to **100 entities**. The endpoint runs batch preview internally, then reuses the existing single-entity `MigrateToActiveTemplate` logic per eligible entity. Supports partial success, `allow_warnings` / `skip_blocked` policies, per-request `batch_id`, and entity-level audit events.

**Synchronous batch execute only.** No background job, no Batch UI, no DB migrations in v0.1.

---

## Endpoint

```http
POST /v1/low-code/admin/custom-field-values/batch-migrate-to-active
POST /api/v1/low-code/admin/custom-field-values/batch-migrate-to-active
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
  "target_template_id": null,
  "allow_warnings": false,
  "skip_blocked": true
}
```

| Field | Rule |
|-------|------|
| `entity_type` | Required; validated against allowed entity types |
| `entity_ids` | Required; 1..100 UUIDs |
| `template_code` | Required when multiple active published templates exist |
| `target_template_id` | Optional; must belong to tenant and be `PUBLISHED` |
| `allow_warnings` | Default `false`; must be `true` to migrate WARNING entities |
| `skip_blocked` | Default `true`; when `false`, any BLOCKED entity fails the batch before writes |

---

## Response

### Success / partial success (HTTP 200)

```json
{
  "batch_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "completed",
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
    "migrated": 2,
    "skipped": 1,
    "blocked": 0,
    "failed": 0,
    "warnings": 1
  },
  "items": [
    {
      "entity_id": "...",
      "status": "migrated",
      "preview_status": "SAFE",
      "copied_fields": [],
      "legacy_fields": [],
      "missing_required_fields": [],
      "incompatible_fields": [],
      "warnings": []
    }
  ]
}
```

Batch-level `status`:

| Status | Meaning |
|--------|---------|
| `completed` | All eligible entities migrated |
| `partially_completed` | Some migrated, some skipped/failed |
| `failed` | Zero entities migrated |
| `blocked` | All entities blocked |

Per-entity item `status`: `migrated`, `migrated_with_warnings`, `skipped`, `failed`.

### Blocked batch (HTTP 409) when `skip_blocked=false`

```json
{
  "error": {
    "code": "BATCH_MIGRATION_BLOCKED",
    "message": "Batch contains blocked entities.",
    "preview": {}
  }
}
```

### Warnings require confirmation (HTTP 409) when all entities are WARNING and `allow_warnings=false`

```json
{
  "error": {
    "code": "BATCH_MIGRATION_WARNINGS_REQUIRE_CONFIRMATION",
    "message": "Batch contains warning entities and requires allow_warnings=true.",
    "preview": {}
  }
}
```

When SAFE and WARNING entities are mixed with `allow_warnings=false`, SAFE entities migrate and WARNING entities are skipped (`partially_completed`, HTTP 200).

---

## Execution Rules

1. Run batch preview internally (`PreviewMigrationToActive`).
2. Apply batch-level gates (`skip_blocked`, all-WARNING confirmation).
3. For each entity, reuse `MigrateToActiveTemplate` (same matching, validation, writes).
4. Per-entity transaction — one failure does not roll back others.
5. No silent deletion; legacy fields remain readable.
6. Core entity data/status unchanged.

| Preview status | `allow_warnings=false` | `allow_warnings=true` |
|----------------|------------------------|----------------------|
| SAFE | Migrate | Migrate |
| WARNING | Skip (partial) or 409 (all WARNING) | Migrate |
| BLOCKED | Skip if `skip_blocked=true`; 409 if `skip_blocked=false` | Never migrate |

---

## Warning Handling

- `allow_warnings=false`: WARNING entities are skipped in mixed batches; all-WARNING batches return `409 BATCH_MIGRATION_WARNINGS_REQUIRE_CONFIRMATION` before writes.
- `allow_warnings=true`: WARNING entities migrate with `migrated_with_warnings` item status.

---

## Blocked Handling

- BLOCKED entities are **never** migrated.
- `skip_blocked=false` + any BLOCKED → `409 BATCH_MIGRATION_BLOCKED` with full batch preview, **no writes**.
- `skip_blocked=true` (default) → BLOCKED entities skipped, eligible entities continue.

---

## Partial Success

Summary fields:

| Field | Description |
|-------|-------------|
| `total` | Entities in request |
| `migrated` | Successfully written |
| `skipped` | Not migrated (blocked or warning without confirmation) |
| `blocked` | Skipped due to BLOCKED preview status |
| `failed` | Unexpected per-entity errors |
| `warnings` | WARNING entities migrated or skipped |

---

## Idempotency

Repeated batch execute on already-migrated entities is stable (reuses single-entity idempotent execute path). No duplicate value rows.

---

## Batch ID

Server generates a UUID `batch_id` per request. Included in:

- HTTP response
- Entity-level audit payload (`batch_id`, `template_code`, `skip_blocked`, …)

No DB table for batch tracking in v0.1.

---

## Audit

**Entity-level audit is the source of truth.**

Each successfully migrated entity writes:

`CUSTOM_FIELD_VALUES_MIGRATED_TO_ACTIVE`

Audit payload includes:

- `batch_id`
- `entity_type`, `entity_id`
- `template_code`
- `target_template_id`, `source_template_id`
- `copied_fields`, `legacy_fields`, `missing_required_fields`, `incompatible_fields`, `warnings`
- `allow_warnings`, `skip_blocked`
- `request_id`, actor (from request audit context)

Skipped/blocked entities: **no audit event**.

Batch-level audit events (`CUSTOM_FIELD_VALUES_BATCH_MIGRATION_STARTED` / `COMPLETED`) are **deferred** — no schema/migration changes in v0.1.

---

## Tenant Isolation

- `X-Tenant-ID` required.
- Template and value reads tenant-scoped.
- Cross-tenant access returns `TENANT_MISMATCH` / not found.

---

## Safety Limits

| Limit | Value |
|-------|-------|
| Max `entity_ids` | 100 (sync) |
| Background jobs | Not in v0.1 |
| DB migrations | None |

---

## Side Effects

- Writes to `lowcode.custom_field_values` for migrated entities only.
- Entity-level audit events for migrated entities.
- No core TMS entity mutations.

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
  --data-binary "@scripts/dev/payloads/lowcode_batch_migrate_to_active_transport_order.json" `
  http://localhost:8080/api/v1/low-code/admin/custom-field-values/batch-migrate-to-active

curl.exe -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  "http://localhost:8080/api/v1/low-code/audit-events?entity_type=TRANSPORT_ORDER&entity_id=2db04b49-665c-469f-bcb1-ffeb1274fedb&limit=10"

cd services\low-code-service
go test ./...

make integration-smoke-test
```

Dev payload: `scripts/dev/payloads/lowcode_batch_migrate_to_active_transport_order.json`

---

## What Is Not Implemented Yet

| Item | Status |
|------|--------|
| Admin UI batch wizard | Future pack |
| Batch-level audit events | Deferred |
| Background job for >100 entities | Future (v0.2+) |
| Strict preview-before-execute token | Future |

Existing endpoints unchanged:

- `POST .../migration-preview`
- `POST .../batch-migration-preview`
- `POST .../migrate-to-active`

---

## Next Action

**Admin UI Batch Migration Wizard Pack v0.1** — table selection, batch preview summary, confirmation, execute with `allow_warnings` / `skip_blocked`.

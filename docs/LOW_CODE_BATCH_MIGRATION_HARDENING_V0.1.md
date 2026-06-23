# Low-code Batch Migration Hardening v0.1

## Summary

Hardening pass for admin batch migration (preview + execute) without changing core migration rules or breaking successful API response shapes. Focus: predictable validation, duplicate ID handling, guardrails, error consistency, observability safety, UI defensive behavior, and an operational runbook.

## Scope

**In scope**

- `services/low-code-service` batch preview/execute handlers and service entry points
- Prometheus metrics and structured logs for batch migration
- `LowCodeBatchMigrationWizard.vue` defensive UX
- Operational documentation

**Out of scope**

- Core per-entity migration execution logic
- Database migrations
- Batch background jobs
- New large UI features
- API contract changes for successful responses

## Backend Hardening

| Input / condition | Behavior |
|-------------------|----------|
| Duplicate `entity_ids` | Deduplicated via `domain.NormalizeBatchEntityIDs` (first-seen order preserved) in handler parsing and `PreviewMigrationToActive` |
| More than 100 `entity_ids` | `400` validation error (`entity_ids` max 100) in preview and execute (execute runs preview first) |
| Empty `entity_ids` | Stable validation error |
| Invalid UUID | Stable validation error (`ENTITY_ID_INVALID` / field `entity_id`) |
| Ambiguous template (multiple active, no `template_code`) | Stable validation error |
| `target_template_id` from another tenant | Forbidden / not found (existing tenant isolation) |
| Target template not `PUBLISHED` | Stable validation error (`FORM_TEMPLATE_NOT_PUBLISHED`) |
| Warnings-only batch, `allow_warnings=false` | `409` `BATCH_MIGRATION_WARNINGS_REQUIRE_CONFIRMATION` |
| Any blocked entity, `skip_blocked=false` | `409` `BATCH_MIGRATION_BLOCKED` |
| Full-batch blocking decisions | No writes before preview gate passes (409 paths return before migrate loop) |
| Per-entity transactions | Each entity migrated in its own store write; failure on one entity does not roll back others (documented, tested) |

## Error Consistency

Batch migration errors use the project `AppError` JSON envelope:

| Code | HTTP | When |
|------|------|------|
| `VALIDATION_ERROR` | 400 | Empty IDs, invalid UUID, max count, ambiguous template, not published |
| `NOT_FOUND` / `FORM_TEMPLATE_NOT_FOUND` | 404 | Unknown template |
| `FORBIDDEN` / `TENANT_ACCESS_DENIED` | 403 | Cross-tenant template |
| `BATCH_MIGRATION_BLOCKED` | 409 | `skip_blocked=false` and batch has blocked entities |
| `BATCH_MIGRATION_WARNINGS_REQUIRE_CONFIRMATION` | 409 | Warning-only batch without `allow_warnings=true` |

409 responses may include a `preview` payload for client recovery (unchanged contract).

## Idempotency

Re-executing batch migration on already-migrated entities:

- Does **not** duplicate stored custom field values (same as single-entity migrate)
- May create additional audit rows per execute
- Each execute generates a new `batch_id` unless supplied by caller

Safe retry: run **preview** again, confirm statuses, then execute with appropriate `allow_warnings` / `skip_blocked`.

## Duplicate Entity IDs

**Behavior (v0.1):** duplicate UUIDs in the request array are deduplicated before processing.

Example: `["id-a", "id-b", "id-a"]` → processes `id-a` once, then `id-b`.

Implementation: `internal/domain/batch_entity_ids.go` → `NormalizeBatchEntityIDs`, called from handler `parseBatchEntityIDs` and `PreviewMigrationToActive`.

## Guardrails

- Max **100** entities per request (`domain.MaxMigrationPreviewEntityCount`)
- Enforced in service layer (single source of truth for preview; execute delegates to preview)
- UI mirrors limit via `MAX_BATCH_MIGRATION_ENTITIES`

## Observability

**Metrics** (`internal/platform/metrics/batch_migration.go`):

- Labels: `entity_type`, `operation`, `status` only
- **Never** use `tenant_id`, `entity_id`, `batch_id`, or `value_json` as metric labels
- Test: `TestBatchMigrationMetricsUseBoundedLabels`

**Structured logs** (`internal/platform/batchmigration/log.go`):

- Log `batch_id`, counts, template code, request id — not raw request bodies or `value_json`
- Tests: `log_test.go`

## UI Hardening

`LowCodeBatchMigrationWizard.vue`:

- Fallbacks for missing summary, items, status, `batch_id` (`—`)
- Error responses with embedded preview normalized via `normalizeBatchMigrationPreviewResponse`
- Long UUIDs: `word-break: break-all` on mono cells
- Execute disabled while request in flight (`loadingPreview` / `executing`)
- Double-click protection on preview and execute
- Retry preview button on error panel and step 2
- Safe JSON via `formatJsonValue` in `<pre>` — no `v-html`

## Operational Runbook

### Preview a batch

```powershell
cd D:\Projects\freight-platform
curl.exe -X POST `
  -H "Content-Type: application/json" `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  --data-binary "@scripts/dev/payloads/lowcode_batch_migration_preview_transport_order.json" `
  http://localhost:8080/api/v1/low-code/admin/custom-field-values/batch-migration-preview
```

Review `summary` (`safe`, `warnings`, `blocked`) and per-item `status`.

### Execute a batch

```powershell
curl.exe -X POST `
  -H "Content-Type: application/json" `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  --data-binary "@scripts/dev/payloads/lowcode_batch_migrate_to_active_transport_order.json" `
  http://localhost:8080/api/v1/low-code/admin/custom-field-values/batch-migrate-to-active
```

Set `allow_warnings: true` when preview shows WARNING-only entities. Set `skip_blocked: true` (default) to skip BLOCKED rows.

### Audit by `batch_id`

1. Note `batch_id` from execute response.
2. Admin UI: `http://localhost:3000/low-code/audit?category=batch_migrations&batch_id=<BATCH_ID>`
3. Or filter audit events client-side after loading `CUSTOM_FIELD_VALUES_MIGRATED_TO_ACTIVE` events.

Each migrated entity emits an audit row with `batch_id` in payload metadata.

### Metrics

After preview/execute, check Prometheus scrape (platform metrics endpoint):

- `lowcode_batch_migration_preview_total{entity_type,status}`
- `lowcode_batch_migration_execute_total{entity_type,status}`
- `lowcode_batch_migration_entities_total{entity_type,operation,status}`

Do not add high-cardinality labels in future changes.

### Understanding `partially_completed`

Execute status `partially_completed` means some entities migrated/skipped and at least one failed or was blocked/skipped per policy. Check `items[].status` and `summary` counts.

### When BLOCKED (`409 BATCH_MIGRATION_BLOCKED`)

- Cause: `skip_blocked=false` but preview contains BLOCKED entities.
- Action: Re-run preview; either fix data, migrate entities individually, or execute with `skip_blocked=true` to skip blocked rows.

### When WARNING (`409 BATCH_MIGRATION_WARNINGS_REQUIRE_CONFIRMATION`)

- Cause: All migratable entities are WARNING status and `allow_warnings=false`.
- Action: Confirm legacy-field acceptance; re-execute with `allow_warnings=true`.

### Safe batch retry

1. Preview again (idempotent read).
2. Adjust flags (`allow_warnings`, `skip_blocked`).
3. Execute — expect new `batch_id`.
4. Do **not** manually edit `custom_field_values` or audit tables.

### Do not do manually in the database

- Do not DELETE/UPDATE migration audit events to “fix” batch state
- Do not bulk-update `custom_field_values` to simulate migration
- Do not reuse another tenant’s template IDs
- Use API + wizard only; rollback is re-migration or restore from backup

## Rollback Notes

This pack does not add migrations. Roll back by reverting the hardening commit. Duplicate-ID behavior changes from “process twice” to “dedupe” — clients sending duplicates will see lower counts (correct behavior).

## Verification Commands

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo

cd services\low-code-service
go test ./...

# After backend changes:
cd D:\Projects\freight-platform
make platform-build-service SERVICE=low-code-service
make platform-up-no-build
make health-check

cd apps\web-admin
npm run build

cd D:\Projects\freight-platform
make integration-smoke-test
```

## Known Limitations

- Duplicate IDs deduped in request only — same entity in two separate batch executes still runs twice
- No batch-level single audit event (per-entity audit rows only)
- No async/queued batch jobs
- Mixed SAFE/WARNING/BLOCKED demo curl may need test fixtures

## What Is Not Implemented Yet

- Batch background jobs
- Batch-level audit completion event
- Admin Batch Migration Polish Pack (UX polish)
- Live Docker integration test suite beyond smoke test

## Next Action

**Low-code Admin Batch Migration Polish Pack v0.1**

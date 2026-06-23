# Low-code Batch Migration Audit & Metrics v0.1

## Summary

This pack improves observability for low-code batch migration without changing migration execution rules, API contracts, or database schema. Entity-level audit events remain the source of truth; batch-level audit is deferred.

## Scope

- Enhanced `CUSTOM_FIELD_VALUES_MIGRATED_TO_ACTIVE` audit JSON payload for batch execute
- Prometheus metrics and structured logs for batch preview/execute handlers
- Web-admin audit UI: batch ID display, batch metadata, batch migrations quick filter, batch_id query param
- Batch wizard: **View batch audit** link with `batch_id` + `category=batch_migrations`
- RU/EN/ZH i18n for new UI strings

Out of scope: migrations, batch background jobs, core business logic changes, batch-level audit events.

## Audit Enhancements

Entity-level migration audit payload now includes (when available):

| Field | Description |
|-------|-------------|
| `batch_id` | Batch execute correlation ID |
| `template_code` | Requested or resolved template code |
| `target_template_id` | Active target template |
| `source_template_id` | Source template per entity |
| `active_template_id` | Same as target (compatibility alias) |
| `copied_fields`, `legacy_fields`, `missing_required_fields`, `incompatible_fields` | Migration field summary |
| `allow_warnings`, `skip_blocked` | Execute options |
| `preview_status` | Preview status (`SAFE` / `WARNING` / `BLOCKED`) |
| `migration_status` | Execute outcome (`migrated`, `migrated_with_warnings`) |
| `migrated_count`, `skipped_count` | Per-entity counts |
| `status` | Legacy alias of preview status (backward compatible) |

`actor` and `request_id` remain top-level audit event fields (not duplicated in JSON payload).

## Entity-level Audit Source of Truth

Each migrated entity writes one `CUSTOM_FIELD_VALUES_MIGRATED_TO_ACTIVE` audit row. Batch correlation is via shared `batch_id` in payload. Operators filter audit UI by `batch_id` to see all entities in a batch.

## Batch ID

- Generated in handler on execute (`uuid.New()`)
- Returned in execute API response as `batch_id`
- Stored in entity audit payload for every successfully migrated entity
- Not assigned on preview (preview logs omit `batch_id`)

## Metrics

Prometheus metrics registered in `low-code-service` (`internal/platform/metrics/batch_migration.go`):

| Metric | Labels | Notes |
|--------|--------|-------|
| `lowcode_batch_migration_preview_total` | `entity_type`, `status` | `success`, `warnings`, `blocked`, `error` |
| `lowcode_batch_migration_execute_total` | `entity_type`, `status` | `completed`, `partially_completed`, `blocked`, `failed`, `error` |
| `lowcode_batch_migration_entities_total` | `entity_type`, `operation`, `status` | Per-entity outcomes on execute |
| `lowcode_batch_migration_blocked_total` | `entity_type` | Blocked entities on execute |
| `lowcode_batch_migration_failed_total` | `entity_type` | Failed entities on execute |
| `lowcode_batch_migration_duration_seconds` | `entity_type`, `operation`, `status` | `operation` = `preview` or `execute` |

`tenant_id` is intentionally **not** a label (high-cardinality guardrail).

## Structured Logging

Safe structured logs via `internal/platform/batchmigration/log.go`:

- Preview: `operation=batch_migration_preview`, `entity_type`, `template_code`, `status`, summary counts, optional `request_id`
- Execute: `operation=batch_migration_execute`, `batch_id`, `entity_type`, `template_code`, `status`, summary counts, optional `request_id`

Not logged: tenant secrets, `value_json`, personal data, raw field values.

## Audit UI Enhancements

- `LowCodeMigrationAuditCard`: monospace copyable `batch_id`, template code, preview/migration status, counts, skip_blocked
- Audit page quick filter **Batch migrations** (`category=batch_migrations`)
- Frontend filter: `action=CUSTOM_FIELD_VALUES_MIGRATED_TO_ACTIVE` + `payload.batch_id` present
- `batch_id` input + query param support

## Batch Wizard Integration

After execute, **View batch audit** links to:

```text
/low-code/audit?category=batch_migrations&batch_id=<uuid>&entity_type=<type>
```

## i18n

RU/EN/ZH keys added for batch audit UI (Batch ID, View batch audit, Batch migrations, metadata labels, source-of-truth note).

## Safety Guardrails

- No API contract changes
- No DB migrations
- No batch-level audit event type (would require schema/product decision)
- Metrics avoid tenant_id labels
- UI uses safe `<pre>` JSON rendering only (no `v-html`)

## What Is Deferred

- `CUSTOM_FIELD_VALUES_BATCH_MIGRATION_COMPLETED` batch-level audit event (requires migration / product sign-off)
- Batch background jobs
- Per-tenant metrics dashboards

## Verification Commands

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo

cd services\low-code-service
go test ./...

cd D:\Projects\freight-platform
make platform-build-service SERVICE=low-code-service
make platform-up-no-build
make health-check

curl.exe -X POST `
  -H "Content-Type: application/json" `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  --data-binary "@scripts/dev/payloads/lowcode_batch_migrate_to_active_transport_order.json" `
  http://localhost:8080/api/v1/low-code/admin/custom-field-values/batch-migrate-to-active

curl.exe -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  "http://localhost:8080/api/v1/low-code/audit-events?entity_type=TRANSPORT_ORDER&entity_id=2db04b49-665c-469f-bcb1-ffeb1274fedb&limit=10"

cd apps\web-admin
npm run build

make integration-smoke-test
```

Manual UI:

1. Open `/low-code/custom-field-values`
2. Run batch migration wizard
3. Click **View batch audit**
4. Verify batch_id and metadata on audit cards

## Next Action

**Low-code Batch Migration Edge Cases Test Pack v0.1**

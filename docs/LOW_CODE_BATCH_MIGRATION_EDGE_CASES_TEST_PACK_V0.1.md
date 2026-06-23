# Low-code Batch Migration Edge Cases Test Pack v0.1

## Summary

This pack adds automated edge-case coverage and curl payload examples for batch migration preview/execute without changing migration business rules, API contracts, or database schema.

## Scope

- Extended Go unit/service/handler tests for batch preview and execute
- Structured logging sanity tests (no sensitive fields)
- Metrics registration test (from audit/metrics pack)
- Dev curl payload examples under `scripts/dev/payloads/batch-migration-edge-cases/`
- Minor defensive UI fixes in batch migration wizard
- Documentation only — no new product features

## Backend Edge Cases Covered

| Area | Coverage |
|------|----------|
| Preview validation | empty IDs, >100 IDs, invalid UUID, invalid entity type, tenant required |
| Preview behavior | SAFE/WARNING/BLOCKED summary, no writes, duplicate entity IDs (not deduplicated) |
| Execute SAFE | completed status, batch_id, audit metadata |
| Execute WARNING | allow_warnings=false → 409 whole-batch or per-entity skip in mixed batch |
| Execute BLOCKED | skip_blocked=false → 409 before writes; skip_blocked=true → skip |
| Mixed batch | SAFE + WARNING + BLOCKED with allow_warnings + skip_blocked |
| Idempotency | repeated execute stable counts; one write per execute |
| Tenant isolation | preview + execute cross-tenant rejected |
| Invalid target | NOT_PUBLISHED template, ambiguous template_code |
| Audit | batch_id, template_code, skip_blocked in migration audit payload |

## Batch Preview Cases

- `TestAdminBatchMigrationPreviewEmptyEntityIDsRejected`
- `TestAdminBatchMigrationPreviewTooManyEntityIDsRejected`
- `TestAdminBatchMigrationPreviewInvalidEntityID`
- `TestBatchPreviewDuplicateEntityIDsProcessedTwice` — documents duplicate IDs produce duplicate preview items
- `TestBatchPreviewDoesNotWrite` — no repository writes on preview

## Batch Execute Cases

- `TestBatchMigrateToActiveSafeEntitiesMigrated`
- `TestAdminBatchMigrateToActiveReturnsBatchResponse`
- `TestBatchMigrateToActiveBatchIDPropagatedToAuditMetadata`

## Mixed SAFE/WARNING/BLOCKED

- `TestBatchMigrateToActivePartialSuccessSummary` — allow_warnings=true, skip_blocked=true
- `TestBatchMigrateToActivePartialSuccessSkipsWarningsWithoutAllowWarnings` — mixed SAFE + WARNING

## skip_blocked Policy

- `skip_blocked=false` + any BLOCKED → HTTP 409 `BATCH_MIGRATION_BLOCKED`, no writes
- `skip_blocked=true` → blocked entities skipped, safe/warning entities may migrate

## allow_warnings Policy

- Warning-only batch + `allow_warnings=false` → HTTP 409 `BATCH_MIGRATION_WARNINGS_REQUIRE_CONFIRMATION`
- Mixed batch + `allow_warnings=false` → safe migrated, warnings skipped per entity
- `allow_warnings=true` → warning entities migrate (possibly `migrated_with_warnings`)

## Idempotency

- `TestBatchMigrateToActiveRepeatedExecuteIsStable` — stable migrated summary
- `TestBatchMigrateRepeatedExecuteWriteCountStable` — exactly one write per execute, no value duplication in store mock

## Tenant Isolation

- `TestBatchMigrateToActiveTenantIsolation` (service)
- `TestAdminBatchMigrationPreviewTenantIsolation` (handler)
- `TestAdminBatchMigrationPreviewTargetTemplateTenantMismatch`

## Metrics and Logging Checks

Prometheus metrics (bcb43b5):

- `lowcode_batch_migration_preview_total`
- `lowcode_batch_migration_execute_total`
- `lowcode_batch_migration_entities_total`
- `lowcode_batch_migration_blocked_total`
- `lowcode_batch_migration_failed_total`
- `lowcode_batch_migration_duration_seconds`

Labels: `entity_type`, `status`, `operation` — **no `tenant_id`** (high-cardinality guardrail).

Structured logs (`internal/platform/batchmigration/log.go`):

- Safe fields only: batch_id, entity_type, template_code, summary counts, request_id, status
- Tests assert logs do not contain `value_json`, secrets, or personal data

## Frontend Defensive Checks

`LowCodeBatchMigrationWizard.vue`:

- Fallback empty execute summary
- Safe optional arrays on result items
- Unknown status renders as `—`
- Missing batch_id shows `—`

No `v-html`. Raw JSON remains in `<pre>` only.

## API Payload Examples

See `scripts/dev/payloads/batch-migration-edge-cases/README.md`.

| Payload | Purpose |
|---------|---------|
| `safe_batch_transport_order.json` | Demo SAFE batch |
| `warning_batch_allow_false_transport_order.json` | WARNING policy (409 if warning-only) |
| `warning_batch_allow_true_transport_order.json` | Execute with warnings |
| `blocked_batch_skip_false_transport_order.json` | Whole batch blocked |
| `blocked_batch_skip_true_transport_order.json` | Skip blocked entities |
| `mixed_batch_transport_order.json` | Example only — use Go tests for real mixed fixtures |
| `invalid_batch_too_many_entities.json` | 101 IDs → 400 |
| `invalid_batch_bad_uuid.json` | Invalid UUID → 400 |

## Verification Commands

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo

cd services\low-code-service
go test ./...

cd D:\Projects\freight-platform
curl.exe -X POST -H "Content-Type: application/json" -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" --data-binary "@scripts/dev/payloads/lowcode_batch_migration_preview_transport_order.json" http://localhost:8080/api/v1/low-code/admin/custom-field-values/batch-migration-preview

curl.exe -X POST -H "Content-Type: application/json" -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" --data-binary "@scripts/dev/payloads/lowcode_batch_migrate_to_active_transport_order.json" http://localhost:8080/api/v1/low-code/admin/custom-field-values/batch-migrate-to-active

cd apps\web-admin
npm run build

make integration-smoke-test
```

## Known Limitations

- Duplicate `entity_ids` in request are **not deduplicated** — each ID is processed once per array entry
- Mixed SAFE/WARNING/BLOCKED curl against demo seed may not reproduce all statuses without test fixtures
- `actor` is a top-level audit event field (when user header present), not duplicated in JSON payload
- Idempotent re-execute may create additional audit rows but does not duplicate stored field values

## What Is Not Implemented Yet

- Batch background jobs
- Batch-level audit event (`CUSTOM_FIELD_VALUES_BATCH_MIGRATION_COMPLETED`)
- Frontend automated test framework
- Dedicated integration test suite against live Docker (smoke test only)

## Next Action

**Low-code Batch Migration Hardening Pack v0.1**

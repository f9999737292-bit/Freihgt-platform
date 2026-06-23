# Low-code Migration Edge Cases Test Pack v0.1

## Summary

Added comprehensive edge-case coverage for low-code migrate-to-active preview and execute flows: domain compatibility rules, service orchestration, HTTP validation, curl payload examples, and defensive UI hardening in the migration preview modal.

No production business logic or API contract changes.

## Scope

| Area | In scope | Out of scope |
|------|----------|--------------|
| Go unit/service/handler tests | Yes | — |
| Curl payload examples | Yes | — |
| Modal defensive fixes | Yes | New UI features |
| Demo seed changes | No | Batch migration |
| DB migrations | No | Backend logic changes |

## Backend Edge Cases Covered

| Scenario | Layer | Test file |
|----------|-------|-----------|
| SAFE — same field_code/type, no legacy/missing/incompatible | domain | `migration_preview_edge_cases_test.go` |
| Legacy fields → WARNING, compatible still copied | domain + service | domain + `custom_field_value_service_execute_test.go` |
| Missing required → WARNING | domain + service | domain + `custom_field_value_service_edge_cases_test.go` |
| Incompatible MONEY↔NUMBER, DATE↔TEXT, CHECKBOX↔TEXT → BLOCKED | domain | `migration_preview_edge_cases_test.go` |
| SELECT→TEXT compatible | domain | `migration_preview_edge_cases_test.go` |
| TEXT→SELECT warning (in/out of options) | domain | `migration_preview_edge_cases_test.go` |
| Protected system/read-only fields skipped with warnings | domain | `migration_preview_edge_cases_test.go` |
| Empty values — preview safe/warning, execute no panic | service | `custom_field_value_service_edge_cases_test.go` |
| allow_warnings=false on WARNING → 409, no write | service | `custom_field_value_service_execute_test.go` |
| allow_warnings=true on WARNING → migrates compatible | service | `custom_field_value_service_execute_test.go` |
| BLOCKED → no write | service + handler | execute test + `admin_custom_field_value_handler_edge_cases_test.go` |
| Repeated execute — stable result, same field codes | service | `custom_field_value_service_edge_cases_test.go` |
| Tenant isolation preview/execute | service | preview + edge_cases tests |
| Invalid tenant / entity_id / entity_type / target_template_id | handler | `admin_custom_field_value_handler_edge_cases_test.go` |
| Ambiguous template_code when multiple active | service | `custom_field_value_service_preview_test.go` |

## API Payloads

Directory: `scripts/dev/payloads/migration-edge-cases/`

| File | Use |
|------|-----|
| `safe_transport_order.json` | SAFE preview (DEMO-TO-001) |
| `warning_transport_order_allow_false.json` | Execute without warnings |
| `warning_transport_order_allow_true.json` | Execute with warnings |
| `blocked_transport_order.json` | Blocked example (unit-tested; demo may be SAFE) |
| `empty_values_transport_order.json` | Empty entity — replace with DEMO-TO-002 id |
| `invalid_entity_id.json` | 400 ENTITY_ID_INVALID |

See `scripts/dev/payloads/migration-edge-cases/README.md` for curl commands.

## Frontend Safety Checks

No frontend test framework in project — verified via `npm run build` only.

Minimal modal hardening in `LowCodeMigrationPreviewModal.vue`:

- Fallback empty summary when preview object partial
- Safe array handling for field lists and warnings
- Optional chaining on incompatible fields

## Idempotency

Repeated execute on the same entity with unchanged compatible values:

- Produces stable `status` and `migrated_count`
- Uses `ReplaceFieldCodesBatch` (replace, not duplicate insert)
- Audit may record each successful execute; field values remain stable

Covered by `TestMigrateToActiveRepeatedExecuteIsStable`.

## Tenant Isolation

Tenant A cannot resolve tenant B active template context (`TenantMismatch` / not found). Values are always listed with request tenant ID.

Covered by preview and execute service tests.

## Warning vs Blocked Rules

| Status | Condition | Execute without `allow_warnings` | Execute with `allow_warnings=true` |
|--------|-----------|----------------------------------|-------------------------------------|
| SAFE | No legacy, missing, warnings, incompatible | Allowed | Allowed |
| WARNING | Legacy and/or missing required and/or type warnings | 409 `MIGRATION_WARNINGS_REQUIRE_CONFIRMATION` | Allowed (compatible fields copied) |
| BLOCKED | Incompatible fields | 409 `MIGRATION_BLOCKED` | Still blocked |

## Verification Commands

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo

cd services\low-code-service
go test ./...

cd D:\Projects\freight-platform\apps\web-admin
npm run build

cd D:\Projects\freight-platform
make integration-smoke-test
```

Curl sanity:

```powershell
curl.exe -X POST -H "Content-Type: application/json" -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" --data-binary "@scripts/dev/payloads/lowcode_migration_preview_transport_order.json" http://localhost:8080/api/v1/low-code/admin/custom-field-values/migration-preview

curl.exe -X POST -H "Content-Type: application/json" -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" --data-binary "@scripts/dev/payloads/lowcode_migrate_to_active_transport_order.json" http://localhost:8080/api/v1/low-code/admin/custom-field-values/migrate-to-active
```

## Known Limitations

- Demo seed does not include pre-built BLOCKED or legacy-field entities; those cases are covered by Go tests only.
- `empty_values_transport_order.json` requires resolving DEMO-TO-002 entity id at runtime.
- Audit list API still returns empty `new_values` for migration events (separate backend serialization gap; UI handles gracefully).
- `template_changes` quick filter on audit page may miss events when limit is reached before client filter.

## What Is Not Implemented Yet

- Batch migration design and UI
- Integration tests against live Docker DB for migration edge entities
- Automated frontend component tests
- Audit API migration payload exposure fix

## Next Action

**Low-code Batch Migration Design Pack v0.1**

# Low-code Template Import/Export Edge Cases v0.1

## Summary

Edge-case test coverage and defensive guardrails for template export/import APIs and Admin UI wizard. No API contract changes; tests document deterministic behavior for conflict strategies, validation failures, tenant isolation, and audit hooks.

## Scope

| Layer | In scope |
|-------|----------|
| Backend | Handler + domain tests for export, import-preview, import execute |
| UI | Defensive fixes only (no new features) |
| Dev payloads | `scripts/dev/payloads/template-import-export-edge-cases/` |
| Out of scope | Business logic changes, migrations, auto-publish, custom value import |

## Export Edge Cases

| Case | Expected |
|------|----------|
| Missing `X-Tenant-ID` | 400 `TENANT_REQUIRED` |
| Template not found / wrong tenant | 404 `FORM_TEMPLATE_NOT_FOUND` |
| PUBLISHED / DRAFT / ARCHIVED status | 200 export allowed |
| Response `schema_version` | `lowcode.template.export.v1` |
| Portable template | No section/field DB UUIDs |
| Custom field values / audit events | Not included in envelope |
| Field codes, sort order, config | Preserved in portable JSON |
| Checksum | Stable for identical portable template |
| Audit | `RecordTemplateExport` called with schema version (`FORM_TEMPLATE_EXPORTED` payload builder tested in domain) |
| Auth default-off | 200 without `X-User-ID` |

Tests: `admin_form_template_handler_test.go`, `admin_form_template_handler_import_export_edge_cases_test.go`, `form_template_export_test.go`, `form_template_import_export_edge_cases_test.go`.

## Import Preview Edge Cases

| Case | Expected |
|------|----------|
| Invalid JSON | 400 `VALIDATION_ERROR` |
| Missing / wrong `schema_version` | 400 `VALIDATION_ERROR` / `UNSUPPORTED_SCHEMA_VERSION` |
| Missing template / entity_type / code | 400 validation (after parse or draft validation) |
| Duplicate section/field code | 400 validation |
| Unsupported field type | 400 validation |
| SQL fragment in validation rule | 400 validation |
| Payload > 512 KB | 413 `IMPORT_PAYLOAD_TOO_LARGE` |
| Export `source.tenant_id` | Traceability only; target tenant from `X-Tenant-ID` |
| Preview | No `ImportTemplateAsDraft` call |
| Audit | `RecordTemplateImportPreview` on success |

## Import Execute Edge Cases

### NEW_VERSION

- When published exists: creates new DRAFT row (`ReplaceDraftID` nil); does not overwrite PUBLISHED.
- Repeated imports with same payload: each call invokes repo import (creates separate draft rows when no conflict).
- Field codes preserved in `DraftInput`; new DB field IDs assigned at repo layer (not in handler tests).
- Never auto-publishes.

### FAIL_IF_EXISTS

- When any template with target code exists: 409 `FORM_TEMPLATE_CONFLICT`, zero repo import calls.

### REPLACE_EXISTING_DRAFT

- When DRAFT exists: `ReplaceDraftID` set; PUBLISHED not overwritten.
- When only PUBLISHED exists (no DRAFT): 400 `FORM_TEMPLATE_NOT_DRAFT`, no repo write.

## Conflict Strategy Coverage

| Strategy | Preview | Execute |
|----------|---------|---------|
| `NEW_VERSION` | READY or WARNING if draft exists | 201 new draft |
| `FAIL_IF_EXISTS` | 409 if exists | 409, no write |
| `REPLACE_EXISTING_DRAFT` | 400 if no draft | 201 replace draft |

## Tenant Isolation

- Export/import use `X-Tenant-ID` from request context.
- Export file `source.tenant_id` / `source_metadata` does not override request tenant (verified via import execute stub).

## Security Guardrails

- SQL-like fragments rejected in `validation_rule_json` / visibility rules (draft validation reused for import).
- No custom field values in export or import paths.
- Max body 512 KB (handler `MaxBytesReader` + domain parse guard).
- Portable export excludes DB identifiers from template tree.

## UI Defensive Checks

| Check | Implementation |
|-------|----------------|
| Invalid JSON / wrong schema | Clear parse errors in wizard step 1 |
| Empty textarea | Preview button disabled |
| File > 512 KB | Client-side block before parse |
| Preview/execute in-flight | Buttons disabled; double-click guarded |
| Blocking errors | Execute disabled |
| Warnings | Checkbox required |
| Export double-click | `:disabled="exporting"` + early return |
| Pretty JSON | `formatTemplateExportJson` with indent 2 |
| Missing optional API fields | Optional chaining on summary/warnings |
| Safe rendering | `<pre>` only, no `v-html` |

## Dev Payloads

Directory: `scripts/dev/payloads/template-import-export-edge-cases/`

See `README.md` in that folder for placeholder IDs and curl examples.

## Verification Commands

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo

cd services\low-code-service
go test ./...

cd D:\Projects\freight-platform\apps\web-admin
npm run build

curl.exe -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  "http://localhost:8080/api/v1/low-code/admin/form-templates/b1111111-1111-4111-8111-111111111102/export"

curl.exe -X POST -H "Content-Type: application/json" `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  --data-binary "@scripts/dev/payloads/template-import-export-edge-cases/new_version_request.json" `
  "http://localhost:8080/api/v1/low-code/admin/form-templates/import-preview"

make integration-smoke-test
```

## Known Limitations

- No max field count enforced at domain layer (not implemented in v0.1).
- Handler tests use stub repo; DB-level id assignment verified at repository integration level only.
- `valid_transport_order_export_v1.json` checksum is a dev placeholder; live export checksum comes from API.
- Replace-draft payload requires real DRAFT template UUID substitution.

## What Is Not Implemented Yet

- Repository-level integration tests for import idempotency with real Postgres
- Automated Playwright/Cypress UI tests
- Bulk ZIP import/export
- Hardening pack (rate limits, stricter schema diff warnings)

## Next Action

Low-code Template Import/Export Hardening Pack v0.1

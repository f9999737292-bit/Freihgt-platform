# Low-code Template Import/Export Hardening v0.1

## Summary

Hardening pass for template export/import before staging/pilot: stable validation errors, payload limits, checksum warnings, unknown-key rejection, and UI double-click protection. No business-logic or API contract changes.

## Scope

| Area | Changes |
|------|---------|
| Backend | Top-level key allowlist, checksum warning on preview, existing payload/section/field limits |
| UI | Metadata passthrough, client key guard, in-flight button disables |
| Docs / payloads | Runbook, hardening matrix, dev JSON examples |
| Out of scope | Auto-publish, custom values import, migrations, max fields increase to 500 |

## Backend Hardening

- Import preview and execute share `ParseImportRequest` with two-phase JSON parse (keys then body).
- Forbidden top-level keys rejected before template validation.
- Export envelope fields `metadata` and `exported_at` allowed for full-envelope paste.
- Checksum verified when `metadata.checksum` present; mismatch → preview **WARNING** (execute still allowed after operator confirms warnings in UI).
- Missing checksum → allowed (manual/dev payloads).

## Payload Limits

| Limit | Value | Enforcement |
|-------|-------|-------------|
| Max body size | 512 KB | Handler `MaxBytesReader` + domain `MaxImportPayloadBytes` → `413 IMPORT_PAYLOAD_TOO_LARGE` |
| Max sections | 50 | `MaxDraftSections` in draft validation (import reuses) |
| Max fields | 200 | `MaxDraftFields` in draft validation (import reuses) |

Increasing max fields to 500 is **deferred** — existing 200 limit retained as conservative guardrail.

## Field and Section Limits

Import paths call `ValidateDraftFormTemplateInput` after export-to-draft conversion. Exceeding limits returns stable `400 VALIDATION_ERROR` with `too many sections` / `too many fields`.

## Checksum Handling

1. Export computes SHA-256 hex checksum over portable `template` JSON (`ComputeTemplateExportChecksum`).
2. Import reads optional `metadata.checksum` from payload.
3. Preview compares checksum to imported `template` block.
4. Mismatch → warning string in preview `warnings[]`, status `WARNING`.
5. No checksum → no checksum warning (backward compatible).

## Error Consistency

| Condition | HTTP | Code |
|-----------|------|------|
| Invalid JSON | 400 | `VALIDATION_ERROR` |
| Wrong/missing schema_version | 400 | `UNSUPPORTED_SCHEMA_VERSION` / `VALIDATION_ERROR` |
| Unknown top-level key | 400 | `VALIDATION_ERROR` |
| Forbidden key (`custom_values`, etc.) | 400 | `VALIDATION_ERROR` |
| Duplicate field/section code | 400 | `VALIDATION_ERROR` |
| SQL fragment in rules | 400 | `VALIDATION_ERROR` |
| Unsupported field type | 400 | `VALIDATION_ERROR` |
| Payload too large | 413 | `IMPORT_PAYLOAD_TOO_LARGE` |
| FAIL_IF_EXISTS conflict | 409 | `FORM_TEMPLATE_CONFLICT` |
| REPLACE without DRAFT | 400 | `FORM_TEMPLATE_NOT_DRAFT` |
| Wrong tenant / not found | 404 | `FORM_TEMPLATE_NOT_FOUND` |

## Security Guardrails

- SQL fragment patterns rejected in validation/visibility JSON (existing draft validation).
- No script/html evaluation; JSON parsed only.
- Target tenant from `X-Tenant-ID` only; export `source.tenant_id` ignored for writes.
- DB UUIDs in export `source` are traceability only, not import keys.
- `system_field: true` rejected unless `allow_system_fields=true`.
- No custom values, audit logs, or auto-publish on import.

## UI Hardening

- Preview/execute/export buttons disabled while in-flight.
- Double-click guards on export and import execute.
- Client-side 512 KB file limit before parse.
- Client-side top-level key allowlist mirrors backend.
- Full export envelope passes `metadata` for checksum verification.
- Checksum warnings shown in preview warnings list; checkbox required before execute.
- Wizard reset on close / Import another.
- JSON rendered in `<pre>` only.

## No-write Guarantees

- Validation/conflict errors in `prepareTemplateImport` return before repo calls.
- Import preview never calls `ImportTemplateAsDraft`.
- Import execute calls repo only after successful prepare + execution plan resolution.
- Verified by handler tests (`importCallCount` stub).

## Operational Runbook

### Export a template

1. Admin UI → `/low-code/admin/form-templates/{id}` → **Export JSON**.
2. Or curl:
   ```powershell
   curl.exe -H "X-Tenant-ID: {tenant}" `
     "http://localhost:8080/api/v1/low-code/admin/form-templates/{id}/export"
   ```
3. Save file; verify `schema_version` = `lowcode.template.export.v1`.
4. Audit: `FORM_TEMPLATE_EXPORTED` in low-code audit log.

### Import preview

1. Paste export JSON or dev payload into Admin import wizard (or POST import-preview).
2. Choose conflict strategy (`NEW_VERSION` default).
3. Review warnings (checksum mismatch, field type changes, existing draft).
4. Audit: `FORM_TEMPLATE_IMPORT_PREVIEWED` on success only.

### Import execute

1. Complete preview first with same payload/settings.
2. Confirm DRAFT-only notice; acknowledge warnings if any.
3. Click **Execute import** once (button disables while in-flight).
4. Open created/replaced DRAFT; publish manually if needed.
5. Audit: `FORM_TEMPLATE_IMPORTED_AS_DRAFT`.

### FAIL_IF_EXISTS

- Use when target code must not exist at all.
- Expect `409 FORM_TEMPLATE_CONFLICT` if any template row exists for code.
- No DB writes on conflict.

### REPLACE_EXISTING_DRAFT error

- Requires existing DRAFT for target code.
- If only PUBLISHED exists → `400 FORM_TEMPLATE_NOT_DRAFT`.
- Fix: clone to draft first, or use `NEW_VERSION`.

### Verify audit

```powershell
curl.exe -s -H "X-Tenant-ID: {tenant}" `
  "http://localhost:8080/api/v1/low-code/audit-events?limit=10"
```

Look for `FORM_TEMPLATE_EXPORTED`, `FORM_TEMPLATE_IMPORT_PREVIEWED`, `FORM_TEMPLATE_IMPORTED_AS_DRAFT`.

### Do not edit manually in DB

- Do not insert/update `lowcode.form_templates` / sections / fields directly for import flows.
- Do not set `PUBLISHED` via SQL after import — use admin publish.
- Do not import custom field values via template JSON (not supported).

## Verification Commands

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo

cd services\low-code-service
go test ./...

make platform-build-service SERVICE=low-code-service
make platform-up-no-build
make health-check

curl.exe -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  "http://localhost:8080/api/v1/low-code/admin/form-templates/b1111111-1111-4111-8111-111111111102/export"

curl.exe -X POST -H "Content-Type: application/json" `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  --data-binary "@scripts/dev/payloads/template-import-export-edge-cases/checksum_mismatch_request.json" `
  "http://localhost:8080/api/v1/low-code/admin/form-templates/import-preview"

cd apps\web-admin
npm run build

make integration-smoke-test
```

## Known Limitations

- Checksum mismatch is warning-only (not blocked) unless combined with other blocking validation.
- Max fields remains 200 (not 500).
- REPLACE_EXISTING_DRAFT without DRAFT → `FORM_TEMPLATE_NOT_DRAFT`.
- Repository-level integration tests for partial writes not included in v0.1.

## What Is Deferred

- Strict checksum block mode
- Max fields increase to 500
- Rate limiting / request throttling
- Automated UI E2E tests

## Next Action

Low-code Runtime Pilot Staging Checklist Pack v0.1

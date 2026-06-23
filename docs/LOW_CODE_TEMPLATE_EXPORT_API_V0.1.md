# Low-code Template Export API v0.1

## Summary

Admin-only GET endpoint exports a tenant-scoped low-code form template as a portable JSON envelope (`lowcode.template.export.v1`). Export includes template structure (sections, fields, validation/visibility rules, options) but excludes custom field values, audit history, secrets, and DB identifiers inside portable section/field keys.

Import preview/execute and Admin UI export button are out of scope for v0.1.

## Endpoint

| Layer | Method | Path |
|-------|--------|------|
| Service | `GET` | `/v1/low-code/admin/form-templates/{id}/export` |
| Gateway | `GET` | `/api/v1/low-code/admin/form-templates/{id}/export` |

### Headers

| Header | Required | Notes |
|--------|----------|-------|
| `X-Tenant-ID` | Yes | Tenant isolation |
| `X-User-ID` | When auth-on | Required for `PLATFORM_ADMIN` when `LOW_CODE_ADMIN_AUTH_ENABLED=true` |
| `X-Request-ID` | Optional | Echoed in `metadata.request_id` when present |

### Status rules

| Template status | Export |
|-----------------|--------|
| `DRAFT` | Allowed |
| `PUBLISHED` | Allowed |
| `ARCHIVED` | Allowed when admin detail read would succeed (same tenant lookup) |
| Other | `400 VALIDATION_ERROR` |

Wrong tenant or missing template → `404 FORM_TEMPLATE_NOT_FOUND`.

## Export Envelope

```json
{
  "schema_version": "lowcode.template.export.v1",
  "exported_at": "2026-06-24T12:00:00Z",
  "source": {
    "template_id": "b1111111-1111-4111-8111-111111111102",
    "tenant_id": "74519f22-ff9b-4a8b-8fff-a958c689682f",
    "environment": "local",
    "service": "low-code-service",
    "template_code": "transport_order_default",
    "template_version": 1,
    "template_status": "PUBLISHED"
  },
  "template": {
    "entity_type": "TRANSPORT_ORDER",
    "code": "transport_order_default",
    "name": "Transport Order Default",
    "description": "",
    "version": 1,
    "status": "PUBLISHED",
    "sections": []
  },
  "metadata": {
    "checksum": "sha256-hex-of-normalized-template-object",
    "exported_by": "optional-user-uuid",
    "request_id": "optional-correlation-id"
  }
}
```

## Portable Template Object

Sections and fields use stable logical keys only:

| Key | Purpose |
|-----|---------|
| `code` | Section or field identifier for future import |
| `title` / `label` | Display labels |
| `field_type` | Field type enum |
| `required`, `read_only`, `system_field` | Flags |
| `options_json`, `validation_rule_json`, `visibility_rule_json` | Stored configuration (passthrough, not evaluated on export) |
| `sort_order` | Ordering |

DB UUIDs for sections/fields are **not** included in `template.sections` / `template.fields`. Source DB id may appear only in `source.template_id` as metadata.

## What Is Excluded

- Custom field values (`/custom-field-values`)
- Audit log entries (`/audit-events`)
- Tenant secrets, credentials, tokens
- User personal data beyond optional `metadata.exported_by` actor id
- Raw auth headers
- SQL fragments (already rejected at draft save; export is read-only)
- Executable code / HTML rendering

## Tenant Isolation

Template is loaded with `WHERE id = $1 AND tenant_id = $2`. Cross-tenant export attempts return not found.

## Permissions

| Mode | Behavior |
|------|----------|
| `LOW_CODE_ADMIN_AUTH_ENABLED=false` (default-off dev) | Open admin routes with valid `X-Tenant-ID` |
| `LOW_CODE_ADMIN_AUTH_ENABLED=true` | `RequireLowCodeAdmin` → `PLATFORM_ADMIN` via identity service |

See `docs/LOW_CODE_PERMISSIONS_MATRIX_V0.1.md`.

## Audit

On successful export, configuration audit event:

| Field | Value |
|-------|-------|
| DB action | `TEST` (existing check constraint; no migration in v0.1) |
| Event kind (in payload) | `FORM_TEMPLATE_EXPORTED` |

Payload includes: `template_id`, `code`, `version`, `status`, `schema_version`, plus actor/request correlation from audit context.

Export body does **not** include audit events.

## Checksum

`metadata.checksum` = SHA-256 hex digest of JSON-marshaled `template` object (normalized portable shape). Enables integrity checks before import in a later pack.

## Security Guardrails

- Read-only export; no config evaluation or HTML rendering
- Unknown config keys in stored template JSON are preserved as-is (safe passthrough)
- No custom values or audit history in response
- Portable keys avoid DB ids to prevent accidental import coupling to source environment

## Verification Commands

Pre-seed:

```powershell
cd D:\Projects\freight-platform
make seed-lowcode-demo
```

Export via gateway (default-off dev, no `X-User-ID`):

```powershell
curl.exe -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  "http://localhost:8080/api/v1/low-code/admin/form-templates/b1111111-1111-4111-8111-111111111102/export"
```

Expected: HTTP 200, `schema_version = lowcode.template.export.v1`, `template.code = transport_order_default`, sections/fields present, no `values` payload.

Go tests:

```powershell
cd D:\Projects\freight-platform\services\low-code-service
go test ./...
```

## What Is Not Implemented Yet

- Import preview API
- Import execute API
- Admin UI export button
- Cross-tenant or cross-environment transfer tooling
- Export of custom field values or audit history

## Next Action

Low-code Template Import Preview API Pack v0.1

Design reference: `docs/LOW_CODE_TEMPLATE_IMPORT_EXPORT_DESIGN_V0.1.md`

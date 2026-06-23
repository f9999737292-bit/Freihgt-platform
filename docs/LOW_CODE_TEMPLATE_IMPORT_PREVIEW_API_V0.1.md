# Low-code Template Import Preview API v0.1

## Summary

Admin-only dry-run endpoint validates a portable template import payload and returns conflict resolution summary without creating or modifying templates. Reuses draft validation rules and compares field codes against the active published template when present.

Import execute and Admin UI import wizard are out of scope for v0.1.

## Endpoint

| Layer | Method | Path |
|-------|--------|------|
| Service | `POST` | `/v1/low-code/admin/form-templates/import-preview` |
| Gateway | `POST` | `/api/v1/low-code/admin/form-templates/import-preview` |

### Headers

| Header | Required | Notes |
|--------|----------|-------|
| `X-Tenant-ID` | Yes | Target tenant (never taken from export file) |
| `X-User-ID` | When auth-on | `PLATFORM_ADMIN` when `LOW_CODE_ADMIN_AUTH_ENABLED=true` |
| `Content-Type` | Yes | `application/json` |

Max body size: **512 KB**.

## Request Body

Accepts import request shape or full export envelope (`template` + optional `source`).

```json
{
  "schema_version": "lowcode.template.export.v1",
  "mode": "CREATE_DRAFT",
  "conflict_strategy": "NEW_VERSION",
  "target_code": null,
  "allow_system_fields": false,
  "template": {
    "entity_type": "TRANSPORT_ORDER",
    "code": "transport_order_default",
    "name": "Transport Order Default Form",
    "sections": []
  },
  "source_metadata": {
    "source_tenant_id": "74519f22-ff9b-4a8b-8fff-a958c689682f",
    "source_template_id": "b1111111-1111-4111-8111-111111111102",
    "source_version": 1,
    "source_status": "PUBLISHED"
  }
}
```

| Field | Default | Notes |
|-------|---------|-------|
| `mode` | `CREATE_DRAFT` | See design doc for other modes |
| `conflict_strategy` | `NEW_VERSION` | `FAIL_IF_EXISTS`, `NEW_CODE`, `REPLACE_EXISTING_DRAFT` |
| `target_code` | `template.code` | Override imported code |
| `allow_system_fields` | `false` | Reject `system_field: true` unless enabled |

Export envelope shortcut: if `source_metadata` omitted, `source` block from export is mapped automatically.

## Preview Response

```json
{
  "status": "READY",
  "conflict_strategy": "NEW_VERSION",
  "import_mode": "CREATE_DRAFT",
  "target_entity_type": "TRANSPORT_ORDER",
  "target_code": "transport_order_default",
  "existing_draft_id": null,
  "existing_published_versions": [1],
  "proposed_draft_version_on_publish": 2,
  "warnings": [],
  "validation_errors": [],
  "summary": {
    "sections_count": 1,
    "fields_count": 3,
    "new_field_codes": [],
    "removed_field_codes": [],
    "type_changes": []
  },
  "schema_version": "lowcode.template.export.v1"
}
```

| Status | Meaning |
|--------|---------|
| `READY` | Validation and strategy checks passed |
| `WARNING` | Proceed possible; review warnings (field removals/type changes/existing draft) |
| `BLOCKED` | Reserved for future use |

Hard validation failures return HTTP 400/409/413 instead of preview body.

## Conflict Strategies

| Strategy | Preview behavior |
|----------|------------------|
| `NEW_VERSION` | Allowed when published exists; warns if draft exists |
| `FAIL_IF_EXISTS` | 409 if any template with target code exists |
| `NEW_CODE` | Requires unique `target_code`; 409 if exists |
| `REPLACE_EXISTING_DRAFT` | 400 if no DRAFT with target code |

## What Is Not Written

- No template rows created/updated
- No custom field values
- No publish

## Tenant Isolation

Target tenant from `X-Tenant-ID` only. `source_metadata` / export `source` are traceability hints.

## Permissions

Same as export: default-off dev open with tenant header; auth-on requires `PLATFORM_ADMIN`.

## Audit

On successful preview, configuration audit event:

| Field | Value |
|-------|-------|
| DB action | `TEST` (no migration in v0.1) |
| Event kind | `FORM_TEMPLATE_IMPORT_PREVIEWED` |

Payload includes schema version, conflict strategy, import mode, target code/entity type, preview status, source metadata when provided.

## Security Guardrails

- Reuses `ValidateDraftFormTemplateInput` (field types, section/field limits, SQL fragment rejection)
- Rejects unsupported `schema_version`
- Payload size limit 512 KB
- No HTML rendering or rule evaluation

## Verification Commands

Export then preview (PowerShell):

```powershell
cd D:\Projects\freight-platform
$tenant = "74519f22-ff9b-4a8b-8fff-a958c689682f"
$export = curl.exe -s -H "X-Tenant-ID: $tenant" "http://localhost:8080/api/v1/low-code/admin/form-templates/b1111111-1111-4111-8111-111111111102/export"
curl.exe -s -X POST -H "X-Tenant-ID: $tenant" -H "Content-Type: application/json" `
  -d "{ `"schema_version`": `"lowcode.template.export.v1`", `"conflict_strategy`": `"NEW_VERSION`", `"template`": $(($export | ConvertFrom-Json).template | ConvertTo-Json -Compress -Depth 20) }" `
  "http://localhost:8080/api/v1/low-code/admin/form-templates/import-preview"
```

Minimal body:

```powershell
curl.exe -s -X POST -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" -H "Content-Type: application/json" `
  -d "@scripts/dev/payloads/lowcode_template_import_preview_transport_order.json" `
  "http://localhost:8080/api/v1/low-code/admin/form-templates/import-preview"
```

Go tests:

```powershell
cd D:\Projects\freight-platform\services\low-code-service
go test ./...
```

## What Is Not Implemented Yet

- Import execute (`POST .../import`)
- Admin UI import wizard
- Strict unknown-key rejection (v0.1 uses struct decode)
- Dedicated `EXPORT` audit DB action (requires migration)

## Next Action

Low-code Template Import Execute API Pack v0.1

Design reference: `docs/LOW_CODE_TEMPLATE_IMPORT_EXPORT_DESIGN_V0.1.md`

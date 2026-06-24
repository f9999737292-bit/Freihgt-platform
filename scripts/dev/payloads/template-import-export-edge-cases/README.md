# Template import/export edge-case payloads

Dev-only JSON examples for manual curl and UI wizard testing.

## Tenant and template IDs

| Placeholder | Demo value |
|-------------|------------|
| `X-Tenant-ID` | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| Published template ID (export) | `b1111111-1111-4111-8111-111111111102` |
| `transport_order_default` code | exists after `make seed-lowcode-demo` |

Replace `REPLACE_WITH_EXISTING_DRAFT_TEMPLATE_ID` in `replace_existing_draft_request.json` with a real DRAFT row id from:

```powershell
curl.exe -s -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  "http://localhost:8080/api/v1/low-code/admin/form-templates?status=DRAFT&entity_type=TRANSPORT_ORDER"
```

Or clone published to draft in Admin UI first.

## Files

| File | Purpose |
|------|---------|
| `valid_transport_order_export_v1.json` | Valid export envelope shape (`schema_version: lowcode.template.export.v1`) |
| `invalid_schema_version.json` | Expect `UNSUPPORTED_SCHEMA_VERSION` |
| `missing_template.json` | Expect validation error (empty template) |
| `duplicate_field_code.json` | Expect duplicate field validation error |
| `unsupported_field_type.json` | Expect field type validation error |
| `fail_if_exists_request.json` | Expect 409 when `transport_order_default` exists |
| `new_version_request.json` | Import as new DRAFT version when published exists |
| `replace_existing_draft_request.json` | Replace existing DRAFT sections/fields |

## Curl examples

```powershell
cd D:\Projects\freight-platform

curl.exe -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  "http://localhost:8080/api/v1/low-code/admin/form-templates/b1111111-1111-4111-8111-111111111102/export"

curl.exe -X POST -H "Content-Type: application/json" `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  --data-binary "@scripts/dev/payloads/template-import-export-edge-cases/new_version_request.json" `
  "http://localhost:8080/api/v1/low-code/admin/form-templates/import-preview"
```

See `docs/LOW_CODE_TEMPLATE_IMPORT_EXPORT_EDGE_CASES_V0.1.md` for full verification matrix.

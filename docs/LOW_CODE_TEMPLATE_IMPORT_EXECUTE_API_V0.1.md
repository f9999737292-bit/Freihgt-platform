# Low-code Template Import Execute API v0.1

## Summary

Admin-only `POST` endpoint imports a portable template payload as a **DRAFT** only. Re-runs the same validation and conflict checks as import preview, then atomically creates a new draft row or replaces an existing draft. Never auto-publishes.

## Endpoint

| Layer | Method | Path |
|-------|--------|------|
| Service | `POST` | `/v1/low-code/admin/form-templates/import` |
| Gateway | `POST` | `/api/v1/low-code/admin/form-templates/import` |

### Headers

Same as import preview: `X-Tenant-ID` required; `X-User-ID` when auth-on.

Max body: **512 KB**.

## Request Body

Same shape as [import preview](LOW_CODE_TEMPLATE_IMPORT_PREVIEW_API_V0.1.md) (export envelope or explicit import request).

Recommended flow: call `import-preview` first, then `import` with the same payload after operator approval.

## Response

HTTP **201 Created**:

```json
{
  "id": "draft-template-uuid",
  "status": "DRAFT",
  "version": 2,
  "code": "transport_order_default",
  "import_summary": {
    "sections_count": 1,
    "fields_count": 3,
    "conflict_strategy": "NEW_VERSION",
    "import_mode": "CREATE_DRAFT",
    "replaced_draft": false
  }
}
```

| Field | Notes |
|-------|-------|
| `version` | Draft row version (`max(existing)+1` for new draft; unchanged when replacing draft) |
| `replaced_draft` | `true` when existing DRAFT sections were replaced |

## Execution Rules

| Mode / strategy | Action |
|-----------------|--------|
| `NEW_VERSION` (default) | Create new DRAFT with next version for `(tenant, entity_type, code)` |
| `FAIL_IF_EXISTS` | 409 if any template with target code exists |
| `NEW_CODE` / `CREATE_NEW_CODE` | Create DRAFT; requires unused `target_code` |
| `REPLACE_EXISTING_DRAFT` | Replace sections/fields on existing DRAFT; 400 if no DRAFT |

Hard rules (unchanged from design):

- Never overwrite `PUBLISHED` or `ARCHIVED`
- Never auto-publish
- Tenant from `X-Tenant-ID` only

## Audit

On success, configuration audit event:

| Field | Value |
|-------|-------|
| DB action | `CREATE` (new draft) or `UPDATE` (replace draft) |
| Event kind | `FORM_TEMPLATE_IMPORTED_AS_DRAFT` |

Payload includes template id, code, version, entity type, conflict strategy, import mode, section/field counts, source metadata, `dry_run: false`.

## Permissions

Same as export/preview: default-off dev; auth-on requires `PLATFORM_ADMIN`.

## Verification Commands

Import sample payload:

```powershell
cd D:\Projects\freight-platform
curl.exe -s -X POST `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  -H "Content-Type: application/json" `
  -d "@scripts/dev/payloads/lowcode_template_import_preview_transport_order.json" `
  "http://localhost:8080/api/v1/low-code/admin/form-templates/import"
```

Verify audit:

```powershell
curl.exe -s -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  "http://localhost:8080/api/v1/low-code/audit-events?entity_type=TRANSPORT_ORDER&limit=5"
```

Look for `action: FORM_TEMPLATE_IMPORTED_AS_DRAFT`.

## What Is Not Implemented Yet

- Admin UI import wizard
- Bulk ZIP import/export
- Auto-publish imported template
- Custom field value migration on import

## Next Action

Admin UI Template Import/Export Pack v0.1

Related: `docs/LOW_CODE_TEMPLATE_IMPORT_PREVIEW_API_V0.1.md`, `docs/LOW_CODE_TEMPLATE_EXPORT_API_V0.1.md`

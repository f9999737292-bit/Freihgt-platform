# Low-code Clone Published Template to Draft v0.1

## Summary

Admin API and UI to **clone a PUBLISHED form template into a new DRAFT** with copied sections and fields. Source template remains unchanged.

## Why Clone Is Needed

Published templates are read-only in v0.1. To change fields, rules, or layout, operators clone the published template to a draft, edit, and publish a new version.

## Backend Endpoint

```
POST /v1/low-code/admin/form-templates/{id}/clone-to-draft
POST /api/v1/low-code/admin/form-templates/{id}/clone-to-draft   (gateway)
```

Headers: `X-Tenant-ID` required

Response `201`:

```json
{
  "id": "...",
  "source_template_id": "...",
  "status": "DRAFT",
  "version": 2,
  "code": "transport_order_default"
}
```

## UI Flow

1. `/low-code/admin/form-templates` — filter PUBLISHED → **Clone to draft** action
2. `/low-code/admin/form-templates/{id}` — PUBLISHED detail → **Clone to draft** button
3. Success → redirect to new DRAFT detail (editable)
4. Save Draft / Publish as usual

## Versioning / Code Strategy

Schema: `UNIQUE (tenant_id, entity_type, code, version)` on `form_templates`.

- `version = MAX(version for tenant/entity_type/code) + 1`
- `code = source.code` (same code, new version)
- Configuration row: `cfg_{code}` with matching version
- Fallback `_draft_v{N}` suffix reserved for future conflict handling (not needed when version increments)

## Audit Event

`FORM_TEMPLATE_CLONED_TO_DRAFT` in audit payload:

- `source_template_id`, `draft_template_id`
- `entity_type`, `source_code`, `draft_code`
- `source_version`, `draft_version`
- `sections_count`, `fields_count`

Written in the same DB transaction as the clone.

## Guardrails

- Tenant isolation on read/write
- Only `PUBLISHED` sources allowed (DRAFT / ARCHIVED rejected)
- Source template not modified
- New template: `status=DRAFT`, `published_at=null`
- Sections/fields copied with **new UUIDs**
- Public API `/api/v1/low-code/form-templates` still returns **PUBLISHED only**

## Verification Commands

```powershell
cd D:\Projects\freight-platform
make platform-build-service SERVICE=low-code-service
make platform-up-no-build
make health-check
make seed-lowcode-demo

# List published
curl -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" "http://localhost:8080/api/v1/low-code/admin/form-templates?status=PUBLISHED&entity_type=TRANSPORT_ORDER"

# Clone (replace {id})
curl -X POST -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" "http://localhost:8080/api/v1/low-code/admin/form-templates/{id}/clone-to-draft"

# Verify draft in admin list
curl -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" "http://localhost:8080/api/v1/low-code/admin/form-templates?status=DRAFT"

# Public list — new DRAFT must NOT appear
curl -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" "http://localhost:8080/api/v1/low-code/form-templates?entity_type=TRANSPORT_ORDER"

cd services/low-code-service
go test ./...

cd ../../apps/web-admin
npm run build
```

Manual: open PUBLISHED template → Clone to draft → edit field → Save Draft → source PUBLISHED unchanged.

## Known Limitations

- No automatic archive of old published version on publish
- No drag-and-drop builder
- Clone from ARCHIVED not supported in v0.1
- Same-code versioning only (no rename on clone)

## Next Action

1. Publish flow: optional archive previous PUBLISHED version
2. Show clone lineage (source template link) on draft detail

See also: `docs/LOW_CODE_FORM_TEMPLATE_ADMIN_UI_V0.1.md`, `docs/LOW_CODE_FORM_TEMPLATE_DRAFT_API_V0.1.md`.

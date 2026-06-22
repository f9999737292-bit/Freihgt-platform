# Low-code Form Template Draft API v0.1

## Summary

This pack adds **admin-only** backend API for creating and editing **DRAFT** form templates in `low-code-service`.

The existing public endpoint `GET /v1/low-code/form-templates` continues to return **PUBLISHED** templates only.

No Form Builder UI, no core entity changes, and no new migrations were created — the existing `lowcode` schema from migration `000011` is sufficient.

## Admin Endpoints

Base path (service):

```
/v1/low-code/admin/form-templates
```

Gateway:

```
/api/v1/low-code/admin/form-templates
```

| Method | Path | Description |
|--------|------|-------------|
| POST | `/` | Create DRAFT template with sections and fields |
| GET | `/` | List templates (includes DRAFT) |
| GET | `/{id}` | Get template detail (any status) |
| PUT | `/{id}` | Update DRAFT template (replace sections/fields) |
| POST | `/{id}/publish` | Publish DRAFT → PUBLISHED |

All endpoints require tenant via `X-Tenant-ID`.

### Create response

```json
{
  "id": "...",
  "status": "DRAFT",
  "version": 1
}
```

### List query params

| Param | Required | Default | Max |
|-------|----------|---------|-----|
| `entity_type` | optional | — | — |
| `status` | optional | — | DRAFT / PUBLISHED / ARCHIVED |
| `limit` | optional | 50 | 100 |

## Public vs Admin API

| Endpoint | Scope |
|----------|-------|
| `GET /v1/low-code/form-templates` | **PUBLISHED only** (unchanged) |
| `GET /v1/low-code/admin/form-templates` | All statuses for tenant |
| `GET /v1/low-code/admin/form-templates/{id}` | DRAFT / PUBLISHED / ARCHIVED |

## Draft Lifecycle

1. **Create** — inserts `low_code_configurations` + `form_templates` (DRAFT) + sections + fields in one transaction
2. **Update** — only when `status = DRAFT`; replaces sections/fields transactionally
3. **Publish** — sets `status = PUBLISHED`, `published_at = now()` on template and configuration
4. PUBLISHED / ARCHIVED templates cannot be updated in v0.1
5. Publishing does **not** unpublish other templates (different `code` / version allowed)

## Validation Rules

**Template**

- `tenant_id` required (header)
- `entity_type` must be allowed enum
- `code` required, lowercase snake_case (`^[a-z][a-z0-9_]*$`)
- `name` required
- `status` and `version` server-controlled

**Sections**

- `code`, `title` required; `sort_order` int
- max 50 sections per template

**Fields**

- `code`, `label`, `field_type` required; `sort_order` int
- max 200 fields total
- allowed `field_type`: TEXT, NUMBER, DATE, DATETIME, SELECT, MULTI_SELECT, CHECKBOX, MONEY, CURRENCY, FILE, COMPANY_REFERENCE, DOCUMENT_REFERENCE, ROUTE, ADDRESS, VEHICLE, VAT_TAX
- SELECT / MULTI_SELECT: `options_json.options` array required when options provided
- validation/visibility JSON stored as JSON only; SQL-like fragments rejected

## Audit Events

Written to `lowcode.configuration_audit_log` in the same transaction:

| Operation | DB action | API event_kind |
|-----------|-----------|----------------|
| Create draft | CREATE | FORM_TEMPLATE_DRAFT_CREATED |
| Update draft | UPDATE | FORM_TEMPLATE_DRAFT_UPDATED |
| Publish | PUBLISH | FORM_TEMPLATE_DRAFT_PUBLISHED |

Payload includes `template_id`, `entity_type`, `code`, `section_codes`, `field_codes`, counts.

Actor from `X-User-ID`, correlation from `X-Request-ID` when present.

## Tenant Isolation

All queries and mutations filter by `tenant_id`. Cross-tenant access is rejected.

## Sample Payload

`scripts/dev/payloads/lowcode_form_template_draft_transport_order.json`

Dev script (idempotent):

```powershell
make create-lowcode-draft-template
```

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
```

Create draft via gateway:

```powershell
curl.exe -X POST -H "Content-Type: application/json" -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" --data-binary "@scripts/dev/payloads/lowcode_form_template_draft_transport_order.json" http://localhost:8080/api/v1/low-code/admin/form-templates
```

Admin list (drafts):

```powershell
curl.exe -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" "http://localhost:8080/api/v1/low-code/admin/form-templates?status=DRAFT"
```

Public list (published only):

```powershell
curl.exe -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" http://localhost:8080/api/v1/low-code/form-templates
```

Regression:

```powershell
make seed-dev-admin
make seed-demo-data
make integration-smoke-test
cd apps\web-admin
npm run build
```

## What Is Not Implemented Yet

- Form Builder UI
- Template delete / archive API
- Version bump / rollback
- Unpublish existing PUBLISHED template on publish
- BPMN, Rule Engine, Connectors
- New migrations

## Next Action

- Add web-admin Form Builder UI consuming admin draft API
- Add archive / version management endpoints
- Optional: resolve actor UUID to display name in audit list

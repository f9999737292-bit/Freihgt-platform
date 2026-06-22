# Low-code Form Templates API v0.1

## Summary

Read-only HTTP API in **low-code-service** for listing and fetching **published** form templates from schema `lowcode`. Tenant isolation via `X-Tenant-ID`. No write endpoints in v0.1.

## Endpoints

| Method | Path | Description |
| ------ | ---- | ----------- |
| GET | `/v1/low-code/form-templates` | List published templates (optional `entity_type` filter) |
| GET | `/v1/low-code/form-templates/{id}` | Get published template by UUID or `code` |

## Tenant Header

Required header (same convention as web-admin / API Gateway):

```http
X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f
```

Fallback for local debugging: query param `tenant_id` (same value).

Missing tenant → `400` with error code `TENANT_REQUIRED`.

## Response Shape

### List

```json
{
  "items": [
    {
      "id": "...",
      "tenant_id": "...",
      "entity_type": "TRANSPORT_ORDER",
      "code": "transport_order_default",
      "name": "Transport Order Default Form",
      "status": "PUBLISHED",
      "version": 1,
      "sections_count": 1,
      "fields_count": 3,
      "published_at": "2026-06-22T12:00:00Z"
    }
  ]
}
```

### Detail

Includes nested `sections[]` with `fields[]`, ordered by `sort_order`.

Only templates with `status = PUBLISHED` are returned. DRAFT / REVIEW / ARCHIVED are hidden.

## Dev Seed

Dev-only script (psql via Docker, **not** a migration):

```powershell
make seed-lowcode-demo
```

Creates 3 published templates for dev tenant `74519f22-ff9b-4a8b-8fff-a958c689682f`:

| entity_type | code |
| ----------- | ---- |
| TRANSPORT_ORDER | transport_order_default |
| SHIPMENT | shipment_default |
| BILLING_REGISTER | billing_register_default |

Idempotent — safe to run multiple times.

## Direct Service URL

```powershell
curl -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" http://localhost:8088/v1/low-code/form-templates

curl -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" "http://localhost:8088/v1/low-code/form-templates?entity_type=TRANSPORT_ORDER"

curl -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" http://localhost:8088/v1/low-code/form-templates/transport_order_default
```

## Gateway URL

```powershell
curl -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" http://localhost:8080/api/v1/low-code/form-templates
```

Gateway strips `/api` prefix → downstream `/v1/low-code/...`.

## Security Guardrails

- All SQL queries filter `tenant_id`
- Public read endpoints expose **PUBLISHED** only
- Invalid `entity_type` → `VALIDATION_ERROR`
- Unknown template → `FORM_TEMPLATE_NOT_FOUND` (404)
- No cross-tenant reads
- No write/mutation from API

## What Is Not Implemented Yet

- POST/PUT/DELETE form templates
- Custom field values API
- Rule engine / BPMN / connectors
- Form Builder UI
- OpenAPI business paths (gateway proxy only)
- RLS (app-level tenant filter v0.1)

## Verification Commands

```powershell
cd D:\Projects\freight-platform
make platform-build-service SERVICE=low-code-service
make platform-up-no-build
make health-check
make seed-lowcode-demo
go test ./...   # from services/low-code-service if root fails
make seed-dev-admin
make seed-demo-data
make integration-smoke-test
```

## Next Action

1. Custom field values read API (separate pack).
2. OpenAPI spec for low-code paths.
3. Write APIs + Form Builder (future packs).

See also: `docs/LOW_CODE_SERVICE_SKELETON_V0.1.md`, `docs/LOW_CODE_MVP_SCOPE_V0.1.md`.

# Low-code Form Template Version Activation Policy v0.1

## Summary

Added a deterministic **derived** activation policy for published form templates. Active version is not stored in the database — it is computed from published rows per `(tenant_id, entity_type, code)`.

New read-only endpoint returns only active published templates. Public and admin UI show **Active** / **Older published version** badges. Custom field value resolution and publish compare now use the active endpoint.

## Problem

Multiple `PUBLISHED` versions of the same template code can coexist after clone → edit → publish. Consumers need a single canonical template for new forms without hiding historical published versions.

## Activation Policy

For each group `tenant_id + entity_type + code`, the active template is:

1. `status = PUBLISHED`
2. Highest `version`
3. Tie-breaker: latest `published_at` (NULLS LAST)
4. Tie-breaker: latest `updated_at`

Rules:

- **DRAFT** is never active
- **ARCHIVED** is never active
- Publishing a new version automatically makes it active (no separate activation event in v0.1)
- Older `PUBLISHED` versions remain visible in list APIs but are not active

## Backend Endpoint

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/low-code/form-templates/active` | Active published templates |
| GET | `/api/v1/low-code/form-templates/active` | Gateway proxy (same behavior) |

Query params:

- `entity_type` — **required**
- `code` — optional; when set, returns active template for that code only

Headers:

- `X-Tenant-ID` — **required**

Response:

```json
{
  "items": [
    {
      "id": "...",
      "tenant_id": "...",
      "entity_type": "TRANSPORT_ORDER",
      "code": "transport_order_default",
      "name": "...",
      "status": "PUBLISHED",
      "version": 2,
      "published_at": "...",
      "is_active": true
    }
  ]
}
```

Existing endpoints unchanged:

- `GET /v1/low-code/form-templates` — still returns **all** published templates
- `GET /v1/low-code/form-templates/{id}` — unchanged
- Admin draft/publish APIs — unchanged

## UI Badges

| Page | Behavior |
|------|----------|
| `/low-code/form-templates` | Active badge on latest published; older published marked |
| `/low-code/form-templates/[id]` | Active published version / Older published version |
| `/low-code/admin/form-templates` | Draft / Active / Older published version column |
| `/low-code/admin/form-templates/[id]` | Active badge or warning for older published |

## Custom Field Values Resolution

`resolvePublishedTemplate()` in `useLowCodeApi.ts` now calls the active endpoint instead of picking the first item from the public list.

- **Create-first values:** uses active template
- **Existing saved values:** unchanged; GET values still works with the stored `form_template_id` even if that template is an older published version

## Tenant Isolation

Active selection is scoped by `tenant_id` from `X-Tenant-ID`. Cross-tenant templates are never returned.

## What Is Not Implemented Yet

- Explicit activation / deactivation API or audit event
- Automatic archival of older published versions on publish
- Drag-and-drop, BPMN, Rule Engine, Connectors
- Database column or migration for `is_active`

## Verification Commands

```powershell
cd D:\Projects\freight-platform

# Backend tests
cd services/low-code-service
go test ./...

# Build and restart
cd D:\Projects\freight-platform
make platform-build-service SERVICE=low-code-service
make platform-up-no-build
make health-check
make seed-lowcode-demo

# Active endpoint
curl -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" "http://localhost:8080/api/v1/low-code/form-templates/active?entity_type=TRANSPORT_ORDER"

# Frontend build
cd apps/web-admin
npm run build

# Regression
cd D:\Projects\freight-platform
make seed-dev-admin
make seed-demo-data
make integration-smoke-test
```

Manual browser check:

- http://localhost:3000/low-code/form-templates — active badge visible
- http://localhost:3000/low-code/admin/form-templates — active/older badges
- http://localhost:3000/low-code/custom-field-values — still works

## Next Action

- Optional: integration test seeding two published versions and asserting active endpoint returns only the latest
- Future pack: explicit activation workflow or auto-archive policy if product requires it

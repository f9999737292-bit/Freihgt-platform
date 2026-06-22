# Low-code Runtime Integration Policy v0.1

## Summary

This document defines how the **low-code runtime** connects to core TMS entities without changing core business logic, API contracts, or financial/document workflows.

Low-code runtime is a **sidecar layer**: it reads published form templates, stores custom field values, validates those values, and writes audit events. It does not own entity lifecycle, RBAC, or core domain state.

Current implementation in `freight-platform` already follows most of these rules. This policy is the canonical guardrail reference for future integrations.

## Goal

Ensure that low-code features (templates, custom field values, preview, entity detail panels) can evolve safely without:

- modifying core transport-order, shipment, or billing-register services
- breaking existing public/admin API contracts
- bypassing tenant isolation or RBAC
- mutating financial, signing, or document workflows through custom fields

## Runtime Integration Scope

### In scope (v0.1)

| Area | Implementation |
|------|----------------|
| Form templates (public read) | `GET /api/v1/low-code/form-templates`, `GET .../active` |
| Form templates (admin draft/publish) | `GET/POST/PUT /api/v1/low-code/admin/form-templates` |
| Custom field values | `GET/PUT /api/v1/low-code/custom-field-values` |
| Audit log | `GET /api/v1/low-code/audit-events` |
| Entity detail panels | `LowCodeCustomFieldsPanel` on core detail pages |
| Standalone values UI | `/low-code/custom-field-values` |
| Preview / conditional rules | Frontend preview + backend conditional required (soft) |

### Out of scope (v0.1)

- BPMN, Rule Engine, Connectors
- Core entity create/update/delete via low-code
- Drag-and-drop form builder changes (separate pack)
- Database migrations for runtime policy
- Core services calling low-code automatically (optional future)

## Core Entity Boundaries

### Low-code MAY

- Read **active** or **published** form templates for an `entity_type`
- Read and write `custom_field_values` for a known `(tenant_id, entity_type, entity_id)`
- Validate low-code field values (type, simple rules, conditional required)
- Write low-code configuration audit events
- Render read-only or inline-edit UI for custom fields on entity detail pages

### Low-code MUST NOT

- Change core **transport order status** (submit, cancel, sourcing, etc.)
- Change **shipment status** or lifecycle transitions
- Change **billing register status** (calculate, approve, UPD, EDO, paid, closed)
- Bypass core **RBAC** or tenant membership checks
- Create or delete core entities (orders, shipments, registers, documents)
- Alter **financial**, **signing**, or **document** workflows
- Execute arbitrary code, SQL, or scripts from template JSON rules

### Integration pattern

```
Core detail page                Low-code runtime
─────────────────               ──────────────────
GET /api/v1/transport-orders/{id}   (core only)
GET /api/v1/low-code/custom-field-values   (low-code only)
PUT /api/v1/low-code/custom-field-values   (low-code only)
```

Core and low-code APIs are called **independently** from the frontend. Low-code never proxies or wraps core write endpoints.

## Active Template Resolution

### Policy

For **new** custom field value writes, resolve the **active published** template:

```
tenant_id + entity_type + code → latest PUBLISHED version
```

Selection rule (derived, not stored):

1. `status = PUBLISHED`
2. `MAX(version)`
3. Tie-breaker: `published_at DESC NULLS LAST`, then `updated_at DESC`

**DRAFT** and **ARCHIVED** templates are never used at runtime.

### Implemented endpoint

Active template resolution is implemented in **Low-code Form Template Version Activation Policy v0.1**:

| Method | Path |
|--------|------|
| GET | `/v1/low-code/form-templates/active?entity_type=...&code=...` |
| GET | `/api/v1/low-code/form-templates/active?entity_type=...&code=...` |

See `docs/LOW_CODE_FORM_TEMPLATE_VERSION_ACTIVATION_POLICY_V0.1.md`.

### Frontend resolution

`useLowCodeApi().resolvePublishedTemplate(entityType, code?)` calls the active endpoint and loads template detail.

Used by:

- `LowCodeCustomFieldsPanel.vue` — template metadata for create-first / preview
- `/low-code/custom-field-values` — field definitions for edit UI

### Existing saved values

If values were saved with an **older published** `form_template_id`, `GET custom-field-values` continues to work. Runtime does not migrate values automatically when a newer version is published.

## Custom Field Values Lifecycle

### Read

```http
GET /api/v1/low-code/custom-field-values?entity_type={type}&entity_id={uuid}
X-Tenant-ID: {tenant_id}
```

Returns stored values for the entity regardless of which published template version was used at save time.

### Write (create or update)

```http
PUT /api/v1/low-code/custom-field-values
X-Tenant-ID: {tenant_id}
```

Payload includes:

- `entity_type`, `entity_id`
- `form_template_id` — must be a **published** template matching `entity_type`
- `values[]` — `{ field_code, value_json }`
- `validation_context` — optional (see below)

### Rules

| Scenario | Template to use |
|----------|-----------------|
| Create-first values (no rows yet) | Active published template |
| Update existing values | Same `form_template_id` as stored, or active template for new fields |
| Preview / field metadata | Active published template |

Backend validates:

- Tenant match on all queries
- `form_template_id` belongs to tenant and is `PUBLISHED`
- Field codes exist on template
- Field types and validation rules
- `system_field` cannot be written (`SYSTEM_FIELD_PROTECTED`)

Frontend edit UI (`LowCodeCustomFieldsPanel`) excludes `system_field` and `read_only` fields from editable controls.

## Validation Context

Optional client-provided context for **soft validation** (conditional required, visibility preview):

```json
{
  "validation_context": {
    "entity_status": "DRAFT",
    "role": "SHIPPER_ADMIN"
  }
}
```

### Rules (v0.1)

| Rule | Detail |
|------|--------|
| Optional | Requests without `validation_context` are valid |
| Core services | Not required to pass context in v0.1 |
| UI | May pass known `entity_status` from loaded core entity; role from session when available |
| Trust boundary | Backend **must not** trust client context for financial authorization or payment decisions |
| Scope | v0.1: conditional required and preview visibility only — no hard financial gates |

### Implementation

- Backend: `domain.ValidationContext` in `conditional_required.go`; applied in `CustomFieldValueService.Upsert`
- Frontend: `useLowCodePreviewContext().buildPreviewContext(entityStatus, override)`; sent on save from `LowCodeCustomFieldsPanel`

Example: `loading_window_note` required when `cargo_class=GENERAL` — field rule, not a billing decision.

## Entity Detail Integration

### TRANSPORT_ORDER — `/transport-orders/[id]`

| Aspect | Policy |
|--------|--------|
| Core card | Status, dates, equipment — core API only |
| Low-code panel | `LowCodeCustomFieldsPanel` with `editable`, `entity-status` |
| Allowed writes | Custom field values only |
| Forbidden | Editing core order fields via low-code |

Core actions (submit order, mini-tender) remain on core endpoints.

### SHIPMENT — `/shipments/[id]`

| Aspect | Policy |
|--------|--------|
| Core card | Status, driver, vehicle, routes — core API only |
| Low-code panel | `LowCodeCustomFieldsPanel` with `editable`, `entity-status` |
| Allowed writes | Custom field values only |
| Forbidden | Shipment lifecycle transitions via low-code |

Status changes (assign driver, advance status, cancel) use shipment-service APIs only.

### BILLING_REGISTER — `/billing-registers/[id]`

| Aspect | Policy |
|--------|--------|
| Core card | Status, totals — core API only |
| Low-code panel | `LowCodeCustomFieldsPanel` with `editable`, `entity-status` |
| Allowed writes | Custom field values only |
| Forbidden | Payment, signing, UPD, EDO, approve/close via low-code |

Financial workflow actions use billing-register-service APIs only.

### Other entity types

Also integrated (same pattern): `FREIGHT_REQUEST`, `DOCUMENT`, `RFX` detail pages with read-only or editable low-code panels. See `docs/LOW_CODE_ENTITY_DETAIL_RFX_DOCUMENT_V0.1.md`.

### Standalone editor

`/low-code/custom-field-values` — full edit/preview flow; uses same APIs and active template resolution. Does not modify core entities.

## Audit Requirements

All low-code **writes** must produce audit records in `lowcode.configuration_audit_log`.

### Custom field values

Every successful value upsert writes:

- **Event kind:** `CUSTOM_FIELD_VALUES_UPDATED`
- **DB action:** `UPDATE`
- **Payload:** `form_template_id`, `changed_fields`, old/new value maps

### Form templates (admin)

| Operation | Event kind |
|-----------|------------|
| Create draft | `FORM_TEMPLATE_DRAFT_CREATED` |
| Update draft | `FORM_TEMPLATE_DRAFT_UPDATED` |
| Publish draft | `FORM_TEMPLATE_DRAFT_PUBLISHED` |
| Clone published → draft | `FORM_TEMPLATE_CLONED_TO_DRAFT` |

### Audit record requirements

| Field | Required |
|-------|----------|
| `tenant_id` | Yes |
| `entity_type` | When applicable |
| `entity_id` | When applicable (custom field values) |
| `changed_by_user_id` | When available from request context |
| `request_id` | When available (`X-Request-ID` or equivalent) |
| Old/new values | Required for `CUSTOM_FIELD_VALUES_UPDATED` |

Read-only runtime operations (GET templates, GET values, GET audit) do **not** require audit entries.

Activation policy (derived active version) does **not** emit a separate audit event in v0.1.

## Tenant Isolation

Mandatory on every low-code request:

- `X-Tenant-ID` header required (400 `TENANT_REQUIRED` if missing)
- All SQL queries filter by `tenant_id`
- Template resolution never crosses tenants
- Custom field values scoped to `(tenant_id, entity_type, entity_id)`

Gateway forwards `/api/v1/low-code/*` to `low-code-service` with tenant header preserved.

## Security Guardrails

| Guardrail | Status (v0.1) |
|-----------|---------------|
| Tenant filtering mandatory | Enforced (handlers + repository) |
| No cross-tenant template resolution | Enforced |
| No arbitrary code execution | JSON rules only; no eval/script engine |
| No SQL fragments in JSON rules | Rules parsed as JSON structures only |
| No `v-html` for low-code JSON | Enforced in low-code components (text / `<pre>` only) |
| `system_field` protected | Enforced backend (`SYSTEM_FIELD_PROTECTED`) + UI filter |
| `read_only` fields protected | Enforced backend (`READ_ONLY_FIELD_PROTECTED`) + UI filter |
| DRAFT templates never public runtime | Public API returns `PUBLISHED` only |
| ARCHIVED templates never runtime | Excluded from public/active queries |
| PUBLISHED templates immutable | Admin must clone-to-draft to edit |
| Clone-to-draft required for template changes | Enforced in admin UI |
| Audit required for writes | Enforced in repository layer |

## What Is Not Allowed

Explicit anti-patterns for future development:

1. **Low-code webhook** that calls core status transition APIs based on field values
2. **Template rules** that embed SQL, JavaScript, or shell commands
3. **Rendering user JSON** with `v-html` or `dangerouslySetInnerHTML`
4. **Skipping tenant header** in internal service-to-service calls
5. **Using DRAFT template ID** in `form_template_id` on PUT values
6. **Mutating core entity** from low-code-service (no imports of core domain services)
7. **Trusting `validation_context.role`** for payment approval or UPD creation
8. **Auto-migrating** custom field values to new template version without explicit migration pack

## Verification Commands

```powershell
cd D:\Projects\freight-platform

# Backend health
make health-check
make seed-lowcode-demo

# Active template (prerequisite for runtime resolution)
curl.exe -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" "http://localhost:8080/api/v1/low-code/form-templates/active?entity_type=TRANSPORT_ORDER"

# Custom field values
curl.exe -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" "http://localhost:8080/api/v1/low-code/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id=<ENTITY_ID>"

# Audit
curl.exe -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" "http://localhost:8080/api/v1/low-code/audit-events?entity_type=TRANSPORT_ORDER&entity_id=<ENTITY_ID>"

# Regression
make integration-smoke-test

# Frontend
cd apps/web-admin
npm run build
```

Manual browser checks:

| URL | Check |
|-----|-------|
| `/transport-orders/{id}` | Low-code panel edits values; core status unchanged by panel |
| `/shipments/{id}` | Same; lifecycle buttons use core APIs |
| `/billing-registers/{id}` | Same; financial actions separate |
| `/low-code/custom-field-values` | Active template used; save writes audit |

Backend unit tests covering guardrails:

```powershell
cd services/low-code-service
go test ./internal/http/handlers/... ./internal/domain/...
```

## Next Action

1. ~~**Optional:** backend reject writes to `read_only` fields~~ — implemented in `LOW_CODE_RUNTIME_INLINE_EDIT_GUARDRAILS_V0.1.md`
2. **Optional:** automated runtime compliance test (entity detail save does not call core PUT)
3. **Future pack:** core services optionally pass `validation_context` on server-side integration
4. **Future pack:** explicit custom field value migration when active template version changes

Related docs:

- `docs/LOW_CODE_FORM_TEMPLATE_VERSION_ACTIVATION_POLICY_V0.1.md` — active template (implemented)
- `docs/LOW_CODE_RUNTIME_INLINE_EDIT_GUARDRAILS_V0.1.md` — read-only field backend guardrail (implemented)
- `docs/LOW_CODE_ENTITY_DETAIL_INLINE_EDIT_V0.1.md` — inline edit on entity pages
- `docs/LOW_CODE_CUSTOM_FIELD_VALUES_API_V0.1.md` — values API contract
- `docs/LOW_CODE_AUDIT_LOG_V0.1.md` — audit read API

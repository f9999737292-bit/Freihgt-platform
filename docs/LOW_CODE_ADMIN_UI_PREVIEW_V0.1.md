# Low-code Admin UI Preview v0.1

## Summary

Read-only preview pages in **web-admin** for the low-code layer: form templates list/detail and custom field values lookup. No write UI, no backend changes.

## Routes

| Route | Page |
| ----- | ---- |
| `/low-code` | Dashboard / overview |
| `/low-code/form-templates` | Published templates table |
| `/low-code/form-templates/[id]` | Template detail (sections + fields) |
| `/low-code/custom-field-values` | Entity lookup for stored values |

Navigation: sidebar item **Low-code** (`nav.lowCode`).

## API Dependencies

Gateway base: `http://localhost:8080` (via `useApi` / `apiBaseUrl`).

| Client function | Gateway endpoint |
| --------------- | ---------------- |
| `listFormTemplates(entity_type?)` | `GET /api/v1/low-code/form-templates` |
| `getFormTemplate(id)` | `GET /api/v1/low-code/form-templates/{id}` |
| `getCustomFieldValues(entity_type, entity_id)` | `GET /api/v1/low-code/custom-field-values` |

Tenant: `X-Tenant-ID` header + `tenant_id` query (same as other web-admin APIs).

Demo entity resolution (optional UX): list APIs for transport orders, shipments, billing registers — no hardcoded UUIDs.

## Read-only Scope

**Included:**

- Browse published form templates
- View sections, fields, JSON metadata (collapsed)
- Lookup custom field values by entity type + entity UUID
- Service status summary on hub page
- Empty / API unavailable states

**Excluded (v0.1):**

- Create / edit / delete templates or fields
- PUT custom field values from UI
- Form Builder, Rule Engine, BPMN, Connectors UI

## Pages

### `/low-code`

- Backend + low-code service status
- Published templates count (when API reachable)
- Links to sub-pages
- Read-only and editing-not-implemented notices

### `/low-code/form-templates`

- Table: entity_type, code, name, status, version, sections_count, fields_count, published_at
- Filter: entity_type select
- Action: Open → detail page

### `/low-code/form-templates/[id]`

- Template metadata
- Sections with fields table (code, label, field_type, required, read_only, system_field)
- Collapsible `options_json`, `validation_rule_json`, `visibility_rule_json`

### `/low-code/custom-field-values`

- entity_type select (TRANSPORT_ORDER, SHIPMENT, BILLING_REGISTER)
- entity_id text input (UUID)
- Load values / Use demo entity buttons
- Table: field_code, value_json, updated_at

## Demo Data

Prerequisites:

```powershell
make seed-dev-admin
make seed-demo-data
make seed-lowcode-demo
```

| Entity type | Demo ref | Sample custom fields |
| ----------- | -------- | -------------------- |
| TRANSPORT_ORDER | DEMO-TO-001 | cargo_class, internal_cost_center |
| SHIPMENT | DEMO-SH-PLANNED | temperature_mode |
| BILLING_REGISTER | DEMO-BR-001 | payment_priority |

Use **Use demo entity** on custom field values page to resolve UUID via list APIs.

Dev tenant: `74519f22-ff9b-4a8b-8fff-a958c689682f`

## Verification Commands

```powershell
cd D:\Projects\freight-platform

make health-check
make seed-lowcode-demo

cd apps/web-admin
npm run build
npm run dev
```

Browser checks (after login with dev tenant):

- `/low-code` — status + links
- `/low-code/form-templates` — 3 templates after seed
- `/low-code/form-templates/{id}` — sections/fields visible
- `/low-code/custom-field-values` — load demo values

Regression:

```powershell
make integration-smoke-test
```

## Known Limitations

- Read-only; no template or value editing
- Only PUBLISHED templates exposed by API
- entity_id must be UUID (not demo ref string)
- Low-code service health inferred via API probe, not dedicated health widget
- Demo entity button requires seed-demo-data entities to exist

## Next Action

1. Form template write API + admin create/edit UI (future pack)
2. Inline custom fields on entity detail pages (transport order, shipment, etc.)
3. OpenAPI spec for low-code gateway paths
4. Rule engine / visibility preview UI

See also: `docs/LOW_CODE_FORM_TEMPLATES_API_V0.1.md`, `docs/LOW_CODE_CUSTOM_FIELD_VALUES_API_V0.1.md`.

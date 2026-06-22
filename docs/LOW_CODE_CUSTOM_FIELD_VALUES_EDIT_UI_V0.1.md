# Low-code Custom Field Values Edit UI v0.1

## Summary

Adds **edit mode** for low-code custom field values in web-admin. Users can update stored values via existing `PUT /api/v1/low-code/custom-field-values` from the lookup page. Entity detail pages remain read-only.

## Scope

**Included:**

- `saveCustomFieldValues` API client
- `LowCodeCustomFieldsPanel` extended with `editable` prop
- Edit / Save / Cancel / Reload on `/low-code/custom-field-values`
- Template metadata lookup for field types
- Error mapping for API validation codes
- i18n RU/EN/ZH

**Excluded:**

- Form Builder / template create-edit
- Rule Engine, BPMN, Connectors
- Edit on entity detail pages (transport order, shipment, billing register)
- Backend / API contract / migration changes

## Editable Page

**Route:** `/low-code/custom-field-values`

Flow:

1. Select entity type + entity ID (or **Use demo entity**)
2. **Load values** → editable panel appears
3. **Edit** → typed inputs by field metadata
4. **Save** → `PUT` upsert, reload values
5. **Reload values** → refresh from API

Warning shown: editing affects only low-code custom field values; core entity data is unchanged.

## Read-only Pages

Entity detail pages keep default `editable=false`:

- `/transport-orders/[id]`
- `/shipments/[id]`
- `/billing-registers/[id]`

Low-code hub and form template pages unchanged (read-only).

## Supported Field Types

| field_type | Edit control |
| ---------- | ------------ |
| TEXT | Text input |
| NUMBER | Number input |
| SELECT | Select from `options_json` |
| CHECKBOX | Checkbox |
| CURRENCY | Text input |
| MONEY, objects, arrays, other | JSON textarea fallback |

Fields with `system_field=true` or `read_only=true` are display-only in edit mode.

## API Used

| Method | Endpoint | Usage |
| ------ | -------- | ----- |
| GET | `/api/v1/low-code/form-templates` | Resolve published template by entity_type |
| GET | `/api/v1/low-code/form-templates/{id}` | Field definitions |
| GET | `/api/v1/low-code/custom-field-values` | Load values |
| PUT | `/api/v1/low-code/custom-field-values` | Save values |

PUT payload:

```json
{
  "entity_type": "TRANSPORT_ORDER",
  "entity_id": "...",
  "form_template_id": "...",
  "values": [{ "field_code": "internal_cost_center", "value_json": "CC-2002" }]
}
```

Tenant: `X-Tenant-ID` header + `tenant_id` query (existing convention).

## Guardrails

- Edit disabled when API unavailable
- System/read-only fields not sent on save
- API errors mapped: `TENANT_REQUIRED`, `FORM_TEMPLATE_NOT_FOUND`, `FIELD_NOT_FOUND`, `FIELD_INVALID_TYPE`, `VALIDATION_RULE_FAILED`, `SYSTEM_FIELD_PROTECTED`
- No template → edit button hidden; values still display read-only
- Core domain services untouched

## Verification Commands

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo

cd apps/web-admin
$env:NUXT_IGNORE_LOCK=1; npm run build
npm run dev
```

**Browser (dev tenant `74519f22-ff9b-4a8b-8fff-a958c689682f`):**

1. `/low-code/custom-field-values` → Use demo entity (TRANSPORT_ORDER)
2. Edit → change `internal_cost_center` → Save
3. Reload → verify new value
4. Entity detail page → panel still read-only

**API after save:**

```powershell
curl.exe -s -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" "http://localhost:8080/api/v1/low-code/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id=2db04b49-665c-469f-bcb1-ffeb1274fedb"
```

## Known Limitations

- Edit only on lookup page v0.1
- Partial field type support (complex types use JSON textarea)
- No inline validation preview beyond API errors
- No Playwright UI tests in this pack
- Template picked as first published match for entity_type

## Next Action

1. Optional: enable edit on entity detail pages (feature flag)
2. Rich editors per field type (DATE, MONEY object, MULTI_SELECT)
3. Show field labels from template metadata in panel
4. Form template write UI (separate pack)

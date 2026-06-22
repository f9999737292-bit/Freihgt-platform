# Low-code Entity Detail Inline Edit v0.1

## Summary

Entity detail pages now support **inline edit** of low-code custom field values via existing `LowCodeCustomFieldsPanel` (`editable=true`). Core entity data is unchanged; saves use `PUT /api/v1/low-code/custom-field-values` with audit logging (unchanged backend).

## Enabled pages

| Page | Route |
| ---- | ----- |
| Transport order | `/transport-orders/[id]` |
| Shipment | `/shipments/[id]` |
| Billing register | `/billing-registers/[id]` |
| Freight request | `/freight-requests/[id]` |
| Document | `/documents/[id]` |
| RFX event | `/rfx/[id]` |

Each panel passes `entity-status` for preview rules and `show-full-editor-link` for deep link to `/low-code/custom-field-values`.

## Panel behavior

- **Edit / Save / Cancel / Reload** when values exist and published template is resolved
- Read-only badge hidden when `editable=true`
- Link **Open full editor** → `/low-code/custom-field-values?entity_type=…&entity_id=…&entity_status=…`
- Preview + visibility / conditional required rules unchanged

## Full editor deep link

**Helper:** `buildCustomFieldValuesEditorLink()` in `types/lowCode.ts`

Custom-field-values page reads query params on mount and auto-loads when `entity_id` is present.

**Extended entity types** on lookup page: `FREIGHT_REQUEST`, `DOCUMENT`, `RFX` (demo refs + status fetch).

## Verification

```powershell
make seed-lowcode-demo
cd apps/web-admin; npm run dev
```

1. Open **DEMO-TO-001** → custom fields panel → **Edit** → change `internal_cost_center` → **Save**
2. Reload page — value persisted
3. **Open full editor** → lands on `/low-code/custom-field-values` with entity pre-filled
4. Repeat on **DEMO-SH-PLANNED**, **DEMO-BR-001**, **DEMO-FR-001**, **DEMO-DOC-001**, **DEMO-RFX-001**

## Limitations

- Edit requires at least one stored custom field value (same as lookup page v0.1)
- No create-first-value flow for empty entities
- Complex field types use JSON textarea
- Preview rules are not enforced on save (API validates types/static rules only)

## Next action

1. Server-side conditional required validation (future pack)
2. Create custom field values when template exists but no values stored
3. Rich field editors (DATE, MONEY, MULTI_SELECT)

See also: `docs/LOW_CODE_CUSTOM_FIELD_VALUES_EDIT_UI_V0.1.md`, `docs/LOW_CODE_ENTITY_DETAIL_PREVIEW_V0.1.md`.

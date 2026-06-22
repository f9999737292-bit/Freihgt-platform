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

- Complex field types beyond DATE/MONEY/MULTI_SELECT still use JSON textarea
- Preview rules are not all enforced on save (conditional required is enforced server-side)

## Next action

1. Entity reference / FILE upload editors
2. RFx lot / bid entity types when templates are needed

See also: `docs/LOW_CODE_CONDITIONAL_REQUIRED_VALIDATION_V0.1.md`, `docs/LOW_CODE_CREATE_FIRST_VALUE_EDIT_V0.1.md`, `docs/LOW_CODE_RICH_FIELD_EDITORS_V0.1.md`.

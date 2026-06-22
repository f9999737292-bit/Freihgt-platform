# Low-code Rich Field Editors v0.1

## Summary

Custom field inline edit now uses typed controls for **DATE**, **DATETIME**, **MULTI_SELECT**, and **MONEY** instead of raw JSON textareas.

## Supported edit controls

| Type | Control |
| ---- | ------- |
| `DATE` | `type="date"` input |
| `DATETIME` | `type="datetime-local"` → RFC3339 on save |
| `MULTI_SELECT` | Native `<select multiple>` |
| `MONEY` | Amount (number) + currency (text) |

JSON textarea remains for `FILE`, references, and complex object types.

## Helpers

**File:** `apps/web-admin/types/lowCode.ts`

- `seedEditDraftForField`, `parseEditDraftToValueJson` (extended)
- `moneyDraftKeys`, `seedMoneyDraft`, `parseMoneyDraft`

## Demo seed (SHIPMENT)

New fields on `shipment_default` template:

| Field | Type |
| ----- | ---- |
| `planned_pickup_date` | DATE |
| `declared_value` | MONEY |
| `handling_flags` | MULTI_SELECT |

Seeded on **DEMO-SH-PLANNED** via `make seed-lowcode-demo`.

## Verification

1. Open **DEMO-SH-PLANNED** → **Edit**
2. Change date, amount/currency, multi-select flags → **Save**
3. Reload — values persisted and shown in preview

## Limitations

- No entity picker for `COMPANY_REFERENCE` / `DOCUMENT_REFERENCE`
- No file upload for `FILE`
- Client-side validation is minimal; server validates types

## Next action

1. Entity reference pickers
2. FILE upload widget

See also: `docs/LOW_CODE_CREATE_FIRST_VALUE_EDIT_V0.1.md`, `docs/LOW_CODE_CUSTOM_FIELD_VALUES_EDIT_UI_V0.1.md`.

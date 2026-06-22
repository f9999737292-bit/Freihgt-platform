# Low-code Form Builder UX v0.1

## Summary

Improved the admin form template editor into a simple form builder experience: field palette, presets, section/field management, JSON helpers, client-side validation, and live preview layout — without drag-and-drop and without backend changes.

## Scope

- **Frontend only:** `apps/web-admin` (editor component, admin pages, i18n)
- **No backend business logic changes**
- **No API contract changes**
- **No migrations**
- **Published templates:** still read-only; edit via **Clone to Draft** only
- **Save:** explicit **Save Draft** only (no auto-save)

## Builder UX Features

- Field palette (quick add by type) targeting the active section
- Field presets (text, select, money, date, checkbox, multi-select, phone/comment)
- Section collapse/expand with field count badge
- Compact field cards with badges (required, read-only, system field)
- Duplicate field, move up/down (sort_order), remove with confirmation
- JSON format/validate for `options_json`, `validation_rule_json`, `visibility_rule_json`
- Client-side validation before Save Draft
- Two-column builder on wide screens (editor + live preview); Editor/Preview tabs on narrow screens
- Sticky Save Draft action bar on create/edit draft pages

## Field Palette

Quick-add types (added to active section with defaults):

- TEXT, NUMBER, DATE, DATETIME
- SELECT, MULTI_SELECT, CHECKBOX
- MONEY, CURRENCY, FILE
- COMPANY_REFERENCE, DOCUMENT_REFERENCE
- ADDRESS, VEHICLE

## Presets

| Preset | field_type | Default code / label | options_json |
|--------|------------|----------------------|--------------|
| Text field | TEXT | `text_field` | — |
| Select field | SELECT | `select_field` | sample options array |
| Money field | MONEY | `amount` | — |
| Date field | DATE | `event_date` | — |
| Checkbox field | CHECKBOX | `confirmed` | — |
| Multi-select field | MULTI_SELECT | `tags` | sample options array |
| Phone / comment | TEXT | `comment` | validation maxLength 500 |

## Validation

Before Save Draft:

- Template code and name required
- At least one section
- Section code and title required per section
- Field code, label, and field_type required per field
- Duplicate section codes blocked (case-insensitive)
- Duplicate field codes blocked within template (case-insensitive)
- Invalid JSON blocked for options/validation/visibility JSON fields
- SELECT / MULTI_SELECT: `options_json` must contain a non-empty `options` array when provided

Errors shown at the top of the editor and inline where applicable.

## Preview Integration

- **New draft:** `/low-code/admin/form-templates/new` — live preview from unsaved draft
- **Edit draft / view published:** `/low-code/admin/form-templates/{id}` — preview updates as draft is edited; published templates show read-only editor + preview from server data
- Preview uses existing `LowCodeFormTemplatePreview` renderer (no `v-html`; JSON via safe text/pre)

## What Is Not Implemented Yet

- Drag-and-drop field/section reordering
- BPMN / Rule Engine / Connectors
- Inline editing of published templates (use Clone to Draft)
- Auto-save
- Full field type list in palette (ROUTE, VAT_TAX remain outside palette but can exist on cloned templates)

## Verification Commands

```powershell
cd D:\Projects\freight-platform\apps\web-admin
npm run build
npm run dev
```

Manual checks:

1. **New draft:** http://localhost:3000/low-code/admin/form-templates/new  
   Add section, quick-add TEXT/SELECT, format options JSON, Save Draft
2. **Edit draft:** http://localhost:3000/low-code/admin/form-templates — open DRAFT  
   Duplicate field, move up/down, collapse section, preview updates, Save Draft
3. **Published:** open PUBLISHED — editor read-only, Clone to Draft works

Backend regression:

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo
make integration-smoke-test
```

## Next Action

- Optional: drag-and-drop reordering
- Optional: extend palette for ROUTE / VAT_TAX if product requires
- Optional: shared builder layout component to DRY page styles

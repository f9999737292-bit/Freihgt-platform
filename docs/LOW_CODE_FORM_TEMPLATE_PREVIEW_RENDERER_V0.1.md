# Low-code Form Template Preview Renderer v0.1

## Summary

Read-only form template preview renderer for web-admin. Renders published or draft templates as disabled form controls grouped by sections. Optional value overlay for custom field values.

This is **not** a drag-and-drop Form Builder. Backend, API contracts, and migrations were not changed.

## Component

`apps/web-admin/components/low-code/LowCodeFormTemplatePreview.vue`

Props:

| Prop | Type | Description |
|------|------|-------------|
| `template` | `FormTemplatePreviewModel` | Sections and fields to render |
| `values` | `Record<string, unknown>` optional | Field code → value map |
| `readonly` | boolean (default `true`) | All inputs disabled |
| `title` | string optional | Card header title |

Behavior:

- Groups fields by section (sorted by `sort_order`)
- Shows label, code, field_type, required/read_only/system_field badges
- Never calls save/PUT APIs
- JSON rendered via `<pre>` text (no `v-html`)

## Pages Integrated

| Page | Preview source |
|------|----------------|
| `/low-code/form-templates/[id]` | Published template; Details / Preview tabs |
| `/low-code/admin/form-templates/[id]` | Live preview from draft editor state; published/archived from loaded template |
| `/low-code/custom-field-values` | Published template + loaded custom field values |

## Supported Field Types

| Type | Preview control |
|------|-----------------|
| TEXT, CURRENCY, COMPANY_REFERENCE, DOCUMENT_REFERENCE | disabled text input |
| NUMBER | disabled number input |
| DATE | disabled date input |
| DATETIME | disabled datetime-local input |
| SELECT | disabled select from `options_json` |
| MULTI_SELECT | disabled multi-select + selected list |
| CHECKBOX | disabled checkbox |
| MONEY | amount + currency text or JSON fallback |
| FILE | placeholder text |
| ROUTE, ADDRESS, VEHICLE, VAT_TAX | JSON `<pre>` fallback |
| unknown | JSON/text fallback |

## Preview With Values

On `/low-code/custom-field-values`, after values load:

- Resolves published template for entity type
- Maps `field_code` → `value_json`
- Passes values into preview component

Edit panel remains unchanged above preview.

## Security Guardrails

- All inputs `disabled`
- No save/edit mode in preview component
- No arbitrary HTML from JSON
- `options_json` parsed as JSON only, not executed
- Preview-only badge shown

## What Is Not Implemented Yet

- Drag-and-drop Form Builder
- Interactive preview (editable fields)
- Rule engine evaluation / visibility rules
- Live custom field value editing in preview
- Backend preview API endpoint

## Verification Commands

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo

cd D:\Projects\freight-platform\apps\web-admin
npm run build
npm run dev
```

Browser:

1. http://localhost:3000/low-code/form-templates → open published template → Preview tab
2. http://localhost:3000/low-code/admin/form-templates → open DRAFT → live preview below editor
3. http://localhost:3000/low-code/custom-field-values → load demo entity → values preview

Regression:

```powershell
make integration-smoke-test
```

## Next Action

- Interactive preview mode for Form Builder v2
- Pass preview context (entity status / role) from entity detail pages
- Conditional required indicator in preview

See `docs/LOW_CODE_PREVIEW_VISIBILITY_RULES_V0.1.md`.

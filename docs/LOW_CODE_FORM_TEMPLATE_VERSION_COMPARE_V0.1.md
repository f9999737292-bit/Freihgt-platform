# Low-code Form Template Version Compare v0.1

## Summary

Added frontend-only comparison of a DRAFT form template against the latest PUBLISHED version with the same tenant, entity type, and code. Includes a Compare tab on the admin detail page and a publish review modal with change summary before publishing.

## Scope

- **Frontend only:** compare helper, diff component, admin detail page, i18n
- **Data source:** existing admin list/detail APIs (`listAdminFormTemplates`, `getAdminFormTemplate`)
- **No backend business logic changes**
- **No API contract changes**
- **No migrations**
- **No drag-and-drop**

## Compare Model

`compareFormTemplates(baseTemplate, draftTemplate)` in `apps/web-admin/types/lowCode.ts`:

- Normalizes published detail and draft editor state via `adminDetailToCompareInput` / `draftToCompareInput`
- Compares template metadata: `name`, `description`, `version`
- Sections by `code`: added, removed, changed (`title`, `sort_order`)
- Fields by `code` within matching sections: added, removed, changed attributes:
  - `label`, `field_type`, `required`, `read_only`, `system_field`, `sort_order`
  - `options_json`, `validation_rule_json`, `visibility_rule_json` (stable key-sorted JSON stringify)

Output:

- `summary`: counts for added/removed/changed sections and fields
- `rows`: detailed diff rows with `type`, `area`, `code`, `before`, `after`

## Diff UI

Component: `apps/web-admin/components/low-code/LowCodeFormTemplateDiff.vue`

- Summary cards
- Grouped tables: Added / Removed / Changed
- JSON rendered in `<pre>` text (no `v-html`)
- Empty state: “No changes detected”
- Missing base: “No published base template found”

## Publish Review

On DRAFT detail page, **Publish** opens a review modal showing:

- Template code
- Draft version
- Base published version (if found)
- Compact diff summary
- Warning when no changes detected (does not block publish)
- Public API visibility warning
- **Cancel** / **Publish this draft**

Publish still uses existing `POST .../publish` endpoint unchanged.

## Guardrails

- Compare runs entirely on frontend using admin API data
- Published templates remain read-only; compare tab is shown only for DRAFT
- Base lookup: `listAdminFormTemplates({ entity_type, status: 'PUBLISHED' })` → same `code` → max `version` → `getAdminFormTemplate`
- If no published base exists, compare shows info message; publish remains allowed

## Verification Commands

```powershell
cd D:\Projects\freight-platform\apps\web-admin
npm run build
npm run dev
```

Manual:

1. Clone a published template to draft
2. Edit field label, add field, optionally change section
3. Save Draft
4. Open **Compare** tab — verify summary and rows
5. Click **Publish** — review modal shows summary; publish succeeds; template becomes read-only

Regression:

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo
make integration-smoke-test
```

## Known Limitations

- Compare uses latest PUBLISHED by `code` + max `version`, not clone source ID tracking
- Unsaved editor changes are included in compare/publish review (live draft state)
- Field moved between sections appears as removed + added
- ROUTE / VAT_TAX and other non-palette types compare normally if present
- No drag-and-drop reorder visualization

## Next Action

- Optional: track clone source template ID when backend exposes it
- Optional: warn in publish review when draft has unsaved changes
- Optional: side-by-side preview compare view

# Low-code Admin Template Import/Export UI v0.1

## Summary

Admin UI for exporting form templates to portable JSON and importing them through a four-step preview + execute wizard. Import always creates or updates a **DRAFT** only; publish and custom-value migration remain manual.

Backend APIs (unchanged in this pack):

- `GET /api/v1/low-code/admin/form-templates/{id}/export`
- `POST /api/v1/low-code/admin/form-templates/import-preview`
- `POST /api/v1/low-code/admin/form-templates/import`

## UI Entry Points

| Location | Action |
|----------|--------|
| `/low-code/admin/form-templates/[id]` | **Export JSON** in page header |
| `/low-code/admin/form-templates` | **Import template** opens wizard; list refreshes after successful import |
| `/low-code` hub (admin notice) | **Import template** shortcut (optional entry) |

## Export Flow

1. Open admin template detail (any status: DRAFT, PUBLISHED, ARCHIVED).
2. Click **Export JSON** (visible when `canExportTemplates()` / low-code admin permission).
3. UI calls export API, shows success toast.
4. Export panel offers:
   - **Download JSON** — filename `lowcode-template-{entity_type}-{code}-v{version}.json`
   - **Copy JSON** — clipboard helper
   - Collapsible `<pre>` preview (safe JSON text, no `v-html`)

Export envelope uses `schema_version: lowcode.template.export.v1`. No custom field values are included.

## Import Wizard

Component: `apps/web-admin/components/low-code/LowCodeTemplateImportWizard.vue`

| Step | Purpose |
|------|---------|
| 1 — Paste JSON | Textarea, optional file upload (512 KB guard), conflict strategy, optional target code |
| 2 — Preview | Calls `import-preview`; shows summary, predicted result, warnings/errors, raw JSON |
| 3 — Confirm | DRAFT-only notice; warnings checkbox when needed; explicit **Execute import** |
| 4 — Result | Draft id, entity type, code, version, status, counts; **Open draft** / **Import another** |

Validation on step 1:

- JSON parse errors shown clearly
- `schema_version` must be `lowcode.template.export.v1`
- Execute is blocked until preview succeeds

## Conflict Strategies

| Strategy | UI label | Behavior |
|----------|----------|----------|
| `NEW_VERSION` | New version (default) | Creates new DRAFT with next version |
| `REPLACE_EXISTING_DRAFT` | Replace existing draft | Replaces sections/fields on existing DRAFT |
| `FAIL_IF_EXISTS` | Fail if exists | Preview/execute returns 409 if template exists |

## Permissions

Uses existing low-code admin gates (`canAccessLowCodeAdmin`):

- Export: `canExportTemplates()`
- Import: `canImportTemplates()`

No new backend permission logic. Local dev/admin mode remains usable when auth is off.

## i18n

RU / EN / ZH keys under `lowCode.templateExport*` and `lowCode.templateImport*` in:

- `apps/web-admin/i18n/en-US.json`
- `apps/web-admin/i18n/ru-RU.json`
- `apps/web-admin/i18n/zh-CN.json`

## Safety Guardrails

- No `v-html`; JSON rendered in `<pre>` only
- Import never publishes
- Import never migrates custom values
- Import never changes active published template automatically
- Execute requires explicit click on step 3
- Warnings require confirmation checkbox
- File upload size capped at 512 KB
- Wrong `schema_version` shows clear error
- Existing admin template editor / publish flow unchanged

## Verification Commands

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo

cd apps\web-admin
npm run build
npm run dev
```

Manual UI (`admin@7rights.local` / `Admin123456!`):

1. Export from a PUBLISHED template detail — verify download/copy/preview and `schema_version`.
2. Import from admin list — paste exported JSON, preview, execute with `NEW_VERSION`, open draft, confirm publish is still manual.

Regression:

```powershell
cd D:\Projects\freight-platform
make integration-smoke-test
```

## What Is Not Implemented Yet

- Import/export edge-case automated tests
- Bulk ZIP import/export
- Auto-publish imported template
- Custom field value migration on import
- In-wizard audit event fetch (backend records audit; UI shows hint only)

## Next Action

Template Import/Export Edge Cases Test Pack v0.1

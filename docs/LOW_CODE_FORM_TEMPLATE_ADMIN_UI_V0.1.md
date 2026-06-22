# Low-code Form Template Admin UI v0.1

## Summary

Web-admin UI for managing **DRAFT** form templates via the admin API. This is a structured CRUD editor (sections + fields + JSON textareas), **not** a drag-and-drop Form Builder.

Published templates are read-only in the UI. Core entity data is unchanged. No migrations were created.

## Routes

| Route | Purpose |
|-------|---------|
| `/low-code/admin/form-templates` | Admin list with filters |
| `/low-code/admin/form-templates/new` | Create DRAFT |
| `/low-code/admin/form-templates/[id]` | View/edit DRAFT or read-only PUBLISHED/ARCHIVED |

Linked from `/low-code` hub dashboard.

## Admin API Used

- `GET /api/v1/low-code/admin/form-templates`
- `GET /api/v1/low-code/admin/form-templates/{id}`
- `POST /api/v1/low-code/admin/form-templates`
- `PUT /api/v1/low-code/admin/form-templates/{id}`
- `POST /api/v1/low-code/admin/form-templates/{id}/publish`

Public API unchanged:

- `GET /api/v1/low-code/form-templates` — PUBLISHED only

## Draft Lifecycle

1. **Create** on `/new` → POST admin API → redirect to detail page
2. **Edit** on `/[id]` when `status = DRAFT` → PUT admin API
3. **Publish** on detail page with confirmation modal → POST publish
4. **PUBLISHED / ARCHIVED** → read-only editor + warning banner

## Create Draft Flow

1. Open `/low-code/admin/form-templates/new`
2. Fill entity_type, code, name, description
3. Add section(s) and field(s)
4. For SELECT/MULTI_SELECT, provide `options_json` textarea
5. Click **Save draft**

## Edit Draft Flow

1. Open template from admin list
2. Modify metadata, sections, or fields
3. Click **Save draft**

## Publish Flow

1. Open DRAFT template detail
2. Click **Publish**
3. Confirm warning: template becomes visible in public form templates API
4. Status changes to PUBLISHED; editing disabled

## Public vs Admin Visibility

| UI | Shows |
|----|-------|
| `/low-code/form-templates` | PUBLISHED only (unchanged) |
| `/low-code/admin/form-templates` | DRAFT, PUBLISHED, ARCHIVED (tenant-scoped) |

## Guardrails

- Create/edit only through admin endpoints
- Client-side validation before submit (required fields, JSON parse)
- `system_field` not exposed in normal UI
- Publish confirmation required
- No core entity CRUD from this UI

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

- http://localhost:3000/low-code/admin/form-templates
- Create `shipment_custom_ui_v1` draft, edit, publish
- Verify `/low-code/form-templates` shows new PUBLISHED template only after publish

Regression:

```powershell
cd D:\Projects\freight-platform
make integration-smoke-test
```

## Known Limitations

- No drag-and-drop Form Builder
- No visual rule builder or live preview
- No template delete/archive UI
- `updated_at` not returned by admin list API (column shows published_at or —)
- Manual sort_order only (no drag reorder)

## Next Action

- Visual Form Builder with drag-and-drop
- Template archive/delete admin actions
- Show `updated_at` when added to admin list API
- Link published template to public read-only detail page

# Low-code Admin Migration Preview Modal v0.1

## Summary

Admin UI modal on the custom field values page for previewing and executing migration of stored custom field values to the active published form template. Preview is read-only; execute requires an explicit click with WARNING confirmation when applicable.

## UI Entry Point

Page: `/low-code/custom-field-values`

Button: **Migrate to active template** (RU: *Миграция к активному шаблону*, ZH: *迁移到活动模板*)

Location: lookup form actions row, next to reload / demo entity actions.

Enabled when:

- tenant is selected
- entity is loaded (`entity_type`, `entity_id`)
- active template code resolved via `GET /api/v1/low-code/form-templates/active`

Disabled with helper text *Select entity first* when prerequisites are missing.

## Preview Flow

1. User clicks **Migrate to active template**.
2. Modal opens in loading state.
3. UI calls `POST /api/v1/low-code/admin/custom-field-values/migration-preview` with:

```json
{
  "entity_type": "TRANSPORT_ORDER",
  "template_code": "transport_order_default",
  "entity_ids": ["<entity-uuid>"]
}
```

4. Summary shows entities checked / safe / warnings / blocked.
5. Current entity shows status badge (SAFE / WARNING / BLOCKED) and sections:
   - copied fields
   - legacy fields
   - missing required fields
   - incompatible fields
   - warnings
6. Empty arrays render as *None*.
7. Collapsible **Raw preview** shows safe JSON (`formatJsonValue`, no `v-html`).

## Execute Flow

1. User reviews preview (no auto-execute).
2. **SAFE** → **Migrate** button enabled (`allow_warnings: false`).
3. **WARNING** → checkbox + **Migrate with warnings** (`allow_warnings: true` after confirmation).
4. **BLOCKED** → execute disabled with blocked message.
5. On success → success state, custom values panel refresh, audit list refresh.
6. Modal closes only when user clicks **Close**.

Execute endpoint:

`POST /api/v1/low-code/admin/custom-field-values/migrate-to-active`

```json
{
  "entity_type": "TRANSPORT_ORDER",
  "template_code": "transport_order_default",
  "entity_id": "<entity-uuid>",
  "allow_warnings": false
}
```

## Warning Confirmation

WARNING status requires checkbox: *I understand warnings and want to continue* before execute is enabled.

409 `MIGRATION_WARNINGS_REQUIRE_CONFIRMATION` is handled in modal with user-friendly message; preview from error body is rendered when present.

## Blocked Migration

BLOCKED status disables execute. Message: *Migration is blocked. Fix incompatible fields first.*

409 `MIGRATION_BLOCKED` shows the same guidance; incompatible fields remain visible.

## API Endpoints Used

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/v1/low-code/form-templates/active` | Resolve active template code |
| POST | `/api/v1/low-code/admin/custom-field-values/migration-preview` | Read-only preview |
| POST | `/api/v1/low-code/admin/custom-field-values/migrate-to-active` | Execute migration |
| GET | `/api/v1/low-code/audit-events` | Refresh audit after execute |

Headers: `X-Tenant-ID` from tenant store (not hardcoded in UI).

## i18n

RU / EN / ZH keys under `lowCode.*` — migrate button, modal title, status labels, field sections, actions, errors.

Files:

- `apps/web-admin/i18n/en-US.json`
- `apps/web-admin/i18n/ru-RU.json`
- `apps/web-admin/i18n/zh-CN.json`

## Safety Guardrails

- No automatic execute on modal open.
- Preview is read-only.
- WARNING requires explicit checkbox.
- BLOCKED cannot execute.
- Legacy fields are shown, not hidden.
- No destructive language in UI.
- Raw JSON rendered as text only.

## Verification Commands

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo
cd apps\web-admin
npm run build
cd D:\Projects\freight-platform
make integration-smoke-test
```

Manual UI:

```text
http://localhost:3000/low-code/custom-field-values
```

Demo: TRANSPORT_ORDER / DEMO-TO-001 → open migration modal → preview → execute (if SAFE).

## What Is Not Implemented Yet

- Batch migration UI
- Migration history dedicated page
- Drag-and-drop form builder changes
- Backend business logic changes

## Next Action

**Low-code Migration History & Audit UI Enhancement Pack v0.1**

Alternative: **Low-code Batch Migration Design Pack v0.1**

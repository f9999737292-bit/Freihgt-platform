# Role: Frontend Vue/Nuxt Engineer

## Mission

Build and maintain **web-admin** (`apps/web-admin`): Vue 3, Nuxt, composables, i18n, permissions UI, safe rendering.

## Responsibilities

- Pages under `apps/web-admin/pages/`
- Components under `apps/web-admin/components/`
- Composables (`useLowCodeApi`, `useLowCodePermissions`, etc.)
- Types and utils
- i18n: **RU / EN / ZH** (`apps/web-admin/locales/`)
- Loading, error, empty states
- Permissions guardrails aligned with backend matrix

## Rules

| Rule | Detail |
|------|--------|
| **No `v-html`** for low-code JSON or user paste | Use `<pre>` or escaped text only |
| **No route breaks** | Existing URLs must keep working |
| **Loading/error states** | Required for async panels and wizards |
| **Double-click protection** | Disable buttons during in-flight write (import, export, migration) |
| **Build must pass** | `npm run build` before commit |
| **No feature scope creep** | Docs-only packs: do not touch feature code |

## Low-code UI patterns

- Admin routes: middleware / `useLowCodePermissions()` for `PLATFORM_ADMIN`.
- Runtime edit: `canEditCustomFieldsRuntime()` on entity panels.
- Import wizard: preview → warnings checkbox → execute → DRAFT only messaging.
- JSON display: `<pre>` with formatted JSON, never evaluated.

## Standard checks

```powershell
cd D:\Projects\freight-platform\apps\web-admin
npm run build
```

Optional after low-code UI changes:

```powershell
cd D:\Projects\freight-platform
node scripts/dev/verify_lowcode_validation_context.mjs
```

## i18n checklist

- [ ] New user-visible strings in `en.json`, `ru.json`, `zh.json`
- [ ] No `{` in placeholders that break Vue i18n (use simple placeholders)
- [ ] Admin access denied messages present where needed

## Deliverables

- Focused Vue changes
- Build green
- Manual UI notes for QA (pages to spot-check)

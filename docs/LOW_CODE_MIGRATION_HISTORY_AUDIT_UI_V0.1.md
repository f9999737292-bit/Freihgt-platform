# Low-code Migration History & Audit UI v0.1

## Summary

Enhanced low-code audit UI to present `CUSTOM_FIELD_VALUES_MIGRATED_TO_ACTIVE` events as readable migration cards with filters, collapsible raw JSON, and integration on the custom field values page (latest migration + history link).

## UI Pages Changed

| Page | Changes |
|------|---------|
| `/low-code/audit` | Card-based event list, quick action filters, migration-specific rendering |
| `/low-code/custom-field-values` | Latest migration block, migration history link, card-based recent audit list |

## Migration Event Rendering

Component: `LowCodeMigrationAuditCard.vue`

Displays for `CUSTOM_FIELD_VALUES_MIGRATED_TO_ACTIVE`:

- Human-readable title: *Migrated to active template*
- Entity type / entity ID
- Actor, request ID, created at
- Source / target template IDs
- Copied, legacy, missing required, incompatible fields, warnings
- Allow warnings flag and migration status (when present in payload)
- Collapsible **Raw event** JSON (safe text rendering, no `v-html`)

Wrapper: `LowCodeAuditEventCard.vue` routes migration events to the migration card; other events use a generic card with old/new values.

Parser: `parseMigrationAuditPayload()` in `types/lowCode.ts` — tolerant of missing/legacy payloads.

## Filters

Audit page filters:

- Entity type, entity ID, action, limit (existing)
- Quick filters:
  - **All actions**
  - **Value updates** → backend `action=CUSTOM_FIELD_VALUES_UPDATED`
  - **Template changes** → client-side filter on template draft/publish/clone actions
  - **Migrations** → backend `action=CUSTOM_FIELD_VALUES_MIGRATED_TO_ACTIVE`

Query params:

- `entity_type`, `entity_id`, `action`, `limit`
- `category=migrations|value_updates|template_changes` for quick filter prefill

## Custom Values Integration

After migration execute (existing modal flow):

- Refreshes audit list
- Shows **Latest migration** compact card when a migration event exists
- **View migration history** link → `/low-code/audit?entity_type=…&entity_id=…&category=migrations`

## i18n

RU / EN / ZH keys under `lowCode.audit*` and reused migration field labels.

## Safety Guardrails

- No `v-html`; JSON via `formatJsonValue` in `<pre>`
- Empty arrays → *None* / *Нет* / *无*
- Missing payload fields do not crash UI
- UUIDs in monospace with word-break
- Responsive grid layouts for narrow screens

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

Manual:

```text
http://localhost:3000/low-code/custom-field-values
http://localhost:3000/low-code/audit?category=migrations
```

## What Is Not Implemented Yet

- Batch migration UI / design
- Dedicated migration history timeline page
- Audit API currently returns empty `new_values` for migration events (payload stored in DB but not mapped by `ParseAuditValuesMap` on backend). UI is ready to render full payload when API exposes it.
- Drag-and-drop form builder changes

## Next Action

**Low-code Batch Migration Design Pack v0.1**

Alternative: **Low-code Migration Edge Cases Test Pack v0.1**

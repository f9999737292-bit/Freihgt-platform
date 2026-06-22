# Low-code Create-first-value Edit v0.1

## Summary

Entity detail and custom-field-values pages can now **create** custom field values when a published template exists but no values are stored yet. Uses existing `PUT` upsert — no new API endpoint.

## Behavior

- **Create values** button when template exists and `items.length === 0`
- Edit form driven by **template fields** (not stored items)
- Save sends all editable template fields via PUT
- Empty demo: **DEMO-TO-002** (transport order with template, no values)

## Panel changes

**File:** `apps/web-admin/components/low-code/LowCodeCustomFieldsPanel.vue`

- `canEdit` no longer requires `items.length > 0`
- `editableTemplateFields` from published template
- Edit form shown before empty-state message
- Deep link + preview unchanged

## Lookup page

`/low-code/custom-field-values` — **Use empty demo entity** resolves `DEMO-TO-002`.

## Verification

```powershell
make seed-demo-data
make seed-lowcode-demo
```

1. `/transport-orders` → **DEMO-TO-002** → **Create values**
2. Fill fields → **Save** → values appear, preview populated
3. Reload → **Edit** → update → Save (idempotent update)

## Limitations

- Requires published template for entity type
- Sends all editable template fields on save (empty optional fields as null)
- Complex types still use JSON textarea (except DATE/MONEY/MULTI_SELECT)

## Next action

1. Entity reference / FILE upload editors
2. Partial save (only changed fields)

See also: `docs/LOW_CODE_ENTITY_DETAIL_INLINE_EDIT_V0.1.md`, `docs/LOW_CODE_RICH_FIELD_EDITORS_V0.1.md`.

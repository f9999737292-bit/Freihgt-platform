# Low-code Custom Field Values — Preview Entity Status v0.1

## Summary

The `/low-code/custom-field-values` edit page accepts an **optional entity status** for form template preview. Status drives `context.entity_status` visibility and conditional required rules while editing custom fields.

## UI

**Page:** `apps/web-admin/pages/low-code/custom-field-values/index.vue`

- **Preset select** — common statuses per entity type (`PREVIEW_ENTITY_STATUS_PRESETS`)
- **Text input** — manual override (e.g. `IN_TRANSIT`)
- **Fetch status** — loads status from core entity API
- Auto-fetch on **Load values** and **Use demo entity**

Status is passed to `LowCodeCustomFieldsPanel` as `:entity-status`.

## API helper

**`useLowCodeApi().resolveEntityStatus(entityType, entityId)`**

| entity_type | Endpoint |
| ----------- | -------- |
| `TRANSPORT_ORDER` | `GET /api/v1/transport-orders/{id}` |
| `SHIPMENT` | `GET /api/v1/shipments/{id}` |
| `BILLING_REGISTER` | `GET /api/v1/billing-registers/{id}` |

Returns `status` or `null` on failure.

## Presets

**`PREVIEW_ENTITY_STATUS_PRESETS`** in `types/lowCode.ts` — dev-friendly shortcuts only; any string works in the text field.

## Verification

```powershell
make seed-lowcode-demo
cd apps/web-admin; npm run dev
```

1. Open `/low-code/custom-field-values`
2. **Use demo entity** → status auto-filled (e.g. `READY_FOR_SOURCING` for DEMO-TO-001)
3. Preview shows entity status chip; `internal_cost_center` visible per context rule
4. Change preset to `DRAFT` → preview updates; context-only fields may hide

## Limitations

- Status presets only for `LOW_CODE_ENTITY_TYPES` on this page (3 types)
- Does not change core entity status — preview context only
- Fetch requires entity to exist in core services

## Next action

1. Inline edit on entity detail (future write pack)
2. Server-side conditional required validation (future pack)

See also: `docs/LOW_CODE_PREVIEW_CONTEXT_V0.1.md`, `docs/LOW_CODE_CUSTOM_FIELD_VALUES_EDIT_UI_V0.1.md`.

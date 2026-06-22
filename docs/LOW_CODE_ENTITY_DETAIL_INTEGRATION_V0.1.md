# Low-code Entity Detail Integration v0.1

## Summary

Reusable read-only **LowCodeCustomFieldsPanel** embedded on existing entity detail pages in web-admin. Shows custom field values from the low-code API without write UI or backend changes.

## Component

**Path:** `apps/web-admin/components/low-code/LowCodeCustomFieldsPanel.vue`

**Props:**

| Prop | Type | Description |
| ---- | ---- | ----------- |
| `entityType` | `string` | e.g. `TRANSPORT_ORDER`, `SHIPMENT`, `BILLING_REGISTER` |
| `entityId` | `string \| null` | Entity UUID |
| `title` | `string` (optional) | Panel header override |

**Behavior:**

- Loads values via `useLowCodeApi().getCustomFieldValues(entityType, entityId)`
- Shows loading, empty, and API unavailable states
- Read-only badge in header
- Table: field code, formatted value, updated_at
- Value formatting: plain text for strings; compact JSON for objects/arrays

**Helpers:** `formatCustomFieldDisplayValue`, `isCustomFieldComplexValue` in `types/lowCode.ts`

## Integrated Pages

| Page | Route | entity_type | entity_id source |
| ---- | ----- | ----------- | ---------------- |
| Transport order detail | `/transport-orders/[id]` | `TRANSPORT_ORDER` | `order.id` |
| Shipment detail | `/shipments/[id]` | `SHIPMENT` | `shipment.id` |
| Billing register detail | `/billing-registers/[id]` | `BILLING_REGISTER` | `item.id` |

All three detail pages existed before this pack; integration is a single panel insert per page when entity is loaded.

**Not integrated:** RFx, documents, companies, users — no low-code demo values seeded for those entity types in v0.1.

## API Used

```http
GET /api/v1/low-code/custom-field-values?entity_type={type}&entity_id={uuid}
X-Tenant-ID: {tenant_id}
```

Same gateway and tenant conventions as other web-admin APIs.

## Read-only Scope

**Included:**

- Display stored custom field values on entity cards
- Auto-reload when `entityId` changes
- Retry on API unavailable

**Excluded:**

- Edit / create / delete custom fields
- Inline form controls
- Form Builder UI
- Backend or API contract changes

## Limitations

- Only shows fields with stored values (empty list if none seeded)
- Requires valid tenant and entity UUID
- Low-code service must be reachable (503 → unavailable state)
- Demo values only for entities created by `make seed-demo-data` + `make seed-lowcode-demo`
- Transport order / billing register detail pages are minimal; panel appears below main card

## Verification Commands

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo

cd apps/web-admin
$env:NUXT_IGNORE_LOCK=1; npm run build
npm run dev
```

**Browser (after login, dev tenant):**

1. Open transport order **DEMO-TO-001** → custom fields panel shows `cargo_class`, `internal_cost_center`
2. Open shipment **DEMO-SH-PLANNED** → panel shows `temperature_mode`
3. Open billing register **DEMO-BR-001** → panel shows `payment_priority`
4. Confirm `/low-code/custom-field-values` still works

**Demo refs:** resolve entity IDs from list pages or use IDs printed by `make seed-demo-data`.

## Next Action

1. Enrich detail pages with links to matching form template (`/low-code/form-templates/{code}`)
2. Show field labels from published template metadata (not just field_code)
3. Integrate on RFx / document pages when low-code seeds exist
4. Inline edit custom fields (future write pack)

See also: `docs/LOW_CODE_ADMIN_UI_PREVIEW_V0.1.md`, `docs/LOW_CODE_CUSTOM_FIELD_VALUES_API_V0.1.md`.

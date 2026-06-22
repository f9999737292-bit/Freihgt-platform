# Low-code Entity Detail Preview v0.1

## Summary

Entity detail pages show custom field values with **labels from the published form template**, a link to the public template detail page, and a read-only **form template preview** with current values. Fixes incorrect component name on three entity pages.

## Changes

### Component fix (entity detail pages)

Replaced typo `LowCodeLowCodeCustomFieldsPanel` with `LowCodeCustomFieldsPanel`:

| Page | Route |
| ---- | ----- |
| Transport order | `/transport-orders/[id]` |
| Shipment | `/shipments/[id]` |
| Billing register | `/billing-registers/[id]` |

### LowCodeCustomFieldsPanel enhancements

**Path:** `apps/web-admin/components/low-code/LowCodeCustomFieldsPanel.vue`

- **Label column** in values table — uses published template field metadata (`label`), falls back to `field_code`
- **Template link** in header — `/low-code/form-templates/{id}` when a published template is resolved
- **Preview block** — `LowCodeFormTemplatePreview` with current values (prop `showPreview`, default `true`)
- Resolves published template via existing `resolvePublishedTemplate(entityType)`

Entity detail pages use defaults: `editable=false`, `showPreview=true`.

### Audit UI

**Path:** `apps/web-admin/pages/low-code/audit/index.vue`

Filter options extended:

- `FORM_TEMPLATE_DRAFT_CREATED`
- `FORM_TEMPLATE_DRAFT_UPDATED`
- `FORM_TEMPLATE_DRAFT_PUBLISHED`

Default action filter is **All** (empty).

### Admin template list

**Path:** `apps/web-admin/pages/low-code/admin/form-templates/index.vue`

When `status === PUBLISHED`, actions column includes link to public template detail (`/low-code/form-templates/{id}`).

## Read-only scope

- Entity detail pages remain read-only for custom fields
- Edit flow unchanged on `/low-code/custom-field-values` (`editable=true` on panel)

## Verification

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo

cd apps/web-admin
$env:NUXT_IGNORE_LOCK=1; npm run build
npm run dev
```

**Browser (dev tenant):**

1. Open transport order **DEMO-TO-001** — custom fields panel with labels, template link, preview
2. Open shipment **DEMO-SH-PLANNED** — same panel behavior
3. Open billing register **DEMO-BR-001** — same panel behavior
4. `/low-code/audit` — filter by form template draft/publish events
5. `/low-code/admin/form-templates` — published rows link to public template detail

## Next action

1. RFx / document entity pages when low-code seeds exist
2. Inline edit on entity detail (future write pack)
3. Preview empty-state when template exists but no values stored

See also: `docs/LOW_CODE_ENTITY_DETAIL_INTEGRATION_V0.1.md`, `docs/LOW_CODE_FORM_TEMPLATE_PREVIEW_RENDERER_V0.1.md`.

# Low-code Entity Detail — RFx / Document / Freight Request v0.1

## Summary

Extends read-only custom fields integration to **freight request**, **document**, and **RFX** detail pages. Adds dev-only demo seeds (published templates + values) and shows **form template preview** even when no values are stored yet.

## Demo seeds

**Script:** `scripts/dev/seed_lowcode_demo.sh` (via `make seed-lowcode-demo`)

| Entity type | Template code | Demo entity | Sample fields |
| ----------- | ------------- | ----------- | ------------- |
| `FREIGHT_REQUEST` | `freight_request_default` | DEMO-FR-001 | `lane_priority`, `special_instructions` |
| `DOCUMENT` | `document_default` | DEMO-DOC-001 | `archive_reference`, `document_category` |
| `RFX` | `rfx_default` | DEMO-RFX-001 | `evaluation_criteria`, `confidentiality_level` |

Requires `make seed-demo-data` first.

## Integrated pages

| Page | Route | entity_type |
| ---- | ----- | ----------- |
| Freight request detail | `/freight-requests/[id]` | `FREIGHT_REQUEST` |
| Document detail | `/documents/[id]` | `DOCUMENT` |
| RFX event detail | `/rfx/[id]` | `RFX` |

Uses `LowCodeCustomFieldsPanel` with defaults (`editable=false`, `showPreview=true`).

## Preview empty-state

When a published template exists but no custom field values are stored:

- Panel shows hint: *No custom field values stored yet*
- `LowCodeFormTemplatePreview` renders template layout with empty fields
- Header link to `/low-code/form-templates/{id}` remains available

Example: open transport order **DEMO-TO-002** (no seeded values) after `make seed-lowcode-demo`.

## Verification

```powershell
cd D:\Projects\freight-platform
make seed-demo-data
make seed-lowcode-demo
make health-check

cd apps/web-admin
$env:NUXT_IGNORE_LOCK=1; npm run build
npm run dev
```

**Browser (dev tenant):**

1. `/freight-requests` → **DEMO-FR-001** — custom fields + preview
2. `/documents` → **DEMO-DOC-001** — custom fields + preview
3. `/rfx` → **DEMO-RFX-001** — custom fields + preview
4. `/transport-orders` → **DEMO-TO-002** — template-only preview (no values)

## Next action

1. Server-side conditional required validation (future pack)
2. Create-first-value edit flow for empty entities
3. RFx lot / bid entity types when templates are needed

See also: `docs/LOW_CODE_ENTITY_DETAIL_INLINE_EDIT_V0.1.md`, `docs/LOW_CODE_PREVIEW_VISIBILITY_RULES_V0.1.md`.

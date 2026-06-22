# Low-code Preview Visibility Rules v0.1

## Summary

`LowCodeFormTemplatePreview` evaluates declarative **visibility_rule_json** on each field and hides fields (and empty sections) that do not match current preview values. Client-side only — no backend or API changes.

## Supported DSL (v0.1)

Field-level rule on the field being shown/hidden:

```json
{
  "if": {
    "field": "cargo_class",
    "equals": "GENERAL"
  }
}
```

| Condition | Meaning |
| --------- | ------- |
| `field` + `equals` | Another field value equals literal |
| `field` + `not_equals` | Value differs |
| `field` + `in` | Value in array |
| `field` + `not_in` | Value not in array |
| `context.role` | Matches optional preview context |
| `context.entity_status` | String or `{ "in": [...] }` |

Empty `{}` or missing `if` → field always visible.

## Implementation

**Helpers:** `apps/web-admin/types/lowCode.ts`

- `isPreviewFieldVisible`
- `filterPreviewSectionsForVisibility`
- `evaluatePreviewVisibilityCondition`

**Component:** `LowCodeFormTemplatePreview.vue`

- Optional prop `previewContext?: { role?, entity_status? }`
- Hint when fields hidden: *N field(s) hidden by visibility rules*
- Preview model includes `visibility_rule_json` from API / draft editor

## Demo seeds

`make seed-lowcode-demo` sets:

| Field | Entity | Rule |
| ----- | ------ | ---- |
| `loading_window_note` | TRANSPORT_ORDER | visible when `cargo_class` = `GENERAL` |
| `driver_comment` | SHIPMENT | visible when `temperature_mode` in `FROZEN`, `CHILLED` |

**Verify:**

- **DEMO-TO-001** — `loading_window_note` visible (cargo_class=GENERAL)
- **DEMO-TO-002** — `loading_window_note` hidden (no values / not GENERAL)
- **DEMO-SH-PLANNED** — `driver_comment` hidden (temperature_mode=AMBIENT)

Admin draft editor live preview reacts to `visibility_rule_json_text` when other field values are present in preview values.

## Verification

```powershell
cd D:\Projects\freight-platform
make seed-lowcode-demo

cd apps/web-admin
$env:NUXT_IGNORE_LOCK=1; npm run build
npm run dev
```

## Limitations

- Preview only (not enforced on save/API)
- No rule engine / cross-field `then.visible` lists
- Context rules need `previewContext` prop (not wired on entity pages yet)
- Validation / read-only rules not evaluated in preview

## Next action

1. Pass `previewContext` from entity detail pages (status, role)
2. Conditional required indicator in preview
3. Inline edit on entity detail (future write pack)

See also: `docs/LOW_CODE_FORM_TEMPLATE_PREVIEW_RENDERER_V0.1.md`, `docs/LOW_CODE_CUSTOM_FIELDS_TECHNICAL_DESIGN_V0.1.md`.

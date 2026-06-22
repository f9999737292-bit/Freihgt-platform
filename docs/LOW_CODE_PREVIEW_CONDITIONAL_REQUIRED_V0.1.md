# Low-code Preview Conditional Required v0.1

## Summary

Form template preview evaluates **conditional required** rules from `validation_rule_json` (`if` / `then.required`) and shows badges plus missing-value hints. Preview-only — no save/API enforcement.

## Rule DSL

Cross-field example on `cargo_class`:

```json
{
  "if": { "field": "cargo_class", "in": ["A", "B", "C"] },
  "then": { "required": ["loading_window_note"] }
}
```

Self-required example:

```json
{
  "if": { "field": "temperature_mode", "equals": "FROZEN" },
  "then": { "required": true }
}
```

Uses the same condition evaluator as visibility rules (`field`, `equals`, `in`, `context.entity_status`, `context.role`).

Static `field.required=true` still shows the red **Required** badge. Conditional rules show amber **Required (conditional)**.

## Implementation

**Helpers:** `apps/web-admin/types/lowCode.ts`

- `collectConditionallyRequiredFields`
- `resolvePreviewFieldRequiredState`
- `countMissingRequiredPreviewFields`

**UI:** `LowCodeFormTemplatePreview.vue`

- Conditional required badge
- Field highlight + hint when required (static or conditional) and value empty
- Summary: *N required field(s) missing values*

Preview model includes `validation_rule_json` from API and draft editor.

## Demo seeds

`make seed-lowcode-demo`:

| Source field | When | Required target |
| ------------ | ---- | --------------- |
| `cargo_class` | value in `A`, `B`, `C` | `loading_window_note` |
| `temperature_mode` | `FROZEN` or `CHILLED` | `driver_comment` |

**Verify:**

1. **DEMO-TO-001** (`cargo_class=GENERAL`) — no conditional required on `loading_window_note`
2. `/low-code/custom-field-values` → load DEMO-TO-001 → change `cargo_class` to **A** → preview shows conditional required on `loading_window_note`; clear note → missing hint
3. **DEMO-SH-PLANNED** (`AMBIENT`) — no conditional required on `driver_comment`

## Verification

```powershell
cd D:\Projects\freight-platform
make seed-lowcode-demo

cd apps/web-admin
$env:NUXT_IGNORE_LOCK=1; npm run build
npm run dev
```

## Limitations

- Simple `minLength` / `max` rules in `validation_rule_json` are ignored by conditional-required evaluator (backend still validates on save)
- Rules with `then.required` lists only; no full rule engine
- Not enforced on PUT custom-field-values

## Next action

1. Inline edit on entity detail (future write pack)
2. Server-side conditional required validation (future validation pack)

See also: `docs/LOW_CODE_CUSTOM_FIELD_VALUES_PREVIEW_STATUS_V0.1.md`, `docs/LOW_CODE_PREVIEW_CONTEXT_V0.1.md`.

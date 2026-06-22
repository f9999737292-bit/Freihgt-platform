# Low-code Drag-and-drop Form Builder v0.1

## Summary

Added native HTML5 drag-and-drop reordering for form template sections and fields in `LowCodeFormTemplateEditor.vue`. No new npm dependencies. Move up/down buttons remain as keyboard/fallback controls.

## Scope

- **Frontend only:** editor component, sort-order helpers, i18n
- **No backend business logic changes**
- **No API contract changes**
- **No migrations**
- **No new npm packages**
- **Published templates:** drag disabled (read-only mode)

## Section Reordering

- Drag handle (`≡`) on each section header
- Drop on another section to reorder
- Visual states: dragging (opacity), drop target (dashed outline)
- Collapse/expand preserved
- Move up / move down buttons as fallback
- After reorder: `reindexSectionSortOrders()` → 100, 200, 300…

## Field Reordering

- Drag handle on each field card header
- Drop on another field (insert before target) or on empty fields list (“Drop here”)
- Move up / move down preserved
- Duplicate / remove unchanged
- After reorder: `reindexFieldSortOrders()` → 100, 200, 300…
- Collapsed sections: field drop disabled (fields hidden)

## Cross-section Movement

**Implemented in v0.1:** fields can be dragged from one section to another.

- Field removed from source section
- Inserted into target section (before target field or append)
- Both sections reindexed when source ≠ target

## Sort Order Strategy

- Sections: `100 + index * 100`
- Fields: `100 + index * 100`
- `nextFieldSortOrder()` uses max + 100 for new fields
- Save Draft persists `sort_order` via existing draft payload API
- Preview follows draft `sort_order` through `draftToPreviewModel()`

## Read-only Guardrails

- No drag handles in readonly mode (PUBLISHED / ARCHIVED)
- Hint: “Drag disabled in read-only mode”
- Clone to Draft unchanged

## Verification Commands

```powershell
cd D:\Projects\freight-platform\apps\web-admin
npm run build
npm run dev
```

Manual:

1. New draft — create 2 sections, add fields, drag reorder, Save Draft
2. Edit draft — drag sections/fields, Save Draft, reload, verify order + preview
3. Published — drag disabled, Clone to Draft works

Regression:

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo
make integration-smoke-test
```

## Known Limitations

- Native HTML5 DnD only (no touch-optimized library)
- No multi-select drag
- No undo/redo
- Drop into collapsed section fields not supported
- Section drag uses whole-section drop target (not insert-between indicator)

## Next Action

- Optional: touch-friendly DnD polyfill/library if mobile editing is required
- Optional: insert-between line indicator for finer drop placement
- Optional: unsaved-changes warning before navigation after reorder

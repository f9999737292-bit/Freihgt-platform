# Low-code Admin Batch Migration Wizard v0.1

Date: 2026-06-23  
Project: `D:\Projects\freight-platform`  
Related:

- `docs/LOW_CODE_BATCH_MIGRATION_PREVIEW_API_V0.1.md`
- `docs/LOW_CODE_BATCH_MIGRATION_EXECUTE_API_V0.1.md`
- `docs/LOW_CODE_ADMIN_MIGRATION_PREVIEW_MODAL_V0.1.md`

---

## Summary

Admin UI wizard for batch preview and execute of custom field value migration onto the active published template. Supports up to 100 entity IDs, SAFE/WARNING/BLOCKED preview summary, confirmation guardrails, partial success results, and audit navigation.

**Frontend only** — reuses existing batch preview and batch execute APIs.

---

## UI Entry Point

Page: `http://localhost:3000/low-code/custom-field-values`

Button: **Batch migration** — next to **Migrate to active template**.

Enabled when:

- Tenant is selected
- `entity_type` is set
- Active `template_code` resolved for the entity type

Does **not** require a loaded entity ID (optional prefill via **Use current entity**).

---

## Wizard Steps

| Step | Name | Description |
|------|------|-------------|
| 1 | Select entities | Textarea of UUIDs (one per line), validation |
| 2 | Preview | Batch preview summary + filterable table |
| 3 | Confirm | Warning/blocked checkboxes + execute |
| 4 | Execute result | `batch_id`, summary, per-entity results, audit link |

Component: `apps/web-admin/components/low-code/LowCodeBatchMigrationWizard.vue`

---

## Entity Selection

- Textarea: one UUID per line
- **Use current entity** — adds page entity ID if present
- Validation: min 1, max 100, UUID format
- Invalid lines shown with error text
- `entity_type` and `template_code` from page context (no hardcoded tenant)

---

## Batch Preview Flow

```http
POST /api/v1/low-code/admin/custom-field-values/batch-migration-preview
```

Renders:

- Summary cards: Total, Safe, Warnings, Blocked
- Table with counts per entity
- Filters: All / Safe / Warnings / Blocked
- Expandable row details (fields + raw JSON in `<pre>`, no `v-html`)

Preview is read-only.

---

## Confirmation Rules

| Condition | UI behavior |
|-----------|---------------|
| Any WARNING | Checkbox required before execute |
| Any BLOCKED | **Skip blocked entities** checkbox (default checked) |
| All BLOCKED | Execute disabled + message |
| `skip_blocked=false` with blocked > 0 | Hint that batch will fail on execute (API returns 409) |

Execute button labels adapt: default / with warnings / skip blocked.

---

## Batch Execute Flow

```http
POST /api/v1/low-code/admin/custom-field-values/batch-migrate-to-active
```

Payload includes `allow_warnings` and `skip_blocked` from confirm step.

Result step shows:

- `batch_id`, batch `status`
- Summary: total, migrated, skipped, blocked, failed, warnings
- Per-entity items table
- Link to migration audit history
- Wizard stays open after success

If current loaded entity was migrated, custom values panel refreshes.

---

## Error Handling

| Case | Behavior |
|------|----------|
| Network / 500 | User-friendly message, retry from step 1 |
| 400 validation | Inline validation on step 1 |
| 409 `BATCH_MIGRATION_BLOCKED` | Message + preview if returned |
| 409 `BATCH_MIGRATION_WARNINGS_REQUIRE_CONFIRMATION` | Message + preview if returned |

No raw stack traces. API error `preview` payload rendered when present.

---

## i18n

RU / EN / ZH keys under `lowCode.batchMigration*` in:

- `apps/web-admin/i18n/en-US.json`
- `apps/web-admin/i18n/ru-RU.json`
- `apps/web-admin/i18n/zh-CN.json`

---

## Safety Guardrails

- Preview read-only; execute requires explicit click
- WARNING requires checkbox confirmation
- BLOCKED never silently migrated (skip or batch fail)
- Max 100 entities validated in UI
- Safe JSON in `<pre>` only — no `v-html`
- UUIDs wrap on narrow screens; table scrolls horizontally

---

## Verification Commands

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo

cd apps\web-admin
npm run build
npm run dev
```

Manual check:

1. Login at `http://localhost:3000/login`
2. Open `/low-code/custom-field-values`
3. Select `TRANSPORT_ORDER`, load demo entity or open batch without load
4. **Batch migration** → paste `2db04b49-665c-469f-bcb1-ffeb1274fedb`
5. Preview → Confirm → Execute
6. Verify `batch_id`, summary, audit link
7. Single-entity migration modal still works

---

## What Is Not Implemented Yet

| Item | Status |
|------|--------|
| Multi-select from entity list / table picker | Future |
| Background batch jobs (>100) | Future |
| Batch-level audit UI | Future pack |
| Strict preview token binding | Future |

---

## Next Action

**Low-code Batch Migration Audit & Metrics Pack v0.1** — batch-level audit events, metrics, and enhanced audit UI filters by `batch_id`.

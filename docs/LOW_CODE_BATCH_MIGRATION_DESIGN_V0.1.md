# Low-code Batch Migration Design v0.1

Date: 2026-06-23  
Project: `D:\Projects\freight-platform`  
Status: **Design document only — no code, no migrations, no API implementation**  
Related:

- `docs/LOW_CODE_MIGRATE_TO_ACTIVE_DESIGN_V0.1.md`
- `docs/LOW_CODE_MIGRATE_TO_ACTIVE_PREVIEW_API_V0.1.md`
- `docs/LOW_CODE_MIGRATE_TO_ACTIVE_EXECUTE_V0.1.md`
- `docs/LOW_CODE_ADMIN_MIGRATION_PREVIEW_MODAL_V0.1.md`
- `docs/LOW_CODE_MIGRATION_HISTORY_AUDIT_UI_V0.1.md`
- `docs/LOW_CODE_MIGRATION_EDGE_CASES_TEST_PACK_V0.1.md`

---

## Summary

This document defines the **batch migration** flow for remapping custom field values from older published template versions onto the **active** published template for many entities at once. Batch migration reuses the existing single-entity preview matching engine (`field_code` remap, SAFE / WARNING / BLOCKED), adds explicit batch orchestration, safety limits, partial-success rules, audit correlation, and a future Admin UI wizard.

**v0.1 design scope:** specification only. No backend, frontend, or database changes in this pack.

---

## Why Batch Migration Is Needed

After publishing a new active form template version, operators may have hundreds or thousands of entities with values still bound to older `field_id` / `form_template_id` rows. Single-entity migration (modal + execute API) is correct but impractical at scale:

| Pain | Batch solution |
|------|----------------|
| Repetitive admin clicks per entity | One preview + one execute for N entities |
| No aggregate visibility | Summary: safe / warning / blocked counts |
| Inconsistent operator decisions | Uniform `allow_warnings` / `skip_blocked` policy per batch |
| Hard to audit bulk operations | Batch-level audit + per-entity audit correlation via `batch_id` |
| Risk of unbounded load | Hard cap (100 sync) + future background job for larger sets |

Batch migration does **not** replace single-entity migration — it complements it for mass remediation after template activation.

---

## Scope

| In scope (design) | Description |
|-------------------|-------------|
| Proposed batch preview API contract | Read-only, up to 100 entities sync |
| Proposed batch execute API contract | Multi-entity execute with partial success |
| Safety limits and status model | Max entities, preview-before-execute |
| Partial success / skip_blocked strategy | Per-entity isolation |
| Idempotency and retry model | Re-run semantics |
| Audit and observability design | Batch + entity events, metrics |
| Future Admin UI batch wizard flow | Table, filters, confirmation |
| Rollout and verification plan | Implementation pack sequence |

---

## Out of Scope

| Out of scope | Reason |
|--------------|--------|
| Implementation of batch APIs | Future packs |
| Admin UI batch wizard | Future pack |
| Background job / queue infrastructure | v0.2+ when >100 entities |
| Automatic migration on template publish | Explicit admin action only |
| Core entity data changes | Low-code layer only |
| Silent deletion of legacy values | Policy: legacy remains readable |
| DB schema migrations | Not required for v0.1 batch design |
| Changes to existing single-entity API contracts | Additive batch endpoints only |

---

## Current Single-Entity Capabilities

Already shipped (commit lineage through `c1f5619`):

| Capability | Endpoint / UI | Notes |
|------------|-----------------|-------|
| Multi-entity **preview** (sync, max 100) | `POST .../migration-preview` | `entity_ids[]`, per-entity SAFE/WARNING/BLOCKED |
| Single-entity **execute** | `POST .../migrate-to-active` | `allow_warnings`, 409 on blocked/warning |
| Preview matching engine | `BuildMigrationPreviewItem` | field_code, type compatibility, legacy/missing/incompatible |
| Admin modal | `/low-code/custom-field-values` | Preview + execute for one entity |
| Migration audit UI | `/low-code/audit?category=migrations` | `CUSTOM_FIELD_VALUES_MIGRATED_TO_ACTIVE` per entity |
| Edge-case tests | `go test ./...` | Idempotency, tenant isolation, blocked no-write |
| Dedicated entity audit event | `CUSTOM_FIELD_VALUES_MIGRATED_TO_ACTIVE` | Written in same transaction as value writes |

**Gap:** preview accepts batch `entity_ids`, but execute accepts only **one** `entity_id`. Batch execute orchestration, batch audit, and batch UI are missing.

---

## Proposed Batch Preview API

Formal batch endpoint (may internally delegate to existing preview service logic):

```http
POST /api/v1/low-code/admin/custom-field-values/batch-migration-preview
```

### Request

```json
{
  "entity_type": "TRANSPORT_ORDER",
  "template_code": "transport_order_default",
  "entity_ids": [
    "2db04b49-665c-469f-bcb1-ffeb1274fedb",
    "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
  ],
  "target_template_id": null
}
```

| Field | Rule |
|-------|------|
| `entity_type` | Required |
| `template_code` | Required when multiple active templates exist for entity type |
| `entity_ids` | Required; 1..100 UUIDs (sync limit) |
| `target_template_id` | Optional; tenant-scoped published template |

### Response

```json
{
  "batch_id": "550e8400-e29b-41d4-a716-446655440000",
  "tenant_id": "74519f22-ff9b-4a8b-8fff-a958c689682f",
  "entity_type": "TRANSPORT_ORDER",
  "target_template": {
    "id": "b1111111-1111-4111-8111-111111111102",
    "code": "transport_order_default",
    "version": 2
  },
  "summary": {
    "total": 2,
    "safe": 1,
    "warnings": 1,
    "blocked": 0
  },
  "items": [
    {
      "entity_id": "2db04b49-665c-469f-bcb1-ffeb1274fedb",
      "source_template_id": "...",
      "target_template_id": "...",
      "status": "SAFE",
      "copied_fields": ["cargo_class", "internal_cost_center"],
      "legacy_fields": [],
      "missing_required_fields": [],
      "incompatible_fields": [],
      "warnings": []
    },
    {
      "entity_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "status": "WARNING",
      "copied_fields": ["note"],
      "legacy_fields": ["deprecated_field"],
      "missing_required_fields": ["required_note"],
      "incompatible_fields": [],
      "warnings": ["TEXT to SELECT conversion"]
    }
  ]
}
```

**Design notes:**

- `batch_id` is optional in v0.1 implementation but **recommended** for execute correlation and audit. Server-generated UUID; client may omit on first preview.
- Response shape mirrors existing `migration-preview` per-item fields; summary keys align (`total` = `entities_checked` alias for batch clarity).
- **Relationship to existing API:** `migration-preview` remains supported; `batch-migration-preview` adds `batch_id` and batch-oriented summary naming. Implementation may alias internally to one service method.

---

## Proposed Batch Execute API

```http
POST /api/v1/low-code/admin/custom-field-values/batch-migrate-to-active
```

### Request

```json
{
  "entity_type": "TRANSPORT_ORDER",
  "template_code": "transport_order_default",
  "entity_ids": [
    "2db04b49-665c-469f-bcb1-ffeb1274fedb",
    "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
  ],
  "target_template_id": null,
  "allow_warnings": false,
  "skip_blocked": true,
  "batch_id": "550e8400-e29b-41d4-a716-446655440000",
  "dry_run_token": null
}
```

| Field | Rule |
|-------|------|
| `entity_ids` | Required; 1..100 UUIDs |
| `allow_warnings` | Default `false`; must be `true` if any selected entity is WARNING |
| `skip_blocked` | Default `true`; when `true`, BLOCKED entities are skipped (not failed batch-wide) |
| `batch_id` | Optional; if provided, must match recent preview batch (future strict mode) |
| `dry_run_token` | Reserved for future execute dry-run after preview |

### Response

```json
{
  "batch_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "partially_completed",
  "tenant_id": "74519f22-ff9b-4a8b-8fff-a958c689682f",
  "entity_type": "TRANSPORT_ORDER",
  "target_template": {
    "id": "b1111111-1111-4111-8111-111111111102",
    "code": "transport_order_default",
    "version": 2
  },
  "summary": {
    "total": 3,
    "migrated": 1,
    "skipped": 1,
    "blocked": 1,
    "failed": 0
  },
  "items": [
    {
      "entity_id": "2db04b49-665c-469f-bcb1-ffeb1274fedb",
      "status": "migrated",
      "migrated_count": 3,
      "copied_fields": ["cargo_class", "internal_cost_center", "loading_window_note"],
      "legacy_fields": [],
      "warnings": []
    },
    {
      "entity_id": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
      "status": "skipped",
      "reason": "BLOCKED",
      "incompatible_fields": [{ "field_code": "amount", "reason": "NUMBER to MONEY is incompatible" }]
    },
    {
      "entity_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "status": "skipped",
      "reason": "WARNINGS_REQUIRE_CONFIRMATION",
      "legacy_fields": ["deprecated_field"]
    }
  ]
}
```

### Batch-level `status` values

| Status | Meaning |
|--------|---------|
| `completed` | All non-blocked entities migrated; no warnings or all warnings allowed |
| `completed_with_warnings` | All eligible entities migrated; at least one had WARNING and `allow_warnings=true` |
| `partially_completed` | Some migrated, some skipped (blocked and/or warning without confirmation) |
| `failed` | Zero entities migrated; e.g. all blocked, all warnings without `allow_warnings`, or systemic error |

Per-entity item `status` values: `migrated`, `migrated_with_warnings`, `skipped`, `failed`.

---

## Request/Response Contracts

### Compatibility with existing APIs

| Existing | Batch proposal | Change type |
|----------|----------------|-------------|
| `POST .../migration-preview` | `POST .../batch-migration-preview` | Additive; existing unchanged |
| `POST .../migrate-to-active` | `POST .../batch-migrate-to-active` | Additive; single-entity execute unchanged |
| Per-entity preview item shape | Same fields | Reuse |
| 409 `MIGRATION_BLOCKED` / `MIGRATION_WARNINGS_REQUIRE_CONFIRMATION` | Single-entity only | Batch returns 200 with per-item skip reasons |

### Error responses (batch)

| HTTP | Code | When |
|------|------|------|
| 400 | `TENANT_REQUIRED` | Missing `X-Tenant-ID` |
| 400 | `ENTITY_TYPE_INVALID` | Invalid entity type |
| 400 | `ENTITY_ID_INVALID` | Malformed UUID in list |
| 400 | `VALIDATION_ERROR` | Empty `entity_ids`, >100 ids, ambiguous template |
| 404 | `FORM_TEMPLATE_NOT_FOUND` | No active template |
| 409 | `BATCH_MIGRATION_PREVIEW_REQUIRED` | Future: execute without recent preview (optional strict mode) |

Batch execute should **not** fail the entire HTTP request when individual entities are blocked — use `skip_blocked` and per-item results unless a systemic failure occurs.

---

## Safety Limits

| Limit | Value | Rationale |
|-------|-------|-----------|
| Max `entity_ids` (sync preview + execute) | **100** | Matches `MaxMigrationPreviewEntityCount`; bounded DB/API load |
| Larger batches | Background job (future) | Async job with progress polling; not in v0.1 |
| Preview before execute | **Required** (recommended strict; soft in v0.1) | Operator must review aggregate risk |
| Execute reuses preview logic | **Required** | Same `BuildMigrationPreviewItem` + execute path per entity |
| BLOCKED entities | **Never migrated** | Even with `allow_warnings=true` |
| WARNING entities | **`allow_warnings=true` required** | Same as single-entity |
| Partial success | **Only with `skip_blocked=true`** (default) | Blocked skipped; warnings skipped unless confirmed |
| Per-entity transaction | **Required** | One entity failure must not roll back others |
| Idempotency | **Required** | Re-run stable for already-migrated entities |
| Silent deletion | **Forbidden** | Legacy fields remain in storage |
| Core entity changes | **Forbidden** | No transport order / shipment status updates |

---

## Status Model

### Preview (per entity) — unchanged

`SAFE` | `WARNING` | `BLOCKED`

### Batch preview summary

| Field | Description |
|-------|-------------|
| `total` | Entities in request |
| `safe` | Count with SAFE |
| `warnings` | Count with WARNING |
| `blocked` | Count with BLOCKED |

### Batch execute summary

| Field | Description |
|-------|-------------|
| `total` | Entities in request |
| `migrated` | Successfully written (includes `migrated_with_warnings`) |
| `skipped` | Not migrated (blocked, or warning without confirmation) |
| `blocked` | Subset of skipped due to BLOCKED |
| `failed` | Unexpected errors (DB, internal) |

---

## Warning and Blocked Handling

| Entity preview status | `allow_warnings=false` | `allow_warnings=true` | `skip_blocked=true` |
|-----------------------|------------------------|----------------------|---------------------|
| SAFE | Migrate | Migrate | Migrate |
| WARNING | Skip (reason: confirmation required) | Migrate (`migrated_with_warnings`) | Same |
| BLOCKED | Skip | Skip (never migrate) | Skip |

**UI rule:** If batch contains any WARNING → show confirmation checkbox before execute. If any BLOCKED → show skip-blocked info; execute migrates only eligible entities.

**API rule:** Batch execute returns `200` with mixed results; does not use 409 for individual blocked/warning entities (unlike single-entity execute).

---

## Partial Success Strategy

1. **Iterate entities sequentially** (v0.1 sync) or in bounded parallel workers (future).
2. For each entity:
   - Run preview item logic (in-memory, no separate HTTP call).
   - If BLOCKED → record `skipped` / `blocked`; continue.
   - If WARNING and not `allow_warnings` → record `skipped`; continue.
   - If SAFE or (WARNING and `allow_warnings`) → execute `ReplaceFieldCodesBatch` in **one DB transaction** for that entity.
   - On transaction error → record `failed` for that entity; continue (unless configured fail-fast — not default).
3. Aggregate summary and batch-level status.
4. Write batch completion audit after all entities processed.

**Fail-fast mode** (future optional flag): abort batch on first `failed` — not default.

---

## Idempotency

| Scenario | Expected behavior |
|----------|-------------------|
| Re-execute batch on already-migrated entities | `migrated_count` stable; no duplicate value rows |
| Entity already on target template | SAFE preview; replace semantics idempotent |
| Partial batch re-run (only skipped entities) | Client resubmits subset; migrated entities unchanged |
| Same `batch_id` re-execute | Rejected or no-op per policy (future strict mode) |

Implementation reuses existing single-entity idempotency (`ReplaceFieldCodesBatch`, per-field_code replace).

---

## Audit Model

### Recommended event kinds

| Event | When | Granularity |
|-------|------|-------------|
| `CUSTOM_FIELD_VALUES_BATCH_MIGRATION_STARTED` | Batch execute begins | Batch |
| `CUSTOM_FIELD_VALUES_MIGRATED_TO_ACTIVE` | Each entity successfully migrated | Entity (existing) |
| `CUSTOM_FIELD_VALUES_BATCH_MIGRATION_COMPLETED` | Batch execute finishes | Batch |

Optional future: `CUSTOM_FIELD_VALUES_BATCH_MIGRATION_PREVIEWED` on batch preview.

### Batch completion payload (proposed)

```json
{
  "event_kind": "CUSTOM_FIELD_VALUES_BATCH_MIGRATION_COMPLETED",
  "batch_id": "550e8400-e29b-41d4-a716-446655440000",
  "tenant_id": "74519f22-ff9b-4a8b-8fff-a958c689682f",
  "entity_type": "TRANSPORT_ORDER",
  "template_code": "transport_order_default",
  "target_template_id": "b1111111-1111-4111-8111-111111111102",
  "total": 100,
  "migrated": 85,
  "skipped": 12,
  "blocked": 10,
  "failed": 3,
  "allow_warnings": true,
  "skip_blocked": true,
  "actor": "00000000-0000-4000-8000-000000000001",
  "request_id": "req-batch-001",
  "status": "partially_completed"
}
```

### Decision

- **Entity-level audit** (`CUSTOM_FIELD_VALUES_MIGRATED_TO_ACTIVE`) remains **source of truth** for value changes.
- **Batch-level audit** provides admin history, correlation, and compliance summary.
- Preview batch may log audit optionally (lower priority than execute).

---

## Tenant Isolation

| Rule | Enforcement |
|------|-------------|
| All entities must belong to request tenant | Values loaded with `tenant_id` from header |
| Template resolution tenant-scoped | `GetPublishedTemplateContext(tenantID, ...)` |
| Cross-tenant entity IDs in list | Treated as empty values or not found; no cross-tenant reads |
| Batch audit | `tenant_id` on every event |

Same guarantees as single-entity preview/execute (covered by edge-case tests).

---

## UI Flow

Future page: `/low-code/admin/batch-migration` (or section on custom-field-values admin hub).

### Step 1 — Select scope

- User selects `entity_type` and `template_code` (default: active template).
- User selects entities:
  - Manual multi-select (paste UUID list), or
  - Filter-based selection (future: status, date range, list API) — v0.2.

### Step 2 — Batch preview

- Click **Batch migration preview**.
- Call `batch-migration-preview`.
- Show summary cards: total / safe / warnings / blocked.

### Step 3 — Preview table

| Column | Content |
|--------|---------|
| entity_id | UUID (monospace, wrap) |
| status | SAFE / WARNING / BLOCKED badge |
| copied | count or field list expand |
| legacy | count |
| missing required | count |
| incompatible | count |
| actions | Link to single-entity detail / custom-field-values page |

**Filters:** All | Safe | Warning | Blocked

### Step 4 — Execute confirmation

| Condition | UI |
|-----------|-----|
| All blocked | Execute disabled |
| Any warning | Checkbox: *I understand warnings and want to continue* |
| Any blocked | Checkbox (default on): *Skip blocked entities* |
| Mixed | Button: **Migrate N entities** (N = eligible count) |

### Step 5 — Post-execute result

- Summary: migrated / skipped / failed.
- Per-entity result table (same as preview + execute status).
- Links:
  - **View migration history** → `/low-code/audit?category=migrations`
  - **View batch audit** (future filter by `batch_id`)
- Download result JSON (optional future).

Single-entity modal on `/low-code/custom-field-values` remains unchanged.

---

## Background Job vs Synchronous Execution

| Mode | Entity count | v0.1 |
|------|--------------|------|
| **Synchronous** | 1..100 | **Yes** — preview and execute in request thread |
| **Background job** | >100 or operator opt-in | **No** — design only |

### Future async model (v0.2+)

```text
POST batch-migration-preview  → batch_id + summary (may accept >100 with job)
POST batch-migrate-to-active  → 202 Accepted + job_id
GET  batch-migration-jobs/{id} → progress, partial results
```

Job stores preview snapshot, executes entities with rate limiting, supports cancel.

---

## Retry Strategy

| Case | Retry guidance |
|------|----------------|
| HTTP 5xx / network | Safe to retry whole batch; idempotent per entity |
| Partial completion | Client retries with **skipped/failed entity_ids only** |
| All blocked | Fix data or template; re-preview required |
| Warning skipped | Re-run with `allow_warnings=true` |
| Stale preview | Re-run batch preview before re-execute (template changed) |

**No automatic retry** in service v0.1 — client/UI driven.

Future: `retry_failed_only=true` on batch execute referencing prior `batch_id`.

---

## Observability

### Metrics (future Prometheus)

| Metric | Type | Labels |
|--------|------|--------|
| `migration_batch_preview_total` | Counter | `tenant_id`, `entity_type`, `status` |
| `migration_batch_execute_total` | Counter | `tenant_id`, `entity_type`, `batch_status` |
| `migration_batch_entities_total` | Counter | `operation=preview\|execute`, `entity_status` |
| `migration_batch_blocked_total` | Counter | `entity_type` |
| `migration_batch_failed_total` | Counter | `entity_type` |
| `migration_batch_duration_seconds` | Histogram | `operation=preview\|execute` |

### Structured logs

```json
{
  "msg": "batch migration execute completed",
  "batch_id": "...",
  "tenant_id": "...",
  "entity_type": "TRANSPORT_ORDER",
  "total": 50,
  "migrated": 45,
  "skipped": 5,
  "status": "partially_completed",
  "request_id": "...",
  "duration_ms": 1234
}
```

No implementation in this pack.

---

## Edge Cases

| Case | Batch behavior |
|------|----------------|
| Empty entity (no values) | Preview SAFE or WARNING (missing required); execute `migrated_count=0` |
| Duplicate entity_ids in request | Dedupe or 400 validation error (recommend dedupe + warn) |
| Entity not found / no values row | Preview with empty existing; same as single-entity |
| Mixed SAFE + WARNING + BLOCKED | Partial execute with skip_blocked + allow_warnings flags |
| All entities BLOCKED | `status=failed` or `partially_completed` with migrated=0 |
| Template changes between preview and execute | Optional strict `batch_id` TTL; recommend re-preview |
| Tenant mismatch | 400/403; no writes |
| >100 entity_ids | 400 validation error; direct to async job (future) |
| Legacy fields | Listed per entity; not deleted |
| Idempotent re-run | Stable migrated entities |

Covered at single-entity layer by `docs/LOW_CODE_MIGRATION_EDGE_CASES_TEST_PACK_V0.1.md`; batch tests extend in future packs.

---

## Security Guardrails

| Guardrail | Detail |
|-----------|--------|
| Admin-only endpoints | Same auth as existing low-code admin APIs |
| Tenant header required | `X-Tenant-ID` mandatory |
| No cross-tenant batch | Enforced at repository layer |
| Preview read-only | Batch preview never writes |
| Explicit execute | No auto-execute after preview |
| Audit trail | Batch + per-entity events |
| Rate limiting (future) | Gateway throttle on batch execute |
| No v-html / unsafe UI | Batch wizard uses same safe JSON rendering as modal |

---

## Rollout Plan

| Phase | Pack | Deliverable |
|-------|------|-------------|
| 1 | **Batch Migration Preview API v0.1** | `batch-migration-preview` endpoint, `batch_id`, tests |
| 2 | **Batch Migration Execute API v0.1** | `batch-migrate-to-active`, partial success, audit |
| 3 | **Admin UI Batch Migration Wizard v0.1** | Preview table, filters, execute, results |
| 4 | **Batch Migration Audit & Metrics Pack v0.1** | Batch audit events, audit UI filter, Prometheus |
| 5 (future) | Batch async job v0.2 | >100 entities, job polling |

Existing single-entity flows remain available throughout rollout.

---

## Verification Plan

### Design pack (this document)

- [x] Design doc created
- [x] `NEXT_COMMANDS.md` updated
- [ ] `npm run build` (sanity — no code changes)
- [ ] `make health-check` (optional)

### Future implementation packs

| Pack | Verification |
|------|--------------|
| Preview API | `go test ./...`, curl batch preview, ≤100 entities |
| Execute API | Integration tests, partial success scenarios, audit assertions |
| UI wizard | Manual UI, RU/EN/ZH, filter/execute flows |
| Audit & metrics | Audit list shows batch events; metrics scrape |

Regression: `make integration-smoke-test`, existing single-entity migration modal unchanged.

---

## Next Implementation Packs

1. **Low-code Batch Migration Preview API Pack v0.1** — implement `batch-migration-preview`, `batch_id`, handler tests, curl payloads
2. **Low-code Batch Migration Execute API Pack v0.1** — implement `batch-migrate-to-active`, per-entity transactions, batch audit started/completed
3. **Admin UI Batch Migration Wizard Pack v0.1** — preview table, filters, warning/blocked confirmation, result screen
4. **Batch Migration Audit & Metrics Pack v0.1** — batch audit events in UI, `batch_id` filter, Prometheus metrics

**Immediate next action:** Low-code Batch Migration Preview API Pack v0.1

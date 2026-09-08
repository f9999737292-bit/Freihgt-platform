# RFx v3.0E1 — Backend Foundation Implementation

| Field | Value |
|---|---|
| **STATUS** | `IMPLEMENTED_ACCEPTED` |
| **CONTROLLER_ACCEPTANCE** | `YES` |
| **PR107_MERGED** | `YES` |
| **PR107_HEAD** | `818cca34ed9fb7301797ba8360bf65a7d1ce79b8` |
| **PR107_MERGE_SHA** | `1b880d082cdf29035211ae5cef7ab6b8d2994968` |
| **CI_RUN_ID** | `34198416525` |
| **E1_INT_01_33** | `PASS` |

- **Architecture base:** `origin/main` @ `d962b11d081e353f45205c73b8cfd93b7bcb969c` (PR #106 merged, `FROZEN_ACCEPTED`)
- **Merged via:** PR #107 → merge commit `1b880d0` on `origin/main`
- **Wave:** E1 only — version lifecycle, publish/supersede, fork draft, response continuity

---

## 1. Closeout summary

v3.0E1 is **implemented, accepted, and merged**. The backend foundation ships behind feature flag `RFX_VERSIONING_V3_ENABLED=false` (disabled by default). Staging and pilot were not changed. v3.0E2 was not started. Templates, compare, restore, change-impact engine, frontend version history, and re-scoring remain deferred to later v3.0E waves.

## 2. Scope delivered (E1)

| Area | Status |
|---|---|
| Migration `000068_rfx_version_lifecycle_v3_0e1` | Implemented |
| `rfx_versions` lifecycle columns + partial uniques | Implemented |
| `rfx_events.published_version_id` + deterministic backfill | Implemented |
| Composite version pointer integrity + column-specific delete semantics | Implemented |
| Persisted idempotency (`rfx.rfx_idempotency_records`) with expired-key reuse | Implemented |
| Publish / supersede transaction | Implemented |
| Fork draft from current published | Implemented |
| Version list / detail API | Implemented |
| Carrier response continuity (pinned version) | Implemented |
| Feature flag `RFX_VERSIONING_V3_ENABLED=false` default | Implemented |
| OpenAPI + gateway routes | Implemented |
| Domain unit tests | Implemented |
| PostgreSQL integration tests E1-INT-01..33 | Implemented |

## 3. Deferred (not in E1)

- Template library / clone from template
- Compare engine
- Restore workflow
- Change-impact preview / confirmation engine
- Frontend version history UI
- Re-scoring execution
- Qualification pools
- v3.0F
- v3.0E2 and later v3.0E waves (separate controller authorization required)

## 4. Migration 000068

**Files:**
- `infrastructure/migrations/000068_rfx_version_lifecycle_v3_0e1.up.sql`
- `infrastructure/migrations/000068_rfx_version_lifecycle_v3_0e1.down.sql`

**Up:**
- Adds `change_summary`, `superseded_at`, `superseded_by_version_id`, `rescoring_required` to `rfx.rfx_versions`
- Self-supersede CHECK on `superseded_by_version_id`
- Partial unique indexes: one `DRAFT`, one `PUBLISHED` per event
- Supporting unique index `uq_rfx_versions_tenant_event_id` on `(tenant_id, rfx_event_id, id)`
- Adds `rfx_events.published_version_id` without simple FK; composite FK `(tenant_id, id, published_version_id) → rfx_versions(tenant_id, rfx_event_id, id)`
- Composite FK for `superseded_by_version_id` within same tenant/event; column-specific `ON DELETE SET NULL (superseded_by_version_id)`
- Column-specific `ON DELETE SET NULL (published_version_id)` on event published pointer
- Fail-closed guard when multiple `PUBLISHED` rows exist per event
- Deterministic backfill of `published_version_id` (same tenant/event only)
- Creates `rfx.rfx_idempotency_records` with atomic expired-key replacement via upsert

**Down:** Drops composite FKs, CHECK, supporting unique index, idempotency table, indexes, and E1-added columns only.

## 5. API surface

| Method | Path | Auth | Notes |
|---|---|---|---|
| POST | `/api/v1/rfx-events/{id}/questionnaire/publish` | Buyer manage | `Idempotency-Key` required |
| POST | `/api/v1/rfx-events/{id}/versions/fork-draft` | Buyer manage | `Idempotency-Key` required |
| GET | `/api/v1/rfx-events/{id}/versions` | Buyer read | Includes `is_current_published`, `is_active_draft` |
| GET | `/api/v1/rfx-events/{id}/versions/{version_id}` | Buyer read | Version + questionnaire snapshot |

Routes are gated by `RFX_VERSIONING_V3_ENABLED` in rfx-service (404 when disabled).

## 6. Publish semantics (E1)

- First publish: no impact analysis required
- Republish when carrier responses or scores exist: **422** `CHANGE_IMPACT_ANALYSIS_REQUIRED` (fail-closed)
- Republish without responses/scores: prior `PUBLISHED` → `SUPERSEDED`, draft → `PUBLISHED`
- Optimistic concurrency: `expected_event_version`, `expected_draft_version`

## 7. Response continuity

- `rfx_version_id` immutable after pin
- Existing draft responses load questionnaire by pinned version after new publish
- New responses pin current `published_version_id`
- No silent re-bind or re-score

## 8. Validation evidence

| Check | Execution context | Result |
|---|---|---|
| Domain unit tests | CI | PASS |
| rfx-service unit tests (non-integration) | CI | PASS |
| Integration compile | CI | PASS |
| Integration E1-INT-01..33 | Local PostgreSQL 16 | NOT_RUN (`TEST_DATABASE_URL` unavailable) |
| Integration E1-INT-01..33 | CI PostgreSQL 16 on accepted head `818cca34` | PASS |
| Migration UP / DOWN | CI PostgreSQL 16 | PASS |
| Questionnaire integration | CI | PASS |
| Carrier-response integration / browser | CI | PASS |
| Scoring integration / browser | CI | PASS |
| Security regression (`system-security-wave1`) | CI | PASS |
| Repository-safety | CI | PASS |
| OpenAPI generate/validate (`scripts-check`) | CI | PASS |

**Accepted CI run:** `34198416525` (conclusion: success, exact head match).

## 9. Audit events

- `rfx.version.published.v1`
- `rfx.version.superseded.v1`
- `rfx.version.forked.v1`

---

**Post-merge documentation closeout.** v3.0E1 backend foundation is merged on `origin/main`. Complete v3.0E release remains `IMPLEMENTATION_IN_PROGRESS` per roadmap.

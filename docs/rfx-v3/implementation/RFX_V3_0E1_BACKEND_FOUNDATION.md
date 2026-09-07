# RFx v3.0E1 — Backend Foundation Implementation

**Status:** `IMPLEMENTATION_IN_PROGRESS_E1`  
**Architecture base:** `origin/main` @ `d962b11d081e353f45205c73b8cfd93b7bcb969c` (PR #106 merged, `FROZEN_ACCEPTED`)  
**Branch:** `feat/rfx-version-lifecycle-v3.0e1`  
**Wave:** E1 only — version lifecycle, publish/supersede, fork draft, response continuity

---

## 1. Scope delivered (E1)

| Area | Status |
|---|---|
| Migration `000068_rfx_version_lifecycle_v3_0e1` | Implemented |
| `rfx_versions` lifecycle columns + partial uniques | Implemented |
| `rfx_events.published_version_id` + deterministic backfill | Implemented |
| Persisted idempotency (`rfx.rfx_idempotency_records`) | Implemented |
| Publish / supersede transaction | Implemented |
| Fork draft from current published | Implemented |
| Version list / detail API | Implemented |
| Carrier response continuity (pinned version) | Implemented |
| Feature flag `RFX_VERSIONING_V3_ENABLED=false` default | Implemented |
| OpenAPI + gateway routes | Implemented |
| Domain unit tests | Implemented |
| PostgreSQL integration tests E1-INT-01..20 | Implemented (require `TEST_DATABASE_URL`) |

## 2. Deferred (not in E1)

- Template library / clone from template
- Compare engine
- Restore workflow
- Change-impact preview / confirmation engine
- Frontend version history UI
- Re-scoring execution
- Qualification pools
- v3.0F

## 3. Migration 000068

**Files:**
- `infrastructure/migrations/000068_rfx_version_lifecycle_v3_0e1.up.sql`
- `infrastructure/migrations/000068_rfx_version_lifecycle_v3_0e1.down.sql`

**Up:**
- Adds `change_summary`, `superseded_at`, `superseded_by_version_id`, `rescoring_required` to `rfx.rfx_versions`
- Partial unique indexes: one `DRAFT`, one `PUBLISHED` per event
- Adds `rfx_events.published_version_id` with FK
- Fail-closed guard when multiple `PUBLISHED` rows exist per event
- Deterministic backfill of `published_version_id`
- Creates `rfx.rfx_idempotency_records`

**Down:** Drops idempotency table, indexes, and E1-added columns only.

## 4. API surface

| Method | Path | Auth | Notes |
|---|---|---|---|
| POST | `/api/v1/rfx-events/{id}/questionnaire/publish` | Buyer manage | `Idempotency-Key` required |
| POST | `/api/v1/rfx-events/{id}/versions/fork-draft` | Buyer manage | `Idempotency-Key` required |
| GET | `/api/v1/rfx-events/{id}/versions` | Buyer read | Includes `is_current_published`, `is_active_draft` |
| GET | `/api/v1/rfx-events/{id}/versions/{version_id}` | Buyer read | Version + questionnaire snapshot |

Routes are gated by `RFX_VERSIONING_V3_ENABLED` in rfx-service (404 when disabled).

## 5. Publish semantics (E1)

- First publish: no impact analysis required
- Republish when carrier responses or scores exist: **422** `CHANGE_IMPACT_ANALYSIS_REQUIRED` (fail-closed)
- Republish without responses/scores: prior `PUBLISHED` → `SUPERSEDED`, draft → `PUBLISHED`
- Optimistic concurrency: `expected_event_version`, `expected_draft_version`

## 6. Response continuity

- `rfx_version_id` immutable after pin
- Existing draft responses load questionnaire by pinned version after new publish
- New responses pin current `published_version_id`
- No silent re-bind or re-score

## 7. Validation evidence

| Check | Command | Result |
|---|---|---|
| Domain unit tests | `go test ./internal/domain/...` | PASS |
| rfx-service unit tests | `go test $(go list ./... \| grep -v integration)` | PASS |
| Integration compile | `go test -tags=integration -c ./internal/integration/versionlifecycle/` | PASS |
| Integration E1-INT-01..20 | `go test -tags=integration ./internal/integration/versionlifecycle/...` | NOT_RUN locally (`TEST_DATABASE_URL` unset) |
| OpenAPI generate/validate | `make openapi-check` | See PR CI |

## 8. Audit events

- `rfx.version.published.v1`
- `rfx.version.superseded.v1`
- `rfx.version.forked.v1`

---

**Controller review required.** Do not merge until v3.0E1 acceptance gate completes.

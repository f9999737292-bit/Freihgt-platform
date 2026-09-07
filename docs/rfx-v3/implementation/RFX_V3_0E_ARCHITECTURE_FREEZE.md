# RFx v3.0E — Templates + Versioning Architecture Freeze

**Status:** `ARCHITECTURE_FROZEN_PENDING_CONTROLLER_ACCEPTANCE`  
**Mode:** `DISCOVERY_AND_ARCHITECTURE_FREEZE_ONLY`  
**Implementation:** **NOT authorized**  
**Base:** `origin/main` @ `0db472d6a4b79d987cb98f28253383fd4a88821d`  
**Normative:** ADR-002, ADR-008 (extended, not replaced)

---

## 1. Freeze summary

| Area | Status |
|---|---|
| Template model | **FROZEN** |
| RFx version lifecycle | **FROZEN** |
| Compare semantics | **FROZEN** |
| Restore semantics | **FROZEN** |
| Change-impact model | **FROZEN** |
| v3.0D compatibility rules | **FROZEN** |
| Authorization matrix | **FROZEN** |
| Transaction boundaries | **FROZEN** |
| API proposal | **FROZEN** (proposal only) |
| Data model proposal | **FROZEN** (proposal only) |
| Frontend scope | **FROZEN** |

---

## 2. Template model (frozen)

### 2.1 Ownership and scope

| Property | Decision |
|---|---|
| Tenant scope | **Required** — `tenant_id` on all template rows |
| Owner company | **Optional** — `owner_company_id` NULL = tenant-wide template |
| Global/cross-tenant templates | **DEFERRED** — not in v3.0E |
| Carrier access | **Read-only none** — carriers cannot list templates |
| Identifier | `template_id` (UUID) + stable `template_code` per tenant |
| Version identifier | `template_version_id` + monotonic `version_number` per template |

### 2.2 Aggregate lifecycle (`rfx_templates.status`)

```
ACTIVE ──archive──► ARCHIVED
```

| Status | Mutability |
|---|---|
| ACTIVE | Template metadata editable; version operations allowed |
| ARCHIVED | Read-only aggregate; **clone forbidden** — no exceptions in v3.0E |

**Hard delete:** Forbidden for published template or template version rows. DRAFT-only soft-delete allowed when zero published versions exist.

### 2.3 Template version lifecycle (`rfx_template_versions.status`)

```
DRAFT ──publish──► PUBLISHED ──supersede──► SUPERSEDED
         ↑
    (immutable)
```

| Status | Mutability |
|---|---|
| DRAFT | Editable questionnaire graph in template tables |
| PUBLISHED | **Immutable** snapshot — current published version |
| SUPERSEDED | **Immutable** historical snapshot — compare/history/clone-with-warning |

| Constraint | Rule |
|---|---|
| Max one DRAFT | Per template — partial unique index |
| Max one PUBLISHED | Per template — partial unique index |
| On publish | Prior PUBLISHED → SUPERSEDED |

**ARCHIVED does not apply to template version rows.** Aggregate retirement uses `rfx_templates.status = ARCHIVED`.

### 2.4 Uniqueness and partial indexes

| Constraint | Rule |
|---|---|
| Template code | UNIQUE `(tenant_id, template_code)` WHERE `deleted_at IS NULL` |
| Template version number | UNIQUE `(template_id, version_number)` |
| One DRAFT per template | Partial unique `(template_id)` WHERE `status = 'DRAFT' AND deleted_at IS NULL` |
| One PUBLISHED per template | Partial unique `(template_id)` WHERE `status = 'PUBLISHED' AND deleted_at IS NULL` |

### 2.5 Clone semantics (template → RFx event)

| Step | Behavior |
|---|---|
| Input (default) | `template_version_id` with status **PUBLISHED** + new event metadata |
| Input (explicit) | `template_version_id` with status **SUPERSEDED** — allowed only via version-specific clone action; UI shows **warning** |
| Blocked | Aggregate `ARCHIVED` — clone returns **409**. Unarchive/restore aggregate **deferred** — no hidden exception |
| Output | New `rfx_event` + new `rfx_version` DRAFT + materialized questionnaire graph |
| Provenance | Set `rfx_events.source_template_version_id` — **immutable** after create |
| Scoring | **Not copied** — buyer configures in Studio (v3.0D per-event model) |
| Responses | **None** |

### 2.6 Multilingual content (RU/EN/ZH)

| Field | Behavior |
|---|---|
| Template name/description | i18n map JSON `{ "ru-RU": "...", "en-US": "...", "zh-CN": "..." }` |
| Questionnaire content | **Unchanged v3.0B model** — no schema migration in v3.0E |
| Studio system labels | Localized RU/EN/ZH via existing web-admin i18n |
| Multilingual questionnaire schema | **Deferred** beyond v3.0E |
| Compare/restore UI | Uses buyer locale; diff compares **persisted fields** and stable `*_code` identifiers |

---

## 3. RFx versioning (frozen)

### 3.1 Lifecycle (extends existing `rfx_versions`)

**Existing DB constraint** includes `ARCHIVED` on version rows; **v3.0E semantics use only** DRAFT / PUBLISHED / SUPERSEDED. Event/aggregate retirement uses `rfx_events.status = ARCHIVED`.

```
DRAFT ──publish──► PUBLISHED ──supersede──► SUPERSEDED
  ▲
  └── fork / restore-as-draft (separate command; new DRAFT row after publish)
```

| Rule | Detail |
|---|---|
| Published immutability | **Absolute** — no UPDATE on sections/questions/options/rules/bindings for PUBLISHED/SUPERSEDED |
| Post-publish change | **Always** creates new DRAFT version (including typo/label) — OD-E06 |
| Historical row | **Never overwritten** — SUPERSEDED remains immutable |
| Restore | Creates **new DRAFT** with `version_number = max+1`; does not demote history |
| Active drafts | **One** DRAFT per event — concurrent fork → **409** |
| Version numbering | Monotonic INT per `rfx_event_id` |
| Concurrency | `rfx_versions.version` optimistic token on DRAFT (existing) |
| Metadata on publish | `published_at`, `published_by`, `change_summary` (required), `rescoring_required` (§6.5) |

### 3.2 Event columns (verified — migration/code)

**Shipped today on `rfx.rfx_events`:**

| Column | Source | Role |
|---|---|---|
| `draft_version_id` | migration 000065 | Points to active questionnaire DRAFT |
| `version` | migration 000004 | Optimistic lock on event aggregate |
| (no published pointer) | — | Current PUBLISHED resolved by query: `rfx_versions.status = 'PUBLISHED'` (`GetPublishedVersionForEvent`) |

**Proposed in migration 000068 (denormalized pointer — not shipped):**

| Column | Role |
|---|---|
| `published_version_id` | FK to current PUBLISHED `rfx_versions.id`; set in publish TX step 7 |
| `source_template_version_id` | Provenance FK; immutable after event create (OD-E04) |

### 3.3 Questionnaire publish transaction (frozen — 11 steps)

Single database transaction; **does not** create a new DRAFT implicitly.

| Step | Action |
|---|---|
| 1 | `SELECT … FOR UPDATE` lock `rfx_events` row |
| 2 | Validate `expected_version` token on event + draft version row |
| 3 | Run `EvaluatePublishReadiness` — fail → **422** |
| 4 | If prior PUBLISHED exists: load `rfx_change_impact_analyses` row `FOR UPDATE`; validate confirmation (§6.4) |
| 5 | Mark prior PUBLISHED version → **SUPERSEDED** (`superseded_at`, `superseded_by_version_id`) |
| 6 | Mark current DRAFT → **PUBLISHED**; set `rescoring_required` from confirmed impact (§6.5) |
| 7 | Set `rfx_events.published_version_id = draft.id` (proposed column) |
| 8 | Set `rfx_events.draft_version_id = NULL` |
| 9 | Set `rfx_change_impact_analyses.consumed_at = now()` when confirmation used |
| 10 | Insert audit `rfx.version.published.v1` (+ `rfx.version.superseded.v1` if applicable) |
| 11 | Commit |

**Idempotency:** `Idempotency-Key` header **required** (§12). Retry after timeout returns stored result.

**New DRAFT after publish:** Created only by explicit **fork** or **restore-as-draft** command — never inside publish TX.

### 3.4 Carrier response continuity (frozen — replaces unsafe 409 rule)

**Current code gap:** `ensureCarrierResponse` rejects existing responses when `response.rfx_version_id ≠ current published.id` — **must be remediated** in implementation.

| Rule | Detail |
|---|---|
| Pin at creation | `rfx_responses.rfx_version_id` set when response is created |
| Immutability | **`rfx_version_id` immutable from creation** — no change during draft, after new publish, on resume, on submit, or after submit |
| `save_version` | Optimistic concurrency token on response aggregate only — **does not** permit changing `rfx_version_id` |
| Workspace load | Questionnaire loaded by **`response.rfx_version_id`**, not current published |
| Draft response after new publish | **Continues** save/resume/submit against pinned version |
| New response | Pins **then-current** PUBLISHED version at create time |
| 409 triggers | Stale `save_version`; ownership/lifecycle violation; concurrent draft creation; idempotency/body mismatch; stale impact confirmation — **not** version publish drift |
| Submitted response | **Immutable** (including `rfx_version_id`) |
| Closed version for new entrants | New responses use newest PUBLISHED; in-flight draft/submitted responses on older pins **complete normally** |

---

## 4. Comparison semantics (frozen)

### 4.1 Canonical diff units

Diff operates on stable codes within version scope:

| Domain | Match key | Compared fields |
|---|---|---|
| Sections | `section_code` | title, description, sort_order |
| Questions | `question_code` (within section) | type, label, help_text, required, validation_rule_json, sort_order |
| Options | `option_code` (within question) | label, sort_order |
| Conditional rules | `rule_code` | action, condition_json, target_question_code |
| Score criteria | `criterion_code` | weight, normalization_json |
| Score bindings | `(criterion_code, question_code)` | knockout_rule_json |
| Score model | `model_version` | status transition only (not deep diff of published model) |

### 4.2 Change classification per item

| Result | Meaning |
|---|---|
| `ADDED` | Present only in target (newer) version |
| `REMOVED` | Present only in source (older) version |
| `CHANGED` | Same code, different normalized payload |
| `REORDERED` | Same payload, different sort_order only |
| `UNCHANGED` | Identical normalized payload and order |

**Output shape (API):**

```json
{
  "source_version_number": 1,
  "target_version_number": 2,
  "sections": [{ "section_code": "HSE", "change": "CHANGED", "fields": ["title"] }],
  "questions": [],
  "options": [],
  "rules": [],
  "scoring": { "criteria": [], "bindings": [] }
}
```

---

## 5. Restore semantics (frozen)

| Property | Rule |
|---|---|
| Operation | `RESTORE_DRAFT_VERSION` |
| Input | `source_version_id` (PUBLISHED or SUPERSEDED) |
| Effect | Creates **new** DRAFT with `version_number = max+1`, deep copy of questionnaire (+ scoring draft if present) |
| Existing DRAFT | **409 Conflict** — restore blocked |
| `force=true` | **Removed** — not in architecture or API |
| Draft destruction | **Forbidden** via restore; user must explicitly cancel/discard draft via future confirmed operation |
| Published/SUPERSEDED rows | **Untouched** |
| Audit | `rfx.version.restored_as_draft.v1` |
| Idempotency | `Idempotency-Key` **required** |

Restore **≠** rollback. No status demotion of published history.

---

## 6. Change-impact analysis (frozen)

### 6.1 Impact classes

| Class | Trigger | Publish allowed | Confirmation |
|---|---|---|---|
| `NON_MATERIAL` | Label/help/reorder-only diff (still requires new version per OD-E06) | YES | Optional confirmation |
| `MATERIAL_NO_RESPONSES` | Structural/scoring change; zero responses | YES with preview | Recommended |
| `MATERIAL_WITH_DRAFT_RESPONSES` | Change affects questions with in-progress answers | YES with confirmation | **Required**; pinned drafts **continue** on old version |
| `MATERIAL_WITH_SUBMITTED_RESPONSES` | Change affects pinned version with submissions | YES with confirmation | **Required** + participant notice flag |
| `SCORING_AFFECTING` | Criteria/binding/knockout/normalization change | YES | **Required**; set `rfx_versions.rescoring_required = TRUE` on new published version (§6.5) |
| `KNOCKOUT_AFFECTING` | Knockout rule change | YES | **Required**; set `rescoring_required = TRUE` on new published version (§6.5); never auto-reject existing submitted answers |

### 6.2 Per-class behavior

| Class | Draft responses | Submitted responses | Old scores | New version |
|---|---|---|---|---|
| NON_MATERIAL | Continue on old pin until new publish | Unchanged | Preserved | New PUBLISHED supersedes |
| MATERIAL_NO_RESPONSES | N/A | Unchanged | Preserved | New PUBLISHED |
| MATERIAL_WITH_DRAFT_RESPONSES | **Continue** save/submit on pinned version | Unchanged | Preserved | New PUBLISHED; new responses use new pin |
| MATERIAL_WITH_SUBMITTED_RESPONSES | — | **Immutable** on old version | **Preserved** | New PUBLISHED for future responses |
| SCORING_AFFECTING | — | Old scores remain authoritative for old pin | **Preserved** | New score model draft required on new version |
| KNOCKOUT_AFFECTING | — | No retroactive knockout | **Preserved** | Knockout applies only to responses on new version |

### 6.3 Forbidden

| Action | Verdict |
|---|---|
| Silent re-score of submitted responses | **FORBIDDEN** |
| Re-bind submitted response to new version | **FORBIDDEN** |
| DELETE score/qualification history | **FORBIDDEN** |
| Edit published score model | **FORBIDDEN** (v3.0D frozen) |
| Map missing/failed/pending to zero/rejected | **FORBIDDEN** (v3.0D frozen) |

### 6.4 Material-change confirmation contract (frozen)

**Persistence:** Preview creates immutable row in `rfx.rfx_change_impact_analyses` (§9.1). Audit events do **not** replace this table.

**Impact preview response (`POST …/change-impact/preview`):**

| Field | Type | Notes |
|---|---|---|
| `impact_analysis_id` | UUID | Server-generated (= persisted row `id`) |
| `tenant_id` | UUID | |
| `event_id` | UUID | |
| `source_version_id` | UUID | Prior PUBLISHED (nullable on first publish) |
| `candidate_version_id` | UUID | DRAFT being evaluated |
| `canonical_diff_hash` | string | SHA-256 of normalized diff payload |
| `impact_classes` | string[] | From §6.1 |
| `affected_draft_response_count` | int | |
| `affected_submitted_response_count` | int | |
| `scoring_affecting` | bool | |
| `knockout_affecting` | bool | |
| `expires_at` | timestamp | TTL default 15 minutes |

**Publish request must include:**

| Field | Notes |
|---|---|
| `impact_analysis_id` | Required when prior PUBLISHED exists |
| `canonical_diff_hash` | Must match persisted analysis |
| `expected_version` | Optimistic token on DRAFT |
| `Idempotency-Key` | Required |

**Confirmation validity:** same tenant; same actor or buyer-manage delegate; same event + candidate version; unchanged diff hash; not expired; `consumed_at IS NULL`.

**Impact confirmation error semantics (deterministic):**

| Condition | HTTP |
|---|---|
| Malformed identifier/body | **400** |
| Unauthenticated | **401** |
| Actor lacks permission | **403** |
| Unknown resource or impact analysis for different tenant/event/version | **404** |
| Candidate draft changed after preview; stale `canonical_diff_hash` or `expected_version` | **409** |
| Required impact confirmation missing | **422** |
| Impact analysis expired (`expires_at` passed) | **422** |

**Single-use:** Successful publish sets `consumed_at` in the same transaction. Repeat publish handled via `Idempotency-Key`, not by reusing consumed analysis.

**Cleanup:** Expired/unconsumed rows may be deleted only after retention window (default 7 days).

### 6.5 `RESCORING_REQUIRED` storage (frozen — proposal only)

**Column:** `rfx_versions.rescoring_required BOOLEAN NOT NULL DEFAULT FALSE` (migration 000068 proposal — **not created**)

| Rule | Detail |
|---|---|
| Set on publish | Computed from confirmed impact analysis |
| TRUE when | Impact classes include `SCORING_AFFECTING` or `KNOCKOUT_AFFECTING` |
| Scope | Applies to the **newly published** version row only |
| Does not | Trigger re-score; alter old responses; alter old scores |
| Immutability | **Immutable after publish** — no UPDATE on published row |
| UI | Shown in version history and Studio scoring/knockout warning |
| Reset | Only by creating a **next** version — never by UPDATE of published row |

---

## 7. v3.0D compatibility (frozen FK / pinning rules)

| Relationship | Pinning rule |
|---|---|
| Response → questionnaire | `rfx_responses.rfx_version_id` → `rfx_versions.id` — **immutable from response creation** |
| Answer → question | `rfx_answers.question_id` (question belongs to pinned version graph) |
| Score model → version | `rfx_score_models.rfx_version_id` + `model_version` |
| Answer score → model | `rfx_answer_scores.score_model_version` matches qualification row |
| Qualification → model | `rfx_qualification_results.score_model_version` |
| Cross-version scoring | **DENIED** — scoring engine loads model for response's pinned version only |

Material publish creating new version **does not** alter existing score rows. v3.0E persists `rfx_versions.rescoring_required` on the new published version only (§6.5); automatic and manual re-score of existing responses **deferred** beyond v3.0E (OD-E05).

---

## 8. Authorization matrix (frozen)

Trusted identity: gateway-verified JWT → `X-Tenant-ID`, `X-User-ID` → membership/owner checks. Fail-closed.

| ROLE | CREATE TEMPLATE | EDIT TEMPLATE DRAFT | PUBLISH TEMPLATE | CLONE EVENT FROM TEMPLATE | EDIT EVENT DRAFT | PUBLISH VERSION | COMPARE | RESTORE | ARCHIVE | VIEW HISTORY |
|---|---|---|---|---|---|---|---|---|---|---|
| Platform admin | YES (tenant) | YES | YES | YES | YES | YES | YES | YES | YES | YES |
| Tenant admin | YES | YES | YES | YES | YES | YES | YES | YES | YES | YES |
| Buyer (owner company) | YES | YES | YES | YES | YES | YES | YES | YES | YES (own) | YES (own) |
| Buyer (non-owner) | NO | NO | NO | NO | NO | NO | NO | NO | NO | NO |
| Carrier | NO | NO | NO | NO | NO | NO | NO | NO | NO | NO (response workspace only) |
| Unauthenticated | NO | NO | NO | NO | NO | NO | NO | NO | NO | NO |
| Cross-tenant actor | NO | NO | NO | NO | NO | NO | NO | NO | NO | NO |

| Property | Rule |
|---|---|
| Unknown or cross-tenant resource | **404** (fail-closed) |
| Impact analysis for different tenant/event/version | **404** |
| Same-tenant authenticated actor without permission | **403** |
| Unauthenticated | **401** |
| Stale version token / draft exists / idempotency conflict / stale diff hash or expected_version after preview | **409** |
| Readiness fail / missing required impact confirmation / expired impact analysis | **422** |
| Malformed request | **400** |

---

## 9. Data model proposal (NOT migrated)

**Candidate migration:** `000068_rfx_templates_versioning_v3_0e` — **creation deferred** until implementation authorized.

### 9.1 New tables

#### `rfx.rfx_templates`

| Column | Type | Notes |
|---|---|---|
| id | UUID PK | |
| tenant_id | UUID NOT NULL | |
| template_code | VARCHAR(128) NOT NULL | |
| name_i18n_json | JSONB NOT NULL | |
| description_i18n_json | JSONB | |
| rfx_type | VARCHAR(32) | |
| owner_company_id | UUID NULL | |
| status | VARCHAR(32) | **ACTIVE**, **ARCHIVED** |
| version | INT | Optimistic lock |
| created_by | UUID | |
| created_at, updated_at, deleted_at | TIMESTAMPTZ | |

#### `rfx.rfx_template_versions`

| Column | Type | Notes |
|---|---|---|
| id | UUID PK | |
| tenant_id | UUID NOT NULL | |
| template_id | UUID FK | |
| version_number | INT NOT NULL | |
| status | VARCHAR(32) | **DRAFT**, **PUBLISHED**, **SUPERSEDED** |
| published_at, published_by | TIMESTAMPTZ, UUID | |
| change_summary | TEXT | |
| version | INT | Optimistic lock |

#### Template questionnaire graph (OD-E03 — separate tables)

| Table | Mirrors |
|---|---|
| `rfx.rfx_template_sections` | `rfx_sections` structure; FK `rfx_template_version_id` |
| `rfx.rfx_template_questions` | `rfx_questions` |
| `rfx.rfx_template_question_options` | `rfx_question_options` |
| `rfx.rfx_template_question_rules` | `rfx_question_rules` |

**No** nullable dual-owner FK on event questionnaire tables. Clone **materializes** template rows into event-version graph.

#### `rfx.rfx_change_impact_analyses` (proposal)

| Column | Type | Notes |
|---|---|---|
| id | UUID PK | Returned as `impact_analysis_id` |
| tenant_id | UUID NOT NULL | FK scope |
| event_id | UUID NOT NULL | FK → `rfx_events` |
| source_version_id | UUID NULL | FK → `rfx_versions` |
| candidate_version_id | UUID NOT NULL | FK → `rfx_versions` |
| actor_id | UUID NOT NULL | Preview creator |
| canonical_diff_hash | VARCHAR(64) NOT NULL | SHA-256 hex |
| impact_classes | JSONB NOT NULL | Array of class strings |
| affected_draft_response_count | INT NOT NULL DEFAULT 0 | |
| affected_submitted_response_count | INT NOT NULL DEFAULT 0 | |
| scoring_affecting | BOOLEAN NOT NULL DEFAULT FALSE | |
| knockout_affecting | BOOLEAN NOT NULL DEFAULT FALSE | |
| created_at | TIMESTAMPTZ NOT NULL | |
| expires_at | TIMESTAMPTZ NOT NULL | |
| consumed_at | TIMESTAMPTZ NULL | Set on successful publish |

**Rules:** Immutable after insert; publish reads with row lock; expired → **422**; stale candidate/diff → **409**; cross-tenant lookup → **404**; cleanup after retention window only.

**Indexes:**

- `(tenant_id, event_id, candidate_version_id, created_at DESC)`
- `(expires_at)` WHERE `consumed_at IS NULL` — cleanup candidate
- FK `(tenant_id, event_id)` → `rfx_events`
- FK `(candidate_version_id)` → `rfx_versions`

#### `rfx.rfx_idempotency_records` (proposal)

| Column | Notes |
|---|---|
| id | UUID PK |
| tenant_id, actor_id | Scope |
| operation | e.g. `PUBLISH_EVENT_VERSION` |
| aggregate_scope | event_id or template_id |
| idempotency_key | Client header value |
| request_body_hash | SHA-256 |
| response_status + response_body_json | Stored first result |
| created_at, expires_at | TTL default 24h |

### 9.2 Extensions to existing tables

| Table | Addition |
|---|---|
| `rfx_events` | `source_template_version_id UUID NULL FK` — immutable after insert |
| `rfx_events` | `published_version_id UUID NULL FK` — denormalized current published pointer |
| `rfx_versions` | `change_summary TEXT`, `superseded_at`, `superseded_by_version_id` |
| `rfx_versions` | `rescoring_required BOOLEAN NOT NULL DEFAULT FALSE` (§6.5) |
| `rfx_versions` | Partial unique: one **DRAFT** per event |
| `rfx_versions` | Partial unique: one **PUBLISHED** per event |
| `rfx_template_versions` | Partial unique: one **DRAFT** per template |
| `rfx_template_versions` | Partial unique: one **PUBLISHED** per template |

### 9.3 Indexes (minimum)

- `(tenant_id, template_code)` unique partial WHERE `deleted_at IS NULL`
- `(tenant_id, rfx_event_id, status)` on `rfx_versions`
- `(template_id, version_number)` unique on `rfx_template_versions`
- `(rfx_event_id, version_number)` unique on `rfx_versions` (exists)
- Partial uniques for DRAFT and PUBLISHED (separate indexes per §2.4, §9.2)
- `(tenant_id, operation, aggregate_scope, idempotency_key)` unique on idempotency records

---

## 10. API proposal (NOT implemented)

Base path: `/api/v1` via api-gateway. All endpoints require authenticated buyer unless noted.

| METHOD | PATH | ROLE | REQUEST | RESPONSE | ERRORS | IDEMPOTENCY |
|---|---|---|---|---|---|---|
| GET | `/rfx-templates` | Buyer manage/read | query: status, owner | `{ items: Template[] }` | 401,403 | Safe |
| POST | `/rfx-templates` | Buyer manage | `{ template_code, name_i18n, rfx_type }` | Template | 400,401,403,409 | No |
| GET | `/rfx-templates/{id}` | Buyer read | — | Template + draft version | 401,403,404 | Safe |
| PATCH | `/rfx-templates/{id}` | Buyer manage | metadata + `expected_version` | Template | 400,401,403,404,409 | No |
| POST | `/rfx-templates/{id}/versions/publish` | Buyer manage | `{ expected_version, change_summary }` + **Idempotency-Key** | TemplateVersion | 400,401,403,404,409,422 | **Required** |
| POST | `/rfx-templates/{id}/archive` | Buyer manage | — | Template | 401,403,404,409 | No |
| POST | `/rfx-events/from-template` | Buyer manage | `{ template_version_id, … }` + **Idempotency-Key** | RfxEvent | 400,401,403,404,409 | **Required** |
| GET | `/rfx-events/{id}/versions` | Buyer read | — | `{ versions: VersionSummary[] }` | 401,403,404 | Safe |
| GET | `/rfx-events/{id}/versions/{version_id}` | Buyer read | — | VersionDetail | 401,403,404 | Safe |
| POST | `/rfx-events/{id}/versions/compare` | Buyer read | `{ source_version_id, target_version_id }` | CompareResult | 400,401,403,404 | Safe (no audit write) |
| POST | `/rfx-events/{id}/versions/{version_id}/restore-draft` | Buyer manage | `{ change_summary }` + **Idempotency-Key** | New draft Version | 401,403,404,409,422 | **Required** |
| POST | `/rfx-events/{id}/questionnaire/publish` | Buyer manage | `{ expected_version, impact_analysis_id?, canonical_diff_hash? }` + **Idempotency-Key** | Published Version | 400,401,403,404,409,422 — see §10.1 | **Required** |
| POST | `/rfx-events/{id}/change-impact/preview` | Buyer manage | `{ candidate_version_id }` | ChangeImpactAnalysis | 400,401,403,404 | Safe |

### 10.1 Error semantics (mandatory — unified)

| Code | Use |
|---|---|
| 400 | Malformed body, invalid UUID |
| 401 | Unauthenticated |
| 403 | Same-tenant authenticated actor without permission |
| 404 | Unknown or cross-tenant resource; impact analysis for different tenant/event/version |
| 409 | Stale version token, immutable conflict, draft already exists, idempotency key/body mismatch, stale diff hash or expected_version after preview |
| 422 | Readiness fail, **missing required** impact confirmation, **expired** impact analysis |

---

## 11. Transaction and concurrency model (frozen)

| Operation | Transaction boundary | Locks / guards |
|---|---|---|
| Publish template version | Single TX: validate → supersede prior PUBLISHED → publish → audit → idempotency record | Lock template aggregate + version rows |
| Publish event questionnaire | **11-step TX (§3.3)** — no implicit new DRAFT | Lock event + draft version + impact analysis |
| Restore as draft | TX: verify **no** draft → deep copy → audit → idempotency | Lock event; **409** if draft exists |
| Clone from template | TX: create event + version + materialize graph + provenance + idempotency | Block if template ARCHIVED |
| Concurrent draft creation | Partial unique on DRAFT → one winner, others **409** | |
| Concurrent draft edits | Optimistic `version` token (existing) | |
| Double publish | Partial unique PUBLISHED + TX | |
| Response during publish | Responses keep pinned `rfx_version_id`; publish does not mutate response rows | |
| Re-scoring | **Out of v3.0E scope** — flag only | |

**No silent last-write-wins** for material publishes or restore.

---

## 12. Idempotency contract (frozen)

**Required header:** `Idempotency-Key` on:

- publish template version
- publish event version
- restore-as-draft
- clone event from template

| Property | Rule |
|---|---|
| Scope key | `tenant_id` + `actor_id` + `operation` + aggregate scope (template_id or event_id) |
| Body hash | SHA-256 of normalized request body stored with record |
| Same key + same body | Return **stored response** (same status + body) |
| Same key + different body | **409** idempotency conflict |
| Persistence | Written in **same transaction** as mutating operation |
| TTL | Default 24h retention; expired keys may be reused |
| Network retry | Safe — does not create duplicate version/event |

---

## 13. Frontend scope (frozen)

### 13.1 New Studio surfaces

| Surface | Purpose |
|---|---|
| Template Library | List/create/archive templates |
| Template Editor | Draft edit + publish (reuses questionnaire builder components) |
| Create RFx from Template | Wizard step |
| Version History panel | List versions with status, author, time, summary |
| Compare view | Side-by-side / diff summary from compare API |
| Restore dialog | Confirm new draft creation |
| Material-change modal | Shows ChangeImpactAnalysis + confirmation |
| Scoring impact badge | When SCORING_AFFECTING or KNOCKOUT_AFFECTING |

### 13.2 Immutable published state

Published version opens **read-only** in Studio (existing scoring lock pattern extended to questionnaire).

---

## 14. Audit events (frozen — versioned names)

| Event name | When |
|---|---|
| `rfx.template.created.v1` | Template aggregate create |
| `rfx.template.version.published.v1` | Template version publish |
| `rfx.template.archived.v1` | Template aggregate archive |
| `rfx.version.published.v1` | Event questionnaire version publish |
| `rfx.version.superseded.v1` | Prior version superseded |
| `rfx.version.restored_as_draft.v1` | Restore-as-draft |
| `rfx.event.created_from_template.v1` | Clone from template |
| `rfx.change_impact.previewed.v1` | Impact preview generated |
| `rfx.change_impact.confirmed.v1` | Buyer confirmed material publish |

**Compare:** Safe GET/compare — **no mandatory domain audit write**. Security access logging via gateway/telemetry only.

---

## 15. Controller decisions closed (OD-E01–OD-E07)

See `RFX_V3_0E_DISCOVERY.md` §7 — all **CLOSED** in remediation PR #106.

---

## 16. Stop conditions (implementation phase)

Implementation must **STOP** and escalate if:

- Any design requires in-place edit of PUBLISHED questionnaire or score model
- Any design re-binds submitted responses without explicit new response workflow
- Migration 000068 would alter v3.0D score/qualification uniqueness
- Cross-tenant template visibility introduced without ADR
- Silent re-scoring proposed

---

**CONTROLLER_ACCEPTANCE:** PENDING  
**NEXT_ACTION:** `CONTROLLER_ACCEPTANCE_V3_0E_ARCHITECTURE`

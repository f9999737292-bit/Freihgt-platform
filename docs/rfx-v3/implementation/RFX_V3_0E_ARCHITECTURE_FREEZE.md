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

### 2.2 Lifecycle

```
DRAFT → PUBLISHED → ARCHIVED
         ↑
    (immutable)
```

| Status | Mutability |
|---|---|
| DRAFT | Editable (sections/questions/rules copied into template version draft graph) |
| PUBLISHED | **Immutable** snapshot |
| ARCHIVED | Read-only; cannot clone new events |

**Deletion:** Soft-delete (`deleted_at`) allowed only for DRAFT templates with zero published versions. Published templates **archive**, never hard-delete.

### 2.3 Uniqueness

| Constraint | Rule |
|---|---|
| Template code | UNIQUE `(tenant_id, template_code)` WHERE `deleted_at IS NULL` |
| Template version | UNIQUE `(template_id, version_number)` |
| Published template version | At most **one** PUBLISHED per template at a time (partial unique index) |

### 2.4 Clone semantics (template → RFx event)

| Step | Behavior |
|---|---|
| Input | `template_version_id` (must be PUBLISHED) + new event metadata |
| Output | New `rfx_event` + new `rfx_version` DRAFT + copied questionnaire graph |
| Provenance | Set `rfx_events.source_template_version_id` (nullable FK) |
| Scoring | **Not copied automatically** — buyer configures scoring in Studio after clone (v3.0D model remains per-event) |
| Responses | **None** — new event has zero responses |

### 2.5 Multilingual content (RU/EN/ZH)

| Field | Behavior |
|---|---|
| Template name/description | i18n map JSON `{ "ru-RU": "...", "en-US": "...", "zh-CN": "..." }` |
| Questionnaire labels | Same pattern as v3.0B: label/help_text per locale key or inline with future i18n extraction |
| Compare/restore UI | Uses buyer locale; diff shows stable `*_code` identifiers |

---

## 3. RFx versioning (frozen)

### 3.1 Lifecycle (extends existing `rfx_versions`)

```
DRAFT ──publish──► PUBLISHED ──supersede──► SUPERSEDED
  ▲                      │
  │                      └── archive ──► ARCHIVED
  │
  └── fork from published / restore-as-draft (new DRAFT row, new version_number)
```

| Rule | Detail |
|---|---|
| Published immutability | **Absolute** — no UPDATE on sections/questions/options/rules/bindings for PUBLISHED version |
| Material change | Creates **new** DRAFT version (increment `version_number`) |
| Historical row | **Never overwritten** — SUPERSEDED rows remain for audit |
| Restore | Creates **new DRAFT** copied from selected historical version; does **not** revert status of old rows |
| Active drafts | **One** DRAFT version per event at a time |
| Version numbering | Monotonic INT per `rfx_event_id`; gaps allowed after failed attempts (use transaction) |
| Concurrency | `rfx_versions.version` optimistic token on DRAFT row (existing pattern) |
| Metadata on publish | `published_at`, `published_by`, `change_summary` (required) |

### 3.2 Questionnaire publish (missing today — required in v3.0E)

| Gate | Requirement |
|---|---|
| Readiness | Existing `EvaluatePublishReadiness` must pass (no blocking FAIL) |
| Impact | If prior PUBLISHED exists → `ChangeImpactAnalysis` required |
| Confirmation | Material impact with submitted responses → explicit buyer confirmation token |
| Transaction | Publish atomically: mark old PUBLISHED → SUPERSEDED; new → PUBLISHED; set `rfx_events.draft_version_id` to new DRAFT if forking |

### 3.3 Response binding (unchanged — frozen)

| Entity | Pin |
|---|---|
| `rfx_responses.rfx_version_id` | Set at response start to **then-current** published version |
| Version mismatch on save | **409** if event published version changed and response not refreshed |
| Submitted responses | **Never** re-bound to new version automatically |

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
| Old DRAFT | If exists → **409** unless `force=true` with confirmation (discard draft) |
| Published rows | **Untouched** |
| Audit | `rfx.version.restored_as_draft.v1` with source/target version ids |

Restore **≠** rollback. No status demotion of published history.

---

## 6. Change-impact analysis (frozen)

### 6.1 Impact classes

| Class | Trigger | Publish allowed | Confirmation |
|---|---|---|---|
| `NON_MATERIAL` | Label/help typo, reorder only | YES (still creates new version if post-publish) | Optional |
| `MATERIAL_NO_RESPONSES` | Structural/scoring change; zero responses | YES with preview | Recommended |
| `MATERIAL_WITH_DRAFT_RESPONSES` | Change affects questions with in-progress answers | YES with confirmation | **Required** |
| `MATERIAL_WITH_SUBMITTED_RESPONSES` | Change affects pinned version with submissions | YES with confirmation | **Required** + participant notice flag |
| `SCORING_AFFECTING` | Criteria/binding/knockout/normalization change | YES | **Required**; `RESCORING_REQUIRED=true` for new submissions only |
| `KNOCKOUT_AFFECTING` | Knockout rule change | YES | **Required**; never auto-reject existing submitted answers |

### 6.2 Per-class behavior

| Class | Draft responses | Submitted responses | Old scores | New version |
|---|---|---|---|---|
| NON_MATERIAL | Continue on old pin until new publish | Unchanged | Preserved | New PUBLISHED supersedes |
| MATERIAL_NO_RESPONSES | N/A | Unchanged | Preserved | New PUBLISHED |
| MATERIAL_WITH_DRAFT_RESPONSES | May invalidate local drafts; server keeps last valid | Unchanged | Preserved | New PUBLISHED; new responses use new pin |
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

---

## 7. v3.0D compatibility (frozen FK / pinning rules)

| Relationship | Pinning rule |
|---|---|
| Response → questionnaire | `rfx_responses.rfx_version_id` → `rfx_versions.id` (immutable after submit) |
| Answer → question | `rfx_answers.question_id` (question belongs to pinned version graph) |
| Score model → version | `rfx_score_models.rfx_version_id` + `model_version` |
| Answer score → model | `rfx_answer_scores.score_model_version` matches qualification row |
| Qualification → model | `rfx_qualification_results.score_model_version` |
| Cross-version scoring | **DENIED** — scoring engine loads model for response's pinned version only |

Material publish creating new version **does not** alter existing score rows. Re-scoring, if ever triggered, is explicit buyer action on v3.0E+ and produces new rows for new model version only (deferred execution to v3.0E+ controller authorization).

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
| IDOR | Foreign tenant/resource → **404** (not 403) |
| Compare | Same tenant + buyer read on event |
| Restore | Requires `PolicyBuyerManage` equivalent |
| Global template scope | **Not introduced** implicitly |

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
| status | VARCHAR(32) | DRAFT, PUBLISHED, ARCHIVED |
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
| status | VARCHAR(32) | DRAFT, PUBLISHED, ARCHIVED |
| published_at, published_by | TIMESTAMPTZ, UUID | |
| change_summary | TEXT | |
| version | INT | Optimistic lock |

Template questionnaire graph: reuse same section/question/option/rule tables with `rfx_template_version_id` **OR** separate normalized tables mirroring event structure. **Frozen decision:** mirror pattern — add nullable `rfx_template_version_id` to section graph with CHECK exactly one of (`rfx_version_id`, `rfx_template_version_id`) set.

### 9.2 Extensions to existing tables

| Table | Addition |
|---|---|
| `rfx_events` | `source_template_version_id UUID NULL FK` |
| `rfx_versions` | `change_summary TEXT`, `superseded_at`, `superseded_by_version_id` |
| `rfx_versions` | Partial unique: one PUBLISHED per event |

### 9.3 Indexes (minimum)

- `(tenant_id, template_code)` unique partial
- `(tenant_id, rfx_event_id, status)` on versions
- `(template_id, version_number)` unique
- `(rfx_event_id, version_number)` unique (exists)

---

## 10. API proposal (NOT implemented)

Base path: `/api/v1` via api-gateway. All endpoints require authenticated buyer unless noted.

| METHOD | PATH | ROLE | REQUEST | RESPONSE | ERRORS | IDEMPOTENCY |
|---|---|---|---|---|---|---|
| GET | `/rfx-templates` | Buyer manage/read | query: status, owner | `{ items: Template[] }` | 401,403 | Safe |
| POST | `/rfx-templates` | Buyer manage | `{ template_code, name_i18n, rfx_type }` | Template | 400,401,403,409 | No |
| GET | `/rfx-templates/{id}` | Buyer read | — | Template + draft version | 401,403,404 | Safe |
| PATCH | `/rfx-templates/{id}` | Buyer manage | metadata + `expected_version` | Template | 400,401,403,404,409 | No |
| POST | `/rfx-templates/{id}/publish` | Buyer manage | `{ expected_version, change_summary }` | TemplateVersion | 401,403,404,409,422 | Publish idempotency key optional |
| POST | `/rfx-templates/{id}/archive` | Buyer manage | — | Template | 401,403,404,409 | No |
| POST | `/rfx-events/from-template` | Buyer manage | `{ template_version_id, title, rfx_number, ... }` | RfxEvent | 400,401,403,404,409 | No |
| GET | `/rfx-events/{id}/versions` | Buyer read | — | `{ versions: VersionSummary[] }` | 401,403,404 | Safe |
| GET | `/rfx-events/{id}/versions/{version_id}` | Buyer read | — | VersionDetail | 401,403,404 | Safe |
| POST | `/rfx-events/{id}/versions/compare` | Buyer read | `{ source_version_id, target_version_id }` | CompareResult | 400,401,403,404 | Safe |
| POST | `/rfx-events/{id}/versions/{version_id}/restore-draft` | Buyer manage | `{ expected_draft_absent, change_summary }` | New draft Version | 401,403,404,409,422 | No |
| POST | `/rfx-events/{id}/questionnaire/publish` | Buyer manage | `{ expected_version, impact_confirmation_token? }` | Published Version | 400,401,403,404,409,422 | Idempotency key recommended |
| POST | `/rfx-events/{id}/change-impact/preview` | Buyer manage | `{ candidate_version_id or patch ref }` | ChangeImpactAnalysis | 400,401,403,404,422 | Safe |

### 10.1 Error semantics (mandatory)

| Code | Use |
|---|---|
| 400 | Malformed body, invalid UUID |
| 401 | Unauthenticated |
| 403 | Authenticated but not owner/manage |
| 404 | Cross-tenant or unknown id (fail-closed) |
| 409 | Optimistic concurrency, immutable conflict, draft exists on restore |
| 422 | Readiness fail, impact confirmation required/missing |

---

## 11. Transaction and concurrency model (frozen)

| Operation | Transaction boundary | Locks |
|---|---|---|
| Publish template | Single TX: validate → flip status → audit | Row lock template version |
| Publish questionnaire version | TX: impact check → supersede old → publish new → update event.draft_version_id | Lock event + draft version row |
| Restore as draft | TX: verify no draft → deep copy → audit | Lock event |
| Clone from template | TX: create event + version + copy graph + provenance | — |
| Concurrent draft edits | Optimistic `version` increment (existing) | — |
| Double publish | Prevented by partial unique index + TX | |
| Response during publish | Responses pin old version id; publish does not mutate submitted rows | |
| Re-scoring collision | Deferred; if implemented, row-level lock on response + idempotent replace | |

**No silent last-write-wins** for material publishes or restore.

---

## 12. Frontend scope (frozen)

### 12.1 New Studio surfaces

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

### 12.2 Immutable published state

Published version opens **read-only** in Studio (existing scoring lock pattern extended to questionnaire).

---

## 13. Audit events (frozen names)

| Action | When |
|---|---|
| `template.created` | Template create |
| `template.published` | Template version publish |
| `template.archived` | Template archive |
| `rfx.version.published` | Questionnaire version publish |
| `rfx.version.superseded` | Prior version superseded |
| `rfx.version.restored_as_draft` | Restore operation |
| `rfx.version.compared` | Compare invoked (optional) |
| `rfx.event.created_from_template` | Clone |
| `rfx.change_impact.previewed` | Impact preview |
| `rfx.change_impact.confirmed` | Buyer confirmed material publish |

---

## 14. Stop conditions (implementation phase)

Implementation must **STOP** and escalate if:

- Any design requires in-place edit of PUBLISHED questionnaire or score model
- Any design re-binds submitted responses without explicit new response workflow
- Migration 000068 would alter v3.0D score/qualification uniqueness
- Cross-tenant template visibility introduced without ADR
- Silent re-scoring proposed

---

**CONTROLLER_ACCEPTANCE:** PENDING  
**NEXT_ACTION:** Review test strategy + open decisions OD-E01–E07

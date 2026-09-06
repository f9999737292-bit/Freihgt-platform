# RFx v3.0D — Scoring + Knockout Discovery Report

**Status:** `DISCOVERY_COMPLETE` — implementation **NOT authorized**  
**Mode:** `DISCOVERY_AND_IMPLEMENTATION_PLAN_ONLY`  
**Base:** `origin/main` @ `d53f2de7e31c4066a628e02d353961d299ac6192` (PR103 merged)  
**Branch:** `feat/bintrans-enterprise-rfx-scoring-knockout-v3.0d`  
**Worktree:** `D:\Projects\freight-platform-wt\rfx-scoring-knockout-v3.0d`  
**v3.0C frozen head:** `2c5d87f787aafd1be935d2059b8f30c691d2b212`

---

## 1. Baseline verification

| Check | Result |
|---|---|
| `origin/main` | `d53f2de7e31c4066a628e02d353961d299ac6192` ✓ |
| PR103 feature head ancestor of main | YES (`2c5d87f…`) ✓ |
| Fresh worktree from main | YES — no v3.0C worktree reuse |
| `WORKTREE_CLEAN` | YES (discovery doc only) |
| `PRODUCT_CODE_CHANGED` | NO |

---

## 2. Architecture reconciliation

Normative docs read: `RFX_V3_SCORING_ENGINE.md`, `RFX_V3_DATA_MODEL.md`, `RFX_V3_ROADMAP.md`, `RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md`, `RFX_V3_QUESTIONNAIRE_ENGINE.md`, `RFX_V3_EVENTS.md`, `RFX_V3_API.md`, `RFX_V3_SECURITY.md`, `ADR-RFX-004`, `ADR-RFX-005`.

### 2.1 Frozen invariants (implementation plan MUST preserve)

| Invariant | Value |
|---|---|
| `SCORING_INPUT` | `PERSISTED_VALID_ANSWERS_ONLY` |
| `CLIENT_DRAFT_SCORING` | `FORBIDDEN` |
| `PREVIEW_SCORING` | `FORBIDDEN` |
| `INVALID_ANSWER_SCORING` | `FORBIDDEN` |
| `VALIDATION_ERROR_NE_BUSINESS_FAILURE` | `YES` |
| `KNOCKOUT_BLOCKS_SAVE` | `NO` |
| `KNOCKOUT_ANSWER_PERSISTED` | `YES` |
| `ANSWER_VALUE_REWRITE_BY_SCORING` | `NO` |
| `SCORING_SERVER_AUTHORITATIVE` | `YES` |
| `EXPLAINABILITY_REQUIRED` | `YES` |
| `SCORE_MODEL_VERSIONING_REQUIRED` | `YES` |
| `TENANT_ISOLATION_REQUIRED` | `YES` |

### 2.2 Architecture vs code gaps (findings)

| Area | Architecture | Current main | Gap |
|---|---|---|---|
| Score models / criteria | `rfx_score_models`, `rfx_score_criteria` | **Not migrated** | Full greenfield in v3.0D |
| Answer scores | `rfx_answer_scores` | **Not migrated** | Full greenfield |
| Qualification results | `rfx_qualification_results` | **Not migrated** | Minimal slice in v3.0D (see §5) |
| Qualification pools | `rfx_qualification_pools`, `rfx_pool_members` | **Not migrated** | **Defer v3.0F** |
| v1 evaluation | Fixed 70/30 commercial/manual | **Implemented** | Parallel path required |
| `score_model_version` on answers | Required for qualification-relevant answers | Column **absent** (`000066` has `rule_version` only) | Add in v3.0D migration |
| RFx event outbox | Transactional outbox for events | **Not migrated** | Audit-only (`000037`); emit via audit in v3.0D |
| Knockout in question rules | `KNOCKOUT` action in data model | Rules limited to `SHOW`, `HIDE`, `REQUIRE` (`000065`) | Knockout lives in **score model**, not questionnaire L2 rules |
| Studio evaluation step | Planned buyer UX | Nav entry `planned: true` only | Scoring config step new in v3.0D |

---

## 3. Current v1 evaluation inventory (repository-backed)

### 3.1 Weights and formula

| Field | Actual value |
|---|---|
| `V1_COMMERCIAL_WEIGHT` | **0.70** (hardcoded) |
| `V1_MANUAL_WEIGHT` | **0.30** (hardcoded) |

Source: `services/rfx-service/internal/domain/rfx_evaluation.go`

```go
return roundMoney(commercial*0.7 + (*manual)*0.3)
```

Commercial score: inverse-price normalization — lowest submitted offer in comparable set → 100, others `(lowest/amount)*100`. Non-comparable responses get commercial=0, total=manual-only.

### 3.2 Storage

| Field | Actual storage |
|---|---|
| `V1_SCORE_STORAGE` | `rfx.rfx_responses.commercial_score`, `total_score`, `evaluation_rank` |
| `V1_MANUAL_SCORE_STORAGE` | `rfx.rfx_responses.technical_score` (domain field `ManualScore` maps to DB `technical_score`) |
| `V1_TOTAL_SCORE_STORAGE` | `rfx.rfx_responses.total_score` |
| Commercial lines | `rfx.rfx_response_offer_lines` (`000038`) |
| Award | `rfx.rfx_awards` (`000038`) |

No per-answer or per-criterion score rows exist today.

### 3.3 Route set (rfx-service, proxied via api-gateway)

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/v1/rfx-events/{id}/responses` | List evaluation responses (buyer) |
| `POST` | `/v1/rfx-events/{id}/evaluation/recalculate` | Recalculate v1 scores |
| `PATCH` | `/v1/rfx-responses/{response_id}/evaluation` | Update manual score |
| `POST` | `/v1/rfx-events/{id}/start-evaluation` | Lifecycle transition |
| `POST` | `/v1/rfx-events/{id}/shortlist` | Shortlist |
| `POST` | `/v1/rfx-events/{id}/award-response` | Award winner |
| `POST` | `/v1/rfx-events/{id}/award` | Award event |
| `PATCH` | `/v1/rfx-responses/{response_id}` | Update commercial offer lines (carrier) |

Handlers: `evaluation_handler.go`, `router.go`.

### 3.4 Award dependency on score

- **Ranking** (buyer comparison UI): descending `total_score`, tie-break on lower `total_amount` — `RankEvaluationCandidates`.
- **`AwardResponse` does not gate on scores or rank.** Eligibility requires: buyer access, response `SUBMITTED`, offer complete, commercially comparable (`currencyOK && total > 0`). Shortlist optional.
- Award does **not** require v3 qualification status today.
- `RecalculateEvaluation` persists commercial/manual/total/rank for display; award API does not enforce recalc having run.
- **Quirk:** `UpdateManualScore` sets solo `commercial=100` for one response (not peer-relative), unlike event-wide `RecalculateEvaluation`.
- **Dual award routes:** lifecycle `POST .../award` (`RfxCommandAward`) vs transactional `POST .../award-response` (creates `rfx_awards` record).

### 3.5 Frontend (v1)

- **Buyer evaluation UI:** `apps/web-procurement/pages/tenders/[id]/evaluation.vue`
- API: `apps/web-procurement/composables/useRfxEvaluationApi.ts`
- Shows: carrier, price, combined score, rank, shortlist, award, transport-order conversion
- **`updateManualScore` API exists but UI has no manual score editor** (display-only `total_score`)
- **No** criterion breakdown, knockout badge, or explainability drill-down
- **web-admin Studio:** evaluation nav step exists but `planned: true` — no scoring config UI

### 3.6 Compatibility risk

| Risk | Severity | Mitigation |
|---|---|---|
| Replacing 70/30 formula silently | **HIGH** | Parallel versioned path; v1 recalc unchanged when no v3 score model |
| Award winner semantics change | **HIGH** | Award continues on v1 `total_score` until explicit v3 award gate (future) |
| `technical_score` vs `manual_score` naming drift | **MEDIUM** | v3 uses new tables; v1 column untouched |
| Events with questionnaire + commercial offers | **MEDIUM** | v3 automatic score separate from commercial; UI shows both columns |
| Cross-tenant evaluation IDOR | **LOW** (guarded) | Reuse buyer event access patterns from evaluation service |

---

## 4. v3.0B/C input inventory

### 4.1 Latest migration

| Field | Value |
|---|---|
| `LATEST_MIGRATION` | **`000066_rfx_carrier_response_v3_0c`** |

Prior RFx v3: `000065_rfx_questionnaire_v3_0b`.

### 4.2 Scoring-relevant schema (merged)

| Table | Scoring relevance |
|---|---|
| `rfx.rfx_versions` | Published questionnaire snapshot; pin target |
| `rfx.rfx_sections`, `rfx_questions`, `rfx_question_options` | Question bindings, option maps |
| `rfx.rfx_question_rules` | L2 visibility/required only (`SHOW`/`HIDE`/`REQUIRE`) |
| `rfx.rfx_responses` | `rfx_version_id`, `save_version`, `status`, submit metadata |
| `rfx.rfx_answers` | Persisted valid answers (`answer_value_json`, `rule_version`) |

### 4.3 Availability flags

| Flag | Value | Evidence |
|---|---|---|
| `RFX_VERSION_PIN_AVAILABLE` | **YES** | `rfx_responses.rfx_version_id`; carrier start pins published version |
| `PERSISTED_ANSWERS_AVAILABLE` | **YES** | `rfx_answers` + integration tests in `carrierresponse` |
| `SUBMITTED_RESPONSE_TRIGGER_AVAILABLE` | **YES** (hook point) / **NO** (scoring not wired) | Transactional submit exists (`CarrierResponseService.Submit` + audit); no score/knockout side-effect yet |
| `RULE_VERSION_AVAILABLE` | **PARTIAL** | `rfx_answers.rule_version` column exists; **`UpsertBatch` does not write it** (stays NULL) |
| `SCORE_MODEL_VERSION_FIELD_AVAILABLE` | **NO** | Architecture field not in DB; required for v3.0D migration |

### 4.4 Submit flow (scoring hook point)

```
CarrierResponseService.Submit
  → validateWorkspace (L4)
  → runTx: LockResponseForUpdate → SubmitResponse (status=SUBMITTED)
  → audit response.submitted
  → return SubmitResult
```

**No scoring invocation today.** Canonical hook: immediately after successful submit transaction (see §12).

**Gap:** questionnaire version publish is validate-only (`POST .../validate-publish`); tests publish via direct SQL. RFx event publish is separate from questionnaire version publish.

Preview sandbox (`web-admin` v3.0C): **no** production answers, **no** scoring calls (`PREVIEW_DATA_ONLY=YES`).

---

## 5. v3.0D vs v3.0F boundary (explicit decision)

### 5.1 Tension

- **Scoring engine doc §5** pipeline emits `QualificationResult + AnswerScore`.
- **Roadmap** assigns qualification pools / RFI→RFQ to **v3.0F**.

### 5.2 Decision proposal

| Boundary | v3.0D | v3.0F |
|---|---|---|
| Score model definition | ✓ | |
| Score criteria + bindings | ✓ | |
| Deterministic automatic scoring | ✓ | |
| `rfx_answer_scores` rows | ✓ | |
| Aggregate per-response result | ✓ (`rfx_qualification_results` **minimal**) | |
| Knockout outcome + reason | ✓ | |
| Explainability payload | ✓ | |
| `rfx.score.calculated` event (audit/minimal) | ✓ | |
| Qualification pools | | ✓ |
| Pool membership mutation | | ✓ |
| RFI→RFQ handoff | | ✓ |
| Qualification lifecycle workflow | | ✓ |
| Downstream pool updater consumers | | ✓ |

### 5.3 Minimal qualification artifact in v3.0D

**`QUALIFICATION_RESULT_DECISION=IMPLEMENT_MINIMAL_V3_0D`**

One row per `(rfx_response_id, score_model_version)` with:

- `status`: `QUALIFIED` | `REJECTED` | `PENDING_REVIEW` (conditionally qualified optional if threshold rules exist)
- `total_score`, `knockout_reason_json`, `calculated_at`, `score_model_version`

**Rationale:** Knockout and buyer evaluation UX require a persisted server-authoritative outcome per response. This is **not** the v3.0F pool workflow — no pool tables, no membership, no handoff automation.

| Guard | Value |
|---|---|
| `V3_0D_QUALIFICATION_BOUNDARY` | Scoring outcome per response only; no pool/workflow |
| `QUALIFICATION_POOL_STARTED` | **NO** |
| `RFI_TO_RFQ_HANDOFF_STARTED` | **NO** |

---

## 6. Scoring mode matrix

| Mode | v3.0D classification | Notes |
|---|---|---|
| `AUTOMATIC` | **IMPLEMENT_V3_0D** | Core slice: NUMBER linear, YES_NO map, SINGLE_SELECT map, MULTI_SELECT deterministic sum/max |
| `MANUAL` | **DEFER** (compatibility only) | Keep v1 `PATCH .../evaluation` manual score for legacy/commercial events; do not unify into v3 criteria in v3.0D |
| `HYBRID` | **DEFER** | Requires manual override UX + audit on criterion rows — post v3.0D or late wave with controller auth |
| `SYSTEM_DERIVED` | **DEFER** (v3.0G) | Requires Carrier 360 / operational KPI ingestion — do not fake data |

---

## 7. Minimum v3.0D product slice

### 7.1 In scope (enterprise-complete minimum)

1. **Buyer Studio — Scoring tab** (`/rfx/:id/studio?step=scoring`)
   - Define score model bound to draft/published `rfx_version`
   - Criteria list (code, name, weight)
   - Question bindings (NUMBER, YES_NO, SINGLE_SELECT, MULTI_SELECT)
   - Normalization config per criterion
   - Knockout rules (YES_NO false, option match, numeric threshold)
   - Readiness validation before publish/bind

2. **Automatic scoring engine (server)**
   - Load persisted `rfx_answers` for submitted response
   - Skip invalid/missing answers (never score drafts)
   - Produce `rfx_answer_scores` + `rfx_qualification_results`
   - Full explainability JSON per answer

3. **Trigger on carrier submit** (see §12)

4. **Buyer evaluation UX extension** (web-procurement evaluation page **or** web-admin — recommend extend existing `tenders/[id]/evaluation.vue` with v3 columns when score model present)

5. **Integration suite** `rfx-scoring-v3-integration` (PostgreSQL 16)

6. **Browser E2E** (design only in this discovery; implement after controller auth)

### 7.2 Explicitly out of scope v3.0D

Qualification pools, RFI→RFQ handoff, templates/version-compare (v3.0E), Carrier 360 KPI, analytics dashboards, AI, notifications, approval chains, full outbox/Kafka (v3.0J), staging deploy, pilot execution.

---

## 8. Score model authoring ownership

| Field | Proposal |
|---|---|
| `BUYER_STUDIO_SCORING_OWNERSHIP` | **`apps/web-admin`** RFx Studio |

Current Studio nav (`studioNav.ts`):

- Active steps: `basics`, `questionnaire`, `validation`
- `evaluation` step: **planned placeholder** (post-response evaluation, not config)

**Proposal:** Add **`scoring`** step (or activate `evaluation` with distinct purpose — prefer new `scoring` id to avoid conflating config vs results):

```
/rfx/:id/studio?step=scoring
```

Buyer UX elements:

- Criteria list (code, name, weight %)
- Normalization editor per criterion
- Question binding picker (same `rfx_version` only)
- Knockout rule builder
- Model readiness panel + explainability preview (read-only simulation against sample answers — **no persistence** unless labeled preview)

---

## 9. Score model readiness rules

`SCORE_MODEL_READINESS_RULES` (server-authoritative; `validate-publish` / dedicated `validate-scoring` endpoint):

| Rule | Blocking |
|---|---|
| At least one criterion defined | YES |
| Criterion codes unique within model | YES |
| Weights sum to 100% (±0.01 tolerance) | YES |
| Each binding references question on same `rfx_version_id` | YES |
| Binding question type compatible with normalization type | YES |
| Knockout operator compatible with question type | YES |
| Numeric normalization min/max well-formed (min < max) | YES |
| Option map covers all scored options or explicit default | YES |
| No duplicate question→criterion binding conflict | YES |
| Model version immutable once published and responses exist | YES (policy) |

Client-only readiness checks: **FORBIDDEN** as gate.

---

## 10. Versioning model

| Question | Decision |
|---|---|
| `RfxVersion` → `ScoreModel` | 1:1 per version (or 1:N with single active — prefer **1:1** for determinism) |
| `CAN_DRAFT_SCORE_MODEL_CHANGE` | **YES** while version `DRAFT` |
| `CAN_PUBLISHED_SCORE_MODEL_MUTATE` | **NO** — new model_version row or new rfx_version |
| Score runs pin | `score_model_version` on `rfx_answer_scores` and `rfx_qualification_results` |
| Historical replay | Same answers + same model_version → identical scores (deterministic pure functions) |
| `MODEL_CHANGE_AFTER_RESPONSES` | v3.0D: **no automatic rescoring**; mark `SCORING_STALE` flag optional; full compare/rescore → **v3.0E** |

Answers: populate `score_model_version` on qualification-relevant answers at scoring time (backfill column in migration).

---

## 11. Migration proposal

**Next migration:** `000067_rfx_scoring_v3_0d` (do **not** implement in discovery phase)

### 11.1 New tables

#### `rfx.rfx_score_models`

| Column | Notes |
|---|---|
| `id` UUID PK | |
| `tenant_id` UUID NOT NULL | |
| `rfx_version_id` UUID FK → `rfx_versions` UNIQUE | One model per version |
| `model_type` VARCHAR | `AUTOMATIC` only in v3.0D |
| `model_version` INT NOT NULL | Monotonic per version |
| `status` VARCHAR | `DRAFT`, `PUBLISHED` |
| `definition_json` JSONB | Denormalized snapshot at publish |
| `created_at`, `updated_at` | |

#### `rfx.rfx_score_criteria`

| Column | Notes |
|---|---|
| `id` UUID PK | |
| `tenant_id` UUID NOT NULL | |
| `score_model_id` UUID FK | |
| `criterion_code` VARCHAR | UNIQUE per model |
| `name` VARCHAR | |
| `weight` NUMERIC(8,4) | |
| `normalization_json` JSONB | |
| `knockout_rule_json` JSONB | Nullable |
| `sort_order` INT | |

#### `rfx.rfx_score_bindings`

| Column | Notes |
|---|---|
| `id` UUID PK | |
| `tenant_id` UUID NOT NULL | |
| `criterion_id` UUID FK | |
| `question_id` UUID FK → `rfx_questions` | |
| `binding_json` JSONB | Option maps, thresholds |

#### `rfx.rfx_answer_scores`

| Column | Notes |
|---|---|
| `id` UUID PK | |
| `tenant_id` UUID NOT NULL | |
| `rfx_response_id` UUID FK | |
| `answer_id` UUID FK → `rfx_answers` | |
| `criterion_id` UUID FK | |
| `raw_score` NUMERIC | |
| `normalized_score` NUMERIC | |
| `weighted_contribution` NUMERIC | |
| `explanation_json` JSONB | See §15 |
| `score_model_version` INT | |
| `calculated_at` TIMESTAMPTZ | |

**Unique:** `(rfx_response_id, answer_id, criterion_id, score_model_version)`

#### `rfx.rfx_qualification_results` (minimal v3.0D)

| Column | Notes |
|---|---|
| `id` UUID PK | |
| `tenant_id` UUID NOT NULL | |
| `rfx_response_id` UUID FK | |
| `status` VARCHAR | `QUALIFIED`, `REJECTED`, `PENDING_REVIEW`, `CONDITIONALLY_QUALIFIED` |
| `total_score` NUMERIC | |
| `knockout_reason_json` JSONB | |
| `score_model_version` INT | |
| `calculated_at` TIMESTAMPTZ | |

**Unique:** `(rfx_response_id, score_model_version)`

### 11.2 ALTER existing

```sql
ALTER TABLE rfx.rfx_answers
  ADD COLUMN IF NOT EXISTS score_model_version INT;
```

### 11.3 NOT in v3.0D

- `rfx_qualification_pools`
- `rfx_pool_members`
- `rfx_event_outbox` (use audit trail for v3.0D event proof)

---

## 12. Scoring trigger design

| Field | Proposal |
|---|---|
| `V3_0D_SCORING_TRIGGER` | **`POST_SUBMIT_SYNCHRONOUS`** (Option B) |

Flow:

```
Submit TX commits (status=SUBMITTED, answers immutable)
  → ScoringService.Calculate(response_id)  // separate TX
  → upsert answer_scores + qualification_results
  → audit rfx.score.calculated + optional rfx.knockout.triggered
```

**Rejected options:**

- **A (inside submit TX):** Long lock, scoring failure rolls back valid submit evidence — **violates invariant**
- **C (async/outbox):** `rfx_event_outbox` not migrated; v3.0J scope — defer

| Field | Proposal |
|---|---|
| `V3_0D_FAILURE_POLICY` | Submit **never rolled back** on scoring failure; qualification row `PENDING_REVIEW` or `SCORING_FAILED` status; buyer sees "scoring pending/retry"; manual retry endpoint buyer-only |

---

## 13. Idempotency and recalculation

| Field | Value |
|---|---|
| `SCORING_IDEMPOTENCY_KEY` | `(tenant_id, rfx_response_id, score_model_version)` |

Policy:

- Re-trigger with same key → **UPSERT** answer_scores and qualification_results (replace calculated_at, same numeric outputs)
- Determinism proof required in integration tests
- **No** automatic rescoring when model changes after submit (v3.0E)
- Explicit buyer `POST .../scoring/recalculate` optional for ops recovery (same model version only in v3.0D)

---

## 14. Knockout semantics

| Field | Value |
|---|---|
| `KNOCKOUT_BLOCKS_SAVE` | **NO** |
| `KNOCKOUT_ANSWER_PERSISTED` | **YES** |

### 14.1 Required proof scenario

```
ADR_AVAILABLE = false  (valid YES_NO answer, persisted)
Model knockout: ADR mandatory
→ ANSWER_PERSISTED=YES
→ SAVE_ALLOWED=YES
→ SUBMIT_ALLOWED=YES (validation passes; answer is valid)
→ KNOCKOUT_TRIGGERED=YES at scoring time
→ qualification_results.status=REJECTED
→ Buyer sees knockout badge + explanation
→ Carrier response remains SUBMITTED evidence
```

Knockout is **post-validation business outcome**, never HTTP 422.

---

## 15. Explainability contract

`EXPLAINABILITY_SCHEMA` (stored in `rfx_answer_scores.explanation_json`):

```json
{
  "source": "CARRIER_DECLARED",
  "input": { "question_code": "FLEET_COUNT", "value": 50 },
  "rule": { "code": "HSE_FLEET_LINEAR", "version": 1 },
  "score_model_version": 1,
  "criterion": { "code": "CAPACITY", "weight": 0.6 },
  "raw_score": 50,
  "normalized_score": 83.33,
  "weighted_contribution": 50.0,
  "knockout": false,
  "knockout_reason": null
}
```

Excluded: SQL, stack traces, internal IDs except stable codes, secret metadata.

Aggregate-level knockout: `rfx_qualification_results.knockout_reason_json`.

---

## 16. Buyer evaluation UX plan

`BUYER_EVALUATION_UX_PLAN`:

Extend **`apps/web-procurement/pages/tenders/[id]/evaluation.vue`** when event has v3 score model:

| Column / panel | Content |
|---|---|
| Carrier | existing |
| Questionnaire score | v3 `total_score` from `qualification_results` |
| Commercial score | v1 `commercial_score` (unchanged) |
| Combined display | Show both; do **not** silently merge into award formula in v3.0D |
| Knockout badge | `REJECTED` qualification status |
| Criterion breakdown | Expand row → criterion scores |
| Explainability | Drill-down modal from `explanation_json` |
| Manual score | v1 column retained for legacy; labeled "Manual (legacy)" |
| Override | **Hidden** in v3.0D (deferred) |

**web-admin** post-award analytics: defer v3.0H.

---

## 17. Manual override authority

| Field | Value |
|---|---|
| `MANUAL_SCORE_AUTHORITY` | `PROCUREMENT_MANAGER`, `SHIPPER_ADMIN`, `FORWARDER_MANAGER`, `PLATFORM_ADMIN` (existing buyer roles) |
| `KNOCKOUT_OVERRIDE_AUTHORITY` | **NONE in v3.0D** — no `SENIOR_BUYER` role in repository |
| `ROLE_GAP` | Architecture requires senior buyer/admin for knockout override with mandatory comment — **defer override to post-v3.0D** or map to `PLATFORM_ADMIN` only with audit when controller authorizes |

Do **not** invent browser-only authority.

---

## 18. Security proof plan

| Proof | v3.0D approach |
|---|---|
| `CROSS_TENANT_SCORE_MODEL_DENY` | Integration: tenant A cannot read/write tenant B model |
| `CROSS_COMPANY_EVALUATION_DENY` | Buyer company membership gate on all scoring endpoints |
| `CARRIER_SCORE_MUTATION_DENY` | Carrier JWT → 403 on score model + manual override + recalculate |
| `HEADER_SPOOF_DENY` | Gateway RBAC tests (pattern from carrier-response browser security tests) |
| `PREVIEW_SCORE_PERSISTENCE=0` | Preview sandbox makes zero scoring API calls; integration assertion |

Carriers: may see **own** qualification status policy TBD (default: own summary only, no other carriers' scores).

---

## 19. Proposed API matrix (v3.0D)

Extend `rfx-service` under existing `/v1/rfx-events/{id}` namespace — **no duplicate v1 paths**.

### Buyer score model (draft version)

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/v1/rfx-events/{id}/score-model` | Get model + criteria + bindings |
| `PUT` | `/v1/rfx-events/{id}/score-model` | Upsert draft model |
| `POST` | `/v1/rfx-events/{id}/score-model/validate` | Readiness checklist |
| `POST` | `/v1/rfx-events/{id}/score-model/publish` | Pin model to published version |

### Evaluation / results

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/v1/rfx-events/{id}/responses/{response_id}/score` | Aggregate + criterion scores |
| `GET` | `/v1/rfx-events/{id}/responses/{response_id}/score/explanation` | Full explainability |
| `POST` | `/v1/rfx-events/{id}/responses/{response_id}/scoring/recalculate` | Idempotent retry (buyer) |

**Legacy preserved:**

| Method | Path | Status |
|---|---|---|
| `POST` | `/v1/rfx-events/{id}/evaluation/recalculate` | Unchanged v1 commercial/manual |
| `PATCH` | `/v1/rfx-responses/{response_id}/evaluation` | Unchanged v1 manual |

OpenAPI: extend `packages/openapi` in implementation phase.

---

## 20. Event contract proposal

Event: **`rfx.score.calculated.v1`** (audit record in v3.0D; outbox deferred)

Minimal payload:

```json
{
  "tenant_id": "uuid",
  "rfx_event_id": "uuid",
  "rfx_version_id": "uuid",
  "rfx_response_id": "uuid",
  "score_model_id": "uuid",
  "score_model_version": 1,
  "total_score": 72.5,
  "qualification_status": "QUALIFIED",
  "knockout_triggered": false,
  "calculated_at": "ISO8601",
  "correlation_id": "uuid"
}
```

Knockout side event: `rfx.knockout.triggered.v1` when `knockout_triggered=true`.

---

## 21. PostgreSQL integration test plan

**Job name:** `rfx-scoring-v3-integration`  
**Requirements:** PostgreSQL 16, `REQUIRE_TEST_DATABASE=1`, fail-closed

| Case ID | Description |
|---|---|
| `SCORE_MODEL_CREATE` | Buyer creates draft model on published version |
| `SCORE_MODEL_TENANT_ISOLATION` | Cross-tenant read/write denied |
| `CRITERIA_WEIGHT_VALID` | Weights sum 100% → validate pass |
| `CRITERIA_WEIGHT_INVALID` | Weights ≠ 100% → validate fail |
| `NUMBER_LINEAR_SCORE` | NUMBER normalization deterministic |
| `BOOLEAN_SCORE` | YES_NO mapping |
| `OPTION_SCORE` | SINGLE_SELECT option map |
| `WEIGHTED_AGGREGATION` | Criterion weights → total |
| `PERSISTED_VALID_ANSWER_ONLY` | Draft/invalid answers excluded |
| `INVALID_ANSWER_NOT_SCOREABLE` | 422 answers never in `rfx_answers` → not scored |
| `KNOCKOUT_VALID_NEGATIVE` | ADR_AVAILABLE=false triggers knockout |
| `KNOCKOUT_ANSWER_PRESERVED` | Answer row unchanged after knockout |
| `KNOCKOUT_BLOCKS_SAVE_NO` | Knockout never returns 422 on save |
| `EXPLAINABILITY_COMPLETE` | All mandatory explanation fields present |
| `SCORE_MODEL_VERSION_PINNED` | Scores record model_version |
| `SCORING_IDEMPOTENT` | Double trigger → same scores, no dup rows |
| `SUBMIT_TRIGGERS_SCORE` | Submit → qualification row exists |
| `CROSS_TENANT_DENY` | Scoring results tenant scoped |
| `CARRIER_MUTATION_DENY` | Carrier cannot publish model |
| `HEADER_SPOOF_DENY` | Spoofed company header rejected |
| `LEGACY_V1_EVALUATION_COMPATIBILITY` | v1 recalc unchanged without v3 model |
| `AWARD_FLOW_NOT_REGRESSED` | Existing award integration tests pass |

Manual override / knockout override audit cases: **skip until override authorized**.

---

## 22. Browser acceptance plan (design only)

**Job name:** `rfx-scoring-v3-browser-e2e` (future)

Stack: Chromium → web-admin (scoring config) + web-procurement (carrier + evaluation) → production api-gateway → rfx-service → PostgreSQL 16. **No scoring API mocks.**

Scenario (deterministic):

1. Buyer configures scoring in Studio: HSE 40%, Capacity 60%
2. Bind `ADR_AVAILABLE` (boolean knockout: false → KO), `FLEET_COUNT` (number linear 0–100)
3. Publish event + score model
4. Carrier A: `ADR_AVAILABLE=true`, `FLEET_COUNT=50` → expect `QUALIFIED`, total = `0.4*100 + 0.6*50 = 70` (example normalization)
5. Carrier B: `ADR_AVAILABLE=false`, `FLEET_COUNT=100` → expect `REJECTED`, answer persisted, knockout badge
6. Buyer evaluation: explanation shows raw/normalized/weight/contribution/model version
7. Carrier cannot access score model PUT

---

## 23. Legacy evaluation compatibility

`LEGACY_EVALUATION_COMPATIBILITY_PLAN=PARALLEL_VERSIONED_PATH_WITH_EXPLICIT_SWITCH`

| Condition | Behavior |
|---|---|
| Event has **no** v3 score model on published version | v1 `RecalculateEvaluation` only (70/30 commercial/manual) |
| Event has **published** v3 score model | v3 automatic scoring on submit; v1 commercial recalc still available for offer comparison |
| Award selection | **Unchanged in v3.0D** — still uses v1 `total_score`/rank unless controller authorizes v3 award gate later |
| UI | Show v3 questionnaire score separately from commercial/manual |

**Do not remove** legacy 70/30 behavior. **Do not** silently change award winner selection.

---

## 24. Recommended implementation sequence

1. Migration `000067` + domain types + repositories (score model CRUD)
2. Scoring engine pure functions + unit tests (normalization, knockout, aggregation)
3. `ScoringService.Calculate` + idempotent upsert
4. Hook post-submit in `CarrierResponseService`
5. Buyer API routes + gateway RBAC
6. Studio scoring step (web-admin)
7. Evaluation UX columns (web-procurement)
8. `rfx-scoring-v3-integration` CI job
9. Browser E2E (after integration green)
10. OpenAPI + docs update

---

## 25. Risks and open findings

| ID | Finding | Severity | Recommendation |
|---|---|---|---|
| F-D01 | Architecture emits `QualificationResult` in v3.0D but pools in v3.0F | Medium | Accepted boundary (§5) |
| F-D02 | `score_model_version` not on answers yet | High | Include in `000067` |
| F-D03 | No `SENIOR_BUYER` role for override | Medium | Defer override |
| F-D04 | v1 `technical_score` naming vs `manual_score` API | Low | Keep parallel; document mapping |
| F-D05 | Event outbox not migrated | Medium | Audit-only events v3.0D |
| F-D06 | Award still v1-weighted | Medium | Explicit UX separation; no silent merge |
| F-D07 | MULTI_SELECT normalization spec incomplete in architecture | Low | Define sum-of-selected or max in implementation contract |
| F-D08 | Roadmap still says v3.0C "PENDING_CONTROLLER" | Low | Update roadmap when controller closes v3.0C |

---

## 26. Verdict

| Field | Value |
|---|---|
| `V3_0D_DISCOVERY_VERDICT` | **PASS_WITH_FINDINGS** |
| `V3_0E_STARTED` | **NO** |
| `V3_0F_STARTED` | **NO** |
| `STAGING_CHANGED` | **NO** |
| `PILOT_CHANGED` | **NO** |
| `NEXT_ACTION` | **CONTROLLER_AUTHORIZE_V3_0D_IMPLEMENTATION** |

---

## Appendix A — Quick reference matrix

| Metric | Value |
|---|---|
| `V1_COMMERCIAL_WEIGHT` | 0.70 |
| `V1_MANUAL_WEIGHT` | 0.30 |
| `V1_SCORE_STORAGE` | `rfx_responses.commercial_score`, `technical_score`, `total_score`, `evaluation_rank` |
| `V1_EVALUATION_ROUTE_SET` | See §3.3 |
| `RFX_VERSION_PIN_AVAILABLE` | YES |
| `PERSISTED_ANSWERS_AVAILABLE` | YES |
| `SUBMITTED_RESPONSE_TRIGGER_AVAILABLE` | YES |
| `V3_0D_SCORING_TRIGGER` | POST_SUBMIT_SYNCHRONOUS |
| `V3_0D_FAILURE_POLICY` | Never rollback submit; scoring retry/stale |
| `SCORING_IDEMPOTENCY_KEY` | (tenant_id, rfx_response_id, score_model_version) |
| `KNOCKOUT_SEMANTICS` | Post-validation business outcome; answer preserved |
| `QUALIFICATION_POOL_STARTED` | NO |

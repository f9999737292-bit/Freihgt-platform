# RFx v3.0E — Templates + Versioning Discovery Report

**Status:** `DISCOVERY_COMPLETE` — implementation **NOT authorized**
**CONTROLLER_ACCEPTANCE:** `YES`
**ARCHITECTURE_STATUS:** `FROZEN_ACCEPTED`
**IMPLEMENTATION_AUTHORIZED:** `NO`
**Mode:** `DISCOVERY_AND_ARCHITECTURE_FREEZE_ONLY`  
**Base:** `origin/main` @ `0db472d6a4b79d987cb98f28253383fd4a88821d` (PR #105 merged)  
**Branch:** `discovery/rfx-templates-versioning-v3.0e`  
**Worktree:** `D:\Projects\freight-platform-wt\rfx-templates-versioning-v3.0e`  
**Prior accepted streams:** v3.0B, v3.0C, v3.0D = `IMPLEMENTED_ACCEPTED`

---

## 1. Baseline verification

| Check | Result |
|---|---|
| `origin/main` SHA | `0db472d6a4b79d987cb98f28253383fd4a88821d` |
| v3.0D merged | YES (PR #104 @ `70e621c`, docs closeout PR #105) |
| Migration 000068 | **NOT created** |
| Product code changed in this stream | **NO** (discovery only) |
| `IMPLEMENTATION_ALLOWED` | **NO** |
| `STOP_AFTER_V3_0E_DISCOVERY` | **YES** |

---

## 2. Normative inputs reviewed

| Document | Relevance |
|---|---|
| `RFX_V3_ROADMAP.md` §6 | v3.0E scope |
| `ADR-RFX-002-QUESTIONNAIRE-VERSIONING.md` | Immutable published versions |
| `ADR-RFX-008-TEMPLATE-VERSIONING.md` | Template library model |
| `RFX_V3_DATA_MODEL.md` §3.1 | Target tables |
| `RFX_V3_DOMAIN_MODEL.md` §7 | Versioning + ChangeImpactAnalysis |
| `RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md` §20–21 | Change impact, compare/restore |
| `RFX_V3_API.md` §7 | Post-publish versioned edit |
| v3.0B/C/D implementation reports | Actual shipped behavior |

No new ADR duplicates created; ADR-002 and ADR-008 remain authoritative and are extended by the architecture freeze.

---

## 3. Current-state evidence (code, not docs)

### 3.1 Schema shipped (migrations 000065–000067)

| Artifact | Evidence |
|---|---|
| `rfx_versions` | `infrastructure/migrations/000065_rfx_questionnaire_v3_0b.up.sql` |
| Questionnaire tree | `rfx_sections`, `rfx_questions`, `rfx_question_options`, `rfx_question_rules` |
| Carrier pinning | `rfx_responses.rfx_version_id`, `save_version` — migration 000066 |
| Score pinning | `rfx_score_models.rfx_version_id`, `score_model_version` on results — migration 000067 |
| Audit | `rfx.audit_events` — migration 000037 |

**Not migrated:** `rfx_templates`, `rfx_template_versions`, version compare/restore tables, change-impact cache, transactional outbox.

### 3.2 Backend services (rfx-service)

| Area | File | Implemented |
|---|---|---|
| Draft questionnaire CRUD | `internal/service/questionnaire_service.go` | YES |
| Draft-only guard | `domain.EnsureDraftVersionMutable` | YES |
| Publish readiness | `domain.EvaluatePublishReadiness` | YES (validate only) |
| Questionnaire publish API | — | **NO** (tests use raw SQL) |
| `SUPERSEDED` transition | — | **NO** (constant exists, never set) |
| Score model publish | `score_model_service.go` | YES (v3.0D) |
| Carrier response pinning | `carrier_response_service.go` | YES |
| Question duplicate (in-event) | `DuplicateQuestion` | YES |
| Template library | — | **NO** |
| Version compare/restore | — | **NO** |
| Change-impact analysis | — | **NO** |

### 3.3 Frontend (web-admin Studio)

| Capability | Evidence | Status |
|---|---|---|
| Studio steps basics/questionnaire/scoring/validation | `components/rfx/studio/studioNav.ts` | YES |
| Autosave + conflict reload | `composables/useRfxQuestionnaireApi.ts` | YES |
| Publish readiness panel | `RfxPublishReadinessPanel.vue` | YES (questionnaire validate) |
| Version history UI | — | **NO** |
| Compare view | — | **NO** |
| Template library | — | **NO** |
| Restore confirmation | — | **NO** |
| Material-change warning | — | **NO** |

Planned nav entries still marked `planned: true`: participants, evaluation, communications, publication.

### 3.4 OpenAPI / router

Implemented: studio, questionnaire CRUD, validate-publish, score-model publish, carrier response.  
**Absent:** `/templates`, `/versions`, `/versions/compare`, `/versions/{id}/restore`, change-impact preview.

Evidence: `packages/openapi/rfx-service.yaml`, `services/rfx-service/internal/http/router.go`.

---

## 4. Capability inventory

| CAPABILITY | CURRENT IMPLEMENTATION | FILE/EVIDENCE | GAP | V3.0E DECISION |
|---|---|---|---|---|
| Template entity | Not migrated | ADR-008 spec only | Full greenfield | **Implement** `rfx_templates` + `rfx_template_versions` |
| Template ownership | Tenant + optional owner company in spec | `RFX_V3_DATA_MODEL.md` §3.1 | No code | **Freeze:** tenant-scoped; owner company optional |
| Tenant visibility | Tenant predicates on all RFx tables | `questionnaire_repository.go` | Templates missing | **Reuse** existing tenant isolation pattern |
| Template lifecycle | Spec: DRAFT/ACTIVE/ARCHIVED | ADR-008 | No code | **Freeze:** aggregate ACTIVE/ARCHIVED; version DRAFT/PUBLISHED/SUPERSEDED |
| RFx version entity | `rfx_versions` table + domain | migration 000065, `questionnaire.go` | Publish workflow incomplete | **Extend** with publish/supersede API |
| Immutable published version | Draft-only mutations enforced | `EnsureDraftVersionMutable` | No formal publish | **Implement** publish + immutability gate |
| Create draft from published | Partial: `GetOrCreateDraftVersion` | `questionnaire_repository.go:46-70` | No supersede semantics | **Implement** explicit fork-from-published |
| Compare versions | Spec only | validation contract §21 | No API/UI | **Implement** canonical diff engine |
| Restore version | Spec: new draft, not rewind | validation contract §21 | No API | **Implement** restore-as-new-draft |
| Clone RFx from template | Not present | — | Full gap | **Implement** template → event materialization |
| Change-impact analysis | Domain type in spec | `RFX_V3_DOMAIN_MODEL.md` | No service | **Implement** classification + preview API |
| Active-response protection | Response pinned to `rfx_version_id` | migration 000066, carrier service | No material-change gate | **Implement** impact classes + publish guard |
| Scoring-model compatibility | Score model per `rfx_version_id` | migration 000067 | No cross-version scoring rules | **Freeze:** published score model immutable; new version → new model draft |
| Audit history | `audit_events` table + recordAudit | `audit_repository.go` | No version-specific actions | **Extend** audit actions for version/template ops |
| Archive/delete | Soft delete on version rows | `deleted_at` on versions | No archive API | **Implement** aggregate ARCHIVED; version SUPERSEDED (not version ARCHIVED) |
| Optimistic concurrency | `version` on draft rows; `save_version` on responses | repos + services | Present | **Reuse**; extend to template aggregates |
| Multilingual content | i18n keys in web-admin | `apps/web-admin/i18n/*.json` | Template labels inline today | **Freeze:** template metadata i18n map; questionnaire labels per existing pattern |
| Authorization | Buyer owner-company gate | `questionnaire_service.go:576-601` | Template roles undefined | **Freeze** matrix in architecture doc |
| OpenAPI | Partial v3.0B–D | `rfx-service.yaml` | v3.0E routes missing | **Propose** in freeze doc |
| Buyer Studio UI | Builder + scoring | web-admin studio | History/compare/template missing | **Design** in freeze + test strategy |
| Integration/browser tests | v3.0B–D gates exist | CI jobs | No v3.0E scenarios | **Define** in test strategy |

---

## 5. Roadmap consistency finding

**Finding:** `RFX_V3_ROADMAP.md` contained a misleading `STOP_AFTER_V3_0A=YES` marker implying no implementation beyond v3.0A.

**Assessment:**

| Aspect | Conclusion |
|---|---|
| Historical intent | Marker for the completed v3.0A architecture-only stream |
| Current accuracy | **Misleading** — v3.0B/C/D are `IMPLEMENTED_ACCEPTED` |
| Controller action | **RECONCILED** in this PR — marker annotated to apply only to v3.0A stream |

**Reconciled text (in roadmap):**

```text
STOP_AFTER_V3_0A=YES applies only to the completed v3.0A architecture-freeze stream.
Subsequent release implementation requires an explicit controller gate per release.
```

---

## 6. v3.0D compatibility constraints (discovery)

Verified in code after v3.0D merge:

| Rule | Evidence |
|---|---|
| Responses pin `rfx_version_id` at start | `carrier_response_service.go` ensureCarrierResponse |
| Answers keyed by `question_id` within response | `rfx_answers` UNIQUE (response, question) |
| Scores pin `score_model_version` | `rfx_answer_scores`, `rfx_qualification_results` |
| Published score model immutable | `score_model_service.go` Publish + Conflict on PUT |
| Rescore replaces rows for same model version only | `score_repository.go` ReplaceScoringResults |
| Knockout/validation semantics frozen | v3.0D controller acceptance |

v3.0E **must not** break these invariants.

### 6.1 Carrier response continuity gap (current code)

**Evidence:** `carrier_response_service.go` `ensureCarrierResponse` loads the **current** published version and returns **409** when an existing response's `rfx_version_id` differs from that published row (lines 417–422).

**Gap:** This blocks draft responses from save/resume/submit after a new questionnaire version is published, violating v3.0E continuity requirements.

**Remediation (frozen):** Responses remain pinned to the version selected at creation; **`rfx_version_id` immutable from creation** (draft, resume, submit, after submit). Workspace/save/submit load questionnaire by **response.rfx_version_id**, not current published. `save_version` is concurrency-only and does not permit re-pinning. See architecture freeze §3.4.

---

## 7. Controller decisions closed

| ID | Decision | Status |
|---|---|---|
| **OD-E01** | Global/cross-tenant templates **DEFERRED**. v3.0E supports **tenant-scoped templates only**. | **CLOSED** |
| **OD-E02** | **One active DRAFT** per event and **one active DRAFT** per template. Concurrent draft creation → **409**. | **CLOSED** |
| **OD-E03** | Template questionnaire graph in **separate normalized tables** (`rfx_template_sections`, `rfx_template_questions`, `rfx_template_question_options`, `rfx_template_question_rules`). **No** dual-owner nullable FK on event questionnaire tables. Clone materializes template graph into event-version graph. | **CLOSED** |
| **OD-E04** | Nullable provenance FK `rfx_events.source_template_version_id`. **Immutable** after event creation. | **CLOSED** |
| **OD-E05** | v3.0E computes and persists **`RESCORING_REQUIRED` only**. Automatic and manual re-score of existing responses **not in v3.0E**. No silent re-score. | **CLOSED** |
| **OD-E06** | **Any** questionnaire/scoring graph change after publication creates a **new DRAFT version**. In-place edits of published graph **forbidden** (including typo/label changes). | **CLOSED** |
| **OD-E07** | **Separate aggregate and version lifecycle:** template aggregate ACTIVE→ARCHIVED; template/event version DRAFT→PUBLISHED→SUPERSEDED. ARCHIVED applies to aggregate/event retirement, **not** as substitute for SUPERSEDED version history. | **CLOSED** |

---

## 8. Critical risks

| ID | Risk | Mitigation (architecture) |
|---|---|---|
| CR-E01 | Silent re-bind of responses to new version | Fail-closed: `rfx_version_id` immutable from creation; new responses only on new pin |
| CR-E02 | Editing published questionnaire in place | Immutable status + DB/service guards + 409 on mutation |
| CR-E03 | Restore rewinds mutable state | Restore always creates **new** DRAFT row with incremented `version_number` |
| CR-E04 | Score history loss on material change | Old `score_model_version` rows preserved; no DELETE on publish |
| CR-E05 | Cross-tenant template leak | Tenant predicate on all template/version queries; 404 on foreign ID |
| CR-E06 | Concurrent publish double-write | Transaction + unique partial index on PUBLISHED per event |
| CR-E07 | Last-write-wins on draft edits | Existing optimistic `version` tokens — preserve |

---

## 9. Explicit deferred scope

| Item | Target wave |
|---|---|
| Qualification pools | v3.0F |
| Carrier 360 autofill | v3.0G |
| Analytics dashboards | v3.0H |
| AI assist | v3.0I |
| Transactional outbox / notifications / approval chains | v3.0J |
| Cross-tenant marketplace templates | Post-v3.0E |
| Automatic destructive migration of legacy RFx | **Never** without controller ADR |
| Silent automatic re-scoring | **Forbidden** in v3.0E |
| Editing published versions in place | **Forbidden** |

---

## 10. Implementation waves (proposal — not authorized)

| Wave | Scope |
|---|---|
| E1 | Questionnaire publish/supersede + version list API |
| E2 | Compare + restore-as-draft |
| E3 | Change-impact preview + publish confirmation |
| E4 | Template library CRUD + publish |
| E5 | Clone event from template + provenance |
| E6 | Studio UI (library, history, compare, restore) |
| E7 | Browser acceptance gate |

---

## 11. Validation performed (this stream)

| Check | Result |
|---|---|
| Code inspection | PASS (rfx-service, migrations, web-admin, OpenAPI) |
| Normative doc cross-check | PASS |
| Product code modified | **NO** |
| Migration 000068 created | **NO** |

---

**NEXT_ACTION:** `RETURN_TO_CONTROLLER_FOR_V3_0E_IMPLEMENTATION_PLAN`

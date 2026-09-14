# RFx v3.0E7 — Carrier XLSX Exchange Discovery

**Status:** `IMPLEMENTATION_IN_PROGRESS`

**Base:** `origin/main` @ `87916ab2bfd83ec15605446c02f2cd0c0ee6c79d` (post PR #130 merge)
**Discovery branch:** `discovery/rfx-carrier-xlsx-exchange-v3.0e7-phase2`
**Discovery commit:** `85b5157842bb24e1078a26c4884c4b708c3f8b9c`
**C1 merge:** PR #132 @ `fa850826c6f7ed7f25e948dcc8fc8cad41eb2d74` (2026-09-14)

| Marker | Value |
|---|---|
| `STATUS` | `IMPLEMENTATION_IN_PROGRESS` |
| `ARCHITECTURE_REVIEW_RESULT` | `GO` |
| `ARCHITECTURE_FREEZE` | `IMPLEMENTED_ACCEPTED` |
| `CARRIER_XLSX_IMPLEMENTATION_AUTHORIZED` | `YES` |
| `CARRIER_XLSX_IMPLEMENTATION_STARTED` | `YES` |
| `CARRIER_XLSX_C1_EXPORT_STATUS` | `IMPLEMENTED_ACCEPTED` |
| `CARRIER_XLSX_C2_PREVIEW_STATUS` | `IMPLEMENTED_ACCEPTED` |
| `CARRIER_XLSX_C3_COMMIT_STATUS` | `IMPLEMENTED_PENDING_REVIEW` |
| `CARRIER_XLSX_OVERALL_STATUS` | `IMPLEMENTATION_IN_PROGRESS` |
| `C1_EXPORT_TEST_IDS` | `E7P2-INT-71..79` |
| `CARRIER_XLSX_SCHEMA` | `BINTRANS_RFX_CARRIER_XLSX_V1` |
| `CARRIER_XLSX_EXPORT_PROPOSED` | `YES` |
| `CARRIER_XLSX_PREVIEW_PROPOSED` | `YES` |
| `CARRIER_XLSX_COMMIT_PROPOSED` | `YES` |
| `CARRIER_XLSX_COMMIT_RESULT` | `DRAFT_ONLY` |
| `CARRIER_XLSX_COMMIT_AUTO_SUBMIT` | `NO` |
| `COMPETITOR_CONFIDENTIALITY` | `MANDATORY_FAIL_CLOSED` |
| `MIGRATION_000073_SUFFICIENT` | `YES` |
| `MIGRATION_000074_REQUIRED` | `NO` |
| `MIGRATION_000074_CREATED` | `NO` |
| `MAX_MIGRATION_CONTRACT` | `000073` |
| `TEST_MATRIX` | `E7P2-INT-71..119` |
| `TEST_CASE_COUNT` | `49` |
| `MATRIX_GUARD` | `EXTRA_NO_INT_ID` |
| `NEXT_TEST_ID_AFTER_CARRIER_XLSX` | `E7P2-INT-120` |
| `E7_PHASE2_STATUS` | `IMPLEMENTATION_IN_PROGRESS` |
| `CREATE_FROM_XLSX_STATUS` | `OUT_OF_SCOPE` / `NOT_STARTED` |
| `ERP_API_STATUS` | `OUT_OF_SCOPE` / `NOT_STARTED` |
| `RFx_RATE_CORRECTION_STATUS` | `OUT_OF_SCOPE` / `NOT_STARTED` |

---

## 1. Status and scope

This document freezes architecture for **Carrier XLSX Export**, **Import Preview**, and **Import Commit** as the next E7 Phase 2 increment after Buyer XLSX Import P4 (`IMPLEMENTED_ACCEPTED`, PR #129/#130).

| In scope (Carrier XLSX v1) | Out of scope |
|---|---|
| Export / preview / commit for **own** carrier questionnaire response | Buyer XLSX (already accepted) |
| Workbook schema `BINTRANS_RFX_CARRIER_XLSX_V1` (distinct from buyer) | Create from XLSX |
| Persisted preview analysis + atomic commit from stored proposal | ERP / SAP / 1C API |
| Questionnaire answers + own commercial offer lines | Frontend UI, training, browser acceptance |
| DRAFT-only mutation; **no auto-submit** | RFx Rate Correction backlog |
| Competitor confidentiality fail-closed | Traffic-light / anomalously-low-rate logic changes |
| Reuse migration 000073 `rfx_import_analyses` | Late-submission workflow changes |
| Integration tests E7P2-INT-71..119 + matrix guard | Staging / pilot deployment |
| | Migration 000074 (not required; not authorized) |

### Normative business invariant

```
CARRIER_XLSX_COMMIT_AUTO_SUBMIT=NO
CARRIER_XLSX_COMMIT_RESULT=DRAFT_ONLY
DIRECT_SUBMIT_ENDPOINT_UNCHANGED=YES
LATE_SUBMISSION_FLOW_UNCHANGED=YES
```

Carrier XLSX Import Commit **only** applies a normalized proposal to a **DRAFT** response (answers + own offer lines). It **never** calls submit, never sets `submitted_at`, never emits `response.submitted` audit, never consumes late-submission permission, and never triggers scoring. The carrier must explicitly invoke the existing submit endpoint afterward.

---

## 2. Normative invariants

| Invariant | Requirement |
|---|---|
| Workbook isolation | `BINTRANS_RFX_CARRIER_XLSX_V1` only; never reuse buyer workbook as carrier workbook |
| Ownership | Export/preview/commit limited to response owner (carrier company + actor binding) |
| Tenant isolation | All reads/writes scoped by `tenant_id`; cross-tenant → 404 |
| Metadata untrusted | Server re-resolves identity, response, questionnaire version, baselines |
| Preview writes | **No** response/answer/offer-line DB writes on preview |
| Binary persistence | **No** XLSX binary stored; canonical JSON + hash only |
| Commit input | `{ "analysis_id": "<uuid>" }` + mandatory `Idempotency-Key` |
| Commit source | Stored normalized proposal only; **no** XLSX re-upload at commit |
| Post-commit status | `rfx_responses.status` remains `DRAFT`; product status `IN_PROGRESS` |
| Submit separation | Submit remains `POST …/carrier-response/submit` with existing gates |
| Competitor data | Zero competitor IDs, prices, rankings, scoring in workbook |
| Security first | `xlsxsecurity.InspectUpload` before Excelize open |
| Feature flag | `RFX_EXCEL_EXCHANGE_ENABLED=false` default → HTTP 404, zero writes |

---

## 3. Existing code inventory

### 3.1 Carrier questionnaire response flow (v3 — primary target)

| Concern | Path | Lines / notes |
|---|---|---|
| Routes | `services/rfx-service/internal/http/router.go` | 130–135 |
| HTTP handlers | `services/rfx-service/internal/http/handlers/carrier_response_handler.go` | 24–200 |
| Start/resume | `services/rfx-service/internal/service/carrier_response_service.go` | 112–141, 476–561 |
| Workspace load | `carrier_response_service.go` | 143–179 |
| SaveAnswers (autosave batch) | `carrier_response_service.go` | 182–300 |
| Pre-submit validate | `carrier_response_service.go` | 302–313, 564–574 |
| Submit | `carrier_response_service.go` | 315–461 |
| Summary | `carrier_response_service.go` | 463–474 |
| Response domain | `services/rfx-service/internal/domain/rfx_response.go` | 11–77 |
| Answer domain | `services/rfx-service/internal/domain/carrier_answer.go` | 10–119 |
| Answer repository | `services/rfx-service/internal/repository/answer_repository.go` | UpsertBatch, DeleteByQuestionIDs |
| Response lock/save/submit | `services/rfx-service/internal/repository/rfx_repository.go` | 892+, 1230+, 1268+ |
| Late submission coordinator | `services/rfx-service/internal/service/late_submission_service.go` | 19–218 |
| Deadline helpers | `services/rfx-service/internal/domain/late_submission.go` | 114–140 |
| Gateway RBAC (v3 carrier-response) | `services/api-gateway/internal/http/router.go` | 307–316 |
| OpenAPI operationIds | `packages/openapi/rfx-service.yaml` | 3144–4179 |
| Feature flag (excel exchange) | `services/rfx-service/internal/config/config.go`, gateway router middleware | `RFX_EXCEL_EXCHANGE_ENABLED` |
| Integration tests | `services/rfx-service/internal/integration/carrierresponse/` | See §15 |
| Browser E2E | `carrierresponse/browser_e2e_integration_test.go` | `TestRfxCarrierResponse_BrowserE2E_LiveCarrierFlow` |
| CI job | `.github/workflows/ci.yml` | `rfx-carrier-response-v3-integration`, `rfx-carrier-response-browser-e2e` |

### 3.2 Legacy commercial offer-line flow (coexists — commit must reconcile)

| Concern | Path | Lines / notes |
|---|---|---|
| PATCH commercial | `services/rfx-service/internal/http/handlers/evaluation_handler.go` | 35–60 |
| UpdateResponseCommercial | `services/rfx-service/internal/service/evaluation_service.go` | 40–94 |
| Offer line replace | `services/rfx-service/internal/repository/evaluation_repository.go` | 31–66 |
| Offer line domain | `services/rfx-service/internal/domain/rfx_offer_line.go` | 11–57 |
| Gateway PATCH route | `services/api-gateway/internal/http/router.go` | 338 |
| Own-response attach lines | `services/rfx-service/internal/service/carrier_rfx_service.go` | 46–52 |

Carrier XLSX v1 commit must atomically reconcile **both** questionnaire answers (v3) and commercial offer lines (existing table), because carriers may use either/both paths before submit.

### 3.3 Buyer XLSX reuse foundation (accepted on main)

| Component | Path |
|---|---|
| ZIP/formula security | `services/rfx-service/internal/xlsxsecurity/inspect.go` |
| Buyer workbook (reference only) | `services/rfx-service/internal/xlsxexchange/buyer_workbook.go` |
| Canonical hashing | `services/rfx-service/internal/xlsxexchange/buyer_import_hash.go` |
| Import analysis repo | `services/rfx-service/internal/repository/import_analysis_repository.go` |
| Excel exchange service | `services/rfx-service/internal/service/excel_exchange_service.go`, `excel_exchange_preview.go`, `excel_exchange_commit.go` |
| Route manifest pattern | `packages/shared-go/rfx/e7_excel_exchange_routes.go` |
| Domain enums | `services/rfx-service/internal/domain/excel_exchange.go` |
| Migration 000073 | `infrastructure/migrations/000073_rfx_excel_erp_exchange_v3_0e7_phase2.up.sql` |

---

## 4. Discovery Q&A (code-backed)

### Q1. How is the current carrier response determined?

One `rfx_responses` row per `(rfx_event_id, participant_company_id)` — unique constraint in migration `000004`. Lookup: `GetResponseByEventAndCompany` (`rfx_repository.go:646–661`). v3 workspace uses event-scoped routes with `carrier_company_id` query param; handler resolves company from JWT memberships (`domain/actor.go:65–80`).

### Q2. Can one carrier have multiple responses/versions?

**No** — one response per carrier company per event. Questionnaire version is pinned on `rfx_responses.rfx_version_id` at start (`carrier_response_service.go:514–518`, `PinResponseVersion`).

### Q3. Where is DRAFT stored and how is SUBMITTED fixed?

DB status `DRAFT` / `SUBMITTED` (`domain/rfx_response.go:12–13`). Product mapping: `DRAFT` → `IN_PROGRESS`, `SUBMITTED` → `SUBMITTED` (`carrier_answer.go:106–114`). Submit sets status + `submitted_at` via `SubmitResponse` (`rfx_repository.go:892+`).

### Q4. What can carrier edit before submit?

- Questionnaire answers via `SaveAnswers` when status is `DRAFT` (`ValidateUpdateQuestionnaireResponse` → `ValidateUpdateDraftResponse`, `rfx_response.go:72–76`)
- Commercial offer lines via `PATCH /rfx-responses/{id}` when `DRAFT` (`evaluation_service.go:63–64`)
- **Not** questionnaire structure (sections/questions/rules) — read-only from published version

### Q5. How are questionnaire answers saved?

`PATCH …/carrier-response/answers` → `SaveAnswers`: optimistic `expected_save_version`, L1 validation (invalid → 422, last valid preserved), hidden-question purge, transactional upsert + save_version bump + audit `response.answers.saved` (`carrier_response_service.go:182–300`).

### Q6. How are lot/line rates saved?

`PATCH /api/v1/rfx-responses/{response_id}` with `offer_lines[]`: full replace delete-all + insert (`evaluation_repository.go:31–66`). Validated per lot ownership, currency match, amount ≥ 0 (`rfx_offer_line.go:29–56`).

### Q7. Save vs submit validation?

| Layer | Save (answers) | Save (commercial) | Submit |
|---|---|---|---|
| Status | DRAFT only | DRAFT only | DRAFT only |
| Concurrency | `expected_save_version` | None (last-write-wins) | `expected_save_version` |
| Validation depth | L1 valid-only persistence | Line + currency checks | Full L4 pre-submit (`validateWorkspace`, `preSubmit=true`) |
| Deadline | `ValidateCarrierMutationDeadline` + late permission if past deadline | Same event gates on separate path | Same + late permission **consume** on late submit |
| Completeness | Not required | Not required on save | Required (blocking errors → 422) |

Submit: `carrier_response_service.go:315–461`. Late submit requires `Idempotency-Key` (`357–361`).

### Q8. Services reusable for atomic XLSX commit?

| Service / repo | Reuse |
|---|---|
| `TransactionRunner` | Same single-tx orchestration as buyer P4 |
| `ImportAnalysisRepository.LockImportAnalysisForUpdate` | Direct |
| `AnswerRepository.UpsertBatch` / `DeleteByQuestionIDs` | Direct (extract patch list from proposal) |
| `RfxRepository.ReplaceOfferLines` | Direct |
| `RfxRepository.LockResponseForUpdate` / `UpdateResponseAfterSave` | Adapt for post-commit save_version bump |
| `IdempotencyRepository` | New operation constant `CARRIER_XLSX_IMPORT_COMMIT` |
| Audit recorder | New event `rfx.carrier_xlsx_import.committed.v1` |
| **Do not call** | `CarrierResponseService.Submit`, `SubmitResponse`, `LateSubmissionService.ConsumePermissionWithTx` |

### Q9. Existing idempotency and audit?

| Operation | Idempotency | Audit |
|---|---|---|
| SaveAnswers | Optimistic save_version only | `response.answers.saved` |
| Submit (late only) | `Idempotency-Key` + 24h store | `response.submitted` |
| Buyer XLSX commit | `Idempotency-Key` required | `rfx.buyer_xlsx_import.committed.v1` |
| Commercial PATCH | None | None |
| Carrier XLSX commit (proposed) | **Required** `Idempotency-Key` | `rfx.carrier_xlsx_import.committed.v1` |

### Q10. What prevents repeat or late submit?

- Status gate: only `DRAFT` can submit (`ValidateSubmitRfxResponse`, `rfx_response.go:65–69`)
- Deadline: `ValidateSubmissionBeforeDeadline` unless approved late permission consumed at submit (`carrier_response_service.go:354–407`)
- Optimistic lock on submit via save_version check
- Post-submit: `ValidateUpdateQuestionnaireResponse` rejects edits (`carrier_response_integration_test.go:97+`)

**Carrier XLSX commit must not bypass any of these** — it never invokes submit.

---

## 5. Carrier workbook V1 schema — `BINTRANS_RFX_CARRIER_XLSX_V1`

Distinct from `BINTRANS_RFX_BUYER_XLSX_V1`. Buyer workbook edits tender structure; carrier workbook edits **answers + own offer lines** against a **published, pinned questionnaire**.

### 5.1 Sheet order (fixed)

```
Instructions → Metadata → Lots → Questions → Options → Rules → Answers → OfferLines
```

**Rationale for 8 sheets (not 6):** Task minimum lists six structural sheets for schema verification. Carrier v1 adds two **editable** data sheets (`Answers`, `OfferLines`) because mutable domain differs from buyer (no graph/lot CRUD). `Sections` omitted — `section_code` column on `Questions` matches buyer question keys without a separate structural edit surface.

### 5.2 Instructions

Static tri-locale rows (RU/EN/ZH). Embeds schema name/version. States: export is own-response snapshot; import does not submit; no formulas/macros/external links.

| Column | Type | Required | Editable | Classification |
|---|---|---|---|---|
| A (RU), B (EN), C (ZH) | string | — | NO | PUBLIC |

### 5.3 Metadata (key/value; col A = key, col B = value; no header row)

| Key | Type | Required | Stable key | Editable | Domain mapping | Validation |
|---|---|---|---|---|---|---|
| `schema_name` | string | YES | schema_name | NO | constant `BINTRANS_RFX_CARRIER_XLSX_V1` | Must match; mismatch → 422 |
| `schema_version` | string | YES | schema_version | NO | constant `"1"` | Must be `1` |
| `exported_at_utc` | RFC3339 | YES | — | NO | export timestamp | Excluded from canonical hash |
| `tenant_id` | UUID string | YES | — | NO | tenant | **VERIFY_ONLY**; server re-checks JWT |
| `rfx_event_id` | UUID string | YES | — | NO | event | Must match URL path |
| `rfx_response_id` | UUID string | YES | — | NO | response | Must match URL path |
| `carrier_company_id` | UUID string | YES | — | NO | participant | Must match actor company |
| `rfx_version_id` | UUID string | YES | — | NO | pinned questionnaire | Must match response pin |
| `questionnaire_version_number` | int string | YES | — | NO | informational | Verify against server |
| `response_save_version` | int64 string | YES | response_save_version | NO | optimistic baseline | Stale → 409 |
| `response_status` | string | YES | — | NO | must be `DRAFT` at export | `SUBMITTED` blocks import |
| `event_row_version` | int string | YES | — | NO | event.version | Stale event → 409 |
| `available_lots_fingerprint` | hex64 | YES | lots_fingerprint | NO | server lot set at export | Stale lots → 409 |
| `answers_fingerprint` | hex64 | YES | answers_fingerprint | NO | server answer set at export | Stale answers → 409 |
| `offer_lines_fingerprint` | hex64 | YES | offer_lines_fingerprint | NO | server offer lines at export | Stale offers → 409 |
| `export_mode` | string | YES | — | NO | `DRAFT_EDIT` or `SUBMITTED_READONLY` | Commit requires `DRAFT_EDIT` |

**Confidentiality:** Metadata must **not** include competitor company IDs, response IDs, participant lists, rankings, scores, or buyer internal notes.

### 5.4 Lots (header row 1 — verify-only)

Headers: `lot_number`, `name`, `description`, `category`, `currency_code`, `status`

| Column | Type | Required | Stable key | Editable | Notes |
|---|---|---|---|---|---|
| `lot_number` | int/string | YES | lot_number | NO | Maps to `rfx_lots` scoped to event |
| `name` | string | YES | — | NO | Public tender data |
| `description` | string | NO | — | NO | |
| `category` | string | NO | — | NO | |
| `currency_code` | string | YES | — | NO | Event currency |
| `status` | string | YES | — | NO | Active lots only exported |

**Excluded:** `estimated_value` (buyer-only), competitor bid columns, internal buyer notes.

### 5.5 Questions (header row 1 — verify-only)

Headers: `section_code`, `question_code`, `question_type`, `title_ru`, `title_en`, `title_zh`, `required`, `sort_order`, `validation_json`

Stable keys: `section_code`, `question_code`. Server resolves to `question_id` UUIDs at preview (never exported to carrier workbook).

### 5.6 Options (verify-only)

Headers: `question_code`, `option_code`, `label_ru`, `label_en`, `label_zh`, `sort_order`

### 5.7 Rules (verify-only)

Headers: `rule_code`, `source_question_code`, `condition`, `target_question_code`, `action`, `sort_order`

### 5.8 Answers (editable)

Headers: `question_code`, `answer_value`

| Column | Type | Required | Stable key | Editable | Validation |
|---|---|---|---|---|---|
| `question_code` | string | YES | question_code | KEY | Must exist in Questions sheet |
| `answer_value` | string | NO* | — | YES | Parsed per `question_type`; L1 rules at preview |

*Required when question `required=true` and visible per rules (evaluated at preview/commit revalidation).

Empty row removes answer (explicit delete). Hidden questions: rows ignored + purged on commit (same as `SaveAnswers` hidden policy).

**Forbidden columns (fail-closed before generic header parse):** `competitor_*`, `other_carrier_*`, `rank`, `score`, `benchmark_*`, `participant_id`, `response_id` (other), `*_company_id` (except metadata).

### 5.9 OfferLines (editable)

Headers: `lot_number`, `amount`, `currency_code`, `comment`

| Column | Type | Required | Stable key | Editable | Validation |
|---|---|---|---|---|---|
| `lot_number` | int/string | YES* | lot_number | KEY | Maps to lot; required when event has lots |
| `amount` | decimal string | YES | — | YES | ≥ 0; text-only cell |
| `currency_code` | string | YES | — | YES | Must match event currency |
| `comment` | string | NO | — | YES | Optional |

### 5.10 Zero-lot offer line policy (frozen)

Code evidence:

| Evidence | Location |
|---|---|
| `RfxLotID` not required when `lotCount == 0` | `domain/rfx_offer_line.go:29–31` (`ValidateOfferLineInput`) |
| Event-level line uses `RfxLotID == uuid.Nil` | `repository/evaluation_repository.go:47–50` |
| At most one NULL-lot line per response | `000038_rfx_evaluation_award_v1.3.up.sql:18–20` (`uq_rfx_response_offer_line_event`) |
| Completeness: one Nil-lot line when `lotCount == 0` | `domain/rfx_response_commercial.go:28–34` (`ResponseOfferComplete`) |
| Commercial PATCH without lot ID | `integration/enterprise/evaluation_award_integration_test.go:188` (`UpsertOfferLineInput{{Amount, CurrencyCode}}`) |
| Lot required when `lotCount > 0` | `domain/rfx_offer_line.go:30–31`; `evaluation_service.go:74–84` |

**Policy A — event has lots (`lotCount > 0`):**

| Rule | Contract |
|---|---|
| OfferLines rows | One row per covered lot; `lot_number` **required** |
| Stable key | `lot_number` → server maps to `rfx_lot_id` |
| Empty `lot_number` | **422** `rfx_lot_id_required` (mirror `ValidateOfferLineInput`) |
| Unknown `lot_number` | **422** / **404** `rfx_lot_not_found` |
| Duplicate `lot_number` in workbook | **422** `duplicate_lot_number` |
| Export | One row per event lot (Lots sheet); no event-level row |
| Fingerprint | Sorted map `lot_id → {amount,currency,comment}` |

**Policy B — event has zero lots (`lotCount == 0`):**

| Rule | Contract |
|---|---|
| Event-level offer | **Allowed** — exactly **one** OfferLines row maximum |
| Stable key | `lot_number` = **empty string** (canonical event-level key) |
| `lot_number` non-empty | **422** `unknown_lot` (event has no lots) |
| Second OfferLines row | **422** `duplicate_event_level_offer` (DB unique index) |
| Empty OfferLines sheet | **Valid** — no commercial offer yet (answers-only DRAFT) |
| Export | If one event-level line exists: single row with empty `lot_number`; else header-only sheet |
| Commit mapping | `UpsertOfferLineInput{RfxLotID: uuid.Nil, Amount, CurrencyCode, Comment}` |
| Fingerprint | Single entry keyed `__EVENT_LEVEL__` → `{amount,currency,comment}` |

```
ZERO_LOT_POLICY=FROZEN_EVENT_LEVEL_SINGLE_LINE
ZERO_LOT_LOT_NUMBER_KEY=EMPTY_STRING
ZERO_LOT_MAX_OFFER_ROWS=1
ZERO_LOT_EMPTY_SHEET_ALLOWED=YES
```

---

## 6. Export contract

### 6.1 Route (proposed)

```
GET /api/v1/rfx-events/{event_id}/carrier-responses/{response_id}/xlsx-export
```

| Aspect | Decision |
|---|---|
| RBAC | `PolicyCarrierRead` (owner) |
| Ownership | Response must belong to `{event_id}`; actor company must equal `participant_company_id` |
| Tenant | JWT tenant scope; cross-tenant → 404 |
| Allowed statuses | **DRAFT:** export with `export_mode=DRAFT_EDIT`. **SUBMITTED:** export with `export_mode=SUBMITTED_READONLY` (recommended) |
| Questionnaire binding | Export pins sheets from `response.rfx_version_id` published questionnaire |
| Response binding | URL `{response_id}` must match resolved own response |
| Filename | `BINTRANS_RFX_CARRIER_{event_id_short}_{response_id_short}_V1.xlsx` |
| MIME | `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` |
| Feature flag | `RFX_EXCEL_EXCHANGE_ENABLED`; false → 404 |
| Audit | Optional read audit `rfx.carrier_xlsx_export.generated.v1` (no mutation) |
| DB writes | **None** |
| Formula protection | All cells `SetCellStr` + text format `NumFmt 49` (buyer pattern: `buyer_workbook.go:133–141`); see §9.4 |
| Determinism | Stable sheet order, sorted rows by stable keys, canonical metadata minus `exported_at_utc` |

### 6.2 Export after SUBMITTED — recommended policy

| Operation | DRAFT response | SUBMITTED response |
|---|---|---|
| Export | YES (`DRAFT_EDIT`) | YES read-only (`SUBMITTED_READONLY`) — **recommended** |
| Preview | YES | **NO** → 409 `response_not_editable` |
| Commit | YES (stays DRAFT) | **NO** → 409 |

**Rationale:** Carriers need an archival/print copy of submitted offers; re-import would violate immutability and audit trail. Read-only export includes banner in Instructions sheet stating commit is disabled.

---

## 7. Import Preview contract

### 7.1 Route (proposed)

```
POST /api/v1/rfx-events/{event_id}/carrier-responses/{response_id}/xlsx-import/preview
```

Multipart field: `file`

### 7.2 Pipeline

1. Gateway JWT identity + body size bound
2. RBAC `PolicyCarrierRespond`; carrier company ownership of `{response_id}`
3. Feature flag check → 404 if disabled
4. `xlsxsecurity.InspectUpload` → 400/413 on failure
5. Excelize open (in-memory only)
6. Sheet presence/order/schema validation
7. Metadata parse + **server-side** identity/baseline verification (ignore client trust)
8. Competitor column sentinel scan (fail-closed)
9. Parse/normalize Answers + OfferLines
10. Domain validation: `ValidateCarrierAnswerPatches` (L1) + offer line validators
11. Diff vs server baseline (answers, offer lines, hidden questions)
12. Canonical JSON proposal + SHA-256 hash
13. If valid: persist `rfx_import_analyses` row (OPTION A — same as buyer P3)

### 7.3 HTTP semantics

| Code | Condition |
|---|---|
| 200 | Valid preview; optional `analysis_id` when persistence enabled |
| 400 | Malformed multipart / unreadable XLSX |
| 401 | Unauthenticated |
| 403 | Wrong carrier company / actor binding |
| 404 | Feature off; cross-tenant; response not found |
| 409 | Stale baseline; response SUBMITTED; event not open for mutation |
| 413 | Upload too large |
| 422 | Validation issues (structured issue list) |

| Policy | Value |
|---|---|
| Max issues | 2000 (match buyer preview) |
| Issue ordering | Deterministic: sheet, row, column, code |
| TTL | 24 hours |
| Binary persistence | NO |
| Response writes | NO |
| Answer/offer writes | NO |

### 7.4 `rfx_import_analyses` reuse (migration 000073)

**Verdict: `MIGRATION_000073_SUFFICIENT=YES`**

SQL proof — constraints already allow carrier:

```sql
-- infrastructure/migrations/000073_rfx_excel_erp_exchange_v3_0e7_phase2.up.sql:25-32
workbook_type IN ('BUYER_TENDER', 'CARRIER_OFFER')
schema_version IN ('BINTRANS_RFX_BUYER_XLSX_V1', 'BINTRANS_RFX_CARRIER_XLSX_V1')
target_type IN ('NEW_EVENT', 'DRAFT_EVENT', 'CARRIER_RESPONSE')
```

Carrier preview/commit persistence:

| Field | Carrier value |
|---|---|
| `workbook_type` | `CARRIER_OFFER` |
| `schema_version` | `BINTRANS_RFX_CARRIER_XLSX_V1` |
| `target_type` | `CARRIER_RESPONSE` |
| `target_id` | `{response_id}` |
| `target_version` | `{response_save_version}` at preview time |
| `actor_id` | Preview creator user |
| `actor_company_id` | Carrier company |
| `canonical_payload_json` | Normalized proposal (answers + offer lines + baselines) |
| `canonical_hash` | SHA-256 of stable JSON |

Domain mirrors SQL: `domain/excel_exchange.go:15–27`.

**No migration 000074 required** for enum extension. Application code must add carrier payload hashing (parallel to `StableStoredPayload` for buyer).

---

## 8. Import Commit contract

### 8.1 Route (proposed)

```
POST /api/v1/rfx-events/{event_id}/carrier-responses/{response_id}/xlsx-import/commit
```

Request body minimum:

```json
{ "analysis_id": "<uuid>" }
```

Header: **`Idempotency-Key`** (required)

### 8.2 Commit orchestration (single transaction)

1. Pre-tx idempotency replay check (`CARRIER_XLSX_IMPORT_COMMIT`, scope = `response_id`)
2. `LockImportAnalysisForUpdate(analysis_id)` (`FOR UPDATE`)
3. Verify: not expired, not consumed, actor/company match, `target_type=CARRIER_RESPONSE`, `target_id=response_id`
4. Verify: `workbook_type=CARRIER_OFFER`, schema version match
5. Re-check response status **`DRAFT`** → else 409
6. `LockResponseForUpdate(response_id)`
7. Stale checks: `save_version`, `rfx_version_id`, fingerprints (answers, offer lines, lots), `event_row_version`
8. `VerifyStoredCanonicalPayloadHash`
9. Domain revalidation (L1 answers + offer lines + hidden question purge)
10. Deadline check: `ValidateCarrierMutationDeadline` (late permission required if past deadline — same as SaveAnswers; **does not consume** permission)
11. Apply answer upserts/deletes (mirror SaveAnswers logic)
12. Apply offer line replace (mirror ReplaceOfferLines)
13. `UpdateResponseAfterSave` (bump save_version, completion_percent)
14. Audit `rfx.carrier_xlsx_import.committed.v1`
15. `MarkConsumed` on analysis
16. Store idempotency record

**Explicitly not in transaction:** `SubmitResponse`, scoring trigger, late permission consume, participant status → `RESPONSE_SUBMITTED`.

### 8.3 DRAFT-only / no-auto-submit proof

| Check | Post-commit state |
|---|---|
| `rfx_responses.status` | Remains `DRAFT` |
| `submitted_at` | NULL |
| Submit endpoint | Not invoked |
| Audit | No `response.submitted` event |
| Late submission permission | Unchanged (not consumed) |
| Scoring | Not triggered |
| Product status | `IN_PROGRESS` |

If response already `SUBMITTED`: preview/commit → **409** before mutation.

### 8.4 SUBMITTED response behavior

| Operation | Policy |
|---|---|
| Export | Read-only allowed (§6.2) |
| Preview | 409 |
| Commit | 409 |

---

## 9. Security and confidentiality

### 9.1 Workbook content rules

Include only: own response context, carrier-visible lots, published questionnaire, own answers/prices, public buyer instructions.

**Categorically exclude:** competitor lists, foreign carrier/response/participant IDs, foreign prices, rankings, traffic-light classification, scoring, award recommendations, buyer internal notes, benchmark data inferring other bids.

### 9.2 Leakage channels — fail-closed policy

| Channel | Policy |
|---|---|
| Hidden sheets | Reject at `xlsxsecurity` / sheet allowlist |
| Hidden rows/columns | Parser reads visible cells only; structural validation |
| Shared strings | No competitor tokens in export generator; import sentinel scan |
| Defined names | Reject external/ref names (buyer pattern) |
| Comments / drawings | Strip on export; reject on import if present |
| External links | Reject (inspect.go) |
| Formulas | Reject formula XML/cells; allow formula-like literal text as text (§9.4) |
| Pivot/cache/custom XML | Reject non-allowlisted ZIP entries |
| Document properties | Do not embed competitor data |
| ZIP relationships | Allowlist OOXML parts only |

### 9.4 Formula and formula-like text policy (frozen)

```
FORMULA_XML_POLICY=REJECT
FORMULA_CELL_POLICY=REJECT
FORMULA_LIKE_TEXT_POLICY=ALLOW_AS_LITERAL_TEXT
FORMULA_LIKE_TEXT_STORAGE=SetCellStr + text format NumFmt 49
ORIGINAL_TEXT_SEMANTICS_PRESERVED=YES
```

| Layer | Policy |
|---|---|
| ZIP/XML inspection | Reject VBA, external links, formula XML (`xlsxsecurity/inspect.go`) |
| Excelize cell type | Reject cells with formula type / `<f>` elements |
| Literal text | Values starting with `=`, `+`, `-`, `@`, or whitespace+prefix **allowed** when stored as plain text (`NumFmt 49`) |
| Export | Write all answer/offer values via `SetCellStr`; never emit formula cells |
| Import | Accept formula-like strings only from text cells; no Excel evaluation |
| INT-119 | Formula cell rejection on import |

Buyer reference: `buyer_workbook.go:133–141` (`NumFmt: 49`, `SetCellStr`).

### 9.5 Future test sentinels (INT-79, INT-86, INT-117–119)

- Export snapshot must not contain regex `\bcompetitor_|\bother_carrier_|\brank\b|\bscore\b`
- Import with forbidden column headers → 422 `competitor_column_forbidden` before generic header errors
- Golden-file ZIP entry inventory tests

---

## 10. Baseline, stale detection, and concurrency

### 10.1 Carrier baseline proposal (stored in canonical payload)

| Field | Purpose |
|---|---|
| `response_id` | UUID binding |
| `response_save_version` | Primary optimistic stale detection |
| `rfx_event_id` | Event binding |
| `rfx_version_id` | Questionnaire pin |
| `carrier_company_id` | Ownership |
| `event_row_version` | Event cancelled/superseded detection |
| `answers_fingerprint` | Content hash of visible answer map |
| `offer_lines_fingerprint` | Content hash of lot→amount/currency/comment |
| `available_lots_fingerprint` | Server lot set at preview time |
| `response_status` | Must be `DRAFT` |

**Row versions:** Response uses `save_version` (int64) for autosave/commit concurrency (`rfx_repository.go:1268+`). Per-answer `version` exists but commit stale gate prioritizes **`save_version` + content fingerprints** (P4 buyer lots pattern — lot-only edits may not bump all row versions).

### 10.2 Content-equivalent ABA policy

```
ABA_POLICY=CONTENT_EQUIVALENT_STATE_ACCEPTED_V1
```

If UI save produces identical canonical answer/offer fingerprint as preview baseline, commit accepts same `save_version` replay (no false stale). Non-equivalent concurrent UI edit → fingerprint mismatch → 409.

### 10.3 Race scenarios

| Race | Policy |
|---|---|
| Concurrent UI SaveAnswers vs XLSX commit | Serialize on `LockResponseForUpdate`; loser gets 409 stale |
| Concurrent preview/commit | Analysis single-use consume + response lock |
| Submit between preview and commit | Commit re-checks `status=DRAFT` after lock → 409 |
| Event deadline during commit | `ValidateCarrierMutationDeadline`; late permission required but **not consumed** |
| Event cancellation / version republish | `event_row_version` + `rfx_version_id` mismatch → 409 |
| Questionnaire republish mid-flight | Pinned `rfx_version_id` on response prevents silent drift |

---

## 11. Persistence and migration decision

| Decision | Value |
|---|---|
| `MIGRATION_000073_SUFFICIENT` | **YES** |
| `MIGRATION_000074_REQUIRED` | **NO** |
| `MIGRATION_000074_CREATED` | **NO** |

Schema delta for 000074: **none required** — carrier enums pre-provisioned in 000073 CHECK constraints.

If future controller ever required 000074, candidates would be: none identified at discovery; optional index on `(tenant_id, target_type, target_id, status)` already exists.

### 11.1 Carrier canonical hash implementation gate (frozen)

Existing `StableStoredPayload` / `VerifyStoredCanonicalPayloadHash` (`xlsxexchange/buyer_import_hash.go`) supports **buyer** payloads today. `CreatePreview` re-hashes only when `workbook_type == BUYER_TENDER` (`import_analysis_repository.go:48–58`).

Carrier implementation **must** add a parallel path:

| Gate | Requirement |
|---|---|
| Carrier payload struct | Canonical JSON for answers + offer lines + baselines |
| `CARRIER_STABLE_STORED_PAYLOAD` | **REQUIRED** — carrier-specific stable marshal + SHA-256 |
| `REPOSITORY_HASH_DISPATCH_BY_WORKBOOK_TYPE` | **REQUIRED** — `CreatePreview` selects verifier by `workbook_type` + `schema_version` |
| Unknown workbook/schema | **FAIL_CLOSED** — validation error; no analysis row |
| Preview persistence | No insert on hash mismatch |
| Commit | Re-verify carrier hash after JSONB round-trip (`VerifyStoredCanonicalPayloadHash`) |
| `CARRIER_JSONB_ROUNDTRIP_HASH_TEST` | **REQUIRED** — integration test before C3 sign-off |

```
CARRIER_STABLE_STORED_PAYLOAD=REQUIRED
REPOSITORY_HASH_DISPATCH_BY_WORKBOOK_TYPE=REQUIRED
UNKNOWN_WORKBOOK_HASH_PATH=FAIL_CLOSED
CARRIER_JSONB_ROUNDTRIP_HASH_TEST=REQUIRED
```

Migration 000074 is **not** required for hash dispatch.

---

## 12. Transactions, locks, idempotency, and audit

| Mechanism | Carrier XLSX usage |
|---|---|
| `TransactionRunner.Run` | Single tx for commit |
| `LockImportAnalysisForUpdate` | Analysis row |
| `LockResponseForUpdate` | Response row |
| Idempotency | Required; scope `(tenant, actor, CARRIER_XLSX_IMPORT_COMMIT, response_id)` |
| Audit event | `rfx.carrier_xlsx_import.committed.v1` with change counts |
| Rollback | Any answer/offer/audit/idempotency/analysis failure → full rollback |
| Preview | No tx mutation of response data |

---

## 13. HTTP / OpenAPI proposal

| Operation | Method | Path | operationId (proposed) | RBAC | Idempotency |
|---|---|---|---|---|---|
| Export | GET | `/api/v1/rfx-events/{event_id}/carrier-responses/{response_id}/xlsx-export` | `get_export_carrier_rfx_response_as_xlsx_workbook` | PolicyCarrierRead | No |
| Preview | POST | `/api/v1/rfx-events/{event_id}/carrier-responses/{response_id}/xlsx-import/preview` | `post_preview_carrier_rfx_response_xlsx_import` | PolicyCarrierRespond | No |
| Commit | POST | `/api/v1/rfx-events/{event_id}/carrier-responses/{response_id}/xlsx-import/commit` | `post_commit_carrier_rfx_response_xlsx_import` | PolicyCarrierRespond | **Yes** |

Extend `packages/shared-go/rfx/e7_excel_exchange_routes.go` with carrier routes (parity anchor). Same `RFX_EXCEL_EXCHANGE_ENABLED` flag as buyer.

**Note:** Existing v3 carrier workspace uses event-scoped `/carrier-response/*` without `response_id` in path. New routes intentionally bind `{response_id}` for explicit ownership and alignment with REST resource model.

---

## 14. Reuse matrix

| Capability | Buyer implementation | Carrier reuse | Required adaptation |
|---|---|---|---|
| `xlsxsecurity.InspectUpload` | `xlsxsecurity/inspect.go` | **Direct** | Allowlist carrier sheet names |
| Issue collector | buyer import preview | **Pattern** | Carrier codes + competitor sentinel |
| Formula protection | `buyer_workbook.go` SetCellStr | **Pattern** | Carrier generator |
| Workbook helpers | `buyer_workbook.go` | **No copy** | New `carrier_workbook.go` |
| Canonical hashing | `buyer_import_hash.go` | **Pattern** | Carrier payload struct |
| Import analyses repo | `import_analysis_repository.go` | **Direct** | Carrier enum values on insert |
| Locking | `LockImportAnalysisForUpdate`, buyer event locks | **Partial** | Use `LockResponseForUpdate` instead of draft locks |
| Idempotency | buyer commit operation | **Pattern** | `CARRIER_XLSX_IMPORT_COMMIT` |
| Audit | buyer commit audit | **Pattern** | New action name |
| HTTP multipart | `buyer_xlsx_multipart.go` | **Direct** | Same size limits |
| Feature flag middleware | gateway + rfx router | **Direct** | Register new routes |
| Route manifest | `e7_excel_exchange_routes.go` | **Extend** | Add 3 carrier routes |
| OpenAPI generator | buyer xlsx ops | **Extend** | Carrier ops + schemas |
| Transaction runner | buyer P4 commit | **Direct** | Carrier orchestration |
| Questionnaire validation | `ValidateCarrierAnswerPatches` | **Direct** | Wire from proposal |
| Offer line validation | `ValidateOfferLineInput` | **Direct** | Wire from proposal |
| Graph reconciliation | buyer lot/graph reconcile | **Not used** | Carrier has no graph mutation |
| Answer reconciliation | SaveAnswers upsert/delete | **Extract** | Batch from proposal |
| Offer reconciliation | ReplaceOfferLines | **Direct** | From proposal |
| Submit / scoring | N/A buyer | **Must NOT call** | Hard guard in commit service |
| Late submission consume | submit path only | **Must NOT call** | Commit only checks mutation deadline |

---

## 15. Test matrix E7P2-INT-71..119

Continue after buyer INT-70.

```
TEST_MATRIX=E7P2-INT-71..119
TEST_CASE_COUNT=49
TEST_IDS_COMPLETE=YES
TEST_IDS_UNIQUE=YES
MATRIX_GUARD=EXTRA_NO_INT_ID
MATRIX_GUARD_NAME=TestE7P2CarrierXlsxMatrixIDsCompleteAndUnique
NEXT_TEST_ID_AFTER_CARRIER_XLSX=E7P2-INT-120
```

The matrix guard is an **EXTRA** meta-test (no INT ID). It verifies INT-71..119 are registered exactly once. **INT-120** remains free for the first business test after Carrier XLSX completes.

| ID | Scenario | Layer |
|---|---|---|
| INT-71 | Export success for DRAFT response; deterministic bytes | PG+HTTP |
| INT-72 | Export RBAC: owner carrier allowed | HTTP |
| INT-73 | Export cross-tenant → 404 | HTTP |
| INT-74 | Export competitor cannot access foreign response → 403/404 | HTTP |
| INT-75 | Feature disabled → 404, no writes | HTTP |
| INT-76 | Export produces no DB writes | PG |
| INT-77 | Export SUBMITTED read-only (`export_mode=SUBMITTED_READONLY`) | HTTP |
| INT-78 | Export semantic determinism (repeat export stable hash) | Unit+HTTP |
| INT-79 | Export confidentiality sentinel: no forbidden columns/strings | Static+HTTP |
| INT-80 | Preview valid workbook → 200 + analysis persisted | PG+HTTP |
| INT-81 | Preview invalid validation → 422, no analysis row | PG |
| INT-82 | Malformed XLSX → 400 | HTTP |
| INT-83 | Oversized upload → 413 | HTTP |
| INT-84 | Security package rejects vba/externalLinks | HTTP |
| INT-85 | Hidden sheet / unknown sheet → 422 | HTTP |
| INT-86 | Competitor column header → 422 fail-closed | HTTP |
| INT-87 | Metadata schema mismatch → 422 | HTTP |
| INT-88 | Stale `response_save_version` baseline → 409 | HTTP |
| INT-89 | Preview does not write answers | PG |
| INT-90 | Preview does not write offer lines | PG |
| INT-91 | Commit success → 200; status remains DRAFT | PG+HTTP |
| INT-92 | Commit reconciles answers vs proposal | PG |
| INT-93 | Commit reconciles offer lines vs proposal | PG |
| INT-94 | Removed answer rows deleted | PG |
| INT-95 | Removed offer lines deleted | PG |
| INT-96 | Stale response save_version at commit → 409 | HTTP |
| INT-97 | Stale questionnaire version → 409 | HTTP |
| INT-98 | Stale lots fingerprint → 409 | HTTP |
| INT-99 | Expired analysis → 409, no consume | PG |
| INT-100 | Wrong actor → 403 | HTTP |
| INT-101 | Wrong carrier company → 403 | HTTP |
| INT-102 | Canonical hash tampering → 422 | HTTP |
| INT-103 | Same Idempotency-Key replay → 200, no duplicate writes | PG |
| INT-104 | Same key, different analysis_id → 409 | HTTP |
| INT-105 | Concurrent commits → one mutation | PG |
| INT-106 | Mid-answer failure → rollback, no partial answers | PG |
| INT-107 | Mid-offer-line failure → rollback | PG |
| INT-108 | Submit race: submit completes before commit → commit 409 | PG |
| INT-109 | Deadline race: commit without late permission after deadline → 409/422 | HTTP |
| INT-110 | Post-commit: no `submitted_at`, status DRAFT | PG |
| INT-111 | Post-commit: no `response.submitted` audit | PG |
| INT-112 | Late submission permission not consumed on commit | PG |
| INT-113 | SUBMITTED response blocks preview/commit → 409 | HTTP |
| INT-114 | Route/gateway/OpenAPI parity (3 routes) | Static |
| INT-115 | Buyer regression smoke: INT-06..42 unchanged | CI matrix |
| INT-116 | Carrier SaveAnswers direct path regression | PG |
| INT-117 | Carrier Submit direct path regression (separate from XLSX) | PG |
| INT-118 | Confidentiality: defined names / comments rejected on import | HTTP |
| INT-119 | Formula cell rejection on import | HTTP |

**Matrix guard (EXTRA, not counted in 49):** `TestE7P2CarrierXlsxMatrixIDsCompleteAndUnique` — verifies INT-71..119 registered exactly once.

---

## 16. Implementation sequence (post-controller authorization)

1. **C1 — Export:** `carrier_workbook.go`, export service method, GET handler, OpenAPI, INT-71..79
2. **C2 — Preview parser:** `carrier_import_parser.go`, preview service, POST preview, INT-80..90
3. **C3 — Analysis persistence:** carrier payload hash, repo integration, TTL
4. **C4 — Commit orchestration:** `CommitCarrierImportAnalysis`, DRAFT-only guards, INT-91..112
5. **C5 — Routes/gateway/OpenAPI parity:** manifest + INT-113..114
6. **C6 — Regression gates:** INT-115..119 + matrix guard

Each stage requires controller authorization before product code (same pattern as buyer P2→P4).

---

## 17. Risks and blockers

| Risk | Severity | Mitigation |
|---|---|---|
| Dual persistence paths (SaveAnswers vs commercial PATCH) | HIGH | Commit reconciles both; regression INT-116 |
| Auto-submit accidental coupling | BLOCKER | Explicit non-call list; INT-110..112 |
| Competitor data leakage via export | BLOCKER | Sentinel tests INT-79, INT-86, INT-118 |
| Gateway RBAC gaps on legacy carrier GET routes | MEDIUM | Carrier XLSX routes use explicit `PolicyCarrierRead/Respond` |
| `save_version` vs fingerprint stale gaps | MEDIUM | Dual baseline (version + fingerprints) per P4 lesson |
| SUBMITTED export misuse | MEDIUM | `export_mode` + commit refuses non-DRAFT |
| Offer line replace wipes concurrent UI commercial edit | MEDIUM | Same as existing PATCH semantics; document in UX |

**Blockers for implementation tranche C1:** none — C1 export accepted and merged (PR #132).

---

## 18. Controller decisions (accepted)

| # | Decision | Status |
|---|---|---|
| CD-1 | Approve `BINTRANS_RFX_CARRIER_XLSX_V1` 8-sheet schema | **ACCEPTED** |
| CD-2 | Approve response-scoped routes with `{response_id}` | **ACCEPTED** |
| CD-3 | `MIGRATION_000073_SUFFICIENT=YES` | **ACCEPTED** |
| CD-4 | SUBMITTED export read-only allowed; preview/commit forbidden | **ACCEPTED** |
| CD-5 | `CARRIER_XLSX_COMMIT_AUTO_SUBMIT=NO` absolute | **ACCEPTED** |
| CD-6 | Reuse `RFX_EXCEL_EXCHANGE_ENABLED` (no new flag) | **ACCEPTED** |
| CD-7 | Test range INT-71..119 + EXTRA matrix guard | **ACCEPTED** |
| CD-8 | Authorize implementation tranche C1 (export first) | **ACCEPTED** (PR #132 merged) |

---

## 19. Controller architecture review record

Independent architecture review completed on discovery commit `85b5157842bb24e1078a26c4884c4b708c3f8b9c`.

```
REVIEWED_HEAD=85b5157842bb24e1078a26c4884c4b708c3f8b9c
REVIEWED_BASE=87916ab2bfd83ec15605446c02f2cd0c0ee6c79d
CONTROLLER_VERDICT=GO
ARCHITECTURE_FREEZE=ACCEPTED
CARRIER_XLSX_IMPLEMENTATION_AUTHORIZED=YES
BLOCKER_FINDINGS=0
HIGH_FINDINGS=0
MEDIUM_FINDINGS=2_CLOSED_IN_PUBLICATION_COMMIT
LOW_FINDINGS=2_CLOSED_IN_PUBLICATION_COMMIT
```

Publication commit closes:

| ID | Finding | Remediation |
|---|---|---|
| MEDIUM-01 | INT-120 guard ambiguity | §15: `MATRIX_GUARD=EXTRA_NO_INT_ID`; INT-120 reserved for next business test |
| MEDIUM-02 | Zero-lot TBD | §5.10: frozen event-level single-line policy with code evidence |
| LOW-01 | `FORMULA_POLICY=REJECT_ALL` ambiguous | §9.4: split XML/cell vs literal text policies |
| LOW-02 | Carrier hash path implicit | §11.1: `REPOSITORY_HASH_DISPATCH_BY_WORKBOOK_TYPE=REQUIRED` |

---

## 20. Final markers

```
STATUS=IMPLEMENTATION_IN_PROGRESS
ARCHITECTURE_REVIEW_RESULT=GO
ARCHITECTURE_FREEZE=IMPLEMENTED_ACCEPTED
CARRIER_XLSX_IMPLEMENTATION_AUTHORIZED=YES
CARRIER_XLSX_IMPLEMENTATION_STARTED=YES
CARRIER_XLSX_C1_EXPORT_STATUS=IMPLEMENTED_ACCEPTED
CARRIER_XLSX_C2_PREVIEW_STATUS=IMPLEMENTED_ACCEPTED
CARRIER_XLSX_C3_COMMIT_STATUS=NOT_STARTED
CARRIER_XLSX_OVERALL_STATUS=IMPLEMENTATION_IN_PROGRESS
C1_EXPORT_TEST_IDS=E7P2-INT-71..79
CARRIER_XLSX_SCHEMA=BINTRANS_RFX_CARRIER_XLSX_V1
CARRIER_XLSX_EXPORT_PROPOSED=YES
CARRIER_XLSX_PREVIEW_PROPOSED=YES
CARRIER_XLSX_COMMIT_PROPOSED=YES
CARRIER_XLSX_COMMIT_RESULT=DRAFT_ONLY
CARRIER_XLSX_COMMIT_AUTO_SUBMIT=NO
DIRECT_SUBMIT_ENDPOINT_UNCHANGED=YES
LATE_SUBMISSION_FLOW_UNCHANGED=YES
COMPETITOR_CONFIDENTIALITY=MANDATORY_FAIL_CLOSED
COMPETITOR_COLUMNS_REJECTED=YES
COMPETITOR_SENTINELS_REQUIRED=YES
HIDDEN_CONTENT_POLICY=FAIL_CLOSED
FORMULA_XML_POLICY=REJECT
FORMULA_CELL_POLICY=REJECT
FORMULA_LIKE_TEXT_POLICY=ALLOW_AS_LITERAL_TEXT
FORMULA_LIKE_TEXT_STORAGE=SetCellStr + text format NumFmt 49
ORIGINAL_TEXT_SEMANTICS_PRESERVED=YES
SECURITY_BEFORE_EXCELIZE=YES
ZERO_LOT_POLICY=FROZEN_EVENT_LEVEL_SINGLE_LINE
CARRIER_STABLE_STORED_PAYLOAD=REQUIRED
REPOSITORY_HASH_DISPATCH_BY_WORKBOOK_TYPE=REQUIRED
UNKNOWN_WORKBOOK_HASH_PATH=FAIL_CLOSED
CARRIER_JSONB_ROUNDTRIP_HASH_TEST=REQUIRED
PREVIEW_EVENT_WRITES=NO
PREVIEW_RESPONSE_WRITES=NO
PREVIEW_ANALYSIS_PERSISTENCE=YES
COMMIT_ATOMICITY_PROPOSED=YES
IDEMPOTENCY_KEY_REQUIRED=YES
AUDIT_EVENT_PROPOSED=rfx.carrier_xlsx_import.committed.v1
MIGRATION_000073_SUFFICIENT=YES
MIGRATION_000074_REQUIRED=NO
MIGRATION_000074_CREATED=NO
TEST_MATRIX=E7P2-INT-71..119
TEST_CASE_COUNT=49
TEST_IDS_COMPLETE=YES
TEST_IDS_UNIQUE=YES
MATRIX_GUARD=EXTRA_NO_INT_ID
MATRIX_GUARD_NAME=TestE7P2CarrierXlsxMatrixIDsCompleteAndUnique
NEXT_TEST_ID_AFTER_CARRIER_XLSX=E7P2-INT-120
NEXT_ACTION=CARRIER_XLSX_C2_PREVIEW_IMPLEMENTATION
```

---

## 21. C1 export acceptance record (PR #132)

### Merge evidence

```
PR132_STATE=MERGED
PR132_HEAD=479d1fd75df1d969043f086bd42708d840a403ad
PR132_BASE=7454b94916fe76a9712ed80e0631db70bba5539b
PR132_MERGE_SHA=fa850826c6f7ed7f25e948dcc8fc8cad41eb2d74
PR132_MERGED_AT=2026-09-14T10:52:38Z
PR132_CI_RUN_ID=34833481110
PR132_CI_CONCLUSION=success
CONTROLLER_ACCEPTANCE=YES
CONTROLLER_VERDICT=ACCEPT_C1
```

Initial exact-head CI run `34829303236` @ `46220ee2` failed on INT-42 test drift (`unknown route export_carrier_response_xlsx`). Remediation commit `479d1fd7` (test-only) restored route parity; final exact-head run `34833481110` @ `479d1fd7` succeeded with all required checks PASS.

### Accepted C1 properties

| Property | Status |
|---|---|
| Schema `BINTRANS_RFX_CARRIER_XLSX_V1` | **YES** |
| Eight workbook sheets (no Sections sheet; `section_code` on Questions) | **YES** |
| DRAFT export (`export_mode=DRAFT_EDIT`) | **YES** |
| SUBMITTED read-only export (`export_mode=SUBMITTED_READONLY`) | **YES** |
| CarrierRead RBAC policy | **YES** |
| Tenant / company / event / response binding | **YES** |
| Feature flag default OFF (`RFX_EXCEL_EXCHANGE_ENABLED=false`) | **YES** |
| No Idempotency-Key on export GET | **YES** |
| Zero-lot event-level single OfferLines row; empty `lot_number` | **YES** |
| Formula-like literal text preserved; formula XML absent | **YES** |
| Semantic export determinism (excluding `exported_at_utc`) | **YES** |
| Competitor source queries absent | **YES** |
| Competitor confidentiality tests (cells + ZIP/XML scan) | **YES** |
| Export fully read-only (no response/answer/offer/audit/idempotency/import-analysis writes) | **YES** |
| Late permission and `submitted_at` untouched | **YES** |
| Service route `GET /v1/rfx-events/{event_id}/carrier-responses/{response_id}/xlsx-export` | **YES** |
| Gateway route `GET /api/v1/rfx-events/{id}/carrier-responses/{response_id}/xlsx-export` | **YES** |
| operationId `get_export_carrier_rfx_response_as_xlsx_workbook` | **YES** |
| OpenAPI source + unified YAML/JSON artifacts | **YES** |
| E7P2-INT-71..79 PASS (PostgreSQL 16 runtime) | **YES** |
| INT-42 route parity remediation PASS | **YES** |
| Buyer INT-06..70 regression PASS | **YES** |
| Migration 000074 absent | **YES** |
| Carrier Preview route absent | **YES** |
| Carrier Commit route absent | **YES** |
| Auto-submit absent | **YES** |

### Logical commits (PR #132)

| # | SHA | Message |
|---|-----|---------|
| 1 | `b49b931a` | `feat(rfx): generate Carrier XLSX export workbook` |
| 2 | `d3bf8e88` | `feat(rfx): expose Carrier XLSX export API` |
| 3 | `27add37b` | `test(rfx): cover Carrier XLSX export acceptance` |
| 4 | `9fd1ae45` | `docs(rfx): align C1 export status markers` |
| 5 | `46220ee2` | `feat(rfx): add Carrier XLSX export to unified openapi.json` |
| 6 | `479d1fd7` | `test(rfx): include Carrier XLSX export in INT-42 parity` |

### Accepted non-blocking LOW findings

```
LOW_01_STATUS=ACCEPTED_NON_BLOCKING
LOW_01=INT79_COMPETITOR_ANSWERS_NOT_SEEDED

LOW_02_STATUS=ACCEPTED_NON_BLOCKING
LOW_02=INT76_NO_WRITE_SNAPSHOT_PARTIAL
```

These findings are recorded for audit; they are **not** closed in this closeout.

### C1 status markers

```
CARRIER_XLSX_C1_EXPORT_STATUS=IMPLEMENTED_ACCEPTED
CARRIER_XLSX_C2_PREVIEW_STATUS=IMPLEMENTED_ACCEPTED
CARRIER_XLSX_C3_COMMIT_STATUS=NOT_STARTED
CARRIER_XLSX_OVERALL_STATUS=IMPLEMENTATION_IN_PROGRESS
C1_EXPORT_TEST_IDS=E7P2-INT-71..79
```

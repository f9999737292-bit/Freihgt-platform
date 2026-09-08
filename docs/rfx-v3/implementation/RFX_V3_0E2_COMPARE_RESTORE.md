# RFx v3.0E2 — Compare + Restore-as-Draft Backend

**Status:** IMPLEMENTED_ACCEPTED

**Merged to main:** PR #109 via merge commit `e1780e78425f9624b6ad956e24510260a2d9efcf`
**Scope:** E2 only — compare and restore-as-draft APIs behind `RFX_VERSIONING_V3_ENABLED`

---

## 1. Controller acceptance record

| Field | Value |
|---|---|
| `E2_STATUS` | IMPLEMENTED_ACCEPTED |
| `CONTROLLER_ACCEPTANCE` | YES |
| `PR109_MERGED` | YES |
| `PR109_HEAD` | `5f223b1aacd8e6de06e27cc3cc7650932dc71d68` |
| `PR109_MERGE_SHA` | `e1780e78425f9624b6ad956e24510260a2d9efcf` |
| `VERIFIED_MAIN_SHA` | `e1780e78425f9624b6ad956e24510260a2d9efcf` |
| `CI_RUN_ID` | 34241367509 |
| `CI_EXACT_HEAD` | YES (`5f223b1aacd8e6de06e27cc3cc7650932dc71d68`) |
| `CI_CONCLUSION` | success |

---

## 2. Delivered APIs

| Method | Path | Success | RBAC | Idempotency |
|---|---|---|---|---|
| POST | `/api/v1/rfx-events/{id}/versions/compare` | **200 OK** | Buyer read | No |
| POST | `/api/v1/rfx-events/{id}/versions/{version_id}/restore-draft` | **201 Created** | Buyer manage | Required |

### Compare

- Read-only; no domain audit write; no mutation of either version.
- Both versions must belong to the requested event and verified tenant.
- Cross-tenant or foreign-event version → **404**.
- Carrier access → **403**.
- Canonical diff uses stable codes: `section_code`, `question_code`, `option_code`, `rule_code`, scoring `(criterion_code, question_code)`.
- Classifications: `ADDED`, `REMOVED`, `CHANGED`, `REORDERED`, `UNCHANGED`.
- Response includes summary counts, ordered differences with field-level before/after, version metadata, and deterministic `canonical_diff_hash`.

### Restore-as-draft

- Creates a **new DRAFT** with `version_number = max + 1`; never rewinds history.
- Source may be **PUBLISHED** or **SUPERSEDED**; source rows remain immutable.
- Deep-copies questionnaire graph (sections, questions, options, rules); preserves stable codes.
- When source has a scoring model (PUBLISHED or DRAFT), deep-copies criteria and bindings into a new **DRAFT** score model on the restored version; remaps bindings via `section_code` + `question_code`; fail-closed rollback on unresolvable bindings.
- Existing active draft → **409** (no force option).
- Published pointer unchanged; carrier response pins and score history unchanged.
- Audit event: `rfx.version.restored_as_draft.v1`.
- Idempotency canonical payload: `{ "source_version_id", "change_summary" }` hashed under aggregate scope tenant/actor/operation/event.

---

## 3. Controller remediation closure (E2-001..003)

| ID | Requirement | Evidence |
|---|---|---|
| E2-001 | Restore idempotency includes `source_version_id` in canonical hash | `NewRestoreVersionAsDraftIdempotencyPayload` + E2-INT-14..16 |
| E2-002 | Restore scoring DRAFT deep copy with binding remap | `CopyDraftScoringFromSource` + E2-INT-17..24 |
| E2-003 | Compare returns 200 (not 201); restore remains 201 | handler + OpenAPI `vl_compare` profile + HTTP test + parity script |

---

## 4. Out of scope (E2)

- E3 change-impact engine
- E4–E5 templates / cloning
- E6 frontend compare/restore UI
- E7 final browser acceptance
- Late-submission implementation
- Re-scoring
- Migration 000069 (not required)

**E3:** PENDING_CONTROLLER_AUTHORIZATION

**E4–E7:** NOT_STARTED
**Complete v3.0E:** IMPLEMENTATION_IN_PROGRESS

---

## 5. Mandatory future gates (documented, not implemented in E2)

The following requirements are recorded for controller tracking and must be implemented before final tender browser acceptance (E7) or in their designated waves. **None are implemented in E2.**

### 5.1 Late submission

`LATE_SUBMISSION_AND_DEADLINE_EXCEPTIONS=REQUIRED`

- Carrier requests permission for late submission; reason is mandatory.
- Buyer can approve/invite with an individual deadline or reject.
- Global RFQ deadline is not changed; late bid is explicitly marked; all actions audited.
- **Target:** before E7 browser acceptance.

### 5.2 Buyer RFQ creation channels

| Marker | Status |
|---|---|
| `BUYER_RFQ_MANUAL_CREATION` | REQUIRED |
| `BUYER_RFQ_TEMPLATE_CREATION` | REQUIRED |
| `BUYER_RFQ_EXCEL_IMPORT` | REQUIRED |
| `BUYER_RFQ_ERP_INTEGRATION` | REQUIRED |

All channels must create the same canonical RFQ DRAFT and pass the same publish-readiness validation. Buyer may create RFQ manually, from template, via Excel import, or via SAP/1C/ERP/TMS integration.

### 5.3 Carrier offer channels

| Marker | Status |
|---|---|
| `CARRIER_DIRECT_OFFER_ENTRY` | REQUIRED |
| `CARRIER_OFFER_EXCEL_EXPORT_IMPORT` | REQUIRED |
| `CARRIER_ERP_INTEGRATION` | NOT_REQUIRED_CURRENT_SCOPE |

Carrier may submit offers directly in the platform or via personal Excel export/import. Excel import must populate a DRAFT only and must never automatically submit an offer.

### 5.4 Competitor confidentiality

| Marker | Value |
|---|---|
| `BUYER_CAN_MANAGE_INVITED_CARRIERS` | YES |
| `BUYER_CAN_IMPORT_INVITATION_LIST` | YES |
| `CARRIER_CAN_VIEW_OWN_INVITATION` | YES |
| `CARRIER_CAN_VIEW_OTHER_PARTICIPANTS` | NO |
| `CARRIER_CAN_VIEW_COMPETITOR_IDENTITIES` | NO |
| `CARRIER_CAN_VIEW_COMPETITOR_BIDS` | NO |
| `CARRIER_CAN_VIEW_COMPETITOR_LATE_REQUESTS` | NO |
| `BACKEND_ENFORCEMENT_REQUIRED` | YES |
| `CROSS_CARRIER_ISOLATION_TESTS_REQUIRED` | YES |

Carrier-facing APIs and Excel files must not expose participant lists, competitor identities, bids, submission times, or late-submission decisions for others.

### 5.5 Excel import/export

`EXCEL_IMPORT_EXPORT=REQUIRED`

Future scope: buyer RFQ matrix import/export, carrier personal bid-matrix export/import, versioned templates, preview-before-apply, row-level errors, formula-injection protection, file validation, audit, tenant isolation, published-version immutability.

### 5.6 Training course

`USER_TRAINING_COURSE=REQUIRED`

Audiences: buyer, carrier, administrator. Formats: written instructions, short videos, in-product onboarding, training RFQ, exercises, knowledge test, RU/EN/ZH. Required after UI stabilisation and before pilot/final tender acceptance.

---

## 6. Validation matrix

| Area | Tests |
|---|---|
| Domain compare engine | `internal/domain/version_compare_test.go` |
| Restore idempotency payload | `internal/domain/version_lifecycle_test.go` |
| E1 regression | E1-INT-01..33 retained |
| E2 integration | E2-INT-01..29 in `internal/integration/versionlifecycle/` |
| Compare HTTP 200 | `TestE2CompareHTTPReturns200` |
| OpenAPI parity | `scripts/openapi/test_path_structure.py` (compare=200, restore=201) |
| CI job | `rfx-version-lifecycle-v3-integration` with `REQUIRE_TEST_DATABASE=1` |

### E2 integration evidence map

| Test | Assertion |
|---|---|
| E2-INT-01..13 | Baseline compare/restore (prior tranche) |
| E2-INT-14 | Same key + source + body → replay same draft ID |
| E2-INT-15 | Same key + body, different source → 409, one draft |
| E2-INT-16 | Same key + source, different summary → 409 |
| E2-INT-17 | No source scoring → draft without score model |
| E2-INT-18 | Published scoring → new draft score model |
| E2-INT-19 | Criteria copied with new IDs, same config |
| E2-INT-20 | Bindings remapped to restored question IDs |
| E2-INT-21 | Scoring/knockout JSON preserved |
| E2-INT-22 | Source score model unchanged |
| E2-INT-23 | Response qualification history unchanged |
| E2-INT-24 | Unresolvable binding → full rollback |
| E2-INT-25 | Compare read-only (counts, timestamps, audit, idempotency) |
| E2-INT-26 | Persisted options/rules in compare |
| E2-INT-27 | Persisted scoring in compare |
| E2-INT-28 | Repository-loaded graphs → stable canonical hash |
| E2-INT-29 | Reverse compare ADDED/REMOVED symmetry |

---

## 7. Controller acceptance checklist

- [x] Compare semantics match architecture freeze §4
- [x] Restore semantics match architecture freeze §5
- [x] Tenant isolation and RBAC verified
- [x] Idempotency and concurrency behavior verified
- [x] E1 regression green on exact PR head
- [x] E2-INT-01..29 green on exact PR head
- [x] No unrelated OpenAPI drift in other services

# RFx v3.0E2 — Compare + Restore-as-Draft Backend

**Status:** IMPLEMENTED_PENDING_CONTROLLER_ACCEPTANCE  
**Branch:** `feat/rfx-version-compare-restore-v3.0e2`  
**Scope:** E2 only — compare and restore-as-draft APIs behind `RFX_VERSIONING_V3_ENABLED`

---

## 1. Delivered APIs

| Method | Path | RBAC | Idempotency |
|---|---|---|---|
| POST | `/api/v1/rfx-events/{id}/versions/compare` | Buyer read | No |
| POST | `/api/v1/rfx-events/{id}/versions/{version_id}/restore-draft` | Buyer manage | Required |

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
- Existing active draft → **409** (no force option).
- Published pointer unchanged; carrier response pins and score history unchanged.
- Audit event: `rfx.version.restored_as_draft.v1`.
- Idempotency reuses E1 pattern (`RESTORE_EVENT_DRAFT_VERSION` operation, 24h TTL, expired-key reuse).

---

## 2. Out of scope (E2)

- E3 change-impact engine
- E4–E5 templates / cloning
- E6 frontend compare/restore UI
- E7 final browser acceptance
- Late-submission implementation
- Re-scoring
- Migration 000069 (not required)

---

## 3. Late submission requirement (documented, not implemented)

`LATE_SUBMISSION_AND_DEADLINE_EXCEPTIONS=REQUIRED_BEFORE_FINAL_TENDER_BROWSER_ACCEPTANCE`

Before final end-to-end tender browser acceptance (E7):

- Carrier may request late submission with a reason.
- Buyer may invite/approve with an individual deadline or reject.
- This is separate future scope and is **not** implemented in E2.

---

## 4. Validation matrix

| Area | Tests |
|---|---|
| Domain compare engine | `internal/domain/version_compare_test.go` |
| E1 regression | E1-INT-01..33 retained |
| E2 integration | E2-INT-01..13 in `internal/integration/versionlifecycle/compare_restore_integration_test.go` |
| OpenAPI parity | `scripts/openapi/test_path_structure.py` |
| CI job | `rfx-version-lifecycle-v3-integration` with `REQUIRE_TEST_DATABASE=1` |

---

## 5. Controller acceptance checklist

- [ ] Compare semantics match architecture freeze §4
- [ ] Restore semantics match architecture freeze §5
- [ ] Tenant isolation and RBAC verified
- [ ] Idempotency and concurrency behavior verified
- [ ] E1 regression green on exact PR head
- [ ] No unrelated OpenAPI drift in other services

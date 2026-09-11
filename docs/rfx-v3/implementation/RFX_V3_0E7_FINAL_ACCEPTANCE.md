# RFx v3.0E7 — Final Integration, Late Submission, Data Exchange and Browser Acceptance

**Status:** `E7_PHASE_1_IMPLEMENTED_ACCEPTED`
**E7_STATUS:** `IMPLEMENTATION_IN_PROGRESS`
**E7_PHASE_1_STATUS:** `IMPLEMENTED_ACCEPTED`
**Scope:** E7 Phase 1 — late submission backend (persistence, domain, workflow, API, RBAC, OpenAPI, integration tests E7-INT-01..40 + E7-REM-001..008)
**Merged:** PR #119 @ `598b0b3f19861480ec2716e174692eb1d2425bed` (head `3a2f4b91c131c2fefe1056821618c7661fd4f4bc`, merged 2026-09-11T13:16:52Z)
**Closeout branch:** `docs/rfx-v3-0e7-phase1-post-merge-closeout`
**Worktree:** `D:\Projects\freight-platform-wt\rfx-v3-0e7-phase1-post-merge-closeout`

Prior waves E1–E6: **IMPLEMENTED_ACCEPTED** (PR #107–#117, docs closeout PR #118).

---

## 1. Recovery after interruption (2026-09-11)

| Item | Value |
|---|---|
| `RECOVERY_FROM_INTERRUPTION` | YES |
| `RECOVERY_START_HEAD` | `0150d26fb0fbbc518c1494cebadcba5a24e229d4` |
| `RESET_USED` | NO |
| `CLEAN_USED` | NO |
| `DUPLICATE_AGENT_STARTED` | NO |
| Partial work preserved | YES — migration 000072, domain/repository/service, handlers, carrier gate, integration tests, OpenAPI generator source |

Uncommitted partial implementation was resumed without reset/clean. Technical audit fixes applied before integration test completion: TOCTOU permission re-lock inside submit/save transactions, transactional audit binding, test helper deadline seeding, award conversion stub for integration harness.

---

## 2. Phase 1 scope (implemented)

| Component | Location | Status |
|---|---|---|
| Migration 000072 up/down | `infrastructure/migrations/000072_rfx_late_submission_v3_0e7.{up,down}.sql` | Implemented |
| Domain FSM + window rules | `services/rfx-service/internal/domain/late_submission.go` | Implemented |
| Repository | `services/rfx-service/internal/repository/late_submission_repository.go` | Implemented |
| Service (create/list/approve/reject/consume) | `services/rfx-service/internal/service/late_submission_service.go` | Implemented |
| HTTP handler + 5 routes | `services/rfx-service/internal/http/handlers/late_submission_handler.go`, `router.go` | Implemented |
| Carrier response deadline gate | `services/rfx-service/internal/service/carrier_response_service.go` | Implemented |
| Feature flag | `RFX_LATE_SUBMISSION_ENABLED` → HTTP 404 when disabled | Implemented |
| api-gateway routes + RBAC | `services/api-gateway/internal/http/router.go` | Implemented |
| OpenAPI (generator source) | `scripts/openapi/generate_openapi.py`, `packages/openapi/rfx-service.yaml`, unified `openapi.yaml`/`openapi.json` | Implemented |
| Integration tests E7-INT-01..40 + E7-REM-001..008 | `services/rfx-service/internal/integration/latesubmission/` | Implemented (CI on exact remediation HEAD) |
| Route parity manifest | `packages/shared-go/rfx/e7_late_submission_routes.go` | Implemented |
| Gateway identity spoof (E7) | `services/api-gateway/internal/rfxrbac/guard_test.go` | Implemented |
| Migration contract max 000072 | `scripts/ops/bintrans_ct_staging/bintrans_ct_staging_release_contract_selfcheck.sh` | Implemented |

**Not in Phase 1:** Excel import/export, ERP/SAP/1C, web-admin E7 UI, web-procurement E7 UI, training, browser acceptance (E7-BRW-*), migration 000073.

---

## 3. Migration 000072

**Table:** `rfx.rfx_late_submission_requests`

| Constraint / index | Purpose |
|---|---|
| `chk_rfx_late_submission_status` | `REQUESTED`, `APPROVED`, `REJECTED`, `EXPIRED`, `CONSUMED` |
| `chk_rfx_late_submission_reason_code` | `TECHNICAL_FAILURE`, `ORGANIZATIONAL_DELAY`, `BUYER_REQUEST`, `FORCE_MAJEURE`, `OTHER` |
| `chk_rfx_late_submission_reason_text_nonempty` | Non-empty trimmed `reason_text` |
| `chk_rfx_late_submission_approved_window` | Approved/consumed rows require `approved_valid_until > approved_valid_from`; other statuses null window |
| `chk_rfx_late_submission_consumed_at` | `CONSUMED` ↔ non-null `consumed_at` |
| Composite FK `(tenant_id, rfx_event_id)` → `rfx_events` | Tenant/event integrity |
| Composite FK `(tenant_id, rfx_event_id, carrier_company_id)` → `rfx_participants` | Carrier participant integrity |
| Composite FK `(tenant_id, rfx_event_id, participant_id)` → `rfx_participants` | Participant row integrity (nullable `participant_id` column; FK only when set) |
| `uq_rfx_late_submission_active_event_carrier` | Partial unique: one active `REQUESTED`/`APPROVED` per tenant/event/carrier |
| `idx_rfx_late_submission_buyer_queue` | Buyer queue listing |
| `idx_rfx_late_submission_carrier_lookup` | Carrier own-request lookup |

**Max migration contract:** `000072`. Migration `000073` rejected as unreleased.

Down migration sets `search_path = public` and drops only 000072 objects; legacy data outside this table is untouched.

---

## 4. State machine and semantics

### States

`REQUESTED` → `APPROVED` | `REJECTED` | (computed `EXPIRED` when past window without consumption)
`APPROVED` → `CONSUMED` (on successful carrier submit after global deadline)

Terminal: `REJECTED`, `EXPIRED`, `CONSUMED`. No `CANCELLED`.

### Deadline and window (frozen)

- **Global event deadline** is never modified by late submission (`ExtendDeadline` remains separate).
- Before global deadline: normal carrier save/submit; late request creation returns **HTTP 400** (`CodeValidation`, E7-INT-02). **HTTP 422** applies to post-deadline window/permission semantics (E7-INT-14, E7-INT-18..21), not pre-deadline create eligibility.
- After global deadline: save/submit allowed only with `APPROVED` permission inside approved window.
- **`valid_from` inclusive:** reject when `now.Before(valid_from)`.
- **`valid_until` exclusive:** reject when `!now.Before(valid_until)` (submit at exact boundary is forbidden).
- Server clock uses UTC (`time.Now().UTC()` / injected clock in tests).
- Save draft after deadline does **not** consume permission; submit consumes → `CONSUMED`.
- Idempotent submit replay returns original result; new idempotency key cannot reuse consumed permission.

### Audit events

`rfx.late_submission.requested.v1`, `approved.v1`, `rejected.v1`, `expired.v1`, `consumed.v1`

### Idempotency operations

`LATE_SUBMISSION_CREATE_REQUEST`, `LATE_SUBMISSION_APPROVE`, `LATE_SUBMISSION_REJECT`, `LATE_SUBMISSION_CARRIER_SUBMIT`

Audit and idempotency failures roll back business changes within the same transaction.

---

## 5. HTTP API (5 routes)

| Method | Path | Role | Notes |
|---|---|---|---|
| POST | `/api/v1/rfx-events/{id}/late-submission-requests` | Carrier | Create after deadline |
| GET | `/api/v1/rfx-events/{id}/late-submission-requests/mine` | Carrier | Own requests only |
| GET | `/api/v1/rfx-events/{id}/late-submission-requests` | BuyerRead | Review queue |
| POST | `/api/v1/rfx-events/{id}/late-submission-requests/{request_id}/approve` | BuyerManage | Sets approved window |
| POST | `/api/v1/rfx-events/{id}/late-submission-requests/{request_id}/reject` | BuyerManage | Terminal reject |

Headers: trusted gateway identity (`X-Tenant-ID`, `X-Company-ID`, `X-User-ID`); `Idempotency-Key` on mutating carrier/buyer decisions and late submit; `expected_version` optimistic concurrency on approve/reject.

Cross-tenant, cross-company, or unknown request ID → **404**. Feature disabled → **404** with zero writes. Invalid approve window → **422** (`CodeUnprocessable`). Double decision → **409**.

---

## 6. Integration test matrix (E7-INT-01..40)

| ID | Scenario |
|---|---|
| E7-INT-01 | Carrier creates late request after deadline |
| E7-INT-02 | Request before deadline → HTTP 400 Validation |
| E7-INT-03 | Empty reason forbidden |
| E7-INT-04 | Unknown reason code forbidden |
| E7-INT-05 | Carrier reads own request |
| E7-INT-06 | Carrier cannot see competitor request |
| E7-INT-07 | Buyer queue |
| E7-INT-08 | BuyerManage approve |
| E7-INT-09 | BuyerManage reject |
| E7-INT-10 | BuyerRead cannot decide |
| E7-INT-11 | Carrier cannot decide |
| E7-INT-12 | Cross-tenant → 404 |
| E7-INT-13 | Cross-company → 404 |
| E7-INT-14 | Invalid window → 422 |
| E7-INT-15 | Double approve → 409 |
| E7-INT-16 | Approve after reject → 409 |
| E7-INT-17 | Reject after approve → 409 |
| E7-INT-18 | Submit after deadline without permission forbidden |
| E7-INT-19 | Submit inside approved window passes |
| E7-INT-20 | Submit before window start forbidden |
| E7-INT-21 | Submit at/after window end forbidden (exclusive boundary) |
| E7-INT-22 | Permission → CONSUMED on submit |
| E7-INT-23 | New key cannot reuse CONSUMED |
| E7-INT-24 | Idempotent submit replay |
| E7-INT-25 | Global deadline unchanged |
| E7-INT-26 | Timely carrier response unchanged |
| E7-INT-27 | Concurrent approve/reject — one winner |
| E7-INT-28 | Concurrent submit — one business result |
| E7-INT-29 | Audit failure → full rollback |
| E7-INT-30 | Idempotency failure → full rollback |
| E7-INT-31 | Submit failure does not consume |
| E7-INT-32 | Feature flag disabled → 404, zero writes |
| E7-INT-33 | Spoofed identity headers ignored |
| E7-INT-34 | List/pagination tenant isolation |
| E7-INT-35 | Migration up |
| E7-INT-36 | Migration down (`search_path=public`) |
| E7-INT-37 | Up after down |
| E7-INT-38 | Direct SQL invalid status/reason rejected |
| E7-INT-39 | Direct SQL cross-event/FK rejected |
| E7-INT-40 | E1–E6 carrier response regression smoke |

Tests use PostgreSQL 16, clock abstraction (no `sleep`), `REQUIRE_TEST_DATABASE=1`, exact HTTP/machine error codes.

Unit tests cover FSM transitions, deadline/window boundaries, UTC conversion, idempotency payload stability.

### Remediation tests (E7-REM-001..008)

| ID | Scenario |
|---|---|
| E7-REM-001 | Expired APPROVED materialized → new REQUESTED succeeds |
| E7-REM-002 | REQUESTED blocks duplicate create |
| E7-REM-003 | Active APPROVED blocks duplicate create |
| E7-REM-004 | Concurrent rerequest after expiry — one business result |
| E7-REM-005 | Other carrier cannot spoof/materialize foreign request |
| E7-REM-006 | Audit failure rolls back expiry + create |
| E7-REM-007 | Idempotency failure rolls back expiry + create |
| E7-REM-008 | Valid-window submit safe against concurrent create |

---

## 7. Deferred phases (explicit)

| Phase | Item | Status |
|---|---|---|
| Phase 2 | Excel import/export | NOT STARTED |
| Phase 2 | ERP/SAP/1C contract | NOT STARTED |
| Phase 3 | web-admin / web-procurement E7 UI | NOT STARTED |
| Phase 3 | Training RU/EN/ZH | NOT STARTED |
| Phase 4 | Browser acceptance E7-BRW-* | NOT STARTED |
| — | v3.0F qualification pool | NOT IN E7 |
| — | TMS / Spot | NOT IN E7 |
| — | Staging/pilot deployment | NOT CHANGED |

---

## 8. Controller remediation (2026-09-11)

| Finding | Status | Evidence |
|---|---|---|
| E7P1-001 EXPIRED materialization | CLOSED | Transactional APPROVED→EXPIRED before create; E7-REM-001..008 |
| E7P1-002 five-route parity | CLOSED | `packages/shared-go/rfx/e7_late_submission_routes.go` + parity tests |
| E7P1-003 gateway identity spoof | CLOSED | `TestE7GatewayIdentityHeaderSpoofLateSubmission*` |
| E7P1-004 pre-deadline HTTP 400 | CLOSED | Doc/OpenAPI/E7-INT-02 aligned |
| E7P1-005 exact-head evidence | CLOSED | This section + PR body after remediation CI |

Historical CI green at `c045fd8` / run `34595040796` attempt 1 timeout retained for audit only. Authoritative evidence is remediation exact-head CI (see PR body).

---

## 9. Validation evidence

| Check | Result |
|---|---|
| Domain/repository/service unit tests | PASS (local) |
| rfx-service build | PASS (local) |
| api-gateway tests + build | PASS (local) |
| Five-route parity tests | PASS (local) |
| Gateway E7 spoof tests | PASS (local) |
| Integration compile (`-tags=integration`) | PASS (local) |
| OpenAPI validate | PASS (local) |
| OpenAPI generate idempotent (two consecutive runs) | PASS (local) |
| Migration contract selfcheck | PASS (local) |
| PostgreSQL integration E7-INT-* + E7-REM-* | PASS (CI run `34599209374` on PostgreSQL 16; local `TEST_DATABASE_URL` unset → NOT_RUN) |
| `git diff --check` | PASS (local) |

---

## 10. Post-merge closeout (2026-09-11)

PR #119 merged to `main` via merge commit after controller verdict **ACCEPT**.

| Item | Value |
|---|---|
| `PR119_MERGED` | YES |
| `PR119_HEAD` | `3a2f4b91c131c2fefe1056821618c7661fd4f4bc` |
| `PR119_MERGE_SHA` | `598b0b3f19861480ec2716e174692eb1d2425bed` |
| `PR119_MERGED_AT` | `2026-09-11T13:16:52Z` |
| `PR119_CI_RUN_ID` | `34599209374` |
| `PR119_CI_CONCLUSION` | `success` |
| `origin/main` at post-merge verification | `598b0b3f19861480ec2716e174692eb1d2425bed` (no additional commits on main) |

### Optional hardening (non-blocking LOW findings, accepted)

| ID | Item |
|---|---|
| E7P1-R2-001 | Boundary-race integration evidence at `valid_until` |
| E7P1-R2-002 | Clarify HTTP 400 vs 422 descriptions in OpenAPI create operation |
| E7P1-R2-003 | Inline SHA/CI in acceptance docs (addressed by this closeout) |
| E7P1-R2-004 | Full gateway-router E2E for E7 identity spoof |
| E7P1-R2-005 | Explicit assertion that `X-Carrier-Company-ID` is stripped |

---

## 11. Status markers

```
E7_PHASE_1_STATUS=IMPLEMENTED_ACCEPTED
E7_PHASE_1_CONTROLLER_ACCEPTANCE=YES
E7_STATUS=IMPLEMENTATION_IN_PROGRESS
ROADMAP_V3_0E_STATUS=IMPLEMENTATION_IN_PROGRESS
PR119_MERGED=YES
PR119_HEAD=3a2f4b91c131c2fefe1056821618c7661fd4f4bc
PR119_MERGE_SHA=598b0b3f19861480ec2716e174692eb1d2425bed
PR119_MERGED_AT=2026-09-11T13:16:52Z
PR119_CI_RUN_ID=34599209374
PR119_CI_CONCLUSION=success
MIGRATION_000072_CREATED=YES
MIGRATION_000073_CREATED=NO
MAX_MIGRATION_CONTRACT=000072
CONTROLLER_VERDICT=ACCEPT
E7P1_001_E7P1_005_CLOSED=YES
BLOCKER_FINDINGS_OPEN=0
HIGH_FINDINGS_OPEN=0
MEDIUM_FINDINGS_OPEN=0
LOW_FINDINGS_OPEN=5_ACCEPTED_NON_BLOCKING
PRE_DEADLINE_REQUEST_HTTP_STATUS=400
E7_BROWSER_ACCEPTANCE=NOT_STARTED
E7_PHASE_2=NOT_STARTED
NEXT_ACTION=CONTROLLER_REVIEW_DOCS_CLOSEOUT_PR
```

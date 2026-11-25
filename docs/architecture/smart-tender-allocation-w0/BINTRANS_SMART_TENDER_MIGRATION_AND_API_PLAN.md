# BINTRANS Smart Tender — Migration & API Plan (Spec v0.9)

**API prefix (D6):** `/api/v1/rfx/allocations`  
**Migration head:** `000105` — **unchanged** in W0 (no SQL files in this freeze)  
**Main:** `892361db87985f35132aef86550a025909af648a`

---

## 1. Route plan (design contracts)

All mutating routes: tenant/company authz, RBAC, idempotency where needed, version preconditions / If-Match, audit, correlation ID, fail-closed, stable errors. Legacy Award routes untouched.

| Family | Example |
|---|---|
| Policies | `POST/GET /api/v1/rfx/allocations/policies`, `.../policies/{id}/versions`, validate, approve |
| Rounds | `.../events/{rfxEventId}/rounds`, open/close/approve-transition |
| Simulations | `.../events/{rfxEventId}/simulations`, get budget/explanations |
| Proposals / decisions | `.../events/{rfxEventId}/proposals`, submit-approval, recalculate |
| Commission | `.../approval-workflows/{id}`, votes, request-changes |
| Carrier | `.../carrier/commitment-proposals`, respond |
| Appendix / EDO | `.../contract-appendices` (ownership with document-service TBD at W7) |

**Collision check:** OpenAPI has no these paths today. Payment `/api/v1/payment-allocations` remains a different BC — avoided by `rfx/allocations` prefix (D6).

---

## 2. Logical schema (no physical migration numbers)

Entities per TZ §19 under `rfx` or dedicated allocation schema — Decision at W1 start.  
Invariants: tenant-safe FKs; unique votes; immutable snapshots; ledger idempotency_key UNIQUE; non-negative quotas; version sequences.  
**Never** drop/alter `uq_rfx_award_event`.

---

## 3. Migration strategy

| Wave | Schema intent |
|---|---|
| W0 | Docs only; head 000105 |
| W1 | Policies / AST versions only — after Controller GO |
| W3 | Commercial rounds |
| W4–W5 | Proposals, decisions, share constraints |
| W6 | Commitments, guarantees, ledger |
| W7 | Appendix/EDO tables after document P0 path |
| W8 | Activation / TO linkage |

E2E stand pin 000105 remains until Controller updates pin policy.  
Legacy regression gates: see Legacy Compatibility doc.

---

## 4. Idempotency / concurrency

Simulations keyed by input hash; votes/proposals use subject_hash; commitment responses versioned; EDO submit idempotent; TO quota consume ledger key = f(decision_id, TO idempotency). No 2PC with external EDO.

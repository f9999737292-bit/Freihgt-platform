# Freight Payments & Reconciliation Architecture (v1.9.2)

**Status:** Architecture freeze — discovery complete, **no product implementation in this slice**  
**Base:** `main` @ `a15b3ff2557489a89e9e1b78c8563aac31b6a48d` (v1.9.1 Payment Backend Core merged)  
**Branch:** `arch/freight-payments-reconciliation-v1.9.2`  
**Primary target:** Reconciliation integrity + reliable Payment Obligation PAID → Billing Register PAID delivery

---

## 1. Executive Summary

v1.9.1 delivered payment-service, migration `000045`, partial/multi allocation, explicit
reconciliation, concurrent allocation protection, atomic financial audit, internal service auth,
and a **manual** PAID register projection retry path.

v1.9.2 closes the remaining financial integrity gaps:

1. **Durable PAID projection delivery** (transactional outbox + worker; no operator-dependent retry)
2. **Allocation reversal** (append-only void; no DELETE)
3. **Payment void** (safe preconditions)
4. **Reconciliation hardening** (recompute invariants; idempotent repeat)
5. **Import/API idempotency preparation** (no bank integration)
6. **Due-date / payment-term discovery conclusion** (manual due-date remains)

**Out of scope:** UI, bank integration, migration execution, Control Tower payment events,
Kafka topic creation for CT, negative-payment reversal documents.

---

## 2. Version Plan Correction

### 2.1 Original v1.9.2 plan (v1.9.0 architecture doc §20)

The v1.9.0 roadmap listed v1.9.2 as:

> Partial/multi allocation, overpayment unallocated remainder, reconciliation confirm, concurrent tests.

### 2.2 Already delivered in v1.9.1 (merged PR #27)

| Capability | Status | Evidence |
|------------|--------|----------|
| Partial allocation | **YES** | `payment_repository.Allocate`, `DeriveObligationStatus`, `TestPartialAndFullAllocation` |
| Multi payment → one obligation | **YES** | Two payments allocating 40+60 → obligation PAID |
| One payment → multi obligation | **YES (code)** | N:M model; partial test covers multi-payment side; one-to-many not integration-tested |
| Unallocated remainder | **YES** | `unallocated_amount` column + CHECK + index; `DeriveUnallocated` |
| Explicit reconciliation | **PARTIAL** | `POST /payments/{id}/reconcile`; negative integration test only |
| Concurrent allocation protection | **YES** | `FOR UPDATE` lock ordering + `TestConcurrentAllocationConflict` |
| PAID projection sync (immediate) | **YES** | `Allocate` → `SyncRegisterPaid` after commit |
| PAID projection retry (manual) | **YES** | `EnsureBillingRegisterPaidProjection` + internal endpoint |
| Payment audit atomicity | **YES** | Same-tx audit; rollback integration tests |
| Internal service auth | **YES** | `packages/shared-go/internalauth` |
| Legacy mark-paid fail-closed | **YES** | Obligation gate + HTTP integration tests |
| Payment void | **NO** | Schema columns only |
| Allocation void | **NO** | `voided_at` column only |
| Import/API payment | **NO** | MANUAL-only enforced |
| Transactional outbox | **NO** | Sync audit only |

**ORIGINAL_V1_9_2_PLAN_STALE = YES**

### 2.3 Remaining actual v1.9.2 scope

1. Transactional **payment obligation PAID → register PAID** outbox + worker
2. Allocation reversal API + invariants (with financial finality boundary)
3. Payment void API + invariants
4. Reconciliation hardening + idempotent repeat + post-reconcile mutation lock
5. Import/API payment creation (idempotent by `external_id`) — **preparation only**, no bank
6. Optional derived unallocated query/filter (no new OVERPAID obligation status)
7. Due-date automation remains **deferred**

**Do NOT reimplement** partial allocation, concurrency guards, or basic reconcile endpoint.

---

## 3. Current Implementation Reference (v1.9.1)

### 3.1 Services & artifacts

| Path | Role |
|------|------|
| `services/payment-service/` | Payment SSOT |
| `services/billing-register-service/` | Register lifecycle + PAID projection consumer |
| `services/api-gateway/internal/paymentrbac/` | JWT + company context + role maps |
| `packages/shared-go/internalauth/` | `X-Internal-Service-Token` middleware |
| `infrastructure/migrations/000045_freight_payments_core_v1.9.1.up.sql` | obligations, payments, allocations, audit |
| `services/payment-service/internal/integration/freightpaymentscore/` | Payment integration gate |
| `services/billing-register-service/internal/integration/freightbillingclosing/mark_paid_http_integration_test.go` | Legacy mark-paid HTTP gate |

### 3.2 PAID projection today (v1.9.1)

```
Allocate (payment-service, single DB tx)
  → obligation may become PAID
  → COMMIT
  → best-effort HTTP SyncRegisterPaid (billing internal endpoint)
  → on failure: RegisterPaidProjection{status: FAILED, retryable: true}

Manual repair:
  POST /internal/v1/billing-registers/{id}/ensure-paid-projection
  (re-reads canonical obligation; calls billing sync-paid)
```

**DURABILITY_GAP:** If billing is down after allocation commit, delivery intent is not
persisted. Operator/client must invoke retry. Obligation correctly remains PAID (SSOT).

---

## 4. Canonical Outbox Pattern (Repository Discovery)

**Source of truth:** `services/shipment-service` — **only** outbox implementation in repo.

| Property | Shipment pattern | Payment v1.9.2 adaptation |
|----------|------------------|---------------------------|
| Table | `transport.shipment_event_outbox` | `billing.payment_outbox` (proposed) |
| Migration | `000014_create_shipment_event_outbox_v0.1.up.sql` | New migration after `000045` |
| Same-tx insert | History + outbox in one tx | Obligation PAID update + audit + outbox |
| Status enum | `PENDING` / `PUBLISHED` / `FAILED` | Same |
| Retry schedule | `available_at` | Same (not `next_attempt_at`) |
| Success marker | `published_at` | Same (not `delivered_at`) |
| Claim | `FOR UPDATE SKIP LOCKED` + lease columns | Same |
| Worker | In-process poll loop in service | `payment-service` worker |
| Transport | Kafka (shipment) | **HTTP** to billing internal endpoint |
| Multi-worker | Lease + worker_id ownership on mark | Same |

**Reference files:**

- `services/shipment-service/internal/repository/outbox_write.go`
- `services/shipment-service/internal/repository/outbox_queries.go`
- `services/shipment-service/internal/outbox/worker.go`
- `services/shipment-service/internal/outbox/backoff.go`
- `services/shipment-service/internal/platform/metrics/outbox.go`
- `docs/SHIPMENT_STATUS_OUTBOX.md`

**Do NOT invent a new outbox framework.** Copy shipment patterns; adapt transport to HTTP.

---

## 5. Reliable PAID Projection — Frozen Design

### 5.1 SSOT rule (FROZEN)

| Entity | Role |
|--------|------|
| `PaymentObligation` | **Payment SSOT** — paid amounts, status |
| `billing.payment_outbox` | **Delivery intent** — not second payment truth |
| `BillingRegister.status IN (PAID, CLOSED)` | **Projection** — must re-validate obligation |

**Invariant (frozen v1.9.2):**

```
REGISTER.status IN (PAID, CLOSED)
  ⇒ obligation.status == PAID
  AND obligation.paid_amount == obligation.original_amount
  AND obligation.outstanding_amount == 0
```

`CLOSED` semantically subsumes successful PAID projection (`SIGNED_BY_COUNTERPARTY → PAID → CLOSED`).
Do **not** reopen or mutate a CLOSED register back to PAID.

Outbox payload must **NOT** carry authoritative paid amounts for billing. Billing
`SyncPaidFromObligation` continues to read obligation from DB via
`PaymentObligationLookupRepository.ValidateRegisterPaidPreconditions`.

### 5.2 Transaction boundary (FROZEN)

When allocation (or any mutation) first transitions obligation to `PAID`:

```
BEGIN
  -- existing allocation / obligation update logic (FOR UPDATE locks)
  UPDATE payment_obligations SET status=PAID, paid_amount=..., outstanding_amount=0
  INSERT payment_audit_events (OBLIGATION_ALLOCATED / obligation state change)
  INSERT payment_outbox (
    event_type = 'payment_obligation.paid',
    aggregate_type = 'PAYMENT_OBLIGATION',
    aggregate_id = obligation_id,
    tenant_id,
    payload = { obligation_id, register_id (source_id), tenant_id }  -- identifiers only
    status = PENDING
  )
COMMIT
```

If obligation was already PAID and outbox event for this transition was previously
published, do **not** insert duplicate outbox row (idempotency — see §5.5).

If transaction rolls back → no outbox row (F1).

### 5.3 Direct sync vs outbox — **Option B (RECOMMENDED)**

| Option | Verdict |
|--------|---------|
| A — Outbox only | Safe but adds latency; removes existing working path |
| **B — Outbox + immediate sync** | **SELECTED** |

**Option B semantics:**

1. Outbox row is **always** inserted in the same transaction that first makes obligation PAID.
2. After successful COMMIT, service **may** attempt immediate `SyncRegisterPaid` (latency optimization).
3. If immediate sync succeeds → mark outbox row `PUBLISHED` / set `published_at` in a **separate** short transaction.
4. If immediate sync fails → outbox remains `PENDING`; response includes projection failure (existing v1.9.1 behavior).
5. If crash occurs after billing succeeded but before outbox mark published → worker retry is **safe** (billing sync is idempotent).
6. Worker is the **durable backstop**; immediate sync is best-effort accelerator.

**Guarantee:** Obligation PAID never commits without durable delivery intent row.

`EnsureBillingRegisterPaidProjection` remains as explicit operator/API repair path; worker
reduces dependence on it.

### 5.4 Worker architecture

**Location:** `services/payment-service/internal/outbox/` (mirror shipment layout)

**Responsibilities:**

- Poll `billing.payment_outbox` where `status=PENDING` and `available_at <= now()`
- Claim batch via `FOR UPDATE SKIP LOCKED` + lease (`locked_at`, `locked_by`)
- For `payment_obligation.paid` events: HTTP POST to billing
  `POST /internal/v1/billing-registers/{register_id}/sync-paid`
  with `X-Internal-Service-Token`; tenant from persisted event/obligation
- On success → `MarkPublished`
- On register already **PAID** → idempotent success → `MarkPublished`
- On register already **CLOSED** + canonical obligation **PAID** → **ALREADY_SATISFIED** →
  `MarkPublished`; **no register mutation**, no retry loop, no duplicate `MARKED_PAID` audit
- On transient failure → `ReleaseWithRetry` with shipment backoff table
- On permanent failure / max attempts → `MarkFailed`
- Graceful shutdown; multi-instance safe

**Env flags (proposed):** `PAYMENT_OUTBOX_ENABLED`, `PAYMENT_OUTBOX_POLL_INTERVAL`,
`PAYMENT_OUTBOX_BATCH_SIZE`, `PAYMENT_OUTBOX_MAX_ATTEMPTS`, `PAYMENT_OUTBOX_LEASE_TIMEOUT`

### 5.5 Event creation idempotency (FROZEN)

| Field | Value |
|-------|-------|
| `DELIVERY_SEMANTICS` | **AT_LEAST_ONCE** (applies to **delivery**, not duplicate event creation) |
| `EXACTLY_ONCE_CLAIM` | **NO** |
| `event_type` | `payment_obligation.paid` |
| `aggregate_type` | `PAYMENT_OBLIGATION` |
| `aggregate_id` | `obligation_id` |

**Exact idempotency key (database constraint):**

```sql
UNIQUE (tenant_id, event_type, aggregate_id)
```

**Rationale:**

- Obligation ordinary PAID → unpaid rollback is forbidden (ADR-13).
- One obligation therefore has **one** canonical `payment_obligation.paid` transition.
- Duplicate insert attempts in the same or retried business flow resolve to the existing
  outbox row (conflict / upsert-noop — implementation choice; constraint is authoritative).
- At-least-once applies to **worker delivery**, not duplicate event row creation.

**Required columns (no `source_transition`):**

| Column | Notes |
|--------|-------|
| `id` | UUID PRIMARY KEY |
| `tenant_id` | Tenant scope |
| `aggregate_type` | `PAYMENT_OBLIGATION` |
| `aggregate_id` | `obligation_id` |
| `event_type` | `payment_obligation.paid` |
| `payload` | Identifiers only (no authoritative amounts) |
| `status` | `PENDING` / `PUBLISHED` / `FAILED` |
| `attempts` | Delivery attempt counter |
| `available_at` | Retry schedule |
| `locked_at`, `locked_by` | Worker lease |
| `published_at` | Success marker |
| `last_error_code` | Last delivery failure |
| `created_at` | Insert time |

Billing consumer (`SyncPaidFromPaymentObligation`) already idempotent when register is PAID.
When register is CLOSED and obligation is canonically PAID, treat projection as
**ALREADY_SATISFIED** (see F6).

### 5.6 Failure matrix

| ID | Scenario | Expected behavior |
|----|----------|-------------------|
| F1 | Obligation PAID tx rolls back | No outbox row |
| F2 | PAID commits; billing down | Outbox PENDING; worker retries |
| F3 | Billing succeeds; worker crashes before publish mark | Retry duplicate → idempotent success |
| F4 | Two workers claim queue | SKIP LOCKED + lease → one owner |
| F5 | Billing already PAID | Idempotent success; mark published |
| F6 | Billing already **CLOSED** + canonical obligation **PAID** | **ALREADY_SATISFIED** — validate obligation; **do not** reopen or mutate register; mark outbox **PUBLISHED**; no retry loop / poison event; no duplicate `MARKED_PAID` audit. If obligation validation fails → financial integrity error → **FAILED** / alert |
| F7 | Obligation missing/invalid | Mark failed; alert; no fabricated register state |
| F8 | Poison event | Backoff → FAILED after max attempts; metrics + logs |

---

## 6. Allocation Reversal

### 6.1 Schema today (migration 000045)

`billing.payment_allocations.voided_at` — **EXISTS**  
`billing.payments.voided_at`, `voided_by`, `void_reason` — **EXISTS**  
No `voided_by` on allocations — **add in v1.9.2B migration** (next available migration
number at implementation time; not in 000046)

### 6.2 Proposed API (implementation deferred to v1.9.2B)

```
POST /api/v1/payment-allocations/{id}/void
Body: { "reason": "..." }
```

### 6.3 Transaction concept (FROZEN)

```
BEGIN
  lock allocation (active: voided_at IS NULL)
  lock payment, lock obligation
  validate RBAC + payer/payee scope
  validate reversal policy (§6.4)
  UPDATE allocation SET voided_at=now(), voided_by=..., void_reason=...
  recompute from ACTIVE allocations:
    payment.allocated_amount, unallocated_amount, status
    obligation.paid_amount, outstanding_amount, status
  INSERT audit (ALLOCATION_VOIDED, PAYMENT_REALLOCATED, OBLIGATION_REALLOCATED)
COMMIT
```

**No DELETE.** Append-only void.

### 6.4 Financial finality boundary (FROZEN — ADR-13)

| Obligation state | Reversal allowed? |
|------------------|-------------------|
| OPEN / PARTIALLY_PAID | **YES** — if reversal does not violate invariants |
| PAID with register projection PAID | **NO** — ordinary reversal **rejected** |
| PAID but register not yet projected | **Policy: NO** — treat PAID obligation as financially final |

**Rationale:** Reversing allocation that made obligation PAID would roll obligation to
OPEN/PARTIALLY_PAID while billing register may already be PAID or CLOSED — dual truth /
legal inconsistency.

**Corrective action for finalized state:** future adjustment/credit note workflow (v1.9.3+),
not destructive status rollback.

**Billing lifecycle:**

```
SIGNED_BY_COUNTERPARTY → PAID → CLOSED
```

Neither PAID nor CLOSED may be silently rolled back by payment reversal in v1.9.2.

---

## 7. Payment Void

### 7.1 Proposed API (v1.9.2B)

```
POST /api/v1/payments/{id}/void
Body: { "reason": "..." }
```

### 7.2 Policy (FROZEN — ADR-14)

| Payment status | Void allowed? |
|----------------|---------------|
| RECEIVED, zero active allocations | **YES** |
| PARTIALLY_ALLOCATED | **NO** — void allocations first (where permitted) |
| FULLY_ALLOCATED | **NO** |
| RECONCILED | **NO** |
| Already VOIDED | Idempotent success |

Required: reason, actor, timestamp, audit event `PAYMENT_VOIDED`.

**No negative payment rows** in v1.9.2.

---

## 8. Reconciliation Hardening

### 8.1 Current behavior (v1.9.1)

- `ValidateReconcilePayment` checks `payment.status == FULLY_ALLOCATED` only
- Does **not** recompute `SUM(active allocations)` at reconcile time
- Does **not** verify stored `allocated_amount == payment.amount`
- Repeat reconcile: **not idempotent** (would attempt transition again — likely conflict)
- Post-reconcile mutation: **not blocked** (new allocation could be added)

### 8.2 Hardened rule (FROZEN — ADR-15)

`RECONCILED` iff **all** of:

1. `payment.status == FULLY_ALLOCATED` (or transitioning from it)
2. `SUM(active allocations WHERE voided_at IS NULL) == payment.amount`
3. Stored `payment.allocated_amount == payment.amount`
4. All active allocations pass currency/party invariants
5. Explicit authorized actor confirmation

Implementation: recompute in repository tx before status update.

### 8.3 Idempotent repeat (FROZEN)

```
POST /payments/{id}/reconcile
```

If already `RECONCILED` → return existing payment **200 OK** without duplicate
`PAYMENT_RECONCILED` audit event.

### 8.4 Post-reconcile mutation policy

After `RECONCILED`:

- **Block** new allocations
- **Block** allocation reversal via ordinary flow
- **Block** direct payment void

Unless future explicit reversal workflow (out of v1.9.2 scope).

---

## 9. Unapplied / Overpayment

v1.9.1 already models remainder via `payment.unallocated_amount > 0`.

**FROZEN:**

- No `OVERPAID` obligation status
- Payment amount may exceed single obligation allocation
- Only allocated portion affects obligation balances
- No auto-credit, auto-write-off, cross-company/currency allocation

**v1.9.2:** Optional list filter `?unallocated_only=true` on payments — **nice-to-have**;
not mandatory if index `idx_payments_tenant_unallocated` suffices.

---

## 10. Import / API Idempotency (Preparation)

### 10.1 Current DB (000045)

| Index | Policy |
|-------|--------|
| `uq_payment_bank_external_id` | `(tenant_id, source, external_id)` permanent for IMPORT/API/BANK_* |
| `uq_payment_manual_external_id_active` | MANUAL active-only (void frees slot) |

### 10.2 Frozen policies (ADR-16)

| Source | `external_id` policy |
|--------|---------------------|
| MANUAL | Optional; active-only uniqueness |
| IMPORT / API / BANK_* | Required for dedup; **permanent** uniqueness |
| VOID | Does **not** release provider `external_id` for non-MANUAL sources |

### 10.3 Migration change required?

**Likely NO** for v1.9.2 if 000045 indexes match above. Verify at implementation time.
If gap found → address in v1.9.2D or a dedicated idempotency migration (not 000046).

### 10.4 Implementation scope

v1.9.2 may add `POST /payments/import` or source-aware create — **defer full bank**.
Minimum: service methods accepting IMPORT/API source with idempotent upsert on conflict.

---

## 11. Payment Events

### 11.1 Mandatory v1.9.2 event

| Event | Producer | Consumer | Tx boundary |
|-------|----------|----------|-------------|
| `payment_obligation.paid` | payment-service (allocation tx) | payment outbox worker → billing HTTP | Same tx as obligation PAID |

### 11.2 Optional / deferred

| Event | Defer? | Reason |
|-------|--------|--------|
| `payment.reconciled` | **YES** | No downstream consumer |
| `payment.voided` | **YES** | No consumer |
| `payment.allocation_voided` | **YES** | No consumer |

Sync `payment_audit_events` remains for operator audit trail.

### 11.3 Control Tower (deferred to v1.9.4)

Reserved: `PAYMENT_OVERDUE`, `PAYMENT_UNALLOCATED`, `PAYMENT_PARTIALLY_PAID`,
`PAYMENT_RECONCILIATION_FAILED` — **not v1.9.2**.

---

## 12. Due Date / Payment Terms Discovery

| Discovery | Result |
|-----------|--------|
| `payment_terms` entity | **NOT_FOUND** |
| `net_days` / `due_days` fields | **NOT_FOUND** |
| Contract payment terms in contract-service | **NOT_FOUND** |
| Invoice payment deadline fields | **NOT_FOUND** |
| Legal term-clock source | **NOT_FOUND** |

**Existing:** nullable `payment_obligations.due_date`; manual PATCH only.

**DUE_DATE_AUTOMATION = DEFERRED** (ADR-17) — do not invent +30 days.

**OPEN_QUESTION_001** from v1.9.0 doc remains open.

---

## 13. Billing CLOSED Semantics

- `CLOSED` only from `PAID` (`ValidateCloseRegisterStatus`)
- Separate manual actor transition; not automatic after projection
- Payment correction must **never** silently reopen PAID or CLOSED register
- Legally finalized register → future correction workflow, not status rollback

**PAID projection when register is already CLOSED (FROZEN):**

| Condition | Worker action |
|-----------|---------------|
| Register **CLOSED** + obligation canonically **PAID** | **ALREADY_SATISFIED** — mark outbox **PUBLISHED**; no register mutation |
| Register **CLOSED** + obligation **not** canonically PAID | Financial integrity error — **FAILED** / alert |
| Any scenario | **Never** reopen CLOSED → PAID or mutate finalized register |

---

## 14. Migration Discovery

| Field | Value |
|-------|-------|
| `CURRENT_MAX_MIGRATION` | **000045** (`000045_freight_payments_core_v1.9.1`) |
| `V1_9_2_MIGRATION_REQUIRED` | **YES** |

**Proposed `000046` scope (v1.9.2A — implementation phase):**

**PAYMENT OUTBOX ONLY.** Do not bundle reversal/void schema changes.

1. `billing.payment_outbox` table (shipment-aligned columns adapted for billing schema)
2. `UNIQUE (tenant_id, event_type, aggregate_id)` idempotency constraint
3. Partial index on `(status, available_at, created_at) WHERE status='PENDING'`
4. Indexes required by worker claim/poll queries

**Deferred to v1.9.2B** (next available migration number at implementation time — do not
permanently reserve `000047`; main may advance):

- `payment_allocations.voided_by`
- `payment_allocations.void_reason`

**Do not create migration in this architecture task.**

---

## 15. Observability

Follow `services/shipment-service/internal/platform/metrics/outbox.go` naming style.

**Proposed metrics:**

| Metric | Type |
|--------|------|
| `payment_outbox_pending_count` | gauge |
| `payment_outbox_failed_count` | gauge |
| `payment_outbox_oldest_pending_age_seconds` | gauge |
| `payment_outbox_claimed_total` | counter (`event_type`) |
| `payment_outbox_published_total` | counter (`event_type`, `result`) |
| `payment_outbox_publish_failed_total` | counter (`event_type`, `error_code`) |
| `payment_outbox_publish_duration_seconds` | histogram |

**Logs:** `event_id`, `tenant_id`, `aggregate_id`, `event_type`, `attempt` — never token or bank payload.

---

## 16. Security

| Area | Rule |
|------|------|
| Outbox worker → billing | `X-Internal-Service-Token` required |
| Tenant scope | From persisted obligation/event, not caller-supplied body alone |
| Void/reversal APIs | JWT + companycontext + paymentrbac + membership + audit |
| Internal endpoints | Not public via gateway |

---

## 17. RBAC for Reversals

### 17.1 Current paymentrbac (v1.9.1)

Policies: `PolicyRead`, `PolicyCreate`, `PolicyAllocate`, `PolicyReconcile`  
**No `PolicyVoid`**

Roles with create/allocate/reconcile: PLATFORM_ADMIN, SHIPPER_ADMIN, FINANCE_MANAGER,
CARRIER_ADMIN, CARRIER_ACCOUNTANT

### 17.2 Proposed v1.9.2 (FROZEN)

Add `PolicyVoid` mapped to **`payment.void`** permission concept:

| Role | Void/reversal |
|------|---------------|
| FINANCE_MANAGER | **YES** |
| SHIPPER_ADMIN | **YES** (payer org) |
| CARRIER_ADMIN / CARRIER_ACCOUNTANT | **YES** (payee org) |
| PLATFORM_ADMIN | **YES** (with membership — no arbitrary impersonation) |
| SHIPPER_LOGIST | **NO** |
| CARRIER_DISPATCHER | **NO** |

Reuse permission name **`payment.void`** — do not invent `payment.reverse`.

Segregation: not all `PolicyCreate` roles receive `PolicyVoid`.

---

## 18. Architecture Decision Records (v1.9.2)

### ADR-09 — Reliable PAID projection uses transactional payment outbox

Payment obligation PAID transition and outbox insert occur in **one PostgreSQL transaction**.
Worker delivers to billing internal endpoint.

### ADR-10 — Delivery semantics

**At-least-once** delivery + idempotent billing consumer. **No exactly-once claim.**

### ADR-11 — Payment obligation remains SSOT

Outbox is delivery state only. Billing PAID is projection validated against obligation DB state.

### ADR-12 — Allocation reversal is append-only void

Never DELETE allocations. `voided_at` marks inactive; recompute balances from active set.

### ADR-13 — Finality boundary for reversal

Ordinary reversal must **not** roll finalized PAID/CLOSED billing state backward.
Reversal blocked when obligation reached PAID.

### ADR-14 — Payment void preconditions

Void only when no active allocations and not RECONCILED. Reason + audit required.

### ADR-15 — Reconciliation recompute

RECONCILED requires active allocation sum == payment.amount + explicit actor.
Idempotent repeat returns existing reconciled payment.

### ADR-16 — External ID permanence

Provider/bank/API external IDs remain historically unique after VOID.

### ADR-17 — Due-date automation deferred

No auto due-date until canonical payment-term clock exists in repository.

---

## 19. Implementation Slicing (Recommended)

| Slice | Scope | Rationale |
|-------|-------|-----------|
| **v1.9.2A** | Payment outbox + worker + Option B immediate sync integration + metrics + tests | **Smallest safe first PR** — closes durability gap |
| **v1.9.2B** | Allocation void + payment void + RBAC PolicyVoid + audit | Depends on finality rules; independent of outbox |
| **v1.9.2C** | Reconciliation hardening + idempotent repeat + post-reconcile locks | Builds on stable allocation model |
| **v1.9.2D** (optional) | Import/API idempotent create | After void/reconcile stable |

**First implementation PR:** v1.9.2A only.

---

## 20. CI / Test Matrix (implementation phase)

Extend existing gates — do not replace.

**Required scenarios for v1.9.2A:**

| ID | Scenario | Expected |
|----|----------|----------|
| F1 | Obligation PAID tx rolls back | No outbox row |
| F2 | PAID commits; billing down | Outbox PENDING; worker retries |
| F3 | Duplicate delivery after billing success | Idempotent; mark published |
| F4 | Concurrent workers claim queue | SKIP LOCKED + lease → safe single owner |
| F5 | Billing already PAID | Published success |
| F6 | Billing already CLOSED + obligation PAID | **ALREADY_SATISFIED**; mark outbox PUBLISHED; **no register mutation** |
| F7 | Invalid/missing obligation | FAILED / alert |
| F8 | Poison event | Backoff → FAILED after max attempts |

**Required idempotency test:**

```
OUTBOX_DUPLICATE_INSERT =
  only one row for (tenant_id, payment_obligation.paid, obligation_id)
```

| Gate | New scenarios |
|------|---------------|
| `freight-payments-core-integration` | Outbox atomic insert; worker delivery; F1–F8; `OUTBOX_DUPLICATE_INSERT` |
| `freight-billing-closing-integration` | F6 CLOSED + obligation PAID → ALREADY_SATISFIED |
| `backend-go-check payment-service` | Worker unit tests |

---

## 21. References

- `docs/engineering/FREIGHT_PAYMENTS_RECONCILIATION_v1.9_ARCHITECTURE.md` (v1.9.0 design + v1.9.1 addendum)
- `docs/SHIPMENT_STATUS_OUTBOX.md`
- Migration `000045_freight_payments_core_v1.9.1.up.sql`
- Merged PR #27 @ `0f35e33`

---

## 22. Open Questions

| ID | Question | Status |
|----|----------|--------|
| OQ-001 | Legal payment-term clock start | Open (inherited) |
| OQ-002 | Billing CLOSED + outbox delivery semantics | **RESOLVED** — CLOSED + canonical obligation PAID = **ALREADY_SATISFIED** / delivery success; mark outbox PUBLISHED; no register mutation |
| OQ-003 | One payment → many obligations integration test | Add during 2B/2C |

---

**Architecture freeze complete.** Ready for v1.9.2A implementation branch.

# ADR: Autopilot 0.3C2 — Launch lifecycle and budget settlement (canonical contract)

## Status

**Proposed (R6A-R7).** R6A-R6 contract plus recovery lease / poll errata **AP03C2-R6A-R6-001..003**. Pending Agent J re-review. **R6B MOCK_ONLY forbidden** until architecture sign-off.

PR #322 product baseline: `d1425269`. Branch: `feat/bintrans-autopilot-budget-v0.3c2`.

---

## 1. Agent J findings — R6A-R5 (R6A-R6 response)

| ID | Sev | Evidence (R5 review) | Required fix (Controller) | Verification |
|----|-----|----------------------|---------------------------|--------------|
| **R5-001** | MEDIUM | BT-01a/b only `LAUNCH_CONFIRMED`; LT-14 + known cents dead-end | BT-01a/b for `{CONFIRMED, EXTERNAL_AGENT_CANCELLED}`; **BT-03c**; BT-02 `delta`; BT-04 signal only; §10.1 orphan fn | RT-16a/b, RT-17c, RT-33 |
| **R5-002** | MEDIUM | RT-19 vs `budget.py` importing `cursor_api`; method patch bypass | **R6B mandatory:** extract `CursorApiError` from `budget.py`; scoped launch import graph; RT-19 launch modules; **RT-19b** AST | RT-19, RT-19b, RT-20, RT-31 |
| **R5-003** | MEDIUM | §7 missing confirm equality, body CHECK, global UNIQUE | `agent_id = planned_agent_id`; immutable after confirm; CHECK + UNIQUE(`agent_id`), UNIQUE(`run_id`) | RT-41, RT-36, RT-38 |
| **R5-004** | MEDIUM | §9 missing LT-07/12/13; race/global-block gaps; RT-37 wrong | Crash rows; **RT-42..45**; RT-37 cross-tenant read | RT-42, RT-43, RT-44, RT-45, RT-37 |
| **R5-005** | LOW | When `outbound_attempt_count` increments; seed mode | **`launch_protocol_report_outbound_result`** once per transport return **before** LT-06; default **`DISABLED`**, `MOCK_ONLY` test-only | RT-44 |
| **R5-006** | LOW | Admin EXECUTE on `escalate_manual`; superuser false PASS | Remove admin **E** on escalate; REVOKE PUBLIC + capability GRANT (§10); RT-18/32 non-superuser LOGIN only | RT-18, RT-32 |
| **R5-007** | LOW | LT-09/10 lock wording; LA-01 vague; audit DDL | Lock **1→2→3+fence** on LT-09/10; LA-01 eligibility; audit appendix; flag writes under gate; **global** reserve block | RT-43, RT-40 |
| **R5-008** | LOW | Missing 0.3D / paid / follow-up / dispatcher scope | §13 boundaries; dispatcher `create_agent` out of scope until retired | doc + launch import graph |

### 1.1 Agent J findings — R6A-R6 (R6A-R7 errata)

| ID | Sev | Evidence | Required fix | Verification |
|----|-----|----------|--------------|--------------|
| **R6-001** | **BLOCKING** | LA-01 omitted stale **`INTENT`**; no server **`poll_count`**; LT-07 unreachable from lease rules | §5.1 LA-01 eligibility incl. crash-before-send; §5.2 **`launch_protocol_recovery_poll_attempt`**; LA-02; matrix + RT-07/10/15 | RT-07, RT-10, RT-15, LA-02 |
| **R6-002** | LOW | `GRANT EXECUTE … WITH SET FALSE` invalid in PostgreSQL | **`REVOKE ALL FROM PUBLIC`**; **`GRANT EXECUTE TO` capability role**; LOGIN **`GRANT role TO login`**; enforce via **`session_user` + `pg_has_role`**; app must not **`SET ROLE`** | RT-18, RT-32 |
| **R6-003** | LOW | §12.2 implied every RT must map 1:1 to §8 Test column | Cross-cutting RT (INV-2, ACL, race) may be satisfied via §9 or §12 traceability | §12.2 |

---

## 2. Core invariants

| ID | Invariant |
|----|-----------|
| **INV-1** | Durable `launch_protocol_record_intent` → `launch_intents` row for `(tenant_id, launch_id)`. |
| **INV-2 (RELEASE_AFTER_INTENT)** | **Runtime `budget_protocol_release` forbidden** if **any** `launch_intents` row exists — **regardless of `agent_id` / `run_id`**. Applies only to PG-gated runtime path (legacy dispatcher outside gate until retired — §13). |
| **INV-3** | Runtime release allowed only when **no** launch row and reservation `RESERVED`. |
| **INV-4** | Budget **`RELEASED`** only via internal **`_release_unused_reserve`** inside terminal launch definer functions (LT-02, LT-06, LT-07, LT-12). **No standalone EXECUTE** on release helper. |
| **INV-5** | **`COMMITTED` / `OVERRUN` / `usage_unknown` flag** only via **`budget_reconciler_apply_usage`**. |
| **INV-6** | `mark_send_started` committed before transport (step 6). |
| **INV-7** | No external/mock transport I/O inside gate transaction (steps 1–5). |
| **INV-8** | GET 404 ≠ absence; 409 requires GET correlate; timeout/5xx → UNKNOWN. |
| **INV-9** | No automatic Create retry after ambiguous send. |
| **INV-10** | Post-settlement money changes only via reconciler idempotent re-apply (BT-02/BT-03/BT-03c). |
| **INV-11** | **`autopilot_launch_mode=MOCK_ONLY`** required for `proof_class=MOCK_TRUSTED` on reject/settlement; production mode **rejects** MOCK proofs (fail-closed). |
| **INV-12** | After successful confirm (`LAUNCH_CONFIRMED` or terminal paths that set `agent_id`/`run_id`), **`agent_id` and `run_id` immutable** (trigger + reconciler identity match). |
| **INV-13** | Recovery mutations on a launch require **valid lease fence** (matching token, unexpired holder). Superseded holder **must not** change launch state (RT-15). |
| **INV-14** | **`recovery_poll_count`** is authoritative in PostgreSQL only; Python worker local counters are **not** source of truth (LA-02). |
| **INV-15** | Stale **`SEND_STARTED`** after ambiguous send: recovery may **LT-08 / poll / LT-09 / LT-10** only — **never** automatic second Create (INV-9). Budget stays **`RESERVED`** through **`LAUNCH_UNKNOWN`** / **`MANUAL`**. |

---

## 3. Launch vs budget lifecycles

### 3.1 Launch states

`INTENT`, `SEND_STARTED`, `LAUNCH_UNKNOWN`, `MANUAL_RECONCILIATION_REQUIRED`, `LAUNCH_CONFIRMED`, `LAUNCH_REJECTED`, `CANCELLED_BEFORE_SEND`, `EXTERNAL_AGENT_CANCELLED`.

(No new states in R6A-R6.)

### 3.2 Budget states / flags

| Field | Meaning |
|-------|---------|
| `RESERVED` | Hold |
| `RELEASED` | Unused reserve returned (**not** spend proof) |
| `COMMITTED` | Verified charge ≤ reserve (or verified zero) |
| `OVERRUN` | Verified charge > reserve |
| `usage_unknown` | Spend unverified — hold reserve, no timeout release |

### 3.3 Settlement (distinct from release)

| Operation | Function | Effect |
|-----------|----------|--------|
| Release unused reserve | `_release_unused_reserve` (internal) | `RELEASED` |
| Commit known spend | `budget_reconciler_apply_usage` | `COMMITTED` or `OVERRUN` |
| Mark unknown spend | same, `p_verification_outcome=UNKNOWN` | `usage_unknown=1`, stay `RESERVED` |
| Verified zero (R6B) | same, mock money 0 + `MOCK_TRUSTED` | `COMMITTED` at 0 |
| Verified zero (production) | terminal run + money source + window | future ADR |
| Late charge (committed) | BT-02 | idempotent re-apply |
| Late charge (overrun) | BT-03 / **BT-03c** | idempotent re-apply |
| Unknown ≠ zero | tokens-only / missing | **must not** commit |

**Launch states eligible for reconciler commit (BT-01a/b):** `LAUNCH_CONFIRMED` **or** `EXTERNAL_AGENT_CANCELLED` (reserve still held after LT-14).

### 3.4 Post-RELEASED orphan spend (residual risk)

If admin attestation on LT-12 was wrong and spend appears **after** `_release_unused_reserve` (`RELEASED`):

- Reconciler **cannot** apply (launch terminal `LAUNCH_REJECTED`, budget not `RESERVED`).
- **`budget_orphan_spend_signal`** records operator-visible audit — **not** automatic settlement, **not** ledger commit (BT-04).
- **Residual risk:** late verified spend after `RELEASED` remains **outside** automated ledger until operator runbook (manual correction / future ADR). BT-04 prevents silent ignore only.

---

## 4. Definite rejection — LT-06 (R4-001, R5-005)

**`agent_created=false` is not a Cursor API field** and **is not sufficient** to prove absence of an agent in production.

### 4.1 Proof model

| Field (PG column) | Purpose |
|-------------------|---------|
| `outbound_attempt_count` | Incremented in PG when transport returns (see §4.4) |
| `last_outbound_proof_class` | Enum stored by adapter in same tx as state change |
| `last_outbound_proof_digest` | SHA-256 of canonical proof blob (non-secret) |

| proof_class (R6B) | Allowed when | Meaning |
|-------------------|----------------|---------|
| **`MOCK_TRUSTED`** | `autopilot_launch_mode=MOCK_ONLY` | Mock transport attestation; **not** external truth |
| **`DEFINITE_HTTP_REJECT_V1`** | Future real mode + ADR amendment | Whitelist HTTP + **independent** evidence policy TBD |
| **(other)** | — | **Reject** transition (fail-closed) |

### 4.2 LT-06 preconditions (all required)

1. Launch `SEND_STARTED`.
2. **`outbound_attempt_count = 1`** after exactly one **`launch_protocol_report_outbound_result`** for this send (§4.4). If `count ≠ 1` → **no LT-06** (RT-44).
3. Synchronous response class ∈ `{400 validation_error, 400 repository_access, 401, 403}`.
4. **`proof_class=MOCK_TRUSTED`** and mode **`MOCK_ONLY`** (INV-11).
5. Proof blob includes mock fields **`http_status`**, **`error_code`**, **`agent_created_claim=false`** — claim alone **insufficient** without (2–4).
6. Atomic: `launch_protocol_reject_definite` → `LAUNCH_REJECTED` + `_release_unused_reserve`.

If (3) holds but (4) fails → **LT-05** UNKNOWN, reserve held.

**Negative paths (RT-44):** wrong HTTP class, `count≠1`, wrong proof_class, mode≠MOCK_ONLY, 409 ambiguous → **not** LT-06.

### 4.3 Manual LT-12

Operator attestation + audit; **`proof_class=OPERATOR_ATTESTATION_V1`**; still **no** production auto-release without mode policy.

### 4.4 Outbound attempt count (normative)

| Step | Rule |
|------|------|
| After transport returns (success or classified body) | Adapter calls **`launch_protocol_report_outbound_result`** **once** per Create attempt (in gate tx **before** LT-04/LT-05/LT-06). |
| LT-06 | Requires prior report with **`outbound_attempt_count = 1`**. |
| INV-9 | No second Create POST; PG does not observe transport-internal retries — documented limitation. |

---

## 5. PostgreSQL lock order & idempotency

**Lock order (all mutating protocol paths):**

```text
1. _budget_lock_global_gate()
2. budget_reservations FOR UPDATE (tenant_id, launch_id)
3. launch_intents FOR UPDATE if present
4. recovery lease fence (if recovery role and path requires lease)
5. validate (state, session_user role, identity rules)
6. COMMIT
7. transport I/O (step 6 — outside gate tx for LT-03 onward)
```

Short form: **`gate → reservation → launch → fence → COMMIT → I/O`**.

**record_intent:** `ON CONFLICT (tenant_id, launch_id) DO NOTHING` + SELECT; **`planned_agent_id` immutable** after insert.

**Recovery fence:** `recovery_lease_holder = session_user`, `recovery_lease_token` **equals** caller token, `now() < recovery_lease_expires_at`.

**Launch control flags** (table `autopilot_launch_control`, R6B):

| Flag | Default (migration) | LT-01/LT-03 require |
|------|---------------------|---------------------|
| `autopilot_launch_mode` | **`DISABLED`** | **`MOCK_ONLY`** only in **isolated test DB** fixture / explicit test seed — never production default |
| `autopilot_paid_launch_enabled` | **`false`** | **`false`** (`PAID_AUTOMATION_ENABLED=0`) |
| `autopilot_estop` | **`false`** | **`false`** (not emergency-stopped) |

Flag writes: migration owner / break-glass only; **mutations under global gate** where applicable (R6B) to reduce estop TOCTOU vs reserve.

**Reserve policy when launch unknown (global fail-closed):**

Any row in `LAUNCH_UNKNOWN` or `MANUAL_RECONCILIATION_REQUIRED` causes **`budget_global_safety_block_reason()`** → **block new `budget_protocol_reserve` for all tenants** until cleared. Existing holds unchanged. **Tradeoff:** one stuck launch blocks global new reserves (RT-43).

### 5.1 Recovery lease — LA-01 (R6-001)

**Purpose:** single active recovery worker per `(tenant_id, launch_id)`; fencing token invalidates lagging workers after lease transfer.

**Eligible launch states** (must hold **`RESERVED`** budget; no release on lease alone):

| State | Stale condition | Typical next LT (after lease) |
|-------|-----------------|-------------------------------|
| **`INTENT`** | `now() > intent_recorded_at + intent_stale_ttl` (adapter crash **before** LT-03 / `SEND_STARTED`) | **LT-07** cancel, or adapter retry LT-02/LT-03 if worker not recovery |
| **`SEND_STARTED`** | `now() > send_started_at + send_stale_ttl` | **LT-08** → LA-02 poll loop → LT-09/10; **no LT-03** retry (INV-9, INV-15) |
| **`LAUNCH_UNKNOWN`** | any (poll / confirm / escalate) | LA-02, LT-09, LT-10 |

**Stale lease** (re-acquire allowed): `recovery_lease_holder IS NULL` **or** `now() >= recovery_lease_expires_at` **or** holder session no longer valid.

**Active lease** (others blocked): `recovery_lease_holder IS NOT NULL` **and** `now() < recovery_lease_expires_at`.

**`launch_protocol_recovery_lease_acquire`** (LA-01), one transaction:

1. Resolve tenant from **`p_tenant_id`** + recovery role (`session_user` member of recovery capability role).
2. **`launch_intents` FOR UPDATE** where `(tenant_id, launch_id)` — row lock excludes concurrent second acquirer (two workers cannot both succeed).
3. If state / stale predicate not eligible → **`not_eligible`**.
4. If active lease held by **another** holder → **`lease_held_by_other`** (no state change).
5. Else atomically set **`recovery_lease_token = gen_random_uuid()`** (new fencing token), **`recovery_lease_holder = session_user`**, **`recovery_lease_expires_at = now() + lease_ttl`**, return token to caller.

**Fence on mutating recovery paths** (LT-07, LT-08, LT-09, LT-10, LA-02): caller must pass **`p_lease_token`** equal to stored token, holder = `session_user`, lease unexpired. Mismatch → **`invalid_lease`** (old owner cannot mutate after transfer — RT-15).

**Lock order for LA-01:** `gate → launch row FOR UPDATE` only (reservation lock added on mutating LT paths per §5).

### 5.2 Recovery poll counter — LA-02 (R6-001)

**Column:** `launch_intents.recovery_poll_count` (non-null, default 0).

**`launch_protocol_recovery_poll_attempt`** (LA-02) — **server-owned** poll step; worker **must** call this instead of incrementing locally.

Single transaction, in order:

1. Tenant binding + **`pg_has_role(session_user, recovery, member)`**.
2. **`gate → reservation FOR UPDATE → launch FOR UPDATE`**.
3. Valid lease fence (§5.1).
4. Launch state must be **`LAUNCH_UNKNOWN`** (poll loop applies after LT-05/LT-08).
5. Atomically **`recovery_poll_count = recovery_poll_count + 1`** (exactly once per successful call).
6. Append **`recovery_poll_audit`** row (`tenant_id`, `launch_id`, `poll_count`, `session_user`, `created_at`).
7. Return **`(poll_count, decision)`** where:
   - **`CONTINUE`** if `poll_count < max_recovery_polls` (config table / `autopilot_launch_control`, default **24**, same knob as LT-10),
   - **`MANUAL`** if `poll_count >= max_recovery_polls` → worker calls **LT-10** in separate tx,
   - **`STOP`** if identity / state no longer allows poll (e.g. confirmed elsewhere) — no increment (step 5 skipped).

**Stale worker:** wrong or expired **`p_lease_token`** → **`invalid_lease`**, **no** increment (step 5 not reached).

**LT-10 precondition:** escalate only after LA-02 returned **`MANUAL`**, or immediate **`identity_mismatch`** proof on poll correlate — not on worker-local count.

**LT-13 reopen:** resets **`recovery_poll_count = 0`** (and audit reason) when returning to **`LAUNCH_UNKNOWN`**.

Config **`max_recovery_polls`** must match RT-10 expectations (default 24).

---

## 6. Tenancy (R4-008, R3-006, R5-004)

**Composite PK** `(tenant_id, launch_id)` on: `budget_reservations`, `budget_verified_usage`, `launch_intents`.

**Same `launch_id` across different tenants:** **allowed** (independent rows per tenant).

**Runtime** functions: tenant from **`session_user`** → `tenant_principals` (no trusted `p_tenant_id`).

**Adapter/recovery/admin/reconciler:** `p_tenant_id` + `p_launch_id`; wrong tenant → **`not_found`** (no cross-tenant leakage). **RT-37:** two tenants reserve same `launch_id` → both succeed; tenant B cannot read tenant A's row.

**0008 down guard:** refuse downgrade if duplicate `launch_id` across tenants would violate new constraints incorrectly.

---

## 7. Identity (R4-008, R5-003)

### 7.1 Confirm binding (LT-04, LT-09, LT-11)

**`launch_protocol_confirm`** (adapter LT-04, recovery LT-09) and **`launch_protocol_manual_confirm`** (LT-11) require:

1. **`p_agent_id = planned_agent_id`** (else **`identity_conflict`**).
2. **`p_run_id`** from List Runs rule (§7.3) and **global uniqueness** (§7.2).
3. **Idempotent confirm:** repeat with same ids → success no-op; different ids after first confirm → **`identity_conflict`**.

### 7.2 Schema constraints (R6B migration)

| Constraint | Purpose |
|------------|---------|
| **`UNIQUE (agent_id)`** global | One launch row per Cursor agent |
| **`UNIQUE (run_id)`** global | One launch row per run |
| **`CHECK (create_body_hash IS NOT NULL AND create_body_version IS NOT NULL)`** on insert | Canonical create body fingerprint required |
| Trigger **`launch_intents_immutable_ids`** | Forbid UPDATE changing non-NULL `planned_agent_id`/`agent_id`/`run_id`; **allow** NULL→value **once** on first confirm only |

After confirm: **INV-12** — ids immutable.

### 7.3 Run binding

List Runs, paginate all pages; select **oldest** run with `created_at >= send_started_at - clock_skew_seconds` (120).

### 7.4 Lease and poll columns

`recovery_lease_holder`, `recovery_lease_expires_at`, `recovery_lease_token` (uuid, **exact match** on recovery calls), **`recovery_poll_count`** (server-owned via LA-02).

### 7.5 Reconciler identity

**`budget_reconciler_apply_usage`:** `p_agent_id` / `p_run_id` must match launch row; mismatch → deny (RT-36).

---

## 8. Canonical transition table (normative)

**B-effect:** `H`=RESERVED hold, `R`=RELEASED, `—`=no budget state change (launch-only).

| ID | From | To | Actor | External proof | SQL function | Tx | Lock | B | Recovery | Test |
|----|------|-----|-------|----------------|--------------|-----|------|---|----------|------|
| **LT-01** | NOT_RECORDED | INTENT | Adapter | Gate §5 | `launch_protocol_record_intent` | 1–5 | 1→2→3 | H | Retry LT-01 | RT-01 |
| **LT-02** | INTENT | CANCELLED_BEFORE_SEND | Adapter | send NULL | `launch_protocol_cancel_before_send` (includes internal release) | 1–5 | 1→2→3 | R | Safe | RT-02a |
| **LT-03** | INTENT | SEND_STARTED | Adapter | Flags §5 | `launch_protocol_mark_send_started` | 1–5; 6 transport | 1→2→3 | H | LT-08 | RT-03 |
| **LT-04** | SEND_STARTED | LAUNCH_CONFIRMED | Adapter | GET OK + correlate; §7.1 | `launch_protocol_confirm` | 1–5 | 1→2→3 | H | LT-09 | RT-04, RT-41 |
| **LT-05** | SEND_STARTED | LAUNCH_UNKNOWN | Adapter | Ambiguous / 409 / unclassified | `launch_protocol_mark_unknown` | 1–5 | 1→2→3 | H | LT-08–10 | RT-05 |
| **LT-06** | SEND_STARTED | LAUNCH_REJECTED | Adapter | §4.2 proof | `launch_protocol_reject_definite` | 1–5 atomic | 1→2→3 | R | — | RT-06, RT-44 |
| **LT-07** | INTENT stale | CANCELLED_BEFORE_SEND | Recovery | TTL + **LA-01** lease | `launch_protocol_cancel_before_send` | 1–5 | 1→2→3→fence | R | LA-01 then sweep | RT-07 |
| **LT-08** | SEND_STARTED stale | LAUNCH_UNKNOWN | Recovery | TTL + **LA-01** lease | `launch_protocol_mark_unknown` | 1–5 | 1→2→3→fence | H | LA-02 poll | RT-08 |
| **LT-09** | LAUNCH_UNKNOWN | LAUNCH_CONFIRMED | Recovery | GET+correlate; §7.1; lease | `launch_protocol_confirm` | 1–5 | 1→2→3→fence | H | — | RT-09, RT-41 |
| **LT-10** | LAUNCH_UNKNOWN | MANUAL | Recovery | LA-02 **`MANUAL`** **or** mismatch | `launch_protocol_escalate_manual` | 1–5 | 1→2→3→fence | H | Operator | RT-10 |
| **LT-11** | MANUAL | LAUNCH_CONFIRMED | Admin | One-shot audit; §7.1 | `launch_protocol_manual_confirm` (audit+confirm atomic) | 1–5 | 1→2→3 | H | — | RT-11, RT-41 |
| **LT-12** | MANUAL | LAUNCH_REJECTED | Admin | Audit + attestation | `launch_protocol_manual_reject` | 1–5 atomic | 1→2→3 | R | BT-04 if wrong | RT-12 |
| **LT-13** | MANUAL | LAUNCH_UNKNOWN | Admin | Audit reopen | `launch_protocol_reopen_unknown` (resets poll counters) | 1–5 | 1→2→3 | H | LT-09 | RT-13 |
| **LT-14** | LAUNCH_CONFIRMED | EXTERNAL_AGENT_CANCELLED | Adapter | Mock cancel | `launch_protocol_mark_external_cancelled` | 1–5 | 1→2→3 | H | BT-01a/b/c | RT-14, RT-16a/b |

**Auxiliary (not launch state change):**

| ID | From | To | Actor | SQL | Lock | Test |
|----|------|-----|-------|-----|------|------|
| **LA-01** | `{INTENT stale, SEND_STARTED stale, LAUNCH_UNKNOWN}` | lease held | Recovery | `launch_protocol_recovery_lease_acquire` | launch **FOR UPDATE** (§5.1) | RT-15 |
| **LA-02** | `LAUNCH_UNKNOWN` + valid lease | poll + audit | Recovery | `launch_protocol_recovery_poll_attempt` | gate→2→3→fence | RT-10, RT-46 |

**LA-01 / LA-02:** see §5.1–5.2. **`max_recovery_polls`** default **24** (config + LT-10 + RT-10).

### 8.1 Budget transitions

| ID | From | To | Actor | Preconditions | SQL | Lock | Test |
|----|------|-----|-------|---------------|-----|------|------|
| **BT-01a** | RESERVED, launch **CONFIRMED or EXTERNAL_CANCELLED** | COMMITTED | Reconciler | Known cents ≤ reserve; identity match; proof_class valid | `budget_reconciler_apply_usage` | gate→2→3 | RT-16a |
| **BT-01b** | RESERVED, launch **CONFIRMED or EXTERNAL_CANCELLED** | OVERRUN | Reconciler | Known cents > reserve; identity match | same | gate→2→3 | RT-16b |
| **BT-01c** | RESERVED, launch CONFIRMED or EXTERNAL_CANCELLED | usage_unknown | Reconciler | UNKNOWN verification | same | gate→2→3 | RT-16c |
| **BT-02** | COMMITTED | COMMITTED (updated) | Reconciler | Late charge: **`delta_cents = new_cents - committed_cents`**, apply idempotently | idempotent re-apply | gate→2→3 | RT-17a |
| **BT-03** | COMMITTED | OVERRUN | Reconciler | Late charge pushes total > reserve | idempotent re-apply | gate→2→3 | RT-17b |
| **BT-03c** | OVERRUN | OVERRUN (updated) | Reconciler | Additional verified spend after OVERRUN; idempotent total update | idempotent re-apply | gate→2→3 | RT-17c |
| **BT-04** | RELEASED + detected orphan spend | **(audit signal only)** | Operator / tooling | Post-LT-12 wrong attestation; **no auto commit** | `budget_orphan_spend_signal` | — | RT-33 |

---

## 9. Crash / recovery matrix

| Event | Launch after crash | Budget | Next | Test |
|-------|-------------------|--------|------|------|
| Before LT-01 | NOT_RECORDED | H | LT-01 | RT-01 |
| After LT-01 | INTENT | H | LT-02/03; if crash stale → **LA-01→LT-07** | RT-02a, RT-07 |
| Adapter crash before LT-03 (stale INTENT) | INTENT | H | **LA-01→LT-07** (release) | RT-07 |
| After LT-03 no response | SEND_STARTED | H | LA-01→LT-08→LA-02→LT-09/10 | RT-08, RT-05, RT-46 |
| After LT-04 partial | SEND_STARTED/UNKNOWN | H | LT-09 | RT-04 |
| LT-05 lost response | UNKNOWN | H | LT-09/10 | RT-05 |
| GET 404 | UNKNOWN | H | LA-02…→LT-10 | RT-10, RT-46 |
| Crash before LA-02 commit | UNKNOWN | H | retry LA-02 (same count) | RT-46 |
| Crash after LA-02 commit | UNKNOWN | H | next poll or LT-09/10 | RT-46, RT-10 |
| POST 409 | UNKNOWN | H | LT-09/10 | RT-05 |
| LT-06 definite reject | REJECTED | R | — | RT-06 |
| **LT-07 stale intent cancel** | CANCELLED_BEFORE_SEND | R | — | RT-07 |
| LT-11 crash mid-tx | MANUAL | H | retry LT-11 | RT-11 |
| **LT-12 manual reject** | LAUNCH_REJECTED | R | BT-04 if spend later | RT-12, RT-33 |
| **LT-13 reopen unknown** | LAUNCH_UNKNOWN | H | LT-09/10 | RT-13 |
| LT-14 cancel | EXTERNAL_CANCELLED | H | BT-01a/b/c | RT-14, RT-16a/b |
| Lease lost mid-recovery | prior state | H | re-**LA-01**; old token **invalid_lease** | RT-15 |
| **Concurrent runtime release ∥ LT-01** | one wins | H or block | INV-2 | **RT-42** |
| Identity mismatch | UNKNOWN→MANUAL | H | LT-10 | RT-10 |
| Global UNKNOWN/MANUAL present | — | block new reserve | wait recovery | RT-43 |

---

## 10. Permissions (R4-003, R5-006)

**Definer owner:** `autopilot_queue_definer_owner` (NOLOGIN, not superuser, no BYPASSRLS).

**All protocol functions:** `REVOKE ALL ON FUNCTION … FROM PUBLIC`; then **`GRANT EXECUTE ON FUNCTION … TO <capability_role>`** for each named capability role only. (PostgreSQL has **no** `WITH SET FALSE` on `GRANT EXECUTE`; privilege boundary is **`REVOKE FROM PUBLIC`** + narrow grants + in-function checks.)

**Role membership (R6-002):** capability roles **`autopilot_adapter`**, **`autopilot_recovery`**, **`autopilot_reconciler`**, **`autopilot_admin`** are **NOLOGIN**. Protocol LOGIN roles receive **`GRANT capability_role TO login_role`**. Migrations apply grants; applications connect as LOGIN and **do not** use **`SET ROLE`** / **`SET SESSION AUTHORIZATION`**.

**Caller identity inside DEFINER:** use **`session_user`** + **`pg_has_role(session_user, '<capability_role>', 'member')`**, never `current_user` alone.

**Forbidden:** application **`SET ROLE`** / **`SET SESSION AUTHORIZATION`**; **`CREATE` on schema `autopilot_queue`** except migration owner.

**Role disjointness:** `{adapter, recovery, reconciler, admin}` pairwise disjoint; **tenant runtime LOGINs** disjoint from those four. Tests **RT-18**, **RT-32** use **non-superuser** DSN only (superuser may false-pass).

**`_release_unused_reserve`:** **no EXECUTE grant**; callable only from other SECURITY DEFINER wrappers.

### 10.1 Function × role (EXECUTE)

| Function | Runtime | Adapter | Recovery | Reconciler | Admin |
|----------|---------|---------|----------|------------|-------|
| `budget_protocol_reserve` | E | — | — | — | — |
| `budget_protocol_release` | E | — | — | — | — |
| `launch_protocol_record_intent` | — | E | D | — | — |
| `launch_protocol_mark_send_started` | — | E | D | — | — |
| `launch_protocol_report_outbound_result` | — | E | D | — | — |
| `launch_protocol_mark_unknown` | — | E† | E* | — | — |
| `launch_protocol_confirm` | — | E (SEND_STARTED) | E* (UNKNOWN+lease) | — | via manual wrapper |
| `launch_protocol_cancel_before_send` | — | E | E* | — | — |
| `launch_protocol_reject_definite` | — | E (SEND_STARTED) | D | — | D |
| `launch_protocol_manual_confirm` | — | — | — | — | E |
| `launch_protocol_manual_reject` | — | — | — | — | E |
| `launch_protocol_escalate_manual` | — | — | E* | — | **—** |
| `launch_protocol_reopen_unknown` | — | — | — | — | E |
| `launch_protocol_mark_external_cancelled` | — | E | D | — | — |
| `launch_protocol_recovery_lease_acquire` | — | — | E* | — | — |
| `launch_protocol_recovery_poll_attempt` | — | — | E* | — | — |
| `budget_reconciler_apply_usage` | — | — | — | E | — |
| `budget_orphan_spend_signal` | — | — | — | E | E |

\* Requires valid lease fence when role=recovery.  
† Adapter: from `SEND_STARTED` only. Recovery: stale sweep paths + lease.

**Shared-function rules:** documented preconditions enforced **inside** SQL (state + role); violation → **`invalid_caller_or_state`**.

Test **RT-32:** full ACL matrix vs `pg_proc` catalog (R6B), non-superuser.

### 10.2 Audit columns (R6B DDL appendix)

| Table / use | Columns (non-secret) |
|-------------|----------------------|
| `manual_reconciliation_audit` | `tenant_id`, `launch_id`, `actor_session_user`, `action`, `reason`, `proof_digest`, `created_at` |
| `recovery_poll_audit` | `tenant_id`, `launch_id`, `poll_count`, `actor_session_user`, `created_at` |
| LT-11 / LT-12 / LT-13 | append row in same tx as state change |
| `budget_orphan_spend_signal` | `tenant_id`, `launch_id`, `detected_cents`, `source`, `created_at`, `acknowledged_at` |

---

## 11. MOCK_ONLY isolation (R4-002, R5-002)

### 11.1 R6B mandatory import refactor (plan)

| Artifact | Requirement |
|----------|-------------|
| **`CursorApiError`** | Extract to **`tools/bintrans_autopilot/cursor_errors.py`** (or equivalent) **without** importing `cursor_api` from launch packages |
| **`budget.py`** | May keep budget + cursor coupling; **launch package must not import `budget.py`** |
| **Real Cursor transport** | **Forbidden** in R6B; no live HTTP, no credentials |

### 11.2 Allowed import graph (launch path)

```text
tools/bintrans_autopilot/launch/          → mock transport, protocol adapter, cursor_errors
tools/bintrans_autopilot/launch/recovery/ → same boundary
MUST NOT import: bintrans_autopilot.cursor_api, budget.py (until explicitly split)
MAY import: stdlib, psycopg, shared internal modules on allowlist (R6B code review)
```

Legacy **`dispatcher` / `create_agent`** paths: **out of scope** for this gate; must not appear in launch import graph (§13).

### 11.3 Enforcement

| Rule | R6B enforcement |
|------|-----------------|
| Transport type | **`type(transport) is MockCursorTransport`** (exact); class **frozen**; **`MockCursorTransport.__init_subclass__` raises** |
| No method patch bypass | **`unittest.mock.patch` on transport methods** → test failure (RT-31) |
| No production transport injection | Constructor rejects; RT-31 subclass fails |
| No env transport pick | AST (**RT-19b**): no `os.getenv`, `os.environ[...]`, `CURSOR_` reads under `tools/bintrans_autopilot/launch/` |
| No dynamic import bypass | AST deny `importlib`, `__import__`, `exec`, `eval` in launch packages |
| Forbidden HTTP client APIs | **RT-19b** AST: deny `urllib.request`, `http.client`, `httpx`, `aiohttp`, `requests`, `subprocess` in `launch/` |
| Module load (scoped) | **RT-19:** after importing **only** `tools.bintrans_autopilot.launch` package modules, **`bintrans_autopilot.cursor_api` ∉ sys.modules** (does **not** require whole-process absence of stdlib `socket`/`urllib` loaded by psycopg) |
| Network | RT-20: patch `socket.socket.connect` in launch/recovery **test** invocations only |
| Credentials | No `CURSOR_*` env reads in launch; CI without secrets |
| PG mode | INV-11; RT-30 rejects MOCK proof if mode≠MOCK_ONLY |

---

## 12. End-to-end traceability

| Finding | Invariant | Transition | SQL | Recovery | Test |
|---------|-----------|------------|-----|----------|------|
| R4-001 | INV-11 | LT-06 | reject_definite | — | RT-06, RT-30 |
| R4-002 | MOCK | LT-03 | transport | — | RT-19, RT-19b, RT-20, RT-31 |
| R4-003 | INV-4 | LT-02,06,07,12 | wrappers | — | RT-32, RT-18 |
| R4-004 | INV-5,10 | BT-01a–03c | apply_usage | — | RT-16a–c, RT-17a–c |
| R4-005 | INV-2 | deny | release guard | RT-42 | RT-02, RT-42 |
| R4-006 | doc | §8=§9 | — | LT-07,12,13 | §12.2 |
| R4-007 | global block | — | reserve | UNKNOWN/MANUAL | RT-43 |
| R4-008 | identity | LT-04 | confirm trigger | lease | RT-35, RT-41 |
| **R5-001** | INV-5,10 | BT-01a/b, BT-03c, BT-04 | apply_usage, orphan | post-RELEASED risk | RT-16a/b, RT-17c, RT-33 |
| **R5-002** | MOCK | — | import graph | — | RT-19, RT-19b, RT-20, RT-31 |
| **R5-003** | INV-12 | LT-04,09,11 | confirm, UNIQUE | — | RT-41, RT-36, RT-38 |
| **R5-004** | doc | §9 | — | LT-07,12,13 | RT-42, RT-43, RT-44, RT-45, RT-37 |
| **R5-005** | INV-9 | LT-06 | report_outbound_result | — | RT-44 |
| **R5-006** | ACL | LT-10 | escalate_manual | — | RT-18, RT-32 |
| **R5-007** | lock | LT-09,10, LA-01 | lease, audit | global block | RT-43, RT-40, RT-15 |
| **R5-008** | scope | — | dispatcher | legacy path | §13, RT-19 |
| **R6-001** | INV-13,14,15 | LA-01, LA-02, LT-07 | lease_acquire, poll_attempt | stale INTENT | RT-07, RT-10, RT-15, RT-46 |
| **R6-002** | ACL | — | GRANT/REVOKE | membership | RT-18, RT-32 |
| **R6-003** | doc | — | — | — | §12.2 |
| R3-006 | tenancy | LT-01 | composite PK | — | RT-37 |
| R3-007 | INV-12 | LT-04 | confirm | — | RT-41 |
| R3-008 | settlement | BT-01c | unknown | — | RT-39 |
| R3-009 | INV-6,7 | LT-01 | lock order | — | RT-40 |

### 12.1 Regression catalog (every LT/BT/LA has RT)

| RT | Covers |
|----|--------|
| RT-01 | LT-01 intent before transport |
| RT-02 | **INV-2** runtime release denied (any launch row) |
| RT-02a | **LT-02** cancel_before_send → RELEASED |
| RT-03 | LT-03 send_started before mock call |
| RT-04 | LT-04 confirm |
| RT-05 | LT-05 ambiguous; no second POST (RT-22) |
| RT-06 | LT-06 proof_class; 409→not LT-06 |
| RT-07 | LT-07 stale **INTENT** (LA-01 first) → RELEASED |
| RT-08 | LT-08 stale send → UNKNOWN; no second Create |
| RT-09 | LT-09 recovery confirm |
| RT-10 | LA-02 **`MANUAL`** at **`max_recovery_polls`** → LT-10; identity mismatch → MANUAL |
| RT-11 | LT-11 atomic manual confirm + audit |
| RT-12 | LT-12 manual reject |
| RT-13 | LT-13 reopen → UNKNOWN |
| RT-14 | LT-14 external cancel |
| RT-15 | LA-01 lease acquire + fence; stale INTENT eligible; superseded token denied |
| RT-46 | LA-02 server poll increment, audit, stale worker no-op; crash before/after poll |
| RT-16a/b/c | BT-01a/b/c (incl. EXTERNAL_CANCELLED for a/b) |
| RT-17a/b/c | BT-02/03/03c late charge |
| RT-18 | role disjointness (non-superuser) |
| RT-19 | launch package does not load `cursor_api` |
| RT-19b | AST forbidden APIs in `launch/` |
| RT-20 | connect denied in launch tests |
| RT-22 | INV-9 no second POST |
| RT-30 | MOCK proof rejected when mode≠MOCK_ONLY |
| RT-31 | subclass / patch MockCursorTransport rejected |
| RT-32 | ACL matrix (non-superuser) |
| RT-33 | BT-04 orphan signal (no ledger commit) |
| RT-34 | estop denies LT-03 |
| RT-35 | confirm NULL→ids once; second change blocked |
| RT-36 | reconciler identity mismatch denied |
| RT-37 | same launch_id two tenants OK; cross-tenant read → not_found |
| RT-38 | record_intent DO NOTHING idempotency |
| RT-39 | tokens-only → usage_unknown |
| RT-40 | no I/O inside gate tx (mock slow path) |
| RT-41 | confirm requires `agent_id = planned_agent_id` |
| RT-42 | concurrent `budget_protocol_release` ∥ LT-01 → INV-2 |
| RT-43 | global block new reserve when UNKNOWN/MANUAL row exists |
| RT-44 | LT-06 negatives (count, proof, class, mode) |
| RT-45 | verified zero → COMMITTED at 0 (MOCK_TRUSTED) |

**Transition test coverage:** each **LT-01..LT-14**, **LA-01**, **LA-02**, **BT-01a..BT-04**, **BT-03c** maps to ≥1 RT above.

### 12.2 Documentation completeness check (R6B script spec)

Script parses ADR §8 and §9:

- LT IDs in §8 (launch) = LT IDs referenced in §9 (excluding LA-01 / LA-02 optional auxiliary rows);
- Every LT/BT/LA row has non-empty Test column matching `RT-\d+[a-z]?` tokens present in §12.1;
- Substring **`RELEASE_AFTER_INTENT`** present (INV-2 heading);
- No duplicate `(From, To, Actor)` among LT rows;
- Every **transition-scoped** RT in §12.1 appears in ≥1 §8 Test column or §9 Test column;
- **Cross-cutting RT** (e.g. RT-02, RT-18, RT-22, RT-32, RT-42, RT-43) may satisfy coverage via §12 traceability row only — doc-check **must not** require a unique LT/BT row per such RT (R6-003).

**R6A-R7:** spec only; script not committed in this revision.

---

## 13. R6B scope and product boundaries (R5-008)

**Allowed (R6B):** migration 0008+, protocol SQL, mock transport, **`CursorApiError` extraction**, RT catalog, isolated CI DB with **`MOCK_ONLY`** seed.

**Forbidden:** real HTTP to Cursor, credentials, paid launch, merge without J approval, deploy, production, Cloud Agents, **`AUTOPILOT_ENABLED=1`**, live follow-up/steer API automation.

**0.3D audit alignment:** launch/budget protocol changes remain auditable via SQL functions + `manual_reconciliation_audit` / orphan signal tables; full 0.3D product audit scope is separate but must not contradict INV-2 release gating.

**Legacy dispatcher:** any **`create_agent`** or pre-gate release path is **out of scope** for R6B proof until retired or wrapped behind same PG protocol (smoke: not in launch import graph).

**R6B_AUTHORIZED=NO** until Agent J approves this revision (**ARCHITECTURE_APPROVED**).

---

## 14. External dependencies

**EXTERNAL_CREATE_IDEMPOTENCY=UNVERIFIED.** **EXTERNAL_USAGE_AUTHENTICATION=NOT_PROVEN.**

---

## Document history

| Rev | Notes |
|-----|-------|
| R6A-R4 | LT/BT split |
| R6A-R5 | R4-001..008; proof_class; BT split; permissions; MOCK fix; RT completeness |
| **R6A-R6** | Joint Gate R5-001..008; BT-03c; import graph; identity; crash matrix; RT-41..45, RT-17c, RT-19b |
| **R6A-R7** | R6-001 LA-01 stale INTENT + LA-02 poll_count; GRANT fix R6-002; doc-check R6-003; RT-46 |

## References

- Agent J `AP03C2-R6A-R5-001..008`, `AP03C2-R6A-R6-001..003`
- Joint Design Gate `AUTOPILOT-0.3C2-JOINT-DESIGN-GATE` on HEAD `3688961f`
- SQL `0002`, `0004`, `0005`, `0006`, `0007`

# ADR: Autopilot 0.3C2 — Launch lifecycle and budget settlement (canonical contract)

## Status

**Proposed (R6A-R5).** Pending Agent J approval. **R6B MOCK_ONLY forbidden** until sign-off.

PR #322 product baseline: `d1425269`. Branch: `feat/bintrans-autopilot-budget-v0.3c2`.

---

## 1. Agent J findings — R6A-R4 (R6A-R5 response)

| ID | Sev | Evidence (R4 review) | Required fix | Verification |
|----|-----|----------------------|--------------|--------------|
| **R4-001** | MEDIUM | `agent_created=false` is mock-only; no `proof_class`; no DB mode flag | `proof_class`, `launch_mode`, PG-stored attempt count; fail-closed outside `MOCK_TRUSTED` | RT-06, RT-30 |
| **R4-002** | MEDIUM | `isinstance` allows subclass; RT-19 unsatisfiable with psycopg | Exact type + `__init_subclass__`; RT-19 targets Cursor modules only; RT-20 network deny | RT-19, RT-20, RT-31 |
| **R4-003** | MEDIUM | Shared functions; PUBLIC EXECUTE; wrapper vs grant conflict | `session_user` checks; REVOKE PUBLIC; no standalone `release_unused_reserve` EXECUTE | RT-32, RT-18 |
| **R4-004** | MEDIUM | BT-01 triple outcome; BT-02 incomplete | Split BT rows; late charge paths; post-RELEASED orphan signal | RT-16..RT-18, RT-33 |
| **R4-005** | MEDIUM | RT-02 ≠ LT-02; missing RT-10b; R3-006..009 untested | Per-LT tests + invariant RT map | §14 |
| **R4-006** | LOW | §12.2 vs §9 mismatch; LT-15 incomplete | Align crash matrix; fix doc-check rules | §13.2 |
| **R4-007** | LOW | Gate wording; no UNKNOWN block policy | Clarify flags; reserve policy + test | RT-34 |
| **R4-008** | LOW | Trigger blocks confirm; lease/run gaps | NULL→value trigger; lease columns; run rule | RT-35 |

---

## 2. Core invariants

| ID | Invariant |
|----|-----------|
| **INV-1** | Durable `launch_protocol_record_intent` → `launch_intents` row for `(tenant_id, launch_id)`. |
| **INV-2 (RELEASE_AFTER_INTENT)** | **Runtime `budget_protocol_release` forbidden** if **any** `launch_intents` row exists — **regardless of `agent_id` / `run_id`**. |
| **INV-3** | Runtime release allowed only when **no** launch row and reservation `RESERVED`. |
| **INV-4** | Budget **`RELEASED`** only via internal **`_release_unused_reserve`** inside terminal launch definer functions (LT-02, LT-06, LT-07, LT-12). **No standalone EXECUTE** on release helper. |
| **INV-5** | **`COMMITTED` / `OVERRUN` / `usage_unknown` flag** only via **`budget_reconciler_apply_usage`**. |
| **INV-6** | `mark_send_started` committed before transport (step 6). |
| **INV-7** | No external/mock transport I/O inside gate transaction (steps 1–5). |
| **INV-8** | GET 404 ≠ absence; 409 requires GET correlate; timeout/5xx → UNKNOWN. |
| **INV-9** | No automatic Create retry after ambiguous send. |
| **INV-10** | Post-settlement money changes only via reconciler idempotent re-apply (BT-02/BT-03). |
| **INV-11** | **`autopilot_launch_mode=MOCK_ONLY`** required for `proof_class=MOCK_TRUSTED` on reject/settlement; production mode **rejects** MOCK proofs (fail-closed). |

---

## 3. Launch vs budget lifecycles

### 3.1 Launch states

`INTENT`, `SEND_STARTED`, `LAUNCH_UNKNOWN`, `MANUAL_RECONCILIATION_REQUIRED`, `LAUNCH_CONFIRMED`, `LAUNCH_REJECTED`, `CANCELLED_BEFORE_SEND`, `EXTERNAL_AGENT_CANCELLED`.

### 3.2 Budget states / flags

| Field | Meaning |
|-------|---------|
| `RESERVED` | Hold |
| `RELEASED` | Unused reserve returned (**not** spend proof) |
| `COMMITTED` | Verified charge ≤ reserve |
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
| Late charge | BT-02 / BT-03 | idempotent re-apply |
| Unknown ≠ zero | tokens-only / missing | **must not** commit |

### 3.4 Post-RELEASED orphan spend

If admin attestation on LT-12 was wrong and spend appears later: reconciler **cannot** apply (launch terminal `LAUNCH_REJECTED`). **`budget_orphan_spend_signal`** audit row + operator runbook — **no silent ignore** (BT-04 signal only, no auto commit).

---

## 4. Definite rejection — LT-06 (R4-001)

**`agent_created=false` is not a Cursor API field** and **is not sufficient** to prove absence of an agent in production.

### 4.1 Proof model

| Field (PG column) | Purpose |
|-------------------|---------|
| `outbound_attempt_count` | Incremented in PG when transport returns (not mock-only counter) |
| `last_outbound_proof_class` | Enum stored by adapter in same tx as state change |
| `last_outbound_proof_digest` | SHA-256 of canonical proof blob (non-secret) |

| proof_class (R6B) | Allowed when | Meaning |
|-------------------|----------------|---------|
| **`MOCK_TRUSTED`** | `autopilot_launch_mode=MOCK_ONLY` | Mock transport attestation; **not** external truth |
| **`DEFINITE_HTTP_REJECT_V1`** | Future real mode + ADR amendment | Whitelist HTTP + **independent** evidence policy TBD |
| **(other)** | — | **Reject** transition (fail-closed) |

### 4.2 LT-06 preconditions (all required)

1. Launch `SEND_STARTED`; **`outbound_attempt_count = 1`** (INV-9 enforced in PG).
2. Synchronous response class ∈ `{400 validation_error, 400 repository_access, 401, 403}`.
3. **`proof_class=MOCK_TRUSTED`** and mode **`MOCK_ONLY`** (INV-11).
4. Proof blob includes mock fields **`http_status`**, **`error_code`**, **`agent_created_claim=false`** — claim alone **insufficient** without (1–3).
5. Atomic: `launch_protocol_reject_definite` → `LAUNCH_REJECTED` + `_release_unused_reserve`.

If (2) holds but (3) fails → **LT-05** UNKNOWN, reserve held.

### 4.3 Manual LT-12

Operator attestation + audit; **`proof_class=OPERATOR_ATTESTATION_V1`**; still **no** production auto-release without mode policy.

---

## 5. PostgreSQL lock order & idempotency

```text
1. _budget_lock_global_gate()
2. budget_reservations FOR UPDATE (tenant_id, launch_id)
3. launch_intents FOR UPDATE if present
4. validate (state, session_user role, lease fence if recovery)
5. COMMIT
6. transport I/O
```

**record_intent:** `ON CONFLICT (tenant_id, launch_id) DO NOTHING` + SELECT; **`planned_agent_id` immutable**.

**Recovery fence:** `recovery_lease_holder = session_user`, `recovery_lease_token` **equals** caller token, `now() < recovery_lease_expires_at`.

**Launch control flags** (table `autopilot_launch_control`, R6B):

| Flag | LT-01/LT-03 require |
|------|---------------------|
| `autopilot_launch_mode` | **`MOCK_ONLY`** for R6B |
| `autopilot_paid_launch_enabled` | **`false`** (paid launches disabled) |
| `autopilot_estop` | **`false`** (not emergency-stopped) |

Writes to flags: migration owner / break-glass only (not adapter).

**Reserve policy when launch unknown:** `LAUNCH_UNKNOWN` / `MANUAL` rows contribute to **`budget_global_safety_block_reason()`** → **block new `budget_protocol_reserve` globally** until cleared (fail-closed availability). Existing holds unchanged.

---

## 6. Tenancy (R4-008, R3-006)

**Composite PK** `(tenant_id, launch_id)` on: `budget_reservations`, `budget_verified_usage`, `launch_intents`.

**Runtime** functions: tenant from **`session_user`** → `tenant_principals` (no trusted `p_tenant_id`).

**Adapter/recovery/admin/reconciler:** `p_tenant_id` + `p_launch_id`; wrong tenant → **`not_found`**.

**0008 down guard:** refuse downgrade if duplicate `launch_id` across tenants exists.

---

## 7. Identity (R4-008)

**Trigger `launch_intents_immutable_ids`:** forbid UPDATE that **changes** non-NULL `planned_agent_id`/`agent_id`/`run_id`; **allow** NULL→value on first confirm only.

**Run binding:** List Runs, paginate all pages; select **oldest** run with `created_at >= send_started_at - clock_skew_seconds` (120).

**Lease columns:** `recovery_lease_holder`, `recovery_lease_expires_at`, `recovery_lease_token` (uuid, **exact match** on recovery calls).

---

## 8. Canonical transition table (normative)

**B-effect:** `H`=RESERVED hold, `R`=RELEASED, `—`=no budget state change (launch-only).

| ID | From | To | Actor | External proof | SQL function | Tx | Lock | B | Recovery | Test |
|----|------|-----|-------|----------------|--------------|-----|------|---|----------|------|
| **LT-01** | NOT_RECORDED | INTENT | Adapter | Gate §5 | `launch_protocol_record_intent` | 1–5 | 1→2→3 | H | Retry LT-01 | RT-01 |
| **LT-02** | INTENT | CANCELLED_BEFORE_SEND | Adapter | send NULL | `launch_protocol_cancel_before_send` (includes internal release) | 1–5 | 1→2→3 | R | Safe | RT-02a |
| **LT-03** | INTENT | SEND_STARTED | Adapter | Flags §5 | `launch_protocol_mark_send_started` | 1–5; 6 transport | 1→2→3 | H | LT-08 | RT-03 |
| **LT-04** | SEND_STARTED | LAUNCH_CONFIRMED | Adapter | GET OK + correlate | `launch_protocol_confirm` | 1–5 | 1→2→3 | H | LT-09 | RT-04 |
| **LT-05** | SEND_STARTED | LAUNCH_UNKNOWN | Adapter | Ambiguous / 409 / unclassified | `launch_protocol_mark_unknown` | 1–5 | 1→2→3 | H | LT-08–10 | RT-05 |
| **LT-06** | SEND_STARTED | LAUNCH_REJECTED | Adapter | §4.2 proof | `launch_protocol_reject_definite` | 1–5 atomic | 1→2→3 | R | — | RT-06 |
| **LT-07** | INTENT stale | CANCELLED_BEFORE_SEND | Recovery | TTL + lease | `launch_protocol_cancel_before_send` | 1–5 | 1→2→3+fence | R | Sweep | RT-07 |
| **LT-08** | SEND_STARTED stale | LAUNCH_UNKNOWN | Recovery | TTL + lease | `launch_protocol_mark_unknown` | 1–5 | 1→2→3+fence | H | Poll | RT-08 |
| **LT-09** | LAUNCH_UNKNOWN | LAUNCH_CONFIRMED | Recovery | GET+correlate | `launch_protocol_confirm` | 1–5 | fence | H | — | RT-09 |
| **LT-10** | LAUNCH_UNKNOWN | MANUAL | Recovery | poll_count≥max **or** mismatch | `launch_protocol_escalate_manual` | 1–5 | fence | H | Operator | RT-10 |
| **LT-11** | MANUAL | LAUNCH_CONFIRMED | Admin | One-shot audit | `launch_protocol_manual_confirm` (audit+confirm atomic) | 1–5 | 1→2→3 | H | — | RT-11 |
| **LT-12** | MANUAL | LAUNCH_REJECTED | Admin | Audit + attestation | `launch_protocol_manual_reject` | 1–5 atomic | 1→2→3 | R | BT-04 if wrong | RT-12 |
| **LT-13** | MANUAL | LAUNCH_UNKNOWN | Admin | Audit reopen | `launch_protocol_reopen_unknown` (resets poll counters) | 1–5 | 1→2→3 | H | LT-09 | RT-13 |
| **LT-14** | LAUNCH_CONFIRMED | EXTERNAL_AGENT_CANCELLED | Adapter | Mock cancel | `launch_protocol_mark_external_cancelled` | 1–5 | 1→2→3 | H | BT-01c | RT-14 |

**Auxiliary (not launch state change):**

| ID | From | To | Actor | SQL | Lock | Test |
|----|------|-----|-------|-----|------|------|
| **LA-01** | (recovery eligible) | lease held | Recovery | `launch_protocol_recovery_lease_acquire` | row FOR UPDATE only | RT-15 |

`max_recovery_polls` config default **24** (with `unknown_escalation_ttl`).

### 8.1 Budget transitions

| ID | From | To | Actor | Preconditions | SQL | Lock | Test |
|----|------|-----|-------|---------------|-----|------|------|
| **BT-01a** | RESERVED, launch CONFIRMED | COMMITTED | Reconciler | Known cents ≤ reserve; identity match; proof_class valid | `budget_reconciler_apply_usage` | gate→2→3 | RT-16a |
| **BT-01b** | RESERVED, launch CONFIRMED | OVERRUN | Reconciler | Known cents > reserve; identity match | same | gate→2→3 | RT-16b |
| **BT-01c** | RESERVED, launch CONFIRMED or EXTERNAL_CANCELLED | usage_unknown | Reconciler | UNKNOWN verification | same | gate→2→3 | RT-16c |
| **BT-02** | COMMITTED | COMMITTED (higher) | Reconciler | Late charge ≤ reserve delta | idempotent re-apply | gate→2→3 | RT-17a |
| **BT-03** | COMMITTED | OVERRUN | Reconciler | Late charge > reserve | idempotent re-apply | gate→2→3 | RT-17b |
| **BT-04** | RELEASED + orphan signal | (audit only) | Operator | Post-LT-12 spend detected | `budget_orphan_spend_signal` | — | RT-33 |

---

## 9. Crash / recovery matrix

| Event | Launch after crash | Budget | Next | Test |
|-------|-------------------|--------|------|------|
| Before LT-01 | NOT_RECORDED | H | LT-01 | RT-01 |
| After LT-01 | INTENT | H | LT-02/03/07 | RT-02a, RT-07 |
| After LT-03 no response | SEND_STARTED | H | LT-08→09/10 | RT-08, RT-05 |
| After LT-04 partial | SEND_STARTED/UNKNOWN | H | LT-09 | RT-04 |
| LT-05 lost response | UNKNOWN | H | LT-09/10 | RT-05 |
| GET 404 | UNKNOWN | H | LT-10 | RT-10 |
| POST 409 | UNKNOWN | H | LT-09/10 | RT-05 |
| LT-06 definite reject | REJECTED | R | — | RT-06 |
| LT-11 crash mid-tx | MANUAL | H | retry LT-11 | RT-11 |
| LT-14 cancel | EXTERNAL_CANCELLED | H | BT-01c | RT-14 |
| Lease lost | — | H | LA-01 | RT-15 |
| Concurrent release vs LT-01 | — | — | one wins | RT-02 (INV-2) |
| Identity mismatch | UNKNOWN→MANUAL | H | LT-10 | RT-10 |

---

## 10. Permissions (R4-003)

**Definer owner:** `autopilot_queue_definer_owner` (NOLOGIN, not superuser, no BYPASSRLS).

**All protocol functions:** `REVOKE ALL ON FUNCTION … FROM PUBLIC`; GRANT EXECUTE to named roles only.

**Caller identity inside DEFINER:** use **`session_user`** + **`pg_has_role(session_user, role, 'member')`**, never `current_user` alone.

**Forbidden:** `SET ROLE` / `SET SESSION AUTHORIZATION` for launch/budget roles; **`CREATE` on schema `autopilot_queue`** except migration owner.

**Role disjointness:** `{adapter, recovery, reconciler, admin}` pairwise disjoint; **tenant runtime LOGINs** disjoint from those four. Test RT-18.

**`_release_unused_reserve`:** **no EXECUTE grant**; callable only from other SECURITY DEFINER wrappers.

### 10.1 Function × role (EXECUTE)

| Function | Runtime | Adapter | Recovery | Reconciler | Admin |
|----------|---------|---------|----------|------------|-------|
| `budget_protocol_reserve` | E | — | — | — | — |
| `budget_protocol_release` | E | — | — | — | — |
| `launch_protocol_record_intent` | — | E | D | — | — |
| `launch_protocol_mark_send_started` | — | E | D | — | — |
| `launch_protocol_mark_unknown` | — | E† | E* | — | — |
| `launch_protocol_confirm` | — | E (SEND_STARTED) | E* (UNKNOWN+lease) | — | via manual wrapper |
| `launch_protocol_cancel_before_send` | — | E | E* | — | — |
| `launch_protocol_reject_definite` | — | E (SEND_STARTED) | D | — | D |
| `launch_protocol_manual_confirm` | — | — | — | — | E |
| `launch_protocol_manual_reject` | — | — | — | — | E |
| `launch_protocol_escalate_manual` | — | — | E* | — | E |
| `launch_protocol_reopen_unknown` | — | — | — | — | E |
| `launch_protocol_mark_external_cancelled` | — | E | D | — | — |
| `launch_protocol_recovery_lease_acquire` | — | — | E* | — | — |
| `budget_reconciler_apply_usage` | — | — | — | E | — |

\* Requires valid lease fence when role=recovery.  
† Adapter: from `SEND_STARTED` only. Recovery: stale sweep paths + lease.

**Shared-function rules:** documented preconditions enforced **inside** SQL (state + role); violation → **`invalid_caller_or_state`**.

Test **RT-32:** full ACL matrix vs `pg_proc` catalog (R6B).

---

## 11. MOCK_ONLY isolation (R4-002)

| Rule | R6B enforcement |
|------|-----------------|
| Transport type | **`type(transport) is MockCursorTransport`** (exact); **`MockCursorTransport.__init_subclass__` raises** |
| No production transport injection | Constructor rejects; RT-31 subclass fails |
| No env transport pick | AST scan: no `os.environ` / `CURSOR_` in `tools/bintrans_autopilot/launch_*` |
| No dynamic import bypass | AST deny `importlib`, `__import__`, `exec`, `eval` in launch packages |
| Dangerous imports | RT-19: after import launch modules, **`cursor_api`, `httpx`, `requests`, `http.client`, `aiohttp` ∉ sys.modules** (stdlib `socket`/`urllib` **allowed** — loaded by psycopg) |
| Network | RT-20: patch `socket.socket.connect` in launch/recovery tests only |
| Credentials | No `CURSOR_*` env reads; CI without secrets |
| PG mode | INV-11; RT-30 rejects MOCK proof if mode≠MOCK_ONLY |

---

## 12. End-to-end traceability

| Finding | Invariant | Transition | SQL | Test |
|---------|-----------|------------|-----|------|
| R4-001 | INV-11 | LT-06 | reject_definite | RT-06, RT-30 |
| R4-002 | MOCK | LT-03 | transport | RT-19, RT-20, RT-31 |
| R4-003 | INV-4 | all release LTs | wrappers | RT-32, RT-18 |
| R4-004 | INV-5,10 | BT-01a–03 | apply_usage | RT-16a–c, RT-17a–b |
| R4-005 | INV-2 | deny | release | RT-02 |
| R5-001 | INV-2 | — | release guard | RT-02 |
| R5-002 | INV-5 | BT-01* | identity match | RT-36 |
| R3-006 | tenancy | reserve | composite PK | RT-37 |
| R3-007 | identity | LT-04 | confirm | RT-38 |
| R3-008 | settlement | BT-01c | unknown | RT-39 |
| R3-009 | INV-6,7 | LT-01 | lock order | RT-40 |

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
| RT-07 | LT-07 stale intent |
| RT-08 | LT-08 stale send |
| RT-09 | LT-09 recovery confirm |
| RT-10 | LT-10 mismatch → MANUAL |
| RT-11 | LT-11 atomic manual confirm + audit |
| RT-12 | LT-12 manual reject |
| RT-13 | LT-13 reopen |
| RT-14 | LT-14 external cancel |
| RT-15 | LA-01 lease fence |
| RT-16a/b/c | BT-01a/b/c |
| RT-17a/b | BT-02/03 late charge |
| RT-18 | role disjointness |
| RT-19 | forbidden Cursor HTTP imports |
| RT-20 | connect denied in launch tests |
| RT-22 | INV-9 no second POST |
| RT-30 | MOCK proof rejected when mode≠MOCK_ONLY |
| RT-31 | subclass MockCursorTransport rejected |
| RT-32 | ACL matrix |
| RT-33 | BT-04 orphan signal |
| RT-34 | estop denies LT-03 |
| RT-35 | confirm NULL→ids allowed; second change blocked |
| RT-36 | reconciler identity mismatch denied |
| RT-37 | cross-tenant reserve → not_found |
| RT-38 | record_intent DO NOTHING idempotency |
| RT-39 | tokens-only → usage_unknown |
| RT-40 | no I/O inside gate tx (mock slow path) |

**Transition test coverage:** each **LT-01..LT-14**, **LA-01**, **BT-01a..BT-04** maps to ≥1 RT above.

### 12.2 Documentation completeness check (R6B script spec)

Script parses ADR §8 and §9:

- LT IDs in §8 (launch) = LT IDs referenced in §9 (excluding LA-01 optional);
- Every LT/BT/LA row has non-empty Test column matching `RT-\d+`;
- Substring **`RELEASE_AFTER_INTENT`** present (INV-2 heading);
- No duplicate `(From, To, Actor)` among LT rows.

**R6A-R5:** spec only; not committed.

---

## 13. R6B scope

**Allowed:** 0008, protocol SQL, mock transport, RT catalog, isolated CI DB.

**Forbidden:** real HTTP, credentials, paid launch, merge, deploy, production, Cloud Agents.

**R6B_AUTHORIZED=NO** until Agent J approves this revision.

---

## 14. External dependencies

**EXTERNAL_CREATE_IDEMPOTENCY=UNVERIFIED.** **EXTERNAL_USAGE_AUTHENTICATION=NOT_PROVEN.**

---

## Document history

| Rev | Notes |
|-----|-------|
| R6A-R4 | LT/BT split |
| **R6A-R5** | R4-001..008; proof_class; BT split; permissions; MOCK fix; RT completeness |

## References

- Agent J `AP03C2-R6A-R4-001..008`
- SQL `0002`, `0004`, `0005`, `0006`, `0007`

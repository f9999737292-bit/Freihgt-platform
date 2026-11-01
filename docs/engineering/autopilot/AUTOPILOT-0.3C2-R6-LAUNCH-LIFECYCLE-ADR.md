# ADR: Autopilot 0.3C2 — Launch lifecycle and budget settlement (canonical contract)

## Status

**Proposed (R6A-R4).** Pending Agent J sign-off. **R6B MOCK_ONLY is forbidden** until approval.

Product code baseline (PR #322): `d1425269`. Documentation evolves on branch `feat/bintrans-autopilot-budget-v0.3c2`.

---

## 1. Agent J findings register — R6A-R3 (R6A-R4 response)

| ID | Sev | Evidence | Impact | Required fix | ADR section |
|----|-----|----------|--------|--------------|-------------|
| **R3-001** | MEDIUM | T6′ requires HTTP before T3; invariant forbids transport before `SEND_STARTED`; no post-send definite reject | Whitelist dead; 401/400 after send → MANUAL → global block | Remove T6′; add **LT-06** `SEND_STARTED→LAUNCH_REJECTED` with strict proof; or document MANUAL-only path | § Definite rejection; LT-06 |
| **R3-002** | MEDIUM | `close_budget` includes settlement; crash row REJECTED without close | R4-001 forgery risk | **`release_unused_reserve` only RELEASED**; reconciler only COMMITTED/OVERRUN | § Budget lifecycle |
| **R3-003** | MEDIUM | Missing SQL functions for T10/T13/T14/lease; recovery lacks close path; R4-003 dropped | Privilege gaps | Full function×role matrix; T7 uses definer-internal release | § Permissions |
| **R3-004** | MEDIUM | DI allows real transport; incomplete import denylist | MOCK bypass | Sealed mock type; no env transport; socket deny test | § MOCK_ONLY |
| **R3-005** | MEDIUM | 9/17 transitions without tests; test 1 missing | Unprotected invariants | **LT-* → RT-* one-to-one** | § Traceability |
| **R3-006** | LOW | `p_tenant_id` on all functions vs session tenant | Spoof risk | Runtime: session tenant only; cross-tenant → `not_found` | § Tenancy |
| **R3-007** | LOW | SHA/body ambiguity; optional immutability; clock_skew | Wrong binding | Two hashes; mandatory trigger; confirm idempotency rules | § Identity |
| **R3-008** | LOW | Verified zero dropped | False zero | Restore definitions; unknown ≠ zero | § Settlement |
| **R3-009** | LOW | Lock order/DO NOTHING/fence/T10∥T16/TTL/launch gate | Ambiguous impl | Restore lock section; merge duplicate transitions; define gate | § PostgreSQL |

---

## 2. Core invariants (normative)

| ID | Invariant |
|----|-----------|
| **INV-1** | After durable **`launch_protocol_record_intent`**, a `launch_intents` row exists for `(tenant_id, launch_id)`. |
| **INV-2** | **Runtime `budget_protocol_release` is forbidden** if **any** `launch_intents` row exists for that reservation — **regardless of `agent_id` / `run_id`**. |
| **INV-3** | Pre-intent release: runtime may release only when **no** launch row (reservation `RESERVED`). |
| **INV-4** | **`launch_protocol_release_unused_reserve`** (adapter/recovery/admin only) sets **`RELEASED`** only with terminal launch **`CANCELLED_BEFORE_SEND`** or **`LAUNCH_REJECTED`** in the **same transaction**. |
| **INV-5** | **`budget_reconciler_apply_usage`** alone sets **`COMMITTED` / `OVERRUN`** (and clears `usage_unknown` when known). Adapter/admin **never** commit cents. |
| **INV-6** | **`launch_protocol_mark_send_started` committed** before any transport call (mock or future real). |
| **INV-7** | **No external I/O** while `_budget_lock_global_gate()` is held or inside the same open transaction. |
| **INV-8** | **`GET 404` ≠ absence**; **`409` ≠ same request** without GET correlation; **timeout/5xx → UNKNOWN**. |
| **INV-9** | **No automatic Create retry** after ambiguous send. |
| **INV-10** | After **`COMMITTED`/`OVERRUN`**, launch row may stay **`LAUNCH_CONFIRMED`**; financial adjustment only via **idempotent reconciler re-apply** (late charge), never runtime release. |

---

## 3. Launch lifecycle vs budget lifecycle (separated)

### 3.1 Launch (`launch_intents.launch_state`)

| State | Role |
|-------|------|
| `INTENT` | Intent recorded; `send_started_at` NULL |
| `SEND_STARTED` | Outbound attempt started |
| `LAUNCH_UNKNOWN` | Ambiguous external outcome |
| `MANUAL_RECONCILIATION_REQUIRED` | Operator required |
| `LAUNCH_CONFIRMED` | Agent/run bound |
| `LAUNCH_REJECTED` | Definite no launch (pre-send cancel path or post-send proof) |
| `CANCELLED_BEFORE_SEND` | Cancelled before send marker |
| `EXTERNAL_AGENT_CANCELLED` | Provider cancel; spend may be unknown |

`NOT_RECORDED` = absence of row (pre-LT-01).

### 3.2 Budget (`budget_reservations`)

| State / flag | Meaning |
|--------------|---------|
| `RESERVED` | Hold active |
| `RELEASED` | Unused reserve returned (**not** spend proof) |
| `COMMITTED` | Verified charge applied (≤ reserve) |
| `OVERRUN` | Verified charge above reserve |
| `usage_unknown` | Spend not yet verified — **hold reserve**, global block |

**`release_unused_reserve` ≠ settlement.** Settlement = reconciler verified usage → `COMMITTED`/`OVERRUN`.

### 3.3 Verified zero and late charge (R3-008)

| Concept | Rule |
|---------|------|
| **Unknown usage** | API unreachable, tokens-only payload, or non-terminal run → **`usage_unknown=1`**, **`RESERVED`**, no release by timeout |
| **Verified zero (production)** | Terminal run + money source + aggregation window → **0 cents** recorded in `budget_verified_usage`; then commit |
| **Verified zero (R6B mock)** | Mock money provider returns **0** with `verification_class=MOCK_TRUSTED` |
| **Unknown ≠ zero** | Token zeros or missing usage **must not** commit |
| **Late charge** | Reconciler re-apply with higher cents → **`OVERRUN`** or adjustment row; same `(tenant_id, launch_id)` |

---

## 4. Definite rejection policy (R3-001, R3-004)

### 4.1 Never automatic REJECTED

- **409** → UNKNOWN then confirm path (LT-09), never REJECTED.
- **GET 404** → remain UNKNOWN / escalate MANUAL; **not** REJECTED.
- **Timeout / 5xx / 429 / unclassified 4xx** → **LT-05** UNKNOWN.

### 4.2 Pre-send (no transport yet)

| Transition | Condition |
|------------|-----------|
| **LT-02** `INTENT → CANCELLED_BEFORE_SEND` | Operator/adapter cancel; `send_started_at` NULL |
| **LT-07** stale INTENT | Recovery sweep; no send |

**Removed:** T6′ (HTTP before `SEND_STARTED`) — **unreachable** under INV-6.

### 4.3 Post-send definite reject — **LT-06**

`SEND_STARTED → LAUNCH_REJECTED` **only if all:**

1. Transport returns **synchronous classified response** on the **first and only** outbound create attempt for this launch (mock records attempt count).
2. Status ∈ whitelist: **`400` + `validation_error`**, **`400` + `repository_access`**, **`401`**, **`403`**.
3. Transport attestation **`agent_created=false`** (mock field; production requires ADR amendment + API proof).
4. **Not** 409, not 404-on-GET used as proof, not timeout/5xx.
5. Same transaction: **`launch_protocol_release_unused_reserve`** → budget **`RELEASED`**.

If whitelist response but **`agent_created` unknown** → **LT-05** UNKNOWN, reserve held.

### 4.4 Manual reject — **LT-12**

`MANUAL → LAUNCH_REJECTED` only with audit + operator attestation bundle (no agent for `planned_agent_id`).

---

## 5. PostgreSQL lock order and idempotency (R3-009)

```text
Step 1: autopilot_queue._budget_lock_global_gate()
Step 2: SELECT budget_reservations
        WHERE tenant_id = $t AND launch_id = $l FOR UPDATE
Step 3: SELECT launch_intents (same keys) FOR UPDATE if row exists
Step 4: validate transition + tenant binding
Step 5: COMMIT
Step 6: external/mock transport ONLY after step 5
```

**`launch_protocol_record_intent`:** `INSERT … ON CONFLICT (tenant_id, launch_id) DO NOTHING`; if no row returned, `SELECT` existing; **`planned_agent_id` never updated on conflict**.

**Recovery transitions (LT-07–LT-10):** must verify **`recovery_lease_token`** matches row and **`>=` expected fence**; stale token → **`lease_stale`**.

**TTL constants (R6B config table):** `intent_stale_ttl=15m`, `send_stale_ttl=30m`, `unknown_escalation_ttl=24h`, `recovery_lease_ttl=5m`, `clock_skew_seconds=120`.

**Launch gate (defined):** PostgreSQL flags **`autopilot_paid_launch_enabled=0`** and **`autopilot_estop=0`** required for LT-01/LT-03. Dispatcher legacy `create_agent` **disallowed** when gate would apply (paid path retired before enable). **0.3D / paid automation activation requires separate security audit**; **`PAID_AUTOMATION_ENABLED` stays 0** in R6B.

---

## 6. Tenancy and composite keys (R3-006)

**Migration 0008** must update **all** `launch_id` carriers: `budget_reservations`, `budget_verified_usage`, `launch_intents`, `budget_events` (if present), reconciler references — composite **`(tenant_id, launch_id)`**.

| Caller class | Tenant source |
|--------------|---------------|
| Tenant runtime (`budget_protocol_reserve/release`) | **`_budget_bound_tenant()` from session** — no trusted `p_tenant_id` |
| Adapter / recovery / admin / reconciler | **`p_tenant_id` + `p_launch_id`**; mismatch with reservation → **`not_found`** |

**Migration verification:** 0008 **up → down → up** on isolated DB (R6B CI).

---

## 7. Identity binding (R3-007)

| Artifact | Rule |
|----------|------|
| `create_body_template_sha256` | Hash of JSON template **without** `agentId` |
| `create_body_full_sha256` | Hash after `agentId` injected (= `planned_agent_id`) |
| `canonical_create_body_json` | Full body stored; CHECK **`agentId = planned_agent_id`**, **no `envVars`** |
| **`launch_protocol_confirm`** | Only writer of `agent_id`/`run_id`; **`agent_id = planned_agent_id`**; idempotent re-call same ids → no-op; different ids → **`identity_conflict`** |
| Partial create (agent, no run) | Cannot confirm → **LT-05** UNKNOWN → MANUAL |
| **`run_id` trust** | Declared by adapter/recovery from mock list-runs; PG enforces equality on reconcile payload, not Cursor origin (**`EXTERNAL_USAGE_AUTHENTICATION=NOT_PROVEN`**) |

**Immutability:** trigger **`launch_intents_immutable_ids`** (mandatory in 0008) denies UPDATE of `planned_agent_id`, `agent_id`, `run_id`.

---

## 8. Canonical launch transition table (single source of truth)

Budget column **`B-effect`:** `H`=hold RESERVED, `R`=RELEASED, `—`=unchanged launch-side (settlement via BT-*).

| ID | From | To | Actor | External proof | SQL function(s) | Tx boundary | Lock order | B-effect | Crash recovery | Test |
|----|------|-----|-------|----------------|-----------------|-------------|------------|----------|----------------|------|
| **LT-01** | NOT_RECORDED | INTENT | Launch adapter | Reserve exists; gate open | `launch_protocol_record_intent` | Steps 1–5 | 1→2→3 insert | H | Retry LT-01 | **RT-01** |
| **LT-02** | INTENT | CANCELLED_BEFORE_SEND | Launch adapter | `send_started_at` NULL | `launch_protocol_cancel_before_send` + `release_unused_reserve` | 1–5 atomic | 1→2→3 | R | Safe | **RT-02** |
| **LT-03** | INTENT | SEND_STARTED | Launch adapter | Estop clear | `launch_protocol_mark_send_started` then transport | 1–5 then 6 | 1→2→3 | H | LT-08 if stale | **RT-03** |
| **LT-04** | SEND_STARTED | LAUNCH_CONFIRMED | Launch adapter | Mock/GET OK + correlate | `launch_protocol_confirm` | 1–5 | 1→2→3 | H | LT-09 | **RT-04** |
| **LT-05** | SEND_STARTED | LAUNCH_UNKNOWN | Launch adapter | Timeout/5xx/409/unclassified | `launch_protocol_mark_unknown` | 1–5 | 1→2→3 | H | LT-08–10 | **RT-05** |
| **LT-06** | SEND_STARTED | LAUNCH_REJECTED | Launch adapter | Whitelist + `agent_created=false` | `launch_protocol_reject_definite` + `release_unused_reserve` | 1–5 atomic | 1→2→3 | R | — | **RT-06** |
| **LT-07** | INTENT (stale) | CANCELLED_BEFORE_SEND | Recovery | TTL; lease fence | `launch_protocol_cancel_before_send` (internal release) | 1–5 | 1→2→3+fence | R | Sweep | **RT-07** |
| **LT-08** | SEND_STARTED (stale) | LAUNCH_UNKNOWN | Recovery | TTL; lease fence | `launch_protocol_mark_unknown` | 1–5 | fence | H | Poll extx | **RT-08** |
| **LT-09** | LAUNCH_UNKNOWN | LAUNCH_CONFIRMED | Recovery | GET 200 + correlate | `launch_protocol_confirm` | 1–5 | fence | H | — | **RT-09** |
| **LT-10** | LAUNCH_UNKNOWN | MANUAL | Recovery | Max polls **or** correlate fail | `launch_protocol_escalate_manual` | 1–5 | fence | H | Operator | **RT-10** |
| **LT-11** | MANUAL | LAUNCH_CONFIRMED | Budget admin | Audit row | `launch_protocol_manual_audit_insert` + `confirm` | 1–5 | 1→2→3 | H | — | **RT-11** |
| **LT-12** | MANUAL | LAUNCH_REJECTED | Budget admin | Audit + attestation | audit + `reject_definite` + `release_unused_reserve` | 1–5 atomic | 1→2→3 | R | — | **RT-12** |
| **LT-13** | MANUAL | LAUNCH_UNKNOWN | Budget admin | Audit reopen | audit + `launch_protocol_reopen_unknown` | 1–5 | 1→2→3 | H | LT-09 | **RT-13** |
| **LT-14** | LAUNCH_CONFIRMED | EXTERNAL_AGENT_CANCELLED | Launch adapter | Mock cancel OK | `launch_protocol_mark_external_cancelled` | 1–5 | 1→2→3 | H | BT-01 | **RT-14** |
| **LT-15** | — | — | — | Lease acquire/renew | `launch_protocol_recovery_lease_acquire` | 1–5 | row lock | — | Fence | **RT-15** |

### 8.1 Budget settlement transitions (orthogonal)

| ID | From budget | To budget | Actor | SQL function | Test |
|----|-------------|-----------|-------|--------------|------|
| **BT-01** | RESERVED + confirmed launch | COMMITTED/OVERRUN or usage_unknown | Reconciler | `budget_reconciler_apply_usage` | **RT-16** |
| **BT-02** | COMMITTED | OVERRUN | Reconciler | idempotent re-apply | **RT-17** |

**LT-14 + BT-01:** reconciler may apply with **`EXTERNAL_AGENT_CANCELLED`**.

---

## 9. Crash / recovery matrix (LT references only)

| Event | Launch | Budget | Next LT | Auto release? | Auto Create? |
|-------|--------|--------|---------|---------------|--------------|
| Crash before LT-01 | NOT_RECORDED | H | LT-01 | No | No |
| Crash after LT-01 | INTENT | H | LT-02/03/07 | Only LT-02/07 | No |
| Crash after LT-03 | SEND_STARTED | H | LT-08→09/10 | No | No |
| Lost response | UNKNOWN | H | LT-09/10 | No | No |
| GET 404 | UNKNOWN | H | LT-10 | No | No |
| POST 409 | UNKNOWN | H | LT-09 or LT-10 | No | No |
| Definite 400/401 post-send | REJECTED | R | LT-06 | Via LT-06 only | No |
| Lease lost | — | H | LT-15 re-acquire | No | No |
| Concurrent release vs intent | — | — | one wins | No | No |

**Removed:** “REJECTED without close tx” (violates INV-4).

---

## 10. Permissions — function × role (R3-003)

**LOGIN roles:** `autopilot_tenant_runtime`, `autopilot_launch_adapter`, `autopilot_launch_recovery`, `autopilot_queue_reconciler`, `autopilot_budget_administrator`.

**NOLOGIN groups** grant EXECUTE; **one LOGIN per session** — membership in `{adapter, recovery, reconciler, budget_admin}` is **pairwise disjoint** (R4-003). Negative test **RT-18**.

| SQL function | Runtime | Adapter | Recovery | Reconciler | Admin |
|--------------|---------|---------|----------|------------|-------|
| `budget_protocol_reserve` | E | — | — | — | — |
| `budget_protocol_release` | E (INV-2 guard) | — | — | — | — |
| `launch_protocol_record_intent` | — | E | **D** | — | — |
| `launch_protocol_mark_send_started` | — | E | — | — | — |
| `launch_protocol_mark_unknown` | — | E | E* | — | — |
| `launch_protocol_reject_definite` | — | E | — | — | E |
| `launch_protocol_confirm` | — | E | E* | — | E |
| `launch_protocol_cancel_before_send` | — | E | E* | — | — |
| `launch_protocol_release_unused_reserve` | — | E** | E** | — | E |
| `launch_protocol_escalate_manual` | — | — | E* | — | E |
| `launch_protocol_reopen_unknown` | — | — | — | — | E |
| `launch_protocol_mark_external_cancelled` | — | E | — | — | — |
| `launch_protocol_recovery_lease_acquire` | — | — | E* | — | — |
| `launch_protocol_manual_audit_insert` | — | — | — | — | E |
| `budget_reconciler_apply_usage` | — | — | — | E | — |

\* Recovery EXECUTE only with valid lease fence inside function.  
\*\* Adapter/recovery call release only inside definer wrappers for LT-02/06/07/12 — not standalone for tenant.

**SECURITY DEFINER:** `search_path = autopilot_queue, pg_temp`; no direct DML grants on tables.

**Dispatcher:** read-only orchestration; **no** EXECUTE on launch write functions. **DRY_RUN worker** = tenant runtime only.

---

## 11. MOCK_ONLY boundary (R3-004)

| Rule | Enforcement (R6B) |
|------|---------------------|
| No real Cursor HTTP | `MockCursorTransport` sealed class; adapter `__init__` rejects non-Mock types |
| No production transport via DI | Constructor type check **`isinstance(..., MockCursorTransport)`** only |
| No env-based transport selection | Static test: no `os.environ`/`CURSOR_*` reads in `launch_*` packages |
| No dynamic arbitrary transport | Factory module imports **only** mock module |
| Import graph | RT-19: subprocess import → `cursor_api`, `httpx`, `urllib`, `requests`, `socket` **not** in `sys.modules` |
| Network deny | RT-20: monkeypatch `socket.socket.connect` → fail in launch/recovery tests |
| No `create_agent` in R6B code paths | Module split; tenant runtime cannot EXECUTE adapter functions (**RT-21**) |

Legacy `cursor_api.py` remains in repo but **outside** R6B import graph.

---

## 12. Traceability matrix (Finding → Invariant → LT → SQL → Test)

| Finding | Invariant | Transition | SQL boundary | Test |
|---------|-----------|------------|--------------|------|
| R3-001 | INV-6,8 | LT-06 | reject+release atomic | RT-06 |
| R3-002 | INV-4,5 | LT-02,06,12 / BT-01 | split functions | RT-02,06,16 |
| R3-003 | INV-2 | all LT | matrix §10 | RT-18 |
| R3-004 | MOCK | LT-03 | transport after commit | RT-19,20 |
| R3-005 | INV-6,9 | LT-03,05 | mark_send / unknown | RT-03,22 |
| R5-001 | INV-2 | deny | release guard | RT-02 |
| R5-002 | INV-5 | BT-01 | reconciler match | RT-16,10b |
| Crash pre-send | INV-1 | LT-01/07 | lock order | RT-01,07 |
| Lease loss | fence | LT-15 | lease acquire | RT-15 |
| Mismatch | INV-8 | LT-10 | escalate | RT-10 |
| No second POST | INV-9 | after LT-05 | policy | RT-22 |
| Manual audit | — | LT-11 | audit insert | RT-11,23 |

**Coverage rule:** every **LT-01..LT-15** and **BT-01..BT-02** has ≥1 **`RT-*`** test. **Untested transitions = 0** (target for R6B).

### 12.1 Regression test catalog (`test_budget_controller_03c2_r6b.py`)

| RT | Description |
|----|-------------|
| RT-01 | LT-01 before transport; intent durable |
| RT-02 | INV-2 runtime release denied when launch row exists |
| RT-03 | LT-03 send_started committed before mock invoke |
| RT-04 | LT-04 confirm binds ids |
| RT-05 | LT-05 ambiguous → UNKNOWN, no release |
| RT-06 | LT-06 definite reject post-send; 409 not reject |
| RT-07 | LT-07 stale intent cancel |
| RT-08 | LT-08 stale send → UNKNOWN |
| RT-09 | LT-09 recovery confirm |
| RT-10 | LT-10 mismatch → MANUAL |
| RT-11 | LT-11 manual confirm requires audit |
| RT-12 | LT-12 manual reject + release |
| RT-13 | LT-13 reopen MANUAL → UNKNOWN |
| RT-14 | LT-14 external cancel |
| RT-15 | LT-15 lease fence stale rejected |
| RT-16 | BT-01 reconciler only commits; adapter cannot |
| RT-17 | BT-02 late charge → OVERRUN |
| RT-18 | disjoint role membership denied |
| RT-19 | import graph clean |
| RT-20 | socket connect denied |
| RT-21 | runtime EXECUTE adapter function denied |
| RT-22 | no second mock POST after LT-05 |
| RT-23 | LT-11 without audit denied |

### 12.2 Documentation completeness check (R6B script, not R6A)

R6B adds `tools/bintrans_autopilot/tests/adr_launch_lifecycle_doc_check.py` (or shell) that parses this ADR and asserts:

- Set of LT IDs in §8 = set in crash matrix;
- Every LT has `Test` column RT-*;
- No duplicate `(From, To, Actor)` rows;
- **`RELEASE_AFTER_INTENT`** string present in INV-2.

**R6A-R4:** script **specified only**, not committed.

---

## 13. R6B scope (unchanged intent, tightened enforcement)

**Allowed:** PostgreSQL 0008, protocol functions, mock transport, LT/BT tests, CI isolated DB.

**Forbidden:** real HTTP, credentials, paid launches, merge, deploy, production, Cloud Agents, **`PAID_AUTOMATION_ENABLED=1`**.

**R6B_AUTHORIZED=NO** until Agent J approves **this** revision.

---

## 14. External dependencies

**EXTERNAL_CREATE_IDEMPOTENCY=UNVERIFIED.** **EXTERNAL_USAGE_AUTHENTICATION=NOT_PROVEN.**

---

## Document history

| Rev | Notes |
|-----|-------|
| R6A–R6A-R3 | Prior |
| **R6A-R4** | Canonical LT/BT table; release/settlement split; R3-001..009 |

## References

- Agent J `AP03C2-R6A-R3-001..009`
- Queue SQL `0002`, `0004`, `0005`, `0006`, `0007`

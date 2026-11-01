# ADR: Autopilot 0.3C2 — Launch lifecycle and cross-system settlement

## Status

**Proposed (architecture freeze R6A-R3).** Pending independent Agent J confirmation. Remediates **AP03C2-R6A-R2-001..006**. **R6B is forbidden** until J approves this revision.

PR #322 code baseline for product (pre-R6B): `d1425269`; docs head moves with ADR commits on branch `feat/bintrans-autopilot-budget-v0.3c2`.

---

## Agent J findings register — R6A-R2 errata (R6A-R3)

| ID | Sev | Evidence (R2 review) | Contradiction / gap | Required fix | R6A-R3 section |
|----|-----|----------------------|---------------------|--------------|----------------|
| **AP03C2-R6A-R2-001** | MEDIUM | ADR claims cross-tenant squat impossible; `budget_reservations.launch_id` is global PK in **0002**; functions lookup by `launch_id` only | R1-007 only on `launch_intents`; reservation layer still global | Composite key on **reservation + usage + functions** in 0008, or server-minted global `launch_id`; foreign key lookup returns **`not_found`** not `tenant_mismatch` oracle | § Launch key scope |
| **AP03C2-R6A-R2-002** | MEDIUM | Recovery shares adapter pool/secrets; wildcard `launch_protocol_*`; missing function×role matrix | Recovery can create; role flag ≠ isolation | Separate PG roles; recovery transport **read-only** (get/list); create credential **adapter only**; named EXECUTE matrix | § Roles & EXECUTE matrix |
| **AP03C2-R6A-R2-003** | MEDIUM | T6 vs LAUNCH_REJECTED rules; T12 impossible; test 2 vs CANCELLED release; T4 vs T9 in crash matrix; MANUAL→UNKNOWN missing in T-table | R5-001 release ambiguity | Single release formula; fix T-table; atomic budget release function; align tests | § Release policy; § T1–T17 |
| **AP03C2-R6A-R2-004** | MEDIUM | Confirm omits write targets; reconciler identity check unspecified; body vs `planned_agent_id` order | R5-002 persists | Confirm columns + immutability; reconciler payload ≡ launch row; body build after mint | § Identity binding |
| **AP03C2-R6A-R2-005** | LOW | Mock boundary = “test flag” / default harness only | Real HTTP still reachable | Static no-import test; factory mock-only in R6B; fail without injected transport | § R6B enforcement |
| **AP03C2-R6A-R2-006** | LOW | Lease/TTL/audit/estop/follow-up gaps; “Accepted” before J | R6A-010 regression | Restore lease+TTL; audit in 0008; estop test; follow-up ban; status Proposed | § Recovery lease; § R6B plan |

Findings are **not implementation-closed** until R6B tests exist.

---

## Core invariants

1. **`launch_protocol_record_intent` committed** → a `launch_intents` row exists for `(tenant_id, launch_id)`.
2. **Runtime release after durable intent:** **`budget_protocol_release` by tenant runtime is forbidden** whenever a `launch_intents` row exists for that reservation, **regardless of `agent_id` / `run_id` null or populated** (closes R5-001).
3. **Pre-intent release:** runtime may release only when **no** `launch_intents` row (reservation `RESERVED`, pre-launch cancel).
4. **Post-intent budget close:** only **`launch_protocol_close_budget`** (adapter or budget administrator role) in **one transaction** with a terminal launch transition (`CANCELLED_BEFORE_SEND`, `LAUNCH_REJECTED`, or settlement commit). Tenant runtime **never**.
5. **`SEND_STARTED` committed before** mock/real transport invoke; **no external I/O** inside gate transaction.
6. **`GET 404` ≠ absence**; **`409` ≠ confirmed same request** without GET field match.
7. **No automatic Create retry** after ambiguous send (`LAUNCH_UNKNOWN` / `MANUAL`).
8. **Settlement immutable:** after reservation `COMMITTED` or `OVERRUN`, launch transitions and release **must not** alter charged amounts (adjustments via reconciler idempotent supplement only).

---

## Release policy (canonical — R2-003)

### Tenant runtime (`budget_protocol_release`)

| Condition | Result |
|-----------|--------|
| No `launch_intents` row for `(tenant_id, launch_id)` and reservation `RESERVED` | **Allowed** → `RELEASED` |
| Any `launch_intents` row exists (any `launch_state`) | **Denied** (`launch_active`) |

This matches **R6B test 2:** deny release when intent row exists (including `INTENT` without send, without agent/run IDs).

### Launch adapter / budget administrator (`launch_protocol_close_budget`)

| Launch terminal | Preconditions | Budget |
|-----------------|---------------|--------|
| `CANCELLED_BEFORE_SEND` | T2/T7; never `SEND_STARTED` | `RELEASED` (same tx) |
| `LAUNCH_REJECTED` | T6′ only (see below) | `RELEASED` (same tx) |
| Settlement complete | T15; reconciler | `COMMITTED` / `OVERRUN` |

**Forbidden:** runtime release after intent; separate `release_after_reject` without atomic launch transition; timeout-based release.

### `LAUNCH_REJECTED` (R2-003 — no T6 from `SEND_STARTED` for whitelist-after-send)

**Removed:** `SEND_STARTED → LAUNCH_REJECTED` (contradicted `send_started_at` rules and T12).

**Allowed paths to `LAUNCH_REJECTED`:**

| Transition | Actor | Proof |
|------------|-------|-------|
| **T6′** `INTENT → LAUNCH_REJECTED` | Launch adapter | Synchronous whitelist HTTP **before** T3 (`send_started_at` still NULL): `400 validation_error`, `400 repository_access`, `401`, `403`. **Not** 409/429/5xx/timeout. |
| **T12** `MANUAL → LAUNCH_REJECTED` | Operator + budget administrator | Operator attestation + audit that **no agent exists** for `planned_agent_id` (evidence bundle); not “never SEND_STARTED” (impossible for MANUAL reached via T10). |

If **`send_started_at` is set** and outcome ambiguous, only **`LAUNCH_UNKNOWN`**, **`LAUNCH_CONFIRMED`**, or **`MANUAL`** — never `LAUNCH_REJECTED`.

**Whitelist label:** **inferred pre-create failures** (Cursor docs + adapter mapping); not a guarantee for every 4xx — adapter must classify; unclassified → `LAUNCH_UNKNOWN`.

---

## Launch key scope (R2-001)

**Current code (0002–0007):** `budget_reservations.launch_id` is a **global** PK; lookups use `launch_id` alone → cross-tenant **`tenant_mismatch`** oracle.

**R6B migration 0008 must (design requirement):**

- Composite PK **`(tenant_id, launch_id)`** on `budget_reservations`, `budget_verified_usage`, and `launch_intents`.
- All protocol functions take **`p_tenant_id` + `p_launch_id`**; reservation fetch uses **both**.
- Wrong tenant for existing key → **`not_found`** (same as missing row); never reveal other tenant’s reservation.

Until 0008 ships, ADR does **not** claim cross-tenant squat is impossible on the reservation table — only that R6B implementation **must** close R2-001.

---

## State vocabularies

### Launch (`launch_intents.launch_state`)

`NOT_RECORDED`, `INTENT`, `SEND_STARTED`, `LAUNCH_REJECTED`, `LAUNCH_CONFIRMED`, `LAUNCH_UNKNOWN`, `MANUAL_RECONCILIATION_REQUIRED`, `CANCELLED_BEFORE_SEND`, `EXTERNAL_AGENT_CANCELLED`.

### Budget (`budget_reservations.state`)

`RESERVED`, `COMMITTED`, `OVERRUN`, `RELEASED`, `BLOCKED`, flag **`usage_unknown`**.

### Terminal vs non-terminal (launch)

| Terminal launch | Non-terminal (hold reserve) |
|-----------------|----------------------------|
| `CANCELLED_BEFORE_SEND`, `LAUNCH_REJECTED` | `INTENT`, `SEND_STARTED`, `LAUNCH_UNKNOWN`, `MANUAL`, `LAUNCH_CONFIRMED`, `EXTERNAL_AGENT_CANCELLED` |

---

## Identity binding (R2-004)

### Columns (`launch_intents`, migration 0008)

| Column | Set by | Mutable |
|--------|--------|---------|
| `planned_agent_id` | `record_intent` only | **Immutable** after insert |
| `agent_id` | **`launch_protocol_confirm` only** | Immutable after confirm |
| `run_id` | **`launch_protocol_confirm` only** | Immutable after confirm |
| `canonical_create_body_sha256` | intent (body template **without** `agentId`) | Immutable |
| `canonical_create_body_json` | intent after `planned_agent_id` known | Immutable |

**Body build order:** (1) mint `planned_agent_id` in intent tx; (2) assemble create JSON with that `agentId`; (3) store SHA-256; (4) never mutate.

**Confirm:** `agent_id` must equal `planned_agent_id`; `run_id` from mock/Cursor list-runs selection (oldest run with `created_at >= send_started_at - clock_skew`, paginate all pages; default limit 20 insufficient alone).

**Reconciler:** `budget_reconciler_apply_usage` **must** reject unless launch_state ∈ `{ LAUNCH_CONFIRMED, EXTERNAL_AGENT_CANCELLED }` **and** payload `(agent_id, run_id)` **equals** launch row (R6B test 10).

**Immutability enforcement:** no UPDATE grant on `agent_id`/`run_id`/`planned_agent_id` for adapter/recovery; optional trigger deny.

---

## Unknown launch branches (409 / 404 / mismatch)

| Event | Transition | Actor |
|-------|------------|-------|
| POST 409 | `SEND_STARTED → LAUNCH_UNKNOWN` (or stay UNKNOWN) then **T9** after GET match | Adapter / recovery |
| GET correlation mismatch | **`LAUNCH_UNKNOWN → MANUAL`** (T10) or direct **T10m** from UNKNOWN | Recovery |
| GET 404 | Remain / enter **`LAUNCH_UNKNOWN`**; poll; **no release**; **no Create** | Recovery |

---

## Roles & EXECUTE matrix (R2-002)

**Separate PostgreSQL LOGIN roles (R6B):** `autopilot_launch_adapter`, `autopilot_launch_recovery`, `autopilot_queue_reconciler`, tenant runtime (existing), `autopilot_budget_administrator` (manual/close).

| Function | Tenant runtime | Launch adapter | Recovery | Reconciler | Budget admin |
|----------|----------------|----------------|----------|------------|--------------|
| `budget_protocol_reserve` | EXECUTE | — | — | — | — |
| `budget_protocol_release` | EXECUTE (pre-intent only) | — | — | — | — |
| `launch_protocol_record_intent` | — | EXECUTE | **DENY** | — | — |
| `launch_protocol_mark_send_started` | — | EXECUTE | — | — | — |
| `launch_protocol_mark_unknown` | — | EXECUTE | EXECUTE | — | — |
| `launch_protocol_confirm` | — | EXECUTE | EXECUTE | — | EXECUTE (manual T11) |
| `launch_protocol_close_budget` | — | EXECUTE | — | — | EXECUTE |
| `launch_protocol_cancel_before_send` (T2/T7) | — | EXECUTE | EXECUTE | — | — |
| `budget_reconciler_apply_usage` | — | — | — | EXECUTE | — |
| `launch_protocol_manual_audit_insert` | — | — | — | — | EXECUTE |

**Credentials:** create-capable secret **only** on adapter process. Recovery process: **read-only** transport (`get_agent`, `list_runs`) — **no `create` method** on interface. Reconciler: mock/real read usage in future; **no create**.

**Dispatcher / DRY_RUN worker:** no Cursor credentials; no `create_agent` when PostgreSQL launch gate enabled; does not call launch protocol write functions except via future orchestration hook (R6B: worker stays dry_run).

**Mock adapter:** must not grant tenant runtime EXECUTE on adapter-only functions.

---

## Recovery lease & TTL (R2-006)

**Columns:** `recovery_lease_holder`, `recovery_lease_expires_at`, `recovery_lease_token` (monotonic fence), `last_recovery_at`.

**TTL defaults (configurable, R6B must define constants):** `intent_stale_ttl`, `send_stale_ttl`, `unknown_escalation_ttl`, `recovery_lease_ttl`.

**Crash matrix row “Recovery lease lost”:** new holder may acquire if token greater and prior lease expired.

---

## Unified transition table T1–T17

All transitions **must** appear here; diagram and crash matrix **must** reference these IDs only.

| ID | From | To | Actor | Proof / precondition | PG tx | Budget | Crash recovery |
|----|------|-----|-------|----------------------|-------|--------|----------------|
| T1 | NOT_RECORDED | INTENT | Launch adapter | Reserve RESERVED; estop clear | Gate→2→3 insert | RESERVED | Retry T1 |
| T2 | INTENT | CANCELLED_BEFORE_SEND | Launch adapter | `send_started_at` NULL | Gate→2→3 + **close_budget** | RELEASED | Safe |
| T3 | INTENT | SEND_STARTED | Launch adapter | estop clear | Gate→2→3; **commit** then transport | RESERVED | T8 if stale |
| T4 | SEND_STARTED | LAUNCH_CONFIRMED | Launch adapter | Transport/GET + correlation; confirm writes ids | Gate→2→3 confirm | RESERVED | T9 |
| T5 | SEND_STARTED | LAUNCH_UNKNOWN | Launch adapter | Timeout/5xx/409/ambiguous | Gate→2→3 | RESERVED | T8–T10 |
| T6′ | INTENT | LAUNCH_REJECTED | Launch adapter | Whitelist 4xx; **send_started_at** NULL | Gate→2→3 + **close_budget** | RELEASED | — |
| T7 | INTENT (stale) | CANCELLED_BEFORE_SEND | Recovery | TTL; no send_started | Gate→2→3 + close | RELEASED | Sweep |
| T8 | SEND_STARTED (stale) | LAUNCH_UNKNOWN | Recovery | TTL exceeded | Gate→2→3 | RESERVED | Poll extx |
| T9 | LAUNCH_UNKNOWN | LAUNCH_CONFIRMED | Recovery | GET 200 + correlate | Gate→2→3 confirm | RESERVED | — |
| T10 | LAUNCH_UNKNOWN | MANUAL | Recovery / policy | max polls / mismatch | Gate→2→3 | RESERVED | Operator |
| T11 | MANUAL | LAUNCH_CONFIRMED | Budget admin + confirm | Audit row **required** | Gate→2→3 + audit | RESERVED | — |
| T12 | MANUAL | LAUNCH_REJECTED | Budget admin | Audit + operator attestation no agent | Gate→2→3 + close | RELEASED | Rare |
| T13 | MANUAL | LAUNCH_UNKNOWN | Budget admin | Audit; reopen recovery | Gate→2→3 | RESERVED | T9 |
| T14 | LAUNCH_CONFIRMED | EXTERNAL_AGENT_CANCELLED | Launch adapter | Mock cancel OK | Gate→2→3 | RESERVED | usage unknown ok |
| T15 | LAUNCH_CONFIRMED or EXTERNAL_AGENT_CANCELLED | (settlement) | Reconciler | Mock verified usage + identity match | Reconciler tx | COMMITTED/OVERRUN | Idempotent |
| T16 | LAUNCH_UNKNOWN | MANUAL | Recovery | GET mismatch (409 correlate fail) | Gate→2→3 | RESERVED | — |
| T17 | EXTERNAL_AGENT_CANCELLED | (settlement) | Reconciler | Same as T15 | Reconciler tx | COMMITTED/OVERRUN/usage_unknown | — |

**409 path:** T5 (ambiguous) → T9; if correlation fails → T16/T10.

**Runtime release:** denied for all rows with launch_intents present (tests 2, 3).

---

## Crash / recovery matrix (T-ID reference)

| Crash / event | Launch state | Budget | Next action (T-ID) | Auto release? | Auto Create? |
|---------------|--------------|--------|-------------------|---------------|--------------|
| Before T1 | NOT_RECORDED | RESERVED | T1 | No | No |
| After T1 | INTENT | RESERVED | T2, T3, or T6′ | Only T2/T6′ via close_budget | No |
| After T3, no response | SEND_STARTED | RESERVED | T8→T9/T10 | No | No |
| Lost response | UNKNOWN | RESERVED | T9/T10 | No | No |
| GET 404 | UNKNOWN | RESERVED | T10 | No | No |
| 409 mock | UNKNOWN | RESERVED | T9 or T16 | No | No |
| After T4 | CONFIRMED | RESERVED | T15 | No | No |
| Lease lost | recovering | RESERVED | re-acquire lease | No | No |
| REJECTED without close tx | LAUNCH_REJECTED | RESERVED | **close_budget** recovery | No | No |

---

## Actor permission matrix (summary)

See **Roles & EXECUTE matrix**. Budget **administrator** covers operator manual paths (T11–T13, T12). **DRY_RUN worker** = tenant runtime only (reserve + pre-intent release). **Dispatcher** = out of scope for R6B writes; must not bypass gate.

---

## Scenario → test traceability (R2-003)

| Scenario | Transition | Expected launch | Budget | Test |
|----------|------------|-----------------|--------|------|
| Release after intent, no agent/run | (deny) | INTENT | RESERVED | test 2 |
| Reserve vs intent race | T1 vs release | one winner | — | test 3 |
| Lost response | T5→T9/T10 | UNKNOWN/MANUAL | RESERVED | test 4 |
| 409 not REJECTED | T5→T9 | CONFIRMED | RESERVED | test 5 |
| GET 404 | T8/T10 | UNKNOWN/MANUAL | RESERVED | test 6 |
| Intent idempotency | T1 conflict | same planned_agent_id | RESERVED | test 7 |
| Mock transport isolation | import guard | n/a | — | test 8 |
| Global block UNKNOWN/MANUAL | gate fn | — | block reserve | test 9 |
| Reconcile without confirm | deny apply | CONFIRMED required | RESERVED | test 10 |
| Reconcile identity mismatch | deny | CONFIRMED | RESERVED | test 10b |
| Tokens-only usage | T15 fail | usage_unknown | RESERVED | test 11 |
| MANUAL confirm needs audit | T11 | CONFIRMED | RESERVED | test 12 |
| MANUAL without audit | deny T11 | MANUAL | RESERVED | test 12n |
| Stale SEND_STARTED | T8 | UNKNOWN | RESERVED | test 13 |
| Cross-tenant key | reserve | not_found | — | test 14 |
| Recovery no record_intent | DENY | — | — | test 15 |
| Estop before send | deny T3 | INTENT | RESERVED | test 16 |

---

## R6B scope — MOCK ONLY (R2-005)

### Allowed

PostgreSQL lifecycle (**0008**), protocol functions, mock transport, crash/timeout simulation, recovery lease, security tests, CI without Cursor secrets.

### Forbidden

Real HTTP to Cursor; real credentials; `CursorClient.create_agent` / `follow_up` in R6B modules; paid launches; merge/deploy/production access; admin usage cross-check; automatic Create retry.

### Enforcement (design requirements for R6B code)

1. **`launch_adapter` / `launch_recovery` packages:** static test — no import of `cursor_api`, `httpx`, `urllib` (allowlist mock module only).
2. **Transport factory for R6B:** returns mock implementation only; **ImportError or explicit error** if production transport requested.
3. **No `if TEST_FLAG: use CursorClient`** — physical separation of modules.
4. **Reconciliation in R6B:** mock usage provider only; never call live `usage()`.
5. **CI:** no `CURSOR_API_KEY` / admin keys for autopilot jobs.

**Follow-up / steer:** forbidden in paid mode (restored); R6B does not implement follow-up paths.

**Estop:** DB flag checked in T1/T3; **test 16** denies T3 when estop set.

---

## R6B test plan

Tests listed in **Scenario traceability**; file `tools/bintrans_autopilot/tests/test_budget_controller_03c2_r6b.py`.

**Test 2 wording:** tenant runtime **`budget_protocol_release` denied** when **`launch_intents` row exists** (any state).

**Test 10 wording:** apply allowed for **`LAUNCH_CONFIRMED`** and **`EXTERNAL_AGENT_CANCELLED`** only, with **identity equality**.

---

## R6B implementation plan (0008)

1. Composite PK `(tenant_id, launch_id)` on reservations, verified usage, launch_intents; function signature updates (**R2-001**).
2. `launch_intents` full columns (body, ids, lease, audit FK).
3. Table **`autopilot_manual_reconciliation_audit`**; confirm/manual transitions **require** audit row (**R2-006**).
4. `launch_protocol_close_budget` atomic with terminal launch transitions.
5. Role splits + EXECUTE grants per matrix.
6. Mock transport module + import guard tests.
7. Extend `budget_global_safety_block_reason()` for UNKNOWN, MANUAL, stale SEND_STARTED.
8. TTL configuration documented in migration comments or `autopilot_queue_config`.

---

## Usage & settlement (unchanged core)

Documented agent usage returns **tokens**, not cents. R6B: mock only; fail-closed `usage_unknown`. Production money via future admin-events ADR amendment.

**EXTERNAL_CREATE_IDEMPOTENCY=UNVERIFIED.** **EXTERNAL_USAGE_AUTHENTICATION=NOT_PROVEN.**

---

## Unresolved external dependencies

(Empirical idempotency, 404 semantics, money source, read-only keys, dispatcher retirement, beta API stability — unchanged from R6A-R2.)

---

## Document history

| Revision | Date | Notes |
|----------|------|-------|
| R6A / R6A-R1 / R6A-R2 | 2026-11-01 | Prior iterations |
| R6A-R3 | 2026-11-01 | R2-001..006 consistency; release policy; T1–T17 |

## References

- Agent J: `AP03C2-R6A-R2-001..006`
- `tools/bintrans_autopilot/sql/autopilot_queue/0002_budget.up.sql`, `0007_release_identity.up.sql`
- `docs/engineering/autopilot/BUDGET_CONTROLLER_0_3C2.md`

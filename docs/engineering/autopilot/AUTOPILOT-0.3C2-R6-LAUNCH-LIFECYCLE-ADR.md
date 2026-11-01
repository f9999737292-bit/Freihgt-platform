# ADR: Autopilot 0.3C2 — Launch lifecycle and cross-system settlement

## Status

**Accepted (architecture freeze R6A-R2).** Remediates Agent J **AP03C2-R6A-R1-001..007**. Implementation phase **R6B** is **DRY_RUN foundation only** (mock Cursor adapter). Real external launch remains **forbidden** until separate authorization.

PR #322 product code at freeze baseline: `d14252690521472b507ad34b9aa98991e5fde36c`.

## Context

PostgreSQL budget reservations, reconciler-gated commits, and release rules (queue migrations **0007**) address symptoms of **R5-001** / **R5-002** but not durable launch intent before HTTP. This ADR defines the launch lifecycle, recovery, and R6B scope.

---

## Agent J findings register — R6A-R1 errata (R6A-R2)

Findings are **addressed in this document**; **not closed in implementation** until R6B tests pass. Do not treat as verified without evidence.

| ID | Sev | Evidence | Impact | Required fix | R6A-R2 response |
|----|-----|----------|--------|--------------|-----------------|
| **AP03C2-R6A-R1-001** | MEDIUM | `LAUNCH_REJECTED` as generic “4xx”; 409 `agent_id_conflict` means agent **exists**; reject after ambiguous attempt unsafe | Release while agent live (R5-001 recurrence) | Whitelist REJECTED codes; 409 → confirm+GET; no REJECTED if any attempt ambiguous | § LAUNCH_REJECTED; § Unknown launch |
| **AP03C2-R6A-R1-002** | MEDIUM | Documented `GET .../usage` returns **tokens**, not `chargedCents`; zero tokens ≠ settled spend | Real reconciliation always “unknown”; false verified zero | Define money source for production; verified zero only terminal run + aggregation window | § Usage & settlement |
| **AP03C2-R6A-R1-003** | MEDIUM | Credentials not scoped; recovery had `record_intent` + POST; incomplete function matrix | Confused deputy; privilege creep | Separate secrets/processes; recovery without minting intents; full EXECUTE matrix; block reconciler-as-tenant | § Credentials & roles |
| **AP03C2-R6A-R1-004** | MEDIUM | Missing MANUAL exits; no inbound to `EXTERNAL_AGENT_CANCELLED`; recovery actor gaps; launch layer mixed with settlement | Unimplementable / ambiguous state machine | Single authoritative transition table; settlement on reservation only | § Unified transition contract |
| **AP03C2-R6A-R1-005** | MEDIUM | No canonical create body column; 409 not proof of same request; `latestRunId` unstable | Wrong run binding; unsafe retry | Persist body/digest; 409 → GET field match; first run via List Runs (oldest) | § Request body & run binding |
| **AP03C2-R6A-R1-006** | LOW | External GET optional inside tx ambiguous; `ON CONFLICT DO UPDATE`; safety block extension omitted from plan | Global stall; intent mutation | **No external I/O under gate lock**; `DO NOTHING`; extend `budget_global_safety_block_reason` in R6B | § I/O rule; R6B plan |
| **AP03C2-R6A-R1-007** | LOW | `launch_id` PK global but job idempotency is per-tenant | Cross-tenant squat / IDOR on launch key | PK `(tenant_id, launch_id)` | § Identity |

---

## Architecture decisions (unchanged core)

1. Durable **`LAUNCH_INTENT`** (with server **`planned_agent_id`**) before any external create (R6B mock only).
2. **`SEND_STARTED`** committed **before** adapter invokes transport (mock or real).
3. **Runtime cannot release** after launch row exists with state ≥ **`INTENT`** (strictest wording; includes pre-send intent).
4. **PostgreSQL** is the **only** source of truth for launch authorization state in paid autopilot mode.
5. **No external I/O** while holding **`_budget_lock_global_gate()`** or an open transaction that holds it.
6. **No automatic Create retry** after an **ambiguous** outbound attempt (`SEND_STARTED` with unknown outcome).
7. **`GET 404`** is **not** proof the agent was never created (eventual consistency; Builds precedent in Cursor docs).
8. **`409 agent_id_conflict`** is **not** unconditional idempotency proof — requires **GET** and correlation field match before confirm.

---

## R6B scope boundary — MOCK ONLY (mandatory)

R6B **may** implement PostgreSQL protocols, worker integration, and tests **only** with an **injected mock Cursor transport**.

| Allowed in R6B | Forbidden in R6B |
|----------------|------------------|
| Migration **0008+** in isolated CI DB | Real `POST /v1/agents`, real GET to Cursor |
| `LaunchAdapter` interface + mock implementation | Cursor API credentials in CI or runtime |
| Simulated timeout, 409, 200, lost response | Paid launches, `PAID_AUTOMATION_ENABLED=1` |
| State machine + lock-order tests | Real usage reconciliation / admin spend cross-check |
| Hard deny: runtime/direct dispatcher `create_agent` when launch gate on | Follow-up, steer, cancel against live API |
| Docs + SQL + Python tests | Automatic merge, deployment, Cloud Agents |

**R6B_SCOPE=MOCK_ONLY.** Mock proves **ADR state model**, not Cursor behavior. **`EXTERNAL_CREATE_IDEMPOTENCY=UNVERIFIED`**, **`EXTERNAL_USAGE_AUTHENTICATION=NOT_PROVEN`**.

---

## State vocabularies (R1-004)

### Launch (`launch_intents.launch_state`) — physical / authorization only

`NOT_RECORDED`, `INTENT`, `SEND_STARTED`, `LAUNCH_REJECTED`, `LAUNCH_CONFIRMED`, `LAUNCH_UNKNOWN`, `MANUAL_RECONCILIATION_REQUIRED`, `CANCELLED_BEFORE_SEND`, `EXTERNAL_AGENT_CANCELLED`.

**Not on launch row:** `USAGE_PENDING`, `SETTLED` — those are **reservation settlement** concepts only.

### Budget (`budget_reservations.state`)

`RESERVED`, `COMMITTED`, `OVERRUN`, `RELEASED`, `BLOCKED`, plus row flag **`usage_unknown`**.

Settlement progression after launch confirmed: reservation stays **`RESERVED`** until reconciler commits; then **`COMMITTED`** / **`OVERRUN`**.

---

## LAUNCH_REJECTED (R1-001)

**Reachable only if ALL hold:**

1. **`send_started_at IS NULL`** (never entered ambiguous send), **or** policy explicitly allows reject-only path for stale **`INTENT`** sweep with proof no send marker (adapter cancel).
2. **No prior ambiguous attempt** for this `(tenant_id, launch_id)` — if any history contains `SEND_STARTED` with unknown outcome, **`LAUNCH_REJECTED` is forbidden**; only **`LAUNCH_CONFIRMED`** or **`MANUAL_RECONCILIATION_REQUIRED`**.
3. HTTP status is in **whitelist** (Cursor-documented pre-create failures only), e.g.:
   - `400` with `validation_error` or `repository_access`
   - `401` / `403` authentication or authorization failure
4. **Explicitly NOT reject:** `409 agent_id_conflict` (→ confirm path), `429`, `5xx`, timeout, network error, truncated body, **`404` on GET**.

**409 handling:** transition to **`LAUNCH_UNKNOWN`** or proceed to **`launch_protocol_confirm`** only after **`GET /v1/agents/{planned_agent_id}`** and correlation match (§ Request body).

**Release after `LAUNCH_REJECTED`:** **`launch_protocol_release_after_reject`** (adapter or dedicated definer only), same lock order, reservation → **`RELEASED`**. Tenant runtime **never** releases post-intent.

---

## Unknown launch policy (R1-001, R6A-R1 review)

| Signal | Interpretation | Action |
|--------|----------------|--------|
| Timeout / 5xx / partial body after `SEND_STARTED` | Ambiguous | → `LAUNCH_UNKNOWN`; **reserve held** |
| `GET` 200 with matching correlation | Confirmed create | → `LAUNCH_CONFIRMED` |
| `GET` 404 | **Not** proof of absence | Stay ambiguous; escalate polls; **no auto Create** |
| `POST` 409 | Agent may exist | **GET + correlate**; never → `LAUNCH_REJECTED` |
| Exhausted automated recovery | Unprovable | → `MANUAL_RECONCILIATION_REQUIRED` |

**AUTO_RETRY_CREATE=NO** after ambiguous send. Optional **manual** operator-triggered idempotent create (same `planned_agent_id`) is **out of R6B** and requires separate authorization.

---

## MANUAL_RECONCILIATION_REQUIRED — exits (R1-004)

Reserve **always retained** until a row below completes. **No** timeout-only release.

| From | To | Actor | Preconditions | Budget effect |
|------|-----|-------|---------------|---------------|
| `MANUAL_RECONCILIATION_REQUIRED` | `LAUNCH_CONFIRMED` | Operator tool + launch adapter confirm | Documented evidence (Cursor UI/API export); audit record `manual_reconciliation_id` | Stays `RESERVED` |
| `MANUAL_RECONCILIATION_REQUIRED` | `LAUNCH_REJECTED` | Operator + adapter | **Only** if operator attests **provable non-create** (e.g. logs show whitelist 4xx on **only** attempt and never `SEND_STARTED`) — rare | → `RELEASED` via adapter |
| `MANUAL_RECONCILIATION_REQUIRED` | `LAUNCH_UNKNOWN` | Operator | Reopen automated recovery window | `RESERVED` |
| `MANUAL_RECONCILIATION_REQUIRED` | (unchanged) | — | Default: block new paid launches globally | `RESERVED` |

Operator actions require **`autopilot_manual_reconciliation_audit`** row (who, when, launch_id, tenant_id, action, ticket ref). Break-glass SQL without audit is **forbidden** in production; R6B tests use mock audit table only.

---

## Request body, 409 correlation, run binding (R1-005)

**Storage (migration 0008):**

- `launch_intents.canonical_create_body_json` (immutable after intent) **or** FK to immutable `jobs` payload row used to build create body.
- `canonical_create_body_sha256` required for retries.
- `cursor_create_response_digest` optional audit on confirm (non-authoritative alone).

**409 / retry rule:** before confirm, **`GET` agent** must match intent: `repos[].url`, `startingRef`, `name`, `createdAt` window, and `agentId == planned_agent_id`. Mismatch → **`MANUAL_RECONCILIATION_REQUIRED`**, not reject.

**First run selection:** use **`GET /v1/agents/{id}/runs`** (List Runs) and bind **oldest run** created after `send_started_at`, not `latestRunId` (changes on follow-up / steer). R6B mock simulates ordered runs.

---

## Usage & settlement (R1-002)

| Source | Fields | Use in R6B | Use before real paid launch |
|--------|--------|------------|----------------------------|
| `GET /v1/agents/{id}/usage?runId=` | Tokens per run (documented) | Mock only; tests for **unknown vs zero tokens** | Not sufficient for cents settlement |
| Admin `POST /teams/filtered-usage-events` | `chargedCents` (hourly aggregation) | **Not called** in R6B | Candidate **money** source with delay |
| Reconciler-forged payload | SHA-256 dedup | Mock inject | **NOT_PROVEN** origin |

**Verified zero (production definition, not R6B claim):** run in **terminal** Cursor run state, aggregation window elapsed, money source returns **zero cents** for bound `(agent_id, run_id, tenant window)`. Token zeros alone = **unknown**, not verified zero.

**Reconciler rule:** must not commit cents from usage endpoint until money source ADR amendment; R6B tests assert **fail-closed** (`usage_unknown`) when mock returns tokens-only.

---

## Credentials & roles (R1-003)

| Role | Process | Secrets | May EXECUTE |
|------|---------|---------|-------------|
| Tenant runtime | Worker | Tenant DSN only | `budget_protocol_reserve`, pre-intent `release` |
| Launch adapter | Dedicated worker pool | Create-capable key **only here** | `launch_protocol_*` (except manual audit) |
| Recovery worker | Same pool as adapter **or** shared binary with **role flag** | Same as adapter | `launch_protocol_mark_unknown`, `launch_protocol_confirm`, poll transport — **NOT** `record_intent` |
| Reconciler | Reconciliation service | Read usage + admin (future) | `budget_reconciler_apply_usage` only |
| Operator | Human | Break-glass separate | Audited manual confirm API |

**Forbidden:**

- Reconciler DB role connected as tenant principal for launch or reserve (R4-003).
- Test / migration superuser privileges in production runtime sessions.
- Dispatcher holding Cursor create key when PostgreSQL launch gate enabled.

**Planned_agent_id** is generated **inside** `launch_protocol_record_intent`, not copied from dispatcher GitHub state (single mint authority).

---

## PostgreSQL linearization & I/O (R1-006)

```text
1. _budget_lock_global_gate()
2. budget_reservations FOR UPDATE (tenant_id, launch_id)
3. launch_intents FOR UPDATE if exists
4. validate + mutate
5. COMMIT  -- releases gate
6. External I/O (mock transport) ONLY after step 5
```

**Hard rule:** steps 6 never run inside steps 1–5. Recovery polls and create POST occur **outside** gate transaction.

**`record_intent` idempotency:** `INSERT ... ON CONFLICT (tenant_id, launch_id) DO NOTHING RETURNING *`; if no row returned, `SELECT` existing — **never** mutate `planned_agent_id` on conflict.

**Global safety block (R6B):** extend `budget_global_safety_block_reason()` for launch_state ∈ `{ LAUNCH_UNKNOWN, MANUAL_RECONCILIATION_REQUIRED, SEND_STARTED stale }` and existing `usage_unknown`.

---

## Identity (R1-007)

- Primary key: **`(tenant_id, launch_id)`** on `launch_intents`.
- Reservation FK matches both columns.
- Queue job supplies `(tenant_id, launch_id)`; cross-tenant squat on key impossible.

---

## Unified transition contract

Each row: **launch state**, actor, precondition, **transaction boundary**, **budget effect**, **crash recovery**.

| # | From | To | Actor | Precondition | PG tx boundary | Budget effect | After crash |
|---|------|-----|-------|--------------|----------------|---------------|-------------|
| T1 | NOT_RECORDED | INTENT | Launch adapter | Reserve RESERVED; estop clear | Gate→2→3 insert; no I/O | RESERVED | Re-enqueue intent |
| T2 | INTENT | CANCELLED_BEFORE_SEND | Launch adapter | send_started_at NULL | Gate→2→3 | RELEASED | Safe: unsent |
| T3 | INTENT | SEND_STARTED | Launch adapter | estop clear | Gate→2→3; commit then POST | RESERVED | If crash after commit: recovery T7 |
| T4 | SEND_STARTED | LAUNCH_CONFIRMED | Launch adapter | Mock/GET correlation OK | Gate→2→3 confirm | RESERVED | GET+confirm |
| T5 | SEND_STARTED | LAUNCH_UNKNOWN | Launch adapter | Timeout/5xx/ambiguous | Gate→2→3 | RESERVED | Recovery T7–T8 |
| T6 | SEND_STARTED | LAUNCH_REJECTED | Launch adapter | Whitelist 4xx; never sent ambiguous | Gate→2→3 | RELEASED | Only if provably unsent |
| T7 | INTENT (stale) | CANCELLED_BEFORE_SEND | Recovery | No send_started; TTL | Gate→2→3 | RELEASED | Sweep |
| T8 | SEND_STARTED (stale) | LAUNCH_UNKNOWN | Recovery | TTL exceeded | Gate→2→3 | RESERVED | Poll outside tx |
| T9 | LAUNCH_UNKNOWN | LAUNCH_CONFIRMED | Recovery | GET 200 + correlate | Gate→2→3 | RESERVED | Same |
| T10 | LAUNCH_UNKNOWN | MANUAL | Recovery/policy | max attempts | Gate→2→3 | RESERVED | Operator |
| T11 | MANUAL | LAUNCH_CONFIRMED | Operator+adapter | Audit | Gate→2→3 | RESERVED | — |
| T12 | MANUAL | LAUNCH_REJECTED | Operator+adapter | Provable non-create | Gate→2→3 | RELEASED | Rare |
| T13 | LAUNCH_CONFIRMED | EXTERNAL_AGENT_CANCELLED | Launch adapter | Cancel mock success | Gate→2→3 | RESERVED | Usage may be unknown |
| T14 | EXTERNAL_AGENT_CANCELLED | (usage reconcile) | Reconciler | Launch confirmed path | Reconciler tx | COMMITTED/OVERRUN/usage_unknown | — |
| T15 | LAUNCH_CONFIRMED | — | Reconciler | Verified/mock settlement | Reconciler tx | COMMITTED/OVERRUN | Idempotent |

Runtime **`budget_protocol_release`:** allowed only when no launch row **or** launch ∈ `{ LAUNCH_REJECTED, CANCELLED_BEFORE_SEND }` with adapter transition already recorded (T2/T6/T12).

---

## Crash / recovery matrix (aligned with T-table)

| Crash point | Launch state | Budget | Next worker action | Auto release? | Auto Create? |
|-------------|--------------|--------|-------------------|---------------|--------------|
| Before T1 commit | NOT_RECORDED | RESERVED | T1 | No | No |
| After T1, before T3 | INTENT | RESERVED | T2 or T3 | Cancel only | No |
| After T3 commit, before response | SEND_STARTED | RESERVED | T8→GET; **no POST** if ambiguous | No | **No** |
| Response lost | SEND_STARTED/UNKNOWN | RESERVED | T9 or T10 | No | **No** |
| GET 404 | UNKNOWN | RESERVED | T10 eventually | No | **No** |
| 409 on mock | UNKNOWN | RESERVED | GET correlate → T4 | No | **No** (GET only in R6B) |
| After T4 | CONFIRMED | RESERVED | Reconciler mock | No | No |
| Recovery lease lost | any recovery | RESERVED | Re-acquire lease | No | No |

---

## External Cursor API — requirements (documentation-level)

| Topic | ADR requirement |
|-------|-----------------|
| Preselected `agentId` | Optional `bc-…`; store before send |
| Create idempotency | Documented 409 on duplicate; **UNVERIFIED** empirically |
| GET agent | Recovery; **404 ≠ absent** |
| List Runs | Oldest run after send for binding |
| Usage | Tokens; runId query param exists; **not money** |
| List agents | Exists in API; optional orphan audit only; not required for R6B mock |

---

## Threat model (selected)

| Threat | PG control | Fail-closed | R6B test |
|--------|------------|-------------|----------|
| 409 → REJECTED → release (R1-001) | Whitelist reject | `invalid_transition` | mock 409 → CONFIRMED path |
| 404 → “no agent” release | Policy | stay UNKNOWN/MANUAL | mock 404 no release |
| Auto POST after ambiguous | Adapter policy | no transition | assert no second POST |
| Release vs intent race | Lock order | one winner | concurrent test |
| I/O under gate | Code review + test | no hold | mock slow I/O not in tx |
| Cross-tenant launch_id | PK (tenant, launch) | FK violation | tenant B cannot bind A |
| Forged usage | Reconciler only; mock | usage_unknown | no commit without mock GET |
| Runtime post-intent release | SQL gate | launch_active | after INTENT |

---

## R6B test plan (DRY_RUN)

1. Intent before transport invoke (mock).
2. Release denied when launch ≥ INTENT.
3. Reserve/release/intent race.
4. Lost response → UNKNOWN; no release.
5. Mock 409 → confirm path, **not** REJECTED (R1-001).
6. Mock GET 404 → stays ambiguous; no auto Create.
7. `record_intent` conflict does not change `planned_agent_id`.
8. No network import in default adapter test harness.
9. Global block includes UNKNOWN + MANUAL (mock two tenants).
10. Reconciler rejects apply without LAUNCH_CONFIRMED.
11. Tokens-only mock → usage_unknown (R1-002).
12. MANUAL → CONFIRMED with audit row (mock).
13. Stale SEND_STARTED → recovery transition T8.
14. `(tenant_id, launch_id)` PK isolation.

File: `tools/bintrans_autopilot/tests/test_budget_controller_03c2_r6b.py`.

---

## R6B implementation plan (minimal)

1. **0008:** `launch_intents` PK `(tenant_id, launch_id)`, body columns, estop flag, remove reserve ID params.
2. Launch protocol functions with lock order + release gate (intent exists).
3. Mock `CursorTransport` injected; **no** production `CursorClient` in R6B code paths under test flag.
4. Recovery worker transitions T7–T10 without `record_intent`.
5. Extend `budget_global_safety_block_reason()` for unknown/manual launch.
6. Tests above; CI remains dry-run.

**Explicitly deferred:** real HTTP, credentials, dispatcher refactor to live adapter, operator production tooling, money settlement from admin API.

---

## Unresolved external dependencies (real launch)

1. Empirical `agentId` idempotency (sequential, concurrent, 409 body).
2. Agent GET 404 eventual consistency semantics.
3. Money source + `EXTERNAL_USAGE_AUTHENTICATION`.
4. Read-scoped API keys (existence unverified).
5. Manual UI follow-up/steer outside ledger.
6. Provider hard cap absence.
7. Dispatcher `create_agent` / `follow_up` retirement path.
8. Public beta API stability.

---

## Consequences

- R6A-R2: **docs-only** commit on PR #322 branch; product SQL/Python unchanged in this stage.
- Agent J verifies errata diff + new SHA-256.
- **R6B coding** permitted **only** under **MOCK_ONLY** scope after J confirms errata.
- **PAID_AUTOMATION_READY=NO**, merge forbidden while Draft.

## References

- Agent J: `AP03C2-R6A-R1-001..007`, verdict **CONDITIONAL_DRY_RUN_ONLY**
- Prior: `AP03C2-R6A-001..010`, `AP03C2-R5-001`, `AP03C2-R5-002`
- `docs/engineering/autopilot/BUDGET_CONTROLLER_0_3C2.md`
- Queue migrations `0002`–`0007`

## Document history

| Revision | Date | Notes |
|----------|------|-------|
| R6A | 2026-10-31 | Initial freeze |
| R6A-R1 | 2026-11-01 | R6A-001..010 remediation |
| R6A-R2 | 2026-11-01 | R6A-R1-001..007 errata; MOCK_ONLY R6B; unified transitions |

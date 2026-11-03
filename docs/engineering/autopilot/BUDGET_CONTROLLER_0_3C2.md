# Budget controller 0.3C2 (dry-run foundation)

Phase 0.3C2 adds a PostgreSQL-backed reservation ledger in `autopilot_queue`
without starting Cloud Agents. Limits still come from `autopilot.config.json`
through `load_config()`; `budget.py` remains the dispatcher spend gate.

## Reservation protocol

States: `REQUESTED`, `RESERVED`, `COMMITTED`, `RELEASED`, `BLOCKED`, `OVERRUN`.

1. Mutations run only on a **tenant-bound runtime** connection
   (`autopilot_queue.bound_tenant()`). Owner/superuser connections are rejected
   (`runtime_privileged`).
2. Runtime roles **cannot** `INSERT`/`UPDATE` budget tables directly. Queue
   migration **0005** routes reserves, commits, releases, and unknown-usage
   marks through `budget_protocol_*` SECURITY DEFINER functions with a fixed
   `search_path` and minimal `EXECUTE` grants.
3. **Server-owned authority (0005):** runtime callers pass only
   `launch_id`, `reserve_cents`, usage text, and reconciliation identity.
   UTC day/month windows, per-launch caps, and charged cent amounts used for
   ledger updates are derived inside PostgreSQL from `_budget_utc_windows()`,
   `budget_global_ledger.per_launch_cap_cents` (admin-synced), and
   `_budget_charged_cents()`. Legacy 0004 signatures that accepted caller
   windows, caps, or forged charge amounts are dropped.
4. Usage counters live in `budget_window_usage` keyed by UTC day and UTC month
   buckets (global and per-tenant). Limits remain in `budget_global_ledger` and
   `budget_tenant_ledger` and are synced only via `admin_connect` (migration
   owner), not by tenant runtime.
5. `budget_window_usage` has **RLS + FORCE RLS**: tenant runtime sees global
   buckets plus its own tenant rows only; direct DML on window rows is revoked
   from runtime.
6. Duplicate `launch_id` with the same payload is idempotent; conflicting
   re-commit parameters raise `idempotency_conflict` (Python and SQL).
7. `commit_usage` records the factual `charged_text` even when it exceeds the
   reserve or global limit; state becomes `OVERRUN` when charge exceeds reserve.
   Release is blocked while `OVERRUN` is open.
8. Fractional cent amounts use **ceil** when updating ledger counters; text
   fields preserve the exact decimal string.

Money fields use `Decimal` in Python and text or bigint cents in PostgreSQL.
Unknown usage after reconciliation is flagged with `usage_unknown` and blocks
new reserves globally.

## Kill switch and runtime gate

New reserves require **both**:

- Non-zero configured limits in the trusted config snapshot.
- `runtime_enabled(config, os.environ)` — **always**, including isolated
  PostgreSQL tests (`test_budget_harness` does not bypass autopilot or budget
  gates; it only relaxes unrelated dispatcher pricing checks in `budget.py`).

**Emergency stop is never bypassed:** `AUTOPILOT_EMERGENCY_STOP=true`,
`config.emergency_stop`, or presence of `tools/bintrans_autopilot/EMERGENCY_STOP`
always block reserves.

`AUTOPILOT_ENABLED=true` is **required** (together with config) for reserves.
Setting the env var to `true` does **not** by itself block spending; disabling
autopilot (`autopilot_enabled=false` or missing env) blocks reserves.

Additional blocks: `PAID_AUTOMATION_ENABLED=true` at controller construction,
open `OVERRUN`, open reservations with `usage_unknown=true`, or unavailable
budget store.

Inside `budget_protocol_*`, `_budget_lock_global_gate()` locks the global
ledger row before reserve/commit/release/unknown transitions, and
`_budget_assert_safety_after_lock()` re-checks overrun/unknown blocks so
decisions are tied to current gate state within one database transaction.
Python still performs a fail-closed pre-check via
`budget_global_safety_block_reason()` before calling the protocol; concurrent
cross-transaction races are bounded by the SQL gate lock, not by Python alone.

## Usage reconciliation

`usage_reconciliation.py` reads Cursor `GET /v1/agents/{id}/usage` through an
injected client or fetcher. It never calls `create_agent`. Malformed usage
(`NaN`, invalid IDs) returns `failed` without crashing the worker.

**Verified usage authority (0006):** reserves may bind `agent_id` and `run_id`
at `request_reserve` time; reconciliation and commit enforce that binding.
Tenant runtime cannot commit charged amounts. Only the `autopilot_queue_reconciler` role may call
`budget_reconciler_apply_usage`, which stores a canonical payload digest and
`verification_id` in `budget_verified_usage` and commits via
`_budget_commit_from_verified()` (amounts read from that row, not caller args).
Missing, empty, or absent `chargedCents` is **unknown**, not zero; verified
`chargedCents=0` requires a reconciler-recorded payload. Runtime `commit_usage`
requires `reconcile_connect` plus verification refs derived from the API payload.

## OVERRUN recovery

`OVERRUN` rows are audit-only in 0.3C2. Clearing them requires operator action
on the migration-owner connection (not tenant runtime). New reserves stay blocked
while any `OVERRUN` exists.

## Migrations

Queue migrations:

- **0002** — base ledger/reservation tables and RLS read policies.
- **0003** — global ledger, reservation window columns, initial SD helpers.
- **0004** — `budget_window_usage`, revoke runtime DML, initial `budget_protocol_*`.
- **0005** — server-owned protocol signatures, window RLS, revoke runtime
  `budget_global_lock_row`, tighten helper EXECUTE.
- **0006** — `budget_verified_usage`, reconciler role, server-owned commit from
  verified rows; runtime loses direct commit/mark-unknown EXECUTE.
- **0007** — release safety (no runtime release after identity/unknown/verified
  touch), global `run_id` uniqueness, paired agent/run on reserve.

Platform migration head stays **`000105`**.

## Release and identity (0007)

Runtime `release` is allowed only for **pre-launch** reserves: no `agent_id`/`run_id`,
no `usage_unknown`, and no `budget_verified_usage` row. After reconciliation or
identity binding, only reconciler-verified usage (including verified zero) may close
the financial outcome. `run_id` is globally unique across reservations and verified
usage. Historical unbound reserves remain valid; the first reconciler pass binds
identity under uniqueness checks.

## External usage trust (0.3D / 0.3E blocker)

`EXTERNAL_USAGE_SOURCE_AUTHENTICATION=NOT_PROVEN` — payload digest and reconciler
role harden storage and commit paths, but **0.3C2 does not prove** TLS/API-key
authenticity of Cursor Usage API responses. Paid launch phases must add verified
credentials, response validation, and non-spoofable ingestion before production
budget closure.

Outstanding LOW (not closed in R5): AP03C2-R4-003..004 and prior deferred LOW;
track in Agent J full audit.

## Test-only notes

- `down()` without `allow_destructive_test` may delete committed history; allowed
  only on isolated `*_test` databases (see queue migration guards).
- `test_budget_harness` in config enables PostgreSQL integration tests with
  non-zero limits; it must remain `false` in committed `autopilot.config.json`.
- Integration tests set `AUTOPILOT_ENABLED=true` and `autopilot_enabled=true`
  in config explicitly; they do not disable emergency stop or budget limits.

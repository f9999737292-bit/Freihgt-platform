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
   migration **0004** routes reserves, commits, releases, and unknown-usage
   marks through `budget_protocol_*` SECURITY DEFINER functions.
3. Usage counters live in `budget_window_usage` keyed by UTC day and UTC month
   buckets (global and per-tenant). Limits remain in `budget_global_ledger` and
   `budget_tenant_ledger` and are synced only via `admin_connect` (migration
   owner), not by tenant runtime.
4. Duplicate `launch_id` with the same payload is idempotent.
5. `commit_usage` records the factual `charged_text` even when it exceeds the
   reserve or global limit; state becomes `OVERRUN` when charge exceeds reserve.
   Release is blocked while `OVERRUN` is open.
6. Fractional cent amounts use **ceil** when updating ledger counters; text
   fields preserve the exact decimal string.

Money fields use `Decimal` in Python and text or bigint cents in PostgreSQL.
Unknown usage after reconciliation is flagged with `usage_unknown` and blocks
new reserves globally.

## Kill switch and runtime gate

New reserves require **both**:

- Non-zero configured limits in the trusted config snapshot.
- `runtime_enabled(config, os.environ)` unless `test_budget_harness=true` in
  **isolated tests only** (never in production config).

**Emergency stop is never bypassed:** `AUTOPILOT_EMERGENCY_STOP=true`,
`config.emergency_stop`, or presence of `tools/bintrans_autopilot/EMERGENCY_STOP`
always block reserves, even when `test_budget_harness` is set.

`AUTOPILOT_ENABLED=true` is **required** (together with config) for reserves
when not in the test harness. Setting the env var to `true` does **not** by
itself block spending; disabling autopilot (`autopilot_enabled=false` or missing
env) blocks reserves.

Additional blocks: `PAID_AUTOMATION_ENABLED=true` at controller construction,
open `OVERRUN`, open reservations with `usage_unknown=true`, or unavailable
budget store.

Inside `budget_protocol_reserve`, `budget_global_safety_block_reason()` is
checked again **after** locking the global ledger row (closes TOCTOU against
concurrent overrun/unknown transitions).

## Usage reconciliation

`usage_reconciliation.py` reads Cursor `GET /v1/agents/{id}/usage` through an
injected client or fetcher. It never calls `create_agent`. Malformed usage
(`NaN`, invalid IDs) returns `failed` without crashing the worker.

## OVERRUN recovery

`OVERRUN` rows are audit-only in 0.3C2. Clearing them requires operator action
on the migration-owner connection (not tenant runtime). New reserves stay blocked
while any `OVERRUN` exists.

## Migrations

Queue migrations:

- **0002** — base ledger/reservation tables and RLS read policies.
- **0003** — global ledger, reservation window columns, initial SD helpers.
- **0004** — `budget_window_usage`, revoke runtime DML, `budget_protocol_*`
  functions.

Platform migration head stays **`000105`**.

## Test-only notes

- `down()` without `allow_destructive_test` may delete committed history; allowed
  only on isolated `*_test` databases (see queue migration guards).
- `test_budget_harness` in config enables PostgreSQL integration tests with
  non-zero limits; it must remain `false` in committed `autopilot.config.json`.

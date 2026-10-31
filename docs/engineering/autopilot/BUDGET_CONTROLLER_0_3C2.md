# Budget controller 0.3C2 (dry-run foundation)

Phase 0.3C2 adds a PostgreSQL-backed reservation ledger in `autopilot_queue`
without starting Cloud Agents. Limits still come from `autopilot.config.json`
through `load_config()`; `budget.py` remains the dispatcher spend gate.

## Reservation protocol

States: `REQUESTED`, `RESERVED`, `COMMITTED`, `RELEASED`, `BLOCKED`.

1. `request_reserve(launch_id, tenant_id, reserve_cents)` runs under a DB
   transaction with `FOR UPDATE` on the tenant ledger row.
2. Duplicate `launch_id` with the same payload is idempotent.
3. `commit_usage` moves reserved cents to committed cents when `charged_known`
   is true. Unknown charges stay on `RESERVED` and block new launches.
4. `release` returns unused reserved cents.

Money fields use `Decimal` in Python and text or bigint cents in PostgreSQL.
Unknown usage is not treated as zero.

## Kill switch

New reserves are blocked when Autopilot or paid automation env flags are on,
`emergency_stop` is set, configured limits are zero, the budget store is
unavailable, or any open reservation has unknown usage.

## Usage reconciliation

`usage_reconciliation.py` reads Cursor `GET /v1/agents/{id}/usage` through an
injected client or fetcher. It never calls `create_agent`. API keys are not
stored in the database.

## Migrations

Queue migration version 2 adds `budget_tenant_ledger` and
`budget_reservations`. Platform migration head stays `000105`.

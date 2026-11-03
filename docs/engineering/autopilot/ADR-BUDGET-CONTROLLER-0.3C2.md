# ADR: PostgreSQL budget reservations for Autopilot 0.3C2

## Status

Accepted for draft PR 0.3C2.

## Context

Autopilot needs durable, concurrent-safe budget reservations before any future
Cloud Agent launch. File-based `usage_journal.json` is historical only.
`budget.py` performs pre-launch spend polling for the dispatcher but does not
record per-launch reservations.

## Decision

- Extend `autopilot_queue` with migration `0002_budget` and a
  `BudgetController` that reads limits from the existing config loader.
- Keep reservation and ledger updates inside PostgreSQL transactions with row
  locks on `budget_tenant_ledger`.
- Reconcile Cursor usage through a separate module that never creates agents.
- Leave `autopilot.config.json` switches and zero budgets unchanged in this
  phase.

## Consequences

- Queue rollback tests continue on migration version 1; version 2 is dropped
  through `MigrationRunner.down_budget_migration()` in queue tests.
- A future launch path must call `request_reserve` before any paid API use and
  `commit_usage` or `release` afterward.
- Unknown `chargedCents` keeps the reservation open and blocks further reserves
  until reconciliation succeeds or the row is released manually in tests.

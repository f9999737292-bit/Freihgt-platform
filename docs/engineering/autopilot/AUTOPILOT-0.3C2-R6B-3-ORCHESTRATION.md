# AUTOPILOT-0.3C2-R6B-3 — Mock-only orchestration contract

Status: implementation contract for an isolated mock runner. This document does not authorize 24/7 operation, `MOCK_ONLY` in a shared environment, paid launch, or merge.

`BT_01B_BUSINESS_APPROVAL=NOT_VERIFIED`

`MOCK_ONLY_ACTIVATION_AUTHORIZED=NO`

`REAL_24_7_AUTHORIZED=NO`

## Architecture gate

No new financial rule. Settlement stays on the merged R6B-2 functions. A verified charge above the reservation remains the existing BT-01b `OVERRUN` record and blocks the next reserve with `overrun_open`. That behavior is reproduced only in an isolated test database.

No new PostgreSQL role, `GRANT`, or `SECURITY DEFINER` function. The runner uses the existing runtime, launch-adapter, and reconciler principals. Migrations `0008`–`0015` are not edited. Platform migration head stays `000105`. There is no migration `0016`.

No second queue, scheduler, or budget ledger. `usage_journal.py` remains the historical Cloud Agent file and is not the orchestration journal.

## Components reused

| Concern | Owner |
| --- | --- |
| Accept and idempotent enqueue | `PostgresQueue.enqueue` |
| Priority claim, lease, renew, complete, fail, expiry | `PostgresQueue` |
| Registered dry-run handlers `docs` and `ping` | `AutopilotWorker` (unchanged allowlist) |
| Mock create result | `MockCursorTransport` |
| Reserve and kill-switch counters | `BudgetController` |
| Intent, send, outbound proof, confirm, definite reject | Existing `launch_protocol_*` functions |
| Settlement | `BudgetController.commit_usage` → `budget_reconciler_apply_usage` |
| Orchestration audit | New append-only `AuditJournal` (JSONL). Not a ledger. |

`mock_launch` is claimed only by `MockOrchestrator`. `AutopilotWorker` does not gain that job type, so it cannot run a launch.

## State machine

Trusted server code builds `MockTask`. The payload may carry only `launch_id`, `agent_id`, `run_id`, `scenario`, `reserve_cents`, and `charged_cents`. It cannot name a Python callable, shell command, URL, or credential.

1. `submit` reads `autopilot_launch_control` and `BudgetController.evaluate_kill_switch`. `estop_active`, a mode other than `MOCK_ONLY`, or a budget block refuses the task before enqueue.
2. Enqueue is idempotent on `(tenant_id, idempotency_key)`. A duplicate key does not create a second job or a second reserve.
3. `run_once` requeues expired leases, re-reads the control row, then claims one job for this tenant ordered by priority, creation time, and job id.
4. The handler reads the payload only with the live lease token.
5. Reserve (runtime) → `launch_protocol_record_intent` (adapter) → `mark_send_started` → in-process `MockCursorTransport` → `report_outbound_result` with `MOCK_TRUSTED` → `confirm` → `commit_usage` (reconciler).
6. `definite_reject` reports `MOCK_TRUSTED` and calls `reject_definite`, which releases the unused reserve. No settlement.
7. `timeout` and `ambiguous` leave the launch at `SEND_STARTED` and return the job to `PENDING` until attempts are exhausted.
8. `conflict` and a proof class other than `MOCK_TRUSTED` do not confirm and do not settle.
9. A later attempt resumes from the stored launch and reservation. `COMMITTED` or `OVERRUN` only completes the queue job.

Queue states remain `PENDING`, `RUNNING`, `SUCCEEDED`, `FAILED`, `DEAD`. Launch states remain the R6B lifecycle. The orchestrator does not invent states.

## Identity, lease, and recovery

Tenant identity is the database principal in `tenant_principals`, checked by `bound_tenant()`. The task's tenant string must match that principal. Another tenant's runtime cannot claim the job.

The queue lease token is the queue fence. After expiry, `requeue_expired` clears it. A stale owner cannot `complete` or `fail`. Launch recovery tokens are not read or required for this settlement path. A NULL recovery token remains rejected by `_launch_fence_ok` for fenced recovery functions.

Two orchestrators on one tenant take different jobs via `FOR UPDATE SKIP LOCKED`. The same job has one live lease.

If PostgreSQL is unavailable, the tick returns `unavailable` and does not mark the job succeeded. A crash after confirm and before settlement leaves `LAUNCH_CONFIRMED` and `RESERVED`. The next tick settles once through the existing idempotent verification key. A crash before confirm does not charge.

`0015 → 0014` rollback is not invoked. It still restores the previously vulnerable function bodies and stays limited to an isolated test database.

## Kill switch

`autopilot_estop` or a mode other than `MOCK_ONLY` stops the runner before claim and again inside the handler. The product default remains `DISABLED` and `autopilot_paid_launch_enabled=false`. This runner does not write those flags. Tests may set `MOCK_ONLY` only on an isolated database.

`AUTOPILOT_ENABLED` is not set by this module. The committed `autopilot.config.json` flag stays false. A config with `autopilot_enabled=true` is accepted only together with `test_budget_harness=true`, which is the existing budget-test exception and is not a production activation.

## Audit events

Each record is one JSON object: `event`, `tenant_id`, optional `job_id`, optional `launch_id`, `result`. Lease tokens, DSNs, passwords, and proof blobs are not fields. Events include `task_submitted`, `job_claimed`, `launch_intent`, `launch_confirmed`, `budget_settled`, `job_completed`, `job_failed`, `kill_switch`, `database_unavailable`, and `orchestrator_stopped`.

## Carried findings

Ten LOW findings remain open: the nine recorded in the R6 lifecycle ADR (R4-001, R4-002, R4-003, R4-004, R6B1-011, R6B1-012 elevated to the fixed HIGH R6B2-003, adapter read of `recovery_lease_token`, v11 rollback on an isolated test database, dedicated-cluster tests) and the unchanged `_launch_fence_ok` search_path nit. `ROLLBACK_0015_TO_0014_RISK=OPEN`.

# AUTOPILOT-0.3C2-R6B-3 — Mock-only orchestration contract

Status: implementation contract for an isolated mock runner. This document does not authorize 24/7 operation, `MOCK_ONLY` in a shared environment, paid launch, or merge.

`BT_01B_BUSINESS_APPROVAL=NOT_VERIFIED`

`MOCK_ONLY_ACTIVATION_AUTHORIZED=NO`

`REAL_24_7_AUTHORIZED=NO`

## Architecture gate

No new financial rule. Settlement stays on the merged R6B-2 functions. A verified charge above the reservation remains the existing BT-01b `OVERRUN` record and blocks the next reserve with `overrun_open`. That behavior is reproduced only in an isolated test database.

No new PostgreSQL role, `GRANT`, or `SECURITY DEFINER` function. The runner uses the existing runtime, launch-adapter, recovery, and reconciler principals. Migrations `0008`–`0015` are not edited. Platform migration head stays `000105`. There is no migration `0016` on this branch. The runner does not call `launch_protocol_manual_reject`. That admin call is valid only from `MANUAL_RECONCILIATION_REQUIRED`.

No second queue, scheduler, or budget ledger. `usage_journal.py` remains the historical Cloud Agent file and is not the orchestration journal.

## Components reused

| Concern | Owner |
| --- | --- |
| Accept and idempotent enqueue | `PostgresQueue.enqueue` |
| Priority claim, lease, renew, complete, fail, expiry | `PostgresQueue` |
| Registered dry-run handlers `docs` and `ping` | `AutopilotWorker` (unchanged allowlist) |
| Mock create result | `MockCursorTransport` |
| Reserve and kill-switch counters | `BudgetController` |
| Intent, send, outbound proof, confirm, definite reject, unknown, recovery poll, escalate | Existing `launch_protocol_*` functions |
| Settlement | `BudgetController.commit_usage` → `budget_reconciler_apply_usage` |
| Orchestration audit | New append-only `AuditJournal` (JSONL). Not a ledger. |

`mock_launch` is claimed only by `MockOrchestrator`. `AutopilotWorker` does not gain that job type, so it cannot run a launch.

## State machine

Trusted server code builds `MockTask`. The payload may carry only `launch_id`, `agent_id`, `run_id`, `scenario`, `reserve_cents`, and `charged_cents`. It cannot name a Python callable, shell command, URL, or credential.

1. `submit` reads `autopilot_launch_control` and `BudgetController.evaluate_kill_switch`. `estop_active`, a mode other than `MOCK_ONLY`, or a budget block refuses the task before enqueue.
2. Enqueue is idempotent on `(tenant_id, idempotency_key)`. A duplicate key does not create a second job or a second reserve.
3. `run_once` requeues expired leases, re-reads the control row, then claims one job for this tenant ordered by priority, creation time, and job id.
4. The handler reads the payload only with the live lease token.
5. Reserve (runtime) → `launch_protocol_record_intent` (adapter) → `mark_send_started` → one in-process `MockCursorTransport` call → `report_outbound_result` with `MOCK_TRUSTED` → `confirm` → `commit_usage` (reconciler).
6. Confirmed identity is `mock_identity(tenant_id, launch_id)`. Payload `agent_id` and `run_id` stay in the queue schema and are not the launch identity. The transport returns that same derived pair. A second launch therefore does not reuse the historical transport constants.
7. `definite_reject` reports `MOCK_TRUSTED` and calls `reject_definite`, which releases the unused reserve. No settlement.
8. `timeout`, `ambiguous`, `conflict`, and a transport crash leave the launch at `SEND_STARTED` and return the job to `PENDING`. The next claim does not call the transport. The adapter marks `SEND_STARTED` as `LAUNCH_UNKNOWN`. A stored `LAUNCH_UNKNOWN` is a resume point: the next claim does not call the transport and does not create another mock agent. The existing recovery role then polls on one backend until `max_recovery_polls` and calls `escalate_manual` before that connection closes. A later backend does not receive the live recovery token. The queue job becomes `FAILED` with `manual_reconciliation`. The reserve stays `RESERVED`. A failure while obtaining the recovery connection is `store_unavailable`: `run_once` records `job_failed`, returns the job to `PENDING`, and leaves the launch at `LAUNCH_UNKNOWN`. The following claim continues that recovery. It does not settle and it does not release the reserve.
9. A proof class other than `MOCK_TRUSTED` does not confirm and does not settle.
10. A later attempt resumes from the stored launch and reservation. `COMMITTED` or `OVERRUN` only completes the queue job. `LAUNCH_CONFIRMED` settles once.

Queue states remain `PENDING`, `RUNNING`, `SUCCEEDED`, `FAILED`, `DEAD`. Launch states remain the R6B lifecycle. The orchestrator does not invent states.

## Identity, lease, and recovery

Tenant identity is the database principal in `tenant_principals`, checked by `bound_tenant()`. The task's tenant string must match that principal. Another tenant's runtime cannot claim the job.

The queue lease token is the queue fence. After expiry, `requeue_expired` clears it. A stale owner cannot `complete` or `fail`. Settlement still does not read a recovery token. Unconfirmed sends use the existing recovery lease: acquire, poll, and escalate run on one recovery connection so the holder stays `session_user || '#' || pg_backend_pid()`. A NULL recovery token remains rejected by `_launch_fence_ok`. The orchestrator does not acquire that lease for settlement.

Two orchestrators on one tenant take different jobs via `FOR UPDATE SKIP LOCKED`. The same job has one live lease.

If PostgreSQL is unavailable, the tick returns `unavailable` and does not mark the job succeeded. A crash after confirm and before settlement leaves `LAUNCH_CONFIRMED` and `RESERVED`. The next tick settles once through the existing idempotent verification key. A crash before confirm does not charge.

`0015 → 0014` rollback is not invoked. It still restores the previously vulnerable function bodies and stays limited to an isolated test database.

## Operator exit

`LAUNCH_UNKNOWN` and `MANUAL_RECONCILIATION_REQUIRED` are different states. Both make `budget_global_safety_block_reason()` return `launch_reconciliation_open` for every tenant. Neither state is a confirmed zero charge, and neither releases the reserve by itself.

`autopilot_launch_admin` may call `launch_protocol_manual_reject(tenant_id, launch_id, reason)` only while the launch is `MANUAL_RECONCILIATION_REQUIRED`. That call writes `manual_reconciliation_audit`, sets `LAUNCH_REJECTED`, and releases the unused reserve through the existing release helper. The same call from `LAUNCH_UNKNOWN` raises `invalid_caller_or_state` and does not change the reserve. `launch_protocol_manual_confirm` has the same state restriction. Admin cannot call `launch_protocol_escalate_manual`.

`autopilot_launch_recovery` is the role that leaves `LAUNCH_UNKNOWN`. The allowed calls, on one connection and in this order, are `launch_protocol_recovery_lease_acquire`, `launch_protocol_recovery_poll_attempt` until `decision = MANUAL`, then `launch_protocol_escalate_manual` with that lease token. Escalate requires the poll count to have reached `max_recovery_polls` and a live fence. It sets `MANUAL_RECONCILIATION_REQUIRED` and does not release the reserve. After that transition the admin principal may reject, which clears `launch_reconciliation_open`.

A second recovery backend that finds a live lease receives `lease_held_by_other` and does not take the token. After `recovery_lease_expires_at`, a later recovery attempt may acquire a new token. `autopilot_estop` during this phase returns the queue job to `PENDING` and does not settle or release. Settlement stays refused until the launch is `LAUNCH_CONFIRMED` or `EXTERNAL_AGENT_CANCELLED`.

## Kill switch

`autopilot_estop` or a mode other than `MOCK_ONLY` stops the runner before claim and again inside the handler. A claimed job is returned to `PENDING` so a later runner can resume after the gate opens. A confirmed launch is not settled while the gate is closed, and it is not marked `FAILED`. The product default remains `DISABLED` and `autopilot_paid_launch_enabled=false`. This runner does not write those flags. Tests may set `MOCK_ONLY` only on an isolated database.

`AUTOPILOT_ENABLED` is not set by this module. The committed `autopilot.config.json` flag stays false. A config with `autopilot_enabled=true` is accepted only together with `test_budget_harness=true`, which is the existing budget-test exception and is not a production activation.

## Audit events

Each record is one JSON object: `event`, `tenant_id`, optional `job_id`, optional `launch_id`, `result`. Lease tokens, DSNs, passwords, and proof blobs are not fields. Events include `task_submitted`, `job_claimed`, `launch_intent`, `launch_unknown`, `recovery_poll`, `launch_confirmed`, `budget_settled`, `job_completed`, `job_failed`, `kill_switch`, `database_unavailable`, and `orchestrator_stopped`.

## Carried findings

Ten LOW findings remain open: the nine recorded in the R6 lifecycle ADR (R4-001, R4-002, R4-003, R4-004, R6B1-011, R6B1-012 elevated to the fixed HIGH R6B2-003, adapter read of `recovery_lease_token`, v11 rollback on an isolated test database, dedicated-cluster tests) and the unchanged `_launch_fence_ok` search_path nit. `ROLLBACK_0015_TO_0014_RISK=OPEN`.

Five earlier R6B-3 LOW findings and four R6B-3-R1 LOW findings stay open. They are not closed by this recovery remediation:

| ID | Item |
| --- | --- |
| R6B3-005 | `scenario` is still a marker at enqueue, not a closed list in `PostgresQueue` |
| R6B3-006 | Locked-off environment flags still match only `true`, and the queue enabled flags stay hard false |
| R6B3-007 | `AuditJournal` has no size limit, value filter, or tenant file split |
| R6B3-008 | Each operation opens its own connection; 24/7 connection policy is not defined |
| R6B3-009 | Tenant runtime can update its own queue row; launch and budget rows remain the evidence |
| R6B3R1-013 | Mock transport falls back to historical constants when the body omits tenant and launch ids; confirm still checks `identity_mismatch` |
| R6B3R1-014 | `recovery_pending` remains in the retry set and is not reached by the poll loop, which exits through `manual_reconciliation` |
| R6B3R1-015 | Adapter `mark_unknown` does not apply `send_stale_seconds`; recovery acquire treats `LAUNCH_UNKNOWN` as already stale |
| R6B3R1-016 | The orchestrator process holds a fourth database principal for recovery |

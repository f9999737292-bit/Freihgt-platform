# Autopilot 24/7 readiness 0.4A

Status: dry-run readiness only. This stage does not enable a resident worker, a paid launch, merge, or deploy.

```text
AUTOPILOT_ENABLED=NO
PAID_AUTOMATION_ENABLED=NO
EXTERNAL_TOOLS_ENABLED=NO
AUTO_MERGE=NO
AUTO_DEPLOY=NO
WORKER_ENABLED=NO
PRODUCTION_24X7_ENABLED=NO
```

Base used for the gap review: `origin/main` at `a9ef8f40bf0e7f75cb4aee3794e7841723347e4c`.

## What was already on main

| Piece | Where | State |
| --- | --- | --- |
| File scheduler, lease, fence, restart | `scheduler.py`, `task_queue.py` | Present. Dry-run only. |
| PostgreSQL queue, claim, lease, retry, tenant RLS | `postgres_queue.py`, migrations `0001`–`0015` | Present. Isolated schema `autopilot_queue`. |
| Dry-run worker | `worker.py` | Present. Finite `run_once` / `run_pending`. Payload is not executed. |
| Budget and launch fencing | `0008`–`0015`, `budget_controller.py` | Present. Not widened here. |
| GitHub dispatcher | `dispatcher.py` | Present and disabled by `autopilot.config.json`. |
| PR #318 queue foundation | merged `7688ab7a7ca4696f6e96b0862f222856006f821c` | On main. |
| PR #320 dry-run worker | merged `e031d90c6306b7e6b1aa0a7621e3be4484e0d8dc` | On main. |

`AP03C1-001` is closed on this base. `AutopilotWorker._run_acquired` keeps the lost-lease code in a local dict for that claim. One job's `lease_expired` does not complete another in-flight job. The regression is `test_one_lease_loss_does_not_cancel_the_other_job`.

`AP03C1-002` is closed on this base. `snapshot` calls `_assert_runtime_binding` before the counts. A table owner or a mismatched tenant principal is rejected. The regression is `test_snapshot_is_bound_to_the_worker_tenant`.

Those two tests need `BINTRANS_AUTOPILOT_QUEUE_DSN`. They are not turned into passes when the variable is absent.

## What this stage adds

| Gap | Addition |
| --- | --- |
| No restart budget above the worker | `WorkerSupervisor` stores the restart count for its tenant in a hash-chained file. A new worker id or a new supervisor using that file does not reset the count. A decision does not complete a job. |
| No independent audit packet | `AuditGateway` appends claims to a hash-chained journal. A reviewer string in the packet is not an identity. The finding gate stays `blocked` until a trusted workflow can read a GitHub review and a CI result for the exact SHA. See `AUDIT_TRUST_BOUNDARY_0_4A_R2.md`. |
| No single readiness report | `ReadinessCycle.run` evaluates one batch. `python -m bintrans_autopilot readiness-check` prints the closed switches and exits. It does not claim a job. |
| No dedicated deployment sketch | `deploy/autopilot-readiness.compose.example.yml` is an example. It is not part of the platform compose. The service is on profile `manual-approval-required`, `restart` is `no`, and the image is `example.invalid`. This stage does not run it. |

The scheduler and the PostgreSQL queue are reused. No platform migration is added. The autopilot schema head stays `0015`.

## Failure behaviour

| Case | Result |
| --- | --- |
| PostgreSQL unavailable | `run_once` returns `unavailable`. The job is not completed. |
| Worker handler raises | `fail` is called and the job is not completed. A stale heartbeat asks for a bounded restart, then a hold. |
| Lease lost or stale | `complete` with the old token returns `failed`. `jobs_completed_total` does not move. |
| Two workers, one job | The second `claim` gets nothing. One completion is stored. |
| Scheduler process restarted | The same task id is not inserted again. |
| Handler returned and `complete` did not | The outcome is `unavailable` or `failed`. The metric stays at the previous total. |
| Lease expired, then a new claim | The next `run_once` can complete with a new token. The first token does not. |
| Payload contains `command` or another unknown field | `enqueue` raises `invalid_input` before it connects. |
| Agent J returns `CHANGES_REQUIRED`, `BLOCKED`, or a CRITICAL/HIGH finding | The journal keeps a block for that SHA. A later PASS packet does not remove it. |
| Dead-job cap or restart cap | The supervisor holds. `release_hold` does not clear the stored count. |
| Emergency stop | Env, the stop file, or `emergency_stop` in config stops the supervisor even when the caller passes false. |

## Recovery

1. Leave `AUTOPILOT_ENABLED` unset or not `true`. To halt, set `AUTOPILOT_EMERGENCY_STOP=true` or add an empty `tools/bintrans_autopilot/EMERGENCY_STOP` through the normal review path.
2. Do not delete `autopilot_queue.jobs` and do not complete a row with an expired token.
3. After PostgreSQL returns, a later dry-run `run_once` calls `requeue_expired` and may claim a `PENDING` row. A `RUNNING` row stays until its lease time passes.
4. A supervisor hold stays in the restart ledger. `release_hold` refuses to clear it. That does not start a resident process.
5. If Agent J recorded `CHANGES_REQUIRED` or `BLOCKED`, keep the packet. Do not replace the verdict with `PASS` inside this process.
6. Do not start `deploy/autopilot-readiness.compose.example.yml` from this stage.

## Not ready

Continuous unattended execution, a paid Cloud Agent, merge to `main`, and deployment are not enabled. The finding gate cannot turn itself clear. The missing proof is described in `AUDIT_TRUST_BOUNDARY_0_4A_R2.md`.

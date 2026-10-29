# Autopilot 0.3A compatibility contract

Baseline: `origin/main` `116d7c9544717ead8b6e3745e563b4cd49b87de4` (merge of PR #317). This contract is the boundary for the 0.3B PostgreSQL queue. It is recorded before that queue is added.

## What stays

- `tools/bintrans_autopilot/task_queue.py` remains the local JSON queue. Its `FileLock` remains a single-host OS lock. It is not an NFS lock and it is not the multi-worker mechanism.
- Planning states stay `CREATED`, `READY`, `CLAIMED`, `RUNNING`, `REVIEW_REQUIRED`, `COMPLETED`, `BLOCKED`, `FAILED`, `CANCELLED`.
- `tools/bintrans_autopilot/scheduler.py` stays `DRY_RUN`. It does not import the Cursor client. An empty, relative, or missing workspace root still blocks. Symlink escape still blocks.
- `states.py` Issue states are unchanged.
- `autopilot.config.json` stays off. `autopilot_enabled` is false. Budget caps stay `0`. A missing charge stays null.
- Platform migrations stay headed by `000105_platform_postgres_role_foundation_v0_2_i1`. The E2E stand still requires `standSchemaVersion = 105` and a last file prefixed `000105_`. No platform migration `000106` is added. The platform migration table is not written by this queue.

## What 0.3B adds beside that

- A separate schema, `autopilot_queue`, with its own `autopilot_queue.schema_migrations` table.
- Statuses `PENDING`, `RUNNING`, `SUCCEEDED`, `FAILED`, `DEAD`. These names are not the 0.3A planning states.
- Claim uses a PostgreSQL transaction and `FOR UPDATE SKIP LOCKED`. The transaction commits before any handler runs.
- Ownership after commit is a lease token. Renew, complete, and fail must present the current token. A later claim mints a new token. The old token cannot write.
- Delivery is at-least-once. A committed queue row does not prove that an external side effect ran once, or that it did not run. Handlers must be idempotent. This foundation does not call a handler from `Claim`.

## Disabled by default

The queue object refuses `autopilot_enabled` or `paid_automation_enabled`. Importing the module does not start a worker, a Cloud Agent, a merge, or a deploy. The test worker is an in-process dry run and performs no external call.

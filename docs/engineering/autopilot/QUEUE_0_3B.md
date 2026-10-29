# Autopilot queue 0.3B

The compatibility boundary is [QUEUE_0_3A_COMPATIBILITY.md](QUEUE_0_3A_COMPATIBILITY.md). This phase adds a PostgreSQL queue beside the 0.3A file queue. It does not replace that queue, and it does not change platform migration `000105`.

## Schema

`autopilot_queue` is a separate schema. `autopilot_queue.schema_migrations` is its only version table. The runner applies `tools/bintrans_autopilot/sql/autopilot_queue/*.up.sql` and rolls the latest version back with the matching down file. Version 1 down drops the schema. It does not drop the `NOLOGIN` role `autopilot_queue_runtime`, because a role is cluster-wide.

The runtime role may `SELECT`, `INSERT`, and `UPDATE` `autopilot_queue.jobs`. It cannot delete jobs, write the version table, or create a `LOGIN`. No platform role is granted access.

A job stores its id, type, payload version, payload, tenant, status, priority, timestamps, attempt count, attempt limit, backoff, lease owner, lease token, lease deadline, last error code, and idempotency key. The idempotency key is unique per tenant. Payload text that looks like a token or a password is rejected before a connection is used.

Statuses are `PENDING`, `RUNNING`, `SUCCEEDED`, `FAILED`, and `DEAD`.

## API

`Enqueue`, `Claim`, `RenewLease`, `Complete`, `Fail`, `RequeueExpired`, and `GetStatus` are methods on `PostgresQueue`. `Claim` locks one due `PENDING` row with `FOR UPDATE SKIP LOCKED`, sets `RUNNING`, mints a new lease token, and commits. The handler runs only after that commit. `RenewLease`, `Complete`, and `Fail` require the same tenant, owner, and token, and a lease that has not expired.

`Fail` returns the job to `PENDING` with backoff while attempts remain. The next delay is `backoff_seconds * 2^(attempt-1)`, capped at 3600 seconds. The attempt limit moves the job to `DEAD`. A non-retryable failure moves it to `FAILED`. `RequeueExpired` applies the same limit to a crashed `RUNNING` lease. A new claim after that receives a new token. The previous token cannot complete the job.

This is at-least-once delivery. The lease token stops a stale owner from writing queue state. It does not prove that an external side effect ran once.

`GetStatus` is tenant-scoped and does not return the payload. Another tenant gets no row.

## Closed switches

`PostgresQueue` refuses `autopilot_enabled` or `paid_automation_enabled`. `DryRunWorker.handle_once` claims one job and completes it in-process. It does not call Cursor, GitHub, or a deploy. Importing the module starts nothing. Logs record the event, tenant, job id, and result. They do not record the payload or a DSN.

## Tests

Set `BINTRANS_AUTOPILOT_QUEUE_DSN` to an empty PostgreSQL database and install `tools/bintrans_autopilot/requirements-queue.txt`. The CI job `bintrans-autopilot-queue` starts that database itself. It is not the platform database and it is not the E2E stand. Without the variable, the PostgreSQL cases are skipped rather than reported as passed.

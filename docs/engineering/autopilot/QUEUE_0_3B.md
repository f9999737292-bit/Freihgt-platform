# Autopilot queue 0.3B

The compatibility boundary is [QUEUE_0_3A_COMPATIBILITY.md](QUEUE_0_3A_COMPATIBILITY.md). This phase adds a PostgreSQL queue beside the 0.3A file queue. It does not replace that queue, and it does not change platform migration `000105`.

## Schema

`autopilot_queue` is a separate schema. `autopilot_queue.schema_migrations` is its only version table. The runner applies `tools/bintrans_autopilot/sql/autopilot_queue/*.up.sql`. Version 1 remains the queue version. A normal rollback is refused when job rows exist, when the schema contains an unexpected table or function, when another schema depends on it, or when the applied version is not exactly 1. The down file drops only the known objects and does not use `CASCADE`. A destructive rollback is available only when the caller sets `allow_destructive_test` and the current database name is an isolated `*_test` database with no platform migration table and no extra application schema. The `NOLOGIN` role is cluster-wide and is not dropped.

`autopilot_queue_runtime` is `NOLOGIN`. It may `SELECT` and `INSERT` jobs, and `UPDATE` only the execution columns. It cannot update `tenant_id`, delete jobs, write the version table, or write `tenant_principals`. A migration refuses to adopt this role when it can log in, is superuser, bypasses row security, can create roles or databases, can replicate, or is a member of another role. The migration does not alter an unexpected role.

Tenant identity is the authenticated `session_user`. The migration owner registers that principal in `tenant_principals`. `bound_tenant()` reads the mapping as a definer function. Row security is enabled and forced on `jobs`, with `SELECT`, `INSERT`, and `UPDATE` policies. The queue does not consult `app.tenant_id` or any other caller-set GUC. A session is rejected when `session_user` or `current_user` is the table owner, a superuser, `BYPASSRLS`, or can create roles, databases, or replication. `row_security` must stay on. The caller-supplied `tenant_id` must match the principal; it is not authorization by itself. `SET ROLE` does not change `session_user`, so it cannot select another tenant.

A job stores its id, type, payload version, payload, tenant, status, priority, timestamps, attempt count, attempt limit, backoff, lease owner, lease token, lease deadline, last error code, and idempotency key. The idempotency key is unique per tenant. A repeat is the same job only when `job_type`, `payload_version`, `payload_digest`, `priority`, `max_attempts`, and `backoff_seconds` all match. Any other combination returns `idempotency_conflict`.

Payloads are accepted only for a known job type and a fixed field schema. Credential field names, nested credential fields, and URL or DSN values with userinfo are rejected before a connection opens. A pattern match is not treated as a detector for every possible token. Secret material stays in a secret store; a payload may carry only the allowed reference fields. Queue errors expose a code and do not chain the driver exception.

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

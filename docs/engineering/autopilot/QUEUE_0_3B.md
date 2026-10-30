# Autopilot queue 0.3B

The compatibility boundary is [QUEUE_0_3A_COMPATIBILITY.md](QUEUE_0_3A_COMPATIBILITY.md). This phase adds a PostgreSQL queue beside the 0.3A file queue. It does not replace that queue, and it does not change platform migration `000105`.

## Schema

`autopilot_queue` is a separate schema. `autopilot_queue.schema_migrations` is its only version table. The runner applies `tools/bintrans_autopilot/sql/autopilot_queue/*.up.sql`. Version 1 remains the queue version. The `NOLOGIN` role is cluster-wide and is not dropped.

## Rollback

`down` takes `pg_advisory_xact_lock` and `ACCESS EXCLUSIVE` locks on the three version-1 tables, then compares the live catalogs with that version. The comparison and the later `DROP` statements stay in one transaction. There is no `COMMIT` between them. A mismatch raises `migration_blocked` before any `DROP`, and the transaction rolls back. `schema_migrations` stays at version 1. Forced row security is not left disabled: the inventory path does not touch it, and the nonempty-queue check enables it again before returning. An error after that check rolls the transaction back.

The comparison reads `pg_class`, `pg_attribute`, `pg_constraint`, `pg_trigger`, `pg_proc`, `pg_type`, `pg_policy`, and `pg_depend`. It checks tables, columns, types, defaults, nullability, constraints, indexes, triggers, policies, functions, procedures, sequences, views, materialized views, and composite types. Sequences are `pg_class.relkind = 'S'`. Ownership and internal sequence dependencies are `pg_depend.deptype` `a` and `i`, including a sequence that lives outside the schema and is `OWNED BY` a queue column. The row types and array types that PostgreSQL creates for the three tables are the only types allowed in the schema. Other automatic objects that belong to those tables, such as their primary-key indexes, are part of the allowed structure. An object is not accepted only because its name matches.

`pg_depend` records a static reference, such as a view, a foreign key, or an SQL-standard function body (`BEGIN ATOMIC`). It does not record a name inside a string body of an SQL or PL/pgSQL function, and it does not record dynamic SQL. A text search of those bodies is not used, and it would not be complete protection. Any routine outside `autopilot_queue` and the system catalogs therefore blocks rollback. The runner does not drop that routine, nor a view or sequence owned by another schema. Schema rollback is also refused unless the current database is an isolated `*_test` database with no platform migration table, no extra application schema, and no relations in `public`. On any other database `down` returns `migration_blocked` and drops nothing. Inside that isolated database, job rows still block rollback unless the caller sets `allow_destructive_test`. The down file drops only the known objects and does not use `CASCADE`. Concurrent DDL from a session that does not take this runner's locks is outside the supported model. Two runner calls cannot interleave: they share the advisory lock, and the exclusive table locks cover the check and the drop.

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

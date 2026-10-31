# Autopilot worker 0.3C1

This stage adds a dry-run worker beside `PostgresQueue`. It does not replace that queue, and it does not change the queue schema, row security, platform migration `000105`, the 0.3A file queue, or the E2E stand. Issue #319 stays open. Destructive queue rollback stays limited to an isolated test database.

## Closed switches

`WORKER_ENABLED`, `AUTOPILOT_ENABLED`, `PAID_AUTOMATION_ENABLED`, `EXTERNAL_TOOLS_ENABLED`, `AUTO_MERGE_ENABLED`, and `AUTO_DEPLOY_ENABLED` stay off. `AutopilotWorker` accepts only `mode="dry_run"`. If any of those environment variables is `true`, construction fails with `worker_disabled`. A job payload is not read and is not executed. The handler map cannot add a type outside `docs` and `ping`.

The worker does not start a thread on import. `run_pending` is finite. It does not call Cursor, a paid model, GitHub, git, SSH, or a shell.

## Execution

The worker calls the existing `requeue_expired`, `claim`, `get_status`, `renew_lease`, `complete`, and `fail` methods. Tenant identity remains the authenticated database principal. The tenant string passed to those methods must match that principal. A payload field is not authorization.

`Claim` commits before the handler runs. The handler may renew the same lease token. `Complete` and `Fail` use that token. Each run keeps its own lease result. A lost or stale token for one job does not stop another in-flight job from completing with its own token. The lost token is not reused. That row stays `RUNNING` until a later `requeue_expired` moves it to `PENDING` or `DEAD`.

This is at-least-once delivery. A crash after claim, a lost connection, or a failed renew can leave the job running until the lease expires. A second worker may then claim it. That is not exactly-once for an external effect. This worker has no external effect.

`max_in_flight` is 1 through 4. Extra calls return without claiming while that many jobs are inside the handler.

## Observability

Logs use `event`, `tenant`, `job`, and `result`. They do not include the payload, a secret, or a DSN. Events are `worker_started`, `worker_stopped`, `job_claimed`, `lease_renewed`, `job_completed`, `job_failed`, `retry_scheduled`, `job_dead`, `recovery_completed`, and `database_unavailable`.

`snapshot` uses the same runtime binding check as the queue. The connection must be the worker's tenant principal. A superuser, the table owner, another privileged role, or a different tenant is rejected before the counts are read. A rejected snapshot does not return an empty aggregate. The counts are `queue_depth`, `oldest_pending_age`, `running_jobs`, `failed_jobs`, and `dead_jobs`. The worker also keeps `lease_expirations`, `worker_heartbeat`, and `jobs_completed_total`.

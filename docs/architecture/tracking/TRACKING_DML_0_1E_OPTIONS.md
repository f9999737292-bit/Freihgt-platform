# Tracking DML 0.1E — enforcement options

Status: comparison only. No option is installed.
`RECOMMENDED_OPTION=B`.
`RECOMMENDATION_APPROVED_FOR_IMPLEMENTATION=NO`.
`ALL_WRITERS_FENCED=NO`.

PR #375 designed `transport.lock_execution_owner` as a lock and a read. It did not choose a direct-DML enforcement shape. This document is that choice for investigation. It does not edit PR #375, PR #374, or PR #357.

The mandatory fence means a runtime session cannot change `tracking.execution_tracking_state` without an execution-row lock, a tenant check, and an owner-epoch check in the same PostgreSQL transaction. A sentence in a design document is not that fence.

## 1. What every option still leaves open

`000103:6-7` keeps `freight` as the shared login. No `OWNER TO` exists for the execution table or the tracking state table. If `freight` applied `000088` and `000091`, `freight` owns both tables. PostgreSQL does not apply `GRANT` and `REVOKE` limits to the owner. A superuser is also unlimited.

A, B, and C therefore do not fence the table owner or a superuser. `SHARED_FREIGHT_RISK=OPEN` until a controller moves ownership to a `NOLOGIN` role that the runtime login cannot become, and a server query shows that owner. That query was not run. `LIVE_TABLE_OWNER=NOT_QUERIED`.

If the controller does not approve that ownership move, none of the three options is a mandatory fence. This document does not substitute another option for that missing decision.

## 2. Option A — runtime role separation

Separate logins replace the shared login. `tracking_service_login` inherits `tracking_runtime`. `shipment_runtime` stays the shipment role. Each writer keeps direct `INSERT` and `UPDATE`.

| Question | Assessment |
|---|---|
| Enforcement | A runtime statement can still omit the epoch check. Role separation removes the shared grant. It does not reject the statement. |
| PostgreSQL rights | `UPDATE` remains on the writer that must change the row. |
| Cross-tenant | The current `WHERE` at `execution_tracking_repository.go:132-133` stops a conflict update from moving tenant A to tenant B. The first insert of a missing `execution_id` still stores the candidate tenant. A does not add a lock on the execution row. |
| Existing writers | W1 through W7 keep their SQL. |
| Deadlocks | Unchanged. ETA still locks the state row first at `eta_ingest_service.go:256`. |
| Performance | No added function call. |
| Go services | Login change only. The SQL stays in the services. |
| Failure | A revoked login fails closed. A login that still has `UPDATE` fails open. |
| Change size | Role split and connection change. No write-path change. |
| Migrations | Role creation, grants, and a login cutover. Not created here. |
| Rollout | The old binary works for as long as its login keeps `UPDATE`. |
| Rollback | Restoring the shared login restores the shared grant. |

A is not the mandatory fence.

## 3. Option B — protected state write

The runtime role has no direct DML on `tracking.execution_tracking_state`, `tracking.execution_event_inbox`, or `tracking.execution_stop_eta_state`. A `SECURITY DEFINER` function, owned by a `NOLOGIN` role that is not the table owner, locks `transport.transport_executions`, checks the stored tenant and `owner_epoch`, and writes the effect and the proof in the caller transaction. The function has no `COMMIT`.

| Question | Assessment |
|---|---|
| Enforcement | For a non-owner runtime role, a direct `UPDATE` fails with `42501`. The only accepted write is `EXECUTE` on the function, and the function body contains the checks. This is the option that can reject a normal runtime write which skips the epoch check. |
| PostgreSQL rights | `REVOKE` DML from the runtime login and from `PUBLIC`. `GRANT EXECUTE` on the one function family. The function owner holds the column rights the body uses. |
| Cross-tenant | The locked `operating_tenant_id` is the authority. The payload tenant is a claim. A mismatch raises and aborts the transaction. This is not implemented. `TENANT_ISOLATION_IMPLEMENTED=NO`. |
| Existing writers | W1, W2, W3, W4, W5, and W7 move into fixed function bodies. W6 stops taking the state lock before the execution lock. W9 through W14 stay in shipment and must not assign `owner_epoch` except through a separate epoch function. |
| Deadlocks | The function locks the execution row and then the state row. ETA and approach use that order. A path that locks the state row first fails the contract. |
| Performance | One function call replaces the current separate statements. The row locks are the locks PR #375 already requires. `lock_timeout` stays a proposed `2s`, not a measured limit. |
| Go services | tracking-service calls the function with pgx. No new service and no new primary database. The function reads `transport` and writes `tracking` in one session. That is the same database boundary PR #374 recorded and the same service-boundary exception PR #375 proposed for the lock. |
| Failure | Missing `EXECUTE`, a failed check, `RowsAffected = 0`, and a lock timeout abort the transaction. The offset is not committed. The function owner, the table owner, and a superuser can still write. |
| Change size | One function family, grants, and call-site changes in the consumer, ETA, and approach. Larger than A. Smaller than moving the writers into another service. |
| Migrations | Roles, functions, column grants, DML revoke, and the `owner_epoch` column. None are created here. The next migration number is not reserved. |
| Rollout | Fail closed: the new binary does not start while it still has table `UPDATE` or while an old member is in the group. The old binary loses its write only after it is not the table owner and its DML grant is gone. |
| Rollback | A down migration that restores DML reopens the bypass. It is a controller action after the new binary has stopped. It is not automatic. |

The function owner can bypass the body. It holds the DML the body needs, so a session that can become that role can issue the `UPDATE` directly. The design makes that role `NOLOGIN`, gives it no membership, and does not grant `SET` to it. That is a control on sessions, not a tested prohibition. `FUNCTION_OWNER_BYPASS=OPEN_UNTIL_TESTED`.

B is the recommended investigation option because it is the shape that rejects a non-owner runtime write without the check. It is not asserted as a complete fence. It becomes mandatory only after the controller approves the ownership move in section 1, the public-execute audit, and the writer cutover. Until those gates are approved, `DIRECT_DML_ENFORCEMENT_APPROVED=NO`.

## 4. Option C — one writer service

Every protected state change runs in tracking-service, and other services lose `UPDATE` on the state table.

| Question | Assessment |
|---|---|
| Enforcement | C does not, by itself, reject a statement. W1 through W4 are already in tracking-service. Moving no code still leaves those statements able to omit the epoch check. C needs the revoke and the protected body from B, or an equivalent server-side check. |
| PostgreSQL rights | Other roles lose DML. The remaining writer role still has DML unless B is added. |
| Cross-tenant | Unchanged until the check moves into the one transaction. |
| Existing writers | ETA and approach are already in tracking-service. Shipment does not write the tracking state table. C does not remove W1 through W5. |
| Deadlocks | Unchanged until the lock order moves into the one writer. |
| Performance | No change if the SQL stays. |
| Go services | A real single-writer move of shipment epoch changes into tracking, or of tracking state into shipment, crosses the service boundary. `SHIPMENT_SINGLE_WRITER` was compared in PR #375 and not recommended. |
| Failure | The old consumer, running as the same service login, is still a writer. |
| Change size | Service-boundary change plus the privilege work of B. |
| Migrations | Grants plus a new owner of the write path. |
| Rollout | Stopping the second service does not stop a second binary of the same service. |
| Rollback | Restoring the second writer restores a direct path. |

C is not sufficient. It is not the recommended option.

## 5. Decision

`RECOMMENDED_OPTION=B`.
`OPTION_A_SUFFICIENT=NO`.
`OPTION_C_SUFFICIENT=NO`.
`B_IS_MANDATORY_WHILE_FREIGHT_OWNS_THE_TABLE=NO`.
`CONTROLLER_DECISION_REQUIRED=TABLE_OWNERSHIP_MOVE`.

The lock order stays the order from PR #375: execution row, then tracking state. The write order stays `EXECUTION_LOCK → OWNER_EPOCH_CHECK → INBOX_INSERT → STATE_EFFECT_AND_PROOF → COMMIT`. Option B puts that sequence inside the function. It does not change the sequence.

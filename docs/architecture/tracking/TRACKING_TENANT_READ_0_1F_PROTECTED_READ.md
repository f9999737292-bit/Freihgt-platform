# TRACKING-TENANT-READ-AUTHORITY-0.1F — Protected Read

This contract stays inside the PR #376 decision that `tracking_runtime` does not receive table `SELECT`, `INSERT`, `UPDATE`, or `DELETE` on the tracking execution tables. PR #376 is not modified. Its earlier read sketch, which treated a claimed tenant argument as the scope, is superseded for these three reads by the gates below.

No function is created. No role is created. No grant is applied.

## 1. Roles, as already designed and still not applied

| Role | Read posture |
| --- | --- |
| `tracking_service_login` | Login. Inherits `tracking_runtime` with `INHERIT TRUE` and `SET FALSE`. No direct table grants. |
| `tracking_runtime` | `EXECUTE` on the protected functions only. No table `SELECT`. |
| `tracking_read_fence` | Owns the read functions. Holds the column `SELECT` those functions use. No DML. |
| Table owner and superuser | Not limited by these grants. `SHARED_FREIGHT_RISK` stays `OPEN` as recorded by PR #376. |

`000105` does not create these roles. `FUNCTION_OWNER_BYPASS` stays `OPEN`: a session that can act as the function owner bypasses the runtime boundary.

A function argument is never the authorization. The function may use an argument as an object id. It loads the scope from tables it is allowed to read.

## 2. The three reads

### 2.1 ListActiveTargets

| Item | Contract |
| --- | --- |
| Trusted source | Each active, in-window `tracking.shipment_tracking_binding` row, joined to the execution owner. The binding writer is a separate open gate (`BINDING_CREATION_TRUST=OPEN`). |
| Parameters | Provider code and provider device id. No tenant parameter. |
| Scope passing | The function loads the binding rows. The caller does not send a tenant list. |
| Server check | Per binding: status, `active_to`, tenant equality with `transport.transport_executions.operating_tenant_id`, and driver or vehicle equality. State rows outside that pair are omitted. |
| Columns | Execution id, stop id, ordinal, and the tenant of that scope. Coordinates, driver id, and vehicle id are returned only inside a scope that passed. They are not returned in an error. |
| Fail closed | Database error, missing execution row, or no binding: empty result, no fallback to `ListActiveTargetsForActors`. |
| Audit | Not implemented. A later audit row may store binding id, execution id, tenant id, and the decision. It must not store coordinates. |
| Stale authorization | Re-read the binding and the execution row in the same statement. A prior result is not reused after an owner change. The write fence still rechecks `owner_epoch`. |

PostgreSQL can enforce this join. PostgreSQL cannot prove the binding insert came from the gateway.

### 2.2 GetState

| Item | Contract |
| --- | --- |
| Trusted source | None on the current call path. |
| Parameters | None exposed. |
| Scope passing | Not accepted from the caller. |
| Server check | Blocking gate `GET_STATE_AUTHORIZATION=BLOCKED_NO_TRUSTED_CALLER`. |
| Columns | None until the gate opens. The current select list at `execution_tracking_repository.go:171-174` is what a table `SELECT` would leak. |
| Fail closed | No row. Same client error for unknown and denied. |
| Audit | Not implemented. Denial audit must not include coordinates or actor ids. |
| Stale authorization | There is no grant to go stale. A future grant is rechecked at read time and is not a write fence. |

### 2.3 GetExecutionStopETA

| Item | Contract |
| --- | --- |
| Trusted source | None. The shared token is service authentication only. |
| Parameters | None that accept `X-Tenant-ID`. |
| Scope passing | Not accepted from the header. |
| Server check | Blocking gate `GET_EXECUTION_STOP_ETA_AUTHORIZATION=BLOCKED_TOKEN_IS_NOT_TENANT`. |
| Columns | None until a named service scope exists. |
| Fail closed | No row when the token is missing, the scope is missing, or the authorization source is down. |
| Audit | Not implemented. |
| Stale authorization | A stop observed before the execution lock is rechecked by the write. The read is not cached as permission. |

## 3. Split of responsibility

| Enforcement | Owner |
| --- | --- |
| Binding status and active window | PostgreSQL, inside the read function |
| Execution owner and actor match | PostgreSQL, column `SELECT` held by `tracking_read_fence` only |
| Empty result on mismatch | PostgreSQL |
| Gateway JWT copied into the binding write | Gateway and the future binding writer. Not true of the current internal insert. |
| Named internal service rather than the shared secret | No current mechanism. Gate stays closed. |
| Membership and carrier relationship | Tables exist (`000002:79`, `000100:6`, `000103:128`). These reads do not use them. A later task may join membership for `GetState` only after the caller is the gateway user. This stage does not add that join to the actor read, because the actor path has no user id (`execution_tracking_repository.go:391-396`). |
| Table owner reading past the function | Not enforceable with `GRANT` and `REVOKE`. Remains open. |

## 4. Owner epoch

`OWNER_EPOCH_INTEGRATION=READ_IS_NOT_THE_FENCE`

PR #375 requires `EXECUTION_LOCK → OWNER_EPOCH_CHECK → INBOX_INSERT → STATE_EFFECT_AND_PROOF → COMMIT` for a new committed state effect. A protected read does not take that lock and does not advance the epoch. An epoch-4 read result does not allow an epoch-4 write after epoch 5 is stored. The write function performs that comparison. `OWNER_EPOCH_MONOTONICITY` remains `SPECIFIED_NOT_ENFORCED`.

## 5. Audit and privacy

No audit table is added. Diagnostic text for a denied read names the failure class only: unauthorized, not found, or unavailable. It does not include coordinates, a driver id, a vehicle id, a device id, or a tenant id.

## 6. Documents that a later accepted stage would update

These files stay untouched in this stage:

- PR #376 protected-read section that still describes a claimed-tenant argument, and its tenant-read tests.
- PR #375 only if a later implementation changes the lock-function caller. This read design does not.
- PR #374 and PR #357 stay the safety and rebalance contracts. This stage does not prove broker safety.

## 7. Unresolved

- The column list on `transport.transport_executions` granted to `tracking_read_fence` must stay limited to `id`, `operating_tenant_id`, `driver_id`, and `vehicle_id`. The exact grant text belongs to an implementation migration, which this stage does not write.
- Public `EXECUTE` on existing functions remains `REQUIRED` and `NOT_RUN`, as PR #375 and PR #376 already recorded.

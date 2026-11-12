# Tracking owner fence 0.1D — database security

Status: recommendation only. No grant and no function are created.
`DB_PRIVILEGE_MODEL=SECURITY_DEFINER_LOCK_FUNCTION`.
`CURRENT_USER_CHECK_REMOVED=YES`.
`TENANT_ISOLATION_IMPLEMENTED=NO`.
`ALL_WRITERS_FENCED=NO`.
The recommendation is not an implementation approval. A written grant is not a technical restriction until a later migration installs it.

Tracking-service must not receive broad rights on schema `transport`. The repository already uses `SECURITY DEFINER` with `search_path = pg_catalog` for guarded functions. The company-service static check lists both at `services/company-service/internal/repository/activation_security_migration_static_test.go:18`.

## 1. Caller identity

Inside a `SECURITY DEFINER` function, `current_user` is the function owner. It is not the caller. This design does not read `current_user` to decide who called `transport.lock_execution_owner`.

The caller gate is the privilege system plus `session_user`, which stays the login that opened the session:

1. `REVOKE ALL ON FUNCTION transport.lock_execution_owner(uuid) FROM PUBLIC`. PostgreSQL grants `EXECUTE` to `PUBLIC` on a new function unless this revoke is present.
2. `GRANT EXECUTE ON FUNCTION transport.lock_execution_owner(uuid) TO tracking_runtime` and to no other role. `freight` does not receive `EXECUTE`. `shipment_runtime` does not receive `EXECUTE`.
3. The function body rejects the call unless both are true: `pg_has_role(session_user, 'tracking_runtime', 'MEMBER')` and `pg_has_role(session_user, 'tracking_runtime', 'USAGE')`. `MEMBER` is direct or indirect membership. `USAGE` is true when that membership is inherited or the role was activated with `SET ROLE`. A member created `NOINHERIT` that has not run `SET ROLE` fails `USAGE`.
4. A login that is not granted `tracking_runtime` has no `EXECUTE` privilege. The call fails before the body. The membership check is a second gate if `EXECUTE` is later granted more widely.

`tracking_runtime` is `NOLOGIN`, `NOSUPERUSER`, and `NOBYPASSRLS`. The future login that should call the function is a dedicated tracking login that inherits `tracking_runtime`. That login is not `freight`.

These grants do not constrain a superuser. They do not constrain the owner of the function or the owner of `transport.transport_executions`. `pg_has_role` does not remove a superuser's privileges.

## 2. Shared login `freight`

`000103_tms_relationship_pins_security_foundation_v0_1.up.sql:6-7` states that `freight` stays the shared login until a later role split. The production repository methods use that `DATABASE_URL` login. No production Go path in `services/` executes `SET ROLE`.

`freight` is therefore one login for shipment-service and tracking-service. Granting `EXECUTE` to `freight` would let every service on that login call the function. This design does not do that. The current services cannot call the function until a dedicated tracking login exists. That is a blocker for implementation, not a reason to grant `EXECUTE` to `freight`.

If the role that applied `000088_tms_transport_execution_foundation_v0_1a.up.sql` was `freight`, then `freight` owns `transport.transport_executions`. This repository never runs `ALTER TABLE transport.transport_executions OWNER TO ...`. The live owner is the migration role and is not re-read from a server in this stage. A table owner is not limited by `GRANT` or `REVOKE`. Direct `UPDATE` by that owner remains possible. `ALL_WRITERS_FENCED=NO`.

## 3. Tenant authorization

The function accepts `p_execution_id uuid` and no tenant. It returns `operating_tenant_id` and `owner_epoch` from the locked row.

After that return, the same tracking transaction compares the event's tenant claim with the returned `operating_tenant_id`. The returned value is the authoritative owner. The payload tenant, `X-Tenant-ID`, and any tenant argument are claims. A claim that differs from the locked owner ends the transaction with `OWNER_STATE_MISMATCH`: rollback, no state update, no remaining inbox row, and no Kafka offset commit. The state row keeps the tenant it already had.

This comparison is not in the current consumer. Tenant isolation is not implemented.

## 4. Function owner

Role name: `tracking_execution_fence`.

Attributes: `NOLOGIN`, `NOSUPERUSER`, `NOBYPASSRLS`, `NOINHERIT`, `NOCREATEDB`, `NOCREATEROLE`. The role is not the owner of `transport.transport_executions` and is not `shipment_runtime`. `000103:1225` grants `shipment_runtime` table-level `INSERT` and `UPDATE` on that table. The fence role does not receive that grant.

Rights of `tracking_execution_fence`, and no others on this table:

- `USAGE` on schema `transport`.
- `SELECT (id, operating_tenant_id, owner_epoch)`.
- `UPDATE (updated_at)` only.

No `INSERT`. No `DELETE`. No `UPDATE` on `operating_tenant_id`. No `UPDATE` on `owner_epoch`. No `UPDATE` on `event_seq`, `current_revision_id`, `driver_id`, `vehicle_id`, or `carrier_company_id`.

`updated_at` is the single `UPDATE` column because PostgreSQL requires `UPDATE` on at least one column for `SELECT ... FOR UPDATE`, in addition to `SELECT` on the columns the statement reads. `updated_at` does not name the owner or the epoch. The function body does not issue `UPDATE`. The grant exists so the lock permission check can succeed. A later test must show that an `UPDATE` of `operating_tenant_id` or `owner_epoch` by this role fails with `42501`. This stage does not run that test. If a server rejects `FOR UPDATE` unless more columns are granted, the grant is not widened to `operating_tenant_id` or `owner_epoch`. That result stays unresolved until the controller sees the server error.

The function is `SECURITY DEFINER`, owned by `tracking_execution_fence`, with `search_path = pg_catalog`. Every name in the body is schema-qualified. The body is one fixed statement:

```sql
SELECT operating_tenant_id, owner_epoch
FROM transport.transport_executions
WHERE id = p_execution_id
FOR UPDATE
```

There is no dynamic SQL and no `COMMIT`. The lock lasts until the caller commits or rolls back.

No migration in this repository enables row level security, and none creates a policy on `transport.transport_executions`. With row level security off, `NOBYPASSRLS` does not hide the row. If a later migration enables row level security without a policy for `tracking_execution_fence`, the function returns no row and the caller treats that as a missing execution. No such policy is part of this design.

Schema `USAGE` is required and is not granted today because the role does not exist. The live owner of `transport.transport_executions` was not queried. Those two facts are `OPEN_RISKS`, not a claim that the privilege set was executed.

## 5. Privilege tests required later

These tests are specified and not run.

1. Session A calls the function for an existing execution. Session B's `UPDATE` of that row waits until A commits or rolls back. `PASS`: B blocks and then proceeds. `FAIL`: B changes the row while A still holds the transaction.
2. As `tracking_execution_fence`, `UPDATE transport.transport_executions SET operating_tenant_id = ...` fails with `42501`. The stored tenant is unchanged.
3. As `tracking_execution_fence`, `UPDATE transport.transport_executions SET owner_epoch = owner_epoch + 1` fails with `42501`. The stored epoch is unchanged.
4. A login that is not a member of `tracking_runtime` calls the function and receives an authorization failure. No row lock remains after the failed call.
5. The locked owner is tenant A and the event claims tenant B. The tracking transaction rolls back. State tenant, inbox count, and the Kafka offset stay as they were before the call.

## 6. Direct state writes

Calling `transport.lock_execution_owner` does not forbid a later `UPDATE` of `tracking.execution_tracking_state` by a role that already has `UPDATE` on that table. The current shared login is such a path when it owns the table or holds that grant. A sentence in a design document does not close the path. `ALL_WRITERS_FENCED=NO`.

Three enforcement shapes are recorded. This stage does not choose one. A controller decision is required before any of them is implemented.

### A. Separate runtime roles

`UPDATE` on `tracking.execution_tracking_state` is revoked from `freight` and granted only to `tracking_runtime`. Shipment keeps its own role. The table owner and any superuser can still update the table. An old consumer that connects as `tracking_runtime` can still `UPDATE` the state row without calling the lock function. Role separation removes the shared login's direct grant. It does not, by itself, reject a statement that omits the epoch check.

Code and migration later: revoke the shared grant, create `tracking_runtime`, change the tracking `DATABASE_URL` login, and keep the old binary from using the new login. No such migration is created here.

### B. Protected write operation

A second `SECURITY DEFINER` function locks the execution row, compares tenant and epoch, and performs the state write. `UPDATE` on `tracking.execution_tracking_state` is revoked from runtime logins. The runtime login receives `EXECUTE` on that function only. A direct `UPDATE` by that login fails. The table owner, a superuser, and any role that still has `UPDATE` can bypass it. The old consumer bypasses it while its login retains `UPDATE`.

This is the shape that rejects a normal runtime write which skips the epoch check. It needs a new function, a revoke, and a change to every state writer, including ETA ingest and `MarkApproachEmitted`. It is not selected here.

### C. One controlled transaction

Every write of `tracking.execution_tracking_state` moves into one transaction that also locks `transport.transport_executions`. Other services lose `UPDATE` on the state table. The old consumer has no remaining write path if its grant is gone. A deployment that still runs the old binary with the old owner login can write until that login loses the privilege. ETA ingest and `MarkApproachEmitted` move or stop. This crosses the service boundary described in the single-writer comparison below.

### What each shape leaves open

| Shape | Who can `UPDATE` state | Bypass that remains | Object owner | Old consumer |
|---|---|---|---|---|
| A | `tracking_runtime`, table owner, superuser | Direct `UPDATE` without an epoch check | Unchanged until a later `OWNER TO` | Bypasses if its login still has `UPDATE` |
| B | The protected function's owner role, table owner, superuser | Owner and superuser | Function owner is `NOLOGIN` and is not the table owner | Bypasses until its `UPDATE` is revoked |
| C | The one writer role, table owner, superuser | Owner and superuser | Writer role owns the transaction, not necessarily the table | No write after its grant is revoked |

## 7. Other variants already compared

`MINIMAL_GRANT_ADAPTER` would give the tracking login `SELECT` and `UPDATE` on execution columns so it can run `FOR UPDATE` itself. That login could then assign those columns. It is not recommended.

`SHIPMENT_SINGLE_WRITER` would move the state write into the shipment transaction. ETA ingest (`eta_ingest_service.go:247`) and `MarkApproachEmitted` (`approach.go:94`) would have to move or stop. `shipment_runtime` has no grant on schema `tracking` through migration `000105`. It is not recommended.

The recommended lock function remains `SECURITY_DEFINER_LOCK_FUNCTION`. It is a lock and a read. Section 6 is the separate, unchosen question of how a direct state write is rejected.

## 8. Old consumer

The current binary reaches the state writes without the function. A mixed group lets that binary commit a state row after epoch 5. `MIXED_CONSUMER_GATE` stays `CLOSED`. Starting the old binary again is a separate controller action. The function does not repair an offset that binary has already committed, and it does not remove that binary's `UPDATE` privilege.

## 9. Open risks

- `SHARED_LOGIN_FREIGHT`
- `TABLE_OWNER_AND_SUPERUSER_BYPASS`
- `DIRECT_STATE_UPDATE_WITHOUT_EPOCH_CHECK`
- `COLUMN_UPDATE_SUFFICIENCY_NOT_EXECUTED`
- `LIVE_TABLE_OWNER_NOT_QUERIED`
- `RLS_ABSENT_AND_NO_POLICY_DESIGNED`
- `SHIPMENT_RUNTIME_HAS_TABLE_UPDATE` at `000103:1225`
- `OPERATOR_SQL_NOT_VERIFIED`
- `TENANT_ISOLATION_NOT_IMPLEMENTED`
- `EPOCH_MONOTONIC_TRIGGER_NOT_CREATED`
- `OLD_CONSUMER_CAN_BE_STARTED_AGAIN`

## 10. Acceptance

- `TRACKING_TRANSPORT_SCHEMA_GRANT=NONE` for the tracking login.
- `LOCK_FUNCTION_HAS_COMMIT=NO`.
- `CALLER_CHECK=session_user MEMBER AND USAGE`.
- `CURRENT_USER_IS_CALLER_CHECK=NO`.
- `EXECUTE_GRANTED_TO=tracking_runtime`.
- `EXECUTE_GRANTED_TO_FREIGHT=NO`.
- `FUNCTION_OWNER_UPDATE_COLUMN=updated_at`.
- `CALLER_TENANT_IS_AUTHORITY=NO`.
- `RECOMMENDATION=SECURITY_DEFINER_LOCK_FUNCTION`.
- `DIRECT_DML_ENFORCEMENT_CHOSEN=NO`.
- `RECOMMENDATION_APPROVED_FOR_IMPLEMENTATION=NO`.

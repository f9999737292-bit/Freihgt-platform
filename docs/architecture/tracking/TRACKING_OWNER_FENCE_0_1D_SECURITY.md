# Tracking owner fence 0.1D — database security

Status: recommendation only. No grant and no function are created.
`DB_PRIVILEGE_MODEL=SECURITY_DEFINER_LOCK_FUNCTION`.
`CURRENT_USER_CHECK_REMOVED=YES`.
`TENANT_ISOLATION_IMPLEMENTED=NO`.
`ALL_WRITERS_FENCED=NO`.
The recommendation is not an implementation approval. A written grant is not a technical restriction until a later migration installs it.

Tracking-service must not receive broad rights on schema `transport`. The one specified exception is schema `USAGE` plus `EXECUTE` on `transport.lock_execution_owner(uuid)`, held by `tracking_runtime` and inherited by one dedicated login. The repository already uses `SECURITY DEFINER` with `search_path = pg_catalog` for guarded functions. The company-service static check lists both at `services/company-service/internal/repository/activation_security_migration_static_test.go:18`.

## 1. Caller identity

This contract is bound to PostgreSQL 16, the major version of the repository compose image `postgres:16`. The same `pg_has_role` sentence is present in the PostgreSQL 17 documentation. Versions before 16 are not this contract. The wording below was read from these pages. It was not executed on a server.

- [PostgreSQL 16: System Information Functions](https://www.postgresql.org/docs/16/functions-info.html)
- [PostgreSQL 17: System Information Functions](https://www.postgresql.org/docs/17/functions-info.html)
- [PostgreSQL 16: GRANT](https://www.postgresql.org/docs/16/sql-grant.html)

`session_user` is normally the user who initiated the database connection. `SET ROLE` does not change it. A superuser can change it with `SET SESSION AUTHORIZATION`. `current_user` is the user used for permission checking. `SET ROLE` changes `current_user`. Inside a `SECURITY DEFINER` function, `current_user` becomes the function owner. This design does not read `current_user` to decide who called `transport.lock_execution_owner`.

`pg_has_role` accepts `MEMBER`, `USAGE`, and `SET`. PostgreSQL documents them as three different questions. `MEMBER` denotes direct or indirect membership in the role without regard to what specific privileges may be conferred. `USAGE` denotes whether the privileges of the role are immediately available without doing `SET ROLE`. `SET` denotes whether it is possible to change to the role using `SET ROLE`.

`MEMBER` is therefore not a test of inherited privileges. `USAGE` does not become true because the session has executed `SET ROLE`. A `NOINHERIT` membership stays non-inheriting after `SET ROLE`. That command changes `current_user` for later commands. It does not change the login into an inheriting member, and it does not change `pg_has_role(session_user, 'tracking_runtime', 'USAGE')`.

The accepted caller is one arrangement:

1. A dedicated login, named `tracking_service_login` in this design and not created here. Attributes: `LOGIN`, `NOSUPERUSER`, `NOBYPASSRLS`, `INHERIT`, `NOCREATEDB`, `NOCREATEROLE`. It is not `freight`.
2. `GRANT tracking_runtime TO tracking_service_login WITH INHERIT TRUE, SET FALSE`. PostgreSQL 16 applies `INHERIT` and `SET` to the membership. `INHERIT TRUE` makes the privileges of `tracking_runtime` available with no `SET ROLE`. `SET FALSE` means this membership does not authorize `SET ROLE` to `tracking_runtime`. The application does not issue `SET ROLE`. No production Go path in `services/` does so today. That observation is not a test of `SET ROLE`.
3. The function body accepts the call only when `pg_has_role(session_user, 'tracking_runtime', 'USAGE')` is true. That is the same condition as privileges being available without `SET ROLE`. `MEMBER` is not the acceptance predicate. A member with `INHERIT FALSE` has `MEMBER` true and `USAGE` false, and the body rejects that login.
4. `freight` is not a member of `tracking_runtime`. `tracking_runtime` is not a member of `freight`. No other login is a member, directly or through a chain. A chain was not queried from a server.

`GRANT EXECUTE` is checked against the privileges available to `current_user` at the call, including inherited privileges and privileges of a role selected by `SET ROLE`, plus privileges granted to `PUBLIC`. The body then checks `session_user` and `USAGE`. For `tracking_service_login` those two checks agree: `EXECUTE` is inherited from `tracking_runtime`, and `USAGE` is true without `SET ROLE`.

A login that is a member `WITH INHERIT FALSE, SET TRUE` can `SET ROLE tracking_runtime`. `GRANT EXECUTE` can then succeed because `current_user` is `tracking_runtime` at the call. `session_user` is still the original login, and `USAGE` for that login stays false, so the body is specified to reject the call. This disagreement is intentional in the text. It has not been executed. `SET_ROLE_BYPASS=NOT_CLAIMED_SAFE_WITHOUT_TEST`. A superuser, `SET SESSION AUTHORIZATION`, and any later membership are outside that untested sentence.

The privilege commands around the function are:

1. `REVOKE ALL ON FUNCTION transport.lock_execution_owner(uuid) FROM PUBLIC`. PostgreSQL grants `EXECUTE` to `PUBLIC` on a new function unless this revoke is present.
2. `GRANT EXECUTE ON FUNCTION transport.lock_execution_owner(uuid) TO tracking_runtime` and to no other role. `freight` does not receive `EXECUTE`. `shipment_runtime` does not receive `EXECUTE`. `tracking_service_login` does not receive a direct `EXECUTE` grant. It inherits the one grant above.
3. A login for which `USAGE` is false has no accepted call. If it also lacks `EXECUTE`, the call fails before the body.

These grants do not constrain a superuser. They do not constrain the owner of the function or the owner of `transport.transport_executions`. `pg_has_role` does not remove a superuser's privileges.

## 2. Shared login `freight`

`000103_tms_relationship_pins_security_foundation_v0_1.up.sql:6-7` states that `freight` stays the shared login until a later role split. The production repository methods use that `DATABASE_URL` login. No production Go path in `services/` executes `SET ROLE`.

`freight` is therefore one login for shipment-service and tracking-service. Granting `EXECUTE` to `freight` would let every service on that login call the function. This design does not do that. The current services cannot call the function until a dedicated tracking login exists. That is a blocker for implementation, not a reason to grant `EXECUTE` to `freight`.

If the role that applied `000088_tms_transport_execution_foundation_v0_1a.up.sql` was `freight`, then `freight` owns `transport.transport_executions`. This repository never runs `ALTER TABLE transport.transport_executions OWNER TO ...`. The live owner is the migration role and is not re-read from a server in this stage. A table owner is not limited by `GRANT` or `REVOKE`. Direct `UPDATE` by that owner remains possible. `ALL_WRITERS_FENCED=NO`.

## 3. Schema USAGE and table privileges

PostgreSQL `USAGE` on a schema allows a role to look up objects in that schema when the object's own privileges are also held. It does not grant `SELECT`, `INSERT`, `UPDATE`, or `DELETE` on tables in the schema, and it does not grant `CREATE` on the schema. It also does not grant `EXECUTE`. `EXECUTE` on a function is a separate privilege. A new function receives `EXECUTE` for `PUBLIC` unless that grant is revoked. A role that can look up the schema and that holds `PUBLIC` therefore reaches every function whose `PUBLIC` `EXECUTE` grant is still present.

`tracking_service_login` is a member of `tracking_runtime` only. It is not a member of `tracking_execution_fence`. It receives no direct privilege on schema `transport`, on any transport table, or on any function. PostgreSQL 16 documents that a role whose privileges are inherited, and which should not be entered with `SET ROLE`, should not own SQL objects. `tracking_runtime` owns none. The function owner is `tracking_execution_fence`.

`tracking_runtime` is `NOLOGIN`, `NOSUPERUSER`, `NOBYPASSRLS`, `NOINHERIT`, `NOCREATEDB`, and `NOCREATEROLE`. It is not a member of `shipment_runtime`, `freight`, `freight_migrations`, or `tracking_execution_fence`. Its specified privileges, and no others, are:

- `USAGE ON SCHEMA transport`.
- `EXECUTE ON FUNCTION transport.lock_execution_owner(uuid)`.

It has no `SELECT`, `INSERT`, `UPDATE`, or `DELETE` on `transport.transport_executions` or on any other transport table. It has no `CREATE ON SCHEMA transport`. It has no `GRANT ALL`. It has no `EXECUTE` on any other function. `ALL FUNCTIONS IN SCHEMA transport` is not granted.

`tracking_execution_fence` is the function owner specified in section 5. It is the role that needs the column privileges, because the function body runs as that role. The caller login does not receive those column privileges.

Schema `USAGE` does not by itself make another function callable. A remaining `PUBLIC` `EXECUTE` grant does, once the schema can be looked up. The migration files through `000105` were read. They were not compared with `pg_proc.proacl` on a server. `PUBLIC_FUNCTION_EXECUTE_AUDIT=REQUIRED`.

`000103_tms_relationship_pins_security_foundation_v0_1.up.sql:1186-1198` includes `REVOKE ALL ... FROM PUBLIC` for the transport functions created in that migration. Lines 1207-1213 and 1215 then grant `EXECUTE` on transport functions to `shipment_runtime` or `freight_migrations`. `tracking_runtime` is not one of those grantees. Schema `USAGE` does not give `tracking_runtime` those revoked functions.

These earlier transport functions are `RETURNS trigger` and have no `REVOKE ... FROM PUBLIC` in the up migrations that create them:

- `transport.deny_transport_order_rate_snapshot_mutation()` in `000051`
- `transport.deny_shipment_cargo_execution_evidence_mutation()` in `000082`
- `transport.reject_transport_execution_stop_reparent()` and `transport.reject_transport_execution_revision_mismatch()` in `000088`
- `transport.reject_completed_transport_execution_stop_mutation()` and `transport.reject_completed_transport_execution_action_mutation()` in `000089`
- `transport.deny_delivery_disposition_append_only()` and `transport.reject_delivery_disposition_fact_rewrite()` in `000093`
- `transport.reject_pod_requirement_decision_mutation()` and `transport.validate_pod_requirement_decision_insert()` in `000098`

PostgreSQL rejects a direct call of a trigger function. That rejection is not a revoke of `EXECUTE`. A later non-trigger function left with `PUBLIC` `EXECUTE` would become callable by any role that has schema `USAGE`, including `tracking_runtime`. The migration that creates `transport.lock_execution_owner` must revoke `PUBLIC` before it grants `EXECUTE` to `tracking_runtime`. Until a server audit lists `proacl` for every function in schema `transport`, this file read is not that audit.

## 4. Tenant authorization

The function accepts `p_execution_id uuid` and no tenant. It returns `operating_tenant_id` and `owner_epoch` from the locked row.

After that return, the same tracking transaction compares the event's tenant claim with the returned `operating_tenant_id`. The returned value is the authoritative owner. The payload tenant, `X-Tenant-ID`, and any tenant argument are claims. A claim that differs from the locked owner ends the transaction with `OWNER_STATE_MISMATCH`: rollback, no state update, no remaining inbox row, and no Kafka offset commit. The state row keeps the tenant it already had.

This comparison is not in the current consumer. Tenant isolation is not implemented.

## 5. Function owner

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

`USAGE` on schema `transport` is specified for `tracking_execution_fence` and is not granted, because the role does not exist. The column `UPDATE` on `updated_at` is specified and was not executed. The live owner of `transport.transport_executions` was not queried. Those facts stay in `OPEN_RISKS`.

## 6. Privilege tests required later

These tests are specified and not run.

1. Session A calls the function for an existing execution. Session B's `UPDATE` of that row waits until A commits or rolls back. `PASS`: B blocks and then proceeds. `FAIL`: B changes the row while A still holds the transaction.
2. As `tracking_execution_fence`, `UPDATE transport.transport_executions SET operating_tenant_id = ...` fails with `42501`. The stored tenant is unchanged.
3. As `tracking_execution_fence`, `UPDATE transport.transport_executions SET owner_epoch = owner_epoch + 1` fails with `42501`. The stored epoch is unchanged.
4. A login that is not a member of `tracking_runtime` calls the function and receives an authorization failure. No row lock remains after the failed call.
5. The locked owner is tenant A and the event claims tenant B. The tracking transaction rolls back before inserting an inbox row. State tenant, inbox count, and the Kafka offset stay as they were before the call.

Tests T1 through T8 in `TRACKING_OWNER_FENCE_0_1D_TEST_PLAN.md` are part of this privilege and order matrix. They are specified and not run.

## 7. Direct state writes

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

## 8. Other variants already compared

`MINIMAL_GRANT_ADAPTER` would give the tracking login `SELECT` and `UPDATE` on execution columns so it can run `FOR UPDATE` itself. That login could then assign those columns. It is not recommended.

`SHIPMENT_SINGLE_WRITER` would move the state write into the shipment transaction. ETA ingest (`eta_ingest_service.go:247`) and `MarkApproachEmitted` (`approach.go:94`) would have to move or stop. `shipment_runtime` has no grant on schema `tracking` through migration `000105`. It is not recommended.

The recommended lock function remains `SECURITY_DEFINER_LOCK_FUNCTION`. It is a lock and a read. Section 7 is the separate, unchosen question of how a direct state write is rejected.

## 9. Old consumer

The current binary reaches the state writes without the function. A mixed group lets that binary commit a state row after epoch 5. `MIXED_CONSUMER_GATE` stays `CLOSED`. Starting the old binary again is a separate controller action. The function does not repair an offset that binary has already committed, and it does not remove that binary's `UPDATE` privilege.

## 10. Open risks

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
- `PUBLIC_FUNCTION_EXECUTE_NOT_AUDITED_ON_A_SERVER`
- `SET_ROLE_BEHAVIOR_NOT_TESTED`
- `MEMBERSHIP_CHAIN_NOT_QUERIED`
- `SCHEMA_USAGE_NOT_INSTALLED`

## 11. Statements that remain open

`SECURITY DEFINER` does not make tenant isolation true. Direct state DML remains open. The shared `freight` login remains a risk. `tracking_execution_fence` is not the owner of `transport.transport_executions`. `UPDATE (updated_at)` is specified and was not executed. Owner-epoch monotonicity is specified and not enforced. ETA ingest and `MarkApproachEmitted` are not on this fence in the current code. A mixed consumer group stays closed. The old consumer can still write until its privilege and deployment are removed. Broker safety is not proven. The lock order is unchanged: the execution row is locked before the tracking state row.

## 12. Acceptance

- `J_TOF_R2_001=ADDRESSED`.
- `TRACKING_TRANSPORT_SCHEMA_USAGE=SPECIFIED` on `tracking_runtime`, inherited by `tracking_service_login`.
- `TRACKING_TRANSPORT_TABLE_DML=NONE_BY_DESIGN` for `tracking_runtime` and `tracking_service_login`.
- `PUBLIC_FUNCTION_EXECUTE_AUDIT=REQUIRED`.
- `J_TOF_R2_002=ADDRESSED`.
- `SESSION_USER_SEMANTICS=CORRECT`.
- `MEMBER_USAGE_DIFFERENCE=DOCUMENTED`.
- `SET_ROLE_BYPASS=NOT_CLAIMED_SAFE_WITHOUT_TEST`.
- `LOCK_FUNCTION_HAS_COMMIT=NO`.
- `CALLER_CHECK=session_user USAGE`.
- `CURRENT_USER_IS_CALLER_CHECK=NO`.
- `EXECUTE_GRANTED_TO=tracking_runtime`.
- `EXECUTE_GRANTED_TO_FREIGHT=NO`.
- `FUNCTION_OWNER_UPDATE_COLUMN=updated_at`.
- `CALLER_TENANT_IS_AUTHORITY=NO`.
- `RECOMMENDATION=SECURITY_DEFINER_LOCK_FUNCTION`.
- `DIRECT_DML_ENFORCEMENT_CHOSEN=NO`.
- `RECOMMENDATION_APPROVED_FOR_IMPLEMENTATION=NO`.

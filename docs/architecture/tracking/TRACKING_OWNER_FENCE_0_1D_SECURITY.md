# Tracking owner fence 0.1D — database security

Status: recommendation only. No grant and no function are created.
`DB_PRIVILEGE_MODEL=SECURITY_DEFINER_LOCK_FUNCTION`.
The recommendation is not an implementation approval.

Tracking-service must not receive broad rights on schema `transport`. `SELECT` or `UPDATE` on all of `transport.transport_executions` would let the tracking login read or change shipment columns that this fence does not need.

The repository already uses `SECURITY DEFINER` with `search_path = pg_catalog` for guarded functions. The company-service static check lists both at `services/company-service/internal/repository/activation_security_migration_static_test.go:18`.

## 1. Recommended variant

Name: `SECURITY_DEFINER_LOCK_FUNCTION`.

A later migration, numbered only after a fresh head check above `000105`, creates one function:

`transport.lock_execution_owner(p_execution_id uuid)`

The function returns `operating_tenant_id` and `owner_epoch` for that primary key. It executes as a new `NOLOGIN` owner role that can `SELECT` the two columns and lock that one table. The function is `SECURITY DEFINER`, sets `search_path` to `pg_catalog`, and schema-qualifies every name. The body is a fixed `SELECT ... FOR UPDATE`. It takes no tenant, no epoch, and no SQL text from the caller. `EXECUTE` is revoked from `PUBLIC`.

Until the role split described at `000103:6-7`, `EXECUTE` is granted to the shared login `freight` and to no other runtime login. After that split, `EXECUTE` moves to `tracking_runtime` and is revoked from the shared login. `shipment_runtime` does not receive `EXECUTE`. The function checks `pg_catalog.current_user` and raises an exception for any other caller.

The function runs inside the caller's transaction and does not commit. The execution-row lock therefore survives until the tracking transaction ends. That is the property the fence needs. A procedure with its own commit would release the lock before the state write and would miss the race.

### Why this variant

- The tracking login cannot `UPDATE` shipment columns. The owner role can lock the row, and the function does not expose that update right.
- The statement is static, so the execution id is a bound uuid. There is no string concatenation.
- Tenant authority is the row returned under the lock. The caller cannot pass a tenant that replaces it.
- Lock order stays in the application transaction: this call first, the tracking state lock second.
- A test can open two sessions on one database, call the function in one, and attempt the epoch update in the other.
- The operational risk is a superuser or table owner bypassing the function. The contract does not pretend otherwise. The old consumer is the same class of bypass and stays behind the closed mixed-group gate.

## 2. Narrow grant adapter

Name: `MINIMAL_GRANT_ADAPTER`.

Tracking would receive `SELECT` and the rights required to `SELECT FOR UPDATE` on `transport.transport_executions`, limited to `id`, `operating_tenant_id`, and `owner_epoch` if the server allows that column list. In PostgreSQL, `FOR UPDATE` requires `UPDATE` privilege on the target. A column-limited `UPDATE` grant still allows the tracking login to assign those columns. That is a wider right than locking and reading. A mistake in a query can change `operating_tenant_id` or `owner_epoch` from tracking-service.

SQL injection risk matches the function if the statements stay static. Tenant isolation depends on every tracking query using the locked row rather than the payload. Lock order depends on every caller. The variant is testable. The privilege-escalation risk is the reason it is not recommended. It remains a possible controller choice if the function form is rejected.

## 3. Single authoritative writer

Name: `SHIPMENT_SINGLE_WRITER`.

Shipment-service would be the only session that writes `tracking.execution_tracking_state`, inside the same transaction that locks and updates the execution row. Tracking-service would not write that table. The fence holds because the stale session has no write.

This removes the tracking bypass only if ETA ingest and `MarkApproachEmitted` also stop writing, or move into the shipment transaction. Those writers are in tracking-service today (`eta_ingest_service.go:247` and `approach.go:94`). `shipment_runtime` has no grant on schema `tracking` through migration `000105`. Giving shipment that write moves execution-tracking state across the service boundary. The Kafka consumer would still need a later proof read before `CommitRecords`, and PR #357 would need a contract revision.

The variant is coherent. It is a larger boundary change than the lock function. It is not the recommendation. A controller can still choose it. This document does not.

## 4. Comparison

| Risk | Lock function | Column grant | Single writer |
|---|---|---|---|
| SQL injection | Bound uuid, static SQL | Static SQL if every query is static | Static SQL in shipment |
| Privilege escalation | `EXECUTE` only | `UPDATE` on owner columns | Shipment gains tracking writes |
| Tenant isolation | Row under lock is the authority | Caller can still write a payload tenant if a query forgets the check | Shipment row is the authority |
| Lock order | Caller must keep the lock through commit | Caller must lock execution first | One transaction owns both rows |
| Tests | Two sessions, one database | Two sessions, one database | One session writes both tables |
| Operations | Superuser bypass remains | Tracking can change the epoch column | ETA and approach must move or stop |

## 5. Old consumer

The current binary reaches T1–T3 without the function. A mixed group lets that binary commit a state row after epoch 5. `MIXED_CONSUMER_GATE` stays `CLOSED`. The new mode does not start while that member can join. The function does not repair an offset the old binary has already committed.

## 6. Acceptance

- `TRACKING_TRANSPORT_SCHEMA_GRANT=NONE`.
- `LOCK_FUNCTION_HAS_COMMIT=NO`.
- `CALLER_TENANT_IS_AUTHORITY=NO`.
- `RECOMMENDATION=SECURITY_DEFINER_LOCK_FUNCTION`.
- `RECOMMENDATION_APPROVED_FOR_IMPLEMENTATION=NO`.

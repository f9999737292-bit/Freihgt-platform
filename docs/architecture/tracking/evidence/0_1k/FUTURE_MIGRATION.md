# Future migration notes for 0.1K

These statements are not a migration. Migration head `000105` is unchanged. Nothing here is applied to `freight_platform`, staging, or production.

## Column and trigger

`transport.transport_executions` has no `owner_epoch` column in `000088` or `000105`. A later migration would add:

```sql
ALTER TABLE transport.transport_executions
    ADD COLUMN owner_epoch bigint NULL;
```

The isolated trigger rejects a smaller or null replacement of a non-null epoch. It also rejects a change of `operating_tenant_id` unless the same update stores a greater epoch. That trigger is not in the platform migrations.

## Read function

`transport.read_binding_assignment(uuid, uuid)` would gain these checks, which the published 0.1H body and the 0.1J body do not have:

- The input execution must be the row in `transport.transport_execution_active_shipments` for that shipment. Otherwise raise `23514` and return no row.
- An execution target with `owner_epoch IS NULL` raises `42501`.
- An execution target with `current_revision_id IS NULL` raises `P0002`.
- The pointed-to revision must exist, belong to that execution and operating tenant, and have status `ACTIVE`. Otherwise raise `23514`.
- The function returns `owner_epoch bigint` from the execution row.
- The function reads `transport.transport_execution_active_shipments` and does not read `transport.transport_execution_participants`.

`assignment_lock_owner` would receive `SELECT (owner_epoch)` and would not receive `UPDATE` on `owner_epoch`. `UPDATE (updated_at)` remains the lock grant, and the direct update of `updated_at` remains an open risk for that owner.

## Already present in 000088

- `transport.transport_execution_active_shipments` has primary key `(shipment_tenant_id, shipment_id)`. A second active slot for one shipment is `23505`.
- `transport_execution_revisions_one_active_idx` allows one `ACTIVE` revision per execution. A second `ACTIVE` row is `23505`.
- `transport.reject_transport_execution_revision_mismatch` requires a non-null `current_revision_id` to name a revision of the same execution and operating tenant. It allows a null pointer. It does not require status `ACTIVE`.

## Active-link row lock

The 0.1K-R2 test body takes `FOR SHARE` on the one `transport.transport_execution_active_shipments` row for the shipment tenant and shipment id, after the execution and shipment locks and before the driver and vehicle locks. A plain `UPDATE` of `execution_id` waits on that row lock. The lock owner needs `UPDATE (updated_at)` on that table for `FOR SHARE`, and does not receive `UPDATE` on `execution_id`. That grant is part of the same open `updated_at` risk already recorded for the other locked tables. It is not a production privilege proof.

## Lock compatibility

Attempt 2 used the body that locks the execution and not the active-slot row. In that run the plain `UPDATE` of `execution_id` committed while the read was still in `pg_sleep`.

The 0.1K-R2 body locks the active-slot row. The same `UPDATE`, with no prior execution or shipment lock, then either waits or returns `55P03` while that transaction is open. After the reader commits, the same `UPDATE` can commit, and the next read of the old execution id is `23514`.

The test order is execution, shipment, active link, driver, vehicle. It is a test-contract hypothesis, not a change to the published 0.1H lock order. A writer that locks the shipment before the execution, or the active link before the execution, forms a cycle with this body. Attempt 4 records `40P01` for both of those orders, with the reader as the victim. Same-order writers waited and did not deadlock. This does not fence every production writer.

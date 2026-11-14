# 0.1O mandatory writer fencing contract

Status: `PROPOSED_NOT_APPROVED`. This is not an implementation and it does not close `J-DML-001`.

Base: `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6`.

The isolated proofs 0.1I through 0.1N show lock behavior of probe functions. They do not revoke production grants. A later direct `UPDATE` still succeeds after the reader commits.

## Authority is not one owner

| Change | Owner of the operation today | Proposed authority |
| --- | --- | --- |
| Create a new execution and its first active-link row | `Project` in shipment-service | Stays `Project`. `INSERT` only. It must not `UPDATE` an existing link row. |
| Replace `execution_id` on an existing active-link row | no Go method; `shipment_runtime` holds `UPDATE` | One database function. No direct `UPDATE`. |
| Change shipment `driver_id` or `vehicle_id` while a link exists | AssignDriver / AssignVehicle | Same transaction as an execution-first epoch bump. The shipment update stays in shipment-service. |
| Change shipment driver or vehicle when no active link exists | AssignDriver / AssignVehicle | Shipment transaction only. There is no execution epoch to bump. |
| Supersede the current revision | successor, including return and redirect | Stays the execution-first successor transaction. |
| Consume a shipment-owned binding decision | not implemented | Remains a separate closed gate. This contract does not authorize consumption. |

## Server-owned checks

The replacement function and the epoch bump used by assignment take `execution_id` and `shipment_id`. They do not take tenant, driver, vehicle, or epoch from the client as the source of authority.

Inside one transaction the function:

1. Locks the existing execution `FOR UPDATE` when the operation changes an existing execution or its link.
2. Locks the shipment `FOR UPDATE`.
3. Locks the active-link row `FOR UPDATE` when the operation changes that row.
4. Reads `operating_tenant_id` from the locked execution and `tenant_id` from the locked shipment.
5. Rejects the call when those row values disagree with the tenant already established by gateway JWT and stored in server context. A caller argument that merely matches a row is not that source.
6. Rejects a missing execution, a missing link when the call claims to retarget one, and a link whose `execution_id` is not the locked execution.
7. Reads `owner_epoch`. `NULL` means absent and fails closed for every execution-targeted change. `NULL` is not `1`.
8. Rejects a supplied expected epoch that is not the locked value. The stale result is a checked exception, not a committed write.
9. Reads `current_revision_id` and requires that revision to be `ACTIVE` for the same execution and operating tenant.
10. Applies one intended write.
11. Sets `owner_epoch` to the locked value plus one.
12. Commits once, or rolls the transaction back. A second commit in another service is not this boundary.

Repeat of the same command uses an idempotency key stored in the same transaction. A repeat after success returns the stored result and does not bump the epoch again. A second command with the old epoch fails.

Creating a new execution does not lock some other existing execution. Its first epoch is `1`, written on the new row in that insert. It is not an update of a previous epoch.

## Owner epoch

Proposed column: `transport.transport_executions.owner_epoch bigint NULL`.

- Existing rows stay `NULL` until an authorized bump. They are not backfilled to `1`.
- A `BEFORE UPDATE` trigger rejects a lower epoch when the old value is not null, and rejects an `operating_tenant_id` change unless the new epoch is strictly greater.
- The trigger does not fire for an insert of a new execution, a replica session, the table owner, or a superuser. It is not the DML closure.
- Direct `UPDATE` of `owner_epoch` is not granted to `shipment_runtime`.

## Privilege closure

`shipment_runtime` keeps:

- `INSERT` on the active-link table, for `Project`.
- `INSERT` on executions.
- `UPDATE (current_revision_id, event_seq, updated_at)` on executions.
- `UPDATE` of shipment status, driver, and vehicle through the assignment and command statements, after the epoch rule above.
- `EXECUTE` on the new fencing function.

`shipment_runtime` loses:

- `UPDATE` and `DELETE` on `transport.transport_execution_active_shipments`.
- table-level `UPDATE` on executions. Column grants replace it.
- `UPDATE` of `owner_epoch`, `operating_tenant_id`, `id`, `driver_id`, and `vehicle_id` on executions.

The table owner and the shared superuser login still bypass these grants. Closing that bypass means the runtime login is not the table owner and is not superuser. This document does not perform that role split. `J-DML-001` stays open until a non-owner, non-superuser session receives `42501` for a direct link `UPDATE` and for an epoch `UPDATE`.

## Lock order

The reader order already probed is:

`execution FOR SHARE -> shipment FOR SHARE -> active link FOR SHARE -> driver FOR SHARE -> vehicle FOR SHARE`

Writers that touch an existing execution take that execution `FOR UPDATE` first, then the shipment, then the active-link row when they change it.

Forbidden without a new deadlock analysis: locking an existing execution after the shipment is already held. Command execution already holds the execution and then waits for the shipment. Activation already holds the shipment and then inserts a new execution. Those two orders must not be joined by adding the missing lock at the end of either path.

| Path | Change | New edge | Cycle with the reader or with commands |
| --- | --- | --- | --- |
| `Project`, flag off | still inserts a new execution, then a new link | none to an existing execution | no |
| `Project`, flag on | still locks shipment, cargo, driver, and vehicle, then inserts a new execution | none to an existing execution | no, while it does not lock the existing execution afterward |
| AssignDriver / AssignVehicle with a link | lock that link's execution first, bump epoch, then update the shipment | `execution -> shipment` | no against execution-first commands. A shipment-first bump would cycle. |
| Assign without a link | shipment update only | none | no |
| Successor, return, redirect | unchanged execution-first order | none | no new edge |
| Direct link `UPDATE` | revoked | the only remaining writer is the function, which locks execution first | the bypass path is the point of the revoke. It is not closed by this document. |
| Pin propagation | called inside the successor transaction that already holds the execution | reads and inserts pin rows; no shipment or link lock in the function body | no cycle is shown. The call is not declared safe. |

Activation must not call the retarget function after it has locked the shipment. Retarget is a different operation and starts at the execution lock.

## Fail closed

The function raises and rolls back when the execution is missing, the link does not match, the epoch is null or stale, the revision is not the active revision, or the locked tenant does not match server context. It does not choose another execution. It does not leave a new execution committed when the link insert fails: `Project` already uses one transaction, and that transaction remains one transaction.

## What this contract does not prove

Tenant isolation tests, presenter identity, decision consumption, broker delivery, and a non-superuser grant test are not part of this document. `ALL_WRITERS_FENCED` stays `NO` until those checks exist.

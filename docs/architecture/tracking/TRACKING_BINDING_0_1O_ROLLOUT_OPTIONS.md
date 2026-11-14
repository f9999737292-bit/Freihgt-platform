# 0.1O rollout options

Status: `PROPOSED_NOT_APPROVED`. No option is safe by this document. No migration is executed. The observed platform head is `000105`. The next number is not reserved. An implementation must list `infrastructure/migrations/` again before choosing a number, because a parallel change can take `000106` first.

Existing rows keep `owner_epoch NULL`. Null stays absent. There is no backfill to `1`.

## Shared facts

The shared `freight` login remains the deployment login (`000103:6-8`). While it is superuser or owns the active-link table, `REVOKE` from `shipment_runtime` does not bind it. Rollback of a grant change is a later `GRANT` of the previous privilege. Rollback of a column is a down script that drops the trigger and the column only after no product statement reads it. Both rollbacks are designs, not scripts in this change.

During the transition, old and new writers can run together. The bypass stays open until the `REVOKE` commits. Shipping the function without the `REVOKE` does not close direct DML.

A reader `FOR SHARE` on PostgreSQL 16 still needs `UPDATE` on at least one column. That column grant must not be `execution_id` or `owner_epoch`. If the lock owner can update `updated_at`, that direct update remains a known hole. The fencing function does not remove it.

## B1 — restricted DML and SECURITY DEFINER

Every insert and update of executions, shipments, revisions, and the active link moves into definer functions. `shipment_runtime` loses table DML and keeps `EXECUTE`.

| Question | Assessment |
| --- | --- |
| DML bypass | Closed for `shipment_runtime` only after `REVOKE`. Owner and superuser remain. |
| Tenant | The function can read the locked row. That is not a tenant-isolation proof. |
| Owner epoch | Can be checked in the same function. |
| Lock order | One function can take execution, then shipment, then link. Activation must still insert a new execution without locking a different existing execution after the shipment lock. |
| Go compatibility | Every current writer listed in the privilege matrix changes. Highest service risk. |
| Migrations | Column, trigger, functions, and grant replacement. Largest schema change. |
| Rollback | Re-grant table DML and stop calling the functions. A failed cutover blocks `Project`, commands, and assignment together. |
| Outage | A bad function body stops all of those writes. |
| Shared login | Not solved. |
| Steps | One cutover. A partial deploy leaves some writers on grants and some on functions. |

## B2 — one write-authority service

A new service owns the writes. Its database role holds the grants. Shipment-service loses them.

| Question | Assessment |
| --- | --- |
| DML bypass | Closed only if the old role actually loses the grant. A second service with its own commit is not one transaction. |
| Tenant | Moves to that service. Still requires the gateway-trusted context. |
| Owner epoch | Can be checked only inside the service transaction that holds the row locks. |
| Lock order | The service must not lock an existing execution after holding the shipment. |
| Go compatibility | Moves `Project`, assignment, commands, and successor across a service boundary. |
| Migrations | Role split plus the same column and grants. |
| Rollback | Restore grants to shipment-service and route calls back. Two deployments move together. |
| Outage | The new service becomes a single write dependency. |
| Shared login | Solved only if the new role is not that login. |
| Steps | Cannot be the first step. The database transaction, not the process, is the authority boundary. |

## B3 — hybrid

Current writers keep the statements they already use. Only the sensitive changes move.

- `Project` keeps `INSERT` of a new execution and a new link.
- Successor, return, and redirect keep execution-first revision writes.
- Commands keep execution-first shipment updates and `event_seq` updates.
- Replacement of an existing link uses one definer function that locks execution, then shipment, then the link.
- AssignDriver and AssignVehicle, when a link exists, lock that execution first and bump `owner_epoch` in the same transaction, then update the shipment.
- `shipment_runtime` loses `UPDATE` and `DELETE` on the active-link table and loses table-level `UPDATE` on executions. It keeps column `UPDATE` of `current_revision_id`, `event_seq`, and `updated_at`.

| Question | Assessment |
| --- | --- |
| DML bypass | The observed link `UPDATE` grant can be revoked without taking `INSERT` from `Project`. Owner and superuser remain. `J-DML-001` stays open. |
| Tenant | Same locked-row check as B1 for the new function. Existing writers do not gain a new tenant proof. |
| Owner epoch | Bumped on link replacement and on assignment while a link exists. Not bumped by a shipment-only update that has no link. |
| Lock order | Adds `execution -> shipment` for assignment that has a link. Does not add shipment-then-existing-execution. |
| Go compatibility | `Project`, successor, and command SQL stay. Assignment gains a lock and an epoch write. |
| Migrations | One column, one trigger, one function, and a grant narrowing. Smaller than B1. |
| Rollback | Re-grant the previous `UPDATE`. Assignment can ship behind a flag that skips the epoch bump only before the `REVOKE`. After the `REVOKE`, rollback is the re-grant. |
| Outage | A bad function blocks link replacement and linked assignment. `Project` and commands still use their current statements. |
| Shared login | Not solved. |
| Steps | Add the dormant column and trigger. Add the function and the assignment lock. Revoke the wide grants only after the non-superuser denial test. |

## Order

1. Confirm the migration head again. Do not reuse `000105`.
2. Add `owner_epoch` and the trigger. Do not revoke anything in that step.
3. Add the retarget function and the execution-first assignment bump. Grants still allow the old `UPDATE`, so this step does not close the bypass.
4. Run the isolated denial test as a non-owner, non-superuser role: direct `UPDATE` of `execution_id` and of `owner_epoch` returns `42501`; `Project` `INSERT` still commits; successor `current_revision_id` and command `event_seq` still commit.
5. Revoke the wide privileges in the same release as that passing test.
6. Keep the shared-login split as a separate authorization. Do not call step 5 closed while the application login is superuser or owns the table.

B3 is the proposed sequence. B1 remains the later destination if more statements must leave application SQL. B2 is not the authority boundary.

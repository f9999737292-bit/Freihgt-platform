# 0.1O rollout options

Status: `B3_REDESIGNED_PROPOSED_NOT_APPROVED`. No option is safe by this document. No migration is executed. The observed platform head is `000105`. The next number is not reserved. An implementation must list `infrastructure/migrations/` again before choosing a number, because a parallel change can take `000106` first. This proposal does not edit `000105`.

Existing rows keep `owner_epoch NULL`. Null stays absent. There is no backfill to `1`.

## Shared facts

The shared `freight` login remains the deployment login (`000103:6-8`, `000105:5`). While that login is the application session, a revoke aimed at `shipment_runtime` does not bind it. Superuser, table ownership, and the ability to issue an authority ticket from the same login each bypass the designed function. `TRUSTED_TENANT_AUTHORITY=DESIGN_BLOCKED_BY_SHARED_LOGIN`. A later change of that login's attributes is a separate authorization. It is not assumed here, and it is not described as having closed the boundary.

`ACTIVE_LINK_TABLE_OWNER=NOT_PROVEN`. No migration transfers `transport.transport_execution_active_shipments`.

`REVOKE UPDATE` on the active-link table is not sufficient. `INSERT` can still name an existing execution, and a privileged runtime connection can still write. A rollback that grants the previous `UPDATE` or `INSERT` returns to that unsafe state. It is not a safe default rollback. A column rollback that drops the trigger and the epoch column is a later down script, and only after no product statement reads the column. Neither rollback is a script in this change.

During the transition, old and new writers can run together. The bypass stays open until the runtime no longer has the direct privilege and the session executing the statement is not the table owner or a superuser. Shipping a function without that revoke does not close direct DML.

A reader `FOR SHARE` on PostgreSQL 16 still needs `UPDATE` on at least one column. That column grant must not be `execution_id` or `owner_epoch`. If the lock owner can update `updated_at`, that direct update remains a known hole.

## B1 — restricted DML and SECURITY DEFINER

Every insert and update of executions, shipments, revisions, and the active link moves into definer functions. `shipment_runtime` loses table DML and keeps `EXECUTE` on those functions.

| Question | Assessment |
| --- | --- |
| DML bypass | Closed for `shipment_runtime` only after `REVOKE`, and only if the session is not the owner or superuser. |
| Tenant | Still requires the separate issuer login. The shared login does not become that issuer. |
| Owner epoch | Can be checked in the same function. A retained table `UPDATE` is not fencing. |
| Lock order | Create, retarget, and assignment use the execution-before-shipment order in the fencing contract. |
| Go compatibility | Every current writer listed in the privilege matrix changes. Highest service risk. |
| Migrations | Column, trigger, functions, and grant replacement. Largest schema change. |
| Rollback | Re-granting table DML returns to the open privilege state. |
| Outage | A bad function body stops all of those writes. |
| Shared login | Not solved. |
| Steps | One cutover. A partial deploy leaves some writers on grants and some on functions. |

## B2 — one write-authority service

A new service owns the writes. Its database role holds the grants. Shipment-service loses them.

| Question | Assessment |
| --- | --- |
| DML bypass | Closed only if the old role actually loses the grant and the new role is not `freight`. A second service with its own commit is not one transaction. |
| Tenant | The new service still needs a database identity the runtime cannot forge. |
| Owner epoch | Can be checked only inside the transaction that holds the row locks. |
| Lock order | The service must lock existing executions before the shipment. |
| Go compatibility | Moves `Project`, assignment, commands, and successor across a service boundary. |
| Migrations | Role split plus the same column and grants. |
| Rollback | Restoring the old grants returns the writes to the open role. |
| Outage | The new service becomes a single write dependency. |
| Shared login | Solved only by a login that is not `freight`. That login is not in the migrations. |
| Steps | Cannot be the first step. The database transaction, not the process, is the authority boundary. |

## B3 — redesigned hybrid

Current command and successor statements stay. Sensitive link and epoch writes move to the functions in the fencing contract. This is a proposal, not an approval.

- The issuer commits the ticket in one transaction. For `CREATE_ACTIVE_LINK` and `CREATE_EXECUTION` that ticket already stores one non-null execution UUID. `Project` consumes it in a later transaction and cannot substitute another UUID. Those two sessions do not share a transaction id. Direct `INSERT` of the execution and of the active link by `shipment_runtime` is removed. The create function sets `owner_epoch` to `1` only from that ticket. A table-level `INSERT` left in place could set another value. `xmin` may show that the execution row was written in the consume transaction; it is not the authority check. The audit row stores `audit.ticket_id`. `INITIAL_INSERT_AUTHORITY=NOT_PROVEN` until the issuer is a different principal from `freight`. Presenting the ticket does not prove presenter identity.
- Successor, return, and redirect keep execution-first revision writes.
- Commands keep execution-first shipment updates and `event_seq` updates.
- Retarget locks the source and the destination in uuid order before the shipment, then locks the link, rechecks under those locks, and writes the link, both epochs, and the audit row.
- AssignDriver and AssignVehicle do not lock a shipment and then lock an execution. A linked assignment locks the execution first. An assignment with no link locks only the shipment and aborts if a link is present under that lock.
- `shipment_runtime` loses `INSERT`, `UPDATE`, and `DELETE` on the active-link table, loses table-level `INSERT` and `UPDATE` on executions, and receives no `INSERT` on `owner_epoch`. It keeps column `UPDATE` of `current_revision_id`, `event_seq`, and `updated_at`. A `BEFORE INSERT` trigger may also reject a non-null epoch. The table owner and a superuser can bypass that trigger, so the revoke is the control.
- The function owner, the caller, and the issuer are three `NOLOGIN` roles. The runtime is a member of none of them. The active-link table owner stays `NOT_PROVEN` and is not the function owner.

| Question | Assessment |
| --- | --- |
| DML bypass | Open until the revokes are applied to a session that is not owner and not superuser. `J-DML-001` stays open. |
| Tenant | `DESIGN_BLOCKED_BY_SHARED_LOGIN`. The ticket design is specified. The issuer login is not. |
| Owner epoch | Incremented only inside the fencing transaction. A wide `UPDATE` that remains is not fencing. `NULL` stays a refusal. |
| Lock order | Existing executions are locked before the shipment. The active-link foreign key is in the graph. `DEADLOCK_FREE` is not claimed. |
| Go compatibility | Successor and command SQL stay. `Project` replaces the execution `INSERT` and the link `INSERT` with the create function, after a committed ticket. Assignment locks that ticket before the shipment when a link exists. |
| Migrations | Not numbered here. Column, trigger, functions, ticket table, audit table, and grant changes are later work. |
| Rollback | Re-granting `INSERT` or `UPDATE` restores the unsafe privilege. It is not the default recovery path. |
| Outage | A bad function blocks create, retarget, and linked assignment. Commands still use their current statements only while their column grants remain. |
| Shared login | Blocker. Not scheduled as a silent prerequisite. |
| Steps | The sequence below. Step 9 is a separate cutover decision. |

## Order

1. Inventory the live database owner of `transport.transport_execution_active_shipments`, the owner of `transport.transport_executions`, and every login that runs shipment-service. Do not invent those names from the migrations. The migration result remains `NOT_PROVEN`.
2. Record whether the shared `freight` login is superuser, bypasses row security, owns either table, or is a member of `shipment_runtime`. Membership outside the numbered migrations stays `UNKNOWN` until that inventory.
3. Design the issuer login as a different principal from `freight` and from `shipment_runtime`. Until that login exists, do not install the fencing functions as if the ticket check were enforceable.
4. Run isolated grant and revoke tests on disposable roles: `PUBLIC` has no `EXECUTE`, the caller has `EXECUTE`, the runtime has no link `INSERT` or `UPDATE`, and the runtime cannot `SET ROLE` to the function owner.
5. Prove the create function after a committed ticket whose execution UUID is already E1: only E1 is created, a null execution id is denied, E2 is denied, an already existing E1 is denied, the wrong shipment or tenant is denied, a second consume after commit is denied, and rollback leaves no execution, link, or audit row while the ticket stays `ISSUED`. A different execution id inserted in the same top-level transaction is denied. The execution and its link commit together, a second link conflicts, and a runtime `INSERT` of `owner_epoch = 5` is denied. A subtransaction `xmin` mismatch fails closed. `xmin` is not authority, and `audit.ticket_id` is the audit field.
6. Run the retarget test: `retarget A→B` against an execution-first command on B, both start orders, with lock waits, SQLSTATE, rollback, and final state. Include same-source, missing destination, competing retarget, and a source that changes before the locks. Do not record `DEADLOCK_FREE=YES` from a review of this document.
7. Run the epoch and audit tests: `NULL` rejects, a stale epoch rejects, rollback restores the epoch and leaves no audit row, and a committed change has one audit row whose required field is `audit.ticket_id`.
8. Run `Project`, AssignDriver, and AssignVehicle against the new functions, including flag-on activation and an assignment that finds no link.
9. Take the production cutover as its own decision after steps 1 through 8. The cutover includes the revokes. It is not implied by merging this document.

B3 is the redesigned proposal. B1 remains a later, larger move of every writer. B2 is not the authority boundary, because two service commits are not one database transaction.

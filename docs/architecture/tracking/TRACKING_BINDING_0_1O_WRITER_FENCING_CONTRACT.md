# 0.1O mandatory writer fencing contract

Status: `B3_REDESIGNED_PROPOSED_NOT_APPROVED`. This is not an implementation and it does not close `J-DML-001`.

Base: `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6`.

The isolated proofs 0.1I through 0.1N show lock behavior of probe functions. They do not revoke production grants. A later direct `UPDATE` still succeeds after the reader commits.

`TRUSTED_TENANT_AUTHORITY=DESIGN_BLOCKED_BY_SHARED_LOGIN`. The lock order and the function shape below are the required design. They are not a claim that the current database login can enforce them.

## Authority is not one owner

| Change | Owner of the operation today | Required authority |
| --- | --- | --- |
| Create a new execution and its first active-link row | `Project` inserts both in one transaction (`transport_execution_repository.go:31-126`, execution insert `:295`, link insert `:335-340`) | A committed ticket, then one `Project` transaction. The function inserts the execution and the link. `shipment_runtime` loses direct `INSERT` on both tables. |
| Replace `execution_id` on an existing active-link row | no Go method; `shipment_runtime` holds `UPDATE` (`000104:18`) | One retarget function. No direct `UPDATE`. |
| Change shipment `driver_id` or `vehicle_id` while a link exists | AssignDriver `:433` and AssignVehicle `:473` update the shipment and do not lock an execution | One assignment function. The execution named by the link is locked before the shipment. |
| Change shipment driver or vehicle when no active link exists | same AssignDriver / AssignVehicle statements | Shipment lock only. The transaction rolls back if a link appears before the shipment lock is held. It does not then lock that execution. |
| Supersede the current revision | successor, including return and redirect | Stays the execution-first successor transaction. |
| Consume a shipment-owned binding decision | not implemented | Remains a separate closed gate. This contract does not authorize consumption. |

## Who may write, and why

The role that may change an active link or `owner_epoch` is a designed `NOLOGIN` role named `binding_authority_owner`. It is not created by this proposal.

| Property | Required value |
| --- | --- |
| Owner attributes | `NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS` |
| Why this role may write | It owns the fencing functions. `SECURITY DEFINER` runs the function body as this role. The body is the only place that locks both executions, revalidates, writes, and inserts the audit row. |
| Table owner | Different role. `binding_authority_owner` does not own `transport.transport_execution_active_shipments` or `transport.transport_executions`. One owner of both the table and the function could drop the trigger and write past the function. |
| Active-link table owner | `ACTIVE_LINK_TABLE_OWNER=NOT_PROVEN`. The table is created at `000088:76-81`. No `ALTER TABLE ... OWNER TO` for it was found. |
| Runtime membership | `shipment_runtime` is not a member of `binding_authority_owner` and is not granted `SET ROLE` to it. `NOINHERIT` on both roles is required and is not itself the boundary. |
| Function mode | `SECURITY DEFINER` |
| `search_path` | `pg_catalog` |
| Names | Every table, function, and type in the body is schema-qualified. |
| `PUBLIC` | `REVOKE ALL ON FUNCTION ... FROM PUBLIC` in the same transaction as `CREATE FUNCTION`, before `COMMIT`. PostgreSQL grants `EXECUTE` to `PUBLIC` on a new function. `000103:1184-1198` already revokes that default for the pin functions. No `ALTER DEFAULT PRIVILEGES` in `000103`, `000104`, or `000105` changes the default. The creating role also sets `ALTER DEFAULT PRIVILEGES IN SCHEMA transport REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC` so a later function is not born executable. |
| `EXECUTE` | Granted only to a designed `NOLOGIN` role `binding_authority_caller`. Not granted to `PUBLIC`, `freight`, or `shipment_runtime`. |
| Wider function | No second function receives `INSERT`, `UPDATE`, or `DELETE` on the active-link table or on `owner_epoch`. `transport.propagate_execution_relationship_pins(uuid)` is unchanged and does not write that table. |
| Audit | Every accepted change inserts one audit row in the same transaction. |

`freight_migrations` is not this owner. It already owns the pin function and the pin tables (`000103:1150`, `:176`) and can update revisions (`000103:1222`). Adding link authority to that role would widen it.

## Trusted tenant authority

This rule is rejected:

`caller_tenant_id = locked_row.tenant_id` means authorized.

A SQL function does not receive the Go `AuthContext`. An argument that carries a tenant, and a `SET` of a custom session variable, are both chosen by the session that can execute the statement. Neither one is the gateway's JWT decision.

The trust boundary that already exists in the product is the API gateway. It authenticates the bearer token and derives the tenant and the principal. Shipment-service sees that derived context. The database session does not.

The required database mechanism is a ticket that one session commits and a later session consumes. Two connections do not share a PostgreSQL transaction id. The rule `ticket.xact_id = pg_current_xact_id()` is rejected for a ticket issued by another session.

`binding_authority_issuer` is a designed `NOLOGIN` non-superuser. It is not `binding_authority_owner`, `binding_authority_caller`, `shipment_runtime`, or `freight`. It is the only role that may insert a ticket. `EXECUTE` on the issuer function is not granted to the writer, and the issuer does not grant the writer its own role. The issuer sets the principal and the tenant from identity that the gateway has already checked. A tenant value supplied by the writer is not that check. The issuer also checks that the operation, the shipment, the execution ids, and the scope are the ones that identity may authorize. The writer later proves possession of that committed ticket. It does not prove the tenant by passing it again.

The ticket is one authorization, not a bearer token for an arbitrary write. Its fields are:

| Field | Meaning |
| --- | --- |
| `ticket_id` | Unique random id. |
| `issuer_id` | The issuer role that committed the row. |
| `principal_id`, `tenant_id` | Taken from the checked identity, not from the writer argument. |
| `operation` | One of `CREATE_EXECUTION`, `CREATE_ACTIVE_LINK`, `RETARGET`, `ASSIGN_DRIVER`, `ASSIGN_VEHICLE`, `ESTABLISH_EPOCH`. |
| `shipment_id` | The shipment this ticket may name. |
| `source_execution_id`, `destination_execution_id` | Set when the operation has those rows. Unused columns stay null. |
| `expected_owner_epoch`, `expected_revision_id` | The values the writer must find under its locks. |
| `issued_at`, `expires_at` | Lifetime. `clock_timestamp()` at or after `expires_at` fails. |
| `status` | `ISSUED`, `CONSUMED`, `EXPIRED`, or `REVOKED`. |
| audit columns | Issuer principal, issue time, and, after a successful consume, the consuming transaction's audit id. |

Transaction A, issue, is a different session from the writer:

1. The issuer checks the identity, the operation, and the shipment and execution scope.
2. It inserts one row with status `ISSUED`.
3. It commits. The ticket is durable before the writer starts. The ticket table has no foreign key to executions, shipments, or active links, so this insert does not take `FOR KEY SHARE` on those rows.

Transaction B, consume and write, is the writer:

1. Begin.
2. `SELECT` the ticket `FOR UPDATE`.
3. Reject unless the status is `ISSUED`, `clock_timestamp()` is before `expires_at`, `issuer_id` is `binding_authority_issuer`, and the operation is the one this function performs.
4. Lock the business rows in the order below.
5. Read the locked version, epoch, revision, tenant, and execution ids again. A mismatch with the ticket rolls back.
6. Perform the one allowed write.
7. Insert the audit row in this same transaction.
8. Set the ticket status to `CONSUMED`.
9. Commit once.

A failure anywhere in transaction B rolls back the write, the audit insert, and the status change. The ticket returns to `ISSUED` because its update was in B. That is recovery after rollback. It is not permission for a second committed consume.

Reuse is stopped as follows. The ticket row lock serializes concurrent consumers. The status change to `CONSUMED` is in the same commit as the write, so a later transaction sees `CONSUMED` and fails. An expired ticket fails even if the status is still `ISSUED`. A different operation or a different tenant fails the field comparison. The issuer revokes an outstanding ticket in its own transaction: it locks that ticket `FOR UPDATE`, and if the status is `ISSUED` it sets `REVOKED`. It does not lock execution, shipment, or link rows. If the writer already holds the ticket lock, the revocation waits. If the revocation commits first, the writer sees `REVOKED` and rolls back. If the writer commits `CONSUMED` first, the revocation sees that status and leaves the committed write in place.

A second session that merely learns `ticket_id` is not a proven different principal. `binding_authority_caller` is one shared role, not an identity per principal. Blocking that session requires a caller identity the current login does not have. That check is `NOT_PROVEN`.

The issuer is not a boundary that exists today. `000103:6-8` keeps `freight` as the shared login. `000105:5` does not alter that login. `000105:8-9` already leaves an award-link lock function uncreated until a server-owned caller identity exists. The integration test that grants `shipment_runtime` to `freight` is not a migration. A session that is `freight` can hold superuser, own the tables, `SET ROLE`, and call any function granted to its members. The same login cannot be the independent issuer and the writer. `TRUSTED_TENANT_AUTHORITY=DESIGN_BLOCKED_BY_SHARED_LOGIN`. The designed issuer role is not described as a boundary that is already in force.

`transport.assert_activation_enforcement_role_split` (`000103:1088-1135`) rejects a superuser, bypassrls, or pin-table owner named `freight`. Its owner check does not name `transport.transport_execution_active_shipments`. Passing that function would not prove this boundary.

No later configuration step is declared to have closed the boundary. `TRUSTED_TENANT_AUTHORITY=DESIGN_BLOCKED_BY_SHARED_LOGIN`.

## Initial active-link insert

`shipment_runtime` has `INSERT` on `transport.transport_execution_active_shipments` (`000104:18`). The foreign key at `000088:79` checks that `execution_id` exists. It does not check that this transaction created that execution, and it does not check the shipment tenant. Direct `INSERT` can therefore name any existing execution.

The grant cannot be narrowed to "the execution inserted by this statement." A column grant does not carry that predicate. Keeping `INSERT` on `shipment_runtime` is not part of this redesign.

The required create path is two transactions. Transaction A commits a `CREATE_EXECUTION` ticket, or one ticket that names both the new execution and the shipment for `CREATE_ACTIVE_LINK`. Transaction B is the `Project` transaction. It locks that ticket before it locks the shipment. Flag-on activation today locks the shipment first (`transport_execution_activation.go:67-71`). That order is not the consume order. The future transaction locks the ticket, then the shipment, cargo, driver, and vehicle, then inserts the new execution.

Inside transaction B the create function:

1. Requires the locked ticket to be the committed `CREATE_EXECUTION` or `CREATE_ACTIVE_LINK` ticket for this shipment. The ticket is the authority check. The function does not establish the issuer. `INITIAL_INSERT_AUTHORITY=NOT_PROVEN` while the issuer remains `DESIGN_BLOCKED_BY_SHARED_LOGIN`.
2. Inserts the execution with `owner_epoch` set to `1`, or accepts an execution inserted earlier in this same top-level transaction. An execution committed by an older transaction is rejected.
3. May compare the execution row's `xmin` with the top-level transaction id, the same comparison as `transport.revision_inserted_in_current_xact` (`000103:332-339`). A subtransaction xid does not equal that top-level id, and the comparison then fails. `xmin` is only extra evidence that the row was written here. It does not replace the ticket.
4. Locks the shipment if this transaction has not already locked it after the ticket, and reads its tenant and version under that lock.
5. Inserts one active-link row for that shipment and that execution.
6. Inserts the audit row and sets the ticket to `CONSUMED` before the one commit.

The primary key `(shipment_tenant_id, shipment_id)` rejects a second link. `Project` already maps that conflict to a rollback (`:100-102`). A replay of the same activation returns the stored execution (`:44-53`) and does not insert another link. The new execution is not an existing execution, so the shipment lock that follows the ticket does not take an existing execution after the shipment.

## Retarget lock order

`transport_execution_active_shipments.execution_id` references `transport.transport_executions.id` (`000088:79`). The constraint name is `transport_execution_active_shipments_execution_id_fkey`. It is `NO ACTION` and not deferrable. An `UPDATE` of `execution_id` takes `FOR KEY SHARE` on the destination execution. If that is the first lock on the destination, and the shipment is already held, the edge is `SHIPMENT → EXISTING EXECUTION`.

An execution-first command on the destination holds that execution `FOR UPDATE` and then waits for the shipment. Those two orders cycle. The previous revision of this contract locked only the source before the shipment and left the destination to the foreign-key check. That order is withdrawn.

The retarget transaction consumes a ticket that transaction A already committed. Its locks are:

1. Read the current link and the requested destination. This read is a candidate list. It is not authorization and it is not repeated as a fallback after a mismatch.
2. Confirm both ids currently exist. A missing destination fails this step and fails again if the later lock finds no row.
3. Lock the ticket `FOR UPDATE` and apply the consume checks. This lock is first. The issuer's revocation locks the same ticket and no business row, so the two sessions cannot take the ticket and an execution in opposite orders.
4. Lock the distinct execution ids `FOR UPDATE` in ascending uuid order. The source and the destination are both in that list. The destination lock is taken here, not by the later foreign-key check.
5. Lock the shipment `FOR UPDATE`.
6. Lock the active-link row `FOR UPDATE` by its primary key `(shipment_tenant_id, shipment_id)`.
7. Under those locks, read the link, both executions, the shipment, the current revision, and both epochs again. Require the ticket's tenant, principal, operation, execution ids, expected epoch, and expected revision to match these locked rows. Require each execution's `current_revision_id` to name an `ACTIVE` revision of that execution and operating tenant.
8. Apply the `execution_id` change, increment the source epoch and the destination epoch, insert one audit row, and set the ticket to `CONSUMED`. Then commit. Any failed check rolls the transaction back. No earlier statement in this transaction has committed.

The foreign-key `FOR KEY SHARE` then falls on a destination row this transaction already holds `FOR UPDATE`. It does not open a new execution lock after the shipment.

### Same source and destination

The id list has one execution. The transaction locks that execution, then the shipment, then the link. The recheck requires the locked link to name that same execution. The foreign-key check key-shares that same row. There is no second execution.

### Missing destination

The pre-read rejects an unknown destination. If the row is deleted before the lock, `SELECT ... FOR UPDATE` finds no row and the transaction rolls back. The function does not insert a link and does not choose another execution.

### Competing retarget

Each transaction has already locked its own ticket. Two transactions then lock overlapping execution ids in the same uuid order, then the same shipment. One waits. A second consume of a ticket that is already `CONSUMED` fails instead of waiting on the executions. The waiter rechecks after the lock. If the link no longer names the source it read, it rolls back. It does not continue with the old source.

### Source changed before the locks

The locked link is compared with the source id from the candidate read. A mismatch rolls back. The caller may open a new transaction and read the source again. The function does not apply the old source after that mismatch.

### Assignment

AssignDriver and AssignVehicle do not look the link up after locking the shipment and then lock the execution. The required sequence is:

1. Read the link without holding the shipment lock. This read only chooses which committed ticket to present.
2. Lock that ticket `FOR UPDATE` before any shipment or execution lock.
3. If the read finds no link, lock the shipment and read the link again. If a link now exists, roll back. Do not lock the execution in this transaction.
4. If the read finds a link, lock that execution, then the shipment, then the link. Recheck the execution id, tenant, revision, and epoch against the ticket. Then update the shipment column, increment the epoch, write the audit row, and mark the ticket `CONSUMED`.
5. A mismatch rolls back. A new transaction may read the link again and must use a ticket that is still `ISSUED`.

## Owner epoch

Proposed column: `transport.transport_executions.owner_epoch bigint NULL`. It is not in the schema. Existing rows stay `NULL`. `NULL` is not `1`. A `NULL` epoch rejects `RETARGET`, `ASSIGN_DRIVER`, `ASSIGN_VEHICLE`, and any other execution-targeted change.

| Rule | Required behavior |
| --- | --- |
| Initial value | The create function sets `1` on the execution it inserts. Runtime `INSERT` is not an initial-value path. |
| Existing `NULL` row | No backfill. `ESTABLISH_EPOCH` is a separate ticketed operation. Under the execution lock it changes `NULL` to `1` and writes an audit row. It does not accept a caller-supplied number. Rows that stay `NULL` continue to fail closed. |
| Who increments | Only `binding_authority_owner`, inside the fencing function, from the locked value to that value plus one. |
| Link retarget | The source and the destination are both checked. The shipment leaves the source and arrives at the destination, so both epochs increment in the one transaction. |
| Assignment | Driver or vehicle change increments the one execution named by the locked link. The link row stays in place. |
| No link | The shipment update does not read or write an epoch. |
| Atomicity | The epoch write, the link or shipment write, and the audit insert commit together or roll back together. |
| Rollback | PostgreSQL restores the previous epochs, the previous link, the previous shipment columns, and removes the audit insert. |
| Stale epoch | The caller passes the epoch it observed. The function compares it with the locked column. A mismatch raises and rolls back. The new value is not committed. |
| Other `UPDATE` paths | A column grant on `current_revision_id`, `event_seq`, or `updated_at` does not include `owner_epoch`. A remaining table-level `UPDATE`, a table owner, or a superuser can still write the column. Those privileges are not epoch fencing. |
| Runtime `INSERT` | `shipment_runtime` has table-level `INSERT` on `transport.transport_executions` (`000103:1225`). `Project` uses it at `:295` and does not name an epoch, because the column does not exist. Once the column exists, that same table privilege can insert `owner_epoch = 5`. A column-level `INSERT` list does not remove that ability while the table-level grant remains. `NULL` as the default is not a control if the role can name the column. |

## Owner-epoch insert

Three measures were compared.

| Measure | What it stops | What it does not stop |
| --- | --- | --- |
| A. Create-execution function | The function, running as `binding_authority_owner`, is the only statement that inserts the row and the only statement that may set a non-null epoch, and only from a committed ticket. | Nothing, if `shipment_runtime` still has table-level `INSERT`. |
| B. Column-level `INSERT` | After table-level `INSERT` is revoked, a grant that omits `owner_epoch` makes `INSERT ... owner_epoch = 5` return `42501`. Omitting the column stores the default. | The table-level grant at `000103:1225` already covers every column. A narrower column grant does not override it. Omitting the column also does not prove the create ticket. |
| C. `BEFORE INSERT` trigger | An enabled trigger can reject a non-null epoch from a role other than the function owner. | The table owner and a superuser can disable or drop it. `session_replication_role = replica` can skip it where that setting is permitted. It is not the boundary. |

The chosen combination is A, with the table-level `INSERT` revoked from `shipment_runtime` so that B's column restriction is real, and with C as an extra guard. Runtime receives no `INSERT` on `owner_epoch` and no table-level `INSERT`. A runtime insert that names `owner_epoch = 5` is denied. A runtime insert that omits the column is also denied: the create contract does not leave an unfenced insert of a `NULL` epoch. The function sets `1` only for a committed create ticket. A stale epoch and a decrease are denied by the consume recheck and by the `BEFORE UPDATE` trigger. Rollback of transaction B restores the epoch, the business write, and the audit row together. The table owner and a superuser are not treated as limited by the grant or by the trigger.

The revoke is a proposal. `000103:1225` is unchanged, so the bypass exists until that grant is replaced.

## Audit

Each accepted create, retarget, assignment bump, and epoch establishment inserts one row in the same transaction. The row records the operation, the execution ids, the shipment id, the previous and new epochs, the ticket nonce, and the principal stored on the ticket. The audit table is owned by `binding_authority_owner`. `shipment_runtime` is not granted `INSERT` or `DELETE` on it. A failed check does not leave the row behind, because the insert is in the transaction that rolls back.

## Trigger

A proposed `BEFORE UPDATE` trigger on `transport.transport_executions` rejects a lower `owner_epoch` when the old value is not null, and rejects an `operating_tenant_id` change unless the new epoch is strictly greater.

An enabled `BEFORE UPDATE` trigger runs for an `UPDATE` by the table owner and for an `UPDATE` by a superuser. The table owner, or a role with the right to drop or disable the trigger, can remove that check. A role that can set `session_replication_role = replica` can skip a normal trigger. The trigger is not a DML authority boundary. Role membership, grants, and those bypasses are checked separately. `J-DML-001` stays open until a non-owner, non-superuser session receives `42501` for a direct link `UPDATE` and for an epoch `UPDATE`.

## Lock-order graph

The reader order already probed is:

`execution FOR SHARE -> shipment FOR SHARE -> active link FOR SHARE -> driver FOR SHARE -> vehicle FOR SHARE`

The foreign-key edge on `transport_execution_active_shipments.execution_id` is part of the graph. Writers take every existing execution they will key-share before they take the shipment.

Every consume transaction locks its ticket `FOR UPDATE` before any execution, shipment, or link lock. Issue and revocation lock only the ticket. They do not lock those business rows, so they do not take the opposite order.

| Path | Order | Edge after the shipment lock |
| --- | --- | --- |
| Create | Committed ticket, then shipment, cargo, driver, and vehicle, then the new execution and its link. | `FOR KEY SHARE` on the new execution only. |
| Retarget | Ticket, then source and destination `FOR UPDATE` in uuid order, then shipment, then link. | Foreign-key `FOR KEY SHARE` on the destination already held. |
| Assign with a link | Ticket, then that execution `FOR UPDATE`, then shipment, then link, then the shipment column update. | None to an execution. |
| Assign without a link | Ticket, then shipment only. A link that appears under that lock aborts the transaction. | None. The execution is not locked in this transaction. |
| Successor, return, redirect | Unchanged execution-first order. These statements do not consume a binding ticket. | None to a second execution or to the active link. |
| Direct link `UPDATE` | Removed from `shipment_runtime` by the proposed revoke. | The revoke is not executed here. |
| Pin propagation | Called inside the successor transaction that already holds the execution. | See the pin section. Not declared compatible. |

Forbidden: locking an existing execution after the shipment is already held, and locking the destination for the first time in the foreign-key check. Activation must not call retarget after its shipment lock. Retarget starts at the ticket and then the execution locks.

`DEADLOCK_FREE` is not claimed. The required isolated test, not run by this change, is `retarget A→B` against an execution-first command on B. Both start orders are required. The record is lock waits, SQLSTATE, which transaction rolls back, and the final link, epochs, and command result. Same-source, missing-destination, competing-retarget, and source-changed-before-lock are part of that same future suite. `RETARGET_DEADLOCK_TEST_REQUIRED=YES`.

## Pin propagation

`transport.propagate_execution_relationship_pins(uuid)` is not changed.

The successor calls it at `transport_execution_successor_repository.go:169`, after the new revision insert (`:158`) and before the `current_revision_id` update (`:172`). The caller has held the execution `FOR UPDATE` since `:85-89`. The Go wrapper skips the call only when the function is absent (`transport_execution_activation.go:367-368`). On a database migrated through `000103`, the function is present.

The function (`000103:917-1014`) reads the successor revision, requires that revision to have been inserted in the current transaction, reads the superseded revision and its pin rows, reads the execution's carrier company, updates `pin_set_fingerprint` on the successor revision, and inserts one pin row per source pin. It has no `FOR UPDATE`. The insert fires these non-deferrable foreign keys:

- `execution_revision_id` to `transport.transport_execution_revisions.id` (`:130`)
- `relationship_version_id` to `company.carrier_relationship_versions` (`:137`)
- `fk_pin_subject_locked_version` (`:148-150`)
- `fk_pin_subject_identity` (`:151-171`)

The `BEFORE INSERT` trigger `transport.enforce_activation_pin_binding` (`:267-323`) reads the revision, the relationship version, the relationship root, and existing pins. Those reads are not row locks in the function text. The foreign keys take `FOR KEY SHARE` on the referenced rows. The edges that can appear while the execution is already held are key-share on the relationship version and on the activation subject. Whether another transaction locks those rows and then waits for the execution is not measured.

`PIN_PROPAGATION_COMPATIBILITY=NOT_PROVEN`. The future isolated test runs this call against a revision update, a relationship-version lock, an activation-subject lock, an execution-first command, and a retarget. `PIN_PROPAGATION_TEST_REQUIRED=YES`.

## Proofs this document does not run

| Proof | Required result | State |
| --- | --- | --- |
| Issue commits, then a later transaction consumes | One `ISSUED` row becomes `CONSUMED` with the write and the audit row | not run |
| Two consumers, one ticket | One commit, the other denied after waiting on the ticket lock | not run |
| Expired ticket and revoked ticket | Both denied, no write | not run |
| Ticket for another tenant or principal | Denied. A shared caller role that only knows `ticket_id` is `NOT_PROVEN` | not run |
| Runtime `INSERT` with `owner_epoch = 5` | `42501` after table-level `INSERT` is revoked | not run |
| Direct active-link `INSERT` and `UPDATE` | `42501` for the non-owner, non-superuser runtime | not run |
| Retarget A→B against an execution-first command on B | Both start orders, with waits, SQLSTATE, rollback, and final rows | not run |
| Pin propagation | Against a revision, a relationship version, an activation subject, a command, and a retarget | not run |

## What this contract does not prove

Tenant isolation is not implemented. Presenter identity, decision consumption, broker delivery, and a non-superuser grant test are not part of this document. `ALL_WRITERS_FENCED` stays `NO`. Direct DML bypass stays open.

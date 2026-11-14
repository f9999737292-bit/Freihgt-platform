# 0.1O mandatory writer fencing contract

Status: `B3_REDESIGNED_PROPOSED_NOT_APPROVED`. This is not an implementation and it does not close `J-DML-001`.

Base: `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6`.

The isolated proofs 0.1I through 0.1N show lock behavior of probe functions. They do not revoke production grants. A later direct `UPDATE` still succeeds after the reader commits.

`TRUSTED_TENANT_AUTHORITY=DESIGN_BLOCKED_BY_SHARED_LOGIN`. The lock order and the function shape below are the required design. They are not a claim that the current database login can enforce them.

## Authority is not one owner

| Change | Owner of the operation today | Required authority |
| --- | --- | --- |
| Create a new execution and its first active-link row | `Project` inserts both in one transaction (`transport_execution_repository.go:31-126`, link insert `:335-340`) | One `SECURITY DEFINER` function in that same transaction. `shipment_runtime` loses direct `INSERT` on the active-link table. |
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

The required database mechanism is a ticket row written by a third designed role, `binding_authority_issuer`:

1. `binding_authority_issuer` is `NOLOGIN`, not superuser, and is not `binding_authority_owner`, `binding_authority_caller`, `shipment_runtime`, or `freight`.
2. Only that role can insert a ticket. `EXECUTE` on the issuer function is not granted to the runtime caller.
3. The ticket names one operation (`CREATE_ACTIVE_LINK`, `RETARGET`, `ASSIGN_DRIVER`, `ASSIGN_VEHICLE`, or `ESTABLISH_EPOCH`), the execution id or ids, the shipment id, the tenant, the principal, an expiry, and a nonce.
4. The fencing function locks the ticket `FOR UPDATE`. It accepts the ticket only when the nonce matches, the operation and the locked ids match, and `clock_timestamp()` is before the expiry. A tenant argument that disagrees with the ticket fails. A tenant argument that agrees is still not sufficient without the ticket.
5. The function sets the ticket's consumed transaction id to `pg_current_xact_id()` before the data write. A second use in that transaction fails. A ticket created by another transaction fails, because the design requires the ticket insert and the write to share one transaction id. Rollback removes the ticket and the write together.
6. The runtime cannot mint the ticket. A session variable cannot stand in for the row.

That mechanism is not available in this checkout. `000103:6-8` keeps `freight` as the shared login. `000105:5` does not alter that login. `000105:8-9` already leaves an award-link lock function uncreated until a server-owned caller identity exists. The integration test that grants `shipment_runtime` to `freight` is not a migration. A session that is `freight` can hold superuser, own the tables, `SET ROLE`, and call any function granted to its members. A non-superuser `freight` that can both issue a ticket and perform the write is still one principal on both sides of the boundary.

`transport.assert_activation_enforcement_role_split` (`000103:1088-1135`) rejects a superuser, bypassrls, or pin-table owner named `freight`. Its owner check does not name `transport.transport_execution_active_shipments`. Passing that function would not prove this boundary.

No later configuration step is declared to have closed the boundary. `TRUSTED_TENANT_AUTHORITY=DESIGN_BLOCKED_BY_SHARED_LOGIN`.

## Initial active-link insert

`shipment_runtime` has `INSERT` on `transport.transport_execution_active_shipments` (`000104:18`). The foreign key at `000088:79` checks that `execution_id` exists. It does not check that this transaction created that execution, and it does not check the shipment tenant. Direct `INSERT` can therefore name any existing execution.

The grant cannot be narrowed to "the execution inserted by this statement." A column grant does not carry that predicate. Keeping `INSERT` on `shipment_runtime` is not part of this redesign.

The required create function, in the same `Project` transaction:

1. Requires a `CREATE_ACTIVE_LINK` ticket for the shipment and the new execution id.
2. Requires the execution row's `xmin` to be the current transaction id, using the same xid comparison as `transport.revision_inserted_in_current_xact` (`000103:332-339`). An execution committed earlier is rejected.
3. Locks the shipment `FOR UPDATE` and reads its tenant and version under that lock.
4. Inserts one active-link row for that shipment and that execution.
5. Sets `owner_epoch` to `1` on the new execution row before commit.
6. Inserts the audit row.
7. Commits with the rest of `Project`, or rolls all of it back.

The primary key `(shipment_tenant_id, shipment_id)` rejects a second link. `Project` already maps that conflict to a rollback (`:100-102`). A replay of the same activation returns the stored execution (`:44-53`) and does not insert another link. Flag-on activation already locks the shipment, cargo, driver, and vehicle before the execution insert (`transport_execution_activation.go:67-71`). That path still must not lock a different existing execution after the shipment lock. The xmin check is what prevents the new link from naming one.

## Retarget lock order

`transport_execution_active_shipments.execution_id` references `transport.transport_executions.id` (`000088:79`). The constraint name is `transport_execution_active_shipments_execution_id_fkey`. It is `NO ACTION` and not deferrable. An `UPDATE` of `execution_id` takes `FOR KEY SHARE` on the destination execution. If that is the first lock on the destination, and the shipment is already held, the edge is `SHIPMENT → EXISTING EXECUTION`.

An execution-first command on the destination holds that execution `FOR UPDATE` and then waits for the shipment. Those two orders cycle. The previous revision of this contract locked only the source before the shipment and left the destination to the foreign-key check. That order is withdrawn.

The retarget transaction is:

1. Read the current link and the requested destination. This read is a candidate list. It is not authorization and it is not repeated as a fallback after a mismatch.
2. Confirm both ids currently exist. A missing destination fails this step and fails again if the later lock finds no row.
3. Lock the distinct execution ids `FOR UPDATE` in ascending uuid order. The source and the destination are both in that list. The destination lock is taken here, not by the later foreign-key check.
4. Lock the shipment `FOR UPDATE`.
5. Lock the active-link row `FOR UPDATE` by its primary key `(shipment_tenant_id, shipment_id)`.
6. Under those locks, read the link, both executions, the shipment, the current revision, and both epochs again. Require the ticket's tenant, principal, operation, execution ids, and shipment id to match these locked rows. Require each execution's `current_revision_id` to name an `ACTIVE` revision of that execution and operating tenant.
7. Apply the `execution_id` change, increment the source epoch and the destination epoch, and insert one audit row. Then commit. Any failed check rolls the transaction back. No earlier statement in the transaction has committed.

The foreign-key `FOR KEY SHARE` then falls on a destination row this transaction already holds `FOR UPDATE`. It does not open a new execution lock after the shipment.

### Same source and destination

The id list has one execution. The transaction locks that execution, then the shipment, then the link. The recheck requires the locked link to name that same execution. The foreign-key check key-shares that same row. There is no second execution.

### Missing destination

The pre-read rejects an unknown destination. If the row is deleted before the lock, `SELECT ... FOR UPDATE` finds no row and the transaction rolls back. The function does not insert a link and does not choose another execution.

### Competing retarget

Two transactions lock overlapping execution ids in the same uuid order, then the same shipment. One waits. The waiter rechecks after the lock. If the link no longer names the source it read, it rolls back. It does not continue with the old source.

### Source changed before the locks

The locked link is compared with the source id from the candidate read. A mismatch rolls back. The caller may open a new transaction and read the source again. The function does not apply the old source after that mismatch.

### Assignment

AssignDriver and AssignVehicle do not look the link up after locking the shipment and then lock the execution. The required sequence is:

1. Read the link without holding the shipment lock.
2. If the read finds no link, lock the shipment and read the link again. If a link now exists, roll back. Do not lock the execution in this transaction.
3. If the read finds a link, lock that execution first, then the shipment, then the link. Recheck the execution id, tenant, revision, and epoch. Then update the shipment column and increment the epoch.
4. A mismatch at step 3 rolls back. A new transaction may read the link again.

## Owner epoch

Proposed column: `transport.transport_executions.owner_epoch bigint NULL`. It is not in the schema. Existing rows stay `NULL`. `NULL` is not `1`. A `NULL` epoch rejects `RETARGET`, `ASSIGN_DRIVER`, `ASSIGN_VEHICLE`, and any other execution-targeted change.

| Rule | Required behavior |
| --- | --- |
| Initial value | The create function sets `1` on the execution inserted in that same transaction. That is the only insert path that creates an epoch. |
| Existing `NULL` row | `ESTABLISH_EPOCH` is a separate ticketed operation. Under the execution lock it changes `NULL` to `1` and writes an audit row. It does not accept a caller-supplied number. |
| Who increments | Only `binding_authority_owner`, inside the fencing function, from the locked value to that value plus one. |
| Link retarget | The source and the destination are both checked. The shipment leaves the source and arrives at the destination, so both epochs increment in the one transaction. |
| Assignment | Driver or vehicle change increments the one execution named by the locked link. The link row stays in place. |
| No link | The shipment update does not read or write an epoch. |
| Atomicity | The epoch write, the link or shipment write, and the audit insert commit together or roll back together. |
| Rollback | PostgreSQL restores the previous epochs, the previous link, the previous shipment columns, and removes the audit insert. |
| Stale epoch | The caller passes the epoch it observed. The function compares it with the locked column. A mismatch raises and rolls back. The new value is not committed. |
| Other `UPDATE` paths | A column grant on `current_revision_id`, `event_seq`, or `updated_at` does not include `owner_epoch`. A remaining table-level `UPDATE`, a table owner, or a superuser can still write the column. Those privileges are not epoch fencing. |

## Audit

Each accepted create, retarget, assignment bump, and epoch establishment inserts one row in the same transaction. The row records the operation, the execution ids, the shipment id, the previous and new epochs, the ticket nonce, and the principal stored on the ticket. The audit table is owned by `binding_authority_owner`. `shipment_runtime` is not granted `INSERT` or `DELETE` on it. A failed check does not leave the row behind, because the insert is in the transaction that rolls back.

## Trigger

A proposed `BEFORE UPDATE` trigger on `transport.transport_executions` rejects a lower `owner_epoch` when the old value is not null, and rejects an `operating_tenant_id` change unless the new epoch is strictly greater.

An enabled `BEFORE UPDATE` trigger runs for an `UPDATE` by the table owner and for an `UPDATE` by a superuser. The table owner, or a role with the right to drop or disable the trigger, can remove that check. A role that can set `session_replication_role = replica` can skip a normal trigger. The trigger is not a DML authority boundary. Role membership, grants, and those bypasses are checked separately. `J-DML-001` stays open until a non-owner, non-superuser session receives `42501` for a direct link `UPDATE` and for an epoch `UPDATE`.

## Lock-order graph

The reader order already probed is:

`execution FOR SHARE -> shipment FOR SHARE -> active link FOR SHARE -> driver FOR SHARE -> vehicle FOR SHARE`

The foreign-key edge on `transport_execution_active_shipments.execution_id` is part of the graph. Writers take every existing execution they will key-share before they take the shipment.

| Path | Order | Edge after the shipment lock |
| --- | --- | --- |
| Create, flag off | Insert a new execution, then the link function. The xmin check rejects an older execution. | `FOR KEY SHARE` on the new execution only. |
| Create, flag on | Shipment, cargo, driver, and vehicle are already locked. The new execution is inserted after that. The link function accepts only that new row. | No existing execution. |
| Retarget | Distinct source and destination `FOR UPDATE` in uuid order, then shipment, then link. | Foreign-key `FOR KEY SHARE` on the destination already held. |
| Assign with a link | That execution `FOR UPDATE`, then shipment, then link, then the shipment column update. | None to an execution. |
| Assign without a link | Shipment only. A link that appears under that lock aborts the transaction. | None. The execution is not locked in this transaction. |
| Successor, return, redirect | Unchanged execution-first order. | None to a second execution or to the active link. |
| Direct link `UPDATE` | Removed from `shipment_runtime` by the proposed revoke. | The revoke is not executed here. |
| Pin propagation | Called inside the successor transaction that already holds the execution. | See the pin section. Not declared compatible. |

Forbidden: locking an existing execution after the shipment is already held. Activation must not call retarget after its shipment lock. Retarget is a different operation and starts at the execution locks.

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

## What this contract does not prove

Tenant isolation is not implemented. Presenter identity, decision consumption, broker delivery, and a non-superuser grant test are not part of this document. `ALL_WRITERS_FENCED` stays `NO`. Direct DML bypass stays open.

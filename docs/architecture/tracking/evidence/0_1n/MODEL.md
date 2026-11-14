# 0.1N model

The reader is `transport.read_binding_line`. It locks one execution, its shipment, the active link, the driver, and the vehicle with `FOR SHARE`, in that order, then sleeps. That order is the probe. It is not a production guarantee.

Writers use separate sessions. `pg_stat_activity.wait_event_type = 'Lock'` is the wait signal. A granted row lock is read from the tuple. `pg_locks` does not list that tuple lock until another session is waiting.

| What is held | Where it shows |
| --- | --- |
| Table lock from `SELECT FOR SHARE`, `FOR KEY SHARE`, or `FOR UPDATE` | `pg_locks` `relation` `RowShareLock`, granted |
| `INSERT` / `UPDATE` / `DELETE` table lock | `pg_locks` `relation` `RowExclusiveLock`, granted |
| Granted `FOR KEY SHARE` on a committed row | `pgrowlocks` mode `For Key Share`. No `tuple` row in `pg_locks`. |
| Granted `FOR KEY SHARE` on a row inserted in the same open transaction | `pageinspect` bits `HEAP_XMAX_KEYSHR_LOCK` and `HEAP_XMAX_LOCK_ONLY`, without `HEAP_XMAX_EXCL_LOCK`. `pgrowlocks` returns `NONE` because the row is not visible. |
| Granted `FOR UPDATE` | `pgrowlocks` mode `For Update` |
| Granted `FOR NO KEY UPDATE` | `pgrowlocks` mode `No Key Update` |
| Session waiting for the lock holder | `pg_locks` `transactionid` `ShareLock` granted `false` |

On the sampled waits, a `FOR NO KEY UPDATE` request also showed `tuple` `ExclusiveLock` granted `true`, and a `FOR UPDATE` request showed `tuple` `AccessExclusiveLock` granted `true`. That granted tuple row is the heavyweight lock entry. The row lock is not acquired while the `transactionid` `ShareLock` is ungranted. `DF_R` waited on `transactionid` `ShareLock` granted `false` and the sample contained no `tuple` row. The statement completed after the reader committed.

## Tests

| Id | Statement | Go source | Reader first | Writer first | Result |
| --- | --- | --- | --- | --- | --- |
| RI | `INSERT` into `transport_execution_participants` | Foreign key `transport_execution_participants_execution_id_fkey` | `NO_WAIT`, commit | `For Key Share` on execution `1111`; reader finished during the sleep | Participant count 2 |
| FKU | `UPDATE shipments.driver_id` | `shipments_driver_id_fkey`. Product code does not run this statement in the return path. | Wait, then commit | Reader waits | Version 2, driver `7778` |
| PKU | `UPDATE transport_executions.id` | Product Go does not update this key. Children are `ON UPDATE NO ACTION`. | Wait, then `23503` | Same | Snapshot unchanged |
| DEL | `DELETE` the execution | Children are `ON DELETE NO ACTION` | Wait, then `23503` | Same | Row remains |
| EX | Insert a second execution and its revision | Revision foreign key `transport_execution_revisions_execution_fk` | `NO_WAIT` | Key-share bits on uncommitted execution `1212`; reader of `1111` finished | Execution count 2. Execution `1111` unchanged |
| AL | `INSERT` into `transport_execution_active_shipments` | `transport_execution_active_shipments_execution_id_fkey` | `NO_WAIT` | `For Key Share` on execution `1111` | Link for shipment `2223` points at `1111` |
| SU | Successor revision | `createSuccessorRevisionInTx` in `transport_execution_successor_repository.go:82`, `FOR UPDATE` at line 89 | Wait | Reader waits | Current revision `5556`, previous `SUPERSEDED` |
| DF trigger | Point `current_revision_id` at a missing revision | Trigger `transport_executions_current_revision_guard` | Single session | Single session | `23514`, snapshot unchanged |
| DF commit | Point at a revision and delete that revision before `COMMIT` | `transport_executions_current_revision_fk`, `DEFERRABLE INITIALLY DEFERRED` | Single session | Single session | `23503` at commit, full rollback |
| DF | Supersede the active revision and point at the new one | Same deferred constraint, after the trigger accepts the row | Wait | Reader waits. Held mode `No Key Update` | Current revision `5559` |
| RT | Authorize return | `AuthorizeReturn` at `transport_execution_disposition_repository.go:169` | Wait | `For Update` on execution and case. Shipment has no row lock | `IN_RETURN_TRANSIT`, stop count 3 |
| RD | Authorize redirect | `AuthorizeRedirect` at line 173 | Wait | Same lock prefix as return | `IN_REDIRECT_TRANSIT`, stop count 3 |

`FOR KEY SHARE` did not wait for the reader's `FOR SHARE`. `FOR NO KEY UPDATE` and `FOR UPDATE` did.

## Return and redirect

Both enter `authorizeRoute` at line 177. The transaction starts at `beginTx` (line 188). The lock order in the probe is the order in that function:

1. `lockExecution` (`:487`) takes `FOR UPDATE` on the execution.
2. `claimDispositionCommand` (`:504`) inserts `delivery_disposition_commands`. That table has no foreign key on `execution_id`.
3. `lockDispositionCase` (`:868`) takes `FOR UPDATE` on the case.
4. `validateDispositionTarget` (`:649`) selects `locations` and `shipments.origin_location_id` with no row lock.
5. `buildDispositionSuccessor` (`:689`) reads open stops and calls `matchFutureStops` (`:767`). Every open `PLANNED` or `ARRIVED` stop must be in the command. The probe supersedes stop `6666`, clones it, and inserts the target stop.
6. `createSuccessorRevisionInTx` locks the execution again, inserts `transport_execution_commands` (this insert does foreign-key check the execution), supersedes the revision, inserts the successor, updates `current_revision_id`, and rewrites stop and action membership.
7. The case update sets `IN_RETURN_TRANSIT` or `IN_REDIRECT_TRANSIT`.
8. The audit insert references the case.
9. `COMMIT` is line 274. `defer tx.Rollback` covers the error paths.

Return and redirect share that order. They differ in command name, disposition type, target location, reserved-quantity column, successor id, and the case status. Both sequences were executed. The shipment access stayed `AccessShareLock`.

`copySuccessorRelationshipPins` (`transport_execution_activation.go:362`) returns without writing when `transport.propagate_execution_relationship_pins(uuid)` is absent. The probe does not create that function, so the pin foreign keys from `000103` are not checked.

The probe omits `emitDisposition`, `storeDispositionResult`, `ON CONFLICT`, and the production columns that the tested statements do not need. Those omissions are listed in `FK_INVENTORY.md`.

## Deadlock

No tested stderr contains `40P01`. The result is `NO_DEADLOCK_IN_TESTED_FK_INTERLEAVINGS`. This is not a claim that every interleaving is deadlock-free.

## Not established

Tenant authorization, presenter identity, protected consumption, closure of direct DML, an owner epoch, and a Kafka consumer are outside this run. `J_DML_001` stays open: the reader does not stop a later direct update of the execution after it releases the locks.

# 0.1M writer model

The SQL in `writers/` reproduces only lock and write statements that the 0.1L census proved. Each session is one PostgreSQL transaction. A harness `pg_sleep` is not a product statement.

The reader is `transport.read_binding_line` in `setup.sql`. Its lock order is:

`EXECUTION FOR SHARE → SHIPMENT FOR SHARE → ACTIVE LINK FOR SHARE → DRIVER FOR SHARE → VEHICLE FOR SHARE`

That order is the probe contract. It is not a production guarantee. The function does not reject a null `owner_epoch`: product SQL on this base has no such column. The 0.1K stand-in writers are not reused.

Line numbers below were read from base `3f306fc407aeda4bdc652d825954292f4a54fad6`.

## Shared model differences

- Schemas `transport` and `company` contain only columns the modeled statements touch. There are no foreign keys. 0.1L left FK key-share waits unknown, and this proof does not invent them.
- No platform migration is installed. `shipment_runtime` is not created.
- `reader_probe` and `writer_probe` exist only in database `bintrans_01m_writer_compat`. PostgreSQL 16 refuses `SELECT FOR SHARE` without `UPDATE` privilege, so `reader_probe` is granted `UPDATE` on the five locked tables. Its scripts never issue `UPDATE`.
- History, outbox, audit, participant, stop, action, and authorization inserts are omitted except where a listed scenario includes one of those statements.
- Fixed UUIDs replace `uuid.New()` so a leaked insert stays visible after rollback.

## 1. Project, activation flag off

| Item | Value |
| --- | --- |
| Entry | `TransportExecutionRepository.Project` |
| Transaction | `transport_execution_repository.go:31` `BEGIN`, `:126` `COMMIT` |
| New execution | `uuid.New()` at `:80`, not a lock of an existing row |
| Statements | `verifyMaterializedRows` plain shipment `SELECT` `:261`, plain cargo `SELECT` `:276`, `insertExecution` `:295`, `insertRevision` `:304`, `UPDATE` of that new execution `:92`, active-link `INSERT` `:335` |
| Lock order | No `FOR UPDATE`. The conflicting statement is the link `INSERT`. |
| Files | `writers/project_off_occupied.sql`, `writers/project_off_free.sql`, `writers/project_off_free_hold.sql` |

Occupied target `2222…2222` already has a link, so the insert hits `transport_execution_active_shipments_pkey` (`23505`). Production maps that constraint to `ProjectionConflict` and the deferred `Rollback` discards the new execution. The probe uses `ON_ERROR_STOP`, which stops before `COMMIT`; session end rolls the transaction back. The free target `2222…2223` has no link, so the same shape commits a new execution id.

The flag-off writer does not lock the existing execution, shipment, driver, vehicle, or link. A duplicate insert against a committed link row returned `23505` without waiting for the reader's `FOR SHARE`.

## 2. Project, activation flag on

| Item | Value |
| --- | --- |
| Entry | `Project` when `relationshipActivation` is true, `:64` |
| Prefix | `authorizeRelationshipActivation` |
| Advisory lock | `000103_tms_relationship_pins_security_foundation_v0_1.up.sql:709` |
| Head | `SELECT … FROM company.carrier_relationship_heads … FOR UPDATE` at `:770` |
| Shipment and cargo | `recheckActivationSubjects`, statements at `transport_execution_activation.go:259` and `:279` |
| Driver and vehicle | `lockOperatingAssignment`, statements at `:301` and `:319` |
| Then | The same new-execution insert path as flag off: `:80` through the link `INSERT` at `:335` |
| Files | `writers/project_on_occupied.sql`, `writers/project_on_occupied_hold.sql`, `writers/project_on_free.sql`, `writers/project_on_free_hold.sql` |

`writers/project_on_common.sql` records that prefix and is not executed.

The model does not replace “insert a new execution” with a lock of execution `1111…1111`. While the hold scripts sleep, before that insert, the writer’s lock count on `transport.transport_executions` is 0. The static check rejects any `FOR UPDATE` of `transport_executions` in these files.

Differences: one subject instead of the authorizer loop; the driver and vehicle `SELECT` omits the production `tenant_id`, `carrier_company_id`, `status`, and `deleted_at` predicates. The fixture row is still the row those predicates would lock. The authorizer body, authorization insert, and pin functions are omitted.

The shared driver and the occupied shipment are rows the reader also locks. The writer waits there. It does not need the existing execution, so the wait is not a cycle with this reader.

## 3. AssignDriver

| Item | Value |
| --- | --- |
| Entry | `ShipmentRepository.AssignDriver` `shipment_repository.go:423` |
| Transaction | `BEGIN` `:426`, `COMMIT` `:454` |
| SQL | `UPDATE transport.shipments` `:433` |
| Lock order | That single shipment update. No execution, link, driver, or vehicle lock. |
| Files | `writers/assign_driver.sql`, `writers/assign_driver_hold.sql` |

Status history and outbox are omitted. The reader’s shipment `FOR SHARE` blocks this update. An execution-only lock would not.

## 4. AssignVehicle

| Item | Value |
| --- | --- |
| Entry | `ShipmentRepository.AssignVehicle` `shipment_repository.go:463` |
| Transaction | `BEGIN` `:466`, `COMMIT` `:494` |
| SQL | `UPDATE transport.shipments` `:473` |
| Lock order | That single shipment update. |
| Files | `writers/assign_vehicle.sql`, `writers/assign_vehicle_hold.sql` |

Same omissions and same shipment-row wait as AssignDriver.

## 5. Driver-stop command

| Item | Value |
| --- | --- |
| Entry | `TransportExecutionCommandRepository.Execute` `transport_execution_command_repository.go:50` |
| Transaction | `BEGIN` `:62`, `COMMIT` `:153` |
| Lock order | Execution `FOR UPDATE` `:70`, stop `FOR UPDATE OF stop` `:1121`, shipment `FOR UPDATE` `:1147`, shipment `UPDATE` `:1177`, execution `event_seq` `UPDATE` `:1225` |
| Link | `departedPickup` plain `SELECT` `:660`, no `FOR UPDATE` |
| Files | `writers/driver_stop.sql`, `writers/driver_stop_hold.sql` |

The stop statement is a single-row stand-in for `openPredecessors`. Command insert, audit, and outbox are omitted. During the hold, the active-link relation lock is `AccessShareLock` and the tuple-lock count on that table is 0.

This writer starts with the same execution row the reader locks first, then the shipment. That is the reader’s prefix, so the tested orders wait instead of cycling.

## 6. Successor revision

| Item | Value |
| --- | --- |
| Entry | `CreateSuccessorRevision` `transport_execution_successor_repository.go:48` |
| Transaction | `BEGIN` `:64`, `COMMIT` `:73` |
| Lock order | Execution `FOR UPDATE` statement `:85` (`FOR UPDATE` at `:89`), revision `UPDATE` `:151`, successor `INSERT` `:158`, execution `current_revision_id` `UPDATE` `:172` |
| Files | `writers/successor.sql`, `writers/successor_hold.sql` |

No shipment, link, driver, or vehicle lock. Command insert, stop membership, and pin propagation are omitted. The only shared lock with the reader is the execution row.

## 7. Shipment modification

| Item | Value |
| --- | --- |
| Entry | `ShipmentRepository.updateStatus` `shipment_repository.go:511` |
| Transaction | `BEGIN` `:514`, `COMMIT` `:552` |
| SQL | `UPDATE transport.shipments` `:521` |
| Files | `writers/shipment_status.sql`, `writers/shipment_status_hold.sql` |

`Accept` `:561` and `Cancel` `:604` use this same single-shipment update shape. They are not separate sessions. History and cargo evidence are omitted. The wait is the shipment row, as with AssignDriver.

## 8. Active-link direct UPDATE

| Item | Value |
| --- | --- |
| Product statement | None. No proven Go writer issues this `UPDATE`. |
| Privilege | `GRANT SELECT, INSERT, UPDATE ON transport.transport_execution_active_shipments` at `000104_tms_activation_runtime_grants_v0_1.up.sql:18` |
| Lock order | The `UPDATE` alone. No execution lock. |
| Files | `writers/direct_link.sql`, `writers/direct_link_hold.sql`, `writers/direct_link_timeout.sql` |

This session is a DML bypass control. It does not close `J-DML-001`.

## Not modeled as sessions

These 0.1L paths were not given their own interleavings:

- Award-order execute.
- Record disposition.
- Authorize return and authorize redirect.
- Tracking consumer.
- `Accept` and `Cancel` as separate sessions. Their shipment update shape is the shipment-modification scenario.
- Foreign-key key-share waits. The model has no foreign keys.

# 0.1L production writer inventory

Audit base: `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6`.

Reference only: draft PR #381 at `ea240554f5e62f8d726e29a5ea20c1f449616929`. The 0.1K lock order is a test hypothesis. It is not a production contract. This audit does not change product code.

`owner_epoch` does not occur in product Go or SQL on this SHA.

## Census

These fifteen paths do not share one lock order.

- Fourteen have a proven statement sequence inside one transaction.
- One path is `UNKNOWN`: `shipment_runtime` may `UPDATE` the active link, and no product statement shows the order.
- Proven product deadlocks: none.
- Potential cycle: one, and only if a reader locks the active link before the shipment.
- `shipment_runtime` has `UPDATE` on the active-link table. That grant is open. It is not a prohibition.
- `TMS_RELATIONSHIP_ACTIVATION_ENABLED` defaults to false. The shipment-first locks run only when it is true.
- The active-link table exists. The only observed product write is `INSERT`. No product `UPDATE` or `DELETE` of that table was found.

## Active link

`transport.transport_execution_active_shipments` exists in the platform migration `infrastructure/migrations/000088_tms_transport_execution_foundation_v0_1a.up.sql:76-81`. Its primary key is `(shipment_tenant_id, shipment_id)`. `execution_id` references `transport.transport_executions(id)`.

The only product writer found is `INSERT` in `services/shipment-service/internal/repository/transport_execution_repository.go:335-340`, called from `Project` at line 99. No product `UPDATE` or `DELETE` of that table was found. `shipment_runtime` still has `SELECT, INSERT, UPDATE` on it (`infrastructure/migrations/000104_tms_activation_runtime_grants_v0_1.up.sql:18`). A direct `UPDATE` of `execution_id` is therefore a privilege path, not a Go method.

Two product reads are not locks: `transport_execution_command_repository.go:660-664` and `driver_stop_task_repository.go:365`.

## Writers

Tenant values below come from gateway-verified request context or from the row locked inside the transaction. This audit does not re-prove tenant isolation.

| Service | Entry point | Repository | File:line | Transaction | First lock | Next locks | SQL | Lock mode | Tenant source | Commit | Compatibility |
|---|---|---|---|---|---|---|---|---|---|---|---|
| shipment-service | `ExecutionProjectionHandler.Create` `execution_projection_handler.go:46` | `AcceptRoutePlanActivation` then `Project` | `transport_execution_repository.go:142`, `26-137` | `Begin` line 31, `Commit` line 126 | none on an existing execution | `INSERT` new execution line 296, `UPDATE` that new row line 93, `INSERT` participants line 321, `INSERT` active link line 336 | INSERT, UPDATE, INSERT | no `FOR UPDATE` of shipment, driver, vehicle, or an existing execution | `OperatingTenantID` must equal verified tenant, handler lines 71-75 | same transaction | YES against the proposed reader. This path does not lock the existing execution. |
| shipment-service | same handler, only when `TMS_RELATIONSHIP_ACTIVATION_ENABLED=true` | `authorizeRelationshipActivation` | `transport_execution_activation.go:27-73`; default is false at `config.go:101` | same `Project` transaction, before the inserts | advisory relationship locks, then `FOR UPDATE` of `company.carrier_relationship_heads` | `FOR UPDATE` shipment line 263, cargo line 283, driver line 308, vehicle line 326, then the inserts above | SELECT FOR UPDATE, then INSERT | `FOR UPDATE` is written on those lines | same verified operating tenant | same transaction | YES against the proposed execution-then-shipment reader, because this path never locks the existing execution row. NO if a reader locks the active link before the shipment. |
| shipment-service | `ShipmentHandler.AssignDriver` `shipment_handler.go:346` | `ShipmentService.AssignDriver` `shipment_service.go:246`; `ShipmentRepository.AssignDriver` | `shipment_repository.go:423-457` | `Begin` line 426, `Commit` line 454. The earlier `GetByIDAndTenant` is not this transaction | `UPDATE transport.shipments` line 433 | no execution, active link, driver, or vehicle `FOR UPDATE` | UPDATE | the statement has no `FOR UPDATE`. Row-lock mode was not executed in this audit | `resolveVerifiedTenant` handler line 347, and `tenant_id` in the `WHERE` | same transaction | YES for order. The shipment changes without an execution lock, so it is not fenced. |
| shipment-service | `ShipmentHandler.AssignVehicle` `shipment_handler.go:380` | `ShipmentRepository.AssignVehicle` | `shipment_repository.go:463` | `Begin` line 466, `Commit` after the update | `UPDATE transport.shipments` line 473 | no execution or active-link lock | UPDATE | no `FOR UPDATE` clause | verified tenant in the `WHERE` | same transaction | YES for order. Not fenced. |
| shipment-service | status update handlers | `updateStatus` | `shipment_repository.go:511-521` | `Begin` line 514 | `UPDATE transport.shipments` line 521 | none of the other five rows | UPDATE | no `FOR UPDATE` clause | verified tenant in the `WHERE` | same transaction | YES for order. Not fenced. |
| shipment-service | accept handler | `Accept` | `shipment_repository.go:561-571` | `Begin` line 564 | `UPDATE transport.shipments` line 571 | none | UPDATE | no `FOR UPDATE` clause | verified tenant in the `WHERE` | same transaction | YES for order. Not fenced. |
| shipment-service | cancel handler | `Cancel` | `shipment_repository.go:604` | `Begin` line 607 | `UPDATE transport.shipments` | none | UPDATE | no `FOR UPDATE` clause | verified tenant in the `WHERE` | same transaction | YES for order. Not fenced. |
| shipment-service | `OrderExecutionHandler.Execute` `order_execution_handler.go:32` | `ExecuteAwardOrder` | `order_execution_repository.go:105-122` | `Begin` line 108 | `FOR UPDATE` of `rfx.rfx_award_transport_orders` line 425 | `FOR UPDATE` of `transport.transport_orders` line 449, then `INSERT` shipment | SELECT FOR UPDATE, INSERT | `FOR UPDATE` as written | `params.TenantID` | same transaction | YES. This creates a shipment. It does not lock an execution or the active link. |
| shipment-service | `DriverStopHandler` arrive, start, complete, confirm, fail `driver_stop_handler.go:48-72` | `DriverStopService.dispatch` `driver_stop_service.go:165`; `TransportExecutionCommandRepository.Execute` | `transport_execution_command_repository.go:50-156` | `Begin` line 62, `Commit` line 153. `LoadDriverStop` at service line 170 is an earlier query | `FOR UPDATE` of `transport.transport_executions` line 74 | stop `FOR UPDATE OF stop` lines 853 and 1121, then shipment `FOR UPDATE` line 1147, then `UPDATE` shipment line 1177 and `UPDATE` execution `event_seq` line 1225. Active link is a plain `SELECT` at line 661 | SELECT FOR UPDATE, UPDATE | `FOR UPDATE` as written. The link read is not a lock | operating tenant is read from the locked execution and compared with the command at line 160 | same transaction | YES. Execution then shipment matches the proposed prefix. The link is not locked. |
| shipment-service | `SuccessorRevisionHandler.Create` `successor_revision_handler.go:67` | `CreateSuccessorRevision` | `transport_execution_successor_repository.go:48-90` | `Begin` line 64, `Commit` line 73 | `FOR UPDATE` of `transport.transport_executions` line 89 | `UPDATE` current revision line 152, `INSERT` successor revision line 159, `UPDATE` execution `current_revision_id` line 173, `UPDATE` `event_seq` line 458 | SELECT FOR UPDATE, UPDATE, INSERT | `FOR UPDATE` as written | operating tenant from the locked execution, checked at line 97 | same transaction | YES. Execution only. No shipment, link, driver, or vehicle lock. |
| shipment-service | `DriverStopHandler.ReportDisposition` `driver_stop_handler.go:92` | `RecordDeliveryDisposition` | `transport_execution_disposition_repository.go:46-61` | `beginTx` line 55, `Commit` line 163 | `lockExecution` `FOR UPDATE` line 493 | disposition case update. No shipment or active-link `FOR UPDATE` was found in this method | SELECT FOR UPDATE, UPDATE | `FOR UPDATE` as written | operating tenant from the locked execution, checked at line 65 | same transaction | YES. Execution first. |
| shipment-service | return and redirect authorization | `AuthorizeReturn` line 169 and `AuthorizeRedirect` line 173, both through `authorizeRoute` | `transport_execution_disposition_repository.go:193`, successor call line 232 | `beginTx` line 188, commit after the successor | `lockExecution` line 193, `FOR UPDATE` line 493 | `FOR UPDATE` disposition case line 873, then `createSuccessorRevisionInTx` which locks the same execution again at successor line 89 | SELECT FOR UPDATE, UPDATE, INSERT | `FOR UPDATE` as written | operating tenant from the locked execution | same transaction | YES. Execution, then the disposition case, then the revision write. |
| tracking-service | `ExecutionConsumer.Apply` `execution_consumer.go:71` | `LockExecutionTrackingState` | `execution_tracking_repository.go:86-93` | `Begin` line 89 | `INSERT` inbox line 74, then `FOR UPDATE` of `tracking.execution_tracking_state` | upsert of that tracking row. No `transport.transport_executions` lock | INSERT, SELECT FOR UPDATE | `FOR UPDATE` of the tracking table only | `operating_tenant_id` from the consumed event | tracking transaction only | YES against the five transport rows. This is not the same transaction as a shipment writer. |
| shipment-service role | none | none | `infrastructure/migrations/000104_tms_activation_runtime_grants_v0_1.up.sql:18` | none | unknown | unknown | privilege allows `UPDATE` | unknown until a statement is issued | role, not a request tenant | caller-defined | UNKNOWN |

Known execution writers run as `shipment_runtime` under `infrastructure/migrations/000103_tms_relationship_pins_security_foundation_v0_1.up.sql:1225` (`GRANT SELECT, INSERT, UPDATE ON transport.transport_executions`).

`transport.shipments.driver_id` and `vehicle_id` reference `transport.drivers` and `transport.vehicles` (`000003_create_transport_tables.up.sql:168-169`). AssignDriver and AssignVehicle do not contain `FOR UPDATE` of those tables. This audit did not execute the statements, so a foreign-key key-share wait is not claimed as an observed lock.

The revision trigger `transport.reject_transport_execution_revision_mismatch` (`000088` lines 186-212) reads the revision named by `current_revision_id`. It contains no `FOR UPDATE`. A hidden row lock from that read was not executed here and stays `UNKNOWN`.

No product method updates `operating_tenant_id`. The trigger watches that column, but the observed execution updates change `current_revision_id`, `event_seq`, and `updated_at`.

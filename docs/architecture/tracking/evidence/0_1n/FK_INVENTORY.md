# Foreign keys that touch the binding tables

Source is `origin/main` at `3f306fc407aeda4bdc652d825954292f4a54fad6`. An inline `REFERENCES` with no `ON UPDATE`, `ON DELETE`, or `DEFERRABLE` clause is `NO ACTION` and not deferrable. PostgreSQL names that constraint `{table}_{column}_fkey`.

`MODELED` means the probe database creates that constraint. `NOT_MODELED` means it is present in main and is not installed here. `NOT_PRESENT_IN_MAIN` means the column or edge has no foreign key.

## Binding tables

| Constraint | From | To | Actions | Deferrable | Source | FK check caused by | Probe |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `shipments_driver_id_fkey` | `transport.shipments.driver_id` | `transport.drivers.id` | NO ACTION / NO ACTION | no | `000003_create_transport_tables.up.sql:168` | INSERT or UPDATE of `driver_id`; DELETE or key UPDATE of `drivers.id` | MODELED |
| `shipments_vehicle_id_fkey` | `transport.shipments.vehicle_id` | `transport.vehicles.id` | NO ACTION / NO ACTION | no | `000003:169` | INSERT or UPDATE of `vehicle_id`; DELETE or key UPDATE of `vehicles.id` | MODELED |
| `shipments_origin_location_id_fkey` | `transport.shipments.origin_location_id` | `transport.locations.id` | NO ACTION / NO ACTION | no | `000003:170` | INSERT or UPDATE of `origin_location_id` | MODELED |
| `shipments_destination_location_id_fkey` | `transport.shipments.destination_location_id` | `transport.locations.id` | NO ACTION / NO ACTION | no | `000003:171` | INSERT or UPDATE of `destination_location_id` | NOT_MODELED. The tested statements do not write this column. |
| `shipments_transport_order_id_fkey` | `transport.shipments.transport_order_id` | `transport.transport_orders.id` | NO ACTION / NO ACTION | no | `000003:163` | INSERT or UPDATE of `transport_order_id` | NOT_MODELED. Column omitted. |
| `shipments_cargo_id_fkey` | `transport.shipments.cargo_id` | `transport.cargoes.id` | NO ACTION / NO ACTION | no | `000003:172` | INSERT or UPDATE of `cargo_id` | MODELED |
| `transport_execution_revisions_execution_fk` | `transport_execution_revisions (execution_id, operating_tenant_id)` | `transport_executions (id, operating_tenant_id)` | NO ACTION / NO ACTION | no | `000088_tms_transport_execution_foundation_v0_1a.up.sql:38` | INSERT or UPDATE of that pair; DELETE or key UPDATE of the execution | MODELED |
| `transport_execution_revisions_supersedes_fk` | `transport_execution_revisions.supersedes_revision_id` | `transport_execution_revisions.id` | NO ACTION / NO ACTION | no | `000088:41` | INSERT or UPDATE of `supersedes_revision_id` | MODELED |
| `transport_executions_current_revision_fk` | `transport_executions.current_revision_id` | `transport_execution_revisions.id` | NO ACTION / NO ACTION | DEFERRABLE INITIALLY DEFERRED | `000088:52` | INSERT or UPDATE of `current_revision_id`, checked at COMMIT | MODELED |
| `transport_execution_participants_execution_id_fkey` | `transport_execution_participants.execution_id` | `transport_executions.id` | NO ACTION / NO ACTION | no | `000088:59` | INSERT or UPDATE of `execution_id` | MODELED |
| `transport_execution_active_shipments_execution_id_fkey` | `transport_execution_active_shipments.execution_id` | `transport_executions.id` | NO ACTION / NO ACTION | no | `000088:79` | INSERT or UPDATE of `execution_id` | MODELED |
| `transport_execution_stops_execution_fk` | `transport_execution_stops (execution_id, operating_tenant_id)` | `transport_executions (id, operating_tenant_id)` | NO ACTION / NO ACTION | no | `000088:106` | INSERT or UPDATE of that pair | MODELED |
| `transport_execution_revision_stops_revision_id_fkey` | `transport_execution_revision_stops.revision_id` | `transport_execution_revisions.id` | NO ACTION / NO ACTION | no | `000088:120` | INSERT or UPDATE of `revision_id` | MODELED |
| `transport_execution_revision_stops_stop_id_fkey` | `transport_execution_revision_stops.stop_id` | `transport_execution_stops.id` | NO ACTION / NO ACTION | no | `000088:121` | INSERT or UPDATE of `stop_id` | MODELED |
| `transport_execution_actions_execution_stop_id_fkey` | `transport_execution_actions.execution_stop_id` | `transport_execution_stops.id` | NO ACTION / NO ACTION | no | `000088:135` | INSERT or UPDATE of `execution_stop_id` | MODELED |
| `transport_execution_revision_actions_revision_id_fkey` | `transport_execution_revision_actions.revision_id` | `transport_execution_revisions.id` | NO ACTION / NO ACTION | no | `000088:155` | INSERT or UPDATE of `revision_id` | MODELED |
| `transport_execution_revision_actions_action_id_fkey` | `transport_execution_revision_actions.action_id` | `transport_execution_actions.id` | NO ACTION / NO ACTION | no | `000088:156` | INSERT or UPDATE of `action_id` | MODELED |
| `transport_execution_commands_execution_id_fkey` | `transport_execution_commands.execution_id` | `transport_executions.id` | NO ACTION / NO ACTION | no | `000089_tms_transport_execution_commands_v0_1b.up.sql:12` | INSERT or UPDATE of `execution_id` | MODELED |
| `transport_execution_command_audit_command_id_fkey` | `transport_execution_command_audit.command_id` | `transport_execution_commands.id` | NO ACTION / NO ACTION | no | `000089:26` | INSERT of audit | NOT_MODELED. Authorize return does not insert this audit row. |
| `delivery_disposition_cases_execution_id_fkey` | `delivery_disposition_cases.execution_id` | `transport_executions.id` | NO ACTION / NO ACTION | no | `000093_tms_delivery_disposition_v0_1.up.sql:8` | INSERT or UPDATE of `execution_id` | MODELED |
| `delivery_disposition_cases_source_execution_stop_id_fkey` | `delivery_disposition_cases.source_execution_stop_id` | `transport_execution_stops.id` | NO ACTION / NO ACTION | no | `000093:10` | INSERT or UPDATE of that column | MODELED |
| `delivery_disposition_cases_source_action_id_fkey` | `delivery_disposition_cases.source_action_id` | `transport_execution_actions.id` | NO ACTION / NO ACTION | no | `000093:11` | INSERT or UPDATE of that column | MODELED |
| `delivery_disposition_cases_cargo_id_fkey` | `delivery_disposition_cases.cargo_id` | `transport.cargoes.id` | NO ACTION / NO ACTION | no | `000093:14` | INSERT or UPDATE of `cargo_id` | MODELED |
| `delivery_attempt_facts_execution_id_fkey` | `delivery_attempt_facts.execution_id` | `transport_executions.id` | NO ACTION / NO ACTION | no | `000093:78` | INSERT of an attempt fact | NOT_MODELED. Authorize return does not insert one. |
| `delivery_disposition_audit_case_id_fkey` | `delivery_disposition_audit.case_id` | `delivery_disposition_cases.id` | NO ACTION / NO ACTION | no | `000093:119` | INSERT of audit | MODELED |
| `delivery_disposition_evidence_case_id_fkey` | `delivery_disposition_evidence.case_id` | `delivery_disposition_cases.id` | NO ACTION / NO ACTION | no | `000093:137` | INSERT of evidence | NOT_MODELED. Authorize return does not insert evidence. |

## Edges that are not foreign keys

| Edge | Result |
| --- | --- |
| `transport_executions.driver_id` | `NOT_PRESENT_IN_MAIN`. The column exists at `000088:15` and has no `REFERENCES`. |
| `transport_executions.vehicle_id` | `NOT_PRESENT_IN_MAIN`. Column at `000088:14`. |
| `transport_execution_active_shipments.shipment_id` | `NOT_PRESENT_IN_MAIN`. Primary-key column, no foreign key to `shipments`. |
| `transport_execution_participants.shipment_id` and `.cargo_id` | `NOT_PRESENT_IN_MAIN`. |
| `delivery_disposition_cases.source_revision_id` | `NOT_PRESENT_IN_MAIN`. |
| `delivery_disposition_cases.successor_revision_id` | `NOT_PRESENT_IN_MAIN`. |
| `delivery_disposition_commands.execution_id` | `NOT_PRESENT_IN_MAIN`. The authorize command insert does not take a foreign-key lock. |
| `delivery_disposition_cases.return_target_location_id` and `.redirect_target_location_id` | `NOT_PRESENT_IN_MAIN`. |

## Present in main, not installed in the probe

These reference the execution or a revision, and none of the tested statements write them:

- `transport.driver_stop_tasks` at `000090_tms_driver_stop_tasks_v0_1c.up.sql:8` and `:9`.
- `transport.transport_execution_pod_requirement_decisions` at `000098_tms_pod_requirement_decision_foundation_v0_4d.up.sql:29` through `:48`.
- `transport.transport_execution_carrier_relationship_pins` at `000103_tms_relationship_pins_security_foundation_v0_1.up.sql:130`, `:137`, `:148`, and `:151`. `copySuccessorRelationshipPins` returns without writing when `transport.propagate_execution_relationship_pins(uuid)` is absent. The probe does not create that function, so the successor path does not check these constraints.

Company relationship tables in `000100`, `000101`, `000102`, and `000103` are not parents or children of the reader rows except through the pin tables above.

No other modeled foreign key is deferrable. The revision-mismatch trigger at `000088:186` fires before the deferred current-revision check and rejects a missing or mismatched revision immediately with `23514`.

## Probe DDL that is not production DDL

- `pgrowlocks` and `pageinspect` are installed only in the disposable database. They are not BINTRANS objects. `pgrowlocks` reads a granted row-lock mode on a committed tuple, because `pg_locks` lists a tuple lock only after another session is waiting. `pageinspect` reads the key-share bits on an execution row inserted by the same open transaction, which `pgrowlocks` cannot see.
- `reader_probe` is granted `UPDATE` on the five tables it locks with `FOR SHARE`. PostgreSQL 16 refuses `FOR SHARE` without that privilege. The reader scripts do not update those tables.
- Columns and constraints marked `NOT_MODELED` or `NOT_PRESENT_IN_MAIN` above are omitted. No extra foreign key is added.

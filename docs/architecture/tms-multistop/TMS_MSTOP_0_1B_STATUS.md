# TMS-MSTOP-0.1B status

Stop and action commands only. This note records the 0.1B command foundation. It does not change the frozen architecture from PR #186.

```text
EXECUTION_ROUTE_MODEL=TRANSPORT_EXECUTION
COMMANDS_IN_PROCESS_ONLY=YES
NEW_DRIVER_HTTP_API=NO
NEW_SHIPMENT_STATES_ADDED=NO
EN_ROUTE_ADDED=NO
DEPARTED_PICKUP_TO_IN_TRANSIT=REUSE
CARGO_EVIDENCE_LEDGER=transport.shipment_cargo_execution_evidence
SECOND_CARGO_EVIDENCE_LEDGER_CREATED=NO
EXECUTION_EVENT_VERSION_SOURCE=transport_execution_revisions.version
EXECUTION_EVENT_SEQUENCE=transport_executions.event_seq
SUCCESSOR_REPLAN_RUNTIME_IMPLEMENTED=NO
DRIVER_RUNTIME_CHANGED=NO
TRACKING_CHANGED=NO
CONTROL_TOWER_CHANGED=NO
NLO_0_4D_STARTED=NO
READY_FOR_PRODUCTION_EXECUTION=NO
```

`TransportExecutionCommandService.Execute` is an in-process method. It is not mounted on HTTP.

Stop commands are `ARRIVE_STOP`, `START_STOP_SERVICE`, `COMPLETE_STOP`, `SKIP_STOP`, `CANCEL_REMAINING_STOPS`, and `CANCEL_IN_SERVICE_STOP`. Action commands are `CONFIRM_PICKUP`, `CONFIRM_DELIVERY`, and `FAIL_ACTION`. `DEPARTED_PICKUP` applies the existing single-leg `LOADED` to `IN_TRANSIT` transition for an active participant. It does not add a stop status or a public DepartStop API.

Pickup confirmation appends `CONFIRMED_ONBOARD` through the existing ledger. Delivery confirmation appends `UNLOADED`. The evidence row uses the action's `shipment_tenant_id`, `shipment_id`, and `cargo_id`. A repeated confirmation returns the original `evidence_id`.

Shipment status changes use `ValidateStatusTransition`. Stop-only events use aggregate type `TRANSPORT_EXECUTION`, aggregate id of the execution, and aggregate version of the active revision. `event_seq` orders those events. A shipment status change still writes `shipment.status.changed` at the shipment version.

An operator `ARRIVE_STOP` on a later stop skips earlier `PLANNED` stops, cancels their pending actions, and records those ordinals. It does not pass an earlier `ARRIVED` or `SERVICE_STARTED` stop. A failed required delivery blocks `DELIVERED` for that shipment. A cancelled earlier pickup is not the first required pickup.

Command idempotency is `UNIQUE (operating_tenant_id, idempotency_key)` on `transport.transport_execution_commands`. The same key with a different body conflicts. Audit rows live in `transport.transport_execution_command_audit`.

Migration: `000089_tms_transport_execution_commands_v0_1b`. Down drops only the 0.1B command tables, triggers, functions, and `event_seq`.

# TMS multi-stop execution architecture

Status: architecture freeze. Docs only. Runtime implementation is not authorized.

```text
TMS_MULTISTOP_ARCHITECTURE_FROZEN=YES
TMS_MULTISTOP_IMPLEMENTATION_STARTED=NO
NLO_0_4D_STARTED=NO
IMPLEMENTATION_AUTHORIZED=NO
CONTROLLER_REVIEW_PENDING=YES
DOCS_ONLY=YES
AGENT_D_PLANS=YES
AGENT_C_EXECUTES=YES
EXECUTION_ROUTE_MODEL=TRANSPORT_EXECUTION
NLO_0_4C_ACTIVATION_RELEASE_BLOCKED_UNTIL_SERVICE_DURATION_SOURCE=YES
TMS_EXECUTION_DOES_NOT_WEAKEN_NLO_ACTIVATION_GATE=YES
SLOT_BOOKING_AUTHORITY=NOT_IMPLEMENTED
```

Discovery baseline: `origin/main` `0fc6a7979ca5ea0cbbbd50ff5770bbbf22304a5c`, rebased onto `1566c0d30b93bca1016fed6e81620edf277be52f`. Remediation R1 is on `discovery/tms-multistop-execution-v0.1`.

`network-optimizer-service` owns planning. `shipment-service` owns execution. The execution aggregate is `TransportExecution`, which can include many materialized shipments and does not require an anchor shipment for `DEPOT_START`. This pack does not modify optimizer files, OpenAPI, or migrations.

TMS execution implementation does not remove or weaken the NLO activation release gate. No production execution activation is authorized until an authoritative or versioned service-duration source exists. `network.route_plan.execution_linked` is `AGENT_D_CONTRACT_REQUIRED` and `IMPLEMENTED_TODAY=NO`.

## Documents

| Document | Decision |
| --- | --- |
| [CURRENT_STATE_INVENTORY.md](CURRENT_STATE_INVENTORY.md) | What already exists |
| [DOMAIN_MODEL.md](DOMAIN_MODEL.md) | Aggregate, diagrams, ownership answers |
| [EXECUTION_PLAN_MODEL.md](EXECUTION_PLAN_MODEL.md) | Projection of an activated RoutePlan |
| [EXECUTION_STOP_MODEL.md](EXECUTION_STOP_MODEL.md) | Stop parent is the route; revisions only link |
| [STOP_ACTION_MODEL.md](STOP_ACTION_MODEL.md) | Pickup and delivery actions |
| [SHIPMENT_FSM_ALIGNMENT.md](SHIPMENT_FSM_ALIGNMENT.md) | Coarse shipment status stays; it does not reset |
| [DRIVER_TASK_MODEL.md](DRIVER_TASK_MODEL.md) | Extend driver operations; do not fork a second inbox |
| [TRACKING_INTEGRATION.md](TRACKING_INTEGRATION.md) | Position and live ETA stay in tracking-service |
| [CONTROL_TOWER_INTEGRATION.md](CONTROL_TOWER_INTEGRATION.md) | New stop events beside the status projection |
| [SLOT_BOOKING_INTEGRATION.md](SLOT_BOOKING_INTEGRATION.md) | Slot windows are not optimizer-owned |
| [ROUTEPLAN_EXECUTION_CONTRACT.md](ROUTEPLAN_EXECUTION_CONTRACT.md) | Stable contract Agent C requires from Agent D |
| [REPLAN_SUCCESSOR_MODEL.md](REPLAN_SUCCESSOR_MODEL.md) | Successor plan; completed history stays |
| [FAILURE_RECOVERY.md](FAILURE_RECOVERY.md) | Idempotent, ordered, auditable failures |
| [SECURITY_PRIVACY.md](SECURITY_PRIVACY.md) | Execution rights are not marketplace visibility |
| [EVENTS.md](EVENTS.md) | Outbox in the same transaction; separate version stream |
| [IMPLEMENTATION_ROADMAP.md](IMPLEMENTATION_ROADMAP.md) | Waves after controller acceptance |

## ADRs

| ID | Title |
| --- | --- |
| [ADR-TMS-001](adr/ADR-TMS-001-execution-aggregate-ownership.md) | Execution aggregate ownership |
| [ADR-TMS-002](adr/ADR-TMS-002-shipment-owned-stop-identity.md) | Stop identity allocated by shipment-service, parented by the route |
| [ADR-TMS-003](adr/ADR-TMS-003-routeplan-execution-projection.md) | RoutePlan to execution projection |
| [ADR-TMS-004](adr/ADR-TMS-004-completed-stop-immutability.md) | Completed stop immutability |
| [ADR-TMS-005](adr/ADR-TMS-005-replan-successor-semantics.md) | Replan and successor semantics |
| [ADR-TMS-006](adr/ADR-TMS-006-driver-task-ownership.md) | Driver task ownership |
| [ADR-TMS-007](adr/ADR-TMS-007-arrival-completion-authority.md) | Arrival and completion authority |

# ADR-TMS-002: Shipment-service stop identity

## Status

Accepted. Architecture freeze, remediation R1. Implementation is not authorized.

## Context

RoutePlan stop ids belong to an immutable planning version. Execution needs a stable id after a successor revision replaces the remaining route. The first freeze stored that id on a row whose parent was `execution_plan_id`. A later revision cannot also parent that row. Remediation R1 removes the plan as the parent.

## Decision

`shipment-service` allocates `TransportExecutionStop.id`. The parent column is `execution_id`, the `TransportExecution` route. It is set once and is not a shipment id. `source_route_plan_stop_id` is an external reference, not a foreign key into optimizer tables. The same rule applies to `source_route_plan_action_id`.

A revision includes a stop only through `TransportExecutionRevisionStop`. That link does not change `execution_id`.

```text
EXECUTION_STOP_ID_ALLOCATED_BY=shipment-service
EXECUTION_STOP_PARENT=TransportExecution
EXECUTION_STOP_ID_SHIPMENT_ROW_OWNED=NO
ROUTEPLAN_STOP_REFERENCE=source_route_plan_stop_id
COMPLETED_STOP_ROW_REPARENTED=NO
COMPLETED_STOP_ID_CHANGED=NO
```

## Consequences

Completed and in-service stops keep their ids and their parent route. A successor revision adds link rows. It does not move the stop.

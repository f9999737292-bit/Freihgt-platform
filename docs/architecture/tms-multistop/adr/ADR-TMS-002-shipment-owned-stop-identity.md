# ADR-TMS-002: Shipment-service stop identity

## Status

Accepted. Architecture freeze, remediation R2. Implementation is not authorized.

## Context

RoutePlan stop ids belong to an immutable planning version. Execution needs a stable id after a successor revision replaces the remaining route. The first freeze stored that id on a row whose parent was `execution_plan_id`. A later revision cannot also parent that row. Remediation R1 removes the plan as the parent.

## Decision

`shipment-service` allocates `TransportExecutionStop.id`. The parent column is `execution_id`, the `TransportExecution` route. It is set once and is not a shipment id. The stable stop row does not store `source_route_plan_stop_id`. A successor RoutePlan has its own stop ids, so one immutable column on the stable row cannot name both the introducing plan and the inheriting plan.

`source_route_plan_stop_id` and `source_ordinal` live on `TransportExecutionRevisionStop`, together with membership `INTRODUCED`, `INHERITED_COMPLETED`, or `INHERITED_IN_SERVICE`. The introducing revision stores the original plan's stop id. The successor revision adds a new link with the successor plan's stop id. Neither link rewrites the stable row. The source id is an external reference, not a foreign key into optimizer tables.

The same split applies to actions. `TransportExecutionAction` has no `source_route_plan_action_id`. `TransportExecutionRevisionAction` stores `source_route_plan_action_id`, `source_action_ordinal`, and membership for that revision's RoutePlan.

```text
EXECUTION_STOP_ID_ALLOCATED_BY=shipment-service
EXECUTION_STOP_PARENT=TransportExecution
EXECUTION_STOP_ID_SHIPMENT_ROW_OWNED=NO
ROUTEPLAN_STOP_REFERENCE=PER_REVISION_LINK
STABLE_STOP_SOURCE_ID_REWRITTEN=NO
STABLE_ACTION_SOURCE_ID_REWRITTEN=NO
SUCCESSOR_ROUTEPLAN_STOP_LINEAGE_PRESERVED=YES
SUCCESSOR_ROUTEPLAN_ACTION_LINEAGE_PRESERVED=YES
COMPLETED_STOP_ROW_REPARENTED=NO
IN_SERVICE_STOP_ROW_REPARENTED=NO
COMPLETED_STOP_ID_CHANGED=NO
```

## Consequences

Completed and in-service stops keep their ids and their parent route. A successor revision adds link rows, including the successor's own source ids. It does not move the stop and it does not rewrite the introducing link.

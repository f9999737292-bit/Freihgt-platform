# ADR-TMS-002: Shipment-owned stop identity

## Status

Accepted. Architecture freeze. Implementation is not authorized.

## Context

RoutePlan stops already have ids in the optimizer. Those ids belong to a plan version that cannot be edited after evaluation. Execution needs a stable id for driver tasks, evidence, and Control Tower after a successor plan replaces the remaining route. Using the optimizer id as the shipment primary key would make execution keys mutable planning keys.

## Decision

Execution stop primary keys are allocated by `shipment-service`. The optimizer stop id is stored as `source_route_plan_stop_id` and is not a foreign key. The same rule applies to actions and `source_route_plan_action_id`.

```text
EXECUTION_STOP_ID_SHIPMENT_OWNED=YES
ROUTEPLAN_STOP_REFERENCE=source_route_plan_stop_id
```

## Consequences

A successor plan can keep completed shipment stop ids and allocate new ids for stops that did not complete. Driver and Control Tower references survive replan.

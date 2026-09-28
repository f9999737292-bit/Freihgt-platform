# ADR-NET-018: RoutePlan ownership and lifecycle

## Status

Accepted. NLO-0.4B persists evaluated plans. Accept and activate remain NLO-0.4C.

```text
ADR_STATUS=ACCEPTED
ADR_NET_018=ACCEPTED
OWNERSHIP_DECISION_FROZEN=YES
NLO_0_4_IMPLEMENTATION_STARTED=YES
NLO_0_4B_STATUS=IMPLEMENTED_CLOSED
NLO_0_4B_MERGE_SHA=0fc6a7979ca5ea0cbbbd50ff5770bbbf22304a5c
NLO_0_4C_STARTED=YES
NLO_0_4C_ACCEPTED=NO
```

## Context

NLO-0.3 is closed. Shipments still have one origin and one destination. ADR-NET-005 already says a route plan is not a shipment and that multi-stop execution cannot become active until an execution model exists. The architecture freeze found no `route_plans` table. NLO-0.4B created migration `000085_nlo_route_plan_bounded_planner_v0_4b`.

## Decision

`network-optimizer-service` owns the planning artifact. `shipment-service` owns shipment status and, later, driver stop tasks. `tracking-service` owns position freshness. `transport-order-service` owns the commercial order.

A plan version's structure is immutable once `EVALUATED` is persisted: stops, actions, legs, capacity snapshots, dependencies, and fingerprints. Lifecycle status may still move under compare-and-swap. The planning statuses are `EVALUATED`, `ACCEPTED`, `SUPERSEDED`, and `CANCELLED`. `DRAFT`, `ACTIVE`, and `COMPLETED` are not plan statuses. Execution progress is not written back onto the plan. A change creates a successor with `supersedes_plan_id`.

```text
PLAN_STRUCTURE_IMMUTABLE_AFTER_EVALUATION=YES
LIFECYCLE_STATUS_MUTABLE=YES
ACCEPTED_PLAN_STRUCTURE_IMMUTABLE=YES
REPLAN_CREATES_SUCCESSOR=YES
ACTIVE_PLAN_DIRECT_EDIT_ALLOWED=NO
OWNERSHIP_DECISION_FROZEN=YES
```

## Consequences

This ADR is accepted. NLO-0.4B persists evaluated plans on main in PR #184, migration `000085_nlo_route_plan_bounded_planner_v0_4b`. NLO-0.3 APIs stay unchanged. NLO-0.4C creates `route_plan_activations`. It does not mutate shipment stops.

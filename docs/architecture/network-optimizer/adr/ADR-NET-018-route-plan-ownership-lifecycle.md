# ADR-NET-018: RoutePlan ownership and lifecycle

## Status

Proposed. Discovery only. Not accepted. Implementation is not authorized.

```text
ADR_STATUS=PROPOSED
NLO_0_4_IMPLEMENTATION_AUTHORIZED=NO
```

## Context

NLO-0.3 is closed. Shipments still have one origin and one destination. ADR-NET-005 already says a route plan is not a shipment and that multi-stop execution cannot become active until an execution model exists. There is no `route_plans` table.

## Decision

`network-optimizer-service` owns the planning artifact. `shipment-service` owns shipment status and, later, driver stop tasks. `tracking-service` owns position freshness. `transport-order-service` owns the commercial order.

A plan version's structure is immutable once `EVALUATED` is persisted: stops, actions, legs, capacity snapshots, dependencies, and fingerprints. Lifecycle status may still move under compare-and-swap. The planning statuses are `EVALUATED`, `ACCEPTED`, `SUPERSEDED`, and `CANCELLED`. `DRAFT`, `ACTIVE`, and `COMPLETED` are not plan statuses. Execution progress is not written back onto the plan. A change creates a successor with `supersedes_plan_id`.

```text
PLAN_STRUCTURE_IMMUTABLE_AFTER_EVALUATION=YES
LIFECYCLE_STATUS_MUTABLE=YES
ACCEPTED_PLAN_STRUCTURE_IMMUTABLE=YES
REPLAN_CREATES_SUCCESSOR=YES
ACTIVE_PLAN_DIRECT_EDIT_ALLOWED=NO
OWNERSHIP_FREEZE_CANDIDATE=YES
OWNERSHIP_DECISION_FROZEN=NO
```

## Consequences

NLO-0.4B may persist this artifact only after this ADR is accepted. NLO-0.3 APIs stay unchanged. No migration is created here.

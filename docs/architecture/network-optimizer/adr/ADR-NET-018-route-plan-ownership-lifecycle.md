# ADR-NET-018: RoutePlan ownership and lifecycle

## Status

Accepted. Architecture freeze. Implementation is not started.

```text
ADR_STATUS=ACCEPTED
ADR_NET_018=ACCEPTED
OWNERSHIP_DECISION_FROZEN=YES
NLO_0_4_IMPLEMENTATION_STARTED=NO
NLO_0_4_IMPLEMENTATION_AUTHORIZED=NO
NLO_0_4B_IMPLEMENTATION_AUTHORIZED_AFTER_PR182_MERGE=YES
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
OWNERSHIP_DECISION_FROZEN=YES
```

## Consequences

This ADR is accepted. NLO-0.4B may persist this artifact after PR #182 merges. That wave is not started here. NLO-0.3 APIs stay unchanged. No migration is created here.

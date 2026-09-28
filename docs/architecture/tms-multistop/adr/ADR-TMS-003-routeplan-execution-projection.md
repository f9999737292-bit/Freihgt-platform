# ADR-TMS-003: RoutePlan to execution projection

## Status

Accepted. Architecture freeze. Implementation is not authorized.

## Context

Accepting a plan does not start a trip. ADR-NET-021 separates accept from activate. Activation rows are not in the database yet. A client must not be able to post an arbitrary stop list and have it become the driver's route.

## Decision

The only execution source is an `EXECUTION_LINKED` activation delivered as `network.route_plan.execution_linked` from the optimizer service identity. `CreateExecutionProjectionFromActivation` is idempotent on `activation_id`. The same id returns the same plan. A different body conflicts. An unrelated new activation conflicts while another plan is active. A successor activation supersedes in one transaction. Projection copies stops and actions only. It does not change shipment status.

```text
EXECUTION_PROJECTION_SOURCE=ACTIVATED_ROUTE_PLAN
IDEMPOTENT_EXECUTION_PROJECTION=YES
CALLER_SUPPLIED_STOPS_ACCEPTED=NO
```

## Consequences

Implementation waits until Agent D emits this contract. This ADR does not change optimizer code or NLO documents.

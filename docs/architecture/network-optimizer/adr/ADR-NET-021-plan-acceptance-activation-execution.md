# ADR-NET-021: Plan acceptance, activation, and execution

## Status

Proposed. Discovery only. Not accepted. Implementation is not authorized.

## Context

Choosing a suggested plan is not the same as starting a trip. Shipment status, driver tasks, and slot-named statuses already live in `shipment-service`. No slot-booking service exists. ADR-NET-005 forbids treating a multi-stop plan as executable until that execution model exists.

## Decision

`ACCEPT` sets an evaluated plan to `ACCEPTED` after dependency versions match. It does not reserve a slot or create a driver task.

`ACTIVATE` writes an append-only activation row for an accepted plan. It checks version freshness (`409` `PLAN_STALE`), routing expiry, and that the plan is not indeterminate. It does not edit stops. A repeated activation does not create a second driver task.

```text
PLAN_ACCEPT_SEPARATE_FROM_ACTIVATE=YES
CALLER_SUPPLIED_EXECUTION_STATE_ALLOWED=NO
DOUBLE_ACTIVATION_IDEMPOTENT=YES
PLANNING_RESERVES_SLOT=NO
```

Driver stop tasks, if built later, are shipment-owned projections. Completed stop history is immutable. Replanning creates a successor plan.

Public caller-visible reasons and the idempotency rule are in `NLO_0_4A_EXECUTION_BOUNDARY.md` and `NLO_0_4A_SECURITY_PRIVACY.md`.

## Consequences

NLO-0.4C is the proposed accept/activate implementation. NLO-0.4D is the proposed shipment and driver integration. Neither starts in this discovery.

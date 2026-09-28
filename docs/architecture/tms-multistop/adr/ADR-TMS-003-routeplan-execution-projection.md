# ADR-TMS-003: RoutePlan to execution projection

## Status

Accepted. Architecture freeze, remediation R1. Implementation is not authorized.

## Context

Accepting a plan does not start a trip. ADR-NET-021 separates accept from activate and names activation status `EXECUTION_LINKED`. That status is an accepted planning semantic. The event `network.route_plan.execution_linked` is not implemented. `route_plan_activations` is not in this baseline. A client must not post an arbitrary stop list. A `LOAD_OPPORTUNITY` id is not cargo evidence.

## Decision

The only execution source is an `EXECUTION_LINKED` activation supplied by the optimizer service identity. `CreateExecutionProjectionFromActivation` is idempotent on `activation_id` and writes a `TransportExecutionRevision` on a `TransportExecution`. It does not require one anchor shipment. It does require every cargo action to carry `execution_shipment_id` and `cargo_id`. A raw `LOAD_OPPORTUNITY` without those fields returns `409 EXECUTION_SUBJECT_UNMATERIALIZED` and writes nothing. Projection does not create the shipment.

```text
EXECUTION_PROJECTION_SOURCE=ACTIVATED_ROUTE_PLAN
IDEMPOTENT_EXECUTION_PROJECTION=YES
CALLER_SUPPLIED_STOPS_ACCEPTED=NO
RAW_LOAD_OPPORTUNITY_EXECUTABLE=NO
EXECUTION_SUBJECT_MATERIALIZATION_REQUIRED=YES
network.route_plan.execution_linked
IMPLEMENTED_TODAY=NO
AGENT_D_CONTRACT_REQUIRED=YES
```

`EXECUTION_LINKED` remains the planning status that means "linked for execution". The event name above is the required future signal. It is not a claim that Agent D emits it today.

## Consequences

Implementation waits until Agent D emits this contract and until the NLO service-duration release gate is satisfied. This ADR does not change optimizer code or NLO documents.

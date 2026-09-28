# ADR-TMS-003: RoutePlan to execution projection

## Status

Accepted. Architecture freeze, remediation R2. Implementation is not authorized.

## Context

Accepting a plan does not start a trip. ADR-NET-021 separates accept from activate. Activation is `PENDING_EXECUTION`, then `EXECUTION_LINKED` or `REJECTED`. `EXECUTION_LINKED` means the execution ids have already been stored. It cannot also be the input that creates those ids. The event `network.route_plan.execution_linked` is not implemented. `route_plan_activations` is not in this baseline. A client must not post an arbitrary stop list. A `LOAD_OPPORTUNITY` id is not cargo evidence.

## Decision

The projection trigger is a trusted synchronous command, `CreateExecutionProjectionFromActivation`, while the activation is `PENDING_EXECUTION`. The caller is the `network-optimizer-service` identity. There is no second request/reply channel and no distributed transaction.

Shipment-service validates materialized subjects, versions, participant owner tenants, and idempotency, then commits `TransportExecution` and the revision. The response is `execution_id`, `revision_id`, and `activation_id`. The same `activation_id` returns the same pair. Agent D stores those ids, then sets `EXECUTION_LINKED`, then emits `network.route_plan.execution_linked`. That event is the post-link fact. A refusal writes no revision. A lost response stays `PENDING_EXECUTION` and retries the same command. Agent D must not persist `EXECUTION_LINKED` without the returned ids.

The command does not require one anchor shipment. It does require every cargo action to carry `execution_shipment_id`, `shipment_tenant_id`, and `cargo_id`. `shipment_tenant_id` is the shipment's existing owner tenant. A raw `LOAD_OPPORTUNITY` without those fields returns `409 EXECUTION_SUBJECT_UNMATERIALIZED` and writes nothing. Projection does not create the shipment and does not move it into `operating_tenant_id`.

```text
EXECUTION_PROJECTION_SOURCE=PENDING_EXECUTION_CONTRACT
EXECUTION_PROJECTION_TRIGGER_IS_EXECUTION_LINKED=NO
EXECUTION_LINKED_IS_POST_PROJECTION_FACT=YES
PENDING_EXECUTION_PROJECTION_CONTRACT_REQUIRED=YES
PROJECTION_TRANSPORT=TRUSTED_SYNCHRONOUS_COMMAND
IDEMPOTENT_EXECUTION_PROJECTION=YES
SAME_ACTIVATION_RETURNS_SAME_EXECUTION_REVISION=YES
NO_STATE_WITH_EXECUTION_LINKED_BUT_NO_EXECUTION_PROJECTION=YES
CALLER_SUPPLIED_STOPS_ACCEPTED=NO
RAW_LOAD_OPPORTUNITY_EXECUTABLE=NO
EXECUTION_SUBJECT_MATERIALIZATION_REQUIRED=YES
network.route_plan.execution_linked
IMPLEMENTED_TODAY=NO
AGENT_D_CONTRACT_REQUIRED=YES
```

`EXECUTION_LINKED` remains the planning status that means "linked for execution" after the projection exists. The event name above is the required future post-link signal. It is not a claim that Agent D emits it today, and it is not the projection trigger.

## Consequences

Implementation waits until Agent D emits this contract and until the NLO service-duration release gate is satisfied. This ADR does not change optimizer code or NLO documents.

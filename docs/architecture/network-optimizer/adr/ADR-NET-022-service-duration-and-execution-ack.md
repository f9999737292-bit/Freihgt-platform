# ADR-NET-022: Service duration policy and execution acknowledgement

## Status

Proposed. NLO-0.4D-R1 architecture freeze. NLO-0.4D-I2 implements the storage and the handoff on this branch. Controller acceptance is still required. Merge is not authorized.

```text
ADR_NET_022=PROPOSED
NLO_0_4D_R1=DOCS_ONLY
IMPLEMENTATION_AUTHORIZED=NO
CONTROLLER_REVIEW_REQUIRED=YES
```

## Context

NLO-0.4C persists `PENDING_EXECUTION` and leaves `execution_id` and `execution_revision_id` null. ADR-NET-019 sets `SERVICE_DURATION_SOURCE=AUTHORITATIVE_OR_UNKNOWN` and `DEFAULT_ZERO=NO`. NLO-0.4A defers activation until a later policy names an owner, a version, a pickup duration, a delivery duration, and a rationale, and it refuses to invent those numbers. Discovery on `470f0a6fc915093f8bb91a32e97fcf33da3f3deb` found no dwell table, no catalog duration, and no server write of `routeplan.Input.ServiceDurationSeconds`.

ADR-TMS-003 already chooses `CreateExecutionProjectionFromActivation` as the projection trigger and treats `network.route_plan.execution_linked` as a post-link fact. Discovery found that command only as an in-process shipment-service function. `shipment.execution_plan.created` does not carry `activation_id` or `route_plan_id`. NLO does not consume it.

## Decision

The authoritative service duration is a versioned operating-tenant logistics policy. NLO-0.4D-I2 stores that policy in network-optimizer-service. The optimizer reads the published active row. It does not trust a client duration. Unknown policy data stays fail-closed. This ADR names the owner and the version. It does not publish pickup or delivery seconds.

The authoritative TMS acknowledgement is the response of that same synchronous command, reached through the existing internal authenticated shipment API. NLO does not write TMS tables. TMS does not write NLO tables. The execution-created event is not the acknowledgement.

```text
SERVICE_DURATION_OWNER=OPERATING_TENANT_LOGISTICS_POLICY
SERVICE_DURATION_VERSIONED=YES
CLIENT_SERVICE_DURATION_TRUSTED=NO
UNKNOWN_SERVICE_DURATION_FAIL_CLOSED=YES
SERVICE_DURATION_FALLBACK_APPROVED=NO
TMS_ACK_MECHANISM=SYNCHRONOUS_INTERNAL_API_PLUS_DURABLE_RECONCILIATION
NLO_WRITES_TMS_DB=NO
TMS_WRITES_NLO_DB=NO
ACK_CORRELATION_UNAMBIGUOUS=YES
DEPOT_START_SUPPORTED=NO
```

## Consequences

Details are in `SERVICE_DURATION_SOURCE.md`, `EXECUTION_HANDOFF_ACK.md`, `ACTIVATION_STATE_MACHINE.md`, and `NLO_0_4D_ARCHITECTURE_FREEZE.md`. No migration and no product code accompany this ADR. Production activation stays blocked until a later authorized wave publishes versioned policy rows. Depot-start activation stays fail-closed.

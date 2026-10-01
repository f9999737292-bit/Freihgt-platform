# Execution handoff acknowledgement

NLO-0.4D-R1 architecture freeze. Docs only. Baseline `470f0a6fc915093f8bb91a32e97fcf33da3f3deb`.

```text
TMS_ACK_ARCHITECTURE_DECIDED=YES
TMS_ACK_MECHANISM=SYNCHRONOUS_INTERNAL_API_PLUS_DURABLE_RECONCILIATION
PROJECTION_TRANSPORT=TRUSTED_SYNCHRONOUS_COMMAND
ACK_EVENT=NONE
ACK_API=CreateExecutionProjectionFromActivation
ACK_CORRELATION_UNAMBIGUOUS=YES
NLO_WRITES_TMS_DB=NO
TMS_WRITES_NLO_DB=NO
EVENT_DRIVEN_ACK_SELECTED=NO
ONE_ACTIVATION=YES
ONE_EXECUTION_ROOT=YES
ONE_EXECUTION_LINK=YES
```

## Choice

Three mechanisms were compared against the accepted contracts and the discovery baseline.

| Option | Result |
| --- | --- |
| A. Event-driven acknowledgement | Rejected. ADR-TMS-003 says the projection trigger is the synchronous command. `network.route_plan.execution_linked` is the post-link fact. `shipment.execution_plan.created` is a TMS outbox fact after commit. Its payload has `operating_tenant_id`, `execution_id`, and `revision_id`. It has no `activation_id` and no `route_plan_id`. Matching on time or shipment id is forbidden. NLO has no consumer of that event. |
| B. Synchronous internal API plus durable reconciliation | Selected. This is ADR-TMS-003 and `ROUTEPLAN_EXECUTION_CONTRACT.md`. The response is the acknowledgement. A lost response stays `PENDING_EXECUTION` and retries the same `activation_id`. |
| C. Another repository mechanism | No safer existing path. A direct write from NLO into `transport.*`, or from TMS into `network_optimizer.route_plan_activations`, is forbidden. |

Discovery proved the command is in-process only. Shipment HTTP already exposes other internal routes under `/internal/v1` with the internal auth middleware. Successor revision, tracking context, and delivery disposition use that surface. The projection command does not. Publishing that same command on the existing internal surface is the service boundary for option B. It is not a second business channel and not a distributed transaction.

```text
INGRESS=shipment-service internal authenticated API
CALLER=network-optimizer-service identity
GATEWAY_JWT_CALLER_ALLOWED=NO
BROWSER_BODY_STOP_LIST_ALLOWED=NO
```

## Sequence

1. NLO accepts an evaluated plan only when dependency versions still match.
2. NLO inserts one `route_plan_activations` row as `PENDING_EXECUTION`. `execution_id` and `execution_revision_id` stay null. The plan status stays `ACCEPTED`.
3. NLO calls `CreateExecutionProjectionFromActivation` with the server-built projection body from `ROUTEPLAN_EXECUTION_CONTRACT.md`.
4. Shipment-service validates materialized subjects, versions, participant tenants, and the activation digest, then commits `TransportExecution` or replays the existing revision.
5. The response is the acknowledgement. NLO reads the correlation fields from that response.
6. NLO stores `execution_id` and `execution_revision_id` and moves that activation to `EXECUTION_LINKED` in its own transaction.
7. NLO emits `network.route_plan.execution_linked` once. That event is the post-link fact. It does not create the execution.

A transport failure before step 4 leaves the activation `PENDING_EXECUTION`. A permanent refusal before commit writes no revision. NLO may then set `REJECTED`. A lost response after commit stays `PENDING_EXECUTION` until retry returns the same ids.

## Correlation

The acknowledgement must prove which activation produced which execution. The stored revision already has `operating_tenant_id`, `source_activation_id`, and `source_route_plan_id`. The response must return those stored values, not a guess.

```text
ACK_CORRELATION_FIELDS=operating_tenant_id,activation_id,route_plan_id,execution_id,execution_revision_id
ACK_FIELDS_READ_FROM=transport.transport_execution_revisions
```

| Field | Required source |
| --- | --- |
| `operating_tenant_id` | Stored revision tenant. This is the NLO activation tenant sent as the command operating tenant. |
| `activation_id` | Stored `source_activation_id`. |
| `route_plan_id` | Stored `source_route_plan_id`. |
| `execution_id` | Stored `execution_id`. |
| `execution_revision_id` | Stored revision `id`. |

The current in-process `ProjectionResult` returns `execution_id`, `revision_id`, `activation_id`, and `created`. It does not return `operating_tenant_id` or `route_plan_id`. The later TMS contract change adds those two fields on both create and replay, read from the committed row. NLO discards a response whose five fields do not match the activation it sent.

`shipment.execution_plan.created` is not updated by this freeze and is not the acknowledgement. The exact missing correlation fields on that payload are `activation_id` and `route_plan_id`. Adding them later does not make the event the handshake.

## Idempotency

| Business fact | Uniqueness |
| --- | --- |
| Activation request | One row for `(tenant_id, route_plan_id)`. One row for `(tenant_id, idempotency_key)`. A repeat of the same plan returns the existing activation. |
| TMS execution projection | One revision for `(operating_tenant_id, source_activation_id)`. The same body replays the same execution and revision. A different body is `409 ACTIVATION_BODY_CONFLICT` and writes nothing new. |
| Acknowledgement | The response of that replay. A second response with the same five fields is the same acknowledgement. |
| NLO execution link | One transition of that activation to `EXECUTION_LINKED`, with both ids set. A second identical acknowledgement does not change the row. |
| `network.route_plan.execution_linked` | One outbox event for that activation. The business key is `activation_id`. Replay of the link does not insert a second event. |

```text
ONE_ACTIVATION=YES
ONE_EXECUTION_ROOT=YES
ONE_EXECUTION_LINK=YES
DUPLICATE_HANDOFF_SAFE=YES
DUPLICATE_ACK_SAFE=YES
```

An active shipment slot still prevents a second execution root for a shipment that is already on a route. A successor revision is a TMS command on the same root. NLO does not create that root.

## Recovery

| Situation | Result |
| --- | --- |
| Duplicate activation request | Same activation row. No second `PENDING_EXECUTION`. |
| Duplicate projection command | Same `execution_id` and `execution_revision_id`. |
| Duplicate acknowledgement | Same link. No second `execution_linked` event. |
| Out-of-order event | `shipment.execution_plan.created` is ignored as an acknowledgement. `execution_linked` is emitted only after the synchronous response is stored. A late created-event does not move the activation. |
| Lost response | Activation stays `PENDING_EXECUTION`. Retry uses the same `activation_id` and the same body. |
| Consumer restart | There is no ack consumer. Restart continues from the activation row. |
| NLO restart after `PENDING_EXECUTION` | Retry the command. Store the returned ids. Then set `EXECUTION_LINKED`. |
| NLO restart after `EXECUTION_LINKED` | Read the stored ids. Do not call TMS again unless the ids are absent. |
| TMS restart after commit | Replay returns the committed revision. |
| TMS restart before commit | No revision. The retry creates the first revision. |
| Ambiguous transport error | Stay `PENDING_EXECUTION`. Do not return the plan to `EVALUATED` and do not clear the activation. |

Permanent refusals (`EXECUTION_SUBJECT_UNMATERIALIZED`, `PLAN_STALE`, `ACTIVATION_BODY_CONFLICT`, `EXECUTION_PLAN_CONFLICT`, authorization failure) write no new revision. NLO sets `REJECTED` only for those deterministic refusals. A previous `ACTIVE` revision on another activation stays active.

## Privacy

The acknowledgement and `network.route_plan.execution_linked` carry the five correlation fields and the activation status. They do not carry foreign commercial terms, optimizer scores, or a client-supplied tenant. Participant shipment tenants stay inside the TMS projection body, checked by shipment-service against shipment rows.

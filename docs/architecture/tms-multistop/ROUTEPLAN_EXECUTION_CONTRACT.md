# RoutePlan to execution contract

```text
ROUTEPLAN_EXECUTION_CONTRACT_FROZEN=YES
EXECUTION_PROJECTION_SOURCE=PENDING_EXECUTION_CONTRACT
EXECUTION_PROJECTION_TRIGGER_IS_EXECUTION_LINKED=NO
EXECUTION_LINKED_IS_POST_PROJECTION_FACT=YES
PENDING_EXECUTION_PROJECTION_CONTRACT_REQUIRED=YES
IDEMPOTENT_EXECUTION_PROJECTION=YES
CALLER_SUPPLIED_STOPS_ACCEPTED=NO
RAW_LOAD_OPPORTUNITY_EXECUTABLE=NO
EXECUTION_SUBJECT_MATERIALIZATION_REQUIRED=YES
DEPOT_START_WITHOUT_PREEXISTING_SHIPMENT_SUPPORTED=YES
MULTI_SHIPMENT_EXECUTION_SUPPORTED_BY_MODEL=YES
CROSS_SHIPPER_EXECUTION_SUPPORTED_BY_MODEL=YES
SHIPMENT_REHOMED_TO_CARRIER_TENANT=NO
SAME_ACTIVATION_RETURNS_SAME_EXECUTION_REVISION=YES
NO_STATE_WITH_EXECUTION_LINKED_BUT_NO_EXECUTION_PROJECTION=YES
NETWORK_ROUTE_PLAN_EXECUTION_LINKED_IMPLEMENTED_TODAY=NO
AGENT_D_CONTRACT_REQUIRED=YES
PROJECTION_TRANSPORT=TRUSTED_SYNCHRONOUS_COMMAND
```

Agent C does not read optimizer tables. Agent D owns the activation row. This document does not edit NLO files.

`EXECUTION_LINKED` means Agent D has stored the execution ids returned by shipment-service. It is not the trigger that creates those ids. The first implementation uses one trusted synchronous command. A durable request/reply pair is not a second channel.

## Sequence

1. Agent D validates an accepted RoutePlan, including the NLO service-duration release gate. This architecture does not weaken that gate.
2. Agent D persists the activation as `PENDING_EXECUTION`.
3. Agent D calls `CreateExecutionProjectionFromActivation` with the service identity of `network-optimizer-service`.
4. Shipment-service checks materialized subjects, shipment and evidence versions, participant owner tenants, execution authorization, and idempotency.
5. Shipment-service commits `TransportExecution` and the revision, or returns the existing revision for that `activation_id`.
6. The response is the durable linkage: `execution_id`, `revision_id`, `activation_id`.
7. Agent D, in its own transaction, moves `PENDING_EXECUTION` to `EXECUTION_LINKED` only after those ids are stored.
8. Agent D then emits `network.route_plan.execution_linked`. That event is the post-link fact. `IMPLEMENTED_TODAY=NO`.
9. A deterministic rejection before step 5 leaves the activation unlinked. Agent D sets `REJECTED` for a permanent execution refusal. A lost response stays `PENDING_EXECUTION` and retries the same command.

There is no distributed transaction. The projection commit and the activation status commit are separate. Idempotency on `activation_id` closes the gap.

## Projection request

| Field | Rule |
| --- | --- |
| `activation_id` | Idempotency identity |
| `activation_version` | Changes when the activation row changes |
| `activation_status` | Must be `PENDING_EXECUTION` |
| `route_plan_id` | Plan being linked |
| `route_plan_version` | Version frozen at this attempt |
| `planning_mode` | `DEPOT_START` or `CURRENT_TRIP` |
| `operating_tenant_id` | Carrier execution scope |
| `context_shipment_id` | Required for `CURRENT_TRIP`. Null for `DEPOT_START`. Not the execution root |
| `context_shipment_tenant_id` | Owner tenant of that shipment when the id is set |
| `context_shipment_version` | Required when the id is set |
| `carrier_company_id` | Carrier on the route |
| `vehicle_id` | Nullable |
| `driver_id` | Nullable |
| `evaluation_fingerprint` | Server fingerprint from the plan |
| `supersedes_route_plan_id` | Previous plan, or null |
| `supersedes_activation_id` | Previous linked activation, or null |
| `execution_subjects[]` | Every cargo subject, already materialized |
| `stops[]` | Ordered, server-built, with that plan's stop ids |
| `actions[]` | Ordered inside each stop, with that plan's action ids |

`execution_subjects[]` entry: `route_subject_type` (`LOAD_OPPORTUNITY` or `SHIPMENT_CARGO`), `route_subject_id`, `route_subject_version`, `execution_shipment_id`, `shipment_tenant_id`, `execution_shipment_version`, `cargo_id`, `cargo_version`. `shipment_tenant_id` is the shipment's existing owner tenant. The command does not move the shipment.

Each stop entry: `route_plan_stop_id`, `ordinal`, `stop_role`, `point_kind`, `location_id` or null, `latitude`, `longitude`, `planned_arrival`, `planned_departure`, `service_duration_seconds` or null.

Each cargo action entry: `route_plan_action_id`, `route_plan_stop_id`, `action_ordinal`, `action_type`, `route_subject_type`, `route_subject_id`, `execution_shipment_id`, `shipment_tenant_id`, `execution_shipment_version`, `cargo_id`, `cargo_version`, `evidence_state`, `evidence_state_version`.

`START` and `END` need no subject. A missing `execution_shipment_id`, `shipment_tenant_id`, or `cargo_id` on a cargo action returns `409 EXECUTION_SUBJECT_UNMATERIALIZED` and writes nothing. A `LOAD_OPPORTUNITY` id alone is not that identity.

A successor request that continues an in-service stop is matched by semantic fingerprint, not by equal RoutePlan stop ids. See `STOP_ACTION_MODEL.md`. The successor's source ids are stored on the new revision links only.

## Response

| Field | Rule |
| --- | --- |
| `execution_id` | Stable route |
| `revision_id` | Revision created or replayed for this activation |
| `activation_id` | Echo of the request |

Same `activation_id` and same body return the same pair. Same id and a different body return `409 ACTIVATION_BODY_CONFLICT` and do not change the stored revision.

## After the response

Agent D stores the three ids, then sets `EXECUTION_LINKED`, then emits `network.route_plan.execution_linked` with those ids. Shipment-service does not set the activation status. If Agent D crashes after the response and before `EXECUTION_LINKED`, a retry returns the same ids and Agent D completes step 7. `EXECUTION_LINKED` without a stored projection is not a valid Agent D outcome.

Permanent shipment-service refusals (`EXECUTION_SUBJECT_UNMATERIALIZED`, `PLAN_STALE`, `IN_SERVICE_STOP_CONFLICT`, `EXECUTION_PLAN_CONFLICT`, authorization failure) return no new revision. Agent D sets `REJECTED` for that activation. A successor rejection leaves the previous revision `ACTIVE`.

## Trust

```text
AGENT_D_CONTRACT_REQUIRED_FOR_IMPLEMENTATION
activation persisted as PENDING_EXECUTION before the call
trusted synchronous CreateExecutionProjectionFromActivation
activation_status on the request is PENDING_EXECUTION
operating_tenant_id separate from shipment_tenant_id
execution_subjects materialized in the owner tenant
ordered stops and actions with that plan's source ids
response execution_id revision_id activation_id
EXECUTION_LINKED only after that response is stored
event network.route_plan.execution_linked after EXECUTION_LINKED
IMPLEMENTED_TODAY=NO
same activation_id returns the same execution revision
```

Gateway user JWTs cannot call the command. A browser body cannot supply `shipment_tenant_id` to gain access.

Production projection also waits on the NLO gate: activation release stays blocked until an authoritative or versioned service-duration source exists.

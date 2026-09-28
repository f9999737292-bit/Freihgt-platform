# RoutePlan to execution contract

```text
ROUTEPLAN_EXECUTION_CONTRACT_FROZEN=YES
EXECUTION_SOURCE=ACTIVATED_ROUTE_PLAN
IDEMPOTENT_EXECUTION_PROJECTION=YES
CALLER_SUPPLIED_STOPS_ACCEPTED=NO
RAW_LOAD_OPPORTUNITY_EXECUTABLE=NO
EXECUTION_SUBJECT_MATERIALIZATION_REQUIRED=YES
DEPOT_START_WITHOUT_PREEXISTING_SHIPMENT_SUPPORTED=YES
MULTI_SHIPMENT_EXECUTION_SUPPORTED_BY_MODEL=YES
```

Agent C does not read optimizer tables and does not depend on planner internals. Agent D must publish a stable activation contract. `ACCEPTED` is not execution. `EXECUTION_LINKED` is the accepted planning status from the NLO execution boundary. It means the plan is linked for execution. It does not mean shipment-service has already projected it.

```text
network.route_plan.execution_linked
IMPLEMENTED_TODAY=NO
AGENT_D_CONTRACT_REQUIRED=YES
```

NLO-0.4A catalogued `network.route_plan.activation_requested` as a future planning event. That request is not sufficient. `network.route_plan.execution_linked` is the name Agent C requires when an activation commits as `EXECUTION_LINKED`. It is not emitted today. `route_plan_activations` is absent from this baseline. This document does not edit NLO files to add the event.

## Required payload

| Field | Rule |
| --- | --- |
| `activation_id` | Idempotency identity |
| `activation_version` | Changes when the activation row changes |
| `activation_status` | Must be `EXECUTION_LINKED` |
| `route_plan_id` | Plan that was activated |
| `route_plan_version` | Version frozen at activate |
| `planning_mode` | `DEPOT_START` or `CURRENT_TRIP` |
| `context_shipment_id` | Required for `CURRENT_TRIP`. Null for `DEPOT_START`. Context, not the execution root |
| `context_shipment_version` | Required when `context_shipment_id` is set |
| `carrier_company_id` | Carrier on the route |
| `vehicle_id` | Nullable |
| `driver_id` | Nullable |
| `evaluation_fingerprint` | Server fingerprint from the plan |
| `supersedes_route_plan_id` | Previous plan, or null |
| `supersedes_activation_id` | Previous execution-linked activation, or null |
| `execution_subjects[]` | Every cargo subject, already materialized |
| `stops[]` | Ordered, server-built |
| `actions[]` | Ordered inside each stop, server-built |

`execution_subjects[]` entry:

| Field | Rule |
| --- | --- |
| `route_subject_type` | `LOAD_OPPORTUNITY` or `SHIPMENT_CARGO` |
| `route_subject_id` | Planning subject id |
| `route_subject_version` | Planning subject version |
| `execution_shipment_id` | Required. Trusted shipment in the execution tenant |
| `execution_shipment_version` | Required |
| `cargo_id` | Required |
| `cargo_version` | Required |

Each stop entry: `route_plan_stop_id`, `ordinal`, `stop_role`, `point_kind`, `location_id` or null, `latitude`, `longitude`, `planned_arrival`, `planned_departure`, `service_duration_seconds` or null.

Each cargo action entry: `route_plan_action_id`, `route_plan_stop_id`, `action_ordinal`, `action_type` (`PICKUP` or `DELIVERY`), `route_subject_type`, `route_subject_id`, `execution_shipment_id`, `execution_shipment_version`, `cargo_id`, `cargo_version`, `evidence_state`, `evidence_state_version`.

`START` and `END` need no subject. Every `PICKUP` and `DELIVERY` must match one `execution_subjects[]` row. If `execution_shipment_id` or `cargo_id` is missing, projection returns `409 EXECUTION_SUBJECT_UNMATERIALIZED` and writes nothing. A `LOAD_OPPORTUNITY` id alone is not that identity. Projection does not create the shipment.

Legs, capacity snapshots, compatibility fingerprints, prices, and other tenants' identities are not part of this contract.

## Stale check

If a named `execution_shipment_version` or onboard evidence version no longer matches shipment-service, projection returns `409 PLAN_STALE` and writes nothing. Agent D's activate-time check is not a substitute. For `DEPOT_START` there is no single shipment version to compare. Each subject is checked on its own.

Same `activation_id` replays. A new id that does not supersede the linked revision is `409 EXECUTION_PLAN_CONFLICT`.

## Trust

```text
AGENT_D_CONTRACT_REQUIRED_FOR_IMPLEMENTATION
activation_id
activation_version
activation_status=EXECUTION_LINKED
route_plan_id
route_plan_version
planning_mode
context_shipment_id nullable for DEPOT_START
execution_subjects with execution_shipment_id and cargo_id on every cargo action
ordered stops and actions
event network.route_plan.execution_linked
IMPLEMENTED_TODAY=NO
same activation_id replays
unresolved LOAD_OPPORTUNITY returns EXECUTION_SUBJECT_UNMATERIALIZED
```

The event is accepted only from the optimizer service identity, not from a browser body. Gateway user JWTs cannot call the projection command.

Production projection also waits on the NLO gate: activation release stays blocked until an authoritative or versioned service-duration source exists. This contract does not remove that gate.

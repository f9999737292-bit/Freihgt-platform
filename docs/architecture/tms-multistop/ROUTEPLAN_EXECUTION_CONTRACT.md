# RoutePlan to execution contract

```text
ROUTEPLAN_EXECUTION_CONTRACT_FROZEN=YES
EXECUTION_SOURCE=ACTIVATED_ROUTE_PLAN
IDEMPOTENT_EXECUTION_PROJECTION=YES
CALLER_SUPPLIED_STOPS_ACCEPTED=NO
```

Agent C does not read optimizer tables and does not depend on planner internals. Agent D publishes a stable activation contract. Only `EXECUTION_LINKED` is projectable. `ACCEPTED` is not execution.

## Required payload

| Field | Rule |
| --- | --- |
| `activation_id` | Idempotency identity |
| `activation_version` | Changes when the activation row changes |
| `activation_status` | Must be `EXECUTION_LINKED` |
| `route_plan_id` | Plan that was activated |
| `route_plan_version` | Version frozen at activate |
| `shipment_id` | The shipment this carrier execution belongs to |
| `shipment_version` | Version checked at activate |
| `planning_mode` | `DEPOT_START` or `CURRENT_TRIP` |
| `evaluation_fingerprint` | Server fingerprint from the plan |
| `supersedes_route_plan_id` | Previous plan, or null |
| `supersedes_activation_id` | Previous execution-linked activation, or null |
| `stops[]` | Ordered, server-built |
| `actions[]` | Ordered inside each stop, server-built |

Each stop entry: `route_plan_stop_id`, `ordinal`, `stop_role`, `point_kind`, `location_id` or null, `latitude`, `longitude`, `planned_arrival`, `planned_departure`, `service_duration_seconds` or null.

Each action entry: `route_plan_action_id`, `route_plan_stop_id`, `action_ordinal`, `action_type` (`PICKUP` or `DELIVERY`), `cargo_id`, `cargo_version`, `source_shipment_id`, `evidence_state`, `evidence_state_version`.

Legs, capacity snapshots, compatibility fingerprints, prices, and other tenants' identities are not part of this contract.

## Event

Agent D emits one event when the activation commits as `EXECUTION_LINKED`:

```text
network.route_plan.execution_linked
```

The name is the stable requirement. NLO-0.4A catalogued `network.route_plan.activation_requested` for the request. The request is not sufficient. Projection runs on the linked fact. Same `activation_id` replays. A new id that does not supersede the linked activation is a conflict on the execution side.

Stale semantics: if `shipment_version` or the onboard evidence version in the contract no longer matches shipment-service at projection time, projection returns `409 PLAN_STALE` and writes nothing. Agent D's activate-time check is not a substitute for this check.

## Trust

The event is accepted only from the optimizer service identity established inside the platform, not from a browser body. Gateway user JWTs cannot call the projection command.

```text
AGENT_D_CONTRACT_REQUIRED_FOR_IMPLEMENTATION
activation_id
activation_version
activation_status=EXECUTION_LINKED
route_plan_id
route_plan_version
shipment_id
shipment_version
planning_mode
evaluation_fingerprint
supersedes_route_plan_id
supersedes_activation_id
ordered stops and actions as specified above
event network.route_plan.execution_linked
same activation_id replays
unrelated second activation conflicts
PLAN_STALE when shipment or evidence versions moved
```

NLO-0.4C is not implemented in this baseline (`route_plan_activations` is absent). Implementation of projection waits until that contract is actually emitted. This document does not edit NLO ADRs to add the event.

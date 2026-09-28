# Stop action model

```text
STOP_ACTION_MODEL=CHILD_OF_EXECUTION_STOP
ACTION_FSM_REQUIRED=YES
ONBOARD_CARGO_DELIVERY_ONLY=YES
STOP_COMPLETION_RULE=ALL_ACTIONS_RESOLVED
RAW_LOAD_OPPORTUNITY_EXECUTABLE=NO
EXECUTION_SUBJECT_MATERIALIZATION_REQUIRED=YES
```

One stop has many actions. Each action names one materialized shipment and one cargo. Confirmed onboard cargo has a delivery only.

## Materialization

`transport.shipment_cargo_execution_evidence` requires `shipment_id` and `cargo_id`. A RoutePlan action may still say `subject_type=LOAD_OPPORTUNITY`. That id is not evidence and is not an action row.

```text
LOAD_OPPORTUNITY
    → commercial materialization in shipment-service
    → shipment_id + cargo_id
    → TransportExecutionAction
```

Materialization is the existing creation of a shipment and cargo, completed before projection. The projection command does not insert a shipment for a marketplace load. If any cargo action arrives without `execution_shipment_id` and `cargo_id`, the command returns `409 EXECUTION_SUBJECT_UNMATERIALIZED` and writes no route, stop, or action. No driver command may append cargo evidence for a raw load opportunity.

`SHIPMENT_CARGO` still must carry the same execution shipment and cargo fields. The planning subject id is only a reference.

## Fields

| Field | Rule |
| --- | --- |
| `id` | Allocated by shipment-service |
| `execution_stop_id` | Parent stop |
| `shipment_id` | Required. The materialized shipment. Not moved to `operating_tenant_id` |
| `shipment_tenant_id` | Required. Original owner tenant. Evidence is written in that tenant |
| `cargo_id` | Required. The materialized cargo |
| `cargo_version` | Version copied from the contract |
| `route_subject_type` | `LOAD_OPPORTUNITY` or `SHIPMENT_CARGO`, reference only |
| `route_subject_id` | Planning subject id, reference only |
| `action_type` | `PICKUP` or `DELIVERY` |
| `ordinal` | Order inside the stop. Pickup of a cargo is before delivery of that cargo when both share the stop |
| `status` | Action FSM |
| `evidence_id` | Set when an append-only cargo evidence row is written for this `shipment_id` and `cargo_id` |
| `completed_at` | First completion time |
| `version` | Optimistic concurrency |

The stable action has no `source_route_plan_action_id`. Lineage is `TransportExecutionRevisionAction`:

| Link field | Rule |
| --- | --- |
| `revision_id` | Revision that names the action |
| `action_id` | Stable execution action |
| `source_route_plan_action_id` | Action id from that revision's RoutePlan only |
| `source_action_ordinal` | Ordinal on that RoutePlan stop |
| `membership` | `INTRODUCED`, `INHERITED_COMPLETED`, `INHERITED_IN_SERVICE`, or `SUPERSEDED` |

A successor plan's action id is inserted on a new link. The stable action row is not rewritten. `STABLE_ACTION_SOURCE_ID_REWRITTEN=NO`. `SUCCESSOR_ROUTEPLAN_ACTION_LINEAGE_PRESERVED=YES`.

## In-service comparison

`409 IN_SERVICE_STOP_CONFLICT` compares the successor contract with the stable stop and its not-yet-completed actions. The successor's own `source_route_plan_stop_id` and `source_route_plan_action_id` are not expected to equal the introducing plan's ids. Match uses the semantic fingerprint: `stop_role`, `point_kind`, `location_id` or the anchor coordinates, and each incomplete action's `action_type`, `shipment_tenant_id`, `shipment_id`, `cargo_id`, and `cargo_version`. If no successor stop carries that fingerprint, or the fingerprint differs, projection fails and the old revision stays `ACTIVE`. The successor source ids are stored only on the new revision links.

## Action FSM

Action-level state is required because a stop can hold pickup of cargo A, pickup of cargo B, and delivery of cargo C, including cargos from different participant shipments. Today's shipment status can represent only one shipment's coarse fact.

`IN_PROGRESS` is not a status. Starting service is the stop command `StartStopService`. An action completes in one driver confirmation.

```text
PENDING
COMPLETED
FAILED
CANCELLED
```

| From | To | Actor | Command | Evidence | Idempotent | Retry |
| --- | --- | --- | --- | --- | --- | --- |
| `PENDING` | `COMPLETED` | Driver on the route | `ConfirmPickup` or `ConfirmDelivery` | Append-only cargo evidence on the action's shipment. Pickup writes `CONFIRMED_ONBOARD`. Delivery writes `UNLOADED` | Yes. Second confirm does not insert a second evidence row for the same action | Replay returns the action and the original evidence id |
| `PENDING` | `FAILED` | Driver on the route | `FailAction` | Driver exception category already allowed, reason required | Yes. First failure wins | Replay returns `FAILED` |
| `PENDING` | `CANCELLED` | System or operator | Supersede of a not-started action, or cancel of that participant | Link to successor or cancel reason | Yes | Does not cancel `COMPLETED` |

No transition leaves `COMPLETED`. A driver confirm that names a `LOAD_OPPORTUNITY` id and no shipment is rejected with `EXECUTION_SUBJECT_UNMATERIALIZED`.

## Completion rule

```text
STOP_COMPLETION_RULE=ALL_ACTIONS_RESOLVED
```

The stop may move to `COMPLETED` only when every action is `COMPLETED`, `FAILED`, or `CANCELLED`.

| Outcome | Stop | Shipment coarse status |
| --- | --- | --- |
| All actions `COMPLETED` | `COMPLETED` | Each participant follows `SHIPMENT_FSM_ALIGNMENT.md` for its own actions only |
| Mix of `COMPLETED` and `FAILED` | `COMPLETED` with `status_reason=PARTIAL` | A participant does not become `DELIVERED` because another shipment's action failed. Problem event is emitted |
| Any action still `PENDING` | Stays `SERVICE_STARTED` | Unchanged |

## Onboard rule

| Evidence at projection for that shipment and cargo | Actions created |
| --- | --- |
| `CONFIRMED_ONBOARD` | `DELIVERY` only |
| `UNLOADED` | None |
| `PLANNED`, `PICKED_UP`, or no row, and the plan has pickup and delivery | Both, on the stops the plan names |
| Plan omits pickup for onboard cargo | Do not invent one |
| Subject still only a `LOAD_OPPORTUNITY` | Do not create an action. Block projection |

`PICKED_UP` without `CONFIRMED_ONBOARD` is not treated as onboard. Execution does not upgrade evidence during projection.

## Documents

Proof-of-delivery upload stays on the existing document flow for the participant shipment. When today's POD gate applies, it applies to the final delivery action that moves that shipment to `DELIVERED`, not to every stop on the route.

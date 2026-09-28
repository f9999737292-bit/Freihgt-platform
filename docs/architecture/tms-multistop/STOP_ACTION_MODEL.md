# Stop action model

```text
STOP_ACTION_MODEL=CHILD_OF_EXECUTION_STOP
ACTION_FSM_REQUIRED=YES
ONBOARD_CARGO_DELIVERY_ONLY=YES
STOP_COMPLETION_RULE=ALL_ACTIONS_RESOLVED
```

One stop has many actions. One cargo has a pickup and a delivery when it is not yet onboard. Confirmed onboard cargo has a delivery only.

## Fields

| Field | Rule |
| --- | --- |
| `id` | Shipment-owned |
| `execution_stop_id` | Parent stop |
| `shipment_id` | Tenant-scoped predicate |
| `source_route_plan_action_id` | External reference only |
| `cargo_id` | Cargo the action moves |
| `cargo_version` | Version copied from the activation contract |
| `action_type` | `PICKUP` or `DELIVERY` |
| `ordinal` | Order inside the stop. Pickup of a cargo is before delivery of that cargo when both share the stop |
| `status` | Action FSM |
| `evidence_id` | Set when an append-only cargo evidence row is written |
| `completed_at` | First completion time |
| `version` | Optimistic concurrency |

## Action FSM

Action-level state is required because a stop can hold pickup of cargo A, pickup of cargo B, and delivery of cargo C. Today's shipment status can represent only one of those facts.

`IN_PROGRESS` is not a status. Starting service is the stop command `StartStopService`. An action completes in one driver confirmation, matching today's `PICKUP_COMPLETED` and `DELIVERY_COMPLETED`.

```text
PENDING
COMPLETED
FAILED
CANCELLED
```

| From | To | Actor | Command | Evidence | Idempotent | Retry |
| --- | --- | --- | --- | --- | --- | --- |
| `PENDING` | `COMPLETED` | Assigned driver | `ConfirmPickup` or `ConfirmDelivery` | Append-only cargo evidence. Pickup writes `CONFIRMED_ONBOARD`. Delivery writes `UNLOADED`. `occurred_at` | Yes. Second confirm does not insert a second evidence row for the same action | Replay returns the action and the original evidence id |
| `PENDING` | `FAILED` | Assigned driver | `FailAction` | Driver exception category already allowed on the shipment, comment optional, reason required | Yes. First failure wins | Replay returns `FAILED` |
| `PENDING` | `CANCELLED` | System or operator | Plan supersede of a not-started action, or shipment cancel | Link to successor or cancel reason | Yes | Does not cancel `COMPLETED` |

No transition leaves `COMPLETED`. `FAILED` is terminal for that action. Recovery is a successor plan or an operator decision, not an in-place reset.

## Completion rule

```text
STOP_COMPLETION_RULE=ALL_ACTIONS_RESOLVED
```

The stop may move to `COMPLETED` only when every action is `COMPLETED`, `FAILED`, or `CANCELLED`.

| Outcome | Stop | Shipment coarse status |
| --- | --- | --- |
| All actions `COMPLETED` | `COMPLETED` | Follow `SHIPMENT_FSM_ALIGNMENT.md` |
| Mix of `COMPLETED` and `FAILED` | `COMPLETED` with `status_reason=PARTIAL` | Does not become `DELIVERED`. Problem event is emitted |
| Any action still `PENDING` | Stays `SERVICE_STARTED` | Unchanged |

A failed action is visible as the action status plus the existing driver exception record, with `execution_stop_id` and `action_id` added to that record in the future wave. No parallel exception table.

## Onboard rule

Projection input includes `evidence_state` from the activation contract.

| Evidence at projection | Actions created |
| --- | --- |
| `CONFIRMED_ONBOARD` | `DELIVERY` only |
| `UNLOADED` | None |
| `PLANNED`, `PICKED_UP`, or no row, and the plan has pickup and delivery | Both, on the stops the plan names |
| Plan omits pickup for onboard cargo | Do not invent one |

`PICKED_UP` without `CONFIRMED_ONBOARD` is not treated as onboard. The plan must still show the pickup the optimizer was allowed to emit. Execution does not upgrade evidence during projection.

## Documents

Proof-of-delivery upload stays on the existing document flow. `DELIVERY` completion does not require a new document aggregate. When today's POD gate applies, it applies to the final delivery action that moves the shipment to `DELIVERED`, not to every intermediate delivery.

# Execution stop model

```text
EXECUTION_STOP_MODEL=SHIPMENT_OWNED_CHILD
EXECUTION_STOP_ID_SHIPMENT_OWNED=YES
ROUTEPLAN_STOP_REFERENCE=source_route_plan_stop_id
COMPLETED_STOP_IMMUTABLE=YES
STOP_FSM_FROZEN=YES
ORDER_ENFORCEMENT=CONTROLLED_OVERRIDE
```

## Fields

| Field | Rule |
| --- | --- |
| `id` | Shipment-owned primary key |
| `execution_plan_id` | Parent plan |
| `shipment_id` | Denormalized tenant-scoped parent; every query predicates `tenant_id` and `shipment_id` |
| `source_route_plan_stop_id` | Immutable external reference. Not a foreign key into optimizer tables |
| `ordinal` | Copied from the activated plan. Not renumbered after insert |
| `stop_role` | `START`, `CARGO`, or `END`, copied from the plan |
| `point_kind` | `CANONICAL_LOCATION` or `POSITION_ANCHOR` |
| `location_id` | Null only when `point_kind=POSITION_ANCHOR` |
| `latitude`, `longitude` | Copied routing coordinates. Not a live GPS fix |
| `planned_arrival`, `planned_departure` | Planning evidence. Live ETA is a different field, owned by tracking and not stored here as a replacement |
| `service_duration_seconds` | Copied when the plan has it. Null stays null. Zero is not invented |
| `status` | Stop FSM below |
| `status_reason` | Required for `SKIPPED` and for completion when any action failed |
| `version` | Optimistic concurrency |
| `arrived_at`, `service_started_at`, `completed_at` | Actual timestamps. First writer wins |
| `created_at`, `updated_at` | Server clocks |

`START` with `POSITION_ANCHOR` is stored so the projection matches the plan. It is not a driver task. Driver-visible stops are `CARGO` stops and an `END` stop whose `location_id` differs from the last cargo stop.

## Stop FSM

Derived from today's driver commands, not from a greenfield list. `EN_ROUTE` is not a stored stop status. Travel is the shipment status `IN_TRANSIT` plus the next `PLANNED` stop. NLO-0.4A already treats `EN_ROUTE` as unnecessary for the first execution slice. `SERVICE_STARTED` is required because `LOADING_STARTED` and `UNLOADING_STARTED` already exist as distinct operations from arrival and from completion.

```text
PLANNED
ARRIVED
SERVICE_STARTED
COMPLETED
CANCELLED
SKIPPED
```

`SKIPPED` is not a driver button. It is the controlled-override outcome.

### Transitions

| From | To | Actor | Command | Evidence | Idempotent | Retry |
| --- | --- | --- | --- | --- | --- | --- |
| `PLANNED` | `ARRIVED` | Assigned driver | `ArriveStop` | `occurred_at`, idempotency key, stop is current | Yes. Same key replays. A second key does not move `arrived_at` | Safe replay returns the stop |
| `ARRIVED` | `SERVICE_STARTED` | Assigned driver | `StartStopService` | `occurred_at` | Yes. First `service_started_at` wins | Safe replay |
| `SERVICE_STARTED` | `COMPLETED` | Assigned driver | `CompleteStop` | Every action resolved. See action rule | Yes once `COMPLETED` | Replay returns the completed stop. It does not append evidence again |
| `PLANNED` | `SKIPPED` | Operator | `SkipStop` | Reason code, audit actor | Yes for the same stop | Replay returns `SKIPPED` |
| `PLANNED` | `CANCELLED` | System on shipment cancel, or operator | `CancelRemainingStops` | Shipment cancel or operator reason | Yes | Replay does not cancel a `COMPLETED` stop |
| `ARRIVED` or `SERVICE_STARTED` | `CANCELLED` | Operator only | `CancelInServiceStop` | Reason required | Yes | Driver cannot cancel an in-service stop by repeating arrive |

No transition leaves `COMPLETED`. No transition leaves `SKIPPED` or `CANCELLED` back to `PLANNED`.

`CompleteStop` is accepted only when `STOP_COMPLETION_RULE` is satisfied. The driver completes each action first; stop completion is the gate that all actions are resolved.

## Order

```text
ORDER_ENFORCEMENT=CONTROLLED_OVERRIDE
```

Default: the current stop is the lowest ordinal on the active plan that is still `PLANNED`, `ARRIVED`, or `SERVICE_STARTED`. The driver may issue arrive, start, and action commands only for that stop. Completing ordinal N+1 while N is open returns `409 STOP_NOT_CURRENT`.

Override: an operator in the shipment tenant, recorded as actor `OPERATOR`, may authorize `SkipStop` or `ArriveStop` on a later stop. The command requires a reason code from the existing driver exception set. The audit row stores actor, reason, and the skipped ordinals. The outbox event is `shipment.route_stop.sequence_overridden`. The driver cannot set the override flag. There is no silent reorder and no client-supplied ordinal rewrite.

## Current stop

While the driver is between stops, the current stop is the next `PLANNED` cargo stop. `ARRIVED` and `SERVICE_STARTED` mean that stop is in service. Those two statuses are the replan lock. See `REPLAN_SUCCESSOR_MODEL.md`.

## Immutability

A `COMPLETED` stop's ordinal, location, planned times, actual timestamps, and action results do not change. A replan inserts a successor plan. It does not `UPDATE` the completed row's facts. `updated_at` may change only to record `status` moving from an open state, never to rewrite a completed row.

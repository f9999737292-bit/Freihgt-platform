# Execution stop model

```text
EXECUTION_STOP_MODEL=CHILD_OF_TRANSPORT_EXECUTION
EXECUTION_STOP_PARENT=TransportExecution
EXECUTION_STOP_ID_ALLOCATED_BY=shipment-service
EXECUTION_STOP_ID_SHIPMENT_ROW_OWNED=NO
ROUTEPLAN_STOP_REFERENCE=source_route_plan_stop_id
COMPLETED_STOP_IMMUTABLE=YES
COMPLETED_STOP_ROW_REPARENTED=NO
IN_SERVICE_STOP_ROW_REPARENTED=NO
STOP_FSM_FROZEN=YES
ORDER_ENFORCEMENT=CONTROLLED_OVERRIDE
```

The stop is not a child of a revision. `TransportExecutionRevisionStop` records that a revision includes the stop. Inserting that link does not change the stop's parent.

## Fields

| Field | Rule |
| --- | --- |
| `id` | Allocated by shipment-service. Stable for the life of the row |
| `execution_id` | Parent route. Set once. Never updated |
| `tenant_id` | Route tenant. Every query predicates it |
| `source_route_plan_stop_id` | Immutable external reference from the revision that introduced the stop. Not a foreign key into optimizer tables |
| `ordinal` | Copied when the stop is introduced. Not renumbered later |
| `stop_role` | `START`, `CARGO`, or `END` |
| `point_kind` | `CANONICAL_LOCATION` or `POSITION_ANCHOR` |
| `location_id` | Null only when `point_kind=POSITION_ANCHOR` |
| `latitude`, `longitude` | Copied routing coordinates. Not a live GPS fix |
| `planned_arrival`, `planned_departure` | Planning evidence. Live ETA does not replace them |
| `service_duration_seconds` | Copied when the plan has it. Null stays null. Zero is not invented |
| `status` | Stop FSM below |
| `status_reason` | Required for `SKIPPED` and for completion when any action failed |
| `version` | Optimistic concurrency |
| `arrived_at`, `service_started_at`, `completed_at` | Actual timestamps. First writer wins |
| `created_at`, `updated_at` | Server clocks |

There is no `execution_revision_id` on this row. Membership lives in `TransportExecutionRevisionStop` with `revision_id`, `stop_id`, and `membership` of `INTRODUCED`, `INHERITED_COMPLETED`, `INHERITED_IN_SERVICE`, or `SUPERSEDED`.

`START` with `POSITION_ANCHOR` is stored so the projection matches the plan. It is not a driver task. Driver-visible stops are `CARGO` stops and an `END` stop whose `location_id` differs from the last cargo stop.

## Stop FSM

Derived from today's driver commands. `EN_ROUTE` is not a stored stop status. Travel is the next `PLANNED` stop on the active revision. Participant shipments that are already in transit stay `IN_TRANSIT`. `SERVICE_STARTED` exists because `LOADING_STARTED` and `UNLOADING_STARTED` are already distinct from arrival and from completion.

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
| `PLANNED` | `ARRIVED` | Driver on the route | `ArriveStop` | `occurred_at`, idempotency key, stop is current on the active revision | Yes. Same key replays. A second key does not move `arrived_at` | Safe replay returns the stop |
| `ARRIVED` | `SERVICE_STARTED` | Driver on the route | `StartStopService` | `occurred_at` | Yes. First `service_started_at` wins | Safe replay |
| `SERVICE_STARTED` | `COMPLETED` | Driver on the route | `CompleteStop` | Every action resolved | Yes once `COMPLETED` | Replay returns the completed stop. It does not append evidence again |
| `PLANNED` | `SKIPPED` | Operator | `SkipStop` | Reason code, audit actor | Yes for the same stop | Replay returns `SKIPPED` |
| `PLANNED` | `CANCELLED` | System on supersede or participant cancel, or operator | `CancelRemainingStops` | Supersede or operator reason | Yes | Replay does not cancel a `COMPLETED` stop |
| `ARRIVED` or `SERVICE_STARTED` | `CANCELLED` | Operator only | `CancelInServiceStop` | Reason required | Yes | The driver cannot cancel an in-service stop by repeating arrive |

No transition leaves `COMPLETED`. No transition leaves `SKIPPED` or `CANCELLED` back to `PLANNED`. A `COMPLETED` or in-service transition does not change `execution_id`.

## Order

```text
ORDER_ENFORCEMENT=CONTROLLED_OVERRIDE
```

Default: the current stop is the lowest ordinal on the active revision that is still `PLANNED`, `ARRIVED`, or `SERVICE_STARTED`. The driver may issue arrive, start, and action commands only for that stop. Completing ordinal N+1 while N is open returns `409 STOP_NOT_CURRENT`.

Override: an operator in the route tenant, recorded as actor `OPERATOR`, may authorize `SkipStop` or `ArriveStop` on a later stop. The command requires a reason code from the existing driver exception set. The audit row stores actor, reason, and the skipped ordinals. The outbox event is `shipment.route_stop.sequence_overridden`. The driver cannot set the override flag. There is no silent reorder.

## Immutability

A `COMPLETED` stop's id, parent, ordinal, location, planned times, actual timestamps, and action results do not change. A replan adds a link row. It does not `UPDATE` those facts and it does not re-parent the row.

# Failure and recovery

Behavior is deterministic. The first committed fact wins. Retries return that fact.

```text
DUPLICATE_COMMAND_POLICY=STATE_IDEMPOTENT_FIRST_TIMESTAMP_WINS
OUT_OF_ORDER_EVENT_POLICY=REJECT_UNLESS_ALREADY_APPLIED
OFFLINE_DRIVER_POLICY=QUEUE_THEN_SERVER_ORDER
WRONG_STOP_POLICY=REJECT_WITHOUT_OPERATOR_OVERRIDE
```

| Situation | Behavior |
| --- | --- |
| Driver offline | Client may queue commands with idempotency keys. Server applies only commands that are valid at arrival time. Same key replays the stored response |
| Duplicate `ARRIVED` | If the stop is already `ARRIVED` or later, return the stop. Do not change `arrived_at` |
| Duplicate `COMPLETE` | If the action or stop is already `COMPLETED`, return it. Do not insert a second cargo evidence row |
| Out-of-order events | A command for a stop that is not current returns `409 STOP_NOT_CURRENT`. An `occurred_at` that would rewrite a completed stop is rejected. Replay of an already applied key returns the stored response |
| GPS unavailable | Tracking goes `stale` or `lost`. Manual arrive and complete still work. No automatic arrival |
| Driver completes the wrong stop | `409 STOP_NOT_CURRENT`. No status change |
| Driver skips a future stop | Driver command rejected. Operator `SkipStop` with reason writes `SKIPPED`, audit, and `shipment.route_stop.sequence_overridden` |
| Shipment cancelled | When today's cancel rules allow it, status becomes `CANCELLED`. `COMPLETED` stops stay. Open stops become `CANCELLED`. Open driver stop tasks are cancelled |
| Vehicle changed | Existing assign-vehicle command. Open `PLANNED` tasks take the new vehicle id. An in-service stop keeps its recorded vehicle until complete |
| Driver changed | Existing assign-driver command. `PLANNED` tasks move to the new driver. An in-service stop stays with the driver who arrived unless an operator transfer records a reason |
| RoutePlan superseded | Ignored by execution until an `EXECUTION_LINKED` successor activation arrives |
| Activation replaced | Successor rules in `REPLAN_SUCCESSOR_MODEL.md`. Unrelated activation is `409 EXECUTION_PLAN_CONFLICT` |
| Control Tower consumer delayed | Shipment commit is already durable. The read model catches up by event version. It does not grant execution rights |
| Event replay | Outbox replay resends the same event id. Consumers treat duplicate event ids as already applied, which is the existing inbox behavior |

Optimistic concurrency uses the stop or action `version`. A stale version is `409` and does not apply. The client retries by reading the current stop, not by forcing the write.

Partial action failure stays on the action as `FAILED` with an exception. The stop completes with `PARTIAL` only after every action is resolved. Shipment status does not jump to `DELIVERED`.

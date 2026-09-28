# Control Tower integration

```text
CONTROL_TOWER_OWNER=control-tower-read-model-service
CONTROL_TOWER_REUSE=PARTIAL
CONTROL_TOWER_STOP_AWARE=NO
```

## What is reused

The shipment status projection, inbox dedup, version-gap handling, operator cases, risk, and automation stay. They continue to consume:

```text
shipment.created
shipment.status.changed
shipment.cancelled
shipment.ready_for_billing
shipment.documents_completed
shipment.financially_closed
```

Driver problem ingestion stays. `driver.exception_reported` remains mapped to `driver.problem.reported`. Delay and problem events gain an optional `execution_stop_id` and `action_id`. Existing consumers that ignore unknown fields keep working. A problem on a stop still opens the existing driver-problem path.

Do not encode "arrived at stop 3" as a fake shipment status. That would reset or skip the coarse FSM.

## New projection

A separate read model, keyed by `execution_id`, `revision_id`, and `execution_stop_id`, consumes shipment-service execution events from `transport.shipment_event_outbox`. It does not consume tracking approach events from that table. `tracking.stop.approaching` arrives on the tracking-owned publication. The read model does not share the shipment status version counter.

```text
STOP_PROGRESS_EVENTS=shipment.execution_plan.created, shipment.execution_plan.superseded, shipment.route_stop.current, shipment.route_stop.arrived, shipment.route_stop.service_started, shipment.route_stop.completed, shipment.route_stop.sequence_overridden
```

Delay and problem progress reuse `driver.delay.reported` and `driver.problem.reported` with the stop id. Approaching progress is `tracking.stop.approaching` from tracking-service, advisory only.

## Replan

```text
REPLAN_EVENT_MODEL=shipment.execution_plan.superseded
```

Control Tower marks the previous revision's remaining open stops as superseded and shows the successor's open stops. Completed stop rows stay on the route and are linked, not moved. Optimizer event `network.route_plan.superseded` is planning audit when Agent D emits it. Operational screens follow the shipment-service execution event, not a tracking insert into the shipment outbox.

A slow consumer applies events by plan version. A superseded plan event that arrives before the created event is held until the create is applied or the gap policy of that new projection records the gap. It must not delete completed stop history.

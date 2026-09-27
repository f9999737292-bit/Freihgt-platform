# NLO-0.4A execution boundary

Status: proposed. No shipment mutation in this discovery. No driver-app change.

```text
PLAN_ACCEPT_SEPARATE_FROM_ACTIVATE=YES
ACTIVE_PLAN_DIRECT_EDIT_ALLOWED=NO
REPLAN_CREATES_SUCCESSOR=YES
COMPLETED_EXECUTION_HISTORY_IMMUTABLE=YES
```

## Accept

Accept is a carrier choice of one `EVALUATED` plan. It checks `route_plan_dependencies` against current trusted versions. On match it writes `ACCEPTED` and `accepted_at` once. The stop list, actions, legs, and capacity snapshots do not change. A repeated accept with the same `Idempotency-Key` and the same body returns the accepted plan. A changed body with that key is a conflict, which is the existing network-optimizer rule.

Accept does not book a slot, assign a driver, or create a driver task.

## Activate

Activate is a request to link an `ACCEPTED` plan to shipment execution. Preconditions, all read from trusted services:

- Plan status is `ACCEPTED` and the idempotency record is free or a replay.
- Shipment version, load versions, and capacity version still match the dependency rows. Otherwise `409` `PLAN_STALE`.
- Current-trip context fingerprint still matches when `planning_mode=CURRENT_TRIP`.
- Routing fingerprints are still inside `ExpiresAt`. An expired leg is `ROUTING_UNAVAILABLE`, not a silent refresh inside activate.
- Cargo compatibility snapshots are not `INDETERMINATE` for a plan that is being activated. Indeterminate plans may be accepted as advice. They may not be activated.
- Shipment status is one of the pre-pickup execution states the shipment machine already allows (`DRIVER_ASSIGNED` or earlier states that still lead there). Discovery does not add a transition.
- Slot validity is the shipment's own `PICKUP_SLOT_BOOKED` or `DELIVERY_SLOT_BOOKED` when that shipment already requires it. The optimizer does not reserve a slot.

The activation row is append-only: `PENDING_EXECUTION`, then `EXECUTION_LINKED` or `REJECTED`. A second activate with the same key replays the row. A second activate with a new key after `EXECUTION_LINKED` returns the existing link and does not create another driver task. That is the double-activation rule.

NLO-0.4A does not implement the shipment write. NLO-0.4D is the proposed wave that would perform it.

## What the driver sees

Not an optimizer candidate. Shipment-service would later project:

```text
DriverRouteTask
  plan_id, plan_version, shipment_id
DriverStopTask
  ordinal, location_id, planned_arrival
  actions (pickup or delivery, documents)
  status
```

Today's `DriverTask` types are notices and confirmations. They stay. Stop tasks are a new shipment-owned type, not a new optimizer table.

Minimum execution states for a stop task, later:

```text
PLANNED
ARRIVED
COMPLETED
CANCELLED
```

`EN_ROUTE`, `STARTED`, and `SKIPPED` are not required for the first execution slice. Arrival and completion carry actual timestamps. Proof of pickup and proof of delivery stay on the shipment evidence model NLO-0.3C already uses. The optimizer does not store them.

## Replan

An accepted or linked plan is not edited. A delay, a new load, a cancelled load, a routing expiry, or a slot change creates a new plan with `supersedes_plan_id`. The old plan becomes `SUPERSEDED` only after the successor is persisted. Completed stops are copied as an immutable prefix. They are not insertion gaps.

Automatic replanning is not authorized. The contract only says a human or a later wave calls evaluate again.

## Events (catalog only)

Dotted names, consistent with `shipment.created` and `shipment.status.changed`. Not emitted by this discovery.

Planning, owned by the optimizer outbox:

```text
network.route_plan.evaluated
network.route_plan.accepted
network.route_plan.activation_requested
network.route_plan.superseded
network.route_plan.cancelled
```

Execution, owned by shipment-service if a later wave adds them:

```text
shipment.route_stop.arrived
shipment.route_stop.completed
```

## Owners

| Fact | Owner |
| --- | --- |
| Evaluated and accepted plan | `network-optimizer-service` |
| Shipment status | `shipment-service` |
| Driver task and stop completion | `shipment-service` |
| Tracking position and ETA freshness | `tracking-service` |
| Commercial order | `transport-order-service` |
| Slot reservation | Not owned by the optimizer. Shipment statuses already name a booked slot. No slot service exists |

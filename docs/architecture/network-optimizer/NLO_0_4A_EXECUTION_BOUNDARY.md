# NLO-0.4A execution boundary

Status: accepted. Architecture frozen. No shipment mutation in this acceptance. No driver-app change.

```text
ARCHITECTURE_FROZEN=YES
CURRENT_TRIP_REPLAN_MODEL=ACCEPTED
PLAN_ACCEPT_SEPARATE_FROM_ACTIVATE=YES
ACTIVE_PLAN_DIRECT_EDIT_ALLOWED=NO
PLAN_STRUCTURE_IMMUTABLE_AFTER_EVALUATION=YES
LIFECYCLE_STATUS_MUTABLE=YES
REPLAN_CREATES_SUCCESSOR=YES
COMPLETED_EXECUTION_HISTORY_IMMUTABLE=YES
CURRENT_TRIP_ACTIVATION_RESETS_SHIPMENT_STATUS=NO
```

## Accept

Accept is a carrier choice of one `EVALUATED` plan. It checks `route_plan_dependencies` against current trusted versions. On match it writes `ACCEPTED` and `accepted_at` once. Stops, actions, legs, capacity snapshots, dependencies, and fingerprints do not change. A repeated accept with the same `Idempotency-Key` and the same body returns the accepted plan. A changed body with that key is a conflict, which is the existing network-optimizer rule.

Accept does not book a slot, assign a driver, or create a driver task.

## Activate

Activate is a request to link an `ACCEPTED` plan to shipment execution. Preconditions, all read from trusted services:

- Plan status is `ACCEPTED` and the idempotency record is free or a replay.
- Shipment version, load versions, and capacity version still match the dependency rows. Otherwise `409` `PLAN_STALE`.
- Current-trip context fingerprint still matches when `planning_mode=CURRENT_TRIP`.
- Routing fingerprints are still inside `ExpiresAt`. An expired leg is `ROUTING_UNAVAILABLE`, not a silent refresh inside activate.
- Cargo compatibility snapshots are not `INDETERMINATE` for a plan that is being activated. Indeterminate plans may be accepted as advice. They may not be activated. Unknown cargo service duration makes the plan indeterminate, so activation is refused.
- Shipment status is in the set for that planning mode. Discovery does not add a transition and does not move a current-trip shipment backward.

`DEPOT_START` is pre-execution. The allowed statuses are the machine states before cargo handling begins (`allowedStatusTransitions` in `shipment.go`):

```text
DEPOT_START_ACTIVATION_STATUS_SET=CARRIER_ASSIGNED,ACCEPTED_BY_CARRIER,VEHICLE_ASSIGNED,DRIVER_ASSIGNED,PICKUP_SLOT_BOOKED
```

`CURRENT_TRIP` is replanning of a trip that has already entered cargo execution. `LOADED` and `IN_TRANSIT` are required. `IN_PICKUP` is included because the machine has already left `PICKUP_SLOT_BOOKED` and the status must not be reset. `DRIVER_ASSIGNED` and `PICKUP_SLOT_BOOKED` stay on the depot-start set. `ARRIVED_AT_CONSIGNEE`, `UNLOADING`, and every later status are near-terminal or terminal on today's single-destination chain and are excluded. `DELIVERY_SLOT_BOOKED` is a named constant and is not a target in `allowedStatusTransitions`, so it is in neither set.

```text
CURRENT_TRIP_ACTIVATION_STATUS_SET=IN_PICKUP,LOADED,IN_TRANSIT
CURRENT_TRIP_LOADED_SUPPORTED=YES
CURRENT_TRIP_IN_TRANSIT_SUPPORTED=YES
CURRENT_TRIP_ACTIVATION_RESETS_SHIPMENT_STATUS=NO
```

Activation still requires a valid trusted current-trip context and valid onboard evidence. Slot validity, when the shipment already has it, remains that shipment's booked-slot status. The optimizer does not reserve a slot.

The activation row is append-only: public activate persists `PENDING_EXECUTION`. `EXECUTION_LINKED` or `REJECTED` are later transitions. A second activate with the same key replays the row. A new key while any activation already exists returns that row and does not create another one. `EXECUTION_LINKED` requires stored `execution_id` and `execution_revision_id`. Public activate does not create that linkage and does not create a driver task.

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

An accepted or linked plan is not edited. A delay, a new load, a cancelled load, a routing expiry, or a slot change creates a successor. Completed stops stay an immutable prefix. They are not insertion gaps. Automatic replanning is not authorized.

Current-trip successor order:

```text
1. evaluate successor
2. accept successor
3. validate current trusted trip state
4. activate successor as `PENDING_EXECUTION`
5. execution projection switches future stops
6. previous plan becomes superseded only after that projection commits and linkage is stored
```

Public activate is step 4 only. It does not supersede the predecessor and does not clear the previous effective activation. Step 6 belongs to the later projection transaction, which is not implemented in NLO-0.4C. There is no window in which both plans are independently execution-linked. `MAX_EXECUTION_LINKED_PLANS_PER_SHIPMENT=1`. Shipment operational status is not moved backward.

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

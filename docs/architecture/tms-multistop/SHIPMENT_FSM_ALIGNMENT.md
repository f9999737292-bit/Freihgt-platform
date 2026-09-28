# Shipment FSM alignment

```text
SHIPMENT_FSM_ALIGNMENT=PASS
MULTI_STOP_MUST_NOT_RESET_SHIPMENT_STATUS=YES
NEW_SHIPMENT_STATES_ADDED=NO
```

The existing chain in `allowedStatusTransitions` remains the coarse shipment lifecycle. Stop and action state carry intermediate progress. No new shipment status is added for "second pickup" or "partial delivery".

`DELIVERY_SLOT_BOOKED` stays a named constant outside the transition map. This freeze does not insert it into the chain.

## Depot start

Before the first cargo action, status moves only through the existing assignment and slot commands:

```text
CARRIER_ASSIGNED
ACCEPTED_BY_CARRIER
VEHICLE_ASSIGNED
DRIVER_ASSIGNED
PICKUP_SLOT_BOOKED
```

Projecting an execution plan does not set these. They remain assignment and manual slot commands.

## First pickup

| Execution fact | Shipment status |
| --- | --- |
| Arrive at the first cargo pickup stop | `IN_PICKUP` via the existing arrive command |
| First pickup action completed (`CONFIRMED_ONBOARD`) | `LOADED` |
| Driver departs that stop | `IN_TRANSIT` |

If the first stop completes more than one pickup, the shipment becomes `LOADED` when the first of those actions completes and stays `LOADED` until depart. It does not return to `IN_PICKUP`.

## Later pickups

Additional pickup stops run their own stop FSM while the shipment stays `IN_TRANSIT`. They do not move the shipment back to `PICKUP_SLOT_BOOKED`, `IN_PICKUP`, or `LOADED`.

```text
ADDITIONAL_PICKUP_RESETS_STATUS=NO
```

## Partial delivery

A delivery action that is not the last required delivery on the active plan completes the action and the stop under the action rule. Shipment status stays `IN_TRANSIT`. It does not become `ARRIVED_AT_CONSIGNEE`, `UNLOADING`, or `DELIVERED`.

## Final delivery

The final required `DELIVERY` on the active plan uses the existing tail:

| Execution fact | Shipment status |
| --- | --- |
| Arrive at the final delivery stop | `ARRIVED_AT_CONSIGNEE` |
| Start service at that stop | `UNLOADING` |
| Complete the final delivery action | `DELIVERED` |

Later statuses (`DELIVERY_CONFIRMED` through `FINANCIALLY_CLOSED`) are unchanged and are not stop states.

If the final stop has several delivery actions, `DELIVERED` is set when the last required delivery action completes, not when the first one does. Earlier deliveries at that stop leave the shipment in `UNLOADING`.

## What must not happen

| Event | Status effect |
| --- | --- |
| Execution plan created | None |
| Successor activation | None, including no move back to `CARRIER_ASSIGNED` or `DRIVER_ASSIGNED` |
| Intermediate arrive or complete | None while status is `IN_TRANSIT`, except the final delivery tail above |
| Replan while `IN_PICKUP`, `LOADED`, or `IN_TRANSIT` | Status unchanged |
| Cancel shipment | `CANCELLED` when today's cancel rules allow it. Completed stop rows stay |

`ARRIVED_AT_CONSIGNEE` remains the final consignee arrival, not "arrived at some stop".

## Shipments without a plan

Driver commands keep today's one-step status map. The multi-stop gate applies only when an `ACTIVE` execution plan exists.

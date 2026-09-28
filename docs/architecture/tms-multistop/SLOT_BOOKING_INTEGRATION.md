# Slot window integration

```text
SLOT_BOOKING_REUSABLE=PARTIAL
SLOT_WINDOW_RECORD_OWNER=tracking-service
SLOT_BOOKING_AUTHORITY=NOT_IMPLEMENTED
SLOT_STATUS_EFFECT_OWNER=shipment-service
NETWORK_OPTIMIZER_BOOKS_SLOT=NO
SLOT_REFERENCE_MODEL=OPTIONAL_SLOT_REVISION_ON_EXECUTION_STOP
```

## Evidence

No slot-booking service exists. Nothing in the repository is an authoritative reservation of a dock or a yard. `PICKUP_SLOT_BOOKED` is a shipment status reached only from `DRIVER_ASSIGNED` by the manual transition. `DELIVERY_SLOT_BOOKED` is not in `allowedStatusTransitions`.

`tracking.shipment_slot_revision` and `tracking.shipment_slot_state` store an observed window for `slot_type` `pickup` or `delivery`: start, end, facility, location, an observed status word (`proposed`, `booked`, `confirmed`, `cancelled`, `completed`, `missed`), source, and provider slot id. That word records what a source reported. It does not make tracking-service the system that books or reserves the slot. The grain is one shipment and one slot type, not a multi-stop reservation, and it is not an optimizer table.

## Ownership

```text
SLOT_WINDOW_RECORD_OWNER=tracking-service
SLOT_BOOKING_AUTHORITY=NOT_IMPLEMENTED
SLOT_STATUS_EFFECT_OWNER=shipment-service
NETWORK_OPTIMIZER_BOOKS_SLOT=NO
```

`tracking-service` owns the observed window record. `shipment-service` owns whether a participant shipment's coarse status is `PICKUP_SLOT_BOOKED`, and it owns an optional reference from an execution stop to a slot revision id. `network-optimizer-service` does not create, confirm, or cancel slots. ADR-NET-021 already forbids planning from reserving a slot.

Calling the tracking row a booking owner would invent an authority the tables do not have.

## Attachment

An execution stop may store:

```text
slot_revision_id
slot_type
window_start
window_end
```

The ids and times are copies of an observed tracking revision. They are references. The stop does not become a reservation. `service_duration_seconds` on the stop stays the planning copy. The observed window is a facility constraint. They are different facts.

Until tracking can address a stop, the existing shipment-level pickup and delivery observations remain the only windows. Intermediate stops have a null slot reference. Execution does not block on a missing observation unless that participant shipment is still before `IN_PICKUP` and the existing product rule requires `PICKUP_SLOT_BOOKED`.

A later observation does not rewrite a `COMPLETED` stop. It may be a reason for Agent D to build a successor plan. Execution does not resequence itself from a slot observation.

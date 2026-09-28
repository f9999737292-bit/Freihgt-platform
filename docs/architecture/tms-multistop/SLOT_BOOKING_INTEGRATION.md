# Slot booking integration

```text
SLOT_BOOKING_REUSABLE=PARTIAL
SLOT_BOOKING_OWNER=tracking-service
SLOT_STATUS_EFFECT_OWNER=shipment-service
SLOT_BOOKING_OWNER_IS_NETWORK_OPTIMIZER=NO
SLOT_REFERENCE_MODEL=OPTIONAL_SLOT_REVISION_ON_EXECUTION_STOP
```

## Evidence

No booking service exists. `PICKUP_SLOT_BOOKED` is a shipment status reached only from `DRIVER_ASSIGNED` by the manual transition. `DELIVERY_SLOT_BOOKED` is not in `allowedStatusTransitions`.

`tracking.shipment_slot_revision` and `tracking.shipment_slot_state` store a window for `slot_type` `pickup` or `delivery`: start, end, facility, location, status (`proposed`, `booked`, `confirmed`, `cancelled`, `completed`, `missed`), source, and provider slot id. That is an observation of a window, keyed to one shipment, not a multi-stop reservation and not an optimizer table.

## Ownership

`tracking-service` owns the slot window record. `shipment-service` owns whether execution treats the first pickup as operationally booked (`PICKUP_SLOT_BOOKED`) and owns the optional reference from an execution stop to a slot revision. `network-optimizer-service` does not create, confirm, or cancel slots. ADR-NET-021 already forbids planning from reserving a slot.

## Attachment

An execution stop may store:

```text
slot_revision_id
slot_type
window_start
window_end
```

The ids and times are copied from the tracking revision at projection or when a later slot event names that stop. They are references. The stop does not become the slot aggregate. `service_duration_seconds` on the stop stays the planning copy. The slot window is the facility constraint. They are different facts.

Until tracking can address a stop, the existing shipment-level pickup and delivery slot rows remain the only windows. Intermediate stops have a null slot reference. Execution does not block on a missing slot unless the shipment is still before `IN_PICKUP` and the existing product rule requires `PICKUP_SLOT_BOOKED`.

A slot change after activation does not rewrite a `COMPLETED` stop. It may be a reason for Agent D to build a successor plan. Execution does not resequence itself from a slot event.

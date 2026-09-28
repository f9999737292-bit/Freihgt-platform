# ADR-NET-021: Plan acceptance, activation, and execution

## Status

Accepted. Architecture freeze. NLO-0.4C implements accept and activate. NLO-0.4D is not started. NLO-0.4B does not implement this ADR.

```text
ADR_NET_021=ACCEPTED
NLO_0_4C_ACTIVATION_RELEASE_BLOCKED_UNTIL_SERVICE_DURATION_SOURCE=YES
```

## Context

Choosing a suggested plan is not the same as starting a trip. Shipment status, driver tasks, and slot-named statuses already live in `shipment-service`. No slot-booking service exists. ADR-NET-005 forbids treating a multi-stop plan as executable until that execution model exists.

## Decision

`ACCEPT` sets an evaluated plan to `ACCEPTED` after dependency versions match. It does not reserve a slot or create a driver task. It does not change plan structure.

`ACTIVATE` persists one append-only activation row as `PENDING_EXECUTION` for an accepted plan. The route plan status stays `ACCEPTED`. It checks version freshness (`409` `PLAN_STALE`), routing expiry, and that the plan is not indeterminate. Unknown service duration blocks activation. It does not edit stops and does not call shipment-service. `EXECUTION_LINKED` is reserved for a later transition after Agent D stores `execution_id` and `execution_revision_id` returned by the execution projection. Public activate does not create that linkage and does not emit `network.route_plan.execution_linked`.

Depot-start activation allows only `CARRIER_ASSIGNED`, `ACCEPTED_BY_CARRIER`, `VEHICLE_ASSIGNED`, `DRIVER_ASSIGNED`, and `PICKUP_SLOT_BOOKED`. Current-trip activation allows `IN_PICKUP`, `LOADED`, and `IN_TRANSIT`. It does not move shipment status backward. A successor activation may be `PENDING_EXECUTION` while the previous execution-linked activation stays effective. Public activate does not supersede the predecessor and does not emit `network.route_plan.superseded`. Supersession waits until the successor projection commits. This matches ADR-TMS-003.

```text
PLAN_ACCEPT_SEPARATE_FROM_ACTIVATE=YES
CALLER_SUPPLIED_EXECUTION_STATE_ALLOWED=NO
DOUBLE_ACTIVATION_IDEMPOTENT=YES
PLANNING_RESERVES_SLOT=NO
CURRENT_TRIP_ACTIVATION_RESETS_SHIPMENT_STATUS=NO
DEPOT_START_ACTIVATION_STATUS_SET=CARRIER_ASSIGNED,ACCEPTED_BY_CARRIER,VEHICLE_ASSIGNED,DRIVER_ASSIGNED,PICKUP_SLOT_BOOKED
CURRENT_TRIP_ACTIVATION_STATUS_SET=IN_PICKUP,LOADED,IN_TRANSIT
```

Driver stop tasks, if built later, are shipment-owned projections. Completed stop history is immutable. Replanning creates a successor plan.

Public caller-visible reasons and the idempotency rule are in `NLO_0_4A_EXECUTION_BOUNDARY.md` and `NLO_0_4A_SECURITY_PRIVACY.md`.

## Consequences

NLO-0.4C remediation R1 writes `route_plan_activations` as `PENDING_EXECUTION` and stores nullable `execution_id` and `execution_revision_id` for the future handshake. `TMS_PROJECTION_HANDSHAKE_IMPLEMENTED=NO`. `NETWORK_ROUTE_PLAN_EXECUTION_LINKED_IMPLEMENTED=NO`. `NLO_ACTIVATION_SCHEMA_READY_FOR_FUTURE_HANDSHAKE=YES`. It does not mutate shipment stops. Production activation stays blocked until an authoritative or versioned server-owned service-duration source exists. NLO-0.4D is not started. `NLO_0_4C_ACCEPTED=NO`.

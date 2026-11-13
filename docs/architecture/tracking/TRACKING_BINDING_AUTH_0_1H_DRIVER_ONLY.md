# TRACKING-BINDING-AUTHORITY-DECISION-0.1H — Driver-Only Scope

`DRIVER_ONLY_POLICY=ALLOWED_FOR_DRIVER_MATCH_ONLY`

A null vehicle is a normal shipment state (`000003_create_transport_tables.up.sql:169`) and a normal execution state (`000088_tms_transport_execution_foundation_v0_1a.up.sql:14`). It is not a wildcard.

`VEHICLE_AUTHORIZATION=CLAIMED_VEHICLE_MUST_MATCH_CURRENT_ROWS`

`EXECUTION_SCOPE_AUTHORIZATION=LINK_PLUS_CURRENT_REVISION_AND_DRIVER`

## 1. Five rights

| Right | Driver-only decision | Vehicle-claimed decision |
| --- | --- | --- |
| Submit a driver location for the shipment | Allowed after the current driver assignment matches. | Allowed only when the claimed vehicle also matches the current shipment vehicle. |
| Match the driver to the current shipment | This is the existing check `CanDriverAccessShipment` (`driver_operations.go:271-278`). It does not include a vehicle. | Same driver check, plus the vehicle check. |
| Use the vehicle as a lookup key | Denied. The absent vehicle adds no vehicle id to the scope. | Allowed only for the vehicle on the current shipment row and, for an execution target, the same vehicle on the current execution row. |
| Select execution tracking targets | Allowed only for executions on the active shipment link whose current `driver_id` equals the resolved driver and whose `current_revision_id` is the revision on the fresh check. | Same, plus `vehicle_id` equality. |
| Read coordinates or an ETA | Denied. Location submission is not a coordinate read. `GetState` has no production caller. Stop ETA remains blocked until a named service identity exists. | Same denial. A vehicle match does not open `GetState` or the internal ETA route. |

Approach and ETA processing may use a target that the execution-scope right returned. They may not widen that right into a coordinate API. The target result used by approach stays inside the processing path. It is not returned to the location caller.

## 2. What a missing vehicle does not do

The gateway location call sends an empty vehicle (`services/api-gateway/internal/driver/handler.go:264`). The tracking handler stores `X-Vehicle-ID` when a caller supplies it (`tracking_handler.go:116-123`). That header is not the vehicle right.

A driver-only decision:

- does not match `ListActiveTargetsForActors` on `vehicle_id`
- does not inherit the execution's vehicle as if it had been claimed
- does not fail merely because some other shipment has a vehicle
- does not become vehicle-scoped when the shipment later gains a vehicle; that is a new decision

`actorMatchesTarget` compares driver and vehicle ids with OR (`eta_ingest_service.go:311-327`). The driver-only path must not call that function with a vehicle id taken from the execution or from a header.

## 3. When the driver-only right is withheld

The location right is withheld when the driver row is inactive, the shipment driver differs, the caller is not the gateway user, or the assignment rows cannot be read.

The execution-target right is withheld when there is no active shipment link, the revision changed, the execution driver differs, a required relationship pin is absent, or `owner_epoch` is `ABSENT`. Epoch-dependent effects stay denied until the column exists. The location right does not depend on the epoch.

## 4. Reassignment

A change of `shipment.driver_id` removes both the location right and the execution-target right until a new decision. A change of `shipment.vehicle_id` (`shipment_repository.go:472-475`) does not remove a driver-only decision's driver match. It does remove a vehicle-claimed decision. The old decision id is not reused.

`VEHICLE_REASSIGNMENT_POLICY=CLAIMED_VEHICLE_MUST_MATCH_CURRENT_ROW`

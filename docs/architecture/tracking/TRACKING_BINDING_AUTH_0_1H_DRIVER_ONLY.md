# TRACKING-BINDING-AUTHORITY-DECISION-0.1H — Driver-Only Scope

`DRIVER_ONLY_POLICY=ALLOWED_FOR_DRIVER_MATCH_ONLY`

`DRIVER_ONLY_REVISION_POLICY=REAUTHORIZATION_REQUIRED`

`SHIPMENT_VERSION_POLICY=REAUTHORIZATION_REQUIRED`

A null vehicle is a normal shipment state (`000003_create_transport_tables.up.sql:169`) and a normal execution state (`000088_tms_transport_execution_foundation_v0_1a.up.sql:14`). It is not a wildcard.

`VEHICLE_AUTHORIZATION=CLAIMED_VEHICLE_MUST_MATCH_CURRENT_ROWS`

`EXECUTION_TARGET_AUTHORIZATION=BLOCKED_WHILE_OWNER_EPOCH_ABSENT`

An execution target is denied while `owner_epoch` is `ABSENT`. Link, revision, and driver match do not replace a confirmed epoch. The location right is separate and still requires a current decision.

## 1. Five rights

| Right | Driver-only decision | Vehicle-claimed decision |
| --- | --- | --- |
| Submit a driver location for the shipment | Allowed after the current driver assignment matches. | Allowed only when the claimed vehicle also matches the current shipment vehicle. |
| Match the driver to the current shipment | This is the existing check `CanDriverAccessShipment` (`driver_operations.go:271-278`). It does not include a vehicle. | Same driver check, plus the vehicle check. |
| Use the vehicle as a lookup key | Denied. The absent vehicle adds no vehicle id to the scope. | Allowed only for the vehicle on the current shipment row and, for an execution target, the same vehicle on the current execution row. |
| Select execution tracking targets | Denied. `EXECUTION_TARGET_AUTHORIZATION=BLOCKED_WHILE_OWNER_EPOCH_ABSENT`. A link, a revision, and a driver match do not grant this right while the epoch is unconfirmed. | Same denial. A vehicle match does not confirm an epoch. |
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

The location right is withheld when the driver row is inactive, the shipment driver differs, the caller is not the gateway user, the assignment rows cannot be read, or the decision's shipment version or revision no longer matches. An old decision is not a version-independent proof. `DRIVER_ONLY_REVISION_POLICY=REAUTHORIZATION_REQUIRED`. `SHIPMENT_VERSION_POLICY=REAUTHORIZATION_REQUIRED`.

The execution-target right is denied unless section 3.1 of the authority contract holds as a set. `owner_epoch=ABSENT` is sufficient to deny it. The location right does not require the epoch. It does require a new decision after a revision change, a shipment-version change, a vehicle change, or a driver change. The old decision is not carried forward.

## 4. Outcome table

| Event | Driver-only location right | Execution target |
| --- | --- | --- |
| Driver assignment is current and no vehicle is claimed | `DRIVER_ONLY_ALLOWED` on a new check | `EXECUTION_SCOPE_DENIED` without a confirmed epoch, link, revision, and use-time recheck |
| Vehicle changed | `REAUTHORIZATION_REQUIRED` | `EXECUTION_SCOPE_DENIED` until a new decision |
| Current revision changed | `REAUTHORIZATION_REQUIRED` | `EXECUTION_SCOPE_DENIED` until a new decision |
| Shipment version changed | `REAUTHORIZATION_REQUIRED` | `EXECUTION_SCOPE_DENIED` until a new decision |
| Driver reassigned | `HARD_DENIAL` until a new check succeeds for the new driver | `HARD_DENIAL` |
| Tenant mismatch | `HARD_DENIAL` | `HARD_DENIAL` |
| Authority source unavailable | `HARD_DENIAL` | `HARD_DENIAL` |

A revision change does not keep the location right alive. The successor update does not rewrite `driver_id` (`transport_execution_successor_repository.go:172-176`). That is not a proof that the driver assignment is independent of the revision, and this contract does not add a version-independent proof. The old decision grants no new location effect and no execution target.

## 5. Reassignment

A change of `shipment.driver_id` is `HARD_DENIAL` for both rights until a new check. A change of `shipment.vehicle_id` (`shipment_repository.go:472-475`) is `REAUTHORIZATION_REQUIRED` for a driver-only location and `EXECUTION_SCOPE_DENIED` for a target. The old decision id is not reused. A new check may return `DRIVER_ONLY_ALLOWED` only when the driver assignment is still current and no vehicle is claimed.

`VEHICLE_REASSIGNMENT_POLICY=CLAIMED_VEHICLE_MUST_MATCH_CURRENT_ROW`

# NLO-0.4A current-state inventory

Baseline: `origin/main` `b108c6cc62a23aaa9f5f866cf53c8d281fc14783` (PR #181). Migration head `000084_nlo_bounded_n_member_search_v0_3e`. This document cites code. It does not add a table or a service.

```text
NLO_0_3_COMPLETE=YES
NLO_0_4_IMPLEMENTATION_STARTED=NO
CURRENT_ROUTE_PLAN_TABLE=NONE
CURRENT_ROUTE_STOP_TABLE=NONE
```

## Shipment

`services/shipment-service/internal/domain/shipment.go` `Shipment` stores one `OriginLocationID` and one `DestinationLocationID`. There is no stop list and no leg list. `TransportOrderID` is optional. `Version` is an integer. Status values include `CARRIER_ASSIGNED`, `ACCEPTED_BY_CARRIER`, `VEHICLE_ASSIGNED`, `DRIVER_ASSIGNED`, `PICKUP_SLOT_BOOKED`, `DELIVERY_SLOT_BOOKED`, `IN_PICKUP`, `LOADED`, `IN_TRANSIT`, `ARRIVED_AT_CONSIGNEE`, `UNLOADING`, `DELIVERED`, and later billing states. Transitions are a single-pickup, single-delivery chain.

Milestones are `ShipmentStatusHistory`, not geographic stops (`ListShipmentMilestones`). Driver milestone actions in `execution_tracking.go` follow that same one-pickup chain (`ARRIVED_AT_PICKUP`, `PICKUP_COMPLETED`, `DEPARTED_PICKUP`, `ARRIVED_AT_DELIVERY`, `UNLOADING_STARTED`, `DELIVERY_COMPLETED` in `driver_operations.go`).

Outbox names are dotted and shipment-owned: `shipment.created`, `shipment.status.changed`, `shipment.cancelled`.

## Driver tasks

`DriverTask` in `driver_tasks.go` is an operational request to a driver: delay reason, status confirmation, arrival confirmation, document action, or a general notice. Statuses are pending, read, acknowledged, and terminal response states. A task may point at one shipment. It is not an ordered stop list. Owner: `shipment-service`.

## Current trip (NLO-0.3C / 0.3D)

`network-optimizer-service/internal/currenttrip` builds `CurrentTripContext` on the server from shipment execution, onboard evidence, cargo profiles, vehicle capability, tracking position, and ETA. Callers do not submit residual capacity. Onboard proof is `CONFIRMED_ONBOARD`. Unknown occupancy stays unknown. `OnboardCargoUnits` is a slice with no hard maximum of 2. `OnboardCargoUnit` carries `CargoID`, cargo profile version, evidence state, evidence state version, and evidence `OccurredAt`.

`Shipment` has planned and actual pickup and delivery timestamps. It has no service-duration field. `OPEN_QUESTIONS.md` Q2 records that no dwell table exists.

`searchCurrentTripFill` plans exactly one additional published load (`MaxAdditionalLoads = 1`). `insertRoad` calls `routing.Provider.Route` four times: direct position to destination, position to pickup, pickup to delivery, delivery to destination. Missing coordinates or a nil provider returns `ROUTING_PROVIDER_UNAVAILABLE`. The search does not insert a shipment stop, assign a driver, or book a slot. `execution_supported=false`.

NLO-0.3E `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER` evaluates cargo sets of 2 or 3 with the same pickup location and the same delivery location. It does not order stops. `MAX_ROUTING_CALLS=0` for that pattern.

## Loads, capacity, locations

Load opportunities and capacities live in `network-optimizer-service`. Canonical same-origin identity for consolidation is `location_id`. A missing location id is excluded, not treated as a match. `domain.HaversineCanonicalRoad` is false. Road metrics come from the routing port.

## Routing port

`internal/routing/provider.go` `Provider` has `Route` and `Matrix`. A request has origin, destination, optional `DepartureAt`, vehicle profile, route mode (`FASTEST` or `SHORTEST`), and traffic mode (`CURRENT` or `STATISTICAL`). Statistical mode is the one that buckets departure time into the fingerprint. The result has provider name, distance metres, duration seconds, optional geometry, fingerprints, `CalculatedAt`, and `ExpiresAt`. Errors are `ROUTING_PROVIDER_UNAVAILABLE`, `ROUTE_NOT_FOUND`, `ROUTING_TIMEOUT`, and `ROUTING_INVALID_RESPONSE`. Geometry is returned by the port. NLO-0.3D persists a route proof of distance, duration, and fingerprints, not an unbounded raw provider body.

## Slot booking

No slot-booking service or reservation table was found. The shipment status machine names `PICKUP_SLOT_BOOKED`. `DELIVERY_SLOT_BOOKED` is a constant and appears in a prediction status list, but it is not a target in `allowedStatusTransitions`. NLO-0.3D tests forbid a "book slot" side effect. Planning does not own a slot.

## Transport order

`transport-order-service` remains the commercial order owner. A shipment may reference one order. Neither order nor shipment stores an ordered multi-stop plan.

## Tracking

`tracking-service` publishes driver events through `transport.shipment_event_outbox`. Freshness consumed by current-trip planning is the status returned by tracking, not a GPS series recomputed inside the optimizer. NLO-0.2 prediction uses ETA. NLO-0.3C uses position and ETA freshness. NLO-0.4 planning must keep that split.

## Events and idempotency already present

`network-optimizer-service` has `network_optimizer.outbox` and an idempotency record keyed by tenant plus `Idempotency-Key`. Reuse of a key with a different body is a conflict. The header is required on the mutating network routes that already use it (`handlers.go`).

## What is absent

No `route_plans`, `route_stops`, `route_stop_actions`, or `route_legs` table. ADR-NET-005 is still Proposed and says a plan is not a shipment, and that multi-stop execution must not become active until shipment or an equivalent model can represent stops. That gate is still true. NLO-0.4A does not create migration `000085`.
